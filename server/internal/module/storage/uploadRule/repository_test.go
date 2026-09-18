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
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	ruleID, err := service.Create(ctx, validCreate(adminID, configID, []string{"avatar"}, yesno.Yes))
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

func TestFindConfigSummariesUsesCurrentPhysicalVersionAndOnlyEnabledConfigs(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	currentID := insertConfig(t, db, ctx, "current", yesno.Yes, "")
	disabledID := insertConfig(t, db, ctx, "disabled", yesno.No, "")

	if err := db.WithContext(ctx).Exec(`
		INSERT INTO storage_cos_config_version
			(cos_config_id, version, bucket, region)
		VALUES (?, 2, 'assets-v2', 'ap-shanghai')
	`, currentID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(
		"UPDATE storage_cos_config SET current_version = 2 WHERE id = ?",
		currentID,
	).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := NewRepository(db).FindConfigSummaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("summaries = %+v, want one enabled config", rows)
	}
	if rows[0].ID != currentID || rows[0].Name != "current" || rows[0].Bucket != "assets-v2" || rows[0].Region != "ap-shanghai" || rows[0].IsEnabled != yesno.Yes {
		t.Fatalf("summary = %+v, want current physical version for config %d", rows[0], currentID)
	}
	for _, row := range rows {
		if row.ID == disabledID {
			t.Fatalf("disabled config %d returned in summaries", disabledID)
		}
	}
}
