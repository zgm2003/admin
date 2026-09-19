package notification

import (
	"context"
	"fmt"
	"net/http"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type mailboxService interface {
	List(context.Context, MailboxQuery) (MailboxPage, error)
	Summary(context.Context, int64, int64) (MailboxSummary, error)
	Read(context.Context, int64, int64, int64) error
	ReadAll(context.Context, int64, int64) error
	Delete(context.Context, int64, int64, int64) error
}
type Actor struct {
	PlatformID int64
	UserID     int64
}
type actorExtractor func(*gin.Context) (Actor, bool)
type Handler struct {
	service mailboxService
	actor   actorExtractor
}

func NewHandler(service mailboxService, extractors ...actorExtractor) *Handler {
	extractor := func(c *gin.Context) (Actor, bool) {
		identity, ok := auth.IdentityFromContext(c)
		return Actor{identity.PlatformID, identity.UserID}, ok
	}
	if len(extractors) > 0 {
		extractor = extractors[0]
	}
	return &Handler{service: service, actor: extractor}
}
func (h *Handler) identity(c *gin.Context) (Actor, bool) {
	actor, ok := h.actor(c)
	if !ok || actor.PlatformID <= 0 || actor.UserID <= 0 {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("authentication identity is missing")))
		return Actor{}, false
	}
	return actor, true
}
func (h *Handler) List(c *gin.Context) {
	actor, ok := h.identity(c)
	if !ok {
		return
	}
	query, err := parseMailboxQuery(c.Request.URL.Query(), actor.PlatformID, actor.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, toListResponse(page))
}
func (h *Handler) Summary(c *gin.Context) {
	actor, ok := h.identity(c)
	if !ok {
		return
	}
	value, err := h.service.Summary(c.Request.Context(), actor.PlatformID, actor.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, toSummaryResponse(value))
}
func (h *Handler) Read(c *gin.Context)   { h.mutateID(c, h.service.Read) }
func (h *Handler) Delete(c *gin.Context) { h.mutateID(c, h.service.Delete) }
func (h *Handler) mutateID(c *gin.Context, mutate func(context.Context, int64, int64, int64) error) {
	actor, ok := h.identity(c)
	if !ok {
		return
	}
	id, err := validate.ParsePositiveInt64(c.Param("id"), "notification id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = mutate(c.Request.Context(), actor.PlatformID, actor.UserID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) ReadAll(c *gin.Context) {
	actor, ok := h.identity(c)
	if !ok {
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.ReadAll(c.Request.Context(), actor.PlatformID, actor.UserID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
