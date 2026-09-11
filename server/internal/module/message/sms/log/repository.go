package log

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrChallengeActive reports that an undelivered challenge still owns the log.
var ErrChallengeActive = errors.New("sms challenge is already active")

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreatePending(ctx context.Context, value *Model) error {
	if err := r.db.WithContext(ctx).Create(&value).Error; err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrChallengeActive
		}
		return err
	}
	return nil
}

func (r *Repository) FindActiveChallenge(ctx context.Context, platformID int64, challengeID string) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).
		Where("platform_id = ? AND challenge_id = ? AND status = ?", platformID, challengeID, StatusPending).
		First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

// Finish closes one pending log. A finished log can never be rewritten.
func (r *Repository) Finish(ctx context.Context, id, platformID int64, status, requestID, serialNo string, fee int, errorCode, errorSummary string, latencyMS int64, sentAt *time.Time, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND platform_id = ? AND status = ?", id, platformID, StatusPending).
		Updates(map[string]any{
			"status":        status,
			"request_id":    requestID,
			"serial_no":     serialNo,
			"fee":           fee,
			"error_code":    errorCode,
			"error_summary": errorSummary,
			"latency_ms":    latencyMS,
			"sent_at":       sentAt,
			"updated_at":    now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Query is the administrative list filter. PhoneToHMAC carries the exact-match
// HMAC of a complete phone number, never the plaintext number.
type Query struct {
	Page        int
	PageSize    int
	Platform    string
	PhoneToHMAC *string
	Scene       string
	Status      string
	From        *time.Time
	To          *time.Time
}

type ListRow struct {
	Model
	Platform string
	Username string
}

// baseQuery joins the platform and the user with LEFT JOIN and without
// deleted_at filters, so soft deleted platforms and users stay visible.
func (r *Repository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table(Table).
		Select(Table + ".*, COALESCE(platform.code, '') AS platform, COALESCE(app_user.username, '') AS username").
		Joins("LEFT JOIN permission_auth_platform AS platform ON platform.id = " + Table + ".platform_id").
		Joins("LEFT JOIN user_account AS app_user ON app_user.id = " + Table + ".user_id")
}

func (r *Repository) List(ctx context.Context, query Query) ([]ListRow, int64, error) {
	db := r.baseQuery(ctx).Session(&gorm.Session{})
	if query.Platform != "" {
		db = db.Where("platform.code LIKE ? ESCAPE '\\'", escapeLikePrefix(query.Platform))
	}
	if query.PhoneToHMAC != nil {
		db = db.Where(Table+".to_phone_hmac = ?", *query.PhoneToHMAC)
	}
	if query.Scene != "" {
		db = db.Where(Table+".scene = ?", query.Scene)
	}
	if query.Status != "" {
		db = db.Where(Table+".status = ?", query.Status)
	}
	if query.From != nil {
		db = db.Where(Table+".created_at >= ?", *query.From)
	}
	if query.To != nil {
		db = db.Where(Table+".created_at <= ?", *query.To)
	}
	var total int64
	if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ListRow
	if err := db.Session(&gorm.Session{}).
		Order(Table + ".created_at DESC, " + Table + ".id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (ListRow, error) {
	var row ListRow
	if err := r.baseQuery(ctx).Session(&gorm.Session{}).
		Where(Table+".id = ?", id).
		Take(&row).Error; err != nil {
		return ListRow{}, err
	}
	return row, nil
}

func escapeLikePrefix(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value) + "%"
}
