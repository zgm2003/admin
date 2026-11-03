package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/ai"
	"admin/server/internal/module/ai/usage"
	"admin/server/internal/shared/apperror"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createRepository interface {
	FindByRequest(context.Context, int64, int64, string) (Model, bool, error)
	Create(context.Context, CreateInput, RuntimeSnapshot, json.RawMessage, time.Time) (Model, bool, error)
}
type runPolicyReader interface {
	Read(context.Context) (Policy, error)
}
type admissionGate interface {
	Check(context.Context, int64, Policy) error
}
type Service struct {
	repository createRepository
	runtime    RuntimeReader
	policies   runPolicyReader
	gate       admissionGate
	files      inputFileValidator
	now        func() time.Time
}

type inputFileValidator interface {
	ValidateAIReferences(context.Context, int64, int64, []string, string) ([]InputFileSnapshot, error)
}

func NewService(repository createRepository, runtime RuntimeReader) *Service {
	return &Service{repository: repository, runtime: runtime, now: time.Now}
}
func (s *Service) SetAdmission(policies runPolicyReader, gate admissionGate) {
	s.policies = policies
	s.gate = gate
}

func (s *Service) SetFileValidator(files inputFileValidator) { s.files = files }

type RunInput struct {
	Text            string              `json:"text"`
	Quality         string              `json:"quality,omitempty"`
	Size            string              `json:"size,omitempty"`
	Count           int64               `json:"count,omitempty"`
	Seconds         string              `json:"seconds,omitempty"`
	Voice           string              `json:"voice,omitempty"`
	Format          string              `json:"format,omitempty"`
	FileIDs         []string            `json:"fileIds,omitempty"`
	FileSnapshots   []InputFileSnapshot `json:"fileSnapshots,omitempty"`
	MaxOutputTokens *int64              `json:"maxOutputTokens,omitempty"`
}

