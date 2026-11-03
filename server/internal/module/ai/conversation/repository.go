package conversation

import (
	"context"
	"errors"
	"time"

	"admin/server/internal/module/ai/message"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

type AgentOption struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) LockOwnedTx(ctx context.Context, tx *gorm.DB, id, userID, platformID int64) (Model, error) {
	var row Model
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=? AND platform_id=?", id, userID, platformID).First(&row).Error
	return row, err
}
func (r *Repository) Create(ctx context.Context, userID, platformID, agentID int64, title string, now time.Time) (Model, error) {
	var row Model
	res := r.db.WithContext(ctx).Raw(`INSERT INTO ai_conversation(user_id,platform_id,agent_id,title,created_at,updated_at) SELECT ?,?,a.id,?,?,? FROM ai_agent a WHERE a.id=? AND a.status=1 AND a.deleted_at IS NULL RETURNING *`, userID, platformID, title, now, now, agentID).Scan(&row)
	if res.Error != nil {
		return row, res.Error
	}
	if res.RowsAffected != 1 {
		return row, gorm.ErrRecordNotFound
	}
	return row, nil
}
func (r *Repository) AgentOptions(ctx context.Context) ([]AgentOption, error) {
	rows := make([]AgentOption, 0)
	err := r.db.WithContext(ctx).Table("ai_agent").Select("id,name").Where("status=1 AND deleted_at IS NULL").Order("name,id").Limit(100).Scan(&rows).Error
	return rows, err
}
func (r *Repository) Find(ctx context.Context, id, userID, platformID int64) (Model, error) {
	var row Model
	err := r.db.WithContext(ctx).Where("id=? AND user_id=? AND platform_id=?", id, userID, platformID).First(&row).Error
	return row, err
}
func (r *Repository) List(ctx context.Context, userID, platformID, beforeID int64, limit int) ([]Model, error) {
	rows := make([]Model, 0)
	if limit < 1 || limit > 100 {
		return rows, errors.New("AI conversation page is invalid")
	}
	q := r.db.WithContext(ctx).Where("user_id=? AND platform_id=?", userID, platformID)
	if beforeID > 0 {
		q = q.Where("(updated_at,id)<(SELECT updated_at,id FROM ai_conversation WHERE id=? AND user_id=? AND platform_id=? AND deleted_at IS NULL)", beforeID, userID, platformID)
	}
	err := q.Order("updated_at DESC,id DESC").Limit(limit + 1).Find(&rows).Error
	return rows, err
}
func (r *Repository) Update(ctx context.Context, id, userID, platformID int64, title string, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND user_id=? AND platform_id=?", id, userID, platformID).Updates(map[string]any{"title": title, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) Hide(ctx context.Context, id, userID, platformID int64, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND user_id=? AND platform_id=?", id, userID, platformID).Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) Messages(ctx context.Context, id, userID, platformID, after int64, limit int) ([]message.Model, error) {
	rows := make([]message.Model, 0)
	if _, e := r.Find(ctx, id, userID, platformID); e != nil {
		return rows, e
	}
	if after < 0 || limit < 1 || limit > 100 {
		return rows, errors.New("AI message page is invalid")
	}
	err := r.db.WithContext(ctx).Where("conversation_id=? AND sequence>?", id, after).Order("sequence").Limit(limit + 1).Find(&rows).Error
	return rows, err
}

func (r *Repository) CandidateRows(ctx context.Context, before int64, limit int, keyword string) ([]AgentOption, error) {
	rows := make([]AgentOption, 0)
	q := r.db.WithContext(ctx).Table("ai_agent a").Select("a.id,a.name").Joins("JOIN ai_agent_model_binding b ON b.id=a.active_binding_id AND b.agent_id=a.id AND b.status=1").Joins("JOIN ai_provider_model m ON m.id=b.provider_model_id AND m.status=1 AND m.deleted_at IS NULL").Joins("JOIN ai_provider p ON p.id=m.provider_id AND p.status=1 AND p.deleted_at IS NULL").Joins("JOIN ai_official_model o ON o.id=m.official_model_id AND o.status=1").Where("a.status=1 AND a.deleted_at IS NULL").Where("EXISTS(SELECT 1 FROM ai_provider_model_price_version v WHERE v.provider_model_id=m.id AND v.status=1 AND v.currency='CNY' AND v.effective_at<=?)", time.Now().UTC())
	if before > 0 {
		q = q.Where("a.id<?", before)
	}
	if keyword != "" {
		q = q.Where("a.name ILIKE ?", "%"+keyword+"%")
	}
	e := q.Order("a.id DESC").Limit(limit + 1).Scan(&rows).Error
	return rows, e
}
