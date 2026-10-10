package uploadrule

import (
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const atomicBody = `{"codes":["atomic"],"name":"Atomic","maxFileSizeBytes":2048,"allowedExtensions":["png"],"allowedMimeTypes":["image/png"],"remark":"","isEnabled":1}`

func atomicUpdateInput(t *testing.T, name, code string, status int) UpdateInput {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"codes": []string{code}, "name": name, "maxFileSizeBytes": 2048, "allowedExtensions": []string{"png"}, "allowedMimeTypes": []string{"image/png"}, "remark": "", "isEnabled": status})
	if err != nil {
		t.Fatal(err)
	}
	var input UpdateInput
	if err = json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	return input
}
func TestAtomicUpdateRequiresStatusPermissionWhenSupplied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, allowStatus := range []bool{false, true} {
		service := &ruleHTTPService{}
		router := gin.New()
		seen := []string{}
		RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(service), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen = append(seen, code)
				if code == PermissionStatus && !allowStatus {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
				c.Next()
			}
		})
		recorder := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", atomicBody)
		want := http.StatusForbidden
		if allowStatus {
			want = http.StatusOK
		}
		if recorder.Code != want {
			t.Fatalf("allowStatus=%v status=%d body=%s", allowStatus, recorder.Code, recorder.Body.String())
		}
		if !reflect.DeepEqual(seen, []string{PermissionUpdate, PermissionStatus}) {
			t.Fatalf("permissions=%v", seen)
		}
		if !allowStatus && service.update.Name != "" {
			t.Fatal("unauthorized save reached service")
		}
	}
}
func TestAtomicUpdateReplacesEnabledRuleAndRollsBackAllChanges(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	pid := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	cid := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	oldID, err := service.Create(ctx, validCreate(pid, cid, []string{"old"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	targetID, err := service.Create(ctx, validCreate(pid, cid, []string{"target"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Update(ctx, targetID, atomicUpdateInput(t, "Atomic", "atomic", 1)); err != nil {
		t.Fatal(err)
	}
	assertEnabledRule(t, db, ctx, pid, targetID)
	updated, err := service.Get(ctx, targetID)
	if err != nil || updated.Name != "Atomic" || !reflect.DeepEqual(updated.Codes, []string{"atomic"}) {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if err = service.UpdateStatus(ctx, oldID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`ALTER TABLE storage_upload_rule ADD CONSTRAINT reject_atomic_save CHECK(name <> 'reject-save')`).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.Update(ctx, targetID, atomicUpdateInput(t, "reject-save", "must-rollback", 1)); err == nil {
		t.Fatal("failed write returned success")
	}
	assertEnabledRule(t, db, ctx, pid, oldID)
	unchanged, err := service.Get(ctx, targetID)
	if err != nil || unchanged.Name != "Atomic" || unchanged.IsEnabled != yesno.No || !reflect.DeepEqual(unchanged.Codes, []string{"atomic"}) {
		t.Fatalf("rollback target=%+v err=%v", unchanged, err)
	}
}
func TestConcurrentAtomicUpdatesPreserveOneEnabledRule(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	pid := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	cid := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	ids := make([]int64, 2)
	inputs := make([]UpdateInput, 2)
	for i, code := range []string{"one", "two"} {
		var err error
		ids[i], err = service.Create(ctx, validCreate(pid, cid, []string{code}, yesno.No))
		if err != nil {
			t.Fatal(err)
		}
		inputs[i] = atomicUpdateInput(t, code, code, 1)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id int64) {
			defer wg.Done()
			<-start
			results[i] = service.Update(context.Background(), id, inputs[i])
		}(i, id)
	}
	close(start)
	wg.Wait()
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var enabled int64
	if err := db.WithContext(ctx).Model(&Model{}).Where("platform_id=? AND is_enabled=1 AND deleted_at IS NULL", pid).Count(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("enabled=%d", enabled)
	}
}

func TestAtomicUpdateKeepsStrictBodyValidationAndFieldOnlyPermission(t *testing.T) {
	for _, test := range []struct {
		body string
		want int
	}{
		{strings.Replace(atomicBody, `,"isEnabled":1`, "", 1), http.StatusOK},
		{strings.Replace(atomicBody, `"isEnabled":1`, `"isEnabled":null`, 1), http.StatusBadRequest},
		{strings.Replace(atomicBody, `"isEnabled":1`, `"isEnabled":2`, 1), http.StatusBadRequest},
		{strings.Replace(atomicBody, `"isEnabled":1`, `"isEnabled":"1"`, 1), http.StatusBadRequest},
		{atomicBody[:len(atomicBody)-1] + `,"unknown":1}`, http.StatusBadRequest},
		{atomicBody[:len(atomicBody)-1] + `,"isEnabled":0}`, http.StatusBadRequest},
		{atomicBody + `{}`, http.StatusBadRequest},
		{strings.Repeat(" ", maxUpdateBodyBytes+1), http.StatusBadRequest},
	} {
		service, router := ruleRouter()
		recorder := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", test.body)
		if recorder.Code != test.want {
			t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
		}
		if test.want != http.StatusOK && service.update.Name != "" {
			t.Fatal("malformed save reached service")
		}
	}
	// Case-folded JSON names must not bypass status authorization.
	router := gin.New()
	service := &ruleHTTPService{}
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(service), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if code == PermissionStatus {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
		}
	})
	for _, key := range []string{"isEnabled", "IsEnabled", "ISENABLED"} {
		body := strings.Replace(atomicBody, "isEnabled", key, 1)
		if recorder := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", body); recorder.Code != http.StatusForbidden {
			t.Fatalf("status key=%s bypassed authorization: %d", key, recorder.Code)
		}
	}
	body := strings.Replace(atomicBody, `,"isEnabled":1`, "", 1)
	if recorder := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", body); recorder.Code != http.StatusOK {
		t.Fatalf("field-only update requires status permission: %d", recorder.Code)
	}
}
func TestAtomicUpdatePreservesStatusWhenOmittedAndRejectsUnavailableEnable(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	pid := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	cid := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	id, err := service.Create(ctx, validCreate(pid, cid, []string{"original"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`UPDATE storage_cos_config SET is_enabled=0 WHERE id=?`, cid).Error; err != nil {
		t.Fatal(err)
	}
	input := atomicUpdateInput(t, "Changed", "changed", 1)
	input.IsEnabled = nil
	if err = service.Update(ctx, id, input); err != nil {
		t.Fatal(err)
	}
	saved, err := service.Get(ctx, id)
	if err != nil || saved.IsEnabled != yesno.No || saved.Name != "Changed" {
		t.Fatalf("field-only value=%+v err=%v", saved, err)
	}
	if err = service.Update(ctx, id, atomicUpdateInput(t, "Must rollback", "rollback", 1)); err == nil {
		t.Fatal("unavailable config was enabled")
	}
	unchanged, err := service.Get(ctx, id)
	if err != nil || unchanged.Name != "Changed" || unchanged.IsEnabled != yesno.No || !reflect.DeepEqual(unchanged.Codes, []string{"changed"}) {
		t.Fatalf("unavailable enable persisted=%+v err=%v", unchanged, err)
	}
	invalid := yesno.Value(2)
	input.IsEnabled = &invalid
	if err = service.Update(ctx, id, input); err == nil {
		t.Fatal("invalid service status returned success")
	}
}
