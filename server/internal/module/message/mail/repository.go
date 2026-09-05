package mail

import (
	"admin/server/internal/shared/yesno"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) FindConfig(ctx context.Context, platformID int64) (Config, error) {
	var v Config
	e := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID).Take(&v).Error
	return v, e
}
func (r *Repository) SaveConfig(ctx context.Context, platformID int64, v map[string]any) (Config, error) {
	var row Config
	tx := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID).First(&row)
	if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		row = Config{PlatformID: platformID}
		for k, val := range v {
			_ = setConfig(&row, k, val)
		}
		if e := r.db.WithContext(ctx).Create(&row).Error; e != nil {
			return Config{}, e
		}
		return row, nil
	}
	if tx.Error != nil {
		return Config{}, tx.Error
	}
	if e := r.db.WithContext(ctx).Model(&row).Updates(v).Error; e != nil {
		return Config{}, e
	}
	return r.FindConfig(ctx, platformID)
}
func setConfig(row *Config, k string, v any) error {
	switch k {
	case "secret_id_ciphertext":
		row.SecretIDCiphertext = v.(string)
	case "secret_key_ciphertext":
		row.SecretKeyCiphertext = v.(string)
	case "secret_id_hint":
		row.SecretIDHint = v.(string)
	case "secret_key_hint":
		row.SecretKeyHint = v.(string)
	case "region":
		row.Region = v.(string)
	case "from_email":
		row.FromEmail = v.(string)
	case "from_name":
		row.FromName = v.(string)
	case "endpoint":
		if value, ok := v.(*string); ok {
			row.Endpoint = value
		} else if value, ok := v.(string); ok {
			row.Endpoint = &value
		}
	case "reply_to":
		if value, ok := v.(*string); ok {
			row.ReplyTo = value
		} else if value, ok := v.(string); ok {
			row.ReplyTo = &value
		}
	case "ttl_minutes":
		row.TTLMinutes = int16(v.(int))
	case "is_enabled":
		switch value := v.(type) {
		case yesno.Value:
			row.IsEnabled = value
		case int16:
			row.IsEnabled = yesno.Value(value)
		case int:
			row.IsEnabled = yesno.Value(value)
		}
	}
	return nil
}
func (r *Repository) DeleteConfig(ctx context.Context, platformID int64) error {
	now := time.Now().UTC()
	q := r.db.WithContext(ctx).Model(&Config{}).Where("platform_id = ? AND deleted_at IS NULL", platformID).Updates(map[string]any{"deleted_at": now, "updated_at": now, "is_enabled": 0})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) ListTemplates(ctx context.Context, platformID int64) ([]Template, error) {
	var v []Template
	e := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID).Order("id").Find(&v).Error
	return v, e
}
func (r *Repository) FindTemplate(ctx context.Context, platformID, id int64) (Template, error) {
	var v Template
	e := r.db.WithContext(ctx).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Take(&v).Error
	return v, e
}
func (r *Repository) UpdateTemplate(ctx context.Context, platformID, id int64, values map[string]any) error {
	q := r.db.WithContext(ctx).Model(&Template{}).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Updates(values)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) CreatePendingLog(ctx context.Context, v *Log) (Log, error) {
	e := r.db.WithContext(ctx).Create(v).Error
	return *v, e
}
func (r *Repository) FindActiveChallenge(ctx context.Context, platformID int64, challenge string) (Log, error) {
	var v Log
	e := r.db.WithContext(ctx).Where("platform_id = ? AND challenge_id = ? AND deleted_at IS NULL", platformID, challenge).Take(&v).Error
	return v, e
}
func (r *Repository) MarkSent(ctx context.Context, platformID, id int64, result ProviderSendResult, latencyMs int64) error {
	now := time.Now().UTC()
	q := r.db.WithContext(ctx).Model(&Log{}).Where("id = ? AND platform_id = ? AND status = ? AND deleted_at IS NULL", id, platformID, StatusPending).Updates(map[string]any{"status": StatusSent, "request_id": result.RequestID, "message_id": result.MessageID, "latency_ms": latencyMs, "sent_at": now, "updated_at": now})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) MarkFailed(ctx context.Context, platformID, id int64, pe *ProviderError, latencyMs int64) error {
	q := r.db.WithContext(ctx).Model(&Log{}).Where("id = ? AND platform_id = ? AND status = ? AND deleted_at IS NULL", id, platformID, StatusPending).Updates(map[string]any{"status": StatusFailed, "error_code": pe.Code, "error_summary": pe.Summary, "latency_ms": latencyMs, "updated_at": time.Now().UTC()})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) RecordTestResult(ctx context.Context, platformID int64, at time.Time, summary string) error {
	q := r.db.WithContext(ctx).Model(&Config{}).Where("platform_id = ? AND deleted_at IS NULL", platformID).Updates(map[string]any{"last_test_at": at, "last_test_error": summary, "updated_at": at})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) AddVerification(ctx context.Context, v *Verification) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *Repository) ListLogs(ctx context.Context, platformID int64, page, size int) ([]Log, int64, error) {
	var rows []Log
	var n int64
	db := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID)
	if e := db.Model(&Log{}).Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, n, e
}
func (r *Repository) GetLogDetail(ctx context.Context, platformID, id int64) (Log, Verification, error) {
	var l Log
	e := r.db.WithContext(ctx).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Take(&l).Error
	if e != nil {
		return l, Verification{}, e
	}
	var v Verification
	e = r.db.WithContext(ctx).Where("platform_id = ? AND mail_log_id = ? AND deleted_at IS NULL", platformID, id).Take(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = nil
	}
	return l, v, e
}
func (r *Repository) DeleteLog(ctx context.Context, platformID, id int64) error {
	q := r.db.WithContext(ctx).Model(&Log{}).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Update("deleted_at", time.Now().UTC())
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) DeleteLogs(ctx context.Context, platformID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&Log{}).Where("platform_id = ? AND id IN ? AND deleted_at IS NULL", platformID, ids).Update("deleted_at", time.Now().UTC()).Error
}
func (r *Repository) ListRules(ctx context.Context, platformID int64) ([]RecipientRule, error) {
	var v []RecipientRule
	e := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID).Order("id").Find(&v).Error
	return v, e
}
func (r *Repository) CreateRule(ctx context.Context, v *RecipientRule) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *Repository) UpdateRule(ctx context.Context, platformID, id int64, values map[string]any) error {
	q := r.db.WithContext(ctx).Model(&RecipientRule{}).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Updates(values)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) DeleteRule(ctx context.Context, platformID, id int64) error {
	q := r.db.WithContext(ctx).Model(&RecipientRule{}).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Update("deleted_at", time.Now().UTC())
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) Lock(ctx context.Context, platformID, id int64) (RecipientRule, error) {
	var v RecipientRule
	e := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Take(&v).Error
	return v, e
}
func (r *Repository) ListRateLimitPolicies(ctx context.Context) (RateLimitCatalog, error) {
	var rows []RateLimitPolicy
	if e := r.db.WithContext(ctx).Find(&rows).Error; e != nil {
		return RateLimitCatalog{}, e
	}
	return buildRateLimitCatalog(rows)
}