func decodeInput(raw json.RawMessage) (RunInput, error) {
	var value RunInput
	if len(raw) > 1024*1024 {
		return value, errors.New("AI input exceeds request limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&value); e != nil {
		return value, e
	}
	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		return value, errors.New("AI input contains trailing data")
	}
	return value, nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (Model, bool, error) {
	if e := ctx.Err(); e != nil {
		return Model{}, false, e
	}
	if s == nil || s.repository == nil {
		return Model{}, false, ErrRuntimeUnavailable
	}
	if in.UserID < 1 || in.PlatformID < 1 || in.AgentID < 1 || uuid.Validate(in.RequestKey) != nil || !ai.Operation(in.Operation).Valid() {
		return Model{}, false, apperror.InvalidRequest(errors.New("AI run request is invalid"))
	}
	if _, e := ai.DecodeJSONObject(in.Input); e != nil {
		return Model{}, false, apperror.InvalidRequest(e)
	}
	fingerprint, e := fingerprint(in)
	if e != nil {
		return Model{}, false, apperror.InvalidRequest(e)
	}
	// Replays precede all mutable configuration, policy and Redis gates. A provider
	// switch or outage must not change the identity or cost of an accepted Run.
	old, found, e := s.repository.FindByRequest(ctx, in.UserID, in.PlatformID, in.RequestKey)
	if e != nil {
		return Model{}, false, e
	}
	if found {
		if old.RequestFingerprint != fingerprint {
			return Model{}, false, ErrIdempotencyConflict
		}
		return old, false, nil
	}
	if in.Operation == string(ai.OperationVideo) {
		return Model{}, false, apperror.InvalidRequest(errors.New("video.generate is disabled"))
	}
	if s.runtime == nil || s.policies == nil || s.gate == nil {
		return Model{}, false, ErrRuntimeUnavailable
	}
	data, e := decodeInput(in.Input)
	if e != nil {
		return Model{}, false, apperror.InvalidRequest(e)
	}
	if len(data.FileSnapshots) > 0 {
		return Model{}, false, apperror.InvalidRequest(errors.New("AI file snapshots are server-owned"))
	}
	p, e := s.policies.Read(ctx)
	if e != nil {
		return Model{}, false, e
	}
	if e = p.Validate(); e != nil {
		return Model{}, false, e
	}
	if int64(len(data.Text)) > p.MaxTextInputBytes || (in.Operation != "audio.transcribe" && strings.TrimSpace(data.Text) == "") || int64(len(data.FileIDs)) > p.MaxInputAttachments {
		return Model{}, false, apperror.InvalidRequest(errors.New("AI input violates model policy"))
	}
	if len(data.FileIDs) > 0 {
		if s.files == nil {
			return Model{}, false, ErrRuntimeUnavailable
		}
		data.FileSnapshots, e = s.files.ValidateAIReferences(ctx, in.UserID, in.PlatformID, data.FileIDs, in.Operation)
		if e != nil {
			return Model{}, false, e
		}
		for _, file := range data.FileSnapshots {
			maxBytes := p.ImageMaxBytes
			if strings.HasPrefix(file.MIMEType, "audio/") {
				maxBytes = p.AudioMaxBytes
			}
			if file.SizeBytes < 1 || file.SizeBytes > maxBytes {
				return Model{}, false, apperror.InvalidRequest(errors.New("AI input file exceeds frozen policy"))
			}
		}
		in.Input, e = json.Marshal(data)
		if e != nil {
			return Model{}, false, e
		}
	}
	now := s.now().UTC()
	snap, e := s.runtime.Get(ctx, in.AgentID, now)
	if e != nil {
		return Model{}, false, e
	}
	if int64(len(snap.Tools)) > p.MaxBoundTools {
		return Model{}, false, ErrInvalidSnapshot
	}
	estimate, variable, timeout, e := estimateRun(snap, in.Operation, data, p)
	if e != nil {
		if errors.Is(e, ErrInvalidSnapshot) {
			return Model{}, false, e
		}
		return Model{}, false, apperror.InvalidRequest(e)
	}
	if e = s.gate.Check(ctx, in.UserID, p); e != nil {
		return Model{}, false, e
	}
	policy, e := json.Marshal(p)
	if e != nil {
		return Model{}, false, e
	}
	snap.Policy = policy
	in.EstimatedUnits = estimate
	in.VariableUsage = variable
	in.DeadlineAt = snap.AcceptedAt.Add(timeout)
	return s.repository.Create(ctx, in, snap, policy, now)
}
func (s *Service) FindByRequest(ctx context.Context, userID, platformID int64, key string) (Model, bool, error) {
	if s == nil || s.repository == nil || uuid.Validate(key) != nil {
		return Model{}, false, errors.New("AI request key is invalid")
	}
	return s.repository.FindByRequest(ctx, userID, platformID, key)
}

func (s *Service) Find(ctx context.Context, id, userID, platformID int64) (Model, error) {
	if s == nil || s.repository == nil || id < 1 || userID < 1 || platformID < 1 {
		return Model{}, errors.New("AI run identity is invalid")
	}
	r, ok := s.repository.(*Repository)
	if !ok || r == nil {
		return Model{}, ErrRuntimeUnavailable
	}
	m, err := r.Find(ctx, id)
	if err != nil {
		return Model{}, err
	}
	if m.UserID != userID || m.PlatformID != platformID {
		return Model{}, gorm.ErrRecordNotFound
	}
	return m, nil
}

func estimateRun(s RuntimeSnapshot, op string, in RunInput, p Policy) (int64, bool, time.Duration, error) {
	var config RuntimeConfig
	if e := json.Unmarshal(s.Config, &config); e != nil {
		return 0, false, 0, e
	}
	supported := false
	for _, v := range config.Operations {
		if v == op {
			supported = true
		}
	}
	if !supported {
		return 0, false, 0, errors.New("AI operation is not enabled on the frozen model")
	}
	capability := strings.Split(op, ".")[0]
	if capability == "audio" {
		capability = "audio"
	}
	allowed := false
	for _, v := range config.Capabilities {
		if v == capability {
			allowed = true
		}
	}
	if !allowed {
		return 0, false, 0, errors.New("AI operation is not enabled on the agent")
	}
	type modelLimits struct {
		MaxInputTokens  int64 `json:"maxInputTokens"`
		MaxOutputTokens int64 `json:"maxOutputTokens"`
		MaxImages       int64 `json:"maxImages"`
		MaxAudioSeconds int64 `json:"maxAudioSeconds"`
	}
	var limits modelLimits
	if e := json.Unmarshal(config.Limits, &limits); e != nil {
		return 0, false, 0, e
	}
	lines := make([]usage.LineInput, 0)
	timeout := p.TextTimeoutSeconds
	variable := false
	switch ai.Operation(op) {
	case ai.OperationText:
		variable = true
		if !ai.TextProtocol(s.Protocol).Valid() || limits.MaxInputTokens < 1 || limits.MaxOutputTokens < 1 {
			return 0, false, 0, ErrInvalidSnapshot
		}
		output := min(p.MaxOutputTokens, limits.MaxOutputTokens)
		if in.MaxOutputTokens != nil {
			if *in.MaxOutputTokens < 1 || *in.MaxOutputTokens > output {
				return 0, false, 0, errors.New("AI output token limit is invalid")
			}
			output = *in.MaxOutputTokens
		}
		// UTF-8 byte length is a conservative upper bound for ordinary text token
		// payloads. Tool rounds reserve the configured context ceiling instead of
		// pretending that future tool output is already known.
		inputBound := int64(len(s.Prompt) + len(in.Text))
		for _, t := range s.Tools {
			inputBound += int64(len(t.Config))
		}
		if inputBound > limits.MaxInputTokens {
			return 0, false, 0, errors.New("AI input exceeds model context limit")
		}
		// Durable conversation history can fill the complete configured context.
		inputBound = limits.MaxInputTokens
		if p.MaxModelRounds < 1 || inputBound > math.MaxInt64/p.MaxModelRounds || output > math.MaxInt64/p.MaxModelRounds {
			return 0, false, 0, ErrInvalidSnapshot
		}
		required := map[string]bool{"input": false, "output": false}
		for _, r := range config.Rates {
			if r.Operation != op {
				continue
			}
			selector, e := normalizeObject(r.Selector)
			if e != nil || string(selector) != "{}" {
				return 0, false, 0, errors.New("AI text pricing condition is not supported by this runner")
			}
			if r.Unit != "token" {
				return 0, false, 0, ErrInvalidSnapshot
			}
			quantity := int64(0)
			switch r.UsageKey {
			case "input":
				quantity = inputBound * p.MaxModelRounds
				required["input"] = true
			case "output":
				quantity = output * p.MaxModelRounds
				required["output"] = true
			case "cacheInput":
				continue
			default:
				return 0, false, 0, errors.New("AI text usage dimension is unsupported")
			}
			if quantity < 0 {
				return 0, false, 0, ErrInvalidSnapshot
			}
			lines = append(lines, usage.LineInput{ItemKey: strconv.FormatInt(r.ID, 10), PriceUnits: r.BillingUnitPriceUnits, Quantity: strconv.FormatInt(quantity, 10), UnitQuantity: r.UnitQuantity, MultiplierPPM: s.MultiplierPPM})
		}
		if !required["input"] || !required["output"] {
			return 0, false, 0, errors.New("AI text price is incomplete")
		}
	case ai.OperationImage, ai.OperationImageEdit:
		timeout = p.ImageTimeoutSeconds
		if (op == "image.edit" && len(in.FileIDs) != 1) || (op == "image.generate" && len(in.FileIDs) != 0) {
			return 0, false, 0, errors.New("AI image attachments are invalid")
		}
		if in.Count < 1 || in.Count > p.MaxImages || limits.MaxImages < 1 || in.Count > limits.MaxImages || in.Quality == "" || in.Size == "" {
			return 0, false, 0, errors.New("AI image options are invalid")
		}
		for _, r := range config.Rates {
			if r.Operation != op || r.Unit != "image" {
				continue
			}
			matched, err := mediaRateMatches(r, in)
			if err != nil {
				return 0, false, 0, err
			}
			if !matched {
				continue
			}
			lines = append(lines, usage.LineInput{ItemKey: strconv.FormatInt(r.ID, 10), PriceUnits: r.BillingUnitPriceUnits, Quantity: strconv.FormatInt(in.Count, 10), UnitQuantity: r.UnitQuantity, MultiplierPPM: s.MultiplierPPM})
		}
		if len(lines) != 1 {
			return 0, false, 0, errors.New("AI image price is unavailable for the selected quality and size")
		}
	case ai.OperationAudio, ai.OperationAudioTranscribe:
		timeout = p.AudioTimeoutSeconds
		if op == "audio.transcribe" && len(in.FileIDs) != 1 {
			return 0, false, 0, errors.New("AI transcription requires one audio attachment")
		}
		if op == "audio.speech" && (strings.TrimSpace(in.Text) == "" || len(in.FileIDs) != 0) {
			return 0, false, 0, errors.New("AI speech input is invalid")
		}
		seen := map[string]bool{}
		basis := ""
		for _, r := range config.Rates {
			if r.Operation != op {
				continue
			}
			matched, err := mediaRateMatches(r, in)
			if err != nil {
				return 0, false, 0, err
			}
			if !matched {
				continue
			}
			if seen[r.UsageKey] || (basis != "" && basis != r.Unit) {
				return 0, false, 0, ErrInvalidSnapshot
			}
			seen[r.UsageKey] = true
			basis = r.Unit
			var quantity int64
			switch r.Unit {
			case "character":
				if op != "audio.speech" || r.UsageKey != "input" {
					return 0, false, 0, ErrInvalidSnapshot
				}
				quantity = int64(utf8.RuneCountInString(in.Text))
			case "second":
				if op != "audio.transcribe" || (r.UsageKey != "audio" && r.UsageKey != "duration") || limits.MaxAudioSeconds < 1 {
					return 0, false, 0, ErrInvalidSnapshot
				}
				quantity = limits.MaxAudioSeconds
			case "token":
				if limits.MaxInputTokens < 1 || limits.MaxOutputTokens < 1 {
					return 0, false, 0, ErrInvalidSnapshot
				}
				switch r.UsageKey {
				case "input":
					quantity = limits.MaxInputTokens
				case "output":
					quantity = limits.MaxOutputTokens
				default:
					return 0, false, 0, ErrInvalidSnapshot
				}
				if op == "audio.speech" && int64(len(in.Text)) > limits.MaxInputTokens {
					return 0, false, 0, errors.New("AI speech input exceeds model limit")
				}
			default:
				return 0, false, 0, ErrInvalidSnapshot
			}
			if r.ID < 1 || quantity < 1 {
				return 0, false, 0, ErrInvalidSnapshot
			}
			lines = append(lines, usage.LineInput{ItemKey: strconv.FormatInt(r.ID, 10), PriceUnits: r.BillingUnitPriceUnits, Quantity: strconv.FormatInt(quantity, 10), UnitQuantity: r.UnitQuantity, MultiplierPPM: s.MultiplierPPM})
		}
		if (basis == "token" && (len(lines) != 2 || !seen["input"] || !seen["output"])) || (basis != "token" && len(lines) != 1) {
			return 0, false, 0, errors.New("AI audio price is incomplete")
		}
		variable = true
	default:
		return 0, false, 0, fmt.Errorf("AI operation %s has no complete admission estimator", op)
	}
	if timeout < 1 || len(lines) == 0 {
		return 0, false, 0, ErrInvalidSnapshot
	}
	charge, e := usage.Calculate(lines)
	return charge.TotalUnits, variable, time.Duration(timeout) * time.Second, e
}
