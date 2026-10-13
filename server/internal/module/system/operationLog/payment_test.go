package operationlog

import (
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPaymentAuditKeepsValidatedTargetWithoutReadingSecrets(t *testing.T) {
	for _, route := range []struct{ method, path, suffix string }{{"POST", "/api/admin/v1/payment/config/:id/version", "/version"}, {"DELETE", "/api/admin/v1/payment/config/:id", ""}, {"POST", "/api/admin/v1/payment/config/:id/test", "/test"}, {"PUT", "/api/admin/v1/payment/config/:id", ""}, {"PATCH", "/api/admin/v1/payment/config/:id/status", "/status"}} {
		t.Run(route.method+route.suffix, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			e := &failingEnqueuer{}
			r := gin.New()
			r.Use(Middleware(slog.New(slog.NewTextHandler(io.Discard, nil)), e))
			r.Handle(route.method, route.path, func(c *gin.Context) { c.JSON(200, gin.H{"code": 0, "data": gin.H{}, "message": "ok"}) })
			req := httptest.NewRequest(route.method, "/api/admin/v1/payment/config/42"+route.suffix, strings.NewReader(`{"configId":999,"privateKey":"secret-key","appCertPEM":"secret-app","name":"visible"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(httptest.NewRecorder(), req)
			if len(e.payloads) != 1 || !strings.Contains(string(e.payloads[0].RequestData), `"configId":"42"`) || strings.Contains(string(e.payloads[0].RequestData), "secret-") {
				t.Fatalf("target audit missing or leaked: %+v", e.payloads)
			}
		})
	}
}

func TestPaymentAuditNeverCapturesCredentials(t *testing.T) {
	for _, r := range []struct{ method, path string }{{"POST", "/api/admin/v1/payment/config"}, {"POST", "/api/admin/v1/payment/config/:id/version"}, {"PUT", "/api/admin/v1/payment/config/:id"}, {"PATCH", "/api/admin/v1/payment/config/:id/status"}, {"DELETE", "/api/admin/v1/payment/config/:id"}, {"POST", "/api/admin/v1/payment/config/:id/test"}} {
		rule, ok := FindRule(r.method, r.path)
		if !ok || rule.Module != "payment" {
			t.Fatalf("audit missing %s", r.path)
		}
		if (strings.Contains(r.path, "version") || r.path == "/api/admin/v1/payment/config") && rule.CaptureRequest {
			t.Fatal("credential input logged")
		}
	}
	v, err := SanitizeJSON([]byte(`{"privateKey":"secret-key","appCertPEM":"secret-app","alipayCertPEM":"secret-ali","rootCertPEM":"secret-root","privateKeyCiphertext":"secret-cipher","name":"visible"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(v), "secret-") || !strings.Contains(string(v), "visible") {
		t.Fatal("payment credential leakage")
	}
}
