package auth

import (
	"fmt"
	"net/http"
	"time"

	"admin/server/internal/module/authclient"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

const refreshCookiePath = "/api/v1/auth"

type Handler struct {
	service      authenticationService
	cookieSecure bool
	now          func() time.Time
}

func NewHandler(service authenticationService, cookieSecure bool) *Handler {
	return &Handler{service: service, cookieSecure: cookieSecure, now: time.Now}
}

func (h *Handler) Register(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	var request RegisterRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	registered, err := h.service.Register(context.Request.Context(), RegisterInput{
		Username:        request.Username,
		Email:           request.Email,
		Password:        request.Password,
		ConfirmPassword: request.ConfirmPassword,
		Client:          client,
	})
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusCreated, RegisteredResponse{
		UserID:   registered.UserID,
		Username: registered.Username,
		Email:    registered.Email,
	})
}

func (h *Handler) Login(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	var request LoginRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	credential, err := h.service.Login(context.Request.Context(), LoginInput{
		Username: request.Username,
		Password: request.Password,
		Client:   client,
	})
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.setRefreshCookie(context, client.Platform, credential); err != nil {
		response.Fail(context, err)
		return
	}
	writeCredential(context, credential)
}

func (h *Handler) Refresh(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	refreshToken, err := context.Cookie(refreshCookieName(client.Platform))
	if err != nil {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("Refresh Cookie is required: %w", err)))
		return
	}
	credential, err := h.service.Refresh(context.Request.Context(), RefreshInput{
		RefreshToken: refreshToken,
		Client:       client,
	})
	if err != nil {
		response.Fail(context, err)
		return
	}
	if err := h.setRefreshCookie(context, client.Platform, credential); err != nil {
		response.Fail(context, err)
		return
	}
	writeCredential(context, credential)
}

func (h *Handler) Logout(context *gin.Context) {
	client, ok := authclient.FromContext(context)
	if !ok {
		response.Fail(context, apperror.InvalidRequest(fmt.Errorf("authentication client metadata is missing")))
		return
	}
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	identity, ok := IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	err := h.service.Logout(context.Request.Context(), identity, client)
	h.expireRefreshCookie(context, client.Platform)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, struct{}{})
}

func (h *Handler) Me(context *gin.Context) {
	identity, ok := IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	current, err := h.service.CurrentUser(context.Request.Context(), identity)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, CurrentUserResponse{
		UserID:   current.ID,
		Username: current.Username,
		Email:    current.Email,
	})
}

func (h *Handler) setRefreshCookie(context *gin.Context, platform string, credential Credential) error {
	maxAge := int(credential.RefreshExpiresAt.Sub(h.now()).Seconds())
	if maxAge <= 0 {
		return apperror.Internal(fmt.Errorf("Refresh Cookie expiry must be in the future"))
	}
	http.SetCookie(context.Writer, &http.Cookie{
		Name:     refreshCookieName(platform),
		Value:    credential.RefreshToken,
		Path:     refreshCookiePath,
		Expires:  credential.RefreshExpiresAt.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *Handler) expireRefreshCookie(context *gin.Context, platform string) {
	http.SetCookie(context.Writer, &http.Cookie{
		Name:     refreshCookieName(platform),
		Path:     refreshCookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func refreshCookieName(platform string) string {
	return "admin_refresh_" + platform
}

func writeCredential(context *gin.Context, credential Credential) {
	response.OK(context, http.StatusOK, CredentialResponse{AccessToken: credential.AccessToken, ExpiresIn: credential.ExpiresIn})
}
