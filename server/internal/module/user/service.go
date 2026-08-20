package user

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/role"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type sessionPointerStore interface {
	Delete(context.Context, string) error
}

type Service struct {
	repository *Repository
	pointers   sessionPointerStore
}

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
	RoleID    *int64
}

type RoleSummary struct {
	ID        int64
	Code      string
	Name      string
	IsEnabled yesno.Value
}

type ListItem struct {
	ID        int64
	Username  string
	Email     string
	IsEnabled yesno.Value
	Roles     []RoleSummary
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Summary struct {
	ID        int64
	Username  string
	Email     string
	IsEnabled yesno.Value
}

type Roles struct {
	User    Summary
	Roles   []RoleSummary
	RoleIDs []int64
}

type UpdateInput struct {
	Username string
}

type UpdatedUsername struct {
	ID        int64
	Username  string
	UpdatedAt time.Time
}

func NewService(repository *Repository, pointers sessionPointerStore) *Service {
	return &Service{repository: repository, pointers: pointers}
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[ListItem], error) {
	if s == nil || s.repository == nil {
		return pagination.Result[ListItem]{}, apperror.DependencyUnavailable(fmt.Errorf("list users requires a repository"))
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 ||
		(query.IsEnabled != nil && !yesno.IsValid(*query.IsEnabled)) ||
		(query.RoleID != nil && *query.RoleID <= 0) || utf8.RuneCountInString(query.Keyword) > 254 {
		return pagination.Result[ListItem]{}, apperror.InvalidRequest(fmt.Errorf("user list query is invalid"))
	}
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, mapUserRepositoryError(err)
	}
	items, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, mapUserRepositoryError(err)
	}
	if items == nil {
		items = make([]ListItem, 0)
	}
	return pagination.Result[ListItem]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) RoleOptions(ctx context.Context) ([]RoleSummary, error) {
	if s == nil || s.repository == nil {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("list user role options requires a repository"))
	}
	options, err := s.repository.FindRoleOptions(ctx)
	if err != nil {
		return nil, mapUserRepositoryError(err)
	}
	if options == nil {
		options = make([]RoleSummary, 0)
	}
	return options, nil
}

func (s *Service) Update(ctx context.Context, actorUserID, targetUserID int64, input UpdateInput) (UpdatedUsername, error) {
	if s == nil || s.repository == nil {
		return UpdatedUsername{}, apperror.DependencyUnavailable(fmt.Errorf("update user requires a repository"))
	}
	if actorUserID <= 0 || targetUserID <= 0 {
		return UpdatedUsername{}, apperror.InvalidRequest(fmt.Errorf("actor or target user id is invalid"))
	}
	username, err := NormalizeUsername(input.Username)
	if err != nil {
		return UpdatedUsername{}, apperror.InvalidRequest(err)
	}
	var updated UpdatedUsername
	err = s.repository.Transaction(ctx, func(repository *Repository) error {
		superAdminRole, err := repository.LockSuperAdminRole(ctx)
		if err != nil {
			return err
		}
		target, err := repository.LockUser(ctx, targetUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return userNotFound(err)
			}
			return err
		}
		actorIsSuperAdmin, err := repository.IsEffectiveSuperAdmin(ctx, actorUserID, superAdminRole.ID)
		if err != nil {
			return err
		}
		targetHasSuperAdmin, err := repository.HasActiveRole(ctx, target.ID, superAdminRole.ID)
		if err != nil {
			return err
		}
		if targetHasSuperAdmin && !actorIsSuperAdmin {
			return userSuperAdminProtected(fmt.Errorf("ordinary actor cannot update a super administrator target"))
		}
		if target.Username == username {
			updated = UpdatedUsername{ID: target.ID, Username: target.Username, UpdatedAt: target.UpdatedAt}
			return nil
		}
		updatedAt := time.Now().UTC().Truncate(time.Microsecond)
		if err := repository.UpdateUsername(ctx, target.ID, username, updatedAt); err != nil {
			return err
		}
		updated = UpdatedUsername{ID: target.ID, Username: username, UpdatedAt: updatedAt}
		return nil
	})
	if err != nil {
		return UpdatedUsername{}, mapUserRepositoryError(err)
	}
	return updated, nil
}

