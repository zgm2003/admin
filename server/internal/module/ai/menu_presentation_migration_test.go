package ai

import (
	"context"
	"os"
	"testing"

	"gorm.io/gorm"
)

func aiMenuPresentationFixture(t *testing.T) (*gorm.DB, context.Context, string) {
	t.Helper()
	db, ctx, menuSQL := aiMenuFixture(t)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(menuSQL).Error }); err != nil {
		t.Fatal(err)
	}
	presentationSQL, err := os.ReadFile("../../../../docs/database/2026-11-04-ai-menu-presentation.sql")
	if err != nil {
		t.Fatal(err)
	}
	return db, ctx, string(presentationSQL)
}

func TestAIMenuPresentationMigrationSetsIconsAndStableOrderAndReplays(t *testing.T) {
	db, ctx, sql := aiMenuPresentationFixture(t)
	apply := func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Code      string
		Icon      *string
		SortOrder int
	}
	if err := db.WithContext(ctx).Raw(`
SELECT code,icon,sort_order
FROM permission_menu
WHERE platform_id=1 AND (code='ai' OR code IN (
 'ai:officialModel:view','ai:provider:view','ai:agent:view','ai:chat:view','ai:run:view','ai:tool:view'
))
ORDER BY sort_order,id`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	want := []struct {
		code, icon string
		order      int
	}{
		{"ai:officialModel:view", "lucide:brain-circuit", 10},
		{"ai:provider:view", "lucide:server", 20},
		{"ai:agent:view", "lucide:bot", 30},
		{"ai:chat:view", "lucide:message-square-more", 40},
		{"ai:run:view", "lucide:activity", 50},
		{"ai:tool:view", "lucide:settings-2", 60},
		{"ai", "lucide:sparkles", 900},
	}
	if len(rows) != len(want) {
		t.Fatalf("AI menu rows=%+v", rows)
	}
	for i, row := range rows {
		if row.Code != want[i].code || row.Icon == nil || *row.Icon != want[i].icon || row.SortOrder != want[i].order {
			t.Fatalf("AI menu row %d = %+v, want %+v", i, row, want[i])
		}
	}
	var version int64
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&version).Error; err != nil || version != 26 {
		t.Fatalf("first migration menu version=%d, err=%v", version, err)
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&version).Error; err != nil || version != 26 {
		t.Fatalf("replay menu version=%d, err=%v", version, err)
	}
	var canvasVersion int64
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=2`).Scan(&canvasVersion).Error; err != nil || canvasVersion != 5 {
		t.Fatalf("Canvas menu version=%d, err=%v", canvasVersion, err)
	}
}
