package file

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type completionContextKey struct{}

// Embed the handler contract so an unexpected call to another use case fails.
type completionProbe struct {
	handlerService
	calls    int
	ctx      context.Context
	identity auth.Identity
	fileID   int64
}

func (p *completionProbe) Complete(ctx context.Context, id auth.Identity, fileID int64) (CompleteResult, error) {
	p.calls++
	p.ctx, p.identity, p.fileID = ctx, id, fileID
	return CompleteResult{FileID: fileID, ObjectKey: "final/key"}, nil
}

func TestCompleteHandlerRequiresAnActuallyEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{"", "{}", "null", " ", `{"fileId":8}`, "not-json"} {
		t.Run("body="+body, func(t *testing.T) {
			service := &completionProbe{}
			id := auth.Identity{UserID: 3, PlatformID: 1}
			var permission string
			router := gin.New()
			RegisterRoutes(router.Group("/api/v1"), NewHandler(service), func(c *gin.Context) {
				c.Set("auth.identity", id)
				c.Next()
			}, func(code string) gin.HandlerFunc {
				return func(c *gin.Context) { permission = code; c.Next() }
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/storage/file/8/complete", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(context.WithValue(request.Context(), completionContextKey{}, "request-context"))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if permission != "storage:object:upload" {
				t.Fatalf("permission = %q", permission)
			}
			if body == "" {
				if recorder.Code != http.StatusOK || service.calls != 1 || service.fileID != 8 || service.identity != id || service.ctx.Value(completionContextKey{}) != "request-context" {
					t.Fatalf("empty request did not reach Complete: status=%d calls=%d body=%s", recorder.Code, service.calls, recorder.Body.String())
				}
				var result response.Envelope[CompleteResult]
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || result.Code != 0 || result.Data.FileID != 8 || result.Data.ObjectKey != "final/key" {
					t.Fatalf("invalid completion response: %s, %v", recorder.Body.String(), err)
				}
			} else {
				var result response.Envelope[any]
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || result.Code != apperror.CodeInvalidRequest || recorder.Code != http.StatusBadRequest || service.calls != 0 {
					t.Fatalf("nonempty request accepted: status=%d calls=%d body=%s", recorder.Code, service.calls, recorder.Body.String())
				}
			}
		})
	}
}
