package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/ai"
	providerrequest "admin/server/internal/module/ai/providerRequest"
	airun "admin/server/internal/module/ai/run"
	auth "admin/server/internal/module/auth/login"
)

type ImageGenerator interface {
	Generate(context.Context, providerrequest.ImageRequest) (providerrequest.ImageResponse, error)
}

type ImageEditor interface {
	Edit(context.Context, providerrequest.ImageEditRequest) (providerrequest.ImageResponse, error)
}

type AudioGenerator interface {
	Transcribe(context.Context, providerrequest.TranscriptionRequest) (providerrequest.TranscriptionResponse, error)
	Synthesize(context.Context, providerrequest.SpeechRequest) (providerrequest.MediaStream, error)
}

type AIFileReader interface {
	ReadAIFrozenReference(context.Context, int64, int64, airun.InputFileSnapshot, int64) ([]byte, string, string, error)
}

type mediaInput struct {
	Text          string                    `json:"text"`
	Quality       string                    `json:"quality,omitempty"`
	Size          string                    `json:"size,omitempty"`
	Count         int                       `json:"count,omitempty"`
	Voice         string                    `json:"voice,omitempty"`
	Format        string                    `json:"format,omitempty"`
	FileIDs       []string                  `json:"fileIds,omitempty"`
	FileSnapshots []airun.InputFileSnapshot `json:"fileSnapshots,omitempty"`
	Params        json.RawMessage           `json:"params,omitempty"`
}

type Engine struct {
	repository  *airun.Repository
	credentials interface {
		Load(context.Context, airun.RuntimeSnapshot) (providerrequest.FrozenConfig, error)
	}
	images ImageGenerator
	editor ImageEditor
	audio  AudioGenerator
	files  AIFileReader
	media  *Service
	now    func() time.Time
}

func NewEngine(repository *airun.Repository, credentials interface {
	Load(context.Context, airun.RuntimeSnapshot) (providerrequest.FrozenConfig, error)
}, images ImageGenerator, editor ImageEditor, audio AudioGenerator, files AIFileReader, mediaService *Service) *Engine {
	return &Engine{repository: repository, credentials: credentials, images: images, editor: editor, audio: audio, files: files, media: mediaService, now: time.Now}
}

func (e *Engine) Execute(ctx context.Context, input airun.Model) error {
	if e == nil || e.repository == nil || e.credentials == nil || e.media == nil {
		return errors.New("AI media engine unavailable")
	}
	row, err := e.repository.Find(ctx, input.ID)
	if err != nil {
		return err
	}
	if row.ExecutionStatus >= 3 || row.ExecutionStatus != 0 {
		return nil
	}
	token := uuid.NewString()
	row, err = e.repository.Claim(ctx, row.ID, token, e.now().UTC())
	if err != nil {
		return err
	}
	if row.ExecutionStatus >= 3 {
		return nil
	}
	work, cancel := context.WithDeadline(ctx, row.DeadlineAt)
	defer cancel()
	work, leaseCancel, _ := airun.LeaseHeartbeat(work, e.repository, row.ID, token, e.now)
	defer leaseCancel(nil)
	code := e.generate(work, row, token, ctx)
	finalize, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer finishCancel()
	status := int16(3)
	if code != "" {
		status = 4
	}
	return e.repository.FinishMedia(finalize, row.ID, token, status, code, e.now().UTC())
}

