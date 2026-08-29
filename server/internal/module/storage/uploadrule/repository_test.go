package uploadrule

import (
	"errors"
	"testing"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestFindUploadTargetIsPlatformScopedAndRequiresEnabledRows(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	adminID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	canvasID := insertPlatform(t, db, ctx, "canvas", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db))
	ruleID, err := service.Create(ctx, validCreate(adminID, configID, "avatar", yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	target, err := repository.FindUploadTarget(ctx, adminID, "avatar")
	if err != nil || target.RuleID != ruleID || target.PlatformID != adminID {
		t.Fatalf("target = %+v,%v", target, err)
	}
	if _, err := repository.FindUploadTarget(ctx, canvasID, "avatar"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-platform target error = %v", err)
	}
	if err := service.UpdateStatus(ctx, ruleID, yesno.No); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindUploadTarget(ctx, adminID, "avatar"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("disabled target error = %v", err)
	}
}
