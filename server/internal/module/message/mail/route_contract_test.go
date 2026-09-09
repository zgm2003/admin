package mail_test

import (
	"testing"

	mail "admin/server/internal/module/message/mail"
	mailconfig "admin/server/internal/module/message/mail/config"
	maillog "admin/server/internal/module/message/mail/log"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
	"github.com/gin-gonic/gin"
)

func TestResourceModulesRegisterTheCompleteMailSurfaceOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/admin/v1/message/mail")
	pass := func(ctx *gin.Context) { ctx.Next() }
	requirePermission := func(string) gin.HandlerFunc { return pass }

	mail.RegisterRoutes(group, mail.NewHandler(nil), pass, requirePermission)
	mailconfig.RegisterRoutes(group, mailconfig.NewHandler(nil), pass, requirePermission)
	mailtemplate.RegisterRoutes(group, mailtemplate.NewHandler(nil), pass, requirePermission)
	maillog.RegisterRoutes(group, maillog.NewHandler(nil), pass, requirePermission)
	ratelimitpolicy.RegisterRoutes(group, ratelimitpolicy.NewHandler(nil), pass, requirePermission)
	recipientrule.RegisterRoutes(group, recipientrule.NewHandler(nil), pass, requirePermission)

	want := map[string]int{
		"GET /api/admin/v1/message/mail/page-init":                          1,
		"POST /api/admin/v1/message/mail/test":                              1,
		"GET /api/admin/v1/message/mail/config":                             1,
		"PUT /api/admin/v1/message/mail/config":                             1,
		"DELETE /api/admin/v1/message/mail/config":                          1,
		"GET /api/admin/v1/message/mail/template":                           1,
		"PUT /api/admin/v1/message/mail/template/:id":                       1,
		"PATCH /api/admin/v1/message/mail/template/:id/status":              1,
		"GET /api/admin/v1/message/mail/log":                                1,
		"GET /api/admin/v1/message/mail/log/:id":                            1,
		"DELETE /api/admin/v1/message/mail/log/:id":                         1,
		"DELETE /api/admin/v1/message/mail/log":                             1,
		"GET /api/admin/v1/message/mail/rate-limit-policy":                  1,
		"PUT /api/admin/v1/message/mail/rate-limit-policy/:platformId/:key": 1,
		"GET /api/admin/v1/message/mail/recipient-rule":                     1,
		"POST /api/admin/v1/message/mail/recipient-rule":                    1,
		"PUT /api/admin/v1/message/mail/recipient-rule/:id":                 1,
		"PATCH /api/admin/v1/message/mail/recipient-rule/:id/status":        1,
		"DELETE /api/admin/v1/message/mail/recipient-rule/:id":              1,
	}
	got := make(map[string]int, len(want))
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, tracked := want[key]; tracked {
			got[key]++
		}
	}
	for route, count := range want {
		if got[route] != count {
			t.Fatalf("route %s count=%d, want %d", route, got[route], count)
		}
	}
}
