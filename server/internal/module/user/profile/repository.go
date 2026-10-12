package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/auth/login"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
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

func (r *Repository) Update(ctx context.Context, userID int64, username string, birthday *time.Time, gender int16, avatar string, updatedAt time.Time) (Value, error) {
	var updated Value
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("user_account").Where("id = ? AND deleted_at IS NULL AND is_enabled = ?", userID, yesno.Yes).
			Updates(map[string]any{"username": username, "updated_at": updatedAt.UTC()})
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

func (r *Repository) UpdatePreservingAvatar(ctx context.Context, userID int64, username string, birthday *time.Time, gender int16, expectedAvatar string, now time.Time, identities ...auth.Identity) (Value, error) {
	var updated Value
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var id int64
		if err := tx.Raw("SELECT id FROM user_account WHERE id=? AND deleted_at IS NULL AND is_enabled=1 FOR UPDATE", userID).Scan(&id).Error; err != nil {
			return err
		}
		if id == 0 {
			return gorm.ErrRecordNotFound
		}
		current, err := NewRepository(tx).Find(ctx, userID)
		if err != nil {
			return err
		}
		if expectedAvatar != "" && current.Avatar != expectedAvatar {
			return apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("avatar changed or requires an owned file selection"))
		}
		updated, err = NewRepository(tx).Update(ctx, userID, username, birthday, gender, expectedAvatar, now)
		if err != nil {
			return err
		}
		if expectedAvatar == "" && current.Avatar != "" && len(identities) > 0 {
			return storagefile.RetireAvatarTx(ctx, tx, identities[0], current.Avatar, now)
		}
		return nil
	})
	return updated, err
}

// UpdateWithAvatar adopts an independently copied avatar in the same transaction
// as the profile write. The user row serializes competing profile updates.
func (r *Repository) UpdateWithAvatar(ctx context.Context, identity auth.Identity, username string, birthday *time.Time, gender int16, copied storagefile.AvatarCopy, requestKey string, now time.Time) (Value, error) {
	var updated Value
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user struct{ ID int64 }
		if err := tx.Raw("SELECT id FROM user_account WHERE id=? AND deleted_at IS NULL AND is_enabled=1 FOR UPDATE", identity.UserID).Scan(&user).Error; err != nil {
			return err
		}
		if user.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		current, err := NewRepository(tx).Find(ctx, identity.UserID)
		if err != nil {
			return err
		}
		already, err := storagefile.AdoptAvatarTx(ctx, tx, identity, copied.ID, copied.ObjectKey, requestKey, now)
		if err != nil {
			return err
		}
		if already {
			updated = current
			return nil
		}
		updated, err = NewRepository(tx).Update(ctx, identity.UserID, username, birthday, gender, copied.ObjectKey, now)
		if err != nil {
			return err
		}
		if current.Avatar != "" && current.Avatar != copied.ObjectKey {
			return storagefile.RetireAvatarTx(ctx, tx, identity, current.Avatar, now)
		}
		return nil
	})
	return updated, err
}
