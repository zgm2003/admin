package authplatform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"admin/server/internal/module/auth/state"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// policyReader is the narrow PostgreSQL source used only by the policy read
// path, so cache hits can be verified without touching the full repository.
type policyReader interface {
	FindPolicy(context.Context, string) (Platform, error)
}

type Policy struct {
	ID              int64         `json:"id"`
	Code            string        `json:"code"`
	Name            string        `json:"name"`
	LoginTypes      []LoginType   `json:"loginTypes"`
	PolicyVersion   int64         `json:"policyVersion"`
	AccessTTL       time.Duration `json:"accessTTL"`
	RefreshTTL      time.Duration `json:"refreshTTL"`
	SessionCacheTTL time.Duration `json:"sessionCacheTTL"`
	AccessCacheTTL  time.Duration `json:"accessCacheTTL"`
	BindDevice      bool          `json:"bindDevice"`
	BindIP          bool          `json:"bindIP"`
	MaxSessions     int16         `json:"maxSessions"`
	AllowRegister   bool          `json:"allowRegister"`
	IsEnabled       bool          `json:"isEnabled"`
	IsBuiltin       bool          `json:"isBuiltin"`
	Deleted         bool          `json:"deleted"`
}

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
}

type ListItem struct {
	Platform
}

type Deployment struct {
	CookieSecure      bool
	CORSOrigin        string
	TrustedProxyMode  string
	TrustedProxyCount int
	RedisStatus       string
}

type CreateInput struct {
	Code                   string
	Name                   string
	LoginTypes             []LoginType
	AccessTTLSeconds       int
	RefreshTTLSeconds      int
	SessionCacheTTLSeconds int
	AccessCacheTTLSeconds  int
	BindDevice             yesno.Value
	BindIP                 yesno.Value
	MaxSessions            int16
	AllowRegister          yesno.Value
	IsEnabled              yesno.Value
}

type UpdateInput struct {
	Name                   string
	LoginTypes             []LoginType
	AccessTTLSeconds       int
	RefreshTTLSeconds      int
	SessionCacheTTLSeconds int
	AccessCacheTTLSeconds  int
	BindDevice             yesno.Value
	BindIP                 yesno.Value
	MaxSessions            int16
	AllowRegister          yesno.Value
}

type platformService interface {
	CurrentPolicy(context.Context, string) (Policy, error)
	List(context.Context, ListQuery) (pagination.Result[ListItem], error)
	Deployment(context.Context) (Deployment, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
}

type Service struct {
	repository            *Repository
	policyReader          policyReader
	policyLoadGroup       singleflight.Group
	policies              *PolicyStore
	redis                 *projectredis.Client
	authStates            *authstate.Store
	authInvalidator       *authstate.Invalidator
	deleteSessionSnapshot SessionSnapshotDeleter
	logger                *slog.Logger
	deployment            Deployment
}

type SessionSnapshotDeleter func(context.Context, string, int64) error

func NewService(repository *Repository, policies *PolicyStore, redis *projectredis.Client, authStates *authstate.Store, authInvalidator *authstate.Invalidator, deleteSessionSnapshot SessionSnapshotDeleter, logger *slog.Logger, deployment Deployment) *Service {
	return &Service{repository: repository, policyReader: repository, policies: policies, redis: redis, authStates: authStates, authInvalidator: authInvalidator, deleteSessionSnapshot: deleteSessionSnapshot, logger: logger, deployment: deployment}
}

func (s *Service) CurrentPolicy(ctx context.Context, code string) (Policy, error) {
	if err := ValidateCode(code); err != nil {
		return Policy{}, apperror.InvalidRequest(err)
	}
	state, found, err := s.policies.read(ctx, code)
	if err != nil {
		return Policy{}, dependencyUnavailable(err)
	}
	if found {
		return s.availablePolicy(state)
	}
	policy, err := s.loadMissingPolicy(ctx, code)
	if err != nil {
		return Policy{}, err
	}
	return requireAvailablePolicy(policy)
}

func (s *Service) availablePolicy(state policyState) (Policy, error) {
	if state.State == "invalidating" {
		return Policy{}, dependencyUnavailable(fmt.Errorf("authentication policy is updating"))
	}
	if state.Policy == nil {
		return Policy{}, dependencyUnavailable(fmt.Errorf("authentication policy ready state is missing its policy"))
	}
	return requireAvailablePolicy(*state.Policy)
}

func (s *Service) loadMissingPolicy(ctx context.Context, code string) (Policy, error) {
	result := s.policyLoadGroup.DoChan(code, func() (any, error) {
		sharedContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), policyLoadLockTTL)
		defer cancel()
		return s.rebuildMissingPolicy(sharedContext, code)
	})
	var sharedResult singleflight.Result
	select {
	case <-ctx.Done():
		return Policy{}, ctx.Err()
	case sharedResult = <-result:
	}
	if sharedResult.Err != nil {
		return Policy{}, sharedResult.Err
	}
	policy, ok := sharedResult.Val.(Policy)
	if !ok {
		return Policy{}, dependencyUnavailable(fmt.Errorf("authentication policy rebuild returned an invalid value"))
	}
	return policy, nil
}

