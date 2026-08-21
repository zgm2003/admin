package auth

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

type SessionCreate struct {
	UserID           int64
	Platform         string
	DeviceID         string
	RefreshTokenHash string
	ClientIP         string
	UserAgent        string
	RefreshExpiresAt time.Time
}

type SessionAuthority struct {
	Session       Session
	UserID        int64
	UserIsEnabled yesno.Value
	UserDeleted   bool
}

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateWithinLimit(ctx context.Context, input SessionCreate, policy authplatform.Policy, now time.Time) (Session, []Session, error) {
	now = now.UTC()
	var created Session
	revoked := make([]Session, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedUser userSessionStatus
		if err := tx.Unscoped().Table("sys_user").Select("id, is_enabled, deleted_at").
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

		created = Session{
			UserID: input.UserID, Platform: input.Platform, DeviceID: input.DeviceID,
			RefreshTokenHash: input.RefreshTokenHash, Version: 1, ClientIP: input.ClientIP,
			UserAgent: input.UserAgent, RefreshExpiresAt: input.RefreshExpiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return fmt.Errorf("create session within limit: %w", err)
		}
		if lockedPlatform.MaxSessions == 0 {
			return nil
		}

		active := make([]Session, 0, int(lockedPlatform.MaxSessions)+1)
		if err := tx.Where("user_id = ? AND platform = ? AND revoked_at IS NULL AND refresh_expires_at > ?", input.UserID, input.Platform, now).
			Order("created_at DESC, id DESC").Find(&active).Error; err != nil {
			return fmt.Errorf("list active platform sessions: %w", err)
		}
		if len(active) <= int(lockedPlatform.MaxSessions) {
			return nil
		}
		revoked = append(revoked, active[int(lockedPlatform.MaxSessions):]...)
		ids := make([]int64, len(revoked))
		for index := range revoked {
			ids[index] = revoked[index].ID
			revoked[index].RevokedAt = &now
			revoked[index].UpdatedAt = now
		}
		if err := tx.Model(&Session{}).Where("id IN ? AND revoked_at IS NULL", ids).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("revoke excess platform sessions: %w", err)
		}
		return nil
	})
	if err != nil {
		return Session{}, nil, err
	}
	return created, revoked, nil
}

type userSessionStatus struct {
	ID        int64
	IsEnabled yesno.Value
	DeletedAt *time.Time
}

type sessionAuthorityRow struct {
	ID               int64
	UserID           int64
	Platform         string
	DeviceID         string
	RefreshTokenHash string
	Version          int64
	ClientIP         string
	UserAgent        string
	RefreshExpiresAt time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	UserIsEnabled    yesno.Value
	UserDeleted      bool
}

func (r *SessionRepository) FindAuthoritative(ctx context.Context, token TokenIdentity, now time.Time) (SessionAuthority, error) {
	var row sessionAuthorityRow
	result := r.db.WithContext(ctx).Unscoped().Raw(`
		SELECT session.id, session.user_id, session.platform, session.device_id,
			session.refresh_token_hash, session.version, session.client_ip, session.user_agent,
			session.refresh_expires_at, session.revoked_at, session.created_at, session.updated_at,
			app_user.is_enabled AS user_is_enabled,
			(app_user.deleted_at IS NOT NULL) AS user_deleted
		FROM sys_user_session AS session
		JOIN sys_user AS app_user ON app_user.id = session.user_id
		WHERE session.id = ? AND session.user_id = ? AND session.platform = ? AND session.version = ?
		  AND session.revoked_at IS NULL AND session.refresh_expires_at > ?`,
		token.SessionID, token.UserID, token.Platform, token.Version, now.UTC()).Scan(&row)
	if result.Error != nil {
		return SessionAuthority{}, fmt.Errorf("find authoritative session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return SessionAuthority{}, fmt.Errorf("find authoritative session: %w", gorm.ErrRecordNotFound)
	}
	return authorityFromRow(row), nil
}

func authorityFromRow(row sessionAuthorityRow) SessionAuthority {
	return SessionAuthority{
		Session: Session{
			ID: row.ID, UserID: row.UserID, Platform: row.Platform, DeviceID: row.DeviceID,
			RefreshTokenHash: row.RefreshTokenHash, Version: row.Version, ClientIP: row.ClientIP,
			UserAgent: row.UserAgent, RefreshExpiresAt: row.RefreshExpiresAt, RevokedAt: row.RevokedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
		UserID: row.UserID, UserIsEnabled: row.UserIsEnabled, UserDeleted: row.UserDeleted,
	}
}

func (r *SessionRepository) FindByRefreshHash(ctx context.Context, platform, hash string, now time.Time) (SessionAuthority, error) {
	var row sessionAuthorityRow
	result := r.db.WithContext(ctx).Unscoped().Raw(`
		SELECT session.id, session.user_id, session.platform, session.device_id,
			session.refresh_token_hash, session.version, session.client_ip, session.user_agent,
			session.refresh_expires_at, session.revoked_at, session.created_at, session.updated_at,
			app_user.is_enabled AS user_is_enabled,
			(app_user.deleted_at IS NOT NULL) AS user_deleted
		FROM sys_user_session AS session
		JOIN sys_user AS app_user ON app_user.id = session.user_id
		WHERE session.platform = ? AND session.refresh_token_hash = ?
		  AND session.revoked_at IS NULL AND session.refresh_expires_at > ?`, platform, hash, now.UTC()).Scan(&row)
	if result.Error != nil {
		return SessionAuthority{}, fmt.Errorf("find session by Refresh Token hash: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return SessionAuthority{}, fmt.Errorf("find session by Refresh Token hash: %w", gorm.ErrRecordNotFound)
	}
	return authorityFromRow(row), nil
}

func (r *SessionRepository) RotateByRefreshHash(
	ctx context.Context,
	sessionID int64,
	platform string,
	oldHash string,
	newHash string,
	now time.Time,
	client authclient.Client,
) (Session, bool, error) {
	var rotated Session
	result := r.db.WithContext(ctx).Raw(`
		UPDATE sys_user_session AS session
		SET refresh_token_hash = ?,
			version = session.version + 1,
			client_ip = ?,
			user_agent = ?,
			updated_at = ?
		WHERE session.id = ?
		  AND session.platform = ?
		  AND session.refresh_token_hash = ?
		  AND session.revoked_at IS NULL
		  AND session.refresh_expires_at > ?
		  AND EXISTS (
			SELECT 1
			FROM sys_user AS app_user
			WHERE app_user.id = session.user_id
			  AND app_user.deleted_at IS NULL
			  AND app_user.is_enabled = ?
		  )
		RETURNING session.*`, newHash, client.ClientIP, client.UserAgent, now, sessionID, platform, oldHash, now, yesno.Yes).Scan(&rotated)
	if result.Error != nil {
		return Session{}, false, fmt.Errorf("rotate Refresh Token hash: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return Session{}, false, nil
	}
	if result.RowsAffected != 1 {
		return Session{}, false, fmt.Errorf("rotate Refresh Token hash affected %d sessions", result.RowsAffected)
	}
	return rotated, true, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, sessionID int64, now time.Time) error {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE sys_user_session
		SET revoked_at = COALESCE(revoked_at, ?),
			updated_at = CASE WHEN revoked_at IS NULL THEN ? ELSE updated_at END
		WHERE id = ?`, now, now, sessionID)
	if result.Error != nil {
		return fmt.Errorf("revoke session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("revoke session: %w", gorm.ErrRecordNotFound)
	}
	return nil
}
