package uploadrule

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"context"
	"errors"
	"testing"
)

func TestFileUploadOptionsMatchBusinessPurpose(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	otherPlatformID := insertPlatform(t, db, ctx, "other", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	ruleID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar", "file", "setting"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	for _, purpose := range []string{"", "avatar"} {
		wantCode := "file"
		if purpose == "avatar" {
			wantCode = "avatar"
		}
		rows, err := service.FileUploadOptions(ctx, platformID, purpose)
		if err != nil || len(rows) != 1 {
			t.Fatalf("purpose %q returned %+v, %v; want one business-specific rule", purpose, rows, err)
		}
		if rows[0].Value != wantCode || rows[0].Label != "AVATAR" || rows[0].Accept != ".png,image/png" || rows[0].MaxFileSizeBytes != 1024 {
			t.Fatalf("purpose %q returned %+v", purpose, rows[0])
		}
	}
	rows, err := service.FileUploadOptions(ctx, otherPlatformID, "")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("cross-platform options = %+v, %v", rows, err)
	}
	if err := service.UpdateStatus(ctx, ruleID, yesno.No); err != nil {
		t.Fatal(err)
	}
	rows, err = service.FileUploadOptions(ctx, platformID, "")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("disabled options = %+v, %v", rows, err)
	}
}

func TestFileUploadOptionsDoNotFallbackToUnrelatedCode(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	if _, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar", "setting"}, yesno.Yes)); err != nil {
		t.Fatal(err)
	}
	rows, err := service.FileUploadOptions(ctx, platformID, "")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("missing file code options = %+v, %v; must not fall back to avatar or setting", rows, err)
	}
	if err := db.WithContext(ctx).Exec("UPDATE storage_cos_config SET status=0 WHERE id=?", configID).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = service.FileUploadOptions(ctx, platformID, "avatar")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("disabled config options = %+v, %v", rows, err)
	}
}

func TestFileUploadOptionsRejectInvalidPurposeAndCanceledIO(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	if _, err := service.FileUploadOptions(ctx, platformID, "setting"); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("unknown purpose error = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.FileUploadOptions(canceled, platformID, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query error = %v", err)
	}
}