func (e *Engine) generate(ctx context.Context, row airun.Model, token string, transferContexts ...context.Context) string {
	mediaCtx := ctx
	if len(transferContexts) > 0 {
		mediaCtx = transferContexts[0]
	}
	var snapshot airun.RuntimeSnapshot
	var config airun.RuntimeConfig
	var policy airun.Policy
	var input mediaInput
	if json.Unmarshal(row.ConfigSnapshot, &snapshot) != nil || json.Unmarshal(snapshot.Config, &config) != nil || json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil || json.Unmarshal(row.InputSnapshot, &input) != nil {
		return "INVALID_SNAPSHOT"
	}
	mediaParams, err := FrozenMediaParams(config)
	if err != nil {
		return "INVALID_PARAMETERS"
	}
	if row.Operation != string(ai.OperationImage) && row.Operation != string(ai.OperationImageEdit) && row.Operation != string(ai.OperationAudio) && row.Operation != string(ai.OperationAudioTranscribe) {
		return "UNSUPPORTED_OPERATION"
	}
	var inputData []byte
	var inputMIME, inputName string
	if row.Operation == string(ai.OperationImageEdit) || row.Operation == string(ai.OperationAudioTranscribe) {
		if e.files == nil || len(input.FileSnapshots) != 1 {
			return "MEDIA_INPUT_UNAVAILABLE"
		}
		maxBytes := policy.ImageMaxBytes
		if row.Operation == string(ai.OperationAudioTranscribe) {
			maxBytes = policy.AudioMaxBytes
		}
		var err error
		inputData, inputMIME, inputName, err = e.files.ReadAIFrozenReference(ctx, row.UserID, row.PlatformID, input.FileSnapshots[0], maxBytes)
		if err != nil {
			return "MEDIA_INPUT_UNAVAILABLE"
		}
	}
	if (row.Operation == string(ai.OperationImage) && e.images == nil) || (row.Operation == string(ai.OperationImageEdit) && e.editor == nil) || ((row.Operation == string(ai.OperationAudio) || row.Operation == string(ai.OperationAudioTranscribe)) && e.audio == nil) {
		return "MEDIA_CLIENT_UNAVAILABLE"
	}
	credential, err := e.credentials.Load(ctx, snapshot)
	if err != nil {
		return "CREDENTIAL_UNAVAILABLE"
	}
	if err = e.repository.Fence(ctx, row.ID, token, e.now().UTC()); err != nil {
		return "CANCELED_OR_LEASE_LOST"
	}
	summary, _ := json.Marshal(map[string]any{"operation": row.Operation, "model": credential.Model, "input": input})
	fact, err := e.repository.StartRequest(ctx, row.ID, token, 1, summary, e.now().UTC())
	if err != nil {
		return "REQUEST_INTENT_FAILED"
	}
	var items json.RawMessage
	var usage providerrequest.MediaUsage
	var requestID string
	var requestErr error
	switch ai.Operation(row.Operation) {
	case ai.OperationImage, ai.OperationImageEdit:
		var response providerrequest.ImageResponse
		var callErr error
		if row.Operation == string(ai.OperationImageEdit) {
			if e.editor == nil || e.files == nil || len(input.FileSnapshots) != 1 {
				return e.finishFailure(ctx, row, token, fact.ID, items, usage, "MEDIA_INPUT_UNAVAILABLE", nil)
			}

			response, callErr = e.editor.Edit(ctx, providerrequest.ImageEditRequest{Config: credential, Prompt: input.Text, N: input.Count, Size: input.Size, Quality: input.Quality, MaxBytes: policy.ImageMaxBytes, Image: bytes.NewReader(inputData), Filename: inputName, MIME: inputMIME, Params: mediaParams})
		} else {
			response, callErr = e.images.Generate(ctx, providerrequest.ImageRequest{Config: credential, Prompt: input.Text, N: input.Count, Size: input.Size, Quality: input.Quality, OutputFormat: input.Format, Params: mediaParams})
		}
		requestID, requestErr, usage = response.RequestID, callErr, response.Usage
		resultAt := e.now().UTC()
		safeItems, prepared := e.prepareImageResultFiles(row, fact.ID, response.Items, credential.MediaHostAllowlist, policy, resultAt)
		items, _ = json.Marshal(safeItems)
		if callErr == nil {
			if err := e.repository.FinishMediaResult(context.WithoutCancel(ctx), row.ID, token, fact.ID, requestID, safeItems, usage, nil, prepared, resultAt); err != nil {
				return "REQUEST_RESULT_FAILED"
			}

			if err := e.repository.FinishMedia(context.WithoutCancel(ctx), row.ID, token, 3, "", e.now().UTC()); err != nil {
				return "MEDIA_FINISH_FAILED"
			}
			_ = e.importImages(mediaCtx, row, response.Items, fact.ID, credential.MediaHostAllowlist, policy)
			return ""
		}
	case ai.OperationAudio:
		if e.audio == nil {
			return e.finishFailure(ctx, row, token, fact.ID, items, usage, "MEDIA_CLIENT_UNAVAILABLE", nil)
		}
		stream, callErr := e.audio.Synthesize(ctx, providerrequest.SpeechRequest{Config: credential, Input: input.Text, Voice: input.Voice, ResponseFormat: input.Format, Params: speechParams(config, mediaParams)})
		requestErr = callErr
		if callErr == nil {
			requestID = stream.RequestID
			usage = speechUsage(config, input.Text, stream.Usage)

			_, importErr := e.media.ImportInline(mediaCtx, auth.Identity{UserID: row.UserID, PlatformID: row.PlatformID}, InlineImport{RunID: row.ID, ProviderID: fact.ID, Ordinal: 0, RequestSeed: fmt.Sprintf("%d", fact.ID), OriginalName: "speech." + normalizedFormat(input.Format), MIME: stream.MIME, MaxBytes: policy.AudioMaxBytes, TransferTimeout: time.Duration(policy.TransferTimeoutSeconds) * time.Second, AllowedMIMEs: []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4"}, Body: stream.Body, ContentLength: stream.ContentLength, SourceKind: SourceStream})
			if final, ok := stream.Body.(interface {
				Usage() providerrequest.MediaUsage
			}); ok {
				usage = final.Usage()
			}
			var providerErr error
			if errors.Is(importErr, ErrSourceRead) {
				providerErr = fmt.Errorf("%w: media stream incomplete", providerrequest.ErrSubmissionUnknown)
			}
			resultAt := e.now().UTC()
			prepared, prepareErr := e.media.PrepareResultFile(InlineImport{RunID: row.ID, ProviderID: fact.ID, Ordinal: 0, RequestSeed: fmt.Sprintf("%d", fact.ID), MIME: stream.MIME, SourceKind: SourceStream, TransferTimeout: time.Duration(policy.TransferTimeoutSeconds) * time.Second}, "", nil, resultAt)
			if prepareErr != nil {
				prepared = e.media.FailedResultFile(InlineImport{RunID: row.ID, ProviderID: fact.ID, Ordinal: 0, RequestSeed: fmt.Sprintf("%d", fact.ID), SourceKind: SourceStream, TransferTimeout: time.Duration(policy.TransferTimeoutSeconds) * time.Second}, resultAt, "INVALID_SOURCE")
			}
			if importErr != nil {
				markResultUnrecoverable(&prepared, "SOURCE_UNRECOVERABLE", "一次性媒体结果保存失败且无法恢复")
			}
			if err := e.repository.FinishMediaResult(context.WithoutCancel(ctx), row.ID, token, fact.ID, requestID, []airun.MediaResultItem{{Ordinal: 0}}, usage, providerErr, []airun.MediaResultFile{prepared}, resultAt); err != nil {
				return "REQUEST_RESULT_FAILED"
			}
			if providerErr != nil {
				return "PROVIDER_STREAM_FAILED"
			}
			return ""
		}

	case ai.OperationAudioTranscribe:
		if e.audio == nil || e.files == nil || len(input.FileSnapshots) != 1 {
			return e.finishFailure(ctx, row, token, fact.ID, items, usage, "MEDIA_INPUT_UNAVAILABLE", nil)
		}

		response, callErr := e.audio.Transcribe(ctx, providerrequest.TranscriptionRequest{Config: credential, MaxBytes: policy.AudioMaxBytes, Audio: bytes.NewReader(inputData), Filename: inputName, MIME: inputMIME, ResponseFormat: input.Format, Prompt: input.Text, Params: mediaParams})
		requestID, requestErr, usage = response.RequestID, callErr, response.Usage
		items, _ = json.Marshal([]map[string]string{{"text": response.Text}})
	}
	if len(items) == 0 {
		items = json.RawMessage(`[]`)
	}
	if requestErr != nil && !errors.Is(requestErr, providerrequest.ErrSubmissionUnknown) {
		usage = zeroMediaUsage()
	}
	if requestID == "" {
		var failure *providerrequest.RequestError
		if errors.As(requestErr, &failure) {
			requestID = failure.RequestID
		}
	}
	if err := e.repository.FinishMediaRequest(context.WithoutCancel(ctx), row.ID, token, fact.ID, requestID, items, usage, requestErr, e.now().UTC()); err != nil {
		return "REQUEST_RESULT_FAILED"
	}
	if requestErr != nil {
		return "PROVIDER_REQUEST_FAILED"
	}
	return ""
}

func (e *Engine) finishFailure(ctx context.Context, row airun.Model, token string, factID int64, items json.RawMessage, usage providerrequest.MediaUsage, code string, requestErr error) string {
	if len(items) == 0 {
		items = json.RawMessage(`[]`)
	}
	if requestErr == nil || !errors.Is(requestErr, providerrequest.ErrSubmissionUnknown) {
		usage = zeroMediaUsage()
	}
	if err := e.repository.FinishMediaRequest(context.WithoutCancel(ctx), row.ID, token, factID, "", items, usage, requestErr, e.now().UTC()); err != nil {
		return "REQUEST_RESULT_FAILED"
	}
	if requestErr != nil && code == "" {
		return "PROVIDER_REQUEST_FAILED"
	}
	return code
}

func (e *Engine) importImages(ctx context.Context, row airun.Model, items []providerrequest.ImageItem, providerRequestID int64, hosts []string, policy airun.Policy) error {
	var failures []error
	for index, item := range items {
		in := imageImport(row, providerRequestID, index, policy)
		if item.URL != "" {
			continue
		}
		if item.B64JSON == "" {
			failures = append(failures, errors.New("image output is empty"))
			continue
		}
		if _, err := e.media.ImportBase64(ctx, auth.Identity{UserID: row.UserID, PlatformID: row.PlatformID}, in, item.B64JSON); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func speechUsage(config airun.RuntimeConfig, text string, usage providerrequest.MediaUsage) providerrequest.MediaUsage {
	if usage.Known {
		return usage
	}
	for _, rate := range config.Rates {
		if rate.Operation == string(ai.OperationAudio) && rate.Unit == "character" {
			n := int64(utf8.RuneCountInString(text))
			return providerrequest.MediaUsage{Known: true, Basis: "character", TextInputTokens: &n, Raw: usage.Raw}
		}
	}
	return usage
}

func normalizedFormat(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "mp3"
	}
	return value
}

func parseID(value string) int64 {
	var id int64
	_, _ = fmt.Sscan(value, &id)
	return id
}

func imageImport(row airun.Model, providerRequestID int64, index int, policy airun.Policy) InlineImport {
	var input mediaInput
	_ = json.Unmarshal(row.InputSnapshot, &input)
	format := strings.ToLower(input.Format)
	if len(input.Params) > 0 {
		var params struct {
			Format string `json:"output_format"`
		}
		if json.Unmarshal(input.Params, &params) == nil && params.Format != "" {
			format = params.Format
		}
	}
	mime, ext := "image/png", "png"
	switch format {
	case "jpeg", "jpg":
		mime, ext = "image/jpeg", "jpg"
	case "webp":
		mime, ext = "image/webp", "webp"
	}
	return InlineImport{RunID: row.ID, ProviderID: providerRequestID, Ordinal: index, RequestSeed: fmt.Sprintf("%d:%d", providerRequestID, index), OriginalName: fmt.Sprintf("image-%d.%s", index, ext), MIME: mime, MaxBytes: policy.ImageMaxBytes, TransferTimeout: time.Duration(policy.TransferTimeoutSeconds) * time.Second, DownloadTimeout: time.Duration(policy.DownloadTimeoutSeconds) * time.Second, AllowedMIMEs: []string{"image/png", "image/jpeg", "image/webp"}}
}

func speechParams(config airun.RuntimeConfig, raw json.RawMessage) json.RawMessage {
	tokens := false
	for _, rate := range config.Rates {
		if rate.Operation == string(ai.OperationAudio) && rate.Unit == "token" {
			tokens = true
		}
	}
	if !tokens {
		return raw
	}
	fields := map[string]json.RawMessage{}
	if len(raw) > 0 && json.Unmarshal(raw, &fields) != nil {
		return raw
	}
	fields["stream_format"] = json.RawMessage(`"sse"`)
	out, _ := json.Marshal(fields)
	return out
}

func zeroMediaUsage() providerrequest.MediaUsage {
	zero := int64(0)
	return providerrequest.MediaUsage{Known: true, Basis: "rejected", ImageCount: 0, DurationSeconds: "0", InputTokens: &zero, OutputTokens: &zero, AudioInputTokens: &zero, TextInputTokens: &zero}
}

func (e *Engine) prepareImageResultFiles(row airun.Model, providerID int64, items []providerrequest.ImageItem, hosts []string, policy airun.Policy, now time.Time) ([]airun.MediaResultItem, []airun.MediaResultFile) {
	safe := make([]airun.MediaResultItem, len(items))
	files := make([]airun.MediaResultFile, len(items))
	for index, item := range items {
		safe[index] = airun.MediaResultItem{Ordinal: index, RevisedPrompt: item.RevisedPrompt}
		in := imageImport(row, providerID, index, policy)
		output, err := e.media.PrepareResultFile(in, item.URL, hosts, now)
		if err != nil {
			if item.URL != "" {
				in.SourceKind = SourceURL
			}
			output = e.media.FailedResultFile(in, now, "INVALID_SOURCE")
		}
		if item.URL == "" && item.B64JSON == "" {
			markResultUnrecoverable(&output, "MEDIA_OUTPUT_EMPTY", "供应商媒体结果为空")
		}
		files[index] = output
	}
	return safe, files
}
