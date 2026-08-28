package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/role"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUsernameConflict = errors.New("active username already exists")
	ErrEmailConflict    = errors.New("active email already exists")
	ErrPhoneConflict    = errors.New("active phone already exists")
	ErrUserDataInvalid  = errors.New("user or user-role data is invalid")
)

type CreateInput struct {
	Username     string
	Email        string
	PasswordHash string
	RoleID       int64
}

type Credential struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	IsEnabled    yesno.Value
}

type Current struct {
	ID       int64
	Username string
	Email    string
	Phone    *string
}

type RevokedSessionRef struct {
	ID       int64
	UserID   int64
	Platform string
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	}); err != nil {
		return fmt.Errorf("user transaction: %w", err)
	}
	return nil
}

func (r *Repository) LockUserWriteTable(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec("LOCK TABLE user_account IN ROW EXCLUSIVE MODE").Error; err != nil {
		return fmt.Errorf("lock user write table: %w", err)
	}
	return nil
}

func (r *Repository) FindAccessVersion(ctx context.Context, userID int64) (accessstate.Version, error) {
	var version accessstate.Version
	result := r.db.WithContext(ctx).Raw(`
		SELECT user_id, version
		FROM rbac_access_version
		WHERE user_id = ?`, userID).Scan(&version)
	if result.Error != nil {
		return accessstate.Version{}, fmt.Errorf("find user access version: %w", result.Error)
	}
	if result.RowsAffected != 1 || version.UserID != userID || version.Version < 1 {
		return accessstate.Version{}, fmt.Errorf("find user access version: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func (r *Repository) LockAccessVersion(ctx context.Context, userID int64) (int64, error) {
	var version int64
	result := r.db.WithContext(ctx).Raw(`
		SELECT version
		FROM rbac_access_version
		WHERE user_id = ?
		FOR UPDATE`, userID).Scan(&version)
	if result.Error != nil {
		return 0, fmt.Errorf("lock user access version: %w", result.Error)
	}
	if result.RowsAffected != 1 || version < 1 {
		return 0, fmt.Errorf("lock user access version: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func (r *Repository) IncrementAccessVersion(ctx context.Context, userID int64, now time.Time) (int64, error) {
	var version int64
	result := r.db.WithContext(ctx).Raw(`
		UPDATE rbac_access_version
		SET version = version + 1, updated_at = ?
		WHERE user_id = ?
		RETURNING version`, now.UTC(), userID).Scan(&version)
	if result.Error != nil {
		return 0, fmt.Errorf("increment user access version: %w", result.Error)
	}
	if result.RowsAffected != 1 || version < 2 {
		return 0, fmt.Errorf("increment user access version: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func (r *Repository) FindActiveSessionPlatforms(ctx context.Context, userID int64) ([]string, error) {
	platforms := make([]string, 0)
	if err := r.db.WithContext(ctx).Table("auth_session").Distinct("platform").
		Where("user_id = ? AND revoked_at IS NULL", userID).Order("platform").Pluck("platform", &platforms).Error; err != nil {
		return nil, fmt.Errorf("find active user session platforms: %w", err)
	}
	return platforms, nil
}

func (r *Repository) LockSuperAdminRole(ctx context.Context) (role.Role, error) {
	roles := make([]role.Role, 0, 2)
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ? AND is_enabled = ?", role.CodeSuperAdmin, yesno.Yes).
		Limit(2).Find(&roles).Error; err != nil {
		return role.Role{}, fmt.Errorf("lock super administrator role: %w", err)
	}
	if len(roles) != 1 {
		return role.Role{}, fmt.Errorf("expected one enabled super administrator role, found %d: %w", len(roles), ErrUserDataInvalid)
	}
	return roles[0], nil
}

func (r *Repository) LockUser(ctx context.Context, userID int64) (User, error) {
	var found User
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).Take(&found).Error; err != nil {
		return User{}, fmt.Errorf("lock user %d: %w", userID, err)
	}
	return found, nil
}

func (r *Repository) LockUserUnscoped(ctx context.Context, userID int64) (User, error) {
	var found User
	if err := r.db.WithContext(ctx).Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).Take(&found).Error; err != nil {
		return User{}, fmt.Errorf("lock user unscoped %d: %w", userID, err)
	}
	return found, nil
}

func (r *Repository) IsEffectiveSuperAdmin(ctx context.Context, userID, superAdminRoleID int64) (bool, error) {
	var exists bool
	if err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM user_account AS app_user
			JOIN rbac_user_role AS user_role
			  ON user_role.user_id = app_user.id
			 AND user_role.deleted_at IS NULL
			JOIN rbac_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE app_user.id = ?
			  AND app_user.deleted_at IS NULL
			  AND app_user.is_enabled = ?
			  AND app_role.id = ?
		)`, yesno.Yes, userID, yesno.Yes, superAdminRoleID).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check effective super administrator: %w", err)
	}
	return exists, nil
}

func (r *Repository) HasActiveRole(ctx context.Context, userID, roleID int64) (bool, error) {
	var exists bool
	if err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 FROM rbac_user_role
			WHERE user_id = ? AND role_id = ? AND deleted_at IS NULL
		)`, userID, roleID).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check active user role: %w", err)
	}
	return exists, nil
}

func (r *Repository) FindUser(ctx context.Context, userID int64) (User, error) {
	var found User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).Take(&found).Error; err != nil {
		return User{}, fmt.Errorf("find user %d: %w", userID, err)
	}
	return found, nil
}

