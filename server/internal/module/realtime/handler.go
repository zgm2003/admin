package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service     *Service
	connections *ConnectionSet
	origin      string
}

func NewHandler(service *Service, connections *ConnectionSet, origin string) *Handler {
	return &Handler{service: service, connections: connections, origin: origin}
}

func (h *Handler) Ticket(c *gin.Context) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(errors.New("authentication identity is missing")))
		return
	}
	raw, expiresAt, err := h.service.IssueTicket(c.Request.Context(), identity)
	if err != nil {
		response.Fail(c, apperror.DependencyUnavailable(err))
		return
	}
	response.OK(c, http.StatusOK, struct {
		Ticket    string    `json:"ticket"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{raw, expiresAt})
}

func (h *Handler) WebSocket(c *gin.Context) {
	if c.GetHeader("Origin") != h.origin {
		c.Status(http.StatusForbidden)
		return
	}
	subject, err := h.service.ConsumeTicket(c.Request.Context(), c.Query("ticket"))
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(16 * 1024)
	connection := NewConnection(subject.PlatformID, subject.UserID, subject.SessionID, 128, func(code int, reason string) { _ = conn.Close(websocket.StatusCode(code), reason) })
	connection.PlatformCode = subject.PlatformCode
	if err := h.connections.Attach(connection); err != nil {
		_ = conn.Close(websocket.StatusTryAgainLater, "connection limit")
		return
	}
	defer h.connections.Detach(connection)
	defer connection.Close(1000, "closed")
	ctx, cancel := context.WithDeadline(c.Request.Context(), subject.AccessExpiresAt)
	defer cancel()
	connectedData, _ := json.Marshal(struct {
		SessionID int64 `json:"sessionId"`
	}{subject.SessionID})
	connected, _ := EncodeEnvelope(Envelope{EventID: uuid.NewString(), Type: EventRealtimeConnected, Sequence: 1, OccurredAt: h.service.now().UTC(), Durability: DurabilityDurable, Data: connectedData})
	if !connection.enqueue(connected) {
		return
	}
	writerDone := make(chan error, 1)
	go h.writeLoop(ctx, conn, connection, writerDone)
	readErr := h.readLoop(ctx, conn, connection, subject)
	if readErr != nil {
		connection.Close(1002, "protocol error")
	}
	cancel()
	<-writerDone
}

func (h *Handler) writeLoop(ctx context.Context, conn *websocket.Conn, connection *Connection, done chan<- error) {
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			done <- ctx.Err()
			return
		case payload := <-connection.Send():
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				done <- err
				return
			}
		case <-ping.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				done <- err
				return
			}
		}
	}
}
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, connection *Connection, subject TicketSubject) error {
	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != websocket.MessageText {
			return errors.New("only text frames are accepted")
		}
		frame, err := DecodeClientFrame(payload)
		if err != nil {
			return err
		}
		switch frame.Type {
		case ClientPing:
			data, _ := json.Marshal(struct{}{})
			request := frame.RequestID
			pong, _ := EncodeEnvelope(Envelope{EventID: uuid.NewString(), Type: EventRealtimePong, RequestID: &request, Sequence: 1, OccurredAt: h.service.now().UTC(), Durability: DurabilityDurable, Data: data})
			if !connection.enqueue(pong) {
				return ErrConnectionLimit
			}
		case ClientResume:
			messages, err := h.service.Resume(ctx, subject, *frame.AfterSequence, frame.RequestID)
			if err != nil {
				return err
			}
			for _, message := range messages {
				if !connection.enqueue(message) {
					return ErrConnectionLimit
				}
			}
		}
	}
}
