package uploadrule

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"admin/server/internal/storage/objectKey"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	repository      *Repository
	keys            *secretkey.KeyRing
	signer          storagecos.Presigner
	configRuntime   configRuntimeSource
	routeCache      *RouteCache
	routeSource     routeSource
	now             func() time.Time
	wait            func(context.Context, time.Duration) error
	routeReadBudget time.Duration
	routeWaitStep   time.Duration
}

type configRuntimeSource interface {
	Runtime(context.Context, int64) (cosconfig.RuntimeConfig, error)
}

func NewService(repository *Repository, keys *secretkey.KeyRing, signer storagecos.Presigner, configRuntime configRuntimeSource, routeCache *RouteCache) *Service {
	service := &Service{
		repository:      repository,
		keys:            keys,
		signer:          signer,
		configRuntime:   configRuntime,
		routeCache:      routeCache,
		now:             time.Now,
		wait:            defaultWait,
		routeReadBudget: routeReadBudgetDefault,
		routeWaitStep:   routeReadWaitStep,
	}
	if repository != nil {
		service.routeSource = repository
	}
	return service
}

// SetRouteCache 由 API 入口显式注入不可变 rule route 的 Redis 快照缓存。
func (s *Service) SetRouteCache(cache *RouteCache) { s.routeCache = cache }

func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.keys == nil || s.signer == nil || s.configRuntime == nil ||
		s.routeCache == nil || s.routeCache.client == nil || s.routeSource == nil {
		return fmt.Errorf("upload rule route and signing dependencies are not configured")
	}
	return nil
}
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.Result[RuleValue], error) {
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 {
		return pagination.Result[RuleValue]{}, invalid(fmt.Errorf("pagination invalid"))
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	n, e := s.repository.Count(ctx, q)
	if e != nil {
		return pagination.Result[RuleValue]{}, dependency(e)
	}
	rows, e := s.repository.List(ctx, q)
	if e != nil {
		return pagination.Result[RuleValue]{}, dependency(e)
	}
	return pagination.Result[RuleValue]{List: rows, Total: n, Page: q.Page, PageSize: q.PageSize}, nil
}
func (s *Service) PageInit(ctx context.Context) (PageInit, error) {
	p, e := s.repository.FindPlatformOptions(ctx)
	if e != nil {
		return PageInit{}, dependency(e)
	}
	c, e := s.repository.FindConfigSummaries(ctx)
	if e != nil {
		return PageInit{}, dependency(e)
	}
	return PageInit{p, c}, nil
}
func (s *Service) Get(ctx context.Context, id int64) (RuleValue, error) {
	if id < 1 {
		return RuleValue{}, invalid(fmt.Errorf("id invalid"))
	}
	m, e := s.repository.FindByID(ctx, id)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return RuleValue{}, notFound(e)
	}
	if e != nil {
		return RuleValue{}, dependency(e)
	}
	return RuleValue{ID: m.ID, PlatformID: m.PlatformID, Codes: append([]string(nil), m.Codes...), Name: m.Name, CosConfigID: m.CosConfigID, MaxFileSizeBytes: m.MaxFileSizeBytes, AllowedExtensions: []string(m.AllowedExtensions), AllowedMimeTypes: []string(m.AllowedMimeTypes), AccessMode: m.AccessMode, IsEnabled: m.IsEnabled, Remark: m.Remark, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (int64, error) {
	in = normalizeCreateInput(in)
	if e := validateFields(in.PlatformID, in.Codes, in.Name, in.CosConfigID, in.MaxFileSizeBytes, in.AllowedExtensions, in.AllowedMimeTypes, in.AccessMode, in.Remark, true); e != nil {
		return 0, invalid(e)
	}
	var id int64
	err := s.repository.Transaction(ctx, func(r *Repository) error {
		// 固定锁顺序：平台行 -> COS 配置行 -> 同平台其它活动规则 -> 创建目标。
		if e := r.LockPlatform(ctx, in.PlatformID); e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return conflict(fmt.Errorf("platform unavailable"))
			}
			return dependency(e)
		}
		config, e := r.LockConfig(ctx, in.CosConfigID)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return conflict(fmt.Errorf("COS config unavailable"))
			}
			return dependency(e)
		}
		if e = validatePublicConfig(in.AccessMode, config); e != nil {
			return e
		}
		now := time.Now().UTC()
		if in.IsEnabled == yesno.Yes {
			others, e := r.LockEnabledOtherRules(ctx, in.PlatformID, 0)
			if e != nil {
				return dependency(e)
			}
			if e := r.DisableRules(ctx, others, now); e != nil {
				return dependency(e)
			}
		}
		var createErr error
		id, createErr = repositoryCreate(r, ctx, in)
		return createErr
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}
func repositoryCreate(r *Repository, ctx context.Context, in CreateInput) (int64, error) {
	now := time.Now().UTC()
	m := &Model{PlatformID: in.PlatformID, Name: in.Name, CosConfigID: in.CosConfigID, MaxFileSizeBytes: in.MaxFileSizeBytes, AllowedExtensions: StringArray(in.AllowedExtensions), AllowedMimeTypes: StringArray(in.AllowedMimeTypes), AccessMode: in.AccessMode, IsEnabled: in.IsEnabled, Remark: in.Remark, CreatedAt: now, UpdatedAt: now}
	if e := r.Create(ctx, m, in.Codes); e != nil {
		if errors.Is(e, ErrConflict) {
			return 0, conflict(e)
		}
		return 0, dependency(e)
	}
	return m.ID, nil
}
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if in.IsEnabled != nil && !yesno.IsValid(*in.IsEnabled) {
		return invalid(fmt.Errorf("isEnabled invalid"))
	}
	in = normalizeUpdateInput(in)
	if err := validateFields(1, in.Codes, in.Name, 1, in.MaxFileSizeBytes, in.AllowedExtensions, in.AllowedMimeTypes, "", in.Remark, false); err != nil {
		return invalid(err)
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		enabling := in.IsEnabled != nil && *in.IsEnabled == yesno.Yes
		m, err := lockMutationTarget(ctx, r, id, enabling)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if enabling {
			others, err := r.LockEnabledOtherRules(ctx, m.PlatformID, id)
			if err != nil {
				return dependency(err)
			}
			if err = r.DisableRules(ctx, others, now); err != nil {
				return dependency(err)
			}
		}
		if err = r.ReplaceCodes(ctx, id, m.Codes, in.Codes, now); err != nil {
			return mutationWriteError(err)
		}
		values := map[string]any{"name": in.Name, "max_file_size_bytes": in.MaxFileSizeBytes, "allowed_extensions": StringArray(in.AllowedExtensions), "allowed_mime_types": StringArray(in.AllowedMimeTypes), "remark": in.Remark, "updated_at": now}
		if in.IsEnabled != nil {
			values["is_enabled"] = *in.IsEnabled
		}
		return mutationWriteError(r.Update(ctx, id, values))
	})
}