func (r *Repository) FindUserUnscoped(ctx context.Context, userID int64) (User, error) {
	var found User
	if err := r.db.WithContext(ctx).Unscoped().Where("id = ?", userID).Take(&found).Error; err != nil {
		return User{}, fmt.Errorf("find user unscoped: %w", err)
	}
	return found, nil
}

func (r *Repository) FindUserRoles(ctx context.Context, userID int64) ([]role.UserRole, error) {
	relations := make([]role.UserRole, 0)
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("role_id ASC, id ASC").Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("find user roles: %w", err)
	}
	return relations, nil
}

func (r *Repository) LockRoles(ctx context.Context) ([]role.Role, error) {
	roles := make([]role.Role, 0)
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("lock user role options: %w", err)
	}
	return roles, nil
}

func (r *Repository) LockUserRoles(ctx context.Context, userID int64) ([]role.UserRole, error) {
	relations := make([]role.UserRole, 0)
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).Order("role_id ASC, id ASC").Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("lock user roles: %w", err)
	}
	return relations, nil
}

func (r *Repository) CountEffectiveSuperAdmins(ctx context.Context, superAdminRoleID int64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("user_account AS app_user").
		Joins("JOIN rbac_user_role AS user_role ON user_role.user_id = app_user.id AND user_role.deleted_at IS NULL").
		Joins("JOIN rbac_role AS app_role ON app_role.id = user_role.role_id AND app_role.deleted_at IS NULL AND app_role.is_enabled = ?", yesno.Yes).
		Where("app_user.deleted_at IS NULL AND app_user.is_enabled = ? AND app_role.id = ?", yesno.Yes, superAdminRoleID).
		Distinct("app_user.id").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count effective super administrators: %w", err)
	}
	return count, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID int64, username string, phone *string, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"username": username, "phone": phone, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return mapUserWriteError("update profile", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update profile %d: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, userID int64, value yesno.Value, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"is_enabled": value, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("update user status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update user status %d: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) TouchUser(ctx context.Context, userID int64, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Update("updated_at", updatedAt.UTC())
	if result.Error != nil {
		return fmt.Errorf("touch user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("touch user %d: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) SoftDeleteUserRoleIDs(ctx context.Context, ids []int64, deletedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Table("rbac_user_role").Where("id IN ? AND deleted_at IS NULL", ids).Updates(map[string]any{
		"updated_at": deletedAt.UTC(), "deleted_at": deletedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("soft delete user roles: %w", err)
	}
	return nil
}

func (r *Repository) CreateUserRoles(ctx context.Context, values []role.UserRole) error {
	if len(values) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&values).Error; err != nil {
		return fmt.Errorf("create user roles: %w", err)
	}
	return nil
}

func (r *Repository) RevokeActiveSessions(ctx context.Context, userID int64, revokedAt time.Time) error {
	if err := r.db.WithContext(ctx).Exec(`
		UPDATE auth_session
		SET revoked_at = ?, updated_at = ?
		WHERE user_id = ? AND revoked_at IS NULL`, revokedAt.UTC(), revokedAt.UTC(), userID).Error; err != nil {
		return fmt.Errorf("revoke active user sessions: %w", err)
	}
	return nil
}

func (r *Repository) SoftDeleteUser(ctx context.Context, userID int64, deletedAt time.Time) error {
	result := r.db.WithContext(ctx).Table("user_account").Where("id = ? AND deleted_at IS NULL", userID).Updates(map[string]any{
		"updated_at": deletedAt.UTC(), "deleted_at": deletedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("soft delete user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("soft delete user %d: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) CreateWithRole(ctx context.Context, input CreateInput) (User, error) {
	var created User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`LOCK TABLE user_account IN ROW EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock user writes: %w", err)
		}
		var effectiveRole role.Role
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_enabled = ?", input.RoleID, yesno.Yes).
			Take(&effectiveRole).Error; err != nil {
			return fmt.Errorf("lock enabled role: %w", err)
		}

		now := time.Now().UTC().Truncate(time.Microsecond)
		created = User{
			Username:     input.Username,
			Email:        input.Email,
			PasswordHash: input.PasswordHash,
			IsEnabled:    yesno.Yes,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return mapCreateError(err)
		}
		userRole := role.UserRole{UserID: created.ID, RoleID: effectiveRole.ID, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&userRole).Error; err != nil {
			return fmt.Errorf("create user role relationship: %w", err)
		}
		if err := tx.Exec(`
			INSERT INTO rbac_access_version (user_id, version, created_at, updated_at)
			VALUES (?, 1, ?, ?)`, created.ID, now, now).Error; err != nil {
			return fmt.Errorf("create user access version: %w", err)
		}
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (r *Repository) FindCredentialByEmail(ctx context.Context, email string) (Credential, error) {
	var credential Credential
	if err := r.db.WithContext(ctx).
		Model(&User{}).
		Select("id", "username", "email", "password_hash", "is_enabled").
		Where("email = ?", email).
		Take(&credential).Error; err != nil {
		return Credential{}, fmt.Errorf("find user credential: %w", err)
	}
	return credential, nil
}

func (r *Repository) FindCredentialByID(ctx context.Context, userID int64) (Credential, error) {
	var credential Credential
	result := r.db.WithContext(ctx).Table("user_account").
		Select("id, username, email, password_hash, is_enabled").
		Where("id = ? AND deleted_at IS NULL", userID).Take(&credential)
	if result.Error != nil {
		return Credential{}, fmt.Errorf("find credential by id: %w", result.Error)
	}
	return credential, nil
}

func (r *Repository) ChangePasswordAndRevokeSessions(ctx context.Context, userID int64, passwordHash string, now time.Time) ([]RevokedSessionRef, error) {
	revoked := make([]RevokedSessionRef, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("user_account").Where("id = ? AND deleted_at IS NULL", userID).
			Updates(map[string]any{"password_hash": passwordHash, "updated_at": now.UTC()})
		if result.Error != nil {
			return fmt.Errorf("update password: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update password %d: %w", userID, gorm.ErrRecordNotFound)
		}
		if err := tx.Table("auth_session").Select("id, user_id, platform").
			Where("user_id = ? AND revoked_at IS NULL", userID).Find(&revoked).Error; err != nil {
			return fmt.Errorf("find active sessions for password change: %w", err)
		}
		if err := tx.Table("auth_session").Where("user_id = ? AND revoked_at IS NULL", userID).
			Updates(map[string]any{"revoked_at": now.UTC(), "updated_at": now.UTC()}).Error; err != nil {
			return fmt.Errorf("revoke sessions after password change: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return revoked, nil
}

func (r *Repository) FindCurrent(ctx context.Context, userID int64) (Current, error) {
	var current Current
	result := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id, app_user.username, app_user.email, app_user.phone
		FROM user_account AS app_user
		WHERE app_user.id = ?
		  AND app_user.deleted_at IS NULL
		  AND app_user.is_enabled = ?
		  AND EXISTS (
			SELECT 1
			FROM rbac_user_role AS user_role
			JOIN rbac_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE user_role.user_id = app_user.id
			  AND user_role.deleted_at IS NULL
		  )`, userID, yesno.Yes, yesno.Yes).Scan(&current)
	if result.Error != nil {
		return Current{}, fmt.Errorf("find current user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Current{}, fmt.Errorf("find current user: %w", gorm.ErrRecordNotFound)
	}
	return current, nil
}

func (r *Repository) FindPersonalProfile(ctx context.Context, userID int64) (PersonalProfile, error) {
	var row struct {
		ID        int64
		Username  string
		Email     string
		Phone     *string
		Birthday  *time.Time
		Gender    int16
		UpdatedAt time.Time
	}
	result := r.db.WithContext(ctx).Table("user_account AS app_user").
		Select("app_user.id, app_user.username, app_user.email, app_user.phone, profile.birthday, COALESCE(profile.gender, 0) AS gender, COALESCE(profile.updated_at, app_user.updated_at) AS updated_at").
		Joins("LEFT JOIN user_profile AS profile ON profile.user_id = app_user.id").
		Where("app_user.id = ? AND app_user.deleted_at IS NULL AND app_user.is_enabled = ?", userID, yesno.Yes).Take(&row)
	if result.Error != nil {
		return PersonalProfile{}, fmt.Errorf("find personal profile: %w", result.Error)
	}
	return PersonalProfile{Current: Current{ID: row.ID, Username: row.Username, Email: row.Email, Phone: row.Phone}, Birthday: row.Birthday, Gender: row.Gender, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) UpdatePersonalProfile(ctx context.Context, userID int64, username string, phone *string, birthday *time.Time, gender int16, updatedAt time.Time) (PersonalProfile, error) {
	var updated PersonalProfile
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("user_account").Where("id = ? AND deleted_at IS NULL AND is_enabled = ?", userID, yesno.Yes).
			Updates(map[string]any{"username": username, "phone": phone, "updated_at": updatedAt.UTC()})
		if result.Error != nil {
			return mapUserWriteError("update personal account", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update personal account %d: %w", userID, gorm.ErrRecordNotFound)
		}
		if err := tx.Exec(`
			INSERT INTO user_profile (user_id, birthday, gender, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (user_id) DO UPDATE SET birthday = EXCLUDED.birthday, gender = EXCLUDED.gender, updated_at = EXCLUDED.updated_at`,
			userID, birthday, gender, updatedAt.UTC(), updatedAt.UTC()).Error; err != nil {
			return fmt.Errorf("upsert personal profile: %w", err)
		}
		var row struct {
			ID        int64
			Username  string
			Email     string
			Phone     *string
			Birthday  *time.Time
			Gender    int16
			UpdatedAt time.Time
		}
		if err := tx.Table("user_account AS app_user").
			Select("app_user.id, app_user.username, app_user.email, app_user.phone, profile.birthday, profile.gender, profile.updated_at").
			Joins("JOIN user_profile AS profile ON profile.user_id = app_user.id").
			Where("app_user.id = ?", userID).Take(&row).Error; err != nil {
			return fmt.Errorf("read updated personal profile: %w", err)
		}
		updated = PersonalProfile{Current: Current{ID: row.ID, Username: row.Username, Email: row.Email, Phone: row.Phone}, Birthday: row.Birthday, Gender: row.Gender, UpdatedAt: row.UpdatedAt}
		return nil
	})
	if err != nil {
		return PersonalProfile{}, err
	}
	return updated, nil
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	var total int64
	if err := applyUserListFilter(r.db.WithContext(ctx), query).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]ListItem, error) {
	type userRow struct {
		ID        int64
		Username  string
		Email     string
		Phone     *string
		IsEnabled yesno.Value
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	userRows := make([]userRow, 0)
	result := applyUserListFilter(r.db.WithContext(ctx), query).
		Select("app_user.id, app_user.username, app_user.email, app_user.phone, app_user.is_enabled, app_user.created_at, app_user.updated_at").
		Order("app_user.created_at ASC, app_user.id ASC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Scan(&userRows)
	if result.Error != nil {
		return nil, fmt.Errorf("list users: %w", result.Error)
	}
	if len(userRows) == 0 {
		return make([]ListItem, 0), nil
	}

	items := make([]ListItem, len(userRows))
	userIDs := make([]int64, len(userRows))
	itemByUserID := make(map[int64]*ListItem, len(userRows))
	for index, row := range userRows {
		items[index] = ListItem{
			ID: row.ID, Username: row.Username, Email: row.Email, Phone: row.Phone, IsEnabled: row.IsEnabled,
			Roles: make([]RoleSummary, 0), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		userIDs[index] = items[index].ID
		itemByUserID[items[index].ID] = &items[index]
	}
	type relationshipRow struct {
		UserID         int64
		RelationshipID int64
		RoleID         sql.NullInt64
		Code           sql.NullString
		Name           sql.NullString
		IsEnabled      sql.NullInt16
		RoleDeletedAt  sql.NullTime
	}
	rows := make([]relationshipRow, 0)
	if err := r.db.WithContext(ctx).Table("rbac_user_role AS user_role").
		Select(`user_role.user_id, user_role.id AS relationship_id,
			app_role.id AS role_id, app_role.code, app_role.name,
			app_role.is_enabled, app_role.deleted_at AS role_deleted_at`).
		Joins("LEFT JOIN rbac_role AS app_role ON app_role.id = user_role.role_id").
		Where("user_role.user_id IN ? AND user_role.deleted_at IS NULL", userIDs).
		Order("user_role.user_id ASC, app_role.code ASC, app_role.id ASC, user_role.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	seen := make(map[int64]map[int64]struct{}, len(items))
	for _, row := range rows {
		item := itemByUserID[row.UserID]
		if item == nil || !row.RoleID.Valid || !row.Code.Valid || !row.Name.Valid || !row.IsEnabled.Valid || row.RoleDeletedAt.Valid {
			return nil, fmt.Errorf("invalid relationship %d: %w", row.RelationshipID, ErrUserDataInvalid)
		}
		value := yesno.Value(row.IsEnabled.Int16)
		if row.RoleID.Int64 <= 0 || !yesno.IsValid(value) {
			return nil, fmt.Errorf("invalid role for relationship %d: %w", row.RelationshipID, ErrUserDataInvalid)
		}
		if seen[row.UserID] == nil {
			seen[row.UserID] = make(map[int64]struct{})
		}
		if _, exists := seen[row.UserID][row.RoleID.Int64]; exists {
			return nil, fmt.Errorf("duplicate role relationship for user %d: %w", row.UserID, ErrUserDataInvalid)
		}
		seen[row.UserID][row.RoleID.Int64] = struct{}{}
		item.Roles = append(item.Roles, RoleSummary{ID: row.RoleID.Int64, Code: row.Code.String, Name: row.Name.String, IsEnabled: value})
	}
	for index := range items {
		if len(items[index].Roles) == 0 {
			return nil, fmt.Errorf("user %d has no active role relationship: %w", items[index].ID, ErrUserDataInvalid)
		}
	}
	return items, nil
}

func (r *Repository) FindRoleOptions(ctx context.Context) ([]RoleSummary, error) {
	options := make([]RoleSummary, 0)
	if err := r.db.WithContext(ctx).Table("rbac_role AS app_role").
		Select("app_role.id, app_role.code, app_role.name, app_role.is_enabled").
		Where("app_role.deleted_at IS NULL").
		Order("app_role.code ASC, app_role.id ASC").
		Scan(&options).Error; err != nil {
		return nil, fmt.Errorf("find user role options: %w", err)
	}
	for _, option := range options {
		if option.ID <= 0 || option.Code == "" || option.Name == "" || !yesno.IsValid(option.IsEnabled) {
			return nil, fmt.Errorf("invalid role option %d: %w", option.ID, ErrUserDataInvalid)
		}
	}
	return options, nil
}

func applyUserListFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	db = db.Table("user_account AS app_user").Where("app_user.deleted_at IS NULL")
	if query.Keyword != "" {
		pattern := "%" + escapeUserLike(query.Keyword) + "%"
		db = db.Where(`(
			app_user.username ILIKE ? ESCAPE E'\\'
			OR app_user.email ILIKE ? ESCAPE E'\\'
			OR app_user.phone ILIKE ? ESCAPE E'\\'
		)`, pattern, pattern, pattern)
	}
	if query.IsEnabled != nil {
		db = db.Where("app_user.is_enabled = ?", *query.IsEnabled)
	}
	if query.RoleID != nil {
		db = db.Where(`EXISTS (
			SELECT 1 FROM rbac_user_role AS user_role
			WHERE user_role.user_id = app_user.id
			  AND user_role.role_id = ?
			  AND user_role.deleted_at IS NULL
		)`, *query.RoleID)
	}
	return db
}

func escapeUserLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func mapCreateError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_user_account_username_active":
			return ErrUsernameConflict
		case "ux_user_account_email_active":
			return ErrEmailConflict
		case "ux_user_account_phone_active":
			return ErrPhoneConflict
		}
	}
	return fmt.Errorf("create user: %w", err)
}

func mapUserWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_user_account_username_active":
			return fmt.Errorf("%s: %w", operation, ErrUsernameConflict)
		case "ux_user_account_phone_active":
			return fmt.Errorf("%s: %w", operation, ErrPhoneConflict)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
