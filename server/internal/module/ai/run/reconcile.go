package run

import (
	"admin/server/internal/module/ai"
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/usage"
	"admin/server/internal/module/ai/usage/charge"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxReconcileEvidenceBytes = 1 << 20

var ErrReconcileEvidence = errors.New("AI reconciliation evidence is invalid or incomplete")
var ErrReconcileConflict = errors.New("AI reconciliation conflicts with durable identity or accounting facts")

type ReconcileEvidence struct {
	SchemaVersion   int                        `json:"schemaVersion"`
	RunID           int64                      `json:"runId,string"`
	Operation       string                     `json:"operation"`
	ProviderModelID int64                      `json:"providerModelId,string"`
	PriceVersionID  int64                      `json:"priceVersionId,string"`
	SnapshotDigest  string                     `json:"snapshotDigest"`
	Requests        []ReconcileRequestEvidence `json:"requests"`
}
type ReconcileRequestEvidence struct {
	RequestID          int64           `json:"requestId,string"`
	RequestKey         string          `json:"requestKey"`
	RequestFingerprint string          `json:"requestFingerprint"`
	UpstreamRequestID  string          `json:"upstreamRequestId"`
	SourceReference    string          `json:"sourceReference"`
	ObservedAt         time.Time       `json:"observedAt"`
	Status             int16           `json:"status"`
	Usage              json.RawMessage `json:"usage"`
}
type ReconcileAudit struct {
	EvidenceDigest    string    `json:"evidenceDigest"`
	Operator          string    `json:"operator"`
	SourceReference   string    `json:"sourceReference"`
	UpstreamRequestID string    `json:"upstreamRequestId"`
	ObservedAt        time.Time `json:"observedAt"`
	AppliedAt         time.Time `json:"appliedAt"`
}
type ReconcileState struct {
	Run           Model
	Charge        charge.Model
	Requests      []pr.Fact
	UnknownTools  bool
	ExistingItems bool
}
type ReconcileRequestUpdate struct {
	ID     int64
	Status int16
	Usage  json.RawMessage
	Audit  ReconcileAudit
}
type ReconcilePlan struct {
	Complete    bool
	ActualUnits *int64
	Requests    []ReconcileRequestUpdate
	Items       []chargeItem
	Report      ReconcileReport
}
type ReconcileReport struct {
	RunID              string `json:"runId"`
	ChargeID           string `json:"chargeId"`
	EvidenceDigest     string `json:"evidenceDigest"`
	DryRun             bool   `json:"dryRun"`
	AlreadyApplied     bool   `json:"alreadyApplied"`
	Complete           bool   `json:"complete"`
	UpdatedRequests    int    `json:"updatedRequests"`
	ActualUnits        *int64 `json:"actualUnits"`
	UnknownToolEffects bool   `json:"unknownToolEffects"`
	SettlementQueued   bool   `json:"settlementQueued"`
}
type reconciliationRepository interface {
	Review(context.Context, int64, bool, func(ReconcileState) (ReconcilePlan, error)) (ReconcilePlan, error)
}
type ReconcileService struct {
	repository reconciliationRepository
	now        func() time.Time
}

func NewReconcileService(r reconciliationRepository) *ReconcileService {
	return &ReconcileService{r, time.Now}
}
func frozenReconcileDigest(config, policy []byte) string {
	data := append(append([]byte{}, bytes.TrimSpace(config)...), 0)
	data = append(data, bytes.TrimSpace(policy)...)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func uniqueEvidenceJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 32 {
			return false
		}
		token, e := d.Token()
		if e != nil {
			return false
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					token, e = d.Token()
					key, ok := token.(string)
					if e != nil || !ok || seen[key] {
						return false
					}
					seen[key] = true
					if !value(depth + 1) {
						return false
					}
				}
				end, e := d.Token()
				return e == nil && end == json.Delim('}')
			case '[':
				for d.More() {
					if !value(depth + 1) {
						return false
					}
				}
				end, e := d.Token()
				return e == nil && end == json.Delim(']')
			default:
				return false
			}
		}
		return true
	}
	if !value(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func strictReconcileJSON(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return ErrReconcileEvidence
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return ErrReconcileEvidence
	}
	return nil
}
func DecodeReconcileEvidence(raw []byte) (ReconcileEvidence, error) {
	var e ReconcileEvidence
	trim := bytes.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > MaxReconcileEvidenceBytes || len(trim) == 0 || trim[0] != '{' || !uniqueEvidenceJSON(raw) || strictReconcileJSON(raw, &e) != nil || e.SchemaVersion != 1 || e.RunID < 1 || e.ProviderModelID < 1 || e.PriceVersionID < 1 || !ai.Operation(e.Operation).Valid() || !validReconcileDigest(e.SnapshotDigest) || len(e.Requests) < 1 || len(e.Requests) > 64 {
		return e, ErrReconcileEvidence
	}
	return e, nil
}
func (s *ReconcileService) Execute(ctx context.Context, runID int64, raw []byte, operator string, apply bool) (ReconcileReport, error) {
	if s == nil || s.repository == nil || runID < 1 {
		return ReconcileReport{}, ErrReconcileEvidence
	}
	operator = strings.TrimSpace(operator)
	if len(operator) < 1 || len(operator) > 128 || strings.ContainsAny(operator, "\r\n\t") {
		return ReconcileReport{}, ErrReconcileEvidence
	}
	e, err := DecodeReconcileEvidence(raw)
	if err != nil || e.RunID != runID {
		return ReconcileReport{}, ErrReconcileEvidence
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	p, err := s.repository.Review(ctx, runID, apply, func(state ReconcileState) (ReconcilePlan, error) {
		return planReconciliation(state, e, operator, digest, s.now().UTC())
	})
	if err != nil {
		return ReconcileReport{}, err
	}
	p.Report.DryRun = !apply
	p.Report.SettlementQueued = apply && p.Complete && !p.Report.AlreadyApplied
	return p.Report, nil
}
func decodeConfirmedUsage(operation string, raw json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, ErrReconcileEvidence
	}
	required := []string{"known"}
	if operation == "text.generate" {
		required = append(required, "inputTokens", "cachedInputTokens", "outputTokens")
	}
	if operation == "image.generate" || operation == "image.edit" {
		required = append(required, "imageCount")
	}
	for _, key := range required {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrReconcileEvidence
		}
	}
	if operation == "text.generate" {
		var u pr.TokenUsage
		if strictReconcileJSON(raw, &u) != nil || !u.Known || u.InputTokens < 0 || u.CachedInputTokens < 0 || u.CachedInputTokens > u.InputTokens || u.OutputTokens < 0 {
			return nil, ErrReconcileEvidence
		}
		u.Raw = nil
		return json.Marshal(u)
	}
	var u pr.MediaUsage
	if strictReconcileJSON(raw, &u) != nil || !u.Known {
		return nil, ErrReconcileEvidence
	}
	u.Raw = nil
	return json.Marshal(u)
}
func planReconciliation(state ReconcileState, e ReconcileEvidence, operator, digest string, now time.Time) (ReconcilePlan, error) {
	p := ReconcilePlan{Requests: make([]ReconcileRequestUpdate, 0), Items: make([]chargeItem, 0)}
	row, ch := state.Run, state.Charge
	p.Report = ReconcileReport{RunID: strconv.FormatInt(row.ID, 10), ChargeID: strconv.FormatInt(ch.ID, 10), EvidenceDigest: digest, UnknownToolEffects: state.UnknownTools}
	if e.SchemaVersion != 1 || e.RunID != row.ID || e.Operation != row.Operation || e.ProviderModelID != row.ProviderModelID || e.PriceVersionID != row.PriceVersionID || row.ExecutionStatus < 3 || row.ExecutionStatus > 5 || ch.RunID != row.ID || ch.UserID != row.UserID || ch.PlatformID != row.PlatformID || ch.PriceVersionID != row.PriceVersionID || ch.MultiplierPPM != row.MultiplierPPM || ch.Currency != "CNY" || frozenReconcileDigest(row.ConfigSnapshot, row.PolicySnapshot) != e.SnapshotDigest || ch.SnapshotDigest != e.SnapshotDigest {
		return p, ErrReconcileConflict
	}
	var snap RuntimeSnapshot
	if json.Unmarshal(row.ConfigSnapshot, &snap) != nil || snap.AgentID != row.AgentID || snap.BindingID != row.BindingID || snap.ProviderModelID != row.ProviderModelID || snap.ProviderVersionID != row.ProviderVersionID || snap.OfficialModelID != row.OfficialModelID || snap.PriceVersionID != row.PriceVersionID || snap.MultiplierPPM != row.MultiplierPPM {
		return p, ErrReconcileConflict
	}
	if _, err := NewRuntimeSnapshot(snap); err != nil {
		return p, ErrInvalidSnapshot
	}
	var policy Policy
	if json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil || int64(len(state.Requests)) > policy.MaxModelRounds || len(state.Requests) == 0 {
		return p, ErrInvalidSnapshot
	}
	requests := append([]pr.Fact(nil), state.Requests...)
	seen := map[int64]bool{}
	for _, proof := range e.Requests {
		if proof.RequestID < 1 || seen[proof.RequestID] || proof.RequestKey == "" || proof.RequestFingerprint == "" || proof.UpstreamRequestID == "" || len(proof.UpstreamRequestID) > 200 || strings.ContainsAny(proof.UpstreamRequestID, "\r\n") || !validReconcileReference(proof.SourceReference) || proof.ObservedAt.IsZero() || proof.ObservedAt.After(now.Add(5*time.Minute)) || (proof.Status != 2 && proof.Status != 3) {
			return p, ErrReconcileEvidence
		}
		seen[proof.RequestID] = true
		index := -1
		for i, request := range requests {
			if request.ID == proof.RequestID {
				index = i
				break
			}
		}
		if index < 0 {
			return p, ErrReconcileConflict
		}
		request := requests[index]
		if request.RunID != row.ID || request.RequestKey != proof.RequestKey || request.RequestFingerprint != proof.RequestFingerprint || request.UpstreamRequestID != nil && *request.UpstreamRequestID != proof.UpstreamRequestID || !request.CreatedAt.IsZero() && proof.ObservedAt.Before(request.CreatedAt) {
			return p, ErrReconcileConflict
		}
		usageValue, err := decodeConfirmedUsage(row.Operation, proof.Usage)
		if err != nil {
			return p, err
		}
		if request.Status != 1 && request.Status != 4 && request.UsageStatus == 1 {
			var summary struct {
				Reconciliation *ReconcileAudit `json:"reconciliation"`
			}
			if json.Unmarshal(request.RequestSummary, &summary) != nil || summary.Reconciliation == nil || summary.Reconciliation.EvidenceDigest != digest || !sameConfirmedUsage(row.Operation, request.Usage, usageValue) {
				return p, ErrReconcileConflict
			}
			continue
		}
		audit := ReconcileAudit{EvidenceDigest: digest, Operator: operator, SourceReference: proof.SourceReference, UpstreamRequestID: proof.UpstreamRequestID, ObservedAt: proof.ObservedAt.UTC(), AppliedAt: now}
		p.Requests = append(p.Requests, ReconcileRequestUpdate{request.ID, proof.Status, usageValue, audit})
		requests[index].Usage = usageValue
		requests[index].UsageStatus = 1
		requests[index].Status = proof.Status
	}
	if ch.Status != 4 {
		if len(p.Requests) > 0 || ch.ActualUnits == nil || ch.UsageComplete != 1 {
			return p, ErrReconcileConflict
		}
		p.Complete = true
		p.ActualUnits = ch.ActualUnits
		p.Report.Complete = true
		p.Report.ActualUnits = ch.ActualUnits
		p.Report.AlreadyApplied = true
		return p, nil
	}
	if row.ReservationStatus != 1 || state.ExistingItems || ch.ActualUnits != nil || ch.UsageComplete != 0 {
		return p, ErrReconcileConflict
	}
	known := !state.UnknownTools
	lines := make([]ChargeLine, 0)
	var input RunInput
	if row.Operation != "text.generate" && json.Unmarshal(row.InputSnapshot, &input) != nil {
		return p, ErrInvalidSnapshot
	}
	for _, request := range requests {
		if request.Status == 1 || request.Status == 4 || request.UsageStatus != 1 {
			known = false
			continue
		}
		var part []ChargeLine
		var err error
		if row.Operation == "text.generate" {
			var u pr.TokenUsage
			if json.Unmarshal(request.Usage, &u) != nil {
				return p, ErrInvalidSnapshot
			}
			part, err = textUsageLines(snap, request.ID, u)
		} else {
			var u pr.MediaUsage
			if json.Unmarshal(request.Usage, &u) != nil {
				return p, ErrInvalidSnapshot
			}
			part, err = mediaUsageLines(snap, request.ID, u, row.Operation, input)
		}
		if err != nil {
			return p, err
		}
		lines = append(lines, part...)
	}
	p.Complete = known
	p.Report.Complete = known
	p.Report.UpdatedRequests = len(p.Requests)
	p.Report.AlreadyApplied = len(p.Requests) == 0 && !known
	if !known {
		return p, nil
	}
	calc := make([]usage.LineInput, 0, len(lines))
	for _, line := range lines {
		calc = append(calc, line.Calculation)
	}
	var calculated usage.CalculatedCharge
	if len(calc) > 0 {
		var err error
		calculated, err = usage.Calculate(calc)
		if err != nil {
			return p, err
		}
	}
	total := calculated.TotalUnits
	p.ActualUnits = &total
	p.Report.ActualUnits = &total
	amounts := map[string]int64{}
	for _, item := range calculated.Items {
		amounts[item.ItemKey] = item.AmountUnits
	}
	for _, line := range lines {
		p.Items = append(p.Items, chargeItem{ChargeID: ch.ID, ProviderRequestID: line.RequestID, PriceRateID: line.Rate.ID, ItemKey: line.Calculation.ItemKey, Operation: line.Rate.Operation, UsageKey: line.Rate.UsageKey, SelectorSnapshot: line.Rate.Selector, Quantity: line.Calculation.Quantity, UnitQuantity: line.Calculation.UnitQuantity, Unit: line.Rate.Unit, BillingUnitPriceUnits: line.Rate.BillingUnitPriceUnits, MultiplierPPM: snap.MultiplierPPM, AmountUnits: amounts[line.Calculation.ItemKey], RawUsageDigest: line.RawUsageDigest, CreatedAt: now, UpdatedAt: now})
	}
	return p, nil
}

func sameConfirmedUsage(operation string, existing, proposed json.RawMessage) bool {
	canonical, e := decodeConfirmedUsage(operation, existing)
	return e == nil && bytes.Equal(canonical, proposed)
}

func validReconcileDigest(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, e := hex.DecodeString(v)
	return e == nil
}
func validReconcileReference(v string) bool {
	if strings.TrimSpace(v) == "" || len(v) > 1024 || strings.ContainsAny(v, "\r\n") {
		return false
	}
	if strings.Contains(v, "://") {
		u, e := url.Parse(v)
		return e == nil && u.User == nil && u.RawQuery == "" && u.Fragment == ""
	}
	return true
}