func (s *Service) rebuildMissingPolicy(ctx context.Context, code string) (Policy, error) {
	lock, err := s.policies.acquireLoadLock(ctx, code)
	if err != nil {
		return s.waitForPolicyReady(ctx, code)
	}
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		lock.release(releaseContext)
	}()
	state, found, err := s.policies.read(ctx, code)
	if err != nil {
		return Policy{}, dependencyUnavailable(err)
	}
	if found {
		return s.availablePolicy(state)
	}
	row, err := s.policyReader.FindPolicy(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Policy{}, notFound(err)
		}
		return Policy{}, dependencyUnavailable(err)
	}
	policy, err := policyFromModel(row)
	if err != nil {
		return Policy{}, dependencyUnavailable(err)
	}
	state, _, err = s.policies.installReadyIfMissing(ctx, policy)
	if err != nil {
		return Policy{}, dependencyUnavailable(err)
	}
	return s.availablePolicy(state)
}

func (s *Service) waitForPolicyReady(ctx context.Context, code string) (Policy, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(policyLoadLockTTL)
	defer timeout.Stop()
	for {
		state, found, err := s.policies.read(ctx, code)
		if err != nil {
			return Policy{}, dependencyUnavailable(err)
		}
		if found {
			return s.availablePolicy(state)
		}
		select {
		case <-ctx.Done():
			return Policy{}, dependencyUnavailable(ctx.Err())
		case <-ticker.C:
		case <-timeout.C:
			state, found, err = s.policies.read(ctx, code)
			if err != nil {
				return Policy{}, dependencyUnavailable(err)
			}
			if found {
				return s.availablePolicy(state)
			}
			return Policy{}, dependencyUnavailable(fmt.Errorf("authentication policy rebuild did not complete"))
		}
	}
}

func requireAvailablePolicy(policy Policy) (Policy, error) {
	if policy.Deleted {
		return Policy{}, notFound(fmt.Errorf("authentication platform is deleted"))
	}
	if !policy.IsEnabled {
		return Policy{}, disabled(fmt.Errorf("authentication platform is disabled"))
	}
	return policy, nil
}

