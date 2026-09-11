package recipientRule

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context) ([]Safe, error)
	Create(context.Context, CreateInput) (Safe, error)
	Update(context.Context, int64, UpdateInput) (Safe, error)
	UpdateStatus(context.Context, int64, yesno.Value) error
	Delete(context.Context, int64) error
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

type createRequest struct {
	Scope     *string      `json:"scope"`
	Pattern   *string      `json:"pattern"`
	Action    *string      `json:"action"`
	Name      *string      `json:"name"`
	Remark    *string      `json:"remark"`
	IsEnabled *yesno.Value `json:"isEnabled"`
}

type updateRequest struct {
	Scope     *string      `json:"scope"`
	Pattern   *string      `json:"pattern"`
	Action    *string      `json:"action"`
	Name      *string      `json:"name"`
	Remark    *string      `json:"remark"`
	IsEnabled *yesno.Value `json:"isEnabled"`
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Scope == nil || r.Pattern == nil || r.Action == nil || r.Name == nil || r.Remark == nil || r.IsEnabled == nil {
		return CreateInput{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule fields are required"))
	}
	return CreateInput{
		Scope: *r.Scope, Pattern: *r.Pattern, Action: *r.Action,
		Name: *r.Name, Remark: *r.Remark, IsEnabled: *r.IsEnabled,
	}, nil
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Scope == nil || r.Action == nil || r.Name == nil || r.Remark == nil || r.IsEnabled == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule fields are required"))
	}
	return UpdateInput{
		Scope: *r.Scope, Pattern: r.Pattern, Action: *r.Action,
		Name: *r.Name, Remark: *r.Remark, IsEnabled: *r.IsEnabled,
	}, nil
}

func (h *Handler) List(c *gin.Context) {
	safes, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, ListResponse{List: safes})
}

func (h *Handler) Create(c *gin.Context) {
	var request createRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	safe, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, safe)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms recipient rule id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request updateRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	safe, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, safe)
}

func (h *Handler) Status(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms recipient rule id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request statusRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.IsEnabled == nil || !yesno.IsValid(*request.IsEnabled) {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1")))
		return
	}
	if err := h.service.UpdateStatus(c.Request.Context(), id, *request.IsEnabled); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms recipient rule id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
