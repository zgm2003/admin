package chat

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrAgentNotFound        = errors.New("chat agent is unavailable")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ConversationView struct {
	Conversation
	AgentName string `gorm:"column:agent_name"`
	Scene     int16  `gorm:"column:scene"`
	ModelName string `gorm:"column:model_name"`
}

func (r *Repository) ListAgents(ctx context.Context) ([]AgentRow, error) {
	rows := make([]AgentRow, 0)
	err := r.db.WithContext(ctx).
		Table("ai_agent AS a").
		Select(`a.id, a.name, a.description, a.scene, a.timeout_ms,
			p.name AS provider_name, pm.display_name AS model_name`).
		Joins("JOIN ai_provider_model pm ON pm.id = a.provider_model_id AND pm.deleted_at IS NULL").
		Joins("JOIN ai_provider p ON p.id = pm.provider_id AND p.deleted_at IS NULL").
		Where(`a.deleted_at IS NULL AND a.status = 1 AND a.agent_type = 1 AND a.scene = 1
			AND pm.status = 1 AND pm.model_kind = 1 AND pm.pricing_mode = 1
			AND p.status = 1`).
		Order("a.id DESC").Find(&rows).Error
	return rows, err
}

func (r *Repository) FindAgent(ctx context.Context, id int64) (AgentRow, error) {
	var row AgentRow
	err := r.db.WithContext(ctx).
		Table("ai_agent AS a").
		Select(`a.id, a.name, a.description, a.scene, a.timeout_ms,
			p.name AS provider_name, pm.display_name AS model_name`).
		Joins("JOIN ai_provider_model pm ON pm.id = a.provider_model_id AND pm.deleted_at IS NULL").
		Joins("JOIN ai_provider p ON p.id = pm.provider_id AND p.deleted_at IS NULL").
		Where(`a.id = ? AND a.deleted_at IS NULL AND a.status = 1 AND a.agent_type = 1 AND a.scene = 1
			AND pm.status = 1 AND pm.model_kind = 1 AND pm.pricing_mode = 1 AND p.status = 1`, id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AgentRow{}, ErrAgentNotFound
	}
	return row, err
}

func (r *Repository) ListConversations(ctx context.Context, platformID, userID int64, keyword string, offset, limit int) ([]ConversationView, int64, error) {
	base := r.db.WithContext(ctx).Table("ai_chat_conversation AS c").
		Joins("JOIN ai_agent a ON a.id = c.agent_id").
		Joins("JOIN ai_provider_model pm ON pm.id = a.provider_model_id").
		Where("c.platform_id = ? AND c.user_id = ? AND c.deleted_at IS NULL", platformID, userID)
	if keyword != "" {
		base = base.Where("c.title ILIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]ConversationView, 0)
	err := base.Select("c.*, a.name AS agent_name, a.scene, pm.display_name AS model_name").
		Order("c.last_message_at DESC, c.id DESC").Offset(offset).Limit(limit).Scan(&rows).Error
	return rows, total, err
}

func (r *Repository) FindConversation(ctx context.Context, platformID, userID, id int64) (Conversation, error) {
	var row Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND platform_id = ? AND user_id = ? AND deleted_at IS NULL", id, platformID, userID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Conversation{}, ErrConversationNotFound
	}
	return row, err
}

func (r *Repository) CreateConversation(ctx context.Context, row Conversation) (int64, error) {
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) UpdateConversationTitle(ctx context.Context, platformID, userID, id int64, title string, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND platform_id = ? AND user_id = ? AND deleted_at IS NULL", id, platformID, userID).
		Updates(map[string]any{"title": title, "updated_at": now, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConversationNotFound
	}
	return nil
}

func (r *Repository) DeleteConversation(ctx context.Context, platformID, userID, id int64, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND platform_id = ? AND user_id = ? AND deleted_at IS NULL", id, platformID, userID).
		Updates(map[string]any{"deleted_at": now, "updated_at": now, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConversationNotFound
	}
	return nil
}

func (r *Repository) ListMessages(ctx context.Context, conversationID int64, beforeSeq int32, limit int) ([]Message, error) {
	rows := make([]Message, 0)
	db := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID)
	if beforeSeq > 0 {
		db = db.Where("seq < ?", beforeSeq)
	}
	err := db.Order("seq DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repository) FindMessage(ctx context.Context, id int64) (Message, error) {
	var row Message
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	return row, err
}

func (r *Repository) MessageByRunID(ctx context.Context, runID int64) (Message, error) {
	var row Message
	err := r.db.WithContext(ctx).Where("run_id = ? AND role = ?", runID, RoleAssistant).Take(&row).Error
	return row, err
}

func (r *Repository) ActiveRunSummary(ctx context.Context, conversationID int64) (ActiveRunSummary, bool, error) {
	var row ActiveRunSummary
	err := r.db.WithContext(ctx).
		Table("ai_chat_run AS r").
		Select("r.id, r.request_id, r.status, r.billing_status, m.id AS assistant_message_id, m.delivery_seq").
		Joins("JOIN ai_chat_message m ON m.run_id = r.id AND m.role = 2").
		Where("r.conversation_id = ? AND r.status IN (1, 2)", conversationID).
		Order("r.id DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ActiveRunSummary{}, false, nil
	}
	return row, err == nil, err
}

func (r *Repository) UpdateStreamingMessage(ctx context.Context, messageID int64, content string, deliverySeq int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Message{}).Where("id = ? AND status = 1", messageID).
		Updates(map[string]any{"content": content, "delivery_seq": deliverySeq, "updated_at": now}).Error
}

func (r *Repository) FinishMessage(ctx context.Context, messageID int64, content string, status MessageStatus, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Message{}).Where("id = ?", messageID).
		Updates(map[string]any{"content": content, "status": status, "updated_at": now}).Error
}

func normalizeLimit(limit int) int {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}