func (s *Service) Roles(ctx context.Context, targetUserID int64) (Roles, error) {
	if s == nil || s.repository == nil {
		return Roles{}, apperror.DependencyUnavailable(fmt.Errorf("query user roles requires a repository"))
	}
	if targetUserID <= 0 {
		return Roles{}, apperror.InvalidRequest(fmt.Errorf("target user id is invalid"))
	}
	target, err := s.repository.FindUser(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Roles{}, userNotFound(err)
		}
		return Roles{}, mapUserRepositoryError(err)
	}
	options, err := s.repository.FindRoleOptions(ctx)
	if err != nil {
		return Roles{}, mapUserRepositoryError(err)
	}
	relations, err := s.repository.FindUserRoles(ctx, targetUserID)
	if err != nil {
		return Roles{}, mapUserRepositoryError(err)
	}
	roleIDs, err := validateUserRoleRelations(options, relations)
	if err != nil {
		return Roles{}, userDataInvalid(err)
	}
	return Roles{
		User:  Summary{ID: target.ID, Username: target.Username, Email: target.Email, IsEnabled: target.IsEnabled},
		Roles: options, RoleIDs: roleIDs,
	}, nil
}

func (s *Service) UpdateRoles(ctx context.Context, actorUserID, targetUserID int64, requestedRoleIDs []int64) (int64, error) {
	if s == nil || s.repository == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("update user roles requires a repository"))
	}
	if actorUserID <= 0 || targetUserID <= 0 {
		return 0, apperror.InvalidRequest(fmt.Errorf("actor or target user id is invalid"))
	}
	normalized, err := normalizeRequestedRoleIDs(requestedRoleIDs)
	if err != nil {
		return 0, userInvalidRoles(err)
	}
	roleCount := int64(len(normalized))
	err = s.repository.Transaction(ctx, func(repository *Repository) error {
		superAdminRole, err := repository.LockSuperAdminRole(ctx)
		if err != nil {
			return err
		}
		target, err := repository.LockUser(ctx, targetUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return userNotFound(err)
			}
			return err
		}
		if actorUserID == target.ID {
			return userSelfOperation(fmt.Errorf("actor cannot change their own roles"))
		}
		actorIsSuperAdmin, err := repository.IsEffectiveSuperAdmin(ctx, actorUserID, superAdminRole.ID)
		if err != nil {
			return err
		}
		targetHasSuperAdmin, err := repository.HasActiveRole(ctx, target.ID, superAdminRole.ID)
		if err != nil {
			return err
		}
		availableRoles, err := repository.LockRoles(ctx)
		if err != nil {
			return err
		}
		rolesByID := make(map[int64]role.Role, len(availableRoles))
		for _, available := range availableRoles {
			if available.ID <= 0 || available.Code == "" || available.Name == "" || !yesno.IsValid(available.IsEnabled) {
				return userDataInvalid(fmt.Errorf("role %d has invalid fields", available.ID))
			}
			if _, exists := rolesByID[available.ID]; exists {
				return userDataInvalid(fmt.Errorf("role %d is duplicated", available.ID))
			}
			rolesByID[available.ID] = available
		}
		hasEnabledRole := false
		requestedHasSuperAdmin := false
		for _, roleID := range normalized {
			available, exists := rolesByID[roleID]
			if !exists {
				return userRoleNotFound(fmt.Errorf("requested role %d is missing", roleID))
			}
			if available.IsEnabled == yesno.Yes {
				hasEnabledRole = true
			}
			if roleID == superAdminRole.ID {
				requestedHasSuperAdmin = true
			}
		}
		if !hasEnabledRole {
			return userInvalidRoles(fmt.Errorf("at least one enabled role is required"))
		}
		storedRelations, err := repository.LockUserRoles(ctx, target.ID)
		if err != nil {
			return err
		}
		currentRoleIDs, err := validateStoredRoleRelations(rolesByID, storedRelations)
		if err != nil {
			return userDataInvalid(err)
		}
		currentHasSuperAdmin := containsSortedID(currentRoleIDs, superAdminRole.ID)
		if currentHasSuperAdmin != targetHasSuperAdmin {
			return userDataInvalid(fmt.Errorf("super administrator relationship state is inconsistent"))
		}
		if !actorIsSuperAdmin && (targetHasSuperAdmin || requestedHasSuperAdmin) {
			return userSuperAdminProtected(fmt.Errorf("ordinary actor cannot change super administrator assignments"))
		}
		if currentHasSuperAdmin && !requestedHasSuperAdmin && target.IsEnabled == yesno.Yes {
			count, err := repository.CountEffectiveSuperAdmins(ctx, superAdminRole.ID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return userLastSuperAdmin(fmt.Errorf("role change would remove the last effective super administrator"))
			}
		}
		if sameInt64s(currentRoleIDs, normalized) {
			return nil
		}
		requested := make(map[int64]struct{}, len(normalized))
		for _, roleID := range normalized {
			requested[roleID] = struct{}{}
		}
		current := make(map[int64]role.UserRole, len(storedRelations))
		removeIDs := make([]int64, 0)
		for _, relation := range storedRelations {
			current[relation.RoleID] = relation
			if _, keep := requested[relation.RoleID]; !keep {
				removeIDs = append(removeIDs, relation.ID)
			}
		}
		operationTime := time.Now().UTC().Truncate(time.Microsecond)
		additions := make([]role.UserRole, 0)
		for _, roleID := range normalized {
			if _, exists := current[roleID]; !exists {
				additions = append(additions, role.UserRole{
					UserID: target.ID, RoleID: roleID, CreatedAt: operationTime, UpdatedAt: operationTime,
				})
			}
		}
		if err := repository.SoftDeleteUserRoleIDs(ctx, removeIDs, operationTime); err != nil {
			return err
		}
		if err := repository.CreateUserRoles(ctx, additions); err != nil {
			return err
		}
		return repository.TouchUser(ctx, target.ID, operationTime)
	})
	if err != nil {
		return 0, mapUserRepositoryError(err)
	}
	return roleCount, nil
}

