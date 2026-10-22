package redeemcode

import (
	"context"
	"net/http"
	"net/url"

	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	Options(context.Context) (Options, error)
	Generate(context.Context, int64, GenerateInput) (GenerateResult, error)
	List(context.Context, ListQuery) (pagination.Result[Value], error)
	Get(context.Context, int64) (Value, error)
	Lookup(context.Context, string) (Value, error)
	Export(context.Context, []int64) ([]ExportValue, error)
	Void(context.Context, []int64) error
	Redeem(context.Context, int64, int64, string) (Redemption, error)
}
type Handler struct{ service handlerService }

func NewHandler(s handlerService) *Handler { return &Handler{service: s} }

// Reject noncanonical query transports before body binding. Redeem codes are
// accepted only in a JSON body, never in a URL/query or a path parameter.
func bodyRequest(c *gin.Context) bool {
	if c.Request.URL.RawQuery != "" {
		response.Fail(c, apperror.InvalidRequest(nil))
		return false
	}
	return true
}
func noStoreResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}
func (h *Handler) Options(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	v, e := h.service.Options(c.Request.Context())
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Generate(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	id, ok := authcontext.Get(c)
	if !ok || id.UserID < 1 || id.PlatformID < 1 {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	var r generateRequest
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	v, e := h.service.Generate(c.Request.Context(), id.UserID, in)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusCreated, v)
}
func (h *Handler) List(c *gin.Context) {
	vals, e := url.ParseQuery(c.Request.URL.RawQuery)
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	q, e := parseListQuery(vals)
	if e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	v, e := h.service.List(c.Request.Context(), q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Get(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	id, e := validate.ParsePositiveInt64(c.Param("id"), "id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.Get(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Lookup(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	var r struct {
		Code string `json:"code"`
	}
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.Lookup(c.Request.Context(), r.Code)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Void(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	var r struct {
		IDs []int64 `json:"ids"`
	}
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.service.Void(c.Request.Context(), r.IDs); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) Export(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	var r struct {
		IDs []int64 `json:"ids"`
	}
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.Export(c.Request.Context(), r.IDs)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Redeem(c *gin.Context) {
	if !bodyRequest(c) {
		return
	}
	v, ok := authcontext.Get(c)
	if !ok || v.UserID < 1 || v.PlatformID < 1 {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	var r struct {
		Code string `json:"code"`
	}
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	row, e := h.service.Redeem(c.Request.Context(), v.UserID, v.PlatformID, r.Code)
	if e != nil {
		response.Fail(c, e)
		return
	}
	noStoreResponse(c)
	response.OK(c, http.StatusOK, row)
}
