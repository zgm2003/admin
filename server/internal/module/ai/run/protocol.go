package run

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
)

var (
	ErrAdmissionDenied = errors.New("AI run admission denied")
	ErrUnknownInflight = errors.New("AI run has unknown inflight usage")
	ErrInvalidSnapshot = errors.New("AI runtime snapshot is invalid")
)

type AdmissionInput struct {
	BalanceUnits           int64
	EstimatedUnits         int64
	InflightEstimatedUnits int64
	InflightCount          int
	InflightLimit          int
	UnknownInflight        bool
	VariableUsage          bool
	Now                    time.Time
}

type AdmissionDecision struct {
	Allowed bool
	Reason  string
}

// Admit implements the short, wallet-locked admission rule. It intentionally
// does not mutate a balance; callers perform that work in their transaction.
func Admit(in AdmissionInput) (AdmissionDecision, error) {
	if in.Now.IsZero() || in.BalanceUnits <= 0 || in.EstimatedUnits < 0 || in.InflightEstimatedUnits < 0 || in.InflightCount < 0 || in.InflightLimit < 1 {
		return AdmissionDecision{}, ErrAdmissionDenied
	}
	if in.InflightCount >= in.InflightLimit {
		return AdmissionDecision{}, fmt.Errorf("%w: inflight limit", ErrAdmissionDenied)
	}
	if in.UnknownInflight {
		return AdmissionDecision{}, ErrUnknownInflight
	}
	if in.InflightEstimatedUnits > math.MaxInt64-in.EstimatedUnits {
		return AdmissionDecision{}, ErrAdmissionDenied
	}
	if !in.VariableUsage || in.InflightCount > 0 {
		if in.BalanceUnits < in.InflightEstimatedUnits+in.EstimatedUnits {
			return AdmissionDecision{}, fmt.Errorf("%w: estimated balance is insufficient", ErrAdmissionDenied)
		}
	}
	return AdmissionDecision{Allowed: true}, nil
}

type ToolSnapshot struct {
	ToolID    int64           `json:"toolId"`
	VersionID int64           `json:"versionId"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
}

// RuntimeSnapshot is the immutable configuration handed to a Run. It has no
// mutable database model pointers and is safe to serialize into ai_run.
type RuntimeSnapshot struct {
	AgentID           int64           `json:"agentId"`
	BindingID         int64           `json:"bindingId"`
	ProviderModelID   int64           `json:"providerModelId"`
	ProviderVersionID int64           `json:"providerVersionId"`
	OfficialModelID   int64           `json:"officialModelId"`
	PriceVersionID    int64           `json:"priceVersionId"`
	Protocol          int16           `json:"protocol"`
	MultiplierPPM     int64           `json:"multiplierPpm"`
	Prompt            string          `json:"prompt"`
	Config            json.RawMessage `json:"config"`
	Policy            json.RawMessage `json:"policy"`
	Tools             []ToolSnapshot  `json:"tools,omitempty"`
	AcceptedAt        time.Time       `json:"acceptedAt"`
}

func NewRuntimeSnapshot(s RuntimeSnapshot) (RuntimeSnapshot, error) {
	if s.AgentID < 1 || s.BindingID < 1 || s.ProviderModelID < 1 || s.ProviderVersionID < 1 || s.OfficialModelID < 1 || s.PriceVersionID < 1 || s.MultiplierPPM < 1 || s.MultiplierPPM > 100000000 || s.AcceptedAt.IsZero() {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	var err error
	s.Config, err = normalizeObject(s.Config)
	if err != nil {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	s.Policy, err = normalizeObject(s.Policy)
	if err != nil {
		return RuntimeSnapshot{}, ErrInvalidSnapshot
	}
	s.Tools = append([]ToolSnapshot(nil), s.Tools...)
	seenIDs := make(map[int64]bool, len(s.Tools))
	seenNames := make(map[string]bool, len(s.Tools))
	for i := range s.Tools {
		if s.Tools[i].ToolID < 1 || s.Tools[i].VersionID < 1 || strings.TrimSpace(s.Tools[i].Name) == "" {
			return RuntimeSnapshot{}, ErrInvalidSnapshot
		}
		if seenIDs[s.Tools[i].ToolID] || seenNames[s.Tools[i].Name] {
			return RuntimeSnapshot{}, ErrInvalidSnapshot
		}
		seenIDs[s.Tools[i].ToolID], seenNames[s.Tools[i].Name] = true, true
		s.Tools[i].Config, err = normalizeObject(s.Tools[i].Config)
		if err != nil {
			return RuntimeSnapshot{}, ErrInvalidSnapshot
		}
	}
	s.AcceptedAt = s.AcceptedAt.UTC()
	sort.Slice(s.Tools, func(i, j int) bool { return s.Tools[i].ToolID < s.Tools[j].ToolID })
	return s, nil
}

func (s RuntimeSnapshot) Digest() (string, error) {
	n, err := NewRuntimeSnapshot(s)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(n)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func normalizeObject(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, ErrInvalidSnapshot
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil || value == nil {
		return nil, ErrInvalidSnapshot
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidSnapshot
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return b, nil
}
