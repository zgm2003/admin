package conversation

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestConversationRoutesUseIndependentActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	action := ""
	require := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) { action = code; c.AbortWithStatus(204) }
	}
	RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) {}, require)
	for _, row := range []struct{ method, path, code string }{{"GET", "/ai/chat/agent", "ai:chat:generate"}, {"GET", "/ai/chat/conversation", "ai:chat:list"}, {"POST", "/ai/chat/conversation", "ai:chat:create"}, {"GET", "/ai/chat/conversation/1", "ai:chat:detail"}, {"GET", "/ai/chat/conversation/1/message", "ai:chat:detail"}, {"PUT", "/ai/chat/conversation/1", "ai:chat:update"}, {"DELETE", "/ai/chat/conversation/1", "ai:chat:delete"}} {
		action = ""
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(row.method, row.path, nil))
		if w.Code != 204 || action != row.code {
			t.Errorf("%s %s %d %s", row.method, row.path, w.Code, action)
		}
	}
}
