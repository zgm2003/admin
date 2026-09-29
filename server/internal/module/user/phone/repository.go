package phone

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
		return Current{}, fmt.Errorf("find current phone identity: %w", err)
	}
	return Current{UserID: row.ID, Phone: row.Phone, IsEnabled: row.IsEnabled == yesno.Yes}, nil
}

func (r *Repository) PhoneInUse(ctx context.Context, userID int64, value string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&account.User{}).
		Where("phone = ? AND id <> ? AND deleted_at IS NULL", value, userID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check phone ownership: %w", err)
	}
	return count > 0, nil
}

// Change locks the account row, verifies the expected current phone and writes
// both the identity change and its audit row in one PostgreSQL transaction.
func (r *Repository) Change(ctx context.Context, input ChangeInput) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current account.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", input.UserID).Take(&current).Error; err != nil {
			return fmt.Errorf("lock account for phone change: %w", err)
		}
		storedPhone := ""
		if current.Phone != nil {
			storedPhone = *current.Phone
		}
		if storedPhone != input.OldPhone {
			return ErrCurrentPhoneChanged
		}
		result := tx.Model(&account.User{}).Where("id = ? AND deleted_at IS NULL", input.UserID).
			Updates(map[string]any{"phone": input.NewPhone, "updated_at": input.Now.UTC()})
		if result.Error != nil {
			return mapWriteError(result.Error)
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		logRow := ChangeLog{
			UserID: input.UserID, PlatformID: input.PlatformID, Action: input.Action,
			OldPhoneHint: input.OldHint, OldPhoneHMAC: input.OldHMAC,
			NewPhoneHint: input.NewHint, NewPhoneHMAC: input.NewHMAC,
			OldPhone: nullablePhone(input.OldPhone), NewPhone: input.NewPhone,
			CreatedAt: input.Now.UTC(), UpdatedAt: input.Now.UTC(),
		}
		if err := tx.Create(&logRow).Error; err != nil {
			return fmt.Errorf("append phone change audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("change phone transaction: %w", err)
	}
	return nil
}

func (r *Repository) ListChangeLogs(ctx context.Context, userID int64, page, pageSize int) (ChangeLogPage, error) {
	var total int64
	base := r.db.WithContext(ctx).Table("user_phone_change_log AS log").Where("log.user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return ChangeLogPage{}, fmt.Errorf("count phone change logs: %w", err)
	}
	rows := make([]ChangeLogItem, 0, pageSize)
	if err := base.Select("log.id, log.action, log.old_phone, log.new_phone, platform.code AS platform, log.created_at").
		Joins("JOIN permission_auth_platform AS platform ON platform.id = log.platform_id").
		Order("log.created_at DESC, log.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return ChangeLogPage{}, fmt.Errorf("list phone change logs: %w", err)
	}
	return ChangeLogPage{List: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

func nullablePhone(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func mapWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "ux_user_account_phone_active" {
		return ErrPhoneConflict
	}
	return fmt.Errorf("update account phone: %w", err)
}
