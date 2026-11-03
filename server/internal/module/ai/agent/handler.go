package agent

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }

type idResponse struct {
	ID string `json:"id"`
}

func parseID(c *gin.Context) (int64, error) { return validate.ParsePositiveInt64(c.Param("id"), "id") }
func actor(c *gin.Context) int64 {
	if id, ok := authcontext.Get(c); ok {
		return id.UserID
	}
	return 0
}
func (h *Handler) Options(c *gin.Context) {
	response.OK(c, http.StatusOK, Options(c.Request.Context()))
}

func (h *Handler) List(c *gin.Context) {
	q, e := parseListQuery(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	rows, total, e := h.service.ListFiltered(c.Request.Context(), q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (h *Handler) Detail(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	m, e := h.service.Find(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	b, e := h.service.Bindings(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	t, e := h.service.Tools(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{"agent": m, "bindings": b, "tools": t})
}
func (h *Handler) Create(c *gin.Context) {
	var in CreateInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e := validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	in.ActorID = actor(c)
	id, e := h.service.Create(c.Request.Context(), in)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, idResponse{strconv.FormatInt(id, 10)})
}
func (h *Handler) Update(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in UpdateInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	in.ActorID = actor(c)
	if e = h.service.Update(c.Request.Context(), id, in); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) Status(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in struct {
		Status *int16 `json:"status"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	if in.Status == nil {
		response.Fail(c, apperror.InvalidRequest(nil))
		return
	}
	if e = h.service.UpdateStatus(c.Request.Context(), id, *in.Status, actor(c)); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) SwitchBinding(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in CurrentBindingInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.SwitchBinding(c.Request.Context(), id, in.BindingID); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.Delete(c.Request.Context(), id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}

var _ context.Context
