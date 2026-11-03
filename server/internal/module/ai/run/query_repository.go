package run

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"strconv"
	"time"
)

type QueryRepository struct{ db *gorm.DB }

func NewQueryRepository(db *gorm.DB) *QueryRepository { return &QueryRepository{db} }
func (r *QueryRepository) scope(ctx context.Context, q RunQuery) *gorm.DB {
	db := r.db.WithContext(ctx).Model(&Model{}).Where("ai_run.accepted_at>=? AND ai_run.accepted_at<?", q.StartAt, q.EndAt)
	for _, v := range []struct {
		column string
		value  int64
	}{{"user_id", q.UserID}, {"platform_id", q.PlatformID}, {"agent_id", q.AgentID}, {"provider_model_id", q.ProviderModelID}} {
		if v.value > 0 {
			db = db.Where("ai_run."+v.column+"=?", v.value)
		}
	}
	if q.ProviderID > 0 {
		db = db.Where("provider_model_id IN (SELECT id FROM ai_provider_model WHERE provider_id=?)", q.ProviderID)
	}
	if q.Operation != "" {
		db = db.Where("operation=?", q.Operation)
	}
	for _, v := range []struct {
		column string
		value  *int16
	}{{"execution_status", q.ExecutionStatus}, {"usage_status", q.UsageStatus}, {"billing_status", q.BillingStatus}, {"transfer_status", q.TransferStatus}} {
		if v.value != nil {
			db = db.Where(v.column+"=?", *v.value)
		}
	}
	return db
}
func (r *QueryRepository) List(ctx context.Context, q RunQuery) ([]Model, error) {
	rows := make([]Model, 0)
	db := r.scope(ctx, q).Omit("input_snapshot", "config_snapshot", "policy_snapshot")
	if q.BeforeID > 0 {
		db = db.Where("(accepted_at,id)<(SELECT accepted_at,id FROM ai_run WHERE id=?)", q.BeforeID)
	}
	err := db.Order("accepted_at DESC,id DESC").Limit(q.Limit + 1).Find(&rows).Error
	return rows, err
}
func (r *QueryRepository) Find(ctx context.Context, id int64, o QueryOwner) (Model, error) {
	var m Model
	db := r.db.WithContext(ctx).Where("id=?", id)
	if o.valid() {
		db = db.Where("user_id=? AND platform_id=?", o.UserID, o.PlatformID)
	}
	e := db.First(&m).Error
	return m, e
}
func (r *QueryRepository) Summary(ctx context.Context, q RunQuery) (QuerySummary, error) {
	var out QuerySummary
	sub := r.scope(ctx, q).Select("id,estimated_units").Limit(100001)
	e := r.db.WithContext(ctx).Table("(?) AS selected", sub).Select("count(*) AS run_count,coalesce(sum(selected.estimated_units),0)::text AS estimated_units,coalesce(sum(c.actual_units) FILTER(WHERE c.status=2),0)::text AS settled_units").Joins("LEFT JOIN ai_usage_charge c ON c.run_id=selected.id").Scan(&out).Error
	if e != nil {
		return out, e
	}
	if out.RunCount > 100000 {
		return QuerySummary{}, ErrQueryRangeTooLarge
	}
	return out, nil
}

