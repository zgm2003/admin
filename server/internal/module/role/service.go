package role

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/menu"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const (
	superAdminName     = "超级管理员"
	registeredUserName = "普通用户"
)

type Service struct {
	repository        *Repository
	accessInvalidator *accessstate.Invalidator
}

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
}

type ListItem struct {
	ID              int64
	Code            string
	Name            string
	IsDefault       yesno.Value
	IsEnabled       yesno.Value
	UserCount       int64
	PermissionCount int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	Code string
	Name string
}

type UpdateInput struct {
	Name string
}

func NewService(repository *Repository, accessInvalidator *accessstate.Invalidator) *Service {
	return &Service{repository: repository, accessInvalidator: accessInvalidator}
}

func (s *Service) EnsureSystemRoles(ctx context.Context) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("ensure system roles requires a repository"))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		if err := repository.LockRoleTable(ctx); err != nil {
			return err
		}
		systemRecords, err := repository.FindSystemRoleRecords(ctx)
		if err != nil {
			return err
		}
		activeRoles, err := repository.LockActiveRoles(ctx)
		if err != nil {
			return err
		}

		if len(systemRecords) == 0 {
			if len(activeRoles) != 0 {
				return roleDataInvalid(fmt.Errorf("fixed roles are missing while role data exists"))
			}
			if err := repository.Create(ctx, &Role{
				Code: CodeSuperAdmin, Name: superAdminName, IsDefault: yesno.No, IsEnabled: yesno.Yes,
			}); err != nil {
				return err
			}
			if err := repository.Create(ctx, &Role{
				Code: CodeRegisteredUser, Name: registeredUserName, IsDefault: yesno.Yes, IsEnabled: yesno.Yes,
			}); err != nil {
				return err
			}
			return nil
		}

		if err := validateSystemRoleRecords(systemRecords); err != nil {
			return err
		}
		return validateDefaultRole(activeRoles)
	})
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[ListItem], error) {
	if s == nil || s.repository == nil {
		return pagination.Result[ListItem]{}, apperror.DependencyUnavailable(fmt.Errorf("list roles requires a repository"))
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 ||
		(query.IsEnabled != nil && !yesno.IsValid(*query.IsEnabled)) || utf8.RuneCountInString(query.Keyword) > 64 {
		return pagination.Result[ListItem]{}, apperror.InvalidRequest(fmt.Errorf("role list query is invalid"))
	}
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, apperror.DependencyUnavailable(err)
	}
	items, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, apperror.DependencyUnavailable(err)
	}
	if items == nil {
		items = make([]ListItem, 0)
	}
	return pagination.Result[ListItem]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repository == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("create role requires a repository"))
	}
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	if !IsValidCode(input.Code) || IsSystemCode(input.Code) || !isValidRoleName(input.Name) {
		return 0, roleInvalidState(fmt.Errorf("role code or name is invalid"))
	}
	created := Role{
		Code: input.Code, Name: input.Name, IsDefault: yesno.No, IsEnabled: yesno.Yes,
	}
	if err := s.repository.Create(ctx, &created); err != nil {
		return 0, mapRoleRepositoryError(err, input.Code, input.Name)
	}
	return created.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update role requires a repository"))
	}
	input.Name = strings.TrimSpace(input.Name)
	if id < 1 || !isValidRoleName(input.Name) {
		return roleInvalidState(fmt.Errorf("role id or name is invalid"))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		stored, err := repository.LockActiveRole(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return roleNotFound(err)
			}
			return err
		}
		if IsSystemCode(stored.Code) {
			return roleSystemProtected(stored.Code, fmt.Errorf("system role name is immutable"))
		}
		if stored.Name == input.Name {
			return nil
		}
		return repository.UpdateName(ctx, stored.ID, input.Name, time.Now().UTC())
	})
	if err != nil {
		return mapRoleRepositoryError(err, "", input.Name)
	}
	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update role status requires a repository"))
	}
	if id < 1 || !yesno.IsValid(value) {
		return roleInvalidState(fmt.Errorf("role id or status is invalid"))
	}
	err := s.mutateAffectedUsers(ctx, id, func(mutationCtx context.Context, repository *Repository, candidates []accessstate.Version) (bool, map[int64]int64, error) {
		stored, err := repository.LockActiveRole(mutationCtx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil, roleNotFound(err)
			}
			return false, nil, err
		}
		actual, err := repository.LockEffectiveAccessVersionsByRole(mutationCtx, id)
		if err != nil {
			return false, nil, err
		}
		if !equalAccessVersions(candidates, actual) {
			return false, nil, errRoleAffectedUsersChanged
		}
		if stored.Code == CodeSuperAdmin {
			return false, nil, roleSystemProtected(stored.Code, fmt.Errorf("super administrator status is immutable"))
		}
		if stored.IsEnabled == value {
			return false, nil, nil
		}
		if value == yesno.No && stored.IsDefault == yesno.Yes {
			return false, nil, roleDefaultProtected(stored.Code, fmt.Errorf("default role cannot be disabled"))
		}
		updatedAt := time.Now().UTC().Truncate(time.Microsecond)
		if err := repository.UpdateStatus(mutationCtx, stored.ID, value, updatedAt); err != nil {
			return false, nil, err
		}
		versions, err := repository.IncrementAccessVersions(mutationCtx, accessUserIDs(actual), updatedAt)
		return true, versions, err
	})
	return mapRoleRepositoryError(err, "", "")
}

