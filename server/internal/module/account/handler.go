package account

import (
	"context"
	"fmt"
	"net/http"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type profileService interface {
	CurrentProfile(context.Context, int64) (user.PersonalProfile, error)
	UpdatePersonalProfile(context.Context, int64, int64, user.PersonalProfileInput) (user.PersonalProfile, error)
}

type passwordService interface {
	ChangePassword(context.Context, auth.Identity, auth.ChangePasswordInput) error
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
	profile, err := h.profile.CurrentProfile(c.Request.Context(), actor)
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
	projectmiddleware.SetAccessLogOperation(c, "account.profile.update", actor, actor)
	updated, err := h.profile.UpdatePersonalProfile(c.Request.Context(), actor, actor, input)
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
	projectmiddleware.SetAccessLogOperation(c, "account.password.update", identity.UserID, identity.UserID)
	if err := h.password.ChangePassword(c.Request.Context(), identity, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, emptyResponse{})
}

type emptyResponse struct{}
