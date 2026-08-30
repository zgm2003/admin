package uploadrule

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosconfig"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	repository *Repository
	keys       *secretkey.KeyRing
	signer     storagecos.Presigner
}

func NewService(repository *Repository, keys *secretkey.KeyRing, signer storagecos.Presigner) *Service {
	return &Service{repository: repository, keys: keys, signer: signer}
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
	platformOK, err := s.repository.PlatformEnabled(ctx, in.PlatformID)
	if err != nil {
		return 0, dependency(err)
	}
	if !platformOK {
		return 0, conflict(fmt.Errorf("platform unavailable"))
	}
	config, err := s.repository.Config(ctx, in.CosConfigID)
	if err != nil {
		return 0, conflict(fmt.Errorf("COS config unavailable"))
	}
	if in.AccessMode == "public" && (config.BucketDomain == nil || strings.TrimSpace(*config.BucketDomain) == "") {
		return 0, conflict(fmt.Errorf("public rule requires bucket domain"))
	}
	var id int64
	err = s.repository.Transaction(ctx, func(r *Repository) error {
		var e error
		id, e = repositoryCreate(r, ctx, in)
		return e
	})
	return id, err
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
	in = normalizeUpdateInput(in)
	if e := validateFields(1, []string{"x"}, in.Name, in.CosConfigID, in.MaxFileSizeBytes, in.AllowedExtensions, in.AllowedMimeTypes, in.AccessMode, in.Remark, false); e != nil {
		return invalid(e)
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		m, e := r.LockByID(ctx, id)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		platformOK, e := r.PlatformEnabled(ctx, m.PlatformID)
		if e != nil || !platformOK {
			return conflict(fmt.Errorf("platform unavailable"))
		}
		config, e := r.Config(ctx, in.CosConfigID)
		if e != nil {
			return conflict(fmt.Errorf("COS config unavailable"))
		}
		if in.AccessMode == "public" && (config.BucketDomain == nil || strings.TrimSpace(*config.BucketDomain) == "") {
			return conflict(fmt.Errorf("public rule requires bucket domain"))
		}
		if e = r.Update(ctx, id, map[string]any{"name": in.Name, "cos_config_id": in.CosConfigID, "max_file_size_bytes": in.MaxFileSizeBytes, "allowed_extensions": StringArray(in.AllowedExtensions), "allowed_mime_types": StringArray(in.AllowedMimeTypes), "access_mode": in.AccessMode, "remark": in.Remark, "updated_at": time.Now().UTC()}); e != nil {
			return dependency(e)
		}
		_ = m
		return nil
	})
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, v yesno.Value) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if !yesno.IsValid(v) {
		return invalid(fmt.Errorf("isEnabled invalid"))
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		var m Model
		var e error
		if v == yesno.Yes {
			m, e = r.FindByID(ctx, id)
			if e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return notFound(e)
				}
				return dependency(e)
			}
			if _, e = r.Config(ctx, m.CosConfigID); e != nil {
				return conflict(fmt.Errorf("COS config unavailable"))
			}
		} else {
			m, e = r.LockByID(ctx, id)
			if e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return notFound(e)
				}
				return dependency(e)
			}
		}
		if e = r.Update(ctx, id, map[string]any{"is_enabled": v, "updated_at": time.Now().UTC()}); e != nil {
			if errors.Is(e, ErrConflict) {
				return conflict(e)
			}
			return dependency(e)
		}
		return nil
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
	if s.keys == nil || s.signer == nil {
		return CredentialResponse{}, dependency(fmt.Errorf("COS signer unavailable"))
	}
	if target.SecretIDCiphertext == nil || target.SecretKeyCiphertext == nil {
		return CredentialResponse{}, dependency(fmt.Errorf("COS credentials unavailable"))
	}
	secretID, secretKey, err := cosconfig.DecryptCredentials(s.keys.StorageEncryptionKey(), *target.SecretIDCiphertext, *target.SecretKeyCiphertext)
	if err != nil {
		return CredentialResponse{}, dependency(err)
	}
	credentials := storagecos.Credentials{AppID: target.AppID, SecretID: secretID, SecretKey: secretKey, Bucket: target.Bucket, Region: target.Region}
	if target.Endpoint != nil {
		credentials.Endpoint = *target.Endpoint
	}
	items := make([]CredentialItem, 0, len(input.Files))
	now := time.Now().UTC()
	for _, file := range input.Files {
		ext, err := validateCredentialFile(file, target)
		if err != nil {
			return CredentialResponse{}, invalid(err)
		}
		key, err := generateObjectKey(target.Code, ext, now)
		if err != nil {
			return CredentialResponse{}, dependency(err)
		}
		signed, err := s.signer.PresignPut(ctx, credentials, storagecos.PutRequest{ObjectKey: key, ContentType: strings.ToLower(strings.TrimSpace(file.ContentType)), ContentLength: file.FileSizeBytes, PublicRead: target.AccessMode == "public"})
		if err != nil {
			return CredentialResponse{}, dependency(err)
		}
		item := CredentialItem{UploadURL: signed.URL, ObjectKey: key, Method: http.MethodPut, Headers: signed.Headers, ExpiresAt: now.Add(storagecos.PresignValidity)}
		if target.AccessMode == "public" {
			if target.BucketDomain == nil || strings.TrimSpace(*target.BucketDomain) == "" {
				return CredentialResponse{}, conflict(fmt.Errorf("public bucket domain unavailable"))
			}
			public := strings.TrimRight(*target.BucketDomain, "/") + "/" + strings.ReplaceAll(url.PathEscape(key), "%2F", "/")
			item.PublicURL = &public
		}
		items = append(items, item)
	}
	return CredentialResponse{Items: items}, nil
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
func generateObjectKey(prefix, extension string, now time.Time) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%04d/%02d/%02d/%s.%s", strings.Trim(prefix, "/"), now.Year(), now.Month(), now.Day(), hex.EncodeToString(random), extension), nil
}
