package sms

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/authcontext"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type adminService interface {
	PageInit(context.Context) (PageInitResult, error)
	SendAdminTest(context.Context, AdminTestInput) (AdminTestResult, error)
}

type Handler struct{ service adminService }

func NewHandler(service adminService) *Handler { return &Handler{service: service} }

type testRequest struct {
	ToPhone *string `json:"toPhone"`
	Scene   *string `json:"scene"`
}

func (h *Handler) PageInit(c *gin.Context) {
	result, err := h.service.PageInit(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Test(c *gin.Context) {
	identity, found := authcontext.Get(c)
	if !found || identity.UserID < 1 || identity.PlatformID < 1 {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authenticated sms platform is unavailable")))
		return
	}
	var request testRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.ToPhone == nil || request.Scene == nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("toPhone and scene are required")))
		return
	}
	if err := template.ValidateScene(*request.Scene); err != nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("scene is invalid")))
		return
	}
	result, err := h.service.SendAdminTest(c.Request.Context(), AdminTestInput{
		PlatformID: identity.PlatformID, Scene: *request.Scene, ToPhone: *request.ToPhone,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
