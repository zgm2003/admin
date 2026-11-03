package run

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

type frozenStorageRow struct {
	ID, UserID, PlatformID, SizeBytes int64
	Kind, Status                      int16
	ObjectKey, MimeType, Etag         string
	DeletedAt                         *time.Time
}

func lockRunInputs(ctx context.Context, tx *gorm.DB, in CreateInput) (RunInput, error) {
	value, err := decodeInput(in.Input)
	if err != nil {
		return value, err
	}
	if len(value.FileIDs) != len(value.FileSnapshots) {
		return value, ErrInvalidSnapshot
	}
	if len(value.FileIDs) == 0 {
		return value, nil
	}
	ids := make([]int64, 0, len(value.FileIDs))
	seen := map[int64]bool{}
	for i, f := range value.FileSnapshots {
		id, e := strconv.ParseInt(f.ID, 10, 64)
		if e != nil || id < 1 || seen[id] || value.FileIDs[i] != f.ID {
			return value, ErrInvalidSnapshot
		}
		seen[id] = true
		ids = append(ids, id)
	}
	var rows []frozenStorageRow
	if err := tx.WithContext(ctx).Table("storage_file").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&rows).Error; err != nil {
		return value, err
	}
	if len(rows) != len(ids) {
		return value, ErrInvalidSnapshot
	}
	byID := map[int64]frozenStorageRow{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for i, id := range ids {
		row, f := byID[id], value.FileSnapshots[i]
		if row.UserID != in.UserID || row.PlatformID != in.PlatformID || row.Kind != 1 || row.Status != 2 || row.DeletedAt != nil || row.ObjectKey != f.ObjectKey || row.MimeType != f.MIMEType || row.SizeBytes != f.SizeBytes || row.Etag == "" || row.Etag != f.ETag {
			return value, ErrInvalidSnapshot
		}
	}
	return value, nil
}

func appendRunInputs(ctx context.Context, tx *gorm.DB, runID int64, input RunInput, now time.Time) error {
	for i, f := range input.FileSnapshots {
		id, err := strconv.ParseInt(f.ID, 10, 64)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{"etag": f.ETag})
		if err != nil {
			return err
		}
		if err = tx.WithContext(ctx).Exec(`INSERT INTO ai_media_file(run_id,role,ordinal,storage_file_id,source_kind,source_recoverable,mime_type,size_bytes,metadata,transfer_status,transfer_attempts,created_at,updated_at) VALUES(?,1,?,?,0,0,?,?,?::jsonb,2,0,?,?)`, runID, i, id, f.MIMEType, f.SizeBytes, string(metadata), now, now).Error; err != nil {
			return err
		}
	}
	return nil
}
