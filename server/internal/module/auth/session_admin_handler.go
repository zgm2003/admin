package auth

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type sessionAdminService interface {
	ListSessions(context.Context, AdminSessionQuery) ([]AdminSession, int64, error)
	SessionStats(context.Context) (AdminSessionStats, error)
	RevokeSession(context.Context, Identity, int64) (AdminRevokeResult, error)
	RevokeSessions(context.Context, Identity, []int64) (AdminRevokeResult, error)
}

type SessionAdminHandler struct {
	service sessionAdminService
}

func NewSessionAdminHandler(service sessionAdminService) *SessionAdminHandler {
	return &SessionAdminHandler{service: service}
}

func (h *SessionAdminHandler) List(context *gin.Context) {
	identity, ok := IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	query, err := parseSessionAdminQuery(context.Request.URL.Query())
	if err != nil {
		response.Fail(context, err)
		return
	}
	rows, total, err := h.service.ListSessions(context.Request.Context(), query)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, newSessionAdminListResponse(rows, total, query, identity.SessionID))
}

func (h *SessionAdminHandler) Stats(context *gin.Context) {
	stats, err := h.service.SessionStats(context.Request.Context())
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, sessionAdminStatsResponse{ActiveTotal: stats.ActiveTotal, Platforms: stats.Platforms})
}

func (h *SessionAdminHandler) RevokeOne(context *gin.Context) {
	if err := validate.RequireEmptyBody(context); err != nil {
		response.Fail(context, err)
		return
	}
	identity, ok := IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	id, err := parseSessionID(context.Param("id"))
	if err != nil {
		response.Fail(context, err)
		return
	}
	result, err := h.service.RevokeSession(context.Request.Context(), identity, id)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, sessionAdminRevokeResponse{Revoked: len(result.Revoked), SkippedCurrent: result.SkippedCurrent, SkippedRevoked: result.SkippedRevoked})
}

func (h *SessionAdminHandler) RevokeMany(context *gin.Context) {
	var request sessionAdminRequest
	if err := validate.BindJSON(context, &request); err != nil {
		response.Fail(context, err)
		return
	}
	ids, err := request.ids()
	if err != nil {
		response.Fail(context, err)
		return
	}
	identity, ok := IdentityFromContext(context)
	if !ok {
		response.Fail(context, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return
	}
	result, err := h.service.RevokeSessions(context.Request.Context(), identity, ids)
	if err != nil {
		response.Fail(context, err)
		return
	}
	response.OK(context, http.StatusOK, sessionAdminRevokeResponse{Revoked: len(result.Revoked), SkippedCurrent: result.SkippedCurrent, SkippedRevoked: result.SkippedRevoked})
}
