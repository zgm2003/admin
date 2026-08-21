package operationlog

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	projectmiddleware "admin/server/internal/middleware"
	"github.com/gin-gonic/gin"
)

const maxSummaryBytes = 32 * 1024

type TaskPayload struct {
	SchemaVersion int       `json:"schemaVersion"`
	EventID       string    `json:"eventId"`
	RequestID     string    `json:"requestId"`
	UserID        *int64    `json:"userId"`
	SessionID     *int64    `json:"sessionId"`
	Platform      *string   `json:"platform"`
	Method        string    `json:"method"`
	Route         string    `json:"route"`
	Module        string    `json:"module"`
	Action        string    `json:"action"`
	ClientIP      string    `json:"clientIp"`
	UserAgent     string    `json:"userAgent"`
	StatusCode    int       `json:"statusCode"`
	IsSuccess     int16     `json:"isSuccess"`
	LatencyMs     int64     `json:"latencyMs"`
	RequestData   JSON      `json:"requestData"`
	ResponseData  JSON      `json:"responseData"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Enqueuer interface {
	Enqueue(context.Context, TaskPayload) error
}

func Middleware(logger *slog.Logger, enqueuer Enqueuer) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		route := ginContext.FullPath()
		rule, matched := FindRule(ginContext.Request.Method, route)
		if !matched {
			ginContext.Next()
			return
		}

		var requestData JSON
		if rule.CaptureRequest {
			requestData = readRequestSummary(ginContext)
		}
		captureWriter := &summaryWriter{ResponseWriter: ginContext.Writer}
		ginContext.Writer = captureWriter
		started := time.Now()
		ginContext.Next()
		ginContext.Writer = captureWriter.ResponseWriter

		authInfo, _ := projectmiddleware.GetAuthenticationLog(ginContext)
		requestID := projectmiddleware.GetRequestID(ginContext)
		eventID, eventIDErr := newEventID()
		if eventIDErr != nil {
			logger.ErrorContext(ginContext.Request.Context(), "generate operation log event ID failed", "requestId", requestID, "route", route, "action", rule.Action, "error", eventIDErr)
			return
		}
		statusCode := captureWriter.Status()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		payload := TaskPayload{
			SchemaVersion: 2,
			EventID:       eventID,
			RequestID:     requestID,
			Method:        ginContext.Request.Method,
			Route:         route,
			Module:        rule.Module,
			Action:        rule.Action,
			ClientIP:      ginContext.ClientIP(),
			UserAgent:     ginContext.GetHeader("User-Agent"),
			StatusCode:    statusCode,
			IsSuccess:     0,
			LatencyMs:     time.Since(started).Milliseconds(),
			RequestData:   requestData,
			CreatedAt:     time.Now().UTC(),
		}
		if authInfo.UserID > 0 {
			value := authInfo.UserID
			payload.UserID = &value
		}
		if authInfo.SessionID > 0 {
			value := authInfo.SessionID
			payload.SessionID = &value
		}
		if authInfo.Platform != "" {
			value := authInfo.Platform
			payload.Platform = &value
		}
		if statusCode < http.StatusBadRequest {
			payload.IsSuccess = 1
		}
		if rule.CaptureResponse {
			payload.ResponseData = captureWriter.summary()
		}
		if enqueuer == nil {
			logger.ErrorContext(ginContext.Request.Context(), "enqueue operation log failed", "requestId", requestID, "route", route, "action", rule.Action, "error", "operation log enqueuer is nil")
			return
		}
		enqueueContext, cancel := context.WithTimeout(ginContext.Request.Context(), 500*time.Millisecond)
		defer cancel()
		if err := enqueuer.Enqueue(enqueueContext, payload); err != nil {
			logger.ErrorContext(ginContext.Request.Context(), "enqueue operation log failed", "requestId", requestID, "route", route, "action", rule.Action, "error", err)
		}
	}
}

func newEventID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate operation log event ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

type summaryWriter struct {
	gin.ResponseWriter
	body      bytes.Buffer
	truncated bool
}

func (w *summaryWriter) Write(data []byte) (int, error) {
	written, err := w.ResponseWriter.Write(data)
	w.capture(data[:written])
	return written, err
}

func (w *summaryWriter) WriteString(value string) (int, error) {
	written, err := w.ResponseWriter.WriteString(value)
	w.capture([]byte(value[:written]))
	return written, err
}

func (w *summaryWriter) capture(data []byte) {
	remaining := maxSummaryBytes - w.body.Len()
	if remaining > len(data) {
		remaining = len(data)
	}
	if remaining > 0 {
		_, _ = w.body.Write(data[:remaining])
	}
	if remaining < len(data) {
		w.truncated = true
	}
}

func (w *summaryWriter) summary() JSON {
	if w.truncated {
		return truncatedSummary()
	}
	summary, err := SanitizeJSON(w.body.Bytes())
	if err != nil {
		return nil
	}
	return summary
}

func readRequestSummary(context *gin.Context) JSON {
	if context.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(context.Request.Body, maxSummaryBytes+1))
	if err != nil {
		return nil
	}
	context.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), context.Request.Body))
	if len(body) > maxSummaryBytes {
		return truncatedSummary()
	}
	sanitized, err := SanitizeJSON(body)
	if err != nil {
		return nil
	}
	return sanitized
}

func SanitizeJSON(raw []byte) (JSON, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var value interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode operation log JSON summary: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("operation log JSON summary contains trailing data")
	}
	sanitized := sanitizeValue("", value)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return nil, fmt.Errorf("encode operation log JSON summary: %w", err)
	}
	return sanitizeWithLimit(encoded), nil
}

func sanitizeValue(field string, value interface{}) interface{} {
	if isSensitiveField(field) {
		return "***"
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = sanitizeValue(key, item)
		}
	case []interface{}:
		for index, item := range typed {
			typed[index] = sanitizeValue("", item)
		}
	}
	return value
}

func isSensitiveField(field string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(field, "-", ""), "_", ""))
	for _, sensitive := range []string{"password", "confirmpassword", "accesstoken", "refreshtoken", "authorization", "cookie", "secret", "key"} {
		if normalized == sensitive || strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}

func sanitizeWithLimit(raw []byte) JSON {
	if len(raw) <= maxSummaryBytes {
		return append(JSON(nil), raw...)
	}
	return truncatedSummary()
}

func truncatedSummary() JSON {
	return JSON(`{"truncated":true}`)
}