func policyFromModel(value Platform) (Policy, error) {
	if err := ValidatePlatform(value); err != nil {
		return Policy{}, err
	}
	loginTypes, err := parseLoginTypes(value.LoginTypes)
	if err != nil {
		return Policy{}, err
	}
	return Policy{
		ID: value.ID, Code: value.Code, Name: value.Name, LoginTypes: loginTypes, PolicyVersion: value.PolicyVersion,
		AccessTTL:       time.Duration(value.AccessTTLSeconds) * time.Second,
		RefreshTTL:      time.Duration(value.RefreshTTLSeconds) * time.Second,
		SessionCacheTTL: time.Duration(value.SessionCacheTTLSeconds) * time.Second,
		AccessCacheTTL:  time.Duration(value.AccessCacheTTLSeconds) * time.Second,
		BindDevice:      value.BindDevice == yesno.Yes, BindIP: value.BindIP == yesno.Yes,
		MaxSessions: value.MaxSessions, AllowRegister: value.AllowRegister == yesno.Yes,
		IsEnabled: value.IsEnabled == yesno.Yes, IsBuiltin: value.IsBuiltin == yesno.Yes,
		Deleted: value.DeletedAt.Valid,
	}, nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[ListItem], error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || (query.IsEnabled != nil && !yesno.IsValid(*query.IsEnabled)) {
		return pagination.Result[ListItem]{}, apperror.InvalidRequest(fmt.Errorf("authentication platform list query is invalid"))
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, dependencyUnavailable(err)
	}
	rows, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[ListItem]{}, dependencyUnavailable(err)
	}
	items := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ListItem{Platform: row})
	}
	return pagination.Result[ListItem]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Deployment(ctx context.Context) (Deployment, error) {
	if err := s.redis.Ping(ctx); err != nil {
		return Deployment{}, dependencyUnavailable(err)
	}
	result := s.deployment
	result.RedisStatus = "up"
	return result, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	loginTypes, err := marshalLoginTypes(input.LoginTypes)
	if err != nil {
		return 0, invalidPolicy(err)
	}
	value := Platform{
		Code: strings.TrimSpace(input.Code), Name: strings.TrimSpace(input.Name), LoginTypes: loginTypes, PolicyVersion: 1,
		AccessTTLSeconds: input.AccessTTLSeconds, RefreshTTLSeconds: input.RefreshTTLSeconds,
		SessionCacheTTLSeconds: input.SessionCacheTTLSeconds, AccessCacheTTLSeconds: input.AccessCacheTTLSeconds,
		BindDevice: input.BindDevice, BindIP: input.BindIP, MaxSessions: input.MaxSessions,
		AllowRegister: input.AllowRegister, IsEnabled: input.IsEnabled, IsBuiltin: yesno.No,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := ValidatePlatform(value); err != nil {
		return 0, invalidPolicy(err)
	}
	lease, err := s.policies.acquire(ctx, value.Code, nil)
	if err != nil {
		return 0, dependencyUnavailable(err)
	}
	if err := s.repository.Transaction(ctx, func(scoped *Repository) error {
		history, findErr := scoped.LockByCodeUnscoped(ctx, value.Code)
		if findErr != nil {
			return findErr
		}
		if len(history) != 0 {
			return ErrCodeConflict
		}
		return scoped.Create(ctx, &value)
	}); err != nil {
		_ = lease.rollback(ctx)
		if errors.Is(err, ErrCodeConflict) {
			return 0, codeConflict(err)
		}
		return 0, dependencyUnavailable(err)
	}
	policy, err := policyFromModel(value)
	if err != nil {
		return 0, dependencyUnavailable(err)
	}
	if err := lease.commit(ctx, policy); err != nil {
		return 0, dependencyUnavailable(err)
	}
	return value.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	return s.mutate(ctx, id, func(_ Platform, current Policy) (mutationPlan, error) {
		candidate := current
		candidate.Name = strings.TrimSpace(input.Name)
		normalizedLoginTypes, err := normalizeLoginTypes(input.LoginTypes)
		if err != nil {
			return mutationPlan{}, invalidPolicy(err)
		}
		candidate.LoginTypes = normalizedLoginTypes
		candidate.AccessTTL = time.Duration(input.AccessTTLSeconds) * time.Second
		candidate.RefreshTTL = time.Duration(input.RefreshTTLSeconds) * time.Second
		candidate.SessionCacheTTL = time.Duration(input.SessionCacheTTLSeconds) * time.Second
		candidate.AccessCacheTTL = time.Duration(input.AccessCacheTTLSeconds) * time.Second
		candidate.BindDevice = input.BindDevice == yesno.Yes
		candidate.BindIP = input.BindIP == yesno.Yes
		candidate.MaxSessions = input.MaxSessions
		candidate.AllowRegister = input.AllowRegister == yesno.Yes
		if err := ValidatePlatform(platformModelFromPolicy(candidate)); err != nil {
			return mutationPlan{}, invalidPolicy(err)
		}
		if samePlatformRuntimeValues(current, candidate) {
			return mutationPlan{}, nil
		}
		plan := mutationPlan{candidate: candidate}
		if input.MaxSessions > 0 && (current.MaxSessions == 0 || input.MaxSessions < current.MaxSessions) {
			plan.enforceLimit = true
			plan.maxSessions = input.MaxSessions
		}
		return plan, nil
	})
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if !yesno.IsValid(value) {
		return invalidPolicy(fmt.Errorf("isEnabled is invalid"))
	}
	return s.mutate(ctx, id, func(_ Platform, current Policy) (mutationPlan, error) {
		if current.IsEnabled == (value == yesno.Yes) {
			return mutationPlan{}, nil
		}
		return mutationPlan{candidate: current, revokeAll: value == yesno.No, status: &value}, nil
	})
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.mutate(ctx, id, func(_ Platform, current Policy) (mutationPlan, error) {
		if current.IsBuiltin {
			return mutationPlan{}, builtinProtected(fmt.Errorf("builtin authentication platform cannot be deleted"))
		}
		return mutationPlan{candidate: current, revokeAll: true, deletePlatform: true}, nil
	})
}

type mutationPlan struct {
	candidate      Policy
	revokeAll      bool
	enforceLimit   bool
	maxSessions    int16
	status         *yesno.Value
	deletePlatform bool
}

func (s *Service) mutate(ctx context.Context, id int64, planner func(Platform, Policy) (mutationPlan, error)) error {
	currentRow, err := s.repository.LockByID(ctx, id)
	if err != nil {
		if errors.Is(err, gormErrRecordNotFound()) {
			return notFound(err)
		}
		return dependencyUnavailable(err)
	}
	current, err := policyFromModel(currentRow)
	if err != nil {
		return dependencyUnavailable(err)
	}
	plan, err := planner(currentRow, current)
	if err != nil {
		return err
	}
	if plan.candidate.ID == 0 {
		return nil
	}
	if s.authStates == nil || s.authInvalidator == nil || s.deleteSessionSnapshot == nil {
		return dependencyUnavailable(fmt.Errorf("authentication platform session dependencies are required"))
	}
	current, err = s.policies.readyForMutation(ctx, current)
	if err != nil {
		return dependencyUnavailable(err)
	}
	lease, err := s.policies.acquire(ctx, current.Code, &current)
	if err != nil {
		return dependencyUnavailable(err)
	}
	var affectedUsers []int64
	if plan.revokeAll {
		affectedUsers, err = s.repository.FindActiveSessionUsers(ctx, current.Code)
	} else if plan.enforceLimit {
		affectedUsers, err = s.repository.FindSessionLimitCandidates(ctx, current.Code, plan.maxSessions)
	}
	if err != nil {
		_ = lease.rollback(ctx)
		return dependencyUnavailable(err)
	}
	authFacts := authstate.MutationFacts{}
	for _, userID := range affectedUsers {
		state, found, readErr := s.authStates.ReadSessions(ctx, current.Code, userID)
		if readErr != nil && found {
			_ = lease.rollback(ctx)
			return dependencyUnavailable(readErr)
		}
		if found {
			if state.State != authstate.StateReady {
				_ = lease.rollback(ctx)
				return sessionUpdating(authstate.ErrUpdating)
			}
			authFacts.Sessions = append(authFacts.Sessions, state.Fact())
			continue
		}
		generation, generationErr := authstate.NewGeneration()
		if generationErr != nil {
			_ = lease.rollback(ctx)
			return dependencyUnavailable(generationErr)
		}
		fact := authstate.SessionsFact{Platform: current.Code, UserID: userID, Generation: generation}
		installed, _, installErr := s.authStates.InstallSessionsReadyIfMissing(ctx, fact)
		if installErr != nil {
			_ = lease.rollback(ctx)
			return dependencyUnavailable(installErr)
		}
		authFacts.Sessions = append(authFacts.Sessions, installed.Fact())
	}
	var authLease *authstate.MutationLease
	if len(authFacts.Sessions) > 0 {
		authLease, err = s.authInvalidator.Acquire(ctx, authFacts)
		if err != nil {
			_ = lease.rollback(ctx)
			return dependencyUnavailable(err)
		}
	}
	mutationCtx, stopPolicyRenewal := lease.StartRenewal(ctx)
	var stopAuthRenewal func()
	if authLease != nil {
		mutationCtx, stopAuthRenewal = authLease.StartRenewal(mutationCtx)
	}
	var revoked []SessionRef
	err = s.repository.Transaction(ctx, func(scoped *Repository) error {
		locked, lockErr := scoped.LockByID(mutationCtx, id)
		if lockErr != nil {
			return lockErr
		}
		if locked.PolicyVersion != currentRow.PolicyVersion || locked.Code != current.Code {
			return ErrUpdating
		}
		if plan.deletePlatform {
			hasMenus, menuErr := scoped.HasActiveMenus(mutationCtx, id)
			if menuErr != nil {
				return menuErr
			}
			if hasMenus {
				return menusAttached(fmt.Errorf("authentication platform has active menus"))
			}
		}
		if authLease != nil {
			lockedUsers, userErr := scoped.LockActiveSessionUsers(mutationCtx, current.Code)
			if userErr != nil {
				return userErr
			}
			actualUsers := lockedUsers
			if plan.enforceLimit {
				actualUsers, userErr = scoped.FindSessionLimitCandidates(mutationCtx, current.Code, plan.maxSessions)
				if userErr != nil {
					return userErr
				}
			}
			if !sameInt64Set(affectedUsers, actualUsers) {
				return ErrUpdating
			}
			if plan.revokeAll {
				revoked, userErr = scoped.RevokePlatformSessions(mutationCtx, current.Code, time.Now().UTC().Truncate(time.Microsecond))
				if userErr != nil {
					return userErr
				}
			} else if plan.enforceLimit {
				revoked, userErr = scoped.EnforcePlatformLimit(mutationCtx, current.Code, plan.maxSessions, time.Now().UTC().Truncate(time.Microsecond))
				if userErr != nil {
					return userErr
				}
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		if plan.deletePlatform {
			_, lockErr = scoped.SoftDelete(mutationCtx, id, now)
		} else if plan.status != nil {
			_, lockErr = scoped.UpdateStatus(mutationCtx, id, *plan.status, now)
		} else {
			loginTypesRaw, marshalErr := marshalLoginTypes(plan.candidate.LoginTypes)
			if marshalErr != nil {
				return marshalErr
			}
			_, lockErr = scoped.UpdatePolicy(mutationCtx, id, UpdateValues{
				Name: plan.candidate.Name, LoginTypes: loginTypesRaw, AccessTTLSeconds: int(plan.candidate.AccessTTL / time.Second), RefreshTTLSeconds: int(plan.candidate.RefreshTTL / time.Second),
				SessionCacheTTLSeconds: int(plan.candidate.SessionCacheTTL / time.Second), AccessCacheTTLSeconds: int(plan.candidate.AccessCacheTTL / time.Second),
				BindDevice: boolToYesNo(plan.candidate.BindDevice), BindIP: boolToYesNo(plan.candidate.BindIP), MaxSessions: plan.candidate.MaxSessions, AllowRegister: boolToYesNo(plan.candidate.AllowRegister),
			}, now)
		}
		return lockErr
	})
	stopPolicyRenewal()
	if stopAuthRenewal != nil {
		stopAuthRenewal()
	}
	if err != nil {
		if authLease != nil {
			_ = authLease.Rollback(ctx)
		}
		_ = lease.rollback(ctx)
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return appErr
		}
		return dependencyUnavailable(err)
	}
	updatedRow, err := s.repository.FindPolicy(ctx, current.Code)
	if err != nil {
		if errors.Is(err, gormErrRecordNotFound()) {
			updatedRow = currentRow
			updatedRow.PolicyVersion++
			updatedRow.DeletedAt.Valid = true
		} else {
			return dependencyUnavailable(err)
		}
	}
	updated, err := policyFromModel(updatedRow)
	if err != nil {
		return dependencyUnavailable(err)
	}
	if authLease != nil {
		nextFacts := authstate.MutationFacts{Sessions: make([]authstate.SessionsFact, 0, len(authFacts.Sessions))}
		for _, fact := range authFacts.Sessions {
			generation, generationErr := authstate.NewGeneration()
			if generationErr != nil {
				return dependencyUnavailable(generationErr)
			}
			nextFacts.Sessions = append(nextFacts.Sessions, authstate.SessionsFact{Platform: fact.Platform, UserID: fact.UserID, Generation: generation})
		}
		if err := authLease.Commit(ctx, nextFacts); err != nil {
			return dependencyUnavailable(err)
		}
	}
	for _, session := range revoked {
		if err := s.deleteSessionSnapshot(ctx, current.Code, session.SessionID); err != nil {
			return dependencyUnavailable(err)
		}
	}
	if err := lease.commit(ctx, updated); err != nil {
		return dependencyUnavailable(err)
	}
	return nil
}

func samePlatformRuntimeValues(left, right Policy) bool {
	return left.Name == right.Name && sameLoginTypes(left.LoginTypes, right.LoginTypes) &&
		left.AccessTTL == right.AccessTTL && left.RefreshTTL == right.RefreshTTL &&
		left.SessionCacheTTL == right.SessionCacheTTL && left.AccessCacheTTL == right.AccessCacheTTL &&
		left.BindDevice == right.BindDevice && left.BindIP == right.BindIP && left.MaxSessions == right.MaxSessions && left.AllowRegister == right.AllowRegister && left.IsEnabled == right.IsEnabled
}

func sameLoginTypes(left, right []LoginType) bool {
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

func platformModelFromPolicy(policy Policy) Platform {
	raw, _ := marshalLoginTypes(policy.LoginTypes)
	return Platform{
		ID: policy.ID, Code: policy.Code, Name: policy.Name, LoginTypes: raw, PolicyVersion: policy.PolicyVersion,
		AccessTTLSeconds: int(policy.AccessTTL / time.Second), RefreshTTLSeconds: int(policy.RefreshTTL / time.Second),
		SessionCacheTTLSeconds: int(policy.SessionCacheTTL / time.Second), AccessCacheTTLSeconds: int(policy.AccessCacheTTL / time.Second),
		BindDevice: boolToYesNo(policy.BindDevice), BindIP: boolToYesNo(policy.BindIP), MaxSessions: policy.MaxSessions,
		AllowRegister: boolToYesNo(policy.AllowRegister), IsEnabled: boolToYesNo(policy.IsEnabled), IsBuiltin: boolToYesNo(policy.IsBuiltin),
	}
}

func sameInt64Set(left, right []int64) bool {
	return sameSortedInt64s(left, right)
}

func boolToYesNo(value bool) yesno.Value {
	if value {
		return yesno.Yes
	}
	return yesno.No
}

func sameSortedInt64s(left, right []int64) bool {
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

func gormErrRecordNotFound() error {
	return gorm.ErrRecordNotFound
}
