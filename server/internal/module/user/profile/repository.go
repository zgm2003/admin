package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Value struct {
	UserID    int64
	Username  string
	Email     string
	Phone     *string
	Avatar    string
	Birthday  *time.Time
	Gender    int16
	UpdatedAt time.Time
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Find(ctx context.Context, userID int64) (Value, error) {
	var value Value
	result := r.db.WithContext(ctx).Table("user_account AS app_user").
		Select("app_user.id AS user_id, app_user.username, app_user.email, app_user.phone, COALESCE(profile.avatar, '') AS avatar, profile.birthday, COALESCE(profile.gender, 0) AS gender, COALESCE(profile.updated_at, app_user.updated_at) AS updated_at").
		Joins("LEFT JOIN user_profile AS profile ON profile.user_id = app_user.id").
		Where("app_user.id = ? AND app_user.deleted_at IS NULL AND app_user.is_enabled = ?", userID, yesno.Yes).
		Take(&value)
	if result.Error != nil {
		return Value{}, fmt.Errorf("find personal profile: %w", result.Error)
	}
	return value, nil
}

func (r *Repository) Update(ctx context.Context, userID int64, username string, phone *string, birthday *time.Time, gender int16, avatar string, updatedAt time.Time) (Value, error) {
	var updated Value
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("user_account").Where("id = ? AND deleted_at IS NULL AND is_enabled = ?", userID, yesno.Yes).
			Updates(map[string]any{"username": username, "phone": phone, "updated_at": updatedAt.UTC()})
		if result.Error != nil {
			return mapAccountWriteError(result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update personal account %d: %w", userID, gorm.ErrRecordNotFound)
		}
		if err := tx.Exec(`
			INSERT INTO user_profile (user_id, avatar, birthday, gender, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (user_id) DO UPDATE
			SET avatar = EXCLUDED.avatar,
				birthday = EXCLUDED.birthday,
				gender = EXCLUDED.gender,
				updated_at = EXCLUDED.updated_at`,
			userID, avatar, birthday, gender, updatedAt.UTC(), updatedAt.UTC()).Error; err != nil {
			return fmt.Errorf("upsert personal profile: %w", err)
		}
		if err := tx.Table("user_account AS app_user").
			Select("app_user.id AS user_id, app_user.username, app_user.email, app_user.phone, profile.avatar, profile.birthday, profile.gender, profile.updated_at").
			Joins("JOIN user_profile AS profile ON profile.user_id = app_user.id").
			Where("app_user.id = ?", userID).Take(&updated).Error; err != nil {
			return fmt.Errorf("read updated personal profile: %w", err)
		}
		return nil
	})
	if err != nil {
		return Value{}, err
	}
	return updated, nil
}

func mapAccountWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_user_account_username_active":
			return fmt.Errorf("update personal account: %w", account.ErrUsernameConflict)
		case "ux_user_account_phone_active":
			return fmt.Errorf("update personal account: %w", account.ErrPhoneConflict)
		}
	}
	return fmt.Errorf("update personal account: %w", err)
}
