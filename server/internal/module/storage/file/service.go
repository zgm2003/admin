package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"admin/server/internal/storage/objectKey"
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type RuntimeSource interface {
	Runtime(context.Context, int64) (cosconfig.RuntimeConfig, error)
}
type Service struct {
	repository *Repository
	keys       *secretkey.KeyRing
	cos        storagecos.ObjectClient
	runtime    RuntimeSource
	rules      UploadRules
	now        func() time.Time
}

func NewService(r *Repository, keys *secretkey.KeyRing, client storagecos.ObjectClient, runtime RuntimeSource, rules UploadRules) *Service {
	return &Service{r, keys, client, runtime, rules, time.Now}
}
func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.repository.db == nil || s.repository.scheduler == nil || s.keys == nil || s.cos == nil || s.runtime == nil || s.rules == nil {
		return errors.New("file dependencies unavailable")
	}
	return nil
}
func validIdentity(id auth.Identity) bool { return id.UserID > 0 && id.PlatformID > 0 }
func conflict(e error) error              { return apperror.Conflict(i18n.KeyConflict, nil, e) }
func mapError(e error) error {
	if e == nil {
		return nil
	}
	var app *apperror.Error
	if errors.As(e, &app) {
		return e
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return apperror.NotFound(e)
	}
	if errors.Is(e, ErrConflict) || errors.Is(e, storagecos.ErrObjectChanged) {
		return conflict(e)
	}
	return apperror.DependencyUnavailable(e)
}
func item(m Model) FileItem {
	return FileItem{m.ID, m.OriginalName, m.MimeType, m.SizeBytes, category(m.MimeType), m.CreatedAt, m.ObjectKey}
}
func category(mimeType string) string {
	for _, v := range []string{"image", "audio", "video"} {
		if strings.HasPrefix(mimeType, v+"/") {
			return v
		}
	}
	if strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf" || strings.Contains(mimeType, "word") || strings.Contains(mimeType, "excel") || strings.Contains(mimeType, "spreadsheet") {
		return "document"
	}
	return "other"
}
func (s *Service) RegisterUpload(ctx context.Context, id auth.Identity, in uploadrule.FileRegistration) (int64, error) {
	if !validIdentity(id) || in.SizeBytes < 1 || in.ObjectKey == in.UploadObjectKey || in.ExpiresAt.IsZero() || in.OriginalName == "" || len(in.OriginalName) > 255 || len(in.MimeType) > 255 {
		return 0, apperror.InvalidRequest(errors.New("file registration invalid"))
	}
	now := s.now().UTC()
	m := Model{UserID: id.UserID, PlatformID: id.PlatformID, Kind: KindOriginal, CosConfigID: in.CosConfigID, ConfigVersion: in.ConfigVersion, UploadRuleID: in.UploadRuleID, RuleCode: in.RuleCode, ObjectKey: in.ObjectKey, UploadObjectKey: &in.UploadObjectKey, OriginalName: in.OriginalName, MimeType: in.MimeType, SizeBytes: in.SizeBytes, Status: StatusPending, CredentialExpiresAt: &in.ExpiresAt, CreatedAt: now, UpdatedAt: now}
	e := s.repository.Create(ctx, &m)
	return m.ID, mapError(e)
}
func (s *Service) List(ctx context.Context, id auth.Identity, q ListQuery) (pagination.Result[FileItem], error) {
	if !validIdentity(id) || q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 {
		return pagination.Result[FileItem]{}, apperror.InvalidRequest(errors.New("file page invalid"))
	}
	if q.Purpose == "avatar" {
		target, e := s.rules.FindFileUploadTarget(ctx, id.PlatformID, "avatar")
		if e != nil {
			return pagination.Result[FileItem]{}, mapError(e)
		}
		q.MaxSize = target.MaxFileSizeBytes
		q.MIMEs = []string(target.AllowedMimeTypes)
		q.Extensions = []string(target.AllowedExtensions)
	}
	rows, total, e := s.repository.List(ctx, id, q)
	if e != nil {
		return pagination.Result[FileItem]{}, mapError(e)
	}
	out := make([]FileItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, item(row))
	}
	return pagination.Result[FileItem]{List: out, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
func (s *Service) Get(ctx context.Context, id auth.Identity, fileID int64) (FileItem, error) {
	m, e := s.ownedReady(ctx, id, fileID)
	return item(m), e
}
func (s *Service) ownedReady(ctx context.Context, id auth.Identity, fileID int64) (Model, error) {
	if !validIdentity(id) || fileID < 1 {
		return Model{}, apperror.InvalidRequest(errors.New("file identity invalid"))
	}
	m, e := s.repository.Find(ctx, id, fileID)
	if e != nil {
		return Model{}, mapError(e)
	}
	if m.Kind != KindOriginal || m.Status != StatusReady {
		return Model{}, apperror.NotFound(gorm.ErrRecordNotFound)
	}
	return m, nil
}
func (s *Service) physical(ctx context.Context, m Model) (storagecos.Credentials, cosconfig.RuntimeVersion, error) {
	if s.runtime == nil || s.keys == nil || s.cos == nil {
		return storagecos.Credentials{}, cosconfig.RuntimeVersion{}, apperror.DependencyUnavailable(errors.New("file storage dependencies unavailable"))
	}
	runtime, e := s.runtime.Runtime(ctx, m.CosConfigID)
	if e != nil {
		return storagecos.Credentials{}, cosconfig.RuntimeVersion{}, mapError(e)
	}
	sid, key, e := cosconfig.DecryptCredentials(s.keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
	if e != nil {
		return storagecos.Credentials{}, cosconfig.RuntimeVersion{}, mapError(e)
	}
	for _, v := range runtime.Versions {
		if v.Version == m.ConfigVersion {
			c := storagecos.Credentials{AppID: runtime.AppID, SecretID: sid, SecretKey: key, Bucket: v.Bucket, Region: v.Region}
			if v.Endpoint != nil {
				c.Endpoint = *v.Endpoint
			}
			return c, v, nil
		}
	}
	return storagecos.Credentials{}, cosconfig.RuntimeVersion{}, apperror.DependencyUnavailable(errors.New("file physical version unavailable"))
}
func (s *Service) Complete(ctx context.Context, id auth.Identity, fileID int64) (CompleteResult, error) {
	if !validIdentity(id) || fileID < 1 {
		return CompleteResult{}, apperror.InvalidRequest(errors.New("file identity invalid"))
	}
	m, e := s.repository.Find(ctx, id, fileID)
	if e != nil {
		return CompleteResult{}, mapError(e)
	}
	if m.Kind != KindOriginal || m.Status == StatusCleanup {
		return CompleteResult{}, conflict(ErrConflict)
	}
	credentials, v, e := s.physical(ctx, m)
	if e != nil {
		return CompleteResult{}, e
	}
	if m.Status == StatusReady {
		return s.completeResult(ctx, m, v)
	}
	if m.UploadObjectKey == nil {
		return CompleteResult{}, conflict(ErrConflict)
	}
	if e = s.repository.BeginRemote(ctx, m.ID, s.now().UTC()); e != nil {
		return CompleteResult{}, mapError(e)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Freeze the first validated ETag in PostgreSQL before any remote copy.
	if m.Etag == "" {
		if m.CredentialExpiresAt == nil || !s.now().Before(m.CredentialExpiresAt.Add(time.Minute)) {
			return CompleteResult{}, conflict(ErrConflict)
		}
		head, e := s.cos.HeadObject(ctx, credentials, *m.UploadObjectKey)
		if e != nil {
			return CompleteResult{}, mapError(e)
		}
		if head.Size != m.SizeBytes || head.MIME != m.MimeType {
			return CompleteResult{}, conflict(errors.New("uploaded metadata mismatch"))
		}
		m, e = s.repository.ReserveETag(ctx, id, m.ID, head.ETag, s.now().UTC())
		if e != nil {
			return CompleteResult{}, mapError(e)
		}
	}
	final, e := s.cos.HeadObject(ctx, credentials, m.ObjectKey)
	if errors.Is(e, storagecos.ErrObjectNotFound) {
		public, e2 := s.publicAccess(ctx, m)
		if e2 != nil {
			return CompleteResult{}, e2
		}
		final, e = s.cos.CopyObject(ctx, credentials, credentials, storagecos.CopyRequest{SourceKey: *m.UploadObjectKey, TargetKey: m.ObjectKey, SourceETag: m.Etag, PublicRead: public})
	}
	if e != nil {
		return CompleteResult{}, mapError(e)
	}
	if final.Size != m.SizeBytes || final.MIME != m.MimeType || final.ETag != m.Etag {
		return CompleteResult{}, conflict(errors.New("final metadata mismatch"))
	}
	e = s.repository.Finish(ctx, m.ID, final.ETag, s.now().UTC(), true)
	if errors.Is(e, ErrConflict) {
		current, readErr := s.repository.Find(ctx, id, m.ID)
		if readErr == nil && current.Status == StatusReady {
			e = nil
		}
	}
	if e != nil {
		if errors.Is(e, ErrConflict) {
			if compensate := s.repository.ReopenCleanup(ctx, m.ID, s.now().UTC()); compensate != nil {
				return CompleteResult{}, mapError(errors.Join(e, compensate))
			}
		}
		return CompleteResult{}, mapError(e)
	}
	return s.completeResult(ctx, m, v)
}
func (s *Service) publicAccess(ctx context.Context, m Model) (bool, error) {
	route, e := s.repository.RuleAccess(ctx, m.UploadRuleID)
	return route == "public", mapError(e)
}
func (s *Service) completeResult(ctx context.Context, m Model, v cosconfig.RuntimeVersion) (CompleteResult, error) {
	result := CompleteResult{FileID: m.ID, ObjectKey: m.ObjectKey}
	public, e := s.publicAccess(ctx, m)
	if e != nil {
		return result, e
	}
	if public {
		if v.BucketDomain == nil || *v.BucketDomain == "" {
			return result, apperror.DependencyUnavailable(errors.New("public domain unavailable"))
		}
		u := strings.TrimRight(*v.BucketDomain, "/") + "/" + strings.ReplaceAll(url.PathEscape(m.ObjectKey), "%2F", "/")
		result.PublicURL = &u
	}
	return result, nil
}
func (s *Service) URL(ctx context.Context, id auth.Identity, fileID int64, disposition string) (URLResult, error) {
	if disposition != "inline" && disposition != "attachment" {
		return URLResult{}, apperror.InvalidRequest(errors.New("disposition invalid"))
	}
	m, e := s.ownedReady(ctx, id, fileID)
	if e != nil {
		return URLResult{}, e
	}
	credentials, _, e := s.physical(ctx, m)
	if e != nil {
		return URLResult{}, e
	}
	if disposition == "inline" && m.MimeType != "image/png" && m.MimeType != "image/jpeg" && m.MimeType != "image/gif" && m.MimeType != "image/webp" {
		disposition = "attachment"
	}
	signed, e := s.cos.PresignGet(ctx, credentials, storagecos.GetRequest{ObjectKey: m.ObjectKey, ContentDisposition: mime.FormatMediaType(disposition, map[string]string{"filename": m.OriginalName})})
	return URLResult{signed.URL, signed.ExpiresAt}, mapError(e)
}
func (s *Service) Delete(ctx context.Context, id auth.Identity, fileID int64) error {
	if !validIdentity(id) || fileID < 1 {
		return apperror.InvalidRequest(errors.New("file identity invalid"))
	}
	return mapError(s.repository.Delete(ctx, id, fileID, s.now().UTC()))
}

func (s *Service) AuthorizeObject(ctx context.Context, id auth.Identity, key string) error {
	if strings.HasPrefix(key, ".staging/") {
		return apperror.NotFound(gorm.ErrRecordNotFound)
	}
	if !validIdentity(id) {
		return apperror.Unauthorized(errors.New("file identity unavailable"))
	}
	m, e := s.repository.FindObject(ctx, key)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil
	}
	if e != nil {
		return mapError(e)
	}
	if m.DeletedAt.Valid || m.Status != StatusReady {
		return apperror.NotFound(gorm.ErrRecordNotFound)
	}
	if m.Kind == KindOriginal && (m.UserID != id.UserID || m.PlatformID != id.PlatformID) {
		return apperror.NotFound(gorm.ErrRecordNotFound)
	}
	if m.Kind == KindAvatar {
		if m.UserID != id.UserID || m.PlatformID != id.PlatformID {
			public, e := s.publicAccess(ctx, m)
			if e != nil {
				return e
			}
			if !public {
				return apperror.NotFound(gorm.ErrRecordNotFound)
			}
		}
		current, e := s.repository.IsCurrentAvatar(ctx, m.UserID, m.ObjectKey)
		if e != nil {
			return mapError(e)
		}
		if !current {
			return apperror.NotFound(gorm.ErrRecordNotFound)
		}
	}
	return nil
}
func (s *Service) PrepareAvatar(ctx context.Context, id auth.Identity, sourceID int64, requestKey string) (AvatarCopy, error) {
	parsed, e := uuid.Parse(requestKey)
	if !validIdentity(id) || sourceID < 1 || e != nil || parsed == uuid.Nil {
		return AvatarCopy{}, apperror.InvalidRequest(errors.New("avatar request invalid"))
	}
	requestKey = parsed.String()
	ctx, cancelRemote := context.WithTimeout(ctx, 30*time.Second)
	defer cancelRemote()
	existing, e := s.repository.ByRequest(ctx, id, requestKey)
	if e == nil {
		if existing.Kind != KindAvatar || existing.Status == StatusCleanup || existing.SourceFileID == nil || *existing.SourceFileID != sourceID {
			return AvatarCopy{}, conflict(ErrConflict)
		}
		if existing.CompletedAt != nil {
			return AvatarCopy{existing.ID, existing.ObjectKey, existing.Status == StatusReady}, nil
		}
	} else if !errors.Is(e, gorm.ErrRecordNotFound) {
		return AvatarCopy{}, mapError(e)
	}
	source, e := s.ownedReady(ctx, id, sourceID)
	if e != nil {
		return AvatarCopy{}, e
	}
	target, e := s.rules.FindFileUploadTarget(ctx, id.PlatformID, "avatar")
	if e != nil {
		return AvatarCopy{}, mapError(e)
	}
	if e = uploadrule.ValidateFileForTarget(uploadrule.FileInput{FileName: source.OriginalName, ContentType: source.MimeType, FileSizeBytes: source.SizeBytes}, target); e != nil {
		return AvatarCopy{}, apperror.InvalidRequest(e)
	}
	sourceCredentials, _, e := s.physical(ctx, source)
	if e != nil {
		return AvatarCopy{}, e
	}
	head, e := s.cos.HeadObject(ctx, sourceCredentials, source.ObjectKey)
	if e != nil {
		return AvatarCopy{}, mapError(e)
	}
	if head.ETag != source.Etag || head.Size != source.SizeBytes {
		return AvatarCopy{}, conflict(ErrConflict)
	}
	const maxAvatarRead = 20 * 1024 * 1024
	if source.SizeBytes > maxAvatarRead {
		return AvatarCopy{}, apperror.InvalidRequest(errors.New("image read limit exceeded"))
	}
	data, e := s.cos.ReadObject(ctx, sourceCredentials, source.ObjectKey, source.Etag, source.SizeBytes)
	if e != nil {
		return AvatarCopy{}, mapError(e)
	}
	config, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return AvatarCopy{}, apperror.InvalidRequest(errors.New("avatar image invalid"))
	}
	if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
		return AvatarCopy{}, apperror.InvalidRequest(e)
	}
	m := existing
	if m.ID == 0 {
		runtime, e := s.runtime.Runtime(ctx, target.CosConfigID)
		if e != nil {
			return AvatarCopy{}, mapError(e)
		}
		if runtime.Deleted || runtime.Status != yesno.Yes {
			return AvatarCopy{}, conflict(ErrConflict)
		}
		now := s.now().UTC()
		key, e := objectkey.Generate("avatar", objectkey.Coordinates{PlatformID: id.PlatformID, RuleID: target.RuleID, CosConfigID: target.CosConfigID, Version: runtime.CurrentVersion}, strings.TrimPrefix(strings.ToLower(filepath.Ext(source.OriginalName)), "."), now)
		if e != nil {
			return AvatarCopy{}, mapError(e)
		}
		expiry := now.Add(storagecos.PresignValidity)
		m = Model{UserID: id.UserID, PlatformID: id.PlatformID, Kind: KindAvatar, CosConfigID: target.CosConfigID, ConfigVersion: runtime.CurrentVersion, UploadRuleID: target.RuleID, RuleCode: "avatar", ObjectKey: key, OriginalName: source.OriginalName, MimeType: source.MimeType, SizeBytes: source.SizeBytes, Etag: source.Etag, Status: StatusPending, CredentialExpiresAt: &expiry, RequestKey: &requestKey, SourceFileID: &sourceID, CreatedAt: now, UpdatedAt: now}
		if e = s.repository.Create(ctx, &m); e != nil {
			m, e = s.repository.ByRequest(ctx, id, requestKey)
			if e != nil {
				return AvatarCopy{}, mapError(e)
			}
		}
	}
	if m.SourceFileID == nil || *m.SourceFileID != sourceID {
		return AvatarCopy{}, conflict(ErrConflict)
	}
	if e = s.repository.BeginRemote(ctx, m.ID, s.now().UTC()); e != nil {
		return AvatarCopy{}, mapError(e)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	credentials, _, e := s.physical(ctx, m)
	if e != nil {
		return AvatarCopy{}, e
	}
	final, e := s.cos.HeadObject(ctx, credentials, m.ObjectKey)
	if errors.Is(e, storagecos.ErrObjectNotFound) {
		final, e = s.cos.CopyObject(ctx, sourceCredentials, credentials, storagecos.CopyRequest{SourceKey: source.ObjectKey, TargetKey: m.ObjectKey, SourceETag: source.Etag, PublicRead: target.AccessMode == "public"})
	}
	if e != nil {
		return AvatarCopy{}, mapError(e)
	}
	if final.Size != m.SizeBytes || final.ETag != m.Etag || final.MIME != m.MimeType {
		return AvatarCopy{}, conflict(ErrConflict)
	}
	if e = s.repository.Finish(ctx, m.ID, final.ETag, s.now().UTC(), false); e != nil {
		if compensate := s.repository.ReopenCleanup(ctx, m.ID, s.now().UTC()); compensate != nil {
			return AvatarCopy{}, mapError(errors.Join(e, compensate))
		}
		return AvatarCopy{}, mapError(e)
	}
	return AvatarCopy{m.ID, m.ObjectKey, false}, nil
}
