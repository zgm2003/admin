package run

import (
	"context"
	"errors"
	"strings"
	"time"

	"admin/server/internal/module/ai/chat"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrRunNotFound   = errors.New("chat run not found")
	ErrActiveRun     = errors.New("conversation already has an active run")
	ErrAgentNotFound = errors.New("chat agent is unavailable")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindAgent(ctx context.Context, id int64) (AgentConfig, error) {
	var row AgentConfig
	err := r.db.WithContext(ctx).
		Table("ai_agent AS a").
		Select(`a.id, a.name, a.description, a.system_prompt, a.scene, a.agent_type, a.timeout_ms,
			a.provider_model_id, p.id AS provider_id, p.name AS provider_name, p.protocol AS provider_protocol,
			p.base_url, p.api_key_ciphertext, p.encryption_key_id,
			p.token_rate_multiplier_ppm, pm.model_key, pm.display_name, pm.upstream_model,
			pm.pricing_mode, pm.official_model_id,
			o.input_cny_micros, o.output_cny_micros, o.cache_read_cny_micros, o.cache_write_cny_micros`).
		Joins("JOIN ai_provider_model pm ON pm.id = a.provider_model_id AND pm.deleted_at IS NULL").
		Joins("JOIN ai_provider p ON p.id = pm.provider_id AND p.deleted_at IS NULL").
		Joins("JOIN ai_official_model o ON o.id = pm.official_model_id").
		Where(`a.id = ? AND a.deleted_at IS NULL AND a.status = 1 AND a.agent_type = 1 AND a.scene = 1
			AND pm.status = 1 AND pm.model_kind = 1 AND pm.pricing_mode = 1 AND p.status = 1`, id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AgentConfig{}, ErrAgentNotFound
	}
	return row, err
}

func (r *Repository) FindRunByRequest(ctx context.Context, platformID, userID int64, requestID string) (Run, bool, error) {
	var row Run
	err := r.db.WithContext(ctx).
		Where("platform_id = ? AND user_id = ? AND request_id = ?", platformID, userID, requestID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Run{}, false, nil
	}
	return row, err == nil, err
}

func (r *Repository) CreateTurnTx(ctx context.Context, tx *gorm.DB, conversation chat.Conversation, userContent, title string, run Run, now time.Time) (chat.Message, chat.Message, error) {
	var active int64
	if err := tx.WithContext(ctx).Model(&Run{}).
		Where("conversation_id = ? AND status IN (1, 2)", conversation.ID).Count(&active).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	if active > 0 {
		return chat.Message{}, chat.Message{}, ErrActiveRun
	}
	var maxSeq int32
	if err := tx.WithContext(ctx).Model(&chat.Message{}).
		Where("conversation_id = ?", conversation.ID).
		Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	userMessage := chat.Message{
		ConversationID: conversation.ID, Seq: maxSeq + 1, Role: chat.RoleUser,
		Content: userContent, Status: chat.MessageCompleted, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.WithContext(ctx).Create(&userMessage).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	assistant := chat.Message{
		ConversationID: conversation.ID, Seq: maxSeq + 2, Role: chat.RoleAssistant,
		Content: "", Status: chat.MessageStreaming, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.WithContext(ctx).Create(&assistant).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	run.ConversationID = conversation.ID
	run.CreatedAt, run.UpdatedAt = now, now
	if err := tx.WithContext(ctx).Create(&run).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	runID := run.ID
	if err := tx.WithContext(ctx).Model(&chat.Message{}).Where("id = ?", assistant.ID).
		Update("run_id", runID).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	updates := map[string]any{
		"last_message_at": now, "updated_at": now,
		"message_count": gorm.Expr("message_count + 2"), "version": gorm.Expr("version + 1"),
	}
	if conversation.MessageCount == 0 && strings.TrimSpace(title) != "" {
		updates["title"] = title
	}
	if err := tx.WithContext(ctx).Model(&chat.Conversation{}).Where("id = ?", conversation.ID).
		Updates(updates).Error; err != nil {
		return chat.Message{}, chat.Message{}, err
	}
	assistant.RunID = &runID
	return userMessage, assistant, nil
}

func (r *Repository) LockRunTx(ctx context.Context, tx *gorm.DB, id int64) (Run, error) {
	var row Run
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Run{}, ErrRunNotFound
	}
	return row, err
}

func (r *Repository) FindRun(ctx context.Context, id int64) (Run, error) {
	var row Run
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Run{}, ErrRunNotFound
	}
	return row, err
}

func (r *Repository) ClaimRun(ctx context.Context, id int64, now time.Time) (Run, bool, error) {
	result := r.db.WithContext(ctx).Model(&Run{}).
		Where("id = ? AND status = 1", id).
		Updates(map[string]any{"status": RunRunning, "started_at": now, "updated_at": now})
	if result.Error != nil {
		return Run{}, false, result.Error
	}
	row, err := r.FindRun(ctx, id)
	if err != nil {
		return Run{}, false, err
	}
	return row, result.RowsAffected == 1, nil
}

func (r *Repository) CancelRun(ctx context.Context, platformID, userID, id int64, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Run{}).
		Where("id = ? AND platform_id = ? AND user_id = ? AND status IN (1, 2)", id, platformID, userID).
		Updates(map[string]any{"cancel_requested_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRunNotFound
	}
	return nil
}

func (r *Repository) CancelRequested(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Run{}).
		Where("id = ? AND cancel_requested_at IS NOT NULL", id).Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListRuns(ctx context.Context, query RunListQuery) ([]Run, int64, error) {
	base := r.db.WithContext(ctx).Model(&Run{}).Where("platform_id = ?", query.PlatformID)
	if !query.ViewAll {
		base = base.Where("user_id = ?", query.SelfUserID)
	}
	if query.UserID != nil {
		base = base.Where("user_id = ?", *query.UserID)
	}
	if query.Status != nil {
		base = base.Where("status = ?", *query.Status)
	}
	if query.Keyword != "" {
		pattern := "%" + query.Keyword + "%"
		base = base.Where("model_key ILIKE ? OR agent_name ILIKE ? OR error_message ILIKE ?", pattern, pattern, pattern)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]Run, 0)
	err := base.Order("created_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error
	return rows, total, err
}

func (r *Repository) ListRunEvents(ctx context.Context, runID int64) ([]RunEvent, error) {
	rows := make([]RunEvent, 0)
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("seq ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) AppendRunEventTx(ctx context.Context, tx *gorm.DB, runID int64, eventType, message, data string, now time.Time) error {
	var seq int32
	if err := tx.WithContext(ctx).Model(&RunEvent{}).Where("run_id = ?", runID).
		Select("COALESCE(MAX(seq), 0)").Scan(&seq).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&RunEvent{
		RunID: runID, Seq: seq + 1, Type: eventType, Message: message, Data: data, CreatedAt: now,
	}).Error
}

func (r *Repository) MarkRunFailed(ctx context.Context, id int64, status RunStatus, billing BillingStatus, code, message string, now time.Time) error {
	if message == "" {
		message = "运行未完成"
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	return r.db.WithContext(ctx).Model(&Run{}).Where("id = ?", id).
		Updates(map[string]any{
			"status": status, "billing_status": billing,
			"error_code": code, "error_message": message,
			"finished_at": now, "updated_at": now,
		}).Error
}

func (r *Repository) RecoverInterrupted(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&Run{}).
		Where("status IN (1, 2)").Updates(map[string]any{
		"status": RunFailed, "billing_status": BillingUnbilled,
		"error_code": "interrupted", "error_message": "API restarted before the run completed",
		"finished_at": now, "updated_at": now,
	})
	return result.RowsAffected, result.Error
}

func (r *Repository) Username(ctx context.Context, userID int64) (string, error) {
	var name string
	err := r.db.WithContext(ctx).Table("user_account").Select("username").
		Where("id = ?", userID).Scan(&name).Error
	return name, err
}

func (r *Repository) Usernames(ctx context.Context, ids []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID       int64  `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("user_account").Select("id, username").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Username
	}
	return result, nil
}
