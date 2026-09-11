package profile

import (
	"context"
	"fmt"
	"net/http"
	"time"

	projectmiddleware "admin/server/internal/middleware"
	authclient "admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	authplatform "admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type profileService interface {
	Current(context.Context, int64) (Value, error)
	Update(context.Context, int64, int64, Input) (Value, error)
}

type passwordService interface {
	ChangePassword(context.Context, auth.Identity, auth.ChangePasswordInput) error
	SetPassword(context.Context, auth.Identity, auth.SetPasswordInput) error
}

type Handler struct {
	profile     profileService
	password    passwordService
	actorUserID func(*gin.Context) (int64, bool)
}

func NewHandler(profile profileService, password passwordService, actorUserID func(*gin.Context) (int64, bool)) *Handler {
	return &Handler{profile: profile, password: password, actorUserID: actorUserID}
}

func (h *Handler) CurrentProfile(c *gin.Context) {
	actor, ok := h.actorUserID(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	profile, err := h.profile.Current(c.Request.Context(), actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, newProfileResponse(profile))
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	actor, ok := h.actorUserID(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
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
	projectmiddleware.SetAccessLogOperation(c, "user.profile.update", actor, actor)
	updated, err := h.profile.Update(c.Request.Context(), actor, actor, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, newUpdatedProfileResponse(updated))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	var request passwordRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(c, "user.password.update", identity.UserID, identity.UserID)
	if err := h.password.ChangePassword(c.Request.Context(), identity, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}

// SetPassword handles POST /account/password/set, the first-time password
// flow for passwordless accounts.
func (h *Handler) SetPassword(c *gin.Context) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	var request setPasswordRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	projectmiddleware.SetAccessLogOperation(c, "user.password.set", identity.UserID, identity.UserID)
	if err := h.password.SetPassword(c.Request.Context(), identity, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}

type passwordCodeService interface {
	SendPasswordCodeForLoginType(context.Context, auth.Identity, authclient.Client, authplatform.LoginType) (auth.SendCodeResult, error)
	ChangePasswordByCode(context.Context, auth.Identity, authclient.Client, auth.ChangePasswordByCodeInput) error
}

type passwordCodeRequest struct {
	LoginType       *string `json:"loginType"`
	ChallengeID     string  `json:"challengeId" binding:"required,max=128"`
	Code            string  `json:"code" binding:"required,len=6,numeric"`
	NewPassword     string  `json:"newPassword" binding:"required"`
	ConfirmPassword string  `json:"confirmPassword" binding:"required"`
}

// SendPasswordCode sends a change_password proof to the authenticated user's
// currently bound email or phone, selected explicitly by loginType.
func (h *Handler) SendPasswordCode(c *gin.Context) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	var request struct {
		LoginType *string `json:"loginType"`
	}
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.LoginType == nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("loginType is required")))
		return
	}
	loginType := authplatform.LoginType(*request.LoginType)
	if loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("loginType must be email or phone")))
		return
	}
	client, ok := authclient.FromContext(c)
	if !ok {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	service, ok := h.password.(passwordCodeService)
	if !ok {
		response.Fail(c, apperror.DependencyUnavailable(fmt.Errorf("password verification service is unavailable")))
		return
	}
	result, err := service.SendPasswordCodeForLoginType(c.Request.Context(), identity, client, loginType)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct {
		ChallengeID        string    `json:"challengeId"`
		ExpiresAt          time.Time `json:"expiresAt"`
		ResendAfterSeconds int       `json:"resendAfterSeconds"`
	}{ChallengeID: result.ChallengeID, ExpiresAt: result.ExpiresAt, ResendAfterSeconds: result.ResendAfterSeconds})
}

// ChangePasswordByCode changes the password using the current phone proof and
// preserves the session represented by the authenticated identity.
func (h *Handler) ChangePasswordByCode(c *gin.Context) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	client, ok := authclient.FromContext(c)
	if !ok {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	var request passwordCodeRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.LoginType == nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("loginType is required")))
		return
	}
	loginType := authplatform.LoginType(*request.LoginType)
	if loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("loginType must be email or phone")))
		return
	}
	service, ok := h.password.(passwordCodeService)
	if !ok {
		response.Fail(c, apperror.DependencyUnavailable(fmt.Errorf("password verification service is unavailable")))
		return
	}
	projectmiddleware.SetAccessLogOperation(c, "user.password.update", identity.UserID, identity.UserID)
	if err := service.ChangePasswordByCode(c.Request.Context(), identity, client, auth.ChangePasswordByCodeInput{
		LoginType: loginType, ChallengeID: request.ChallengeID, Code: request.Code, NewPassword: request.NewPassword, ConfirmPassword: request.ConfirmPassword,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}

type emptyResponse struct{}
