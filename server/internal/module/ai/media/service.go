package media

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	providerrequest "admin/server/internal/module/ai/providerRequest"
	airun "admin/server/internal/module/ai/run"
	auth "admin/server/internal/module/auth/login"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/shared/apperror"
)

type RunLoader interface {
	Find(context.Context, int64) (airun.Model, error)
}

type CredentialLoader interface {
	Load(context.Context, airun.RuntimeSnapshot) (providerrequest.FrozenConfig, error)
}

type VideoPoller interface {
	Poll(context.Context, providerrequest.FrozenConfig, string) (providerrequest.VideoResource, error)
}

type FileImporter interface {
	ImportGeneratedMedia(context.Context, auth.Identity, storagefile.GeneratedMediaImport) (int64, error)
}

type SourceCipher interface {
	Open(string, string) (string, error)
}

type EventSink interface {
	MediaUpdated(context.Context, int64, int64, int64, time.Time) error
}

type Service struct {
	repository  *Repository
	runs        RunLoader
	credentials CredentialLoader
	video       VideoPoller
	spool       *Spooler
	files       FileImporter
	cipher      SourceCipher
	events      EventSink
	now         func() time.Time
}

var ErrSourceRead = errors.New("provider media stream could not be read completely")

type InlineImport struct {
	RunID           int64
	ProviderID      int64
	Ordinal         int
	RequestSeed     string
	OriginalName    string
	MIME            string
	MaxBytes        int64
	AllowedMIMEs    []string
	Body            io.ReadCloser
	ContentLength   int64
	SourceKind      int16
	DownloadTimeout time.Duration
	TransferTimeout time.Duration
}

func NewService(repository *Repository, runs RunLoader, credentials CredentialLoader, video VideoPoller, spool *Spooler, files FileImporter, cipher SourceCipher, events EventSink) *Service {
	return &Service{repository: repository, runs: runs, credentials: credentials, video: video, spool: spool, files: files, cipher: cipher, events: events, now: time.Now}
}

func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.runs == nil || s.credentials == nil || s.video == nil || s.spool == nil || s.files == nil || s.cipher == nil {
		return errors.New("AI media dependencies unavailable")
	}
	return nil
}

