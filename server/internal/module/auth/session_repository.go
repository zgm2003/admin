package auth

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type SessionCreate struct {
	UserID           int64
	RefreshTokenHash string
	ClientIP         string
	UserAgent        string
	RefreshExpiresAt time.Time
}

type Identity struct {
	UserID    int64
	SessionID int64
	Version   int64
}

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateReplacingActive(ctx context.Context, input SessionCreate, now time.Time) (Session, error) {
	var created Session
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedUserID int64
		lockResult := tx.Raw(`
			SELECT app_user.id
			FROM sys_user AS app_user
			WHERE app_user.id = ?
			  AND app_user.deleted_at IS NULL
			  AND app_user.is_enabled = ?
			  AND EXISTS (
				SELECT 1
				FROM sys_user_role AS user_role
				JOIN sys_role AS app_role
				  ON app_role.id = user_role.role_id
				 AND app_role.deleted_at IS NULL
				 AND app_role.is_enabled = ?
				WHERE user_role.user_id = app_user.id
				  AND user_role.deleted_at IS NULL
			  )
			FOR UPDATE`, input.UserID, yesno.Yes, yesno.Yes).Scan(&lockedUserID)
		if lockResult.Error != nil {
			return fmt.Errorf("lock session user: %w", lockResult.Error)
		}
		if lockResult.RowsAffected != 1 {
			return fmt.Errorf("lock session user: %w", gorm.ErrRecordNotFound)
		}

		if err := tx.Model(&Session{}).
			Where("user_id = ? AND revoked_at IS NULL", input.UserID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("revoke replaced sessions: %w", err)
		}
		created = Session{
			UserID:           input.UserID,
			RefreshTokenHash: input.RefreshTokenHash,
			Version:          1,
			ClientIP:         input.ClientIP,
			UserAgent:        input.UserAgent,
			RefreshExpiresAt: input.RefreshExpiresAt,
		}
		if err := tx.Create(&created).Error; err != nil {
			return fmt.Errorf("create current session: %w", err)
		}
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	return created, nil
}

func (r *SessionRepository) FindActiveIdentity(ctx context.Context, sessionID, version int64, now time.Time) (Identity, error) {
	var identity Identity
	result := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id AS user_id, session.id AS session_id, session.version
		FROM sys_user_session AS session
		JOIN sys_user AS app_user
		  ON app_user.id = session.user_id
		 AND app_user.deleted_at IS NULL
		 AND app_user.is_enabled = ?
		WHERE session.id = ?
		  AND session.version = ?
		  AND session.revoked_at IS NULL
		  AND session.refresh_expires_at > ?
		  AND EXISTS (
			SELECT 1
			FROM sys_user_role AS user_role
			JOIN sys_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE user_role.user_id = app_user.id
			  AND user_role.deleted_at IS NULL
		  )`, yesno.Yes, sessionID, version, now, yesno.Yes).Scan(&identity)
	if result.Error != nil {
		return Identity{}, fmt.Errorf("find active session identity: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Identity{}, fmt.Errorf("find active session identity: %w", gorm.ErrRecordNotFound)
	}
	return identity, nil
}

func (r *SessionRepository) FindCurrentByUser(ctx context.Context, userID int64, now time.Time) (Session, error) {
	return r.findActiveSession(ctx, `session.user_id = ?`, userID, now)
}

func (r *SessionRepository) FindByRefreshHash(ctx context.Context, hash string, now time.Time) (Session, error) {
	return r.findActiveSession(ctx, `session.refresh_token_hash = ?`, hash, now)
}

func (r *SessionRepository) findActiveSession(ctx context.Context, predicate string, value any, now time.Time) (Session, error) {
	query := fmt.Sprintf(`
		SELECT session.*
		FROM sys_user_session AS session
		JOIN sys_user AS app_user
		  ON app_user.id = session.user_id
		 AND app_user.deleted_at IS NULL
		 AND app_user.is_enabled = ?
		WHERE %s
		  AND session.revoked_at IS NULL
		  AND session.refresh_expires_at > ?
		  AND EXISTS (
			SELECT 1
			FROM sys_user_role AS user_role
			JOIN sys_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE user_role.user_id = app_user.id
			  AND user_role.deleted_at IS NULL
		  )`, predicate)
	var session Session
	result := r.db.WithContext(ctx).Raw(query, yesno.Yes, value, now, yesno.Yes).Scan(&session)
	if result.Error != nil {
		return Session{}, fmt.Errorf("find active session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Session{}, fmt.Errorf("find active session: %w", gorm.ErrRecordNotFound)
	}
	return session, nil
}

func (r *SessionRepository) RotateByRefreshHash(
	ctx context.Context,
	sessionID int64,
	oldHash string,
	newHash string,
	now time.Time,
	clientIP string,
	userAgent string,
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
		  AND session.refresh_token_hash = ?
		  AND session.revoked_at IS NULL
		  AND session.refresh_expires_at > ?
		  AND EXISTS (
			SELECT 1
			FROM sys_user AS app_user
			WHERE app_user.id = session.user_id
			  AND app_user.deleted_at IS NULL
			  AND app_user.is_enabled = ?
			  AND EXISTS (
				SELECT 1
				FROM sys_user_role AS user_role
				JOIN sys_role AS app_role
				  ON app_role.id = user_role.role_id
				 AND app_role.deleted_at IS NULL
				 AND app_role.is_enabled = ?
				WHERE user_role.user_id = app_user.id
				  AND user_role.deleted_at IS NULL
			  )
		  )
		RETURNING session.*`, newHash, clientIP, userAgent, now, sessionID, oldHash, now, yesno.Yes, yesno.Yes).Scan(&rotated)
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