func (s *Service) SetDefault(ctx context.Context, id int64) error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("set default role requires a repository"))
	}
	if id < 1 {
		return roleInvalidState(fmt.Errorf("role id is invalid"))
	}
	err := s.repository.Transaction(ctx, func(repository *Repository) error {
		records, err := repository.LockActiveRoles(ctx)
		if err != nil {
			return err
		}
		if err := validateDefaultRole(records); err != nil {
			return err
		}
		var target *Role
		var current *Role
		for index := range records {
			if records[index].ID == id {
				target = &records[index]
			}
			if records[index].IsDefault == yesno.Yes {
				current = &records[index]
			}
		}
		if target == nil {
			return roleNotFound(gorm.ErrRecordNotFound)
		}
		if target.Code == CodeSuperAdmin {
			return roleSystemProtected(target.Code, fmt.Errorf("super administrator cannot be default"))
		}
		if target.IsEnabled != yesno.Yes {
			return roleInvalidState(fmt.Errorf("default target is disabled"))
		}
		if target.IsDefault == yesno.Yes {
			return nil
		}
		if current == nil {
			return roleDataInvalid(fmt.Errorf("current default role is missing"))
		}
		updatedAt := time.Now().UTC()
		if err := repository.ClearDefault(ctx, current.ID, updatedAt); err != nil {
			return err
		}
		if err := repository.SetDefault(ctx, target.ID, updatedAt); err != nil {
			return err
		}
		current.IsDefault = yesno.No
		target.IsDefault = yesno.Yes
		return validateDefaultRole(records)
	})
	return mapRoleRepositoryError(err, "", "")
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("delete role requires a repository"))
	}
	if id < 1 {
		return roleInvalidState(fmt.Errorf("role id is invalid"))
	}
	err := s.mutateAffectedUsers(ctx, id, func(mutationCtx context.Context, repository *Repository, candidates []accessstate.Version) (bool, map[int64]int64, error) {
		stored, err := repository.LockActiveRole(mutationCtx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil, roleNotFound(err)
			}
			return false, nil, err
		}
		actual, err := repository.LockEffectiveAccessVersionsByRole(mutationCtx, id)
		if err != nil {
			return false, nil, err
		}
		if !equalAccessVersions(candidates, actual) {
			return false, nil, errRoleAffectedUsersChanged
		}
		if IsSystemCode(stored.Code) {
			return false, nil, roleSystemProtected(stored.Code, fmt.Errorf("system role cannot be deleted"))
		}
		if stored.IsDefault == yesno.Yes {
			return false, nil, roleDefaultProtected(stored.Code, fmt.Errorf("default role cannot be deleted"))
		}
		userCount, err := repository.CountEffectiveUsers(mutationCtx, stored.ID)
		if err != nil {
			return false, nil, err
		}
		if userCount > 0 {
			return false, nil, roleUsersAttached(stored.Code, fmt.Errorf("role has %d effective users", userCount))
		}
		deletedAt := time.Now().UTC().Truncate(time.Microsecond)
		if err := repository.SoftDeleteRoleMenus(mutationCtx, stored.ID, deletedAt); err != nil {
			return false, nil, err
		}
		if err := repository.SoftDeleteRole(mutationCtx, stored.ID, deletedAt); err != nil {
			return false, nil, err
		}
		versions, err := repository.IncrementAccessVersions(mutationCtx, accessUserIDs(actual), deletedAt)
		return true, versions, err
	})
	return mapRoleRepositoryError(err, "", "")
}

