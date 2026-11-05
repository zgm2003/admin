package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	authclient "admin/server/internal/module/auth/client"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/module/system/scheduler"
	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

func TestSchedulerOptionsLocalizesRegisteredTasksWithoutDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	definitions := append(scheduler.BuiltinDefinitions(nil, nil, nil),
		scheduler.NotificationBatchDefinition(nil),
		scheduler.PaymentReconcileDefinition(nil),
		storagefile.CleanupDefinition(nil),
		storagefile.ScanDefinition(nil),
	)
	wantNames := map[string][2]string{
		"message.notification.retention.cleanup": {"站内通知历史清理", "Notification history cleanup"},
		"message.notificationtask.batch":         {"站内通知批次", "Notification batch"},
		"payment.order.reconcile":                {"支付订单巡检", "Payment order reconciliation"},
		"realtime.retention.cleanup":             {"实时事件历史清理", "Realtime event history cleanup"},
		"storage.file.cleanup":                   {"文件对象清理", "File object cleanup"},
		"storage.file.scan":                      {"文件清理巡检", "File cleanup scan"},
		"system.scheduler.history.cleanup":       {"调度器历史清理", "Scheduler history cleanup"},
	}
	cases := []struct {
		name        string
		definitions []scheduler.TaskDefinition
	}{{name: "all registered tasks", definitions: definitions}}
	for _, definition := range definitions {
		cases = append(cases, struct {
			name        string
			definitions []scheduler.TaskDefinition
		}{name: definition.Type, definitions: []scheduler.TaskDefinition{definition}})
	}

	for localeIndex, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, tc := range cases {
			t.Run(string(locale)+"/"+tc.name, func(t *testing.T) {
				catalog, err := scheduler.NewTaskCatalog(tc.definitions...)
				if err != nil {
					t.Fatal(err)
				}
				router := buildRouter(routerDependencies{
					CORSOrigin: "http://localhost:16300",
					Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
					Scheduler:  scheduler.NewHandler(scheduler.NewService(nil, catalog)),
					Authenticate: func(c *gin.Context) {
						c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), locale))
					},
					RequirePermission: func(string) gin.HandlerFunc {
						return func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }
					},
				})
				request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/system/scheduler/options", nil)
				request.Header.Set(authclient.PlatformHeader, "admin")
				request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					t.Fatalf("options status = %d, body = %s", recorder.Code, recorder.Body.String())
				}
				var envelope struct {
					Code int `json:"code"`
					Data struct {
						Tasks []struct {
							Type        string `json:"type"`
							DisplayName string `json:"displayName"`
						} `json:"tasks"`
					} `json:"data"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Code != 0 || len(envelope.Data.Tasks) != len(tc.definitions) {
					t.Fatalf("options envelope = %+v", envelope)
				}
				seen := make(map[string]bool, len(tc.definitions))
				for _, task := range envelope.Data.Tasks {
					names, ok := wantNames[task.Type]
					if !ok || seen[task.Type] || task.DisplayName != names[localeIndex] {
						t.Errorf("task option = %+v, want unique task with localized name %q", task, names[localeIndex])
					}
					seen[task.Type] = true
				}
			})
		}
	}
}
