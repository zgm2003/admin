package authplatform

import (
	"net/http"

	"admin/server/internal/module/authclient"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service platformService
}

func NewHandler(service platformService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(context *gin.Context) {
	query, err := parseListQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.List(context.Request.Context(), query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newListResponse(result.List, result.Total, result.Page, result.PageSize))
}

func (h *Handler) Deployment(context *gin.Context) {
	value, err := h.service.Deployment(context.Request.Context())
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, deploymentResponse(value))
}

func (h *Handler) Create(context *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, err)
		return
	}
	id, err := h.service.Create(context.Request.Context(), input)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusCreated, idResponse{ID: id})
}

func (h *Handler) Update(context *gin.Context) {
	id, err := parsePlatformID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	var request updateRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.Update(context.Request.Context(), id, input); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) UpdateStatus(context *gin.Context) {
	id, err := parsePlatformID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	var request statusRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	value, err := request.value()
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.UpdateStatus(context.Request.Context(), id, value); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, statusResponse{ID: id, IsEnabled: int16(value)})
}

func (h *Handler) Delete(context *gin.Context) {
	id, err := parsePlatformID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.Delete(context.Request.Context(), id); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, emptyResponse{})
}

func (h *Handler) Policy(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(nil))
		return
	}
	policy, err := h.service.CurrentPolicy(context.Request.Context(), client.Platform)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newPublicPolicyResponse(policy))
}