func (s *Service) UpdateStatus(ctx context.Context, actorUserID, targetUserID int64, value yesno.Value) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update user status requires a repository"))
	}
	if actorUserID <= 0 || targetUserID <= 0 || !yesno.IsValid(value) {
		return apperror.InvalidRequest(fmt.Errorf("actor, target, or user status is invalid"))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		superAdminRole, err := repository.LockSuperAdminRole(ctx)
		if err != nil {
			return err
		}
		target, err := repository.LockUser(ctx, targetUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return userNotFound(err)
			}
			return err
		}
		if value == yesno.No && actorUserID == target.ID {
			return userSelfOperation(fmt.Errorf("actor cannot disable their own account"))
		}
		actorIsSuperAdmin, err := repository.IsEffectiveSuperAdmin(ctx, actorUserID, superAdminRole.ID)
		if err != nil {
			return err
		}
		targetHasSuperAdmin, err := repository.HasActiveRole(ctx, target.ID, superAdminRole.ID)
		if err != nil {
			return err
		}
		if targetHasSuperAdmin && !actorIsSuperAdmin {
			return userSuperAdminProtected(fmt.Errorf("ordinary actor cannot change a super administrator status"))
		}
		if value == yesno.Yes {
			availableRoles, err := repository.LockRoles(ctx)
			if err != nil {
				return err
			}
			relations, err := repository.LockUserRoles(ctx, target.ID)
			if err != nil {
				return err
			}
			rolesByID := make(map[int64]role.Role, len(availableRoles))
			for _, available := range availableRoles {
				rolesByID[available.ID] = available
			}
			if _, err := validateStoredRoleRelations(rolesByID, relations); err != nil {
				return userDataInvalid(err)
			}
			if target.IsEnabled == yesno.Yes {
				return nil
			}
			return repository.UpdateStatus(ctx, target.ID, yesno.Yes, time.Now().UTC().Truncate(time.Microsecond))
		}
		if targetHasSuperAdmin && target.IsEnabled == yesno.Yes {
			count, err := repository.CountEffectiveSuperAdmins(ctx, superAdminRole.ID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return userLastSuperAdmin(fmt.Errorf("status change would disable the last effective super administrator"))
			}
		}
		operationTime := time.Now().UTC().Truncate(time.Microsecond)
		if target.IsEnabled != yesno.No {
			if err := repository.UpdateStatus(ctx, target.ID, yesno.No, operationTime); err != nil {
				return err
			}
		}
		return repository.RevokeActiveSessions(ctx, target.ID, operationTime)
	})
	if err != nil {
		return mapUserRepositoryError(err)
	}
	if value == yesno.No {
		return s.deleteSessionPointer(ctx, targetUserID)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, actorUserID, targetUserID int64) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("delete user requires a repository"))
	}
	if actorUserID <= 0 || targetUserID <= 0 {
		return apperror.InvalidRequest(fmt.Errorf("actor or target user id is invalid"))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		superAdminRole, err := repository.LockSuperAdminRole(ctx)
		if err != nil {
			return err
		}
		target, err := repository.LockUserUnscoped(ctx, targetUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return userNotFound(err)
			}
			return err
		}
		if target.DeletedAt.Valid {
			return nil
		}
		if actorUserID == target.ID {
			return userSelfOperation(fmt.Errorf("actor cannot delete their own account"))
		}
		actorIsSuperAdmin, err := repository.IsEffectiveSuperAdmin(ctx, actorUserID, superAdminRole.ID)
		if err != nil {
			return err
		}
		targetHasSuperAdmin, err := repository.HasActiveRole(ctx, target.ID, superAdminRole.ID)
		if err != nil {
			return err
		}
		if targetHasSuperAdmin && !actorIsSuperAdmin {
			return userSuperAdminProtected(fmt.Errorf("ordinary actor cannot delete a super administrator"))
		}
		if targetHasSuperAdmin && target.IsEnabled == yesno.Yes {
			count, err := repository.CountEffectiveSuperAdmins(ctx, superAdminRole.ID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return userLastSuperAdmin(fmt.Errorf("delete would remove the last effective super administrator"))
			}
		}
		availableRoles, err := repository.LockRoles(ctx)
		if err != nil {
			return err
		}
		relations, err := repository.LockUserRoles(ctx, target.ID)
		if err != nil {
			return err
		}
		rolesByID := make(map[int64]role.Role, len(availableRoles))
		for _, available := range availableRoles {
			rolesByID[available.ID] = available
		}
		if _, err := validateStoredRoleRelations(rolesByID, relations); err != nil {
			return userDataInvalid(err)
		}
		operationTime := time.Now().UTC().Truncate(time.Microsecond)
		relationIDs := make([]int64, len(relations))
		for index, relation := range relations {
			relationIDs[index] = relation.ID
		}
		if err := repository.SoftDeleteUserRoleIDs(ctx, relationIDs, operationTime); err != nil {
			return err
		}
		if err := repository.RevokeActiveSessions(ctx, target.ID, operationTime); err != nil {
			return err
		}
		return repository.SoftDeleteUser(ctx, target.ID, operationTime)
	})
	if err != nil {
		return mapUserRepositoryError(err)
	}
	return s.deleteSessionPointer(ctx, targetUserID)
}

