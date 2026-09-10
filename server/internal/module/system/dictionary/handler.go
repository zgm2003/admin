package dictionary

import (
	"context"
	"net/http"
	"strings"

	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context, ListQuery) (ListResult, error)
	Get(context.Context, int64) (Detail, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
	Options(context.Context, []string, string) (OptionResult, error)
	CreateItem(context.Context, int64, CreateItemInput) (int64, error)
	UpdateItem(context.Context, int64, int64, UpdateItemInput) error
	UpdateItemStatus(context.Context, int64, int64, yesno.Value) error
	DeleteItem(context.Context, int64, int64) error
}
type Handler struct{ service service }

func NewHandler(s service) *Handler { return &Handler{service: s} }
func (h *Handler) List(c *gin.Context) {
	q, e := parseListQuery(c.Request.URL.Query())
	if e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.List(c.Request.Context(), q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, dictionaryListResponse(v))
}
func (h *Handler) Get(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.Get(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, dictionaryDetailResponse(v))
}
func (h *Handler) Create(c *gin.Context) {
	var r createRequest
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, e)
		return
	}
	id, e := h.service.Create(c.Request.Context(), in)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, idResponse{ID: id})
}
func (h *Handler) Update(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
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
		response.Fail(c, e)
		return
	}
	if e = h.service.Update(c.Request.Context(), id, in); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
func (h *Handler) Status(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r statusRequest
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	v, e := r.value()
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.UpdateStatus(c.Request.Context(), id, v); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, statusResponse{ID: id, IsEnabled: int16(v)})
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.Delete(c.Request.Context(), id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
func (h *Handler) Options(c *gin.Context) {
	var codes []string
	if raw := c.QueryArray("codes"); len(raw) == 1 {
		for _, v := range strings.Split(raw[0], ",") {
			codes = append(codes, strings.TrimSpace(v))
		}
	} else {
		codes = raw
	}
	v, e := h.service.Options(c.Request.Context(), codes, string(i18n.LocaleFromContext(c.Request.Context())))
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) CreateItem(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r createItemRequest
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.CreateItem(c.Request.Context(), id, in)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, idResponse{ID: v})
}
func (h *Handler) UpdateItem(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	itemID, e := parseID(c.Param("itemId"), "dictionary item id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r updateItemRequest
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	in, e := r.input()
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.UpdateItem(c.Request.Context(), id, itemID, in); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
func (h *Handler) StatusItem(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	itemID, e := parseID(c.Param("itemId"), "dictionary item id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r statusRequest
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	v, e := r.value()
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.UpdateItemStatus(c.Request.Context(), id, itemID, v); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, statusResponse{ID: itemID, IsEnabled: int16(v)})
}
func (h *Handler) DeleteItem(c *gin.Context) {
	id, e := parseID(c.Param("id"), "dictionary id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	itemID, e := parseID(c.Param("itemId"), "dictionary item id")
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.DeleteItem(c.Request.Context(), id, itemID); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}
