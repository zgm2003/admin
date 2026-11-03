package file

import (
	airun "admin/server/internal/module/ai/run"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	objectkey "admin/server/internal/storage/objectKey"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/url"
	"path/filepath"
	"strconv"
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

// ImportGeneratedMedia streams a validated AI result into COS and creates a
// ready storage_file row. The request key and deterministic object key make
// retries idempotent across worker restarts.
func (s *Service) ImportGeneratedMedia(ctx context.Context, id auth.Identity, in GeneratedMediaImport) (int64, error) {
	if !validIdentity(id) || s == nil || in.Body == nil || in.SizeBytes < 1 || in.OriginalName == "" || len(in.OriginalName) > 255 || in.MIME == "" || len(in.MIME) > 255 {
		return 0, apperror.InvalidRequest(errors.New("generated media import invalid"))
	}
	parsed, err := uuid.Parse(in.ImportRequestKey)
	if err != nil {
		return 0, apperror.InvalidRequest(errors.New("generated media request key invalid"))
	}
	requestKey := parsed.String()
	mimeType, _, err := mime.ParseMediaType(in.MIME)
	if err != nil {
		return 0, apperror.InvalidRequest(errors.New("generated media MIME invalid"))
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if s.repository == nil || s.repository.db == nil {
		return 0, apperror.DependencyUnavailable(nil)
	}
	frozen, err := s.repository.ByRequest(ctx, id, requestKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, mapError(err)
	}
	if err == nil {
		if err := generatedImportMatches(frozen, id, requestKey, mimeType, in.SizeBytes); err != nil {
			return 0, mapError(err)
		}
		if frozen.Status == StatusReady {
			return frozen.ID, nil
		}
	}
	if s.runtime == nil || s.keys == nil || s.cos == nil {
		return 0, apperror.DependencyUnavailable(errors.New("generated media storage unavailable"))
	}
	if frozen.ID == 0 {
		if s.rules == nil {
			return 0, apperror.DependencyUnavailable(nil)
		}
		var ruleCode string
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			ruleCode = "ai/image"
		case strings.HasPrefix(mimeType, "audio/"):
			ruleCode = "ai/audio"
		case strings.HasPrefix(mimeType, "video/"):
			ruleCode = "ai/video"
		default:
			return 0, apperror.InvalidRequest(errors.New("unsupported generated media MIME"))
		}
		target, err := s.rules.FindFileUploadTarget(ctx, id.PlatformID, ruleCode)
		if err != nil {
			return 0, mapError(err)
		}
		extension := mediaExtension(mimeType, in.OriginalName)
		// Preserve a supported source extension when MIME has multiple standard spellings.
		nameExtension := strings.TrimPrefix(strings.ToLower(filepath.Ext(in.OriginalName)), ".")
		if known, _ := mime.ExtensionsByType(mimeType); len(known) > 0 {
			for _, candidate := range known {
				if strings.TrimPrefix(candidate, ".") == nameExtension {
					extension = nameExtension
					break
				}
			}
		}
		allowedExtension := false
		for _, candidate := range target.AllowedExtensions {
			if strings.EqualFold(strings.TrimPrefix(candidate, "."), extension) {
				allowedExtension = true
				break
			}
		}
		if target.MaxFileSizeBytes < 1 || in.SizeBytes > target.MaxFileSizeBytes || !allowedUploadMIME(mimeType, []string(target.AllowedMimeTypes)) || !allowedExtension {
			return 0, apperror.InvalidRequest(errors.New("generated media exceeds storage rule"))
		}
		runtime, err := s.runtime.Runtime(ctx, target.CosConfigID)
		if err != nil {
			return 0, mapError(err)
		}
		if _, ok := currentVersion(runtime); !ok {
			return 0, apperror.DependencyUnavailable(errors.New("generated media COS version unavailable"))
		}
		now := s.now().UTC()
		seed := fmt.Sprintf("%d:%d:%s", id.PlatformID, id.UserID, requestKey)
		key, err := objectkey.GenerateStable(target.Code, objectkey.Coordinates{PlatformID: id.PlatformID, RuleID: target.RuleID, CosConfigID: target.CosConfigID, Version: runtime.CurrentVersion}, extension, seed, now)
		if err != nil {
			return 0, mapError(err)
		}
		frozen = Model{UserID: id.UserID, PlatformID: id.PlatformID, Kind: KindOriginal, CosConfigID: target.CosConfigID, ConfigVersion: runtime.CurrentVersion, UploadRuleID: target.RuleID, RuleCode: target.Code, ObjectKey: key, OriginalName: in.OriginalName, MimeType: mimeType, SizeBytes: in.SizeBytes, Status: StatusPending, RequestKey: &requestKey, CreatedAt: now, UpdatedAt: now}
		// Register the exact physical destination before any remote side effect. A
		// retry after midnight or a COS version switch must use this persisted key.
		if createErr := s.repository.Create(ctx, &frozen); createErr != nil {
			var findErr error
			frozen, findErr = s.repository.ByRequest(ctx, id, requestKey)
			if findErr != nil {
				return 0, mapError(createErr)
			}
			if err := generatedImportMatches(frozen, id, requestKey, mimeType, in.SizeBytes); err != nil {
				return 0, mapError(err)
			}
			if frozen.Status == StatusReady {
				return frozen.ID, nil
			}
		}
	}
	coordinates, err := objectkey.Parse(frozen.ObjectKey)
	if err != nil || coordinates.PlatformID != id.PlatformID || coordinates.RuleID != frozen.UploadRuleID || coordinates.CosConfigID != frozen.CosConfigID || coordinates.Version != frozen.ConfigVersion {
		return 0, conflict(errors.New("generated media destination changed"))
	}
	runtime, err := s.runtime.Runtime(ctx, frozen.CosConfigID)
	if err != nil {
		return 0, mapError(err)
	}
	var version cosconfig.RuntimeVersion
	for _, candidate := range runtime.Versions {
		if candidate.Version == frozen.ConfigVersion {
			version = candidate
			break
		}
	}
	if version.Version == 0 {
		return 0, apperror.DependencyUnavailable(errors.New("generated media frozen COS version unavailable"))
	}
	secretID, secretKey, err := cosconfig.DecryptCredentials(s.keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
	if err != nil {
		return 0, mapError(err)
	}
	credentials := storagecos.Credentials{AppID: runtime.AppID, SecretID: secretID, SecretKey: secretKey, Bucket: version.Bucket, Region: version.Region}
	if version.Endpoint != nil {
		credentials.Endpoint = *version.Endpoint
	}
	remote, err := s.cos.HeadObject(ctx, credentials, frozen.ObjectKey)
	if errors.Is(err, storagecos.ErrObjectNotFound) {
		access, accessErr := s.repository.RuleAccess(ctx, frozen.UploadRuleID)
		if accessErr != nil {
			return 0, mapError(accessErr)
		}
		putter, ok := s.cos.(storagecos.StreamPutter)
		if !ok {
			return 0, apperror.DependencyUnavailable(errors.New("COS stream import unavailable"))
		}
		remote, err = putter.PutStream(ctx, credentials, storagecos.PutStreamRequest{ObjectKey: frozen.ObjectKey, ContentType: mimeType, ContentLength: in.SizeBytes, Body: in.Body, PublicRead: access == "public"})
	}
	if err != nil {
		return 0, mapError(err)
	}
	if remote.ETag == "" || remote.Size != in.SizeBytes || !strings.EqualFold(strings.TrimSpace(strings.Split(remote.MIME, ";")[0]), mimeType) {
		return 0, conflict(errors.New("generated media remote identity mismatch"))
	}
	if err = s.repository.Finish(ctx, frozen.ID, remote.ETag, s.now().UTC(), true); err != nil {
		existing, findErr := s.repository.ByRequest(ctx, id, requestKey)
		if findErr != nil || existing.Status != StatusReady || existing.Etag != remote.ETag {
			return 0, mapError(err)
		}
		if err := generatedImportMatches(existing, id, requestKey, mimeType, in.SizeBytes); err != nil {
			return 0, mapError(err)
		}
	}
	return frozen.ID, nil
}

// ValidateAIReferences verifies ownership and freezes the physical file
// identity used by a Run. It intentionally returns no mutable storage model
// pointer; the Run stores this value in its immutable input snapshot.
func (s *Service) ValidateAIReferences(ctx context.Context, userID, platformID int64, ids []string, operation string) ([]airun.InputFileSnapshot, error) {
	id := auth.Identity{UserID: userID, PlatformID: platformID}
	if !validIdentity(id) || len(ids) == 0 || len(ids) > 100 || (operation != "image.edit" && operation != "audio.transcribe" && operation != "text.generate") {
		return nil, apperror.InvalidRequest(errors.New("AI file references invalid"))
	}
	numeric := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, raw := range ids {
		fileID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || fileID < 1 || seen[fileID] {
			return nil, apperror.InvalidRequest(errors.New("AI file reference invalid or duplicated"))
		}
		seen[fileID] = true
		numeric = append(numeric, fileID)
	}
	rows, err := s.repository.FindAIInputs(ctx, id, numeric)
	if err != nil {
		return nil, mapError(err)
	}
	if len(rows) != len(ids) {
		return nil, apperror.NotFound(nil)
	}
	byID := make(map[int64]Model, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}
	out := make([]airun.InputFileSnapshot, 0, len(ids))
	for _, fileID := range numeric {
		m := byID[fileID]
		if m.Kind != KindOriginal || m.Status != StatusReady || m.DeletedAt.Valid || m.ObjectKey == "" || m.Etag == "" || m.SizeBytes < 1 {
			return nil, apperror.InvalidRequest(errors.New("AI file reference is unavailable"))
		}
		prefix := "image/"
		if operation == "audio.transcribe" {
			prefix = "audio/"
		}
		if !strings.HasPrefix(strings.ToLower(m.MimeType), prefix) {
			return nil, apperror.InvalidRequest(errors.New("AI file type does not match operation"))
		}
		out = append(out, airun.InputFileSnapshot{ID: strconv.FormatInt(m.ID, 10), ObjectKey: m.ObjectKey, MIMEType: m.MimeType, SizeBytes: m.SizeBytes, ETag: m.Etag})
	}
	return out, nil
}

// ReadAIReference returns a bounded copy of an owned ready file for provider
// multipart requests. The object is fetched through the frozen COS version
// and conditioned on the stored ETag so a replacement cannot be forwarded.
func (s *Service) ReadAIReference(ctx context.Context, userID, platformID int64, fileID, maxBytes int64) ([]byte, string, string, error) {
	id := auth.Identity{UserID: userID, PlatformID: platformID}
	if !validIdentity(id) || fileID < 1 || maxBytes < 1 {
		return nil, "", "", apperror.InvalidRequest(errors.New("AI file read invalid"))
	}
	m, err := s.repository.Find(ctx, id, fileID)
	if err != nil {
		return nil, "", "", mapError(err)
	}
	if m.Kind != KindOriginal || m.Status != StatusReady || m.DeletedAt.Valid || m.Etag == "" || m.SizeBytes < 1 || m.SizeBytes > maxBytes {
		return nil, "", "", apperror.InvalidRequest(errors.New("AI file is unavailable"))
	}
	credentials, _, err := s.physical(ctx, m)
	if err != nil {
		return nil, "", "", err
	}
	data, err := s.cos.ReadObject(ctx, credentials, m.ObjectKey, m.Etag, maxBytes)
	if err != nil {
		return nil, "", "", mapError(err)
	}
	if int64(len(data)) != m.SizeBytes {
		return nil, "", "", conflict(errors.New("AI file size changed"))
	}
	return data, m.MimeType, m.OriginalName, nil
}

func currentVersion(runtime cosconfig.RuntimeConfig) (cosconfig.RuntimeVersion, bool) {
	for _, version := range runtime.Versions {
		if version.Version == runtime.CurrentVersion {
			return version, true
		}
	}
	return cosconfig.RuntimeVersion{}, false
}

func allowedUploadMIME(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(strings.Split(candidate, ";")[0]), value) {
			return true
		}
	}
	return false
}

func mediaExtension(mimeType, name string) string {
	if values, _ := mime.ExtensionsByType(mimeType); len(values) > 0 {
		return strings.TrimPrefix(values[0], ".")
	}
	if ext := filepath.Ext(name); ext != "" {
		return strings.TrimPrefix(strings.ToLower(ext), ".")
	}
	return "bin"
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