func lockMutationTarget(ctx context.Context, r *Repository, id int64, enabling bool) (Model, error) {
	// Parent identities are immutable; the unlocked read only selects lock keys.
	current, err := r.FindByID(ctx, id)
	if err != nil {
		return Model{}, mutationReadError(err)
	}
	// PUT and PATCH share platform -> config -> target -> enabled peers.
	if enabling {
		err = r.LockPlatform(ctx, current.PlatformID)
	} else {
		err = r.LockPlatformForEdit(ctx, current.PlatformID)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Model{}, conflict(fmt.Errorf("platform unavailable"))
		}
		return Model{}, dependency(err)
	}
	var config cosconfig.Current
	if enabling {
		config, err = r.LockConfig(ctx, current.CosConfigID)
	} else {
		config, err = r.LockConfigForEdit(ctx, current.CosConfigID)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Model{}, conflict(fmt.Errorf("COS config unavailable"))
		}
		return Model{}, dependency(err)
	}
	target, err := r.LockByID(ctx, id)
	if err != nil {
		return Model{}, mutationReadError(err)
	}
	if enabling {
		if err = validatePublicConfig(target.AccessMode, config); err != nil {
			return Model{}, err
		}
	}
	return target, nil
}

func validatePublicConfig(accessMode string, config cosconfig.Current) error {
	if accessMode == "public" && (config.BucketDomain == nil || strings.TrimSpace(*config.BucketDomain) == "") {
		return conflict(fmt.Errorf("public rule requires bucket domain"))
	}
	return nil
}

func mutationReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	return dependency(err)
}
func mutationWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrConflict) {
		return conflict(err)
	}
	return dependency(err)
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, v yesno.Value) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if !yesno.IsValid(v) {
		return invalid(fmt.Errorf("isEnabled invalid"))
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		current, e := lockMutationTarget(ctx, r, id, v == yesno.Yes)
		if e != nil {
			return e
		}
		// Only the locked target may establish a no-op; a concurrent PUT/PATCH
		// can change the status after the parent-key read.
		if current.IsEnabled == v {
			return nil
		}
		now := s.now().UTC()
		if v == yesno.Yes {
			others, e := r.LockEnabledOtherRules(ctx, current.PlatformID, id)
			if e != nil {
				return dependency(e)
			}
			if e := r.DisableRules(ctx, others, now); e != nil {
				return dependency(e)
			}
		}
		return mutationWriteError(r.Update(ctx, id, map[string]any{"is_enabled": v, "updated_at": now}))
	})
}

