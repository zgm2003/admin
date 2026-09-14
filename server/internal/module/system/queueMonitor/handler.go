package queuemonitor

import (
	"context"
	"errors"
	"net/http"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type grantIssuer interface {
	Issue(context.Context, Subject) (IssuedGrant, error)
}

type identityReader func(*gin.Context) (Subject, bool)

type Handler struct {
	service     grantIssuer
	secure      bool
	readSubject identityReader
}

func NewHandler(service grantIssuer, secure bool, readSubject identityReader) *Handler {
	return &Handler{service: service, secure: secure, readSubject: readSubject}
}

func (h *Handler) Grant(c *gin.Context) {
	if h == nil || h.service == nil || h.readSubject == nil {
		response.Fail(c, apperror.DependencyUnavailable(errors.New("queue monitor grant handler is unavailable")))
		return
	}
	subject, ok := h.readSubject(c)
	if !ok {
		response.Fail(c, apperror.Unauthorized(errors.New("authentication identity is missing")))
		return
	}
	issued, err := h.service.Issue(c.Request.Context(), subject)
	if err != nil {
		response.Fail(c, apperror.DependencyUnavailable(err))
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieName, Value: issued.credential, Path: UIPath, HttpOnly: true,
		Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(GrantTTL.Seconds()),
	})
	c.Header("Cache-Control", "no-store")
	response.OK(c, http.StatusOK, struct {
		ExpiresAt string `json:"expiresAt"`
	}{ExpiresAt: issued.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
}

func SubjectFromContext(c *gin.Context) (Subject, bool) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		return Subject{}, false
	}
	return Subject{UserID: identity.UserID, PlatformID: identity.PlatformID}, true
}