// ImportInline streams a completed provider body through the same bounded
// spool and storage import path as asynchronous media. It does not create a
// second charge or regenerate upstream content.
func (s *Service) ImportInline(ctx context.Context, id auth.Identity, in InlineImport) (int64, error) {
	if err := s.ValidateDependencies(); err != nil {
		return 0, err
	}
	if in.Body == nil || in.RunID < 1 || in.ProviderID < 1 || in.Ordinal < 0 || in.RequestSeed == "" || in.MaxBytes < 1 || in.TransferTimeout <= 0 {
		return 0, errors.New("AI inline media import invalid")
	}
	f, err := s.prepareInline(ctx, in)
	if err != nil {
		// A persistence outage cannot erase final upstream usage. Drain through the
		// same bounded spool while the owner worker still has the one-shot stream.
		work, cancel := context.WithTimeout(ctx, in.TransferTimeout)
		defer cancel()
		spool, readErr := s.spool.FromStream(work, in.Body, in.MIME, in.ContentLength, Constraints{MaxBytes: in.MaxBytes, AllowedMIMEs: in.AllowedMIMEs, SourceMIME: in.MIME})
		if readErr != nil {
			return 0, errors.Join(err, ErrSourceRead, readErr)
		}
		defer spool.Close()
		return 0, err
	}
	if f.TransferStatus == TransferReady && f.StorageFileID != nil {
		in.Body.Close()
		return *f.StorageFileID, nil
	}
	if f.TransferStatus == TransferFailed {
		in.Body.Close()
		return 0, errors.New("media source is no longer recoverable")
	}
	deadline := *f.TransferDeadline
	work, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	spool, err := s.spool.FromStream(work, in.Body, in.MIME, in.ContentLength, Constraints{MaxBytes: in.MaxBytes, AllowedMIMEs: in.AllowedMIMEs, SourceMIME: in.MIME, Deadline: deadline})
	if err == nil {
		defer spool.Close()
		return s.saveInlineSpool(work, id, in, f, spool)
	}
	return 0, s.recordInlineFailure(ctx, f, errors.Join(ErrSourceRead, err))
}
func (s *Service) saveInlineSpool(ctx context.Context, id auth.Identity, in InlineImport, f *File, spool *SpoolFile) (int64, error) {
	body, err := os.Open(spool.Path)
	if err != nil {
		return 0, s.recordInlineFailure(ctx, f, err)
	}
	defer body.Close()
	storageID, err := s.files.ImportGeneratedMedia(ctx, id, storagefile.GeneratedMediaImport{ImportRequestKey: *f.ImportRequestKey, OriginalName: in.OriginalName, MIME: spool.MIME, SizeBytes: spool.SizeBytes, Body: body})
	if err != nil {
		return 0, s.recordInlineFailure(ctx, f, err)
	}
	finalize, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.repository.FinishImport(finalize, f.ID, storageID, TransferReady, "", "", nil, s.now().UTC(), &spool.MIME, &spool.SizeBytes, f.TransferAttempts); err != nil {
		return 0, err
	}
	if s.events != nil {
		if err := s.events.MediaUpdated(finalize, f.RunID, f.ID, storageID, s.now().UTC()); err != nil {
			return 0, err
		}
	}
	return storageID, nil
}
func (s *Service) recordInlineFailure(ctx context.Context, f *File, cause error) error {
	finalize, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.finishImportFailure(finalize, *f, "SOURCE_UNRECOVERABLE", "一次性媒体结果保存失败且无法恢复", s.now().UTC()); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// ImportURL freezes the encrypted source before any download. Durable workers save the same output.
func (s *Service) ImportURL(ctx context.Context, id auth.Identity, in InlineImport, sourceURL string, hosts []string) (int64, error) {
	if err := s.ValidateDependencies(); err != nil {
		return 0, err
	}
	if in.RunID < 1 || in.ProviderID < 1 || in.Ordinal < 0 || in.RequestSeed == "" || sourceURL == "" || in.TransferTimeout <= 0 {
		return 0, errors.New("AI URL media import invalid")
	}
	sealed, keyID, err := s.cipherSeal(sourceURL)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	deadline := now.Add(in.TransferTimeout)
	key := deterministicUUID(fmt.Sprintf("%d:%d:%d:%s", in.RunID, in.ProviderID, in.Ordinal, in.RequestSeed))
	meta, _ := json.Marshal(map[string]any{"allowedHosts": hosts, "originalName": in.OriginalName})
	f := &File{RunID: in.RunID, Role: FileRoleGenerated, Ordinal: in.Ordinal, ImportRequestKey: &key, SourceKind: SourceURL, SourceCiphertext: &sealed, EncryptionKeyID: &keyID, SourceRecoverable: 1, TransferDeadline: &deadline, MIMEType: &in.MIME, Metadata: meta, TransferStatus: TransferQueued, NextTransferAt: &now, CreatedAt: now, UpdatedAt: now}
	return 0, s.repository.CreateGeneratedFile(ctx, f)
}
func (s *Service) ImportBase64(ctx context.Context, id auth.Identity, in InlineImport, encoded string) (int64, error) {
	src := io.NopCloser(strings.NewReader(encoded))
	in.Body = &base64ReadCloser{Reader: base64.NewDecoder(base64.StdEncoding, src), source: src}
	in.ContentLength = -1
	in.SourceKind = SourceBase64
	return s.ImportInline(ctx, id, in)
}
func (s *Service) RecoverDue(ctx context.Context, now time.Time, limit int) (int, error) {
	return s.repository.RecoverDue(ctx, now, limit)
}

func (s *Service) PollOne(ctx context.Context, payload json.RawMessage) error {
	if err := s.ValidateDependencies(); err != nil {
		return err
	}
	taskID, ok := mediaTaskID(payload)
	if !ok {
		return errors.New("AI media poll payload is invalid")
	}
	now := s.now().UTC()
	task, found, err := s.repository.ClaimPollByID(ctx, taskID, now)
	if err != nil || !found {
		return err
	}
	if task.Status == TaskExpired {
		return nil
	}
	if !task.DeadlineAt.After(now) {
		return s.expirePoll(ctx, task, now)
	}
	if task.UpstreamTaskID == nil || strings.TrimSpace(*task.UpstreamTaskID) == "" {
		return s.finishPollFailure(ctx, task, "UPSTREAM_TASK_MISSING", "供应商媒体任务标识缺失", now)
	}
	runRow, err := s.runs.Find(ctx, task.RunID)
	if err != nil {
		return s.retryPoll(ctx, task, "RUN_UNAVAILABLE", "媒体依赖暂时不可用", now)
	}
	var snapshot airun.RuntimeSnapshot
	if json.Unmarshal(runRow.ConfigSnapshot, &snapshot) != nil {
		return s.finishPollFailure(ctx, task, "INVALID_SNAPSHOT", "媒体任务配置快照无效", now)
	}
	config, err := s.credentials.Load(ctx, snapshot)
	if err != nil {
		return s.retryPoll(ctx, task, "CREDENTIAL_UNAVAILABLE", "媒体依赖暂时不可用", now)
	}
	pollCtx, pollCancel := context.WithDeadline(ctx, task.DeadlineAt)
	resource, err := s.video.Poll(pollCtx, config, *task.UpstreamTaskID)
	pollCancel()
	completedAt := s.now().UTC()
	if err != nil {
		if errors.Is(pollCtx.Err(), context.DeadlineExceeded) && !task.DeadlineAt.After(completedAt) {
			return s.expirePoll(ctx, task, completedAt)
		}
		return s.retryPoll(ctx, task, "UPSTREAM_POLL_FAILED", "媒体依赖暂时不可用", completedAt)
	}
	if !task.DeadlineAt.After(completedAt) {
		return s.expirePoll(ctx, task, completedAt)
	}
	status := strings.ToLower(strings.TrimSpace(resource.Status))
	progress := int16(resource.Progress)
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	result, _ := json.Marshal(struct {
		ID       string                     `json:"id"`
		Status   string                     `json:"status"`
		Progress int                        `json:"progress"`
		Usage    providerrequest.MediaUsage `json:"usage"`
	}{resource.ID, resource.Status, resource.Progress, resource.Usage})
	switch status {
	case "completed", "succeeded", "success":
		if strings.TrimSpace(resource.URL) == "" {
			return s.finishPollFailure(ctx, task, "MEDIA_URL_MISSING", "供应商未返回媒体地址", completedAt)
		}
		if err := s.createURLFile(ctx, task, runRow, resource.URL, result, config.MediaHostAllowlist, completedAt); err != nil {
			return err
		}
		return s.repository.MarkPoll(ctx, task.ID, TaskSucceeded, &progress, result, "", "", nil, completedAt)
	case "failed", "error", "canceled", "cancelled":
		message := "供应商媒体任务失败"
		return s.finishPollFailure(ctx, task, "UPSTREAM_MEDIA_FAILED", message, completedAt)
	default:
		return s.repository.MarkPoll(ctx, task.ID, TaskPolling, &progress, result, "", "", timePtr(completedAt.Add(time.Minute)), completedAt)
	}
}

func (s *Service) expirePoll(ctx context.Context, task Task, now time.Time) error {
	finalize, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	result := task.UpstreamResult
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	return s.repository.MarkPoll(finalize, task.ID, TaskExpired, task.Progress, result, "MEDIA_DEADLINE", "媒体任务超过转存截止时间", nil, now)
}

func (s *Service) createURLFile(ctx context.Context, task Task, runRow airun.Model, sourceURL string, result json.RawMessage, hosts []string, now time.Time) error {
	key := fmt.Sprintf("%d-%d", task.RunID, task.ID)
	requestKey := deterministicUUID(key)
	sealed, encryptionID, err := s.cipherSeal(sourceURL)
	if err != nil {
		return err
	}
	mimeType := "video/mp4"
	metadata, _ := json.Marshal(map[string]any{"sourceUrl": true, "allowedHosts": hosts})
	var policy airun.Policy
	if json.Unmarshal(runRow.PolicySnapshot, &policy) != nil || policy.Validate() != nil {
		return airun.ErrPolicyInvalid
	}
	deadline := now.Add(time.Duration(policy.TransferTimeoutSeconds) * time.Second)
	file := &File{RunID: task.RunID, MediaTaskID: &task.ID, Role: FileRoleGenerated, Ordinal: 0, ImportRequestKey: &requestKey, SourceKind: SourceURL, SourceCiphertext: &sealed, EncryptionKeyID: &encryptionID, SourceRecoverable: 1, TransferDeadline: &deadline, MIMEType: &mimeType, Metadata: metadata, TransferStatus: TransferQueued, TransferAttempts: 0, NextTransferAt: &now, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateGeneratedFile(ctx, file); err != nil {
		return err
	}
	if s.events != nil {
		return s.events.MediaUpdated(ctx, runRow.ID, file.ID, 0, now)
	}
	return nil
}

func (s *Service) ImportOne(ctx context.Context, payload json.RawMessage) error {
	if err := s.ValidateDependencies(); err != nil {
		return err
	}
	fileID, ok := mediaFileID(payload)
	if !ok {
		return errors.New("AI media import payload is invalid")
	}
	now := s.now().UTC()
	file, found, err := s.repository.ClaimImportByID(ctx, fileID, now)
	if err != nil || !found {
		return err
	}
	if file.TransferStatus != Transfering {
		if s.events != nil {
			return s.events.MediaUpdated(ctx, file.RunID, file.ID, 0, now)
		}
		return nil
	}
	if file.ImportRequestKey == nil || file.SourceCiphertext == nil || file.EncryptionKeyID == nil || file.TransferDeadline == nil {
		return s.finishImportFailure(ctx, file, "SOURCE_UNRECOVERABLE", "媒体源不可恢复", now)
	}
	source, err := s.cipher.Open(*file.SourceCiphertext, *file.EncryptionKeyID)
	if err != nil {
		return s.finishImportFailure(ctx, file, "SOURCE_UNAVAILABLE", "媒体源解密失败", now)
	}
	runRow, err := s.runs.Find(ctx, file.RunID)
	if err != nil {
		return s.retryImport(ctx, file, "RUN_UNAVAILABLE", "媒体依赖暂时不可用", now)
	}
	var policy airun.Policy
	if err := json.Unmarshal(runRow.PolicySnapshot, &policy); err != nil || policy.Validate() != nil {
		return s.finishImportFailure(ctx, file, "INVALID_POLICY", "媒体转存策略无效", now)
	}
	constraints, err := mediaConstraints(file, policy, now)
	if err != nil {
		return s.finishImportFailure(ctx, file, "INVALID_POLICY", "媒体转存策略无效", now)
	}
	work, stop := s.importHeartbeat(ctx, file)
	defer stop()
	ctx = work
	work, cancel := context.WithDeadline(ctx, constraints.Deadline)
	defer cancel()
	ctx = work
	var metadata struct {
		AllowedHosts []string `json:"allowedHosts"`
	}
	if json.Unmarshal(file.Metadata, &metadata) == nil {
		constraints.AllowedHosts = metadata.AllowedHosts
	}
	var spoolFile *SpoolFile
	switch file.SourceKind {
	case SourceURL:
		spoolFile, err = s.spool.FromURL(ctx, source, constraints)
	case SourceBase64:
		spoolFile, err = s.spool.FromBase64(ctx, io.NopCloser(strings.NewReader(source)), constraints)
	default:
		err = errors.New("stream source is not recoverable")
	}
	if err != nil {
		var corrupt base64.CorruptInputError
		if errors.Is(err, ErrSourceInvalid) || errors.As(err, &corrupt) {
			return s.finishImportFailure(ctx, file, "SOURCE_INVALID", "媒体来源大小、格式或内容无效", s.now().UTC())
		}
		return s.retryImport(ctx, file, "SOURCE_FETCH_FAILED", "媒体依赖暂时不可用", now)
	}
	defer spoolFile.Close()
	body, err := os.Open(spoolFile.Path)
	if err != nil {
		return s.retryImport(ctx, file, "SPOOL_OPEN_FAILED", "媒体依赖暂时不可用", now)
	}
	defer body.Close()
	storageID, err := s.files.ImportGeneratedMedia(ctx, auth.Identity{UserID: runRow.UserID, PlatformID: runRow.PlatformID}, storagefile.GeneratedMediaImport{ImportRequestKey: *file.ImportRequestKey, OriginalName: generatedName(file), MIME: spoolFile.MIME, SizeBytes: spoolFile.SizeBytes, Body: body})
	if err != nil {
		var rejected *apperror.Error
		if errors.As(err, &rejected) && (rejected.Code == apperror.CodeInvalidRequest || rejected.Code == apperror.CodeConflict || rejected.Code == apperror.CodeForbidden || rejected.Code == apperror.CodeNotFound) {
			return s.finishImportFailure(ctx, file, "STORAGE_REJECTED", "媒体不符合冻结来源或存储规则", s.now().UTC())
		}
		return s.retryImport(ctx, file, "STORAGE_IMPORT_FAILED", "媒体依赖暂时不可用", now)
	}
	if err := s.repository.FinishImport(ctx, file.ID, storageID, TransferReady, "", "", nil, now, &spoolFile.MIME, &spoolFile.SizeBytes, file.TransferAttempts); err != nil {
		return err
	}
	if s.events != nil {
		return s.events.MediaUpdated(ctx, runRow.ID, file.ID, storageID, now)
	}
	return nil
}

func (s *Service) ImportTimeoutForPayload(ctx context.Context, payload json.RawMessage, now time.Time) (time.Duration, error) {
	if s == nil || s.repository == nil {
		return 0, ErrUnavailable
	}
	fileID, ok := mediaFileID(payload)
	if !ok {
		return 30 * time.Second, nil
	}
	file, found, err := s.repository.FindImport(ctx, fileID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 30 * time.Second, nil
	}
	return s.importTimeout(file, now)
}

func (s *Service) importTimeout(file File, now time.Time) (time.Duration, error) {
	const finishWindow = 30 * time.Second
	if file.TransferStatus != TransferQueued && file.TransferStatus != TransferRetry && file.TransferStatus != Transfering || file.TransferDeadline == nil {
		return finishWindow, nil
	}
	deadline := *file.TransferDeadline
	if file.SourceExpiresAt != nil && file.SourceExpiresAt.Before(deadline) {
		deadline = *file.SourceExpiresAt
	}
	if !deadline.After(now) {
		return finishWindow, nil
	}
	remaining := deadline.Sub(now)
	if remaining > time.Duration(1<<63-1)-finishWindow {
		return time.Duration(1<<63 - 1), nil
	}
	return remaining + finishWindow, nil
}

func mediaTaskID(payload json.RawMessage) (int64, bool) {
	var value struct {
		ID int64 `json:"mediaTaskId"`
	}
	if json.Unmarshal(payload, &value) != nil || value.ID < 1 {
		return 0, false
	}
	return value.ID, true
}

func mediaFileID(payload json.RawMessage) (int64, bool) {
	var value struct {
		ID int64 `json:"mediaFileId"`
	}
	if json.Unmarshal(payload, &value) != nil || value.ID < 1 {
		return 0, false
	}
	return value.ID, true
}

func (s *Service) finishPollFailure(ctx context.Context, task Task, code, message string, now time.Time) error {
	return s.repository.MarkPoll(ctx, task.ID, TaskFailed, nil, json.RawMessage(`{}`), code, message, nil, now)
}

func (s *Service) retryPoll(ctx context.Context, task Task, code, message string, now time.Time) error {
	if s.now != nil {
		now = s.now().UTC()
	}
	if !task.DeadlineAt.After(now) || task.PollAttempts >= 10 {
		return s.finishPollFailure(ctx, task, code, message, now)
	}
	next := now.Add(backoff(task.PollAttempts))
	if next.After(task.DeadlineAt) {
		next = task.DeadlineAt
	}
	return s.repository.MarkPoll(ctx, task.ID, TaskPolling, task.Progress, json.RawMessage(`{}`), code, message, &next, now)
}

func (s *Service) finishImportFailure(ctx context.Context, file File, code, message string, now time.Time) error {
	if err := s.repository.FinishImport(ctx, file.ID, 0, TransferFailed, code, message, nil, now, nil, nil, file.TransferAttempts); err != nil {
		return err
	}
	if s.events != nil {
		return s.events.MediaUpdated(ctx, file.RunID, file.ID, 0, now)
	}
	return nil
}

func (s *Service) retryImport(ctx context.Context, file File, code, message string, now time.Time) error {
	if s.now != nil {
		now = s.now().UTC()
	}
	if file.TransferDeadline == nil || !file.TransferDeadline.After(now) || file.TransferAttempts >= 3 || file.SourceRecoverable != 1 {
		return s.finishImportFailure(ctx, file, code, message, now)
	}
	next := now.Add(importBackoff(file.TransferAttempts))
	if next.After(*file.TransferDeadline) {
		next = *file.TransferDeadline
	}
	if err := s.repository.FinishImport(ctx, file.ID, 0, TransferRetry, code, message, &next, now, nil, nil, file.TransferAttempts); err != nil {
		return err
	}
	if s.events != nil {
		return s.events.MediaUpdated(ctx, file.RunID, file.ID, 0, now)
	}
	return nil
}

func mediaConstraints(file File, policy airun.Policy, now time.Time) (Constraints, error) {
	allowed := []string{"image/png", "image/jpeg", "image/webp", "image/gif", "audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "video/mp4", "video/webm"}
	maxBytes := policy.VideoMaxBytes
	if file.MIMEType != nil && strings.TrimSpace(*file.MIMEType) != "" {
		allowed = []string{*file.MIMEType}
		switch {
		case strings.HasPrefix(normalizeMIME(*file.MIMEType), "image/"):
			maxBytes = policy.ImageMaxBytes
		case strings.HasPrefix(normalizeMIME(*file.MIMEType), "audio/"):
			maxBytes = policy.AudioMaxBytes
		case strings.HasPrefix(normalizeMIME(*file.MIMEType), "video/"):
			maxBytes = policy.VideoMaxBytes
		}
	}
	if maxBytes <= 0 || policy.DownloadTimeoutSeconds <= 0 || policy.TransferTimeoutSeconds <= 0 {
		return Constraints{}, airun.ErrPolicyInvalid
	}
	deadline := now.Add(time.Duration(policy.TransferTimeoutSeconds) * time.Second)
	if file.TransferDeadline != nil {
		deadline = *file.TransferDeadline
	}
	if file.SourceExpiresAt != nil && file.SourceExpiresAt.Before(deadline) {
		deadline = *file.SourceExpiresAt
	}
	if !deadline.After(now) {
		return Constraints{}, errors.New("media transfer deadline expired")
	}
	return Constraints{MaxBytes: maxBytes, AllowedMIMEs: allowed, DownloadTimeout: time.Duration(policy.DownloadTimeoutSeconds) * time.Second, Deadline: deadline}, nil
}

func generatedName(file File) string {
	ext := "bin"
	if file.MIMEType != nil {
		switch normalizeMIME(*file.MIMEType) {
		case "image/png":
			ext = "png"
		case "image/jpeg":
			ext = "jpg"
		case "image/webp":
			ext = "webp"
		case "image/gif":
			ext = "gif"
		case "audio/mpeg":
			ext = "mp3"
		case "audio/wav":
			ext = "wav"
		case "audio/ogg":
			ext = "ogg"
		case "audio/mp4":
			ext = "m4a"
		case "video/mp4":
			ext = "mp4"
		case "video/webm":
			ext = "webm"
		}
	}
	return fmt.Sprintf("ai-%d.%s", file.ID, ext)
}

func importBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return 5 * time.Second
	}
	return 30 * time.Second
}

func backoff(attempts int) time.Duration {
	if attempts < 1 {
		return time.Minute
	}
	if attempts > 5 {
		attempts = 5
	}
	return time.Duration(1<<attempts) * time.Minute
}

func timePtr(v time.Time) *time.Time { return &v }

func deterministicUUID(seed string) string {
	// UUIDv5 is deterministic and has the same canonical shape as storage's
	// request_key contract without persisting worker-local state.
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("ai-media:"+seed)).String()
}

