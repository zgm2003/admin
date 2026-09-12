package setting

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context, ListQuery) (ListResult, error)
	Find(context.Context, string) (Record, error)
	FindByKey(context.Context, string) (sharedsetting.Record, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, string, UpdateInput) error
	UpdateStatus(context.Context, string, yesno.Value) error
	Delete(context.Context, string) error
}
type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	query, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, listResultResponse(result))
}
func (h *Handler) Detail(c *gin.Context) {
	row, err := h.service.Find(c.Request.Context(), c.Param("key"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, apperror.NotFound(err))
		} else {
			response.Fail(c, apperror.DependencyUnavailable(err))
		}
		return
	}
	response.OK(c, http.StatusOK, detailResponse{Setting: itemResponse(row)})
}
func (h *Handler) Create(c *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	id, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, idResponse{ID: id})
}
func (h *Handler) Update(c *gin.Context) {
	var request updateRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	if err := h.service.Update(c.Request.Context(), c.Param("key"), input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) Status(c *gin.Context) {
	var request statusRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.IsEnabled == nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("isEnabled is required")))
		return
	}
	if err := h.service.UpdateStatus(c.Request.Context(), c.Param("key"), *request.IsEnabled); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, statusResponse{Key: c.Param("key"), IsEnabled: int16(*request.IsEnabled)})
}
func (h *Handler) Delete(c *gin.Context) {
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("key")); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