func (s *Service) Permissions(ctx context.Context, roleID int64) (Permissions, error) {
	if s == nil || s.repository == nil {
		return Permissions{}, apperror.DependencyUnavailable(fmt.Errorf("query role permissions requires a repository"))
	}
	if roleID < 1 {
		return Permissions{}, roleInvalidState(fmt.Errorf("role id is invalid"))
	}
	stored, err := s.repository.FindActiveRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Permissions{}, roleNotFound(err)
		}
		return Permissions{}, apperror.DependencyUnavailable(err)
	}
	if stored.Code == CodeSuperAdmin {
		return Permissions{}, roleSuperAdminAuthorization(fmt.Errorf("super administrator has implicit permissions"))
	}
	platforms, err := s.repository.FindPermissionPlatforms(ctx)
	if err != nil {
		return Permissions{}, apperror.DependencyUnavailable(err)
	}
	menus, err := s.repository.LockActiveMenus(ctx)
	if err != nil {
		return Permissions{}, apperror.DependencyUnavailable(err)
	}
	index, err := buildPermissionIndex(menus)
	if err != nil {
		return Permissions{}, roleDataInvalid(err)
	}
	grants, err := s.repository.FindActiveRoleMenus(ctx, roleID)
	if err != nil {
		return Permissions{}, apperror.DependencyUnavailable(err)
	}
	menuIDs, err := index.validateStored(grants)
	if err != nil {
		return Permissions{}, roleDataInvalid(err)
	}
	platformIDs := make(map[int64]struct{}, len(platforms))
	for platformIndex := range platforms {
		platform := &platforms[platformIndex]
		if platform.ID < 1 || strings.TrimSpace(platform.Code) == "" || strings.TrimSpace(platform.Name) == "" || !yesno.IsValid(platform.IsEnabled) {
			return Permissions{}, roleDataInvalid(fmt.Errorf("permission platform %d has invalid stored values", platform.ID))
		}
		if _, exists := platformIDs[platform.ID]; exists {
			return Permissions{}, roleDataInvalid(fmt.Errorf("permission platform %d is duplicated", platform.ID))
		}
		platformIDs[platform.ID] = struct{}{}
		tree, treeErr := index.tree(platform.ID)
		if treeErr != nil {
			return Permissions{}, roleDataInvalid(treeErr)
		}
		platform.MenuTree = tree
	}
	for _, row := range menus {
		if _, exists := platformIDs[row.PlatformID]; !exists {
			return Permissions{}, roleDataInvalid(fmt.Errorf("menu %d references an unavailable platform", row.ID))
		}
	}
	return Permissions{
		Role:      Summary{ID: stored.ID, Code: stored.Code, Name: stored.Name, IsDefault: stored.IsDefault, IsEnabled: stored.IsEnabled},
		Platforms: platforms, MenuIDs: menuIDs,
	}, nil
}

