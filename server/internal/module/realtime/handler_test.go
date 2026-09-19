package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authclient "admin/server/internal/module/auth/client"
	auth "admin/server/internal/module/auth/login"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type realtimeAuthStub struct{ identity auth.Identity }

func (s realtimeAuthStub) Authenticate(context.Context, string, authclient.Client) (auth.Identity, error) {
	return s.identity, nil
}

type realtimeSessionValidatorStub struct {
	err   error
	calls int
}

func (s *realtimeSessionValidatorStub) ValidateRealtimeSession(context.Context, string, int64, int64, int64) error {
	s.calls++
	return s.err
}

func TestRouteIssuesTicketAndWebSocketHandlesPingAndReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisClient, _ := openRealtimeTicketRedis(t)
	now := time.Now().UTC()
	identity := auth.Identity{UserID: 10, SessionID: 20, PlatformID: 1, Platform: "admin", Version: 3, AccessExpiresAt: now.Add(time.Minute)}
	validator := &realtimeSessionValidatorStub{}
	service := NewService(NewTicketStore(redisClient), validator, nil, 1)
	service.now = func() time.Time { return now }
	connections := NewConnectionSet(10, 3)
	router := gin.New()
	server := httptest.NewServer(router)
	defer server.Close()
	handler := NewHandler(service, connections, server.URL)
	shared := router.Group("/api/v1", authclient.Require())
	RegisterRoutes(shared, router, handler, auth.RequireOrigin(server.URL), auth.Authenticate(realtimeAuthStub{identity: identity}))

	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/realtime/ticket", nil)
	request.Header.Set("Origin", server.URL)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set(authclient.PlatformHeader, "admin")
	request.Header.Set(authclient.DeviceIDHeader, "11111111-1111-4111-8111-111111111111")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ticket status=%d", response.StatusCode)
	}
	var envelope struct {
		Data struct {
			Ticket    string    `json:"ticket"`
			ExpiresAt time.Time `json:"expiresAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ExpiresAt.IsZero() {
		t.Fatal("ticket expiresAt is required")
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/realtime/ws?ticket=" + envelope.Data.Ticket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	_, connected, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEnvelope(connected)
	if err != nil || decoded.Type != EventRealtimeConnected {
		t.Fatalf("connected=%+v err=%v", decoded, err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"realtime.ping.v1","requestId":"p-1"}`)); err != nil {
		t.Fatal(err)
	}
	_, pong, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeEnvelope(pong)
	if err != nil || decoded.Type != EventRealtimePong {
		t.Fatalf("pong=%+v err=%v", decoded, err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "done")
	if validator.calls != 1 {
		t.Fatalf("validator calls=%d", validator.calls)
	}
	if _, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{server.URL}}}); err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reuse response=%v err=%v", response, err)
	}
}

func TestWebSocketRejectsWrongOriginBeforeConsumingTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisClient, _ := openRealtimeTicketRedis(t)
	now := time.Now().UTC()
	store := NewTicketStore(redisClient)
	store.now = func() time.Time { return now }
	raw, err := store.Issue(context.Background(), TicketSubject{SchemaVersion: 1, PlatformID: 1, PlatformCode: "admin", UserID: 10, SessionID: 20, SessionVersion: 3, AccessExpiresAt: now.Add(time.Minute), IssuedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	validator := &realtimeSessionValidatorStub{}
	service := NewService(store, validator, nil, 1)
	router := gin.New()
	server := httptest.NewServer(router)
	defer server.Close()
	RegisterRoutes(router.Group("/api/v1"), router, NewHandler(service, NewConnectionSet(10, 3), server.URL), func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() })
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/realtime/ws?ticket=" + raw
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://wrong.example"}}}); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong origin response=%v err=%v", response, err)
	}
	if _, err := store.Consume(context.Background(), raw); err != nil {
		t.Fatalf("ticket consumed before origin check: %v", err)
	}
}
