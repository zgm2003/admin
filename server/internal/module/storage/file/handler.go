package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

const (
	PermissionView   = "storage:file:view"
	PermissionList   = "storage:file:list"
	PermissionDetail = "storage:file:detail"
	PermissionDelete = "storage:file:delete"
)

type handlerService interface {
	List(context.Context, auth.Identity, ListQuery) (pagination.Result[FileItem], error)
	Get(context.Context, auth.Identity, int64) (FileItem, error)
	Options(context.Context, auth.Identity, string) (OptionsResult, error)
	URL(context.Context, auth.Identity, int64, string) (URLResult, error)
	Complete(context.Context, auth.Identity, int64) (CompleteResult, error)
	Delete(context.Context, auth.Identity, int64) error
}
type Handler struct{ s handlerService }

func NewHandler(s handlerService) *Handler { return &Handler{s} }
func identity(c *gin.Context) (auth.Identity, bool) {
	id, ok := auth.IdentityFromContext(c)
	if !ok || !validIdentity(id) {
		response.Fail(c, apperror.Unauthorized(errors.New("identity unavailable")))
		return id, false
	}
	return id, true
}
func (h *Handler) List(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	q, e := parseListQuery(c.Request.URL.Query())
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	result, e := h.s.List(c.Request.Context(), id, q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, result)
}
func (h *Handler) Options(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	for key, values := range c.Request.URL.Query() {
		if key != "purpose" || len(values) != 1 {
			response.Fail(c, apperror.InvalidRequest(errors.New("invalid options query")))
			return
		}
	}
	result, e := h.s.Options(c.Request.Context(), id, c.Query("purpose"))
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, result)
}
func fileID(c *gin.Context) (int64, bool) {
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return 0, false
	}
	return id, true
}
func (h *Handler) Get(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	f, ok := fileID(c)
	if !ok {
		return
	}
	result, e := h.s.Get(c.Request.Context(), id, f)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, result)
}
func (h *Handler) URL(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	f, ok := fileID(c)
	if !ok {
		return
	}
	var in struct {
		Disposition string `json:"disposition"`
	}
	if e := validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	result, e := h.s.URL(c.Request.Context(), id, f, in.Disposition)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, result)
}
func (h *Handler) Complete(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	f, ok := fileID(c)
	if !ok {
		return
	}
	if e := validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	result, e := h.s.Complete(c.Request.Context(), id, f)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, result)
}
func (h *Handler) Delete(c *gin.Context) {
	id, ok := identity(c)
	if !ok {
		return
	}
	f, ok := fileID(c)
	if !ok {
		return
	}
	if e := validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.Delete(c.Request.Context(), id, f); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusAccepted, struct {
		Accepted bool `json:"accepted"`
	}{true})
}