func (r *QueryRepository) Events(ctx context.Context, id, after int64, limit int) ([]EventResponse, error) {
	rows := make([]EventResponse, 0)
	e := r.db.WithContext(ctx).Table("ai_run_event").Select("id::text,run_id::text,sequence,event_type,payload,created_at").Where("run_id=? AND sequence>?", id, after).Order("sequence").Limit(limit + 1).Scan(&rows).Error
	return rows, e
}
func (r *QueryRepository) Children(ctx context.Context, id, before int64, limit int, kind string) (any, error) {
	db := r.db.WithContext(ctx)
	where := "run_id=?"
	args := []any{id}
	if before > 0 {
		where += " AND id<?"
		args = append(args, before)
	}
	switch kind {
	case "request":
		rows := make([]RequestResponse, 0)
		e := db.Table("ai_provider_request").Select("id::text,run_id::text,sequence,status,response_items,usage,usage_status,started_at,finished_at").Where(where, args...).Order("id DESC").Limit(limit + 1).Scan(&rows).Error
		if e == nil {
			for i := range rows {
				var x error
				rows[i].ResponseItems, x = publicProviderItems(rows[i].ResponseItems)
				if x != nil {
					return nil, x
				}
				rows[i].Usage, x = publicProviderUsage(rows[i].Usage)
				if x != nil {
					return nil, x
				}
			}
		}
		return page(rows, limit, func(v RequestResponse) int64 { i, _ := strconv.ParseInt(v.ID, 10, 64); return i }), e
	case "tool-call":
		rows := make([]ToolResponse, 0)
		e := db.Table("ai_tool_call").Select("id::text,run_id::text,provider_request_id::text,exposed_name,status,started_at,finished_at").Where(where, args...).Order("id DESC").Limit(limit + 1).Scan(&rows).Error
		return page(rows, limit, func(v ToolResponse) int64 { i, _ := strconv.ParseInt(v.ID, 10, 64); return i }), e
	case "usage-charge":
		rows := make([]ChargeResponse, 0)
		e := db.Table("ai_usage_charge").Select("id::text,run_id::text,status,currency,price_version_id::text,multiplier_ppm,actual_units,usage_complete,settled_at,(SELECT t.id::text FROM payment_transaction t WHERE t.ai_usage_charge_id=ai_usage_charge.id LIMIT 1) AS transaction_id").Where(where, args...).Order("id DESC").Limit(limit + 1).Scan(&rows).Error
		return page(rows, limit, func(v ChargeResponse) int64 { i, _ := strconv.ParseInt(v.ID, 10, 64); return i }), e
	case "media":
		rows := make([]MediaResponse, 0)
		e := db.Table("ai_media_file").Select("id::text,run_id::text,role,ordinal,storage_file_id::text,mime_type,size_bytes,transfer_status,error_code,error_message,(storage_file_id IS NOT NULL AND transfer_status=2) AS can_preview,coalesce((role=2 AND source_kind=1 AND source_recoverable=1 AND source_ciphertext IS NOT NULL AND source_ciphertext<>'' AND encryption_key_id IS NOT NULL AND encryption_key_id<>'' AND transfer_status IN(3,4) AND transfer_deadline_at>? AND (source_expires_at IS NULL OR source_expires_at>?)),false) AS can_retry_transfer", time.Now().UTC(), time.Now().UTC()).Where(where, args...).Order("id DESC").Limit(limit + 1).Scan(&rows).Error
		return page(rows, limit, func(v MediaResponse) int64 { i, _ := strconv.ParseInt(v.ID, 10, 64); return i }), e
	case "usage-charge-item":
		rows := make([]ChargeItemResponse, 0)
		where = "charge_id IN (SELECT id FROM ai_usage_charge WHERE run_id=?)"
		if before > 0 {
			where += " AND id<?"
		}
		e := db.Table("ai_usage_charge_item").Select("id::text,charge_id::text,provider_request_id::text,price_rate_id::text,operation,usage_key,selector_snapshot AS selector,quantity::text,unit_quantity::text,unit,billing_unit_price_units,multiplier_ppm,amount_units").Where(where, args...).Order("id DESC").Limit(limit + 1).Scan(&rows).Error
		return page(rows, limit, func(v ChargeItemResponse) int64 { i, _ := strconv.ParseInt(v.ID, 10, 64); return i }), e
	}
	return nil, errors.New("invalid AI child query")
}
func (r *QueryRepository) Result(ctx context.Context, id int64) (json.RawMessage, error) {
	var row struct{ Content json.RawMessage }
	res := r.db.WithContext(ctx).Table("ai_message").Select("content").Where("run_id=? AND role=2", id).Order("sequence DESC").Limit(1).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		var request struct{ ResponseItems json.RawMessage }
		e := r.db.WithContext(ctx).Table("ai_provider_request").Select("response_items").Where("run_id=?", id).Order("sequence DESC").Limit(1).Scan(&request).Error
		if e != nil {
			return nil, e
		}
		if len(request.ResponseItems) == 0 {
			return json.RawMessage(`null`), nil
		}
		return request.ResponseItems, nil
	}
	return row.Content, nil
}
