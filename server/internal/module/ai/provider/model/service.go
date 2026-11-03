package model

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	ai "admin/server/internal/module/ai"
	"admin/server/internal/shared/money"
)

type Service struct {
	repository Repository
	runtime    RuntimeCoordinator
}

func NewService(repository Repository, runtime ...RuntimeCoordinator) *Service {
	s := &Service{repository: repository}
	if len(runtime) > 0 {
		s.runtime = runtime[0]
	}
	return s
}
func (s *Service) PreviewSync(ctx context.Context, providerModelID int64, source SourceSnapshot, actorID int64) (*PriceVersion, error) {
	var out *PriceVersion
	err := s.mutateModel(ctx, providerModelID, func(repo Repository) error {
		var err error
		out, err = NewService(repo).previewSync(ctx, providerModelID, source, actorID)
		return err
	})
	return out, err
}
func (s *Service) previewSync(ctx context.Context, providerModelID int64, source SourceSnapshot, actorID int64) (*PriceVersion, error) {
	if providerModelID <= 0 || actorID <= 0 {
		return nil, errors.New("AI price identity is invalid")
	}
	if source.ExpectedProviderVersionID > 0 {
		versionID, err := s.repository.CurrentProviderVersionID(ctx, providerModelID)
		if err != nil {
			return nil, err
		}
		if versionID != source.ExpectedProviderVersionID {
			return nil, ErrSourceVersionChanged
		}
	}
	if len(source.Payload) > 1<<20 || len(source.Rates) == 0 || len(source.Rates) > 1000 {
		return nil, errors.New("AI source price snapshot size is invalid")
	}
	payload, err := ai.DecodeJSONObject(source.Payload)
	if err != nil {
		return nil, fmt.Errorf("source payload: %w", err)
	}
	if source.SyncedAt.IsZero() {
		return nil, errors.New("source digest and sync time are required")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, err
	}
	payload, _ = json.Marshal(object)
	sum := sha256.Sum256(payload)
	source.Digest = hex.EncodeToString(sum[:])
	latest, err := s.repository.Latest(ctx, providerModelID)
	if err != nil {
		return nil, err
	}
	rates := make([]PriceRate, 0, len(source.Rates))
	identities := map[string]struct{}{}
	for _, r := range source.Rates {
		normalized, hash, err := normalizeSelector(r.Selector)
		if err != nil {
			return nil, err
		}
		if !ai.Operation(r.Operation).Valid() || r.UsageKey == "" || len(r.UsageKey) > 64 || !validUnit(r.Unit) || !validDecimal(r.UnitQuantity, false) || !validDecimal(r.SourceUnitPrice, true) || !validUnitQuantity(r.Unit, r.UnitQuantity) {
			return nil, errors.New("invalid AI source price rate")
		}
		r.UnitQuantity = canonicalDecimal(r.UnitQuantity)
		r.SourceUnitPrice = canonicalDecimal(r.SourceUnitPrice)
		id := rateIdentity(r.Operation, r.UsageKey, hash, r.Unit, r.UnitQuantity)
		if _, ok := identities[id]; ok {
			return nil, errors.New("duplicate AI source price identity")
		}
		identities[id] = struct{}{}
		for _, existing := range rates {
			if selectorsOverlap(existing.Selector, normalized) && existing.Operation == r.Operation && existing.UsageKey == r.UsageKey {
				return nil, errors.New("overlapping AI source price selectors")
			}
		}
		var billing *int64
		if latestRate := findRate(latest, r.Operation, r.UsageKey, hash, r.Unit, r.UnitQuantity); latestRate != nil {
			if latestRate.BillingUnitPriceUnits != nil {
				v := *latestRate.BillingUnitPriceUnits
				billing = &v
			}
		} else {
			billing = initialBilling(r.SourceUnitPrice)
		}
		rates = append(rates, PriceRate{Operation: r.Operation, UsageKey: r.UsageKey, Selector: normalized, SelectorHash: hash, Unit: r.Unit, UnitQuantity: r.UnitQuantity, SourceUnitPrice: r.SourceUnitPrice, BillingUnitPriceUnits: billing})
	}
	if source.EffectiveAt == nil {
		now := time.Now().UTC()
		source.EffectiveAt = &now
	}
	stamp := source.EffectiveAt.UTC().Truncate(time.Microsecond)
	source.EffectiveAt = &stamp
	p := &PriceVersion{ProviderModelID: providerModelID, Version: nextVersion(latest), Status: 0, Currency: "CNY", SourceCurrency: "USD", SourceDigest: source.Digest, SourcePayload: payload, SyncedAt: source.SyncedAt, CreatedBy: actorID, Rates: rates, EffectiveAt: source.EffectiveAt}
	p.SnapshotDigest = versionDigest(p)
	if err := s.repository.CreatePreview(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// PreviewUpdate clones an existing version, preserving the USD source while changing only RMB prices.
func (s *Service) PreviewUpdate(ctx context.Context, providerModelID, versionID int64, req UpdatePriceRequest, actorID int64) (*PriceVersion, error) {
	var out *PriceVersion
	err := s.mutateModel(ctx, providerModelID, func(repo Repository) error {
		var err error
		out, err = NewService(repo).previewUpdate(ctx, providerModelID, versionID, req, actorID)
		return err
	})
	return out, err
}
func (s *Service) previewUpdate(ctx context.Context, providerModelID, versionID int64, req UpdatePriceRequest, actorID int64) (*PriceVersion, error) {
	if actorID <= 0 {
		return nil, errors.New("AI price identity is invalid")
	}
	base, err := s.repository.Find(ctx, providerModelID, versionID)
	if err != nil {
		return nil, err
	}
	if base == nil || len(base.Rates) == 0 {
		return nil, ErrPriceNotFound
	}
	latest, err := s.repository.Latest(ctx, providerModelID)
	if err != nil {
		return nil, err
	}
	clone := &PriceVersion{ProviderModelID: providerModelID, Version: nextVersion(latest), Status: PriceStatusPreview, Currency: base.Currency, SourceCurrency: base.SourceCurrency, SourceDigest: base.SourceDigest, SourcePayload: append([]byte(nil), base.SourcePayload...), SyncedAt: base.SyncedAt, CreatedBy: actorID, EffectiveAt: req.EffectiveAt}
	clone.Rates = make([]PriceRate, len(base.Rates))
	copy(clone.Rates, base.Rates)
	for i := range clone.Rates {
		if clone.Rates[i].BillingUnitPriceUnits != nil {
			v := *clone.Rates[i].BillingUnitPriceUnits
			clone.Rates[i].BillingUnitPriceUnits = &v
		}
	}
	byID := map[int64]*PriceRate{}
	for i := range clone.Rates {
		byID[clone.Rates[i].ID] = &clone.Rates[i]
	}
	for _, u := range req.Rates {
		r := byID[u.RateID]
		if r == nil {
			return nil, errors.New("AI price rate not found")
		}
		if u.BillingUnitPriceUnits == nil || *u.BillingUnitPriceUnits < 0 {
			return nil, errors.New("AI billing price is invalid")
		}
		v := *u.BillingUnitPriceUnits
		r.BillingUnitPriceUnits = &v
	}
	if clone.EffectiveAt == nil {
		now := time.Now().UTC()
		clone.EffectiveAt = &now
	}
	stamp := clone.EffectiveAt.UTC().Truncate(time.Microsecond)
	clone.EffectiveAt = &stamp
	clone.SnapshotDigest = versionDigest(clone)
	if err := s.repository.CreatePreview(ctx, clone); err != nil {
		return nil, err
	}
	return clone, nil
}
func (s *Service) SetBillingPrice(ctx context.Context, providerModelID, versionID int64, req UpdatePriceRequest, actorID int64) error {
	return s.mutateModel(ctx, providerModelID, func(repo Repository) error {
		return NewService(repo).setBillingPrice(ctx, providerModelID, versionID, req, actorID)
	})
}
func (s *Service) setBillingPrice(ctx context.Context, providerModelID, versionID int64, req UpdatePriceRequest, actorID int64) error {
	if actorID <= 0 || providerModelID <= 0 || versionID <= 0 {
		return errors.New("AI price identity is invalid")
	}
	p, err := s.repository.Find(ctx, providerModelID, versionID)
	if err != nil {
		return err
	}
	if p.Status != 0 {
		return ErrPublishedImmutable
	}
	byID := map[int64]*PriceRate{}
	for i := range p.Rates {
		byID[p.Rates[i].ID] = &p.Rates[i]
	}
	for _, u := range req.Rates {
		r := byID[u.RateID]
		if r == nil {
			return errors.New("AI price rate not found")
		}
		if u.BillingUnitPriceUnits == nil || *u.BillingUnitPriceUnits < 0 {
			return errors.New("AI billing price is invalid")
		}
		v := *u.BillingUnitPriceUnits
		r.BillingUnitPriceUnits = &v
	}
	if req.EffectiveAt != nil {
		stamp := req.EffectiveAt.UTC().Truncate(time.Microsecond)
		p.EffectiveAt = &stamp
	}
	p.SnapshotDigest = versionDigest(p)
	return s.repository.UpdatePreview(ctx, p)
}
func (s *Service) Publish(ctx context.Context, providerModelID, versionID int64, digest string, actorID int64) error {
	return s.mutateModel(ctx, providerModelID, func(repo Repository) error {
		return NewService(repo).publish(ctx, providerModelID, versionID, digest, actorID)
	})
}
func (s *Service) publish(ctx context.Context, providerModelID, versionID int64, digest string, actorID int64) error {
	if providerModelID <= 0 || versionID <= 0 || actorID <= 0 {
		return errors.New("AI price identity is invalid")
	}
	p, err := s.repository.Find(ctx, providerModelID, versionID)
	if err != nil {
		return err
	}
	if p.Status == PriceStatusPublished && p.SnapshotDigest == digest {
		return errNoRuntimeChange
	}
	if p.Status != 0 {
		return ErrPublishedImmutable
	}
	if digest == "" || digest != p.SnapshotDigest || digest != versionDigest(p) {
		return ErrPriceDigestMismatch
	}
	if len(p.Rates) == 0 || p.EffectiveAt == nil {
		return ErrIncompleteBillingPrice
	}
	for _, r := range p.Rates {
		if r.BillingUnitPriceUnits == nil {
			return ErrIncompleteBillingPrice
		}
	}
	return s.repository.Publish(ctx, providerModelID, versionID, digest, actorID)
}
func (s *Service) Effective(ctx context.Context, providerModelID int64, acceptedAt time.Time) (*PriceVersion, error) {
	return s.repository.Effective(ctx, providerModelID, acceptedAt)
}
func withModel(repo Repository, ctx context.Context, modelID int64, fn func(Repository) error) error {
	return repo.WithModel(ctx, modelID, fn)
}
func canonicalDecimal(v string) string {
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return v
	}
	return strings.TrimRight(strings.TrimRight(r.FloatString(12), "0"), ".")
}
func nextVersion(p *PriceVersion) int64 {
	if p == nil {
		return 1
	}
	return p.Version + 1
}
func findRate(p *PriceVersion, op, key, hash, unit, qty string) *PriceRate {
	if p == nil {
		return nil
	}
	for i := range p.Rates {
		r := &p.Rates[i]
		if rateIdentity(r.Operation, r.UsageKey, r.SelectorHash, r.Unit, r.UnitQuantity) == rateIdentity(op, key, hash, unit, qty) {
			return r
		}
	}
	return nil
}
func rateIdentity(op, key, hash, unit, qty string) string {
	return strings.Join([]string{op, key, hash, unit, canonicalDecimal(qty)}, "\x00")
}
func validUnit(v string) bool {
	switch v {
	case "token", "image", "second", "character", "request":
		return true
	}
	return false
}
func normalizeSelector(raw []byte) ([]byte, string, error) {
	v, err := ai.DecodeJSONObject(raw)
	if err != nil {
		return nil, "", err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(v, &obj); err != nil {
		return nil, "", err
	}
	if len(obj) > 16 {
		return nil, "", errors.New("AI selector has too many dimensions")
	}
	for key, value := range obj {
		if key == "" || len(key) > 64 || len(value) > 256 {
			return nil, "", errors.New("AI selector is invalid")
		}
		if len(value) == 0 || value[0] == '{' || value[0] == '[' || bytes.Equal(value, []byte("null")) {
			return nil, "", errors.New("AI selector must use scalar predicates")
		}
	}
	canonical := make(map[string]any, len(obj))
	for key, value := range obj {
		var scalar any
		dec := json.NewDecoder(bytes.NewReader(value))
		dec.UseNumber()
		if err := dec.Decode(&scalar); err != nil {
			return nil, "", err
		}
		if n, ok := scalar.(json.Number); ok {
			rat, ok := new(big.Rat).SetString(string(n))
			if !ok {
				return nil, "", errors.New("AI selector number invalid")
			}
			if !validDecimal(string(n), true) {
				return nil, "", errors.New("AI selector numeric precision invalid")
			}
			scalar = json.Number(canonicalDecimal(rat.FloatString(12)))
		}
		canonical[key] = scalar
	}
	canon, _ := json.Marshal(canonical)
	sum := sha256.Sum256(canon)
	return canon, hex.EncodeToString(sum[:]), nil
}
func versionDigest(p *PriceVersion) string {
	type d struct {
		ModelID     int64       `json:"modelId"`
		EffectiveAt *time.Time  `json:"effectiveAt"`
		Source      string      `json:"source"`
		Rates       []PriceRate `json:"rates"`
	}
	rs := append([]PriceRate(nil), p.Rates...)
	for i := range rs {
		rs[i].ID = 0
		rs[i].PriceVersionID = 0
		rs[i].UnitQuantity = canonicalDecimal(rs[i].UnitQuantity)
		rs[i].SourceUnitPrice = canonicalDecimal(rs[i].SourceUnitPrice)
	}
	sort.Slice(rs, func(i, j int) bool {
		return rateIdentity(rs[i].Operation, rs[i].UsageKey, rs[i].SelectorHash, rs[i].Unit, rs[i].UnitQuantity) < rateIdentity(rs[j].Operation, rs[j].UsageKey, rs[j].SelectorHash, rs[j].Unit, rs[j].UnitQuantity)
	})
	var effective *time.Time
	if p.EffectiveAt != nil {
		stamp := p.EffectiveAt.UTC().Truncate(time.Microsecond)
		effective = &stamp
	}
	b, _ := json.Marshal(d{p.ProviderModelID, effective, p.SourceDigest, rs})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func validDecimal(v string, allowZero bool) bool {
	if v == "" || strings.TrimSpace(v) != v || strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-") || strings.Count(v, ".") > 1 {
		return false
	}
	parts := strings.Split(v, ".")
	if parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return false
	}
	maxPlaces := 12
	maxInteger := 18
	if !allowZero {
		maxPlaces = 8
		maxInteger = 16
	}
	if len(strings.TrimLeft(parts[0], "0")) > maxInteger {
		return false
	}
	if len(parts) == 2 && len(parts[1]) > maxPlaces {
		return false
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return false
	}
	return (allowZero && r.Sign() >= 0) || (!allowZero && r.Sign() > 0)
}
func validUnitQuantity(unit, quantity string) bool {
	switch unit {
	case "token":
		return canonicalDecimal(quantity) == "1000000"
	case "image":
		return canonicalDecimal(quantity) == "1"
	}
	return true
}
func selectorsOverlap(a, b []byte) bool {
	var x, y map[string]any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	for key, value := range x {
		if other, ok := y[key]; ok {
			a, _ := json.Marshal(value)
			b, _ := json.Marshal(other)
			if !bytes.Equal(a, b) {
				return false
			}
		}
	}
	return true
}
func initialBilling(v string) *int64 {
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return nil
	}
	r.Mul(r, new(big.Rat).SetInt64(money.UnitsPerRMB))
	if r.Denom().Cmp(big.NewInt(1)) != 0 || !r.Num().IsInt64() {
		return nil
	}
	n := r.Num().Int64()
	if n < 0 {
		return nil
	}
	return &n
}
