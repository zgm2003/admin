package session

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateWithinLimit(ctx context.Context, input CreateInput, policy authplatform.Policy, now time.Time) (Record, []Record, error) {
	now = now.UTC()
	var created Record
	revoked := make([]Record, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedUser struct {
			ID        int64
			IsEnabled yesno.Value
			DeletedAt *time.Time
		}
		if err := tx.Unscoped().Table("user_account").Select("id, is_enabled, deleted_at").
			Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.UserID).Take(&lockedUser).Error; err != nil {
			return fmt.Errorf("lock session user: %w", err)
		}
		if lockedUser.IsEnabled != yesno.Yes || lockedUser.DeletedAt != nil {
			return fmt.Errorf("lock session user: %w", gorm.ErrRecordNotFound)
		}

		var lockedPlatform authplatform.Platform
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", input.Platform).Take(&lockedPlatform).Error; err != nil {
			return fmt.Errorf("lock authentication platform for session: %w", err)
		}
		if lockedPlatform.DeletedAt.Valid || lockedPlatform.IsEnabled != yesno.Yes || lockedPlatform.ID != policy.ID || lockedPlatform.PolicyVersion != policy.PolicyVersion {
			return fmt.Errorf("authentication platform changed before session creation: %w", gorm.ErrRecordNotFound)
		}

		created = Record{
			UserID: input.UserID, PlatformID: lockedPlatform.ID, DeviceID: input.DeviceID,
			RefreshTokenHash: input.RefreshTokenHash, Version: 1, ClientIP: input.ClientIP,
			UserAgent: input.UserAgent, RefreshExpiresAt: input.RefreshExpiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
			Platform: lockedPlatform.Code,
		}
		if err := tx.Omit("Platform").Create(&created).Error; err != nil {
			return fmt.Errorf("create session within limit: %w", err)
		}
		if lockedPlatform.MaxSessions == 0 {
			return nil
		}

		active := make([]Session, 0, int(lockedPlatform.MaxSessions)+1)
		if err := tx.Where("user_id = ? AND platform_id = ? AND revoked_at IS NULL AND refresh_expires_at > ?", input.UserID, lockedPlatform.ID, now).
			Order("created_at DESC, id DESC").Find(&active).Error; err != nil {
			return fmt.Errorf("list active platform sessions: %w", err)
		}
		if len(active) <= int(lockedPlatform.MaxSessions) {
			return nil
		}
		for _, value := range active[int(lockedPlatform.MaxSessions):] {
			value.RevokedAt = &now
			value.UpdatedAt = now
			value.Platform = lockedPlatform.Code
			revoked = append(revoked, value)
		}
		ids := make([]int64, len(revoked))
		for index := range revoked {
			ids[index] = revoked[index].ID
		}
		if err := tx.Model(&Session{}).Where("id IN ? AND revoked_at IS NULL", ids).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("revoke excess platform sessions: %w", err)
		}
		return nil
	})
	if err != nil {
		return Record{}, nil, err
	}
	return created, revoked, nil
}

type authorityRow struct {
	Session
	Platform      string
	UserIsEnabled yesno.Value
	UserDeleted   bool
}

func (r *Repository) FindAuthoritative(ctx context.Context, userID, sessionID int64, platform string, version int64, now time.Time) (Authority, error) {
	var row authorityRow
	result := r.db.WithContext(ctx).Unscoped().Raw(`
		SELECT session.*, platform.code AS platform,
			app_user.is_enabled AS user_is_enabled,
			(app_user.deleted_at IS NOT NULL) AS user_deleted
		FROM user_session AS session
		JOIN auth_platform AS platform ON platform.id = session.platform_id
		JOIN user_account AS app_user ON app_user.id = session.user_id
		WHERE session.id = ? AND session.user_id = ? AND platform.code = ? AND session.version = ?
		  AND session.revoked_at IS NULL AND session.refresh_expires_at > ?`,
		sessionID, userID, platform, version, now.UTC()).Scan(&row)
	if result.Error != nil {
		return Authority{}, fmt.Errorf("find authoritative session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Authority{}, fmt.Errorf("find authoritative session: %w", gorm.ErrRecordNotFound)
	}
	return authorityFromRow(row), nil
}

func (r *Repository) FindByRefreshHash(ctx context.Context, platform, hash string, now time.Time) (Authority, error) {
	var row authorityRow
	result := r.db.WithContext(ctx).Unscoped().Raw(`
		SELECT session.*, platform.code AS platform,
			app_user.is_enabled AS user_is_enabled,
			(app_user.deleted_at IS NOT NULL) AS user_deleted
		FROM user_session AS session
		JOIN auth_platform AS platform ON platform.id = session.platform_id
		JOIN user_account AS app_user ON app_user.id = session.user_id
		WHERE platform.code = ? AND session.refresh_token_hash = ?
		  AND session.revoked_at IS NULL AND session.refresh_expires_at > ?`, platform, hash, now.UTC()).Scan(&row)
	if result.Error != nil {
		return Authority{}, fmt.Errorf("find session by Refresh Token hash: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Authority{}, fmt.Errorf("find session by Refresh Token hash: %w", gorm.ErrRecordNotFound)
	}
	return authorityFromRow(row), nil
}

func authorityFromRow(row authorityRow) Authority {
	row.Session.Platform = row.Platform
	return Authority{
		Session: row.Session,
		UserID:  row.UserID, UserIsEnabled: row.UserIsEnabled,
		UserDeleted: row.UserDeleted,
	}
}

func (r *Repository) RotateByRefreshHash(ctx context.Context, sessionID int64, platform, oldHash, newHash string, now time.Time, client authclient.Client) (Record, bool, error) {
	var row authorityRow
	result := r.db.WithContext(ctx).Raw(`
		UPDATE user_session AS session
		SET refresh_token_hash = ?, version = session.version + 1,
			client_ip = ?, user_agent = ?, updated_at = ?
		FROM auth_platform AS platform
		WHERE session.id = ? AND session.platform_id = platform.id AND platform.code = ?
		  AND session.refresh_token_hash = ? AND session.revoked_at IS NULL
		  AND session.refresh_expires_at > ?
		  AND EXISTS (
			SELECT 1 FROM user_account AS app_user
			WHERE app_user.id = session.user_id AND app_user.deleted_at IS NULL AND app_user.is_enabled = ?
		  )
		RETURNING session.*, platform.code AS platform`, newHash, client.ClientIP, client.UserAgent, now.UTC(), sessionID, platform, oldHash, now.UTC(), yesno.Yes).Scan(&row)
	if result.Error != nil {
		return Record{}, false, fmt.Errorf("rotate Refresh Token hash: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return Record{}, false, nil
	}
	if result.RowsAffected != 1 {
		return Record{}, false, fmt.Errorf("rotate Refresh Token hash affected %d sessions", result.RowsAffected)
	}
	row.Session.Platform = row.Platform
	return row.Session, true, nil
}

func (r *Repository) Revoke(ctx context.Context, sessionID int64, now time.Time) error {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE user_session
		SET revoked_at = COALESCE(revoked_at, ?),
			updated_at = CASE WHEN revoked_at IS NULL THEN ? ELSE updated_at END
		WHERE id = ?`, now.UTC(), now.UTC(), sessionID)
	if result.Error != nil {
		return fmt.Errorf("revoke session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("revoke session: %w", gorm.ErrRecordNotFound)
	}
	return nil
}
