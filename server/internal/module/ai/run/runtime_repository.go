package run

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	price "admin/server/internal/module/ai/provider/model"
	"gorm.io/gorm"
)

type RuntimeRate struct {
	ID                    int64           `json:"id"`
	Operation             string          `json:"operation"`
	UsageKey              string          `json:"usageKey"`
	Selector              json.RawMessage `json:"selector"`
	Unit                  string          `json:"unit"`
	UnitQuantity          string          `json:"unitQuantity"`
	BillingUnitPriceUnits int64           `json:"billingUnitPriceUnits"`
}
type RuntimeConfig struct {
	ProviderID       int64           `json:"providerId"`
	UpstreamModelID  string          `json:"upstreamModelId"`
	Operations       []string        `json:"operations"`
	Capabilities     []string        `json:"capabilities"`
	Parameters       json.RawMessage `json:"parameters"`
	GenerationParams json.RawMessage `json:"generationParams"`
	Limits           json.RawMessage `json:"limits"`
	PriceDigest      string          `json:"priceDigest"`
	Rates            []RuntimeRate   `json:"rates"`
}
type RuntimeRepository struct{ db *gorm.DB }

func NewRuntimeRepository(db *gorm.DB) *RuntimeRepository { return &RuntimeRepository{db: db} }

func (r *RuntimeRepository) ValidateBinding(ctx context.Context, modelID int64, protocol int16, capabilities []string) error {
	if r == nil || r.db == nil || modelID < 1 || protocol < 1 || protocol > 2 || len(capabilities) == 0 {
		return ErrInvalidSnapshot
	}
	var row struct {
		ID         int64
		Operations json.RawMessage
		Limits     json.RawMessage
	}
	res := r.db.WithContext(ctx).Raw(`SELECT m.id,to_json(m.operations) AS operations,m.limits FROM ai_provider_model m JOIN ai_provider p ON p.id=m.provider_id AND p.status=1 AND p.deleted_at IS NULL JOIN ai_official_model o ON o.id=m.official_model_id AND o.status=1 WHERE m.id=? AND m.status=1 AND m.deleted_at IS NULL`, modelID).Scan(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrInvalidSnapshot
	}
	var operations []string
	if err := json.Unmarshal(row.Operations, &operations); err != nil {
		return err
	}
	version, err := price.NewRepository(r.db).Effective(ctx, modelID, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, capability := range capabilities {
		covered := false
		for _, op := range operations {
			if strings.HasPrefix(op, capability+".") {
				for _, rate := range version.Rates {
					if rate.Operation == op && rate.BillingUnitPriceUnits != nil {
						covered = true
					}
				}
			}
		}
		if !covered {
			return ErrInvalidSnapshot
		}
	}
	return nil
}
func (r *RuntimeRepository) LoadRuntime(ctx context.Context, agentID int64, at time.Time) (RuntimeSnapshot, *time.Time, error) {
	var snap RuntimeSnapshot
	var next *time.Time
	if r == nil || r.db == nil || agentID < 1 || at.IsZero() {
		return snap, nil, ErrRuntimeUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			AgentID, BindingID, ProviderID, ProviderModelID, ProviderVersionID, OfficialModelID, MultiplierPPM int64
			Protocol                                                                                           int16
			Prompt, UpstreamModelID                                                                            string
			Operations, Capabilities, Parameters, GenerationParams, Limits                                     json.RawMessage
		}
		res := tx.Raw(`SELECT a.id AS agent_id,b.id AS binding_id,p.id AS provider_id,m.id AS provider_model_id,v.id AS provider_version_id,o.id AS official_model_id,a.multiplier_ppm,b.text_protocol AS protocol,a.system_prompt AS prompt,m.upstream_model_id,to_json(m.operations) AS operations,to_json(a.capabilities) AS capabilities,b.parameters,a.generation_params,m.limits
   FROM ai_agent a JOIN ai_agent_model_binding b ON b.agent_id=a.id AND b.id=a.active_binding_id AND b.status=1
   JOIN ai_provider_model m ON m.id=b.provider_model_id AND m.status=1 AND m.deleted_at IS NULL
   JOIN ai_provider p ON p.id=m.provider_id AND p.status=1 AND p.deleted_at IS NULL
   JOIN ai_provider_version v ON v.provider_id=p.id AND v.version=p.current_version
   JOIN ai_official_model o ON o.id=m.official_model_id AND o.status=1
   WHERE a.id=? AND a.status=1 AND a.deleted_at IS NULL`, agentID).Scan(&row)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		config := RuntimeConfig{ProviderID: row.ProviderID, UpstreamModelID: row.UpstreamModelID, Parameters: row.Parameters, GenerationParams: row.GenerationParams, Limits: row.Limits}
		if err := json.Unmarshal(row.Operations, &config.Operations); err != nil {
			return err
		}
		if err := json.Unmarshal(row.Capabilities, &config.Capabilities); err != nil {
			return err
		}
		p, err := price.NewRepository(tx).Effective(ctx, row.ProviderModelID, at)
		if err != nil {
			return err
		}
		if p == nil || p.Status != 1 || p.EffectiveAt == nil || p.EffectiveAt.After(at) || p.Currency != "CNY" {
			return ErrInvalidSnapshot
		}
		config.PriceDigest = p.SnapshotDigest
		for _, v := range p.Rates {
			if v.BillingUnitPriceUnits == nil {
				return ErrInvalidSnapshot
			}
			config.Rates = append(config.Rates, RuntimeRate{ID: v.ID, Operation: v.Operation, UsageKey: v.UsageKey, Selector: v.Selector, Unit: v.Unit, UnitQuantity: v.UnitQuantity, BillingUnitPriceUnits: *v.BillingUnitPriceUnits})
		}
		var future struct{ At *time.Time }
		if err := tx.Raw(`SELECT min(effective_at) AS at FROM ai_provider_model_price_version WHERE provider_model_id=? AND status=1 AND effective_at>?`, row.ProviderModelID, at).Scan(&future).Error; err != nil {
			return err
		}
		next = future.At
		snap = RuntimeSnapshot{AgentID: row.AgentID, BindingID: row.BindingID, ProviderModelID: row.ProviderModelID, ProviderVersionID: row.ProviderVersionID, OfficialModelID: row.OfficialModelID, PriceVersionID: p.ID, Protocol: row.Protocol, MultiplierPPM: row.MultiplierPPM, Prompt: row.Prompt, Policy: json.RawMessage(`{}`), AcceptedAt: at.UTC()}
		snap.Config, err = json.Marshal(config)
		if err != nil {
			return err
		}
		var tools []struct {
			ToolID, VersionID int64
			Name              string
			Config            json.RawMessage
		}
		if err := tx.Raw(`SELECT t.id AS tool_id,v.id AS version_id,b.exposed_name AS name,jsonb_build_object('description',v.description,'inputSchema',v.input_schema,'outputSchema',v.output_schema,'constraints',b.parameter_constraints,'timeoutMs',v.timeout_ms,'outputLimitBytes',v.output_limit_bytes,'hasSideEffect',v.has_side_effect,'idempotencyMode',v.idempotency_mode) AS config
   FROM ai_agent_tool b JOIN ai_tool t ON t.id=b.tool_id AND t.status=1 AND t.deleted_at IS NULL JOIN ai_tool_version v ON v.tool_id=t.id AND v.version=t.current_version WHERE b.agent_id=? AND b.status=1 ORDER BY t.id LIMIT 17`, agentID).Scan(&tools).Error; err != nil {
			return err
		}
		if len(tools) > 16 {
			return errors.New("AI bound tool limit exceeded")
		}
		for _, v := range tools {
			snap.Tools = append(snap.Tools, ToolSnapshot{ToolID: v.ToolID, VersionID: v.VersionID, Name: v.Name, Config: v.Config})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return RuntimeSnapshot{}, nil, err
	}
	snap, err = NewRuntimeSnapshot(snap)
	return snap, next, err
}
