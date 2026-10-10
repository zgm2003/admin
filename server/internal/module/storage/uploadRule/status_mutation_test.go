package uploadrule

import (
	"context"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type statusReadContextKey struct{}

func TestStatusMutationRechecksNoOpAfterConcurrentAtomicSave(t *testing.T) {
	for _, requested := range []yesno.Value{yesno.No, yesno.Yes} {
		t.Run(map[yesno.Value]string{yesno.No: "disable", yesno.Yes: "enable"}[requested], func(t *testing.T) {
			db, ctx := openRuleDatabase(t)
			platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
			configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
			service := NewService(NewRepository(db), nil, nil, nil, nil)
			ruleID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar"}, requested))
			if err != nil {
				t.Fatal(err)
			}

			// Pause the PATCH after its real unlocked read. A second service commits
			// an atomic PUT before PATCH resumes, invalidating the initial no-op fact.
			readDone := make(chan struct{})
			resumeRead := make(chan struct{})
			var paused atomic.Bool
			if err = db.Callback().Query().After("gorm:query").Register("test:pause_status_read", func(query *gorm.DB) {
				if query.Statement.Context.Value(statusReadContextKey{}) != true || query.Statement.Table != "storage_upload_rule" {
					return
				}
				if _, ok := query.Statement.Dest.(*Model); !ok || !paused.CompareAndSwap(false, true) {
					return
				}
				close(readDone)
				select {
				case <-resumeRead:
				case <-query.Statement.Context.Done():
				}
			}); err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() {
				finished <- service.UpdateStatus(context.WithValue(ctx, statusReadContextKey{}, true), ruleID, requested)
			}()
			select {
			case <-readDone:
			case <-ctx.Done():
				close(resumeRead)
				t.Fatal("status request did not reach its initial database read")
			}
			opposite := yesno.Yes
			if requested == yesno.Yes {
				opposite = yesno.No
			}
			otherService := NewService(NewRepository(db), nil, nil, nil, nil)
			input := atomicUpdateInput(t, "Concurrent change", "avatar", int(opposite))
			concurrentErr := otherService.Update(ctx, ruleID, input)
			close(resumeRead)
			select {
			case err = <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("status mutation failed to complete")
			}
			if concurrentErr != nil || err != nil {
				t.Fatalf("concurrent PUT=%v status PATCH=%v", concurrentErr, err)
			}
			saved, err := service.Get(ctx, ruleID)
			if err != nil || saved.IsEnabled != requested || saved.Name != "Concurrent change" {
				t.Fatalf("status request used stale no-op fact: saved=%+v err=%v", saved, err)
			}
		})
	}
}

func TestStatusEnableRejectsPublicRuleWithoutCurrentBucketDomain(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "https://cdn.example.com")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	peerID, err := service.Create(ctx, validCreate(platformID, configID, []string{"peer"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	input := validCreate(platformID, configID, []string{"public"}, yesno.No)
	input.AccessMode = "public"
	ruleID, err := service.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []any{nil, "   "} {
		if err = db.WithContext(ctx).Exec("UPDATE storage_cos_config_version SET bucket_domain=? WHERE cos_config_id=? AND version=1", domain, configID).Error; err != nil {
			t.Fatal(err)
		}
		if err = service.UpdateStatus(ctx, ruleID, yesno.Yes); appCode(err) != apperror.CodeConflict {
			t.Fatalf("public enable without domain returned %v", err)
		}
		assertEnabledRule(t, db, ctx, platformID, peerID)
	}
}

func TestStatusMutationValidatesEnabledParentsAndAllowsDisable(t *testing.T) {
	for _, parent := range []string{"platform", "config"} {
		t.Run(parent, func(t *testing.T) {
			db, ctx := openRuleDatabase(t)
			platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
			configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
			service := NewService(NewRepository(db), nil, nil, nil, nil)
			ruleID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar"}, yesno.Yes))
			if err != nil {
				t.Fatal(err)
			}
			query, parentID := "UPDATE permission_auth_platform SET is_enabled=0 WHERE id=?", platformID
			if parent == "config" {
				query, parentID = "UPDATE storage_cos_config SET is_enabled=0 WHERE id=?", configID
			}
			if err = db.WithContext(ctx).Exec(query, parentID).Error; err != nil {
				t.Fatal(err)
			}
			if err = service.UpdateStatus(ctx, ruleID, yesno.Yes); appCode(err) != apperror.CodeConflict {
				t.Fatalf("enable no-op with disabled %s returned %v", parent, err)
			}
			if err = service.UpdateStatus(ctx, ruleID, yesno.No); err != nil {
				t.Fatalf("disable with disabled %s: %v", parent, err)
			}
			before, err := service.Get(ctx, ruleID)
			if err != nil {
				t.Fatal(err)
			}
			if err = service.UpdateStatus(ctx, ruleID, yesno.No); err != nil {
				t.Fatal(err)
			}
			after, err := service.Get(ctx, ruleID)
			if err != nil || after.IsEnabled != yesno.No || !after.UpdatedAt.Equal(before.UpdatedAt) {
				t.Fatalf("locked no-op changed row: before=%+v after=%+v err=%v", before, after, err)
			}
		})
	}
}

func TestStatusPatchUsesStatusPermissionWithoutFieldUpdatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := []string{}
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(&ruleHTTPService{}), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			permissions = append(permissions, code)
			if code != PermissionStatus {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
		}
	})
	recorder := ruleJSON(router, http.MethodPatch, "/api/admin/v1/storage/uploadrule/7/status", `{"isEnabled":1}`)
	if recorder.Code != http.StatusOK || !reflect.DeepEqual(permissions, []string{PermissionStatus}) {
		t.Fatalf("PATCH status=%d permissions=%v body=%s", recorder.Code, permissions, recorder.Body.String())
	}
}
