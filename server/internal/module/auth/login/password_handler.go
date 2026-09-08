package auth

import (
	authclient "admin/server/internal/module/auth/client"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// ForgotPassword handles POST /auth/password/forgot.
func (h *Handler) ForgotPassword(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	var request ForgotPasswordRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.ForgotPassword(context.Request.Context(), ForgotPasswordInput{
		Email:  *request.Email,
		Client: client,
	})
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, SendCodeResponse{ChallengeID: result.ChallengeID, ExpiresAt: result.ExpiresAt, ResendAfterSeconds: result.ResendAfterSeconds})
}

// ResetPassword handles POST /auth/password/reset.
func (h *Handler) ResetPassword(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	var request ResetPasswordRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.service.ResetPassword(context.Request.Context(), ResetPasswordInput{
		Email:           *request.Email,
		Code:            *request.Code,
		NewPassword:     *request.NewPassword,
		ConfirmPassword: *request.ConfirmPassword,
		Client:          client,
	}); err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, struct{}{})
}
