package run

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct {
	service *Service
	queries *QueryService
	media   MediaAccess
}

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func runActor(c *gin.Context) (int64, int64, bool) {
	id, ok := authcontext.Get(c)
	if !ok {
		return 0, 0, false
	}
	return id.UserID, id.PlatformID, true
}
func (h *Handler) Create(c *gin.Context) {
	u, p, ok := runActor(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	var in struct {
		AgentID        int64           `json:"agentId,string"`
		RequestKey     string          `json:"requestKey"`
		Operation      string          `json:"operation"`
		ConversationID *int64          `json:"conversationId,string"`
		Input          json.RawMessage `json:"input"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	if e := validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	m, created, e := h.service.PublicCreate(c.Request.Context(), CreateInput{UserID: u, PlatformID: p, AgentID: in.AgentID, RequestKey: in.RequestKey, Operation: in.Operation, ConversationID: in.ConversationID, Input: in.Input})
	if e != nil {
		response.Fail(c, e)
		return
	}
	code := http.StatusOK
	if created {
		code = http.StatusAccepted
	}
	response.OK(c, code, PublicResponse(m))
}
func (h *Handler) Detail(c *gin.Context) {
	u, p, ok := runActor(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	if h.queries == nil {
		response.Fail(c, apperror.DependencyUnavailable(nil))
		return
	}
	m, e := h.queries.Detail(c.Request.Context(), id, QueryOwner{u, p}, false)
	if e != nil {
		response.Fail(c, e)
		return
	}
	if h.queries == nil {
		response.Fail(c, apperror.DependencyUnavailable(nil))
		return
	}
	input, result, e := h.queries.Content(c.Request.Context(), m)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, DetailResponse{Response: PublicResponse(m), Input: input, Result: result})
}
func (h *Handler) Cancel(c *gin.Context) {
	u, p, ok := runActor(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	e = h.service.Cancel(c.Request.Context(), id, u, p)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	r.GET("/ai/run/options", auth, h.Options)
	r.GET("/ai/chat/options", auth, h.Options)
	r.GET("/ai/run", auth, require("ai:run:list"), h.AdminList)
	r.GET("/ai/run/summary", auth, require("ai:run:list"), h.Summary)
	r.GET("/ai/run/:id", auth, require("ai:run:detail"), h.AdminDetail)
	r.GET("/ai/run/:id/request", auth, require("ai:run:detail"), h.AdminRequests)
	r.GET("/ai/run/:id/tool-call", auth, require("ai:run:detail"), h.AdminTools)
	r.GET("/ai/run/:id/usage-charge", auth, require("ai:run:detail"), h.AdminCharges)
	r.GET("/ai/run/:id/usage-charge-item", auth, require("ai:run:detail"), h.AdminChargeItems)
	r.GET("/ai/run/:id/media", auth, require("ai:run:detail"), h.AdminMedia)
	r.GET("/ai/run/:id/media/:mediaId/preview", auth, require("ai:run:detail"), h.AdminPreview)
	r.POST("/ai/chat/run", auth, require("ai:chat:generate"), h.Create)
	r.GET("/ai/chat/run", auth, require("ai:chat:list"), h.OwnList)
	r.GET("/ai/chat/run/:id", auth, require("ai:chat:detail"), h.Detail)
	r.GET("/ai/chat/run/:id/event", auth, require("ai:chat:detail"), h.Events)
	r.POST("/ai/chat/run/:id/cancel", auth, require("ai:chat:cancel"), h.Cancel)
	r.GET("/ai/chat/run/:id/request", auth, require("ai:chat:detail"), h.OwnRequests)
	r.GET("/ai/chat/run/:id/usage-charge", auth, require("ai:chat:detail"), h.OwnCharges)
	r.GET("/ai/chat/run/:id/usage-charge-item", auth, require("ai:chat:detail"), h.OwnChargeItems)
	r.GET("/ai/chat/run/:id/media", auth, require("ai:chat:detail"), h.OwnMedia)
	r.GET("/ai/chat/run/:id/media/:mediaId/preview", auth, require("ai:chat:detail"), h.OwnPreview)
	r.POST("/ai/chat/run/:id/media/retry", auth, require("ai:chat:retryTransfer"), h.RetryMedia)
}