func normalizeCreateInput(in CreateInput) CreateInput {
	in.Codes = normalize(in.Codes, false)
	in.Name = strings.TrimSpace(in.Name)
	in.Remark = strings.TrimSpace(in.Remark)
	in.AllowedExtensions = normalize(in.AllowedExtensions, true)
	in.AllowedMimeTypes = normalize(in.AllowedMimeTypes, false)
	return in
}
func normalizeUpdateInput(in UpdateInput) UpdateInput {
	in.Codes = normalize(in.Codes, false)
	in.Name = strings.TrimSpace(in.Name)
	in.Remark = strings.TrimSpace(in.Remark)
	in.AllowedExtensions = normalize(in.AllowedExtensions, true)
	in.AllowedMimeTypes = normalize(in.AllowedMimeTypes, false)
	return in
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		m, e := r.LockByID(ctx, id)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		if m.IsEnabled == yesno.Yes {
			return conflict(fmt.Errorf("rule must be disabled"))
		}
		now := time.Now().UTC()
		if e = r.MarkDeleted(ctx, id, now); e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		if e = r.MarkCodesDeleted(ctx, id, now); e != nil {
			return dependency(e)
		}
		return nil
	})
}

func (s *Service) IssueCredentials(ctx context.Context, identity auth.Identity, input CredentialInput) (CredentialResponse, error) {
	if identity.PlatformID < 1 || strings.TrimSpace(input.RuleCode) == "" || len(input.Files) == 0 {
		return CredentialResponse{}, invalid(fmt.Errorf("credential request invalid"))
	}
	target, err := s.repository.FindUploadTarget(ctx, identity.PlatformID, strings.TrimSpace(input.RuleCode))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CredentialResponse{}, notFound(err)
	}
	if err != nil {
		return CredentialResponse{}, dependency(err)
	}
	runtime, version, err := s.currentUploadRuntime(ctx, target.CosConfigID)
	if err != nil {
		return CredentialResponse{}, err
	}
	secretID, secretKey, err := cosconfig.DecryptCredentials(s.keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
	if err != nil {
		return CredentialResponse{}, dependency(err)
	}
	credentials := runtimeCredentials(runtime, version, secretID, secretKey)
	items := make([]CredentialItem, 0, len(input.Files))
	now := time.Now().UTC()
	for _, file := range input.Files {
		ext, err := validateCredentialFile(file, target)
		if err != nil {
			return CredentialResponse{}, invalid(err)
		}
		key, err := objectkey.Generate(target.Code, objectkey.Coordinates{
			PlatformID:  target.PlatformID,
			RuleID:      target.RuleID,
			CosConfigID: target.CosConfigID,
			Version:     runtime.CurrentVersion,
		}, ext, now)
		if err != nil {
			return CredentialResponse{}, dependency(err)
		}
		signed, err := s.signer.PresignPut(ctx, credentials, storagecos.PutRequest{ObjectKey: key, ContentType: strings.ToLower(strings.TrimSpace(file.ContentType)), ContentLength: file.FileSizeBytes, PublicRead: target.AccessMode == "public"})
		if err != nil {
			return CredentialResponse{}, dependency(err)
		}
		item := CredentialItem{UploadURL: signed.URL, ObjectKey: key, Method: http.MethodPut, Headers: signed.Headers, ExpiresAt: now.Add(storagecos.PresignValidity)}
		if target.AccessMode == "public" {
			if version.BucketDomain == nil || strings.TrimSpace(*version.BucketDomain) == "" {
				return CredentialResponse{}, conflict(fmt.Errorf("public bucket domain unavailable"))
			}
			public := publicObjectURL(*version.BucketDomain, key)
			item.PublicURL = &public
		}
		items = append(items, item)
	}
	return CredentialResponse{Items: items}, nil
}

