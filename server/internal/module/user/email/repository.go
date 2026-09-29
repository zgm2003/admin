package email

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Current(ctx context.Context, userID int64) (Current, error) {
	var row account.User
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).Take(&row).Error; err != nil {
		return Current{}, fmt.Errorf("find current email identity: %w", err)
	}
	var value *string
	if row.Email != "" {
		value = &row.Email
	}
	return Current{UserID: row.ID, Email: value, IsEnabled: row.IsEnabled == yesno.Yes}, nil
}

func (r *Repository) EmailInUse(ctx context.Context, userID int64, value string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&account.User{}).
		Where("lower(email) = lower(?) AND id <> ? AND email <> '' AND deleted_at IS NULL", value, userID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check email ownership: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) Change(ctx context.Context, input ChangeInput) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current account.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", input.UserID).Take(&current).Error; err != nil {
			return fmt.Errorf("lock account for email change: %w", err)
		}
		if current.Email != input.OldEmail {
			return ErrCurrentEmailChanged
		}
		result := tx.Model(&account.User{}).Where("id = ? AND deleted_at IS NULL", input.UserID).
			Updates(map[string]any{"email": input.NewEmail, "updated_at": input.Now.UTC()})
		if result.Error != nil {
			return mapWriteError(result.Error)
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Create(&ChangeLog{UserID: input.UserID, PlatformID: input.PlatformID, Action: input.Action,
			OldEmailHint: input.OldHint, OldEmailHMAC: input.OldHMAC, NewEmailHint: input.NewHint, NewEmailHMAC: input.NewHMAC,
			OldEmail: nullableEmail(input.OldEmail), NewEmail: input.NewEmail,
			CreatedAt: input.Now.UTC(), UpdatedAt: input.Now.UTC()}).Error; err != nil {
			return fmt.Errorf("append email change audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("change email transaction: %w", err)
	}
	return nil
}

func (r *Repository) ListChangeLogs(ctx context.Context, userID int64, page, pageSize int) (ChangeLogPage, error) {
	var total int64
	base := r.db.WithContext(ctx).Table("user_email_change_log AS log").Where("log.user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return ChangeLogPage{}, fmt.Errorf("count email change logs: %w", err)
	}
	rows := make([]ChangeLogItem, 0, pageSize)
	if err := base.Select("log.id, log.action, log.old_email, log.new_email, platform.code AS platform, log.created_at").
		Joins("JOIN permission_auth_platform AS platform ON platform.id = log.platform_id").
		Order("log.created_at DESC, log.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return ChangeLogPage{}, fmt.Errorf("list email change logs: %w", err)
	}
	return ChangeLogPage{List: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

func nullableEmail(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func mapWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "ux_user_account_email_active" {
		return ErrEmailConflict
	}
	return fmt.Errorf("update account email: %w", err)
}
