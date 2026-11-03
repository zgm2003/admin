package conversation

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/module/ai/message"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func owner(c *gin.Context) (int64, int64, bool) {
	v, ok := authcontext.Get(c)
	if !ok {
		return 0, 0, false
	}
	return v.UserID, v.PlatformID, true
}
func (h *Handler) List(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	before, limit, ok := conversationPage(c, "beforeId")
	if !ok {
		return
	}
	rows, e := h.service.List(c.Request.Context(), u, p, before, limit)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, conversationRows(rows, limit))
}
func (h *Handler) Create(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	var in struct {
		AgentID int64  `json:"agentId,string"`
		Title   string `json:"title"`
	}
	if e := validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	row, e := h.service.Create(c.Request.Context(), u, p, in.AgentID, in.Title)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, publicConversation(row))
}
func (h *Handler) AgentOptions(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	before, limit, ok := conversationPage(c, "beforeId")
	if !ok {
		return
	}
	rows, e := h.service.Candidates(c.Request.Context(), u, p, before, limit, c.Query("keyword"))
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, rows)
}

func (h *Handler) Messages(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	after, limit, ok := conversationPage(c, "afterSequence")
	if !ok {
		return
	}
	rows, e := h.service.Messages(c.Request.Context(), id, u, p, after, limit)
	if e != nil {
		response.Fail(c, e)
		return
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	next := after
	if len(rows) > 0 {
		next = rows[len(rows)-1].Sequence
	}
	response.OK(c, 200, struct {
		List         []message.Model `json:"list"`
		HasMore      bool            `json:"hasMore"`
		NextSequence int64           `json:"nextSequence"`
	}{rows, more, next})
}
func (h *Handler) Detail(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	row, e := h.service.Find(c.Request.Context(), id, u, p)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, publicConversation(row))
}
func (h *Handler) Update(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.Update(c.Request.Context(), id, u, p, in.Title); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, struct{}{})
}
func (h *Handler) Hide(c *gin.Context) {
	u, p, ok := owner(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.Hide(c.Request.Context(), id, u, p); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, struct{}{})
}
func conversationPage(c *gin.Context, key string) (int64, int, bool) {
	cursor, e := strconv.ParseInt(c.DefaultQuery(key, "0"), 10, 64)
	limit, x := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if e != nil || x != nil || cursor < 0 || limit < 1 || limit > 100 {
		response.Fail(c, apperror.InvalidRequest(nil))
		return 0, 0, false
	}
	return cursor, limit, true
}
func conversationRows(rows []Model, limit int) any {
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	next := ""
	if len(rows) > 0 {
		next = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	return struct {
		List         []ConversationResponse `json:"list"`
		HasMore      bool                   `json:"hasMore"`
		NextBeforeID string                 `json:"nextBeforeId"`
	}{projectConversations(rows), more, next}
}
func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	r.GET("/ai/chat/agent", auth, require("ai:chat:generate"), h.AgentOptions)
	r.GET("/ai/chat/conversation", auth, require("ai:chat:list"), h.List)
	r.POST("/ai/chat/conversation", auth, require("ai:chat:create"), h.Create)
	r.GET("/ai/chat/conversation/:id", auth, require("ai:chat:detail"), h.Detail)
	r.GET("/ai/chat/conversation/:id/message", auth, require("ai:chat:detail"), h.Messages)
	r.PUT("/ai/chat/conversation/:id", auth, require("ai:chat:update"), h.Update)
	r.DELETE("/ai/chat/conversation/:id", auth, require("ai:chat:delete"), h.Hide)
}

func projectConversations(rows []Model) []ConversationResponse {
	out := make([]ConversationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicConversation(row))
	}
	return out
}