func (s *Service) ObjectURL(ctx context.Context, identity auth.Identity, objectKey string) (ObjectURLResult, error) {
	if identity.PlatformID < 1 {
		return ObjectURLResult{}, invalid(fmt.Errorf("object URL request invalid"))
	}
	coordinates, err := objectkey.Parse(strings.TrimSpace(objectKey))
	if err != nil {
		return ObjectURLResult{}, invalid(err)
	}
	if coordinates.PlatformID != identity.PlatformID {
		return ObjectURLResult{}, apperror.Forbidden(fmt.Errorf("object platform does not match authenticated platform"))
	}
	route, err := s.loadRoute(ctx, coordinates.RuleID)
	if err != nil {
		return ObjectURLResult{}, err
	}
	if route.PlatformID != coordinates.PlatformID || route.CosConfigID != coordinates.CosConfigID {
		return ObjectURLResult{}, invalid(fmt.Errorf("object route coordinates do not match"))
	}
	if s.configRuntime == nil {
		return ObjectURLResult{}, dependency(fmt.Errorf("COS config runtime unavailable"))
	}
	runtime, err := s.configRuntime.Runtime(ctx, coordinates.CosConfigID)
	if err != nil {
		return ObjectURLResult{}, dependency(err)
	}
	if runtime.ID != coordinates.CosConfigID {
		return ObjectURLResult{}, dependency(fmt.Errorf("COS config runtime coordinates do not match"))
	}
	version, found := findRuntimeVersion(runtime, coordinates.Version)
	if !found {
		return ObjectURLResult{}, notFound(fmt.Errorf("COS physical version not found"))
	}
	if route.AccessMode == "public" {
		if version.BucketDomain == nil || strings.TrimSpace(*version.BucketDomain) == "" {
			return ObjectURLResult{}, dependency(fmt.Errorf("public bucket domain unavailable"))
		}
		return ObjectURLResult{URL: publicObjectURL(*version.BucketDomain, objectKey)}, nil
	}
	if s.keys == nil || s.signer == nil {
		return ObjectURLResult{}, dependency(fmt.Errorf("COS signer unavailable"))
	}
	secretID, secretKey, err := cosconfig.DecryptCredentials(s.keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
	if err != nil {
		return ObjectURLResult{}, dependency(err)
	}
	signed, err := s.signer.PresignGet(ctx, runtimeCredentials(runtime, version, secretID, secretKey), storagecos.GetRequest{ObjectKey: objectKey})
	if err != nil {
		return ObjectURLResult{}, dependency(err)
	}
	if strings.TrimSpace(signed.URL) == "" || signed.ExpiresAt.IsZero() {
		return ObjectURLResult{}, dependency(fmt.Errorf("COS GET signer returned an invalid result"))
	}
	expiresAt := signed.ExpiresAt
	return ObjectURLResult{URL: signed.URL, ExpiresAt: &expiresAt}, nil
}

func (s *Service) currentUploadRuntime(ctx context.Context, configID int64) (cosconfig.RuntimeConfig, cosconfig.RuntimeVersion, error) {
	if s.configRuntime == nil || s.keys == nil || s.signer == nil {
		return cosconfig.RuntimeConfig{}, cosconfig.RuntimeVersion{}, dependency(fmt.Errorf("COS upload runtime unavailable"))
	}
	runtime, err := s.configRuntime.Runtime(ctx, configID)
	if err != nil {
		return cosconfig.RuntimeConfig{}, cosconfig.RuntimeVersion{}, dependency(err)
	}
	if runtime.ID != configID || runtime.Deleted || runtime.IsEnabled != yesno.Yes {
		return cosconfig.RuntimeConfig{}, cosconfig.RuntimeVersion{}, conflict(fmt.Errorf("COS config unavailable"))
	}
	version, found := findRuntimeVersion(runtime, runtime.CurrentVersion)
	if !found {
		return cosconfig.RuntimeConfig{}, cosconfig.RuntimeVersion{}, dependency(fmt.Errorf("current COS physical version not found"))
	}
	return runtime, version, nil
}

func findRuntimeVersion(runtime cosconfig.RuntimeConfig, version int64) (cosconfig.RuntimeVersion, bool) {
	for _, candidate := range runtime.Versions {
		if candidate.Version == version {
			return candidate, true
		}
	}
	return cosconfig.RuntimeVersion{}, false
}

func runtimeCredentials(runtime cosconfig.RuntimeConfig, version cosconfig.RuntimeVersion, secretID, secretKey string) storagecos.Credentials {
	credentials := storagecos.Credentials{AppID: runtime.AppID, SecretID: secretID, SecretKey: secretKey, Bucket: version.Bucket, Region: version.Region}
	if version.Endpoint != nil {
		credentials.Endpoint = *version.Endpoint
	}
	return credentials
}

func publicObjectURL(bucketDomain, objectKey string) string {
	return strings.TrimRight(strings.TrimSpace(bucketDomain), "/") + "/" + strings.ReplaceAll(url.PathEscape(objectKey), "%2F", "/")
}
func validateCredentialFile(file FileInput, target UploadTarget) (string, error) {
	name := strings.TrimSpace(file.FileName)
	if name == "" || strings.ContainsAny(name, "/\\\r\n\t") || strings.Contains(name, "..") {
		return "", fmt.Errorf("fileName invalid")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" || !contains(target.AllowedExtensions, ext) {
		return "", fmt.Errorf("extension invalid")
	}
	mime := strings.ToLower(strings.TrimSpace(file.ContentType))
	if len(target.AllowedMimeTypes) > 0 && !contains(target.AllowedMimeTypes, mime) {
		return "", fmt.Errorf("contentType invalid")
	}
	if file.FileSizeBytes < 1 || file.FileSizeBytes > target.MaxFileSizeBytes {
		return "", fmt.Errorf("fileSizeBytes invalid")
	}
	return ext, nil
}
func contains(values StringArray, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
