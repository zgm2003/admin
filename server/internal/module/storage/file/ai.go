package file

import (
	airun "admin/server/internal/module/ai/run"
	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	storagecos "admin/server/internal/storage/cos"
	"context"
	"errors"
	"fmt"
	"mime"
	"strconv"
	"time"
)

func validateAIFrozen(m Model, f airun.InputFileSnapshot, maxBytes int64) error {
	if maxBytes < 1 || f.ID != strconv.FormatInt(m.ID, 10) || f.ObjectKey != m.ObjectKey || f.MIMEType != m.MimeType || f.SizeBytes != m.SizeBytes || f.ETag == "" || f.ETag != m.Etag || m.SizeBytes < 1 || m.SizeBytes > maxBytes || m.Kind != KindOriginal || m.Status != StatusReady || m.DeletedAt.Valid {
		return ErrConflict
	}
	return nil
}

func (s *Service) ReadAIFrozenReference(ctx context.Context, userID, platformID int64, f airun.InputFileSnapshot, maxBytes int64) ([]byte, string, string, error) {
	id, err := strconv.ParseInt(f.ID, 10, 64)
	if err != nil || id < 1 || userID < 1 || platformID < 1 {
		return nil, "", "", apperror.InvalidRequest(nil)
	}
	m, err := s.repository.Find(ctx, auth.Identity{UserID: userID, PlatformID: platformID}, id)
	if err != nil {
		return nil, "", "", mapError(err)
	}
	if err = validateAIFrozen(m, f, maxBytes); err != nil {
		return nil, "", "", mapError(err)
	}
	credentials, _, err := s.physical(ctx, m)
	if err != nil {
		return nil, "", "", err
	}
	data, err := s.cos.ReadObject(ctx, credentials, m.ObjectKey, m.Etag, maxBytes)
	if err != nil {
		return nil, "", "", mapError(err)
	}
	if int64(len(data)) != f.SizeBytes {
		return nil, "", "", mapError(ErrConflict)
	}
	return data, m.MimeType, m.OriginalName, nil
}

func (r *Repository) FindAIOutput(ctx context.Context, runID, mediaID, userID, platformID int64, admin bool) (Model, error) {
	var m Model
	q := r.db.WithContext(ctx).Table("storage_file s").Select("s.*").Joins("JOIN ai_media_file f ON f.storage_file_id=s.id AND f.role=2 AND f.transfer_status=2").Joins("JOIN ai_run r ON r.id=f.run_id").Where("f.id=? AND f.run_id=? AND s.status=2 AND s.deleted_at IS NULL", mediaID, runID)
	if !admin {
		q = q.Where("r.user_id=? AND r.platform_id=?", userID, platformID)
	}
	err := q.Take(&m).Error
	return m, err
}

// AIOutputURL is a narrow read surface for an already permission-checked Run
// endpoint. It verifies the exact Run/media relation; it never impersonates
// another owner or widens the ordinary storage file endpoint.
func (s *Service) AIOutputURL(ctx context.Context, runID, mediaID, userID, platformID int64, admin bool) (airun.MediaPreview, error) {
	if runID < 1 || mediaID < 1 || userID < 1 || platformID < 1 {
		return airun.MediaPreview{}, apperror.InvalidRequest(nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	m, err := s.repository.FindAIOutput(ctx, runID, mediaID, userID, platformID, admin)
	if err != nil {
		return airun.MediaPreview{}, mapError(err)
	}
	credentials, _, err := s.physical(ctx, m)
	if err != nil {
		return airun.MediaPreview{}, err
	}
	disposition := "attachment"
	switch m.MimeType {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "video/mp4", "video/webm":
		disposition = "inline"
	}
	name := m.OriginalName
	if name == "" {
		name = fmt.Sprintf("ai-%d", mediaID)
	}
	signed, err := s.cos.PresignGet(ctx, credentials, storagecos.GetRequest{ObjectKey: m.ObjectKey, ContentDisposition: mime.FormatMediaType(disposition, map[string]string{"filename": name})})
	if err != nil {
		return airun.MediaPreview{}, mapError(err)
	}
	if signed.URL == "" || !signed.ExpiresAt.After(time.Now()) {
		return airun.MediaPreview{}, apperror.DependencyUnavailable(errors.New("AI media URL unavailable"))
	}
	return airun.MediaPreview{URL: signed.URL, ExpiresAt: signed.ExpiresAt.UTC().Format(time.RFC3339)}, nil
}
