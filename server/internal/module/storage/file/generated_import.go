package file

import (
	"admin/server/internal/module/auth/login"
	"strings"
)

func generatedImportMatches(m Model, id auth.Identity, key, mimeType string, size int64) error {
	if m.UserID != id.UserID || m.PlatformID != id.PlatformID || m.Kind != KindOriginal || m.RequestKey == nil || *m.RequestKey != key || m.DeletedAt.Valid || m.ObjectKey == "" || m.SizeBytes != size || !strings.EqualFold(m.MimeType, mimeType) || (m.Status != StatusPending && m.Status != StatusReady) || (m.Status == StatusReady && m.Etag == "") {
		return ErrConflict
	}
	return nil
}