func (s *Service) UpdatePermissions(ctx context.Context, roleID int64, menuIDs []int64) (int64, error) {
	if s == nil || s.repository == nil || s.accessInvalidator == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("update role permissions requires a repository"))
	}
	if roleID < 1 || menuIDs == nil {
		return 0, roleInvalidPermission(fmt.Errorf("role id or menu ids are invalid"))
	}
	var permissionCount int64
	err := s.mutateAffectedUsers(ctx, roleID, func(mutationCtx context.Context, repository *Repository, candidates []accessstate.Version) (bool, map[int64]int64, error) {
		stored, err := repository.LockActiveRole(mutationCtx, roleID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil, roleNotFound(err)
			}
			return false, nil, err
		}
		if stored.Code == CodeSuperAdmin {
			return false, nil, roleSuperAdminAuthorization(fmt.Errorf("super administrator has implicit permissions"))
		}
		menus, err := repository.LockActiveMenus(mutationCtx)
		if err != nil {
			return false, nil, err
		}
		actual, err := repository.LockEffectiveAccessVersionsByRole(mutationCtx, roleID)
		if err != nil {
			return false, nil, err
		}
		if !equalAccessVersions(candidates, actual) {
			return false, nil, errRoleAffectedUsersChanged
		}
		index, err := buildPermissionIndex(menus)
		if err != nil {
			return false, nil, roleDataInvalid(err)
		}
		normalized, err := index.normalizeRequested(menuIDs)
		if err != nil {
			return false, nil, roleInvalidPermission(err)
		}
		grants, err := repository.FindActiveRoleMenus(mutationCtx, roleID)
		if err != nil {
			return false, nil, err
		}
		storedIDs, err := index.validateStored(grants)
		if err != nil {
			return false, nil, roleDataInvalid(err)
		}
		permissionCount = int64(len(normalized))
		if equalInt64Slices(storedIDs, normalized) {
			return false, nil, nil
		}
		wanted := make(map[int64]struct{}, len(normalized))
		for _, id := range normalized {
			wanted[id] = struct{}{}
		}
		current := make(map[int64]menu.RoleMenu, len(grants))
		removeIDs := make([]int64, 0)
		for _, grant := range grants {
			current[grant.MenuID] = grant
			if _, keep := wanted[grant.MenuID]; !keep {
				removeIDs = append(removeIDs, grant.ID)
			}
		}
		additions := make([]menu.RoleMenu, 0)
		for _, id := range normalized {
			if _, exists := current[id]; !exists {
				additions = append(additions, menu.RoleMenu{RoleID: roleID, MenuID: id})
			}
		}
		updatedAt := time.Now().UTC().Truncate(time.Microsecond)
		if err := repository.SoftDeleteRoleMenuIDs(mutationCtx, removeIDs, updatedAt); err != nil {
			return false, nil, err
		}
		if err := repository.CreateRoleMenus(mutationCtx, additions); err != nil {
			return false, nil, err
		}
		if err := repository.TouchRole(mutationCtx, roleID, updatedAt); err != nil {
			return false, nil, err
		}
		versions, err := repository.IncrementAccessVersions(mutationCtx, accessUserIDs(actual), updatedAt)
		return true, versions, err
	})
	if err != nil {
		return 0, mapRoleRepositoryError(err, "", "")
	}
	return permissionCount, nil
}

var errRoleAffectedUsersChanged = errors.New("role affected users changed")

const roleMutationAttempts = 3

type roleAccessMutation func(context.Context, *Repository, []accessstate.Version) (bool, map[int64]int64, error)

