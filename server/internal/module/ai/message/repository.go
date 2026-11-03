package message

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) AppendTx(ctx context.Context, tx *gorm.DB, conversationID, runID int64, role int16, content json.RawMessage, status int16, now time.Time) (Model, error) {
	var row Model
	if tx == nil || conversationID < 1 || runID < 1 || role < 1 || role > 3 || status < 1 || status > 3 || len(content) > 1024*1024 {
		return row, errors.New("AI message invalid")
	}
	var sequence int64
	res := tx.WithContext(ctx).Raw(`UPDATE ai_conversation SET last_message_sequence=last_message_sequence+1,updated_at=? WHERE id=? RETURNING last_message_sequence`, now, conversationID).Scan(&sequence)
	if res.Error != nil {
		return row, res.Error
	}
	if res.RowsAffected != 1 {
		return row, gorm.ErrRecordNotFound
	}
	row = Model{ConversationID: conversationID, RunID: &runID, Sequence: sequence, Role: role, Content: content, Status: status, CreatedAt: now, UpdatedAt: now}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}
