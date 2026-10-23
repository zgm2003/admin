package config

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
	Options(context.Context) (Options, error)
	List(context.Context, ListQuery) (pagination.Result[SafeValue], error)
	Get(context.Context, int64) (SafeValue, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, MetadataInput) error
	PublishVersion(context.Context, int64, VersionInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
	ValidateLocal(context.Context, int64) error
}
type Handler struct{ service handlerService }

func NewHandler(s handlerService) *Handler { return &Handler{s} }
func (h *Handler) Options(c *gin.Context) {
	v, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) List(c *gin.Context) {
	q, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	v, err := h.service.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, v)
}
func configID(c *gin.Context) (int64, error) { return validate.ParsePositiveInt64(c.Param("id"), "id") }
func bindConfigJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxSnapshotBytes)
	return validate.BindJSON(c, target)
}
func (h *Handler) Get(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	v, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) Create(c *gin.Context) {
	var r createRequest
	if err := bindConfigJSON(c, &r); err != nil {
		response.Fail(c, err)
		return
	}
	in, err := r.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	id, err := h.service.Create(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 201, struct {
		ID int64 `json:"id"`
	}{id})
}
func (h *Handler) Update(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var r metadataRequest
	if err = bindConfigJSON(c, &r); err != nil {
		response.Fail(c, err)
		return
	}
	in, err := r.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.Update(c.Request.Context(), id, in); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, struct{}{})
}
func (h *Handler) Version(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var r versionRequest
	if err = bindConfigJSON(c, &r); err != nil {
		response.Fail(c, err)
		return
	}
	in, err := r.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.PublishVersion(c.Request.Context(), id, in); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, struct{}{})
}
func (h *Handler) Status(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var r statusRequest
	if err = bindConfigJSON(c, &r); err != nil {
		response.Fail(c, err)
		return
	}
	if r.Status == nil {
		response.Fail(c, invalid(fmt.Errorf("status required")))
		return
	}
	if err = h.service.UpdateStatus(c.Request.Context(), id, *r.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, struct {
		ID        int64       `json:"id"`
		Status yesno.Value `json:"status"`
	}{id, *r.Status})
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, struct{}{})
}
func (h *Handler) Test(c *gin.Context) {
	id, err := configID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.ValidateLocal(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, struct{}{})
}
