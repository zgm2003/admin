package main

import (
	aimedia "admin/server/internal/module/ai/media"
	airun "admin/server/internal/module/ai/run"
	storagefile "admin/server/internal/module/storage/file"
	"context"
)

// These two concrete services retain their own data/authorization boundaries.
// The HTTP layer invokes only the Run-facing service contract.
type aiMediaAccess struct {
	files *storagefile.Service
	retry *aimedia.TransferRetryService
}

func (a aiMediaAccess) Preview(ctx context.Context, runID, mediaID, userID, platformID int64, admin bool) (airun.MediaPreview, error) {
	return a.files.AIOutputURL(ctx, runID, mediaID, userID, platformID, admin)
}
func (a aiMediaAccess) Retry(ctx context.Context, runID, userID, platformID int64) error {
	return a.retry.Retry(ctx, runID, userID, platformID)
}

var _ airun.MediaAccess = aiMediaAccess{}