func (s *Service) mutateAffectedUsers(ctx context.Context, roleID int64, mutate roleAccessMutation) error {
	for attempt := 0; attempt < roleMutationAttempts; attempt++ {
		candidates, err := s.repository.FindEffectiveAccessVersionsByRole(ctx, roleID)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		lease, err := s.accessInvalidator.Acquire(ctx, candidates)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		mutationCtx, stopRenewal := lease.StartRenewal(ctx)
		changed := false
		advanced := map[int64]int64{}
		err = s.repository.Transaction(mutationCtx, func(repository *Repository) error {
			var mutationErr error
			changed, advanced, mutationErr = mutate(mutationCtx, repository, candidates)
			return mutationErr
		})
		renewalCause := context.Cause(mutationCtx)
		stopRenewal()
		if errors.Is(err, errRoleAffectedUsersChanged) {
			if rollbackErr := lease.Rollback(ctx); rollbackErr != nil {
				return apperror.DependencyUnavailable(errors.Join(err, renewalCause, rollbackErr))
			}
			continue
		}
		if err != nil {
			return errors.Join(err, renewalCause, lease.Rollback(ctx))
		}
		if renewalCause != nil {
			return apperror.DependencyUnavailable(errors.Join(renewalCause, lease.Rollback(ctx)))
		}
		if !changed {
			if err := lease.Rollback(ctx); err != nil {
				return apperror.DependencyUnavailable(err)
			}
			return nil
		}
		if err := lease.Commit(ctx, advanced); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		return nil
	}
	return apperror.Conflict(i18n.KeyConflict, nil, errRoleAffectedUsersChanged)
}

func equalAccessVersions(left, right []accessstate.Version) bool {
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

func accessUserIDs(versions []accessstate.Version) []int64 {
	userIDs := make([]int64, len(versions))
	for index, version := range versions {
		userIDs[index] = version.UserID
	}
	return userIDs
}

func equalInt64Slices(left, right []int64) bool {
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

func isValidRoleName(name string) bool {
	count := utf8.RuneCountInString(name)
	return count >= 1 && count <= 64
}

func mapRoleRepositoryError(err error, code, name string) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	switch {
	case errors.Is(err, ErrRoleCodeConflict):
		return roleCodeConflict(code, err)
	case errors.Is(err, ErrRoleNameConflict):
		return roleNameConflict(name, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func validateSystemRoleRecords(records []Role) error {
	if len(records) != 2 {
		return roleDataInvalid(fmt.Errorf("expected exactly two fixed role records, found %d", len(records)))
	}
	byCode := make(map[string]Role, 2)
	for _, record := range records {
		if record.DeletedAt.Valid {
			return roleDataInvalid(fmt.Errorf("fixed role %s contains deleted history", record.Code))
		}
		if _, exists := byCode[record.Code]; exists {
			return roleDataInvalid(fmt.Errorf("fixed role %s is duplicated", record.Code))
		}
		byCode[record.Code] = record
	}
	superAdmin, hasSuperAdmin := byCode[CodeSuperAdmin]
	registeredUser, hasRegisteredUser := byCode[CodeRegisteredUser]
	if !hasSuperAdmin || !hasRegisteredUser {
		return roleDataInvalid(fmt.Errorf("fixed role set is incomplete"))
	}
	if superAdmin.Name != superAdminName || superAdmin.IsEnabled != yesno.Yes || superAdmin.IsDefault != yesno.No {
		return roleDataInvalid(fmt.Errorf("super administrator fields are invalid"))
	}
	if registeredUser.Name != registeredUserName || !yesno.IsValid(registeredUser.IsEnabled) || !yesno.IsValid(registeredUser.IsDefault) {
		return roleDataInvalid(fmt.Errorf("registered user role fields are invalid"))
	}
	return nil
}

func validateDefaultRole(records []Role) error {
	var defaultRole *Role
	for index := range records {
		record := &records[index]
		if !yesno.IsValid(record.IsEnabled) || !yesno.IsValid(record.IsDefault) {
			return roleDataInvalid(fmt.Errorf("role %d has an invalid yes/no state", record.ID))
		}
		if record.IsDefault != yesno.Yes {
			continue
		}
		if defaultRole != nil {
			return roleDataInvalid(fmt.Errorf("multiple active default roles exist"))
		}
		defaultRole = record
	}
	if defaultRole == nil {
		return roleDataInvalid(fmt.Errorf("active default role is missing"))
	}
	if defaultRole.IsEnabled != yesno.Yes || defaultRole.Code == CodeSuperAdmin {
		return roleDataInvalid(fmt.Errorf("default role is disabled or protected"))
	}
	return nil
}
