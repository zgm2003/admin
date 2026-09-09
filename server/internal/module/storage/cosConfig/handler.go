package cosconfig

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	List(context.Context, ListQuery) (pagination.Result[SafeValue], error)
	Get(context.Context, int64) (SafeValue, error)
	Create(context.Context, CreateInput) (int64, error)
	Update(context.Context, int64, UpdateInput) error
	UpdateStatus(context.Context, int64, yesno.Value) error
	TestConnection(context.Context, int64) error
	Delete(context.Context, int64) error
}

type Handler struct {
	service handlerService
}

func NewHandler(service handlerService) *Handler {
	return &Handler{service: service}
}

func requestID(context *gin.Context) (int64, error) {
	return validate.ParsePositiveInt64(context.Param("id"), "id")
}

func (h *Handler) List(context *gin.Context) {
	query, err := parseListQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, invalid(err))
		return
	}
	result, err := h.service.List(context, query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, pageResponse(result))
}

func (h *Handler) Get(context *gin.Context) {
	id, err := requestID(context)
	if err != nil {
		response.Fail(context, err)
		return
	}
	value, err := h.service.Get(context, id)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, value)
}

func (h *Handler) Create(context *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, invalid(err))
		return
	}
	id, err := h.service.Create(context, input)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusCreated, idResponse{ID: id})
}

func (h *Handler) Update(context *gin.Context) {
	id, err := requestID(context)
	if err != nil {
		response.Fail(context, err)
		return
	}
	var request updateRequest
	if err = validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, invalid(err))
		return
	}
	if err = h.service.Update(context, id, input); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) Status(context *gin.Context) {
	id, err := requestID(context)
	if err != nil {
		response.Fail(context, err)
		return
	}
	var request statusRequest
	if err = validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	if request.IsEnabled == nil {
		response.Fail(context, invalid(fmt.Errorf("isEnabled is required")))
		return
	}
	if err = h.service.UpdateStatus(context, id, *request.IsEnabled); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, statusResponse{ID: id, IsEnabled: int16(*request.IsEnabled)})
}

func (h *Handler) Test(context *gin.Context) {
	id, err := requestID(context)
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err = validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	if err = h.service.TestConnection(context, id); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) Delete(context *gin.Context) {
	id, err := requestID(context)
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err = validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	if err = h.service.Delete(context, id); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}
