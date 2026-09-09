package log

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ProviderResult struct {
	RequestID string
	MessageID string
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreatePending(ctx context.Context, value *Model) (Model, error) {
	err := r.db.WithContext(ctx).Create(value).Error
	return *value, err
}

func (r *Repository) FindActiveChallenge(ctx context.Context, platformID int64, challenge string) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("platform_id = ? AND challenge_id = ?", platformID, challenge).Take(&value).Error
	return value, err
}

func (r *Repository) MarkSent(ctx context.Context, platformID, id int64, result ProviderResult, latencyMs int64) error {
	now := time.Now().UTC()
	query := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND platform_id = ? AND status = ?", id, platformID, "pending").
		Updates(map[string]any{
			"status": "sent", "request_id": result.RequestID, "message_id": result.MessageID,
			"latency_ms": latencyMs, "sent_at": now, "updated_at": now,
		})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, platformID, id int64, errorCode, errorSummary string, latencyMs int64) error {
	query := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND platform_id = ? AND status = ?", id, platformID, "pending").
		Updates(map[string]any{
			"status": "failed", "error_code": errorCode, "error_summary": errorSummary,
			"latency_ms": latencyMs, "updated_at": time.Now().UTC(),
		})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListRow carries a mail log together with the source platform code and the
// associated account username. The admin console is the control plane for
// every platform, so listing is not scoped to the caller's platform.
type ListRow struct {
	Model    `gorm:"embedded"`
	Platform string `gorm:"column:platform"`
	Username string `gorm:"column:username"`
}

// ListQuery filters delivery logs. Platform matches the platform code prefix,
// ToEmail matches the recipient address prefix; Scene and Status match exactly.
type ListQuery struct {
	Platform string
	ToEmail  string
	Scene    string
	Status   string
	From     *time.Time
	To       *time.Time
}

func (r *Repository) List(ctx context.Context, filter ListQuery, page, size int) ([]ListRow, int64, error) {
	count := r.db.WithContext(ctx).
		Table(Table).
		Joins("LEFT JOIN permission_auth_platform ON permission_auth_platform.id = message_mail_log.platform_id")
	count = applyListFilters(count, filter)
	var total int64
	if err := count.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ListRow
	query := r.db.WithContext(ctx).
		Table(Table).
		Select("message_mail_log.*, COALESCE(permission_auth_platform.code, '') AS platform, COALESCE(user_account.username, '') AS username").
		Joins("LEFT JOIN permission_auth_platform ON permission_auth_platform.id = message_mail_log.platform_id").
		Joins("LEFT JOIN user_account ON user_account.id = message_mail_log.user_id AND user_account.deleted_at IS NULL")
	query = applyListFilters(query, filter)
	err := query.
		Order("message_mail_log.id DESC").
		Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	return rows, total, err
}

func applyListFilters(db *gorm.DB, filter ListQuery) *gorm.DB {
	if filter.Platform != "" {
		db = db.Where("permission_auth_platform.code LIKE ? ESCAPE '\\'", escapeLikePrefix(filter.Platform))
	}
	if filter.ToEmail != "" {
		db = db.Where("message_mail_log.to_email LIKE ? ESCAPE '\\'", escapeLikePrefix(filter.ToEmail))
	}
	if filter.Scene != "" {
		db = db.Where("message_mail_log.scene = ?", filter.Scene)
	}
	if filter.Status != "" {
		db = db.Where("message_mail_log.status = ?", filter.Status)
	}
	if filter.From != nil {
		db = db.Where("message_mail_log.created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		db = db.Where("message_mail_log.created_at <= ?", *filter.To)
	}
	return db
}

func escapeLikePrefix(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value) + "%"
}

func (r *Repository) Find(ctx context.Context, id int64) (ListRow, error) {
	var value ListRow
	err := r.db.WithContext(ctx).
		Table(Table).
		Select("message_mail_log.*, COALESCE(permission_auth_platform.code, '') AS platform, COALESCE(user_account.username, '') AS username").
		Joins("LEFT JOIN permission_auth_platform ON permission_auth_platform.id = message_mail_log.platform_id").
		Joins("LEFT JOIN user_account ON user_account.id = message_mail_log.user_id AND user_account.deleted_at IS NULL").
		Where("message_mail_log.id = ?", id).
		Take(&value).Error
	return value, err
}
