package template

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
	Update(context.Context, int64, UpdateInput) (Safe, error)
	UpdateStatus(context.Context, int64, yesno.Value) error
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

type updateRequest struct {
	Scene            *string            `json:"scene"`
	Name             *string            `json:"name"`
	TemplateID       *string            `json:"tencentTemplateId"`
	ParameterKeys    *[]string          `json:"parameterKeys"`
	ExampleVariables *map[string]string `json:"exampleVariables"`
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Scene == nil || r.Name == nil || r.TemplateID == nil || r.ParameterKeys == nil || r.ExampleVariables == nil {
		return UpdateInput{}, apperror.InvalidRequest(fmt.Errorf("sms template fields are required"))
	}
	return UpdateInput{
		Scene:             *r.Scene,
		Name:              *r.Name,
		TencentTemplateID: *r.TemplateID,
		ParameterKeys:     *r.ParameterKeys,
		ExampleVariables:  *r.ExampleVariables,
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

func (h *Handler) Update(c *gin.Context) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms template id")
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
	id, err := validate.ParsePositiveInt64(c.Param("id"), "sms template id")
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