func (s *Service) cipherSeal(value string) (string, string, error) {
	sealer, ok := s.cipher.(interface {
		Seal(string) (string, string, error)
	})
	if !ok {
		return "", "", errors.New("AI media source cipher cannot seal")
	}
	return sealer.Seal(value)
}

func (s *Service) Retry(ctx context.Context, runID, userID, platformID int64) error {
	return s.repository.RetryRunImports(ctx, runID, userID, platformID, s.now().UTC())
}

func (s *Service) prepareInline(ctx context.Context, in InlineImport) (*File, error) {
	now := s.now().UTC()
	deadline := now.Add(in.TransferTimeout)
	key := deterministicUUID(fmt.Sprintf("%d:%d:%d:%s", in.RunID, in.ProviderID, in.Ordinal, in.RequestSeed))
	f := &File{RunID: in.RunID, Role: FileRoleGenerated, Ordinal: in.Ordinal, ImportRequestKey: &key, SourceKind: in.SourceKind, SourceRecoverable: 0, TransferDeadline: &deadline, MIMEType: &in.MIME, Metadata: json.RawMessage(`{}`), TransferStatus: Transfering, TransferAttempts: 1, NextTransferAt: &deadline, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateGeneratedFile(ctx, f); err != nil {
		return nil, err
	}

	return f, nil
}

func (s *Service) importHeartbeat(ctx context.Context, file File) (context.Context, context.CancelFunc) {
	work, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if err := s.repository.RenewImport(work, file.ID, file.TransferAttempts, s.now().UTC()); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return work, func() { cancel(); <-done }
}
