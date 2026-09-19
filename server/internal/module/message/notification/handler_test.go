package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerRejectsUnknownRepeatedAndInvalidMailboxQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(mailboxServiceStub{}, func(*gin.Context) (Actor, bool) { return Actor{1, 2}, true })
	for _, raw := range []string{"?unknown=1", "?limit=1&limit=2", "?limit=51", "?beforeId=-1", "?filter=bad", "?variant=bad", "?priority=bad", "?platformId=2"} {
		r := gin.New()
		r.GET("/message/notification", h.List)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/message/notification"+raw, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("query=%s status=%d body=%s", raw, w.Code, w.Body.String())
		}
	}
}

type mailboxServiceStub struct{}

func (mailboxServiceStub) List(context.Context, MailboxQuery) (MailboxPage, error) {
	return MailboxPage{}, nil
}
func (mailboxServiceStub) Summary(context.Context, int64, int64) (MailboxSummary, error) {
	return MailboxSummary{}, nil
}
func (mailboxServiceStub) Read(context.Context, int64, int64, int64) error   { return nil }
func (mailboxServiceStub) ReadAll(context.Context, int64, int64) error       { return nil }
func (mailboxServiceStub) Delete(context.Context, int64, int64, int64) error { return nil }
