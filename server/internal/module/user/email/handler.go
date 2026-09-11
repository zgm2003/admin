package email

import (
	"fmt"
	"net/http"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service handlerService
	actor   func(*gin.Context) (Actor, bool)
}

func NewHandler(service handlerService, actor func(*gin.Context) (Actor, bool)) *Handler {
	return &Handler{service: service, actor: actor}
}

func (h *Handler) SendCode(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	var request sendCodeRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.Target == nil || (*request.Target != TargetCurrent && *request.Target != TargetNext) || (*request.Target == TargetCurrent && request.Email != nil) || (*request.Target == TargetNext && request.Email == nil) {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("email target payload is invalid")))
		return
	}
	challengeID := ""
	if request.ChallengeID != nil {
		challengeID = *request.ChallengeID
	}
	result, err := h.service.SendCode(c.Request.Context(), actor, SendCodeInput{Target: *request.Target, Email: request.Email, ChallengeID: challengeID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, sendCodeResponse{ChallengeID: result.ChallengeID, ExpiresAt: result.ExpiresAt, ResendAfterSeconds: result.ResendAfterSeconds})
}

func (h *Handler) BindOrChange(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	var request bindOrChangeRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	if request.NextEmail == nil || request.NextChallengeID == nil || request.NextCode == nil {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("next email proof is required")))
		return
	}
	input := BindOrChangeInput{NextEmail: *request.NextEmail, NextChallengeID: *request.NextChallengeID, NextCode: *request.NextCode}
	if request.CurrentChallengeID != nil {
		input.CurrentChallengeID = *request.CurrentChallengeID
	}
	if request.CurrentCode != nil {
		input.CurrentCode = *request.CurrentCode
	}
	projectmiddleware.SetAccessLogOperation(c, "user.email.update", actor.UserID, actor.UserID)
	result, err := h.service.BindOrChange(c.Request.Context(), actor, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, emailResponse{Email: result.Email})
}
