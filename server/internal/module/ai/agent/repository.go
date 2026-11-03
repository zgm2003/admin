package agent

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) })
}
func (r *Repository) Lock(ctx context.Context, id int64) (Model, error) {
	var m Model
	e := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, id).Error
	return m, e
}
func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	var m Model
	e := r.db.WithContext(ctx).First(&m, id).Error
	return m, e
}
func (r *Repository) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	var rows []Model
	var n int64
	q := r.db.WithContext(ctx).Model(&Model{})
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := q.Order("updated_at DESC,id DESC").Limit(size).Offset((page - 1) * size).Find(&rows).Error
	return rows, n, e
}
func (r *Repository) Bindings(ctx context.Context, id int64) ([]Binding, error) {
	rows := make([]Binding, 0)
	e := r.db.WithContext(ctx).Where("agent_id=?", id).Order("id").Find(&rows).Error
	return rows, e
}
func (r *Repository) ToolBindings(ctx context.Context, id int64) ([]ToolBinding, error) {
	rows := make([]ToolBinding, 0)
	e := r.db.WithContext(ctx).Where("agent_id=?", id).Order("id").Find(&rows).Error
	return rows, e
}
func (r *Repository) Create(ctx context.Context, in CreateInput) (Model, error) {
	m := Model{Code: in.Code, Name: in.Name, Description: in.Description, AvatarFileID: in.AvatarFileID, Capabilities: StringArray(in.Capabilities), SystemPrompt: in.SystemPrompt, MultiplierPPM: in.MultiplierPPM, GenerationParams: in.GenerationParams, Status: 0, CreatedBy: in.ActorID, UpdatedBy: in.ActorID}
	e := r.db.WithContext(ctx).Create(&m).Error
	return m, e
}
func (r *Repository) SaveConfiguration(ctx context.Context, m Model, in UpdateInput) error {
	return r.db.WithContext(ctx).Model(&m).Updates(map[string]any{"name": in.Name, "description": in.Description, "avatar_file_id": in.AvatarFileID, "capabilities": StringArray(in.Capabilities), "system_prompt": in.SystemPrompt, "multiplier_ppm": in.MultiplierPPM, "generation_params": in.GenerationParams, "updated_by": in.ActorID, "updated_at": time.Now().UTC()}).Error
}
func (r *Repository) SetCurrent(ctx context.Context, id int64, bindingID *int64) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id=?", id).Updates(map[string]any{"active_binding_id": bindingID, "updated_at": time.Now().UTC()}).Error
}
func (r *Repository) SetStatus(ctx context.Context, id int64, status int16, actor int64) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id=?", id).Updates(map[string]any{"status": status, "updated_by": actor, "updated_at": time.Now().UTC()}).Error
}
func (r *Repository) ReplaceBindings(ctx context.Context, m Model, inputs []BindingInput) error {
	old, e := r.Bindings(ctx, m.ID)
	if e != nil {
		return e
	}
	byModel := map[int64]Binding{}
	for _, b := range old {
		byModel[b.ProviderModelID] = b
	}
	seen := map[int64]bool{}
	for _, in := range inputs {
		if seen[in.ProviderModelID] {
			return configurationError("duplicate provider model binding")
		}
		seen[in.ProviderModelID] = true
		b, ok := byModel[in.ProviderModelID]
		if ok && in.ID != 0 && in.ID != b.ID {
			return configurationError("binding id mismatch")
		}
		if m.Status == 1 && m.ActiveBindingID != nil && b.ID == *m.ActiveBindingID && in.Status != 1 {
			return configurationError("switch current binding before disabling it")
		}
		protocol := in.TextProtocol
		if protocol == 0 {
			protocol = 1
		}
		if protocol != 1 && protocol != 2 {
			return configurationError("text protocol invalid")
		}
		params := in.Parameters
		if params == nil {
			params = json.RawMessage(`{}`)
		}
		if ok {
			e = r.db.WithContext(ctx).Model(&b).Updates(map[string]any{"text_protocol": protocol, "parameters": params, "status": in.Status, "updated_at": time.Now().UTC()}).Error
		} else {
			b = Binding{AgentID: m.ID, ProviderModelID: in.ProviderModelID, TextProtocol: protocol, Parameters: params, Status: in.Status}
			e = r.db.WithContext(ctx).Create(&b).Error
		}
		if e != nil {
			return e
		}
	}
	for _, b := range old {
		if !seen[b.ProviderModelID] {
			if m.Status == 1 && m.ActiveBindingID != nil && *m.ActiveBindingID == b.ID {
				return configurationError("switch current binding before removing it")
			}
			if e := r.db.WithContext(ctx).Model(&b).Updates(map[string]any{"status": 0, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}
func (r *Repository) ReplaceTools(ctx context.Context, id int64, inputs []ToolBindingInput) error {
	old, e := r.ToolBindings(ctx, id)
	if e != nil {
		return e
	}
	if e = r.db.WithContext(ctx).Exec(`UPDATE ai_agent_tool SET status=0,exposed_name='__admin_retired_'||id::text,updated_at=? WHERE agent_id=?`, time.Now().UTC(), id).Error; e != nil {
		return e
	}
	byID := map[int64]ToolBinding{}
	for _, b := range old {
		byID[b.ToolID] = b
	}
	seen := map[string]bool{}
	for _, in := range inputs {
		if seen[in.ExposedName] {
			return configurationError("duplicate exposed tool name")
		}
		seen[in.ExposedName] = true
		if in.Status == 1 {
			var n int64
			if e = r.db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_tool WHERE id=? AND status=1 AND deleted_at IS NULL`, in.ToolID).Scan(&n).Error; e != nil {
				return e
			}
			if n != 1 {
				return configurationError("tool unavailable")
			}
		}
		p := in.ParameterConstraints
		if p == nil {
			p = json.RawMessage(`{}`)
		}
		if b, ok := byID[in.ToolID]; ok {
			e = r.db.WithContext(ctx).Model(&b).Updates(map[string]any{"exposed_name": in.ExposedName, "parameter_constraints": p, "status": in.Status, "updated_at": time.Now().UTC()}).Error
		} else {
			b := ToolBinding{AgentID: id, ToolID: in.ToolID, ExposedName: in.ExposedName, ParameterConstraints: p, Status: in.Status}
			e = r.db.WithContext(ctx).Create(&b).Error
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (r *Repository) ValidateBinding(ctx context.Context, agentID, bindingID int64) (Binding, error) {
	var b Binding
	if e := r.db.WithContext(ctx).Where("agent_id=? AND id=? AND status=1", agentID, bindingID).First(&b).Error; e != nil {
		return b, e
	}
	var n int64
	e := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_provider_model m JOIN ai_provider p ON p.id=m.provider_id JOIN ai_official_model o ON o.id=m.official_model_id WHERE m.id=? AND m.status=1 AND m.deleted_at IS NULL AND p.status=1 AND p.deleted_at IS NULL AND o.status=1`, b.ProviderModelID).Scan(&n).Error
	if e != nil {
		return b, e
	}
	if n != 1 {
		return b, configurationError("current provider model unavailable")
	}
	return b, nil
}
func (r *Repository) Delete(ctx context.Context, m Model) error {
	var n int64
	if e := r.db.WithContext(ctx).Raw(`SELECT (SELECT count(*) FROM ai_run WHERE agent_id=?)+(SELECT count(*) FROM ai_conversation WHERE agent_id=?)`, m.ID, m.ID).Scan(&n).Error; e != nil {
		return e
	}
	if n > 0 {
		return ErrReferenced
	}
	return r.db.WithContext(ctx).Delete(&m).Error
}

func (r *Repository) Mutate(ctx context.Context, expected int64, fn func(*Repository) error) (configcache.MutationResult, error) {
	if r == nil || r.db == nil {
		return configcache.MutationResult{}, errors.New("AI database unavailable")
	}
	var result configcache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := fn(NewRepository(tx)); e != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, e)
		}
		event, e := configcache.NewRepository(tx).AdvanceTx(ctx, tx, configcache.Scope{Namespace: "ai.runtime", ScopeKey: "global"}, expected, time.Now().UTC())
		if e != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, e)
		}
		result = configcache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}
