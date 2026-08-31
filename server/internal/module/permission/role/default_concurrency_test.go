package role_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/permission/role"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestSetDefaultSerializesConcurrentWriters(t *testing.T) {
	db, ctx := openRoleDatabase(t)
	service := newRoleTestService(t, role.NewRepository(db))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	firstID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("concurrent_first_%d", time.Now().UnixNano()), Name: "Concurrent First"})
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("concurrent_second_%d", time.Now().UnixNano()), Name: "Concurrent Second"})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, id := range []int64{firstID, secondID} {
		wait.Add(1)
		go func(roleID int64) {
			defer wait.Done()
			<-start
			results <- service.SetDefault(context.Background(), roleID)
		}(id)
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if roleErrorCode(err) != apperror.CodeConflict {
			t.Fatalf("concurrent SetDefault returned application error: %v", err)
		}
	}
	if successes == 0 {
		t.Fatal("concurrent SetDefault produced no successful writer")
	}

	var count int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM permission_role WHERE is_default = ? AND deleted_at IS NULL", yesno.Yes).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("enabled default role count = %d, want 1", count)
	}
	var enabled int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM permission_role WHERE is_default = ? AND is_enabled = ? AND deleted_at IS NULL", yesno.Yes, yesno.Yes).Scan(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("enabled selected default count = %d, want 1", enabled)
	}
}
