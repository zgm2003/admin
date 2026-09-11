package auth

import (
	authclient "admin/server/internal/module/auth/client"
	"admin/server/internal/module/permission/authPlatform"
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
	if request.Account == nil || request.LoginType == nil {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("account and loginType are required")))
		return
	}
	loginType := authplatform.LoginType(*request.LoginType)
	if loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("loginType must be email or phone")))
		return
	}
	result, err := h.service.ForgotPassword(context.Request.Context(), ForgotPasswordInput{
		Account:   *request.Account,
		LoginType: loginType,
		Client:    client,
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
	if request.Account == nil || request.LoginType == nil || request.ChallengeID == nil || request.Code == nil ||
		request.NewPassword == nil || request.ConfirmPassword == nil {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("account, loginType, challengeId, code and passwords are required")))
		return
	}
	loginType := authplatform.LoginType(*request.LoginType)
	if (loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone) || validateChallengeID(*request.ChallengeID) != nil || !isSixASCIIDigits(*request.Code) {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("password reset proof is invalid")))
		return
	}
	if err := h.service.ResetPassword(context.Request.Context(), ResetPasswordInput{
		Account:         *request.Account,
		LoginType:       loginType,
		ChallengeID:     *request.ChallengeID,
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