func (r *Repository) UpdateRateLimitPolicy(ctx context.Context, input RateLimitPolicyInput) (RateLimitCatalog, error) {
	var catalog RateLimitCatalog
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []RateLimitPolicy
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("policy_key ASC").Find(&rows).Error; e != nil {
			return e
		}
		current, e := buildRateLimitCatalog(rows)
		if e != nil {
			return e
		}
		nextRevision := current.Version + 1
		q := tx.Model(&RateLimitPolicy{}).Where("policy_key = ?", input.Key).Updates(map[string]any{
			"limit_count":    input.Limit,
			"window_seconds": input.WindowSeconds,
			"revision":       nextRevision,
			"updated_at":     time.Now().UTC(),
		})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		var after []RateLimitPolicy
		if e := tx.Find(&after).Error; e != nil {
			return e
		}
		catalog, e = buildRateLimitCatalog(after)
		return e
	})
	if err != nil {
		return RateLimitCatalog{}, err
	}
	return catalog, nil
}

func buildRateLimitCatalog(rows []RateLimitPolicy) (RateLimitCatalog, error) {
	if len(rows) != len(fixedRateLimitPolicyKeys) {
		return RateLimitCatalog{}, fmt.Errorf("rate limit policy table must contain exactly %d rows, got %d", len(fixedRateLimitPolicyKeys), len(rows))
	}
	byKey := make(map[string]RateLimitPolicy, len(rows))
	var version int64
	for _, row := range rows {
		if _, exists := byKey[row.Key]; exists {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q is duplicated", row.Key)
		}
		spec, ok := fixedRateLimitSpecByKey(row.Key)
		if !ok {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q is unknown", row.Key)
		}
		if row.Mode != spec.Mode || row.Dimension != spec.Dimension {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q has invalid mode or dimension", row.Key)
		}
		if row.Limit < 1 || row.Limit > 100000 || row.WindowSeconds < 1 || row.WindowSeconds > 86400 {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q values are out of range", row.Key)
		}
		if row.Revision < 1 || row.UpdatedAt.IsZero() {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q has invalid revision or timestamp", row.Key)
		}
		byKey[row.Key] = row
		if row.Revision > version {
			version = row.Revision
		}
	}
	ordered := make([]RateLimitPolicy, 0, len(fixedRateLimitPolicyKeys))
	for _, key := range fixedRateLimitPolicyKeys {
		row, ok := byKey[key]
		if !ok {
			return RateLimitCatalog{}, fmt.Errorf("rate limit policy %q is missing", key)
		}
		ordered = append(ordered, row)
	}
	if version < 1 {
		return RateLimitCatalog{}, fmt.Errorf("rate limit policy catalog version is invalid")
	}
	return RateLimitCatalog{Version: version, Policies: ordered}, nil
}

func wrapRepo(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	if isUniqueViolation(err) {
		return conflict(err)
	}
	return dependency(fmt.Errorf("mail repository: %w", err))
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