func (s *Service) deleteSessionPointer(ctx context.Context, userID int64) error {
	if s.pointers == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("delete current session pointer requires Redis"))
	}
	if err := s.pointers.Delete(ctx, CurrentSessionPointerKey(userID)); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func normalizeRequestedRoleIDs(values []int64) ([]int64, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("role set is empty")
	}
	normalized := append([]int64(nil), values...)
	sort.Slice(normalized, func(left, right int) bool { return normalized[left] < normalized[right] })
	write := 0
	for _, value := range normalized {
		if value <= 0 {
			return nil, fmt.Errorf("role id must be positive")
		}
		if write == 0 || normalized[write-1] != value {
			normalized[write] = value
			write++
		}
	}
	return normalized[:write], nil
}

func validateUserRoleRelations(options []RoleSummary, relations []role.UserRole) ([]int64, error) {
	rolesByID := make(map[int64]struct{}, len(options))
	for _, option := range options {
		if option.ID <= 0 || option.Code == "" || option.Name == "" || !yesno.IsValid(option.IsEnabled) {
			return nil, fmt.Errorf("role option %d has invalid fields", option.ID)
		}
		if _, exists := rolesByID[option.ID]; exists {
			return nil, fmt.Errorf("role option %d is duplicated", option.ID)
		}
		rolesByID[option.ID] = struct{}{}
	}
	if len(relations) == 0 {
		return nil, fmt.Errorf("user has no active role relationship")
	}
	roleIDs := make([]int64, 0, len(relations))
	seen := make(map[int64]struct{}, len(relations))
	for _, relation := range relations {
		if relation.ID <= 0 || relation.UserID <= 0 || relation.RoleID <= 0 {
			return nil, fmt.Errorf("user role relationship has invalid fields")
		}
		if _, exists := rolesByID[relation.RoleID]; !exists {
			return nil, fmt.Errorf("user role %d refers to a missing role", relation.ID)
		}
		if _, exists := seen[relation.RoleID]; exists {
			return nil, fmt.Errorf("user role %d is duplicated", relation.RoleID)
		}
		seen[relation.RoleID] = struct{}{}
		roleIDs = append(roleIDs, relation.RoleID)
	}
	sort.Slice(roleIDs, func(left, right int) bool { return roleIDs[left] < roleIDs[right] })
	return roleIDs, nil
}

func validateStoredRoleRelations(rolesByID map[int64]role.Role, relations []role.UserRole) ([]int64, error) {
	options := make([]RoleSummary, 0, len(rolesByID))
	for _, available := range rolesByID {
		options = append(options, RoleSummary{ID: available.ID, Code: available.Code, Name: available.Name, IsEnabled: available.IsEnabled})
	}
	return validateUserRoleRelations(options, relations)
}

func containsSortedID(values []int64, wanted int64) bool {
	index := sort.Search(len(values), func(index int) bool { return values[index] >= wanted })
	return index < len(values) && values[index] == wanted
}

func sameInt64s(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mapUserRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, ErrUserDataInvalid) {
		return userDataInvalid(err)
	}
	if errors.Is(err, ErrUsernameConflict) {
		return userUsernameConflict(err)
	}
	return apperror.DependencyUnavailable(err)
}
