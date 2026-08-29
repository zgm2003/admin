package uploadrule

import (
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type handlerService interface {
	List(context.Context, ListQuery) (pagination.Result[RuleValue], error)
	PageInit(context.Context) (PageInit, error)
	Get(context.Context, int64) (RuleValue, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
}
type Handler struct{ s handlerService }

func NewHandler(s handlerService) *Handler { return &Handler{s} }

const (
	PermissionList   = "storage:object:list"
	PermissionCreate = "storage:upload-rule:create"
	PermissionUpdate = "storage:upload-rule:update"
	PermissionStatus = "storage:upload-rule:status"
	PermissionDelete = "storage:upload-rule:delete"
)

func (h *Handler) List(c *gin.Context) {
	q, e := parseListQuery(c.Request.URL.Query())
	if e != nil {
		response.Fail(c, invalid(e))
		return
	}
	r, e := h.s.List(c, q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, pageResponse(r))
}
func (h *Handler) PageInit(c *gin.Context) {
	r, e := h.s.PageInit(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, r)
}
func (h *Handler) Get(c *gin.Context) {
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	r, e := h.s.Get(c, id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, r)
}
func (h *Handler) Create(c *gin.Context) {
	var r createRequest
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, invalid(e))
		return
	}
	id, e := h.s.Create(c, in)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, idResponse{id})
}
func (h *Handler) Update(c *gin.Context) {
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r updateRequest
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, invalid(e))
		return
	}
	if e = h.s.Update(c, id, in); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
func (h *Handler) Status(c *gin.Context) {
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r statusRequest
	if e = validate.BindJSON(c, &r); e != nil || r.IsEnabled == nil {
		if e == nil {
			e = invalid(fmt.Errorf("isEnabled required"))
		}
		response.Fail(c, e)
		return
	}
	if e = h.s.UpdateStatus(c, id, *r.IsEnabled); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, statusResponse{id, int16(*r.IsEnabled)})
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.s.Delete(c, id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
