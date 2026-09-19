package notificationtask

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type taskService interface {
	Create(context.Context, int64, DraftInput) (Task, error)
	Update(context.Context, int64, DraftInput) (Task, error)
	Submit(context.Context, int64, time.Time) (Task, error)
	Cancel(context.Context, int64, time.Time) (Task, error)
	Copy(context.Context, int64, int64, time.Time) (Task, error)
	Detail(context.Context, int64) (Task, error)
	List(context.Context, Status, int, int) ([]Task, int64, error)
	Delete(context.Context, int64, time.Time) error
	Options(context.Context, string, string, int64, int) ([]Option, *int64, error)
}
type Handler struct {
	service taskService
	actor   func(*gin.Context) (int64, bool)
}

func NewHandler(service taskService) *Handler {
	return &Handler{service: service, actor: func(c *gin.Context) (int64, bool) {
		identity, ok := auth.IdentityFromContext(c)
		return identity.UserID, ok
	}}
}
func (h *Handler) List(c *gin.Context) {
	values := c.Request.URL.Query()
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "status") || len(entries) != 1 {
			response.Fail(c, apperror.InvalidRequest(fmt.Errorf("invalid query")))
			return
		}
	}
	page, size := 1, 20
	var err error
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	if err == nil {
		if raw := c.Query("pageSize"); raw != "" {
			size, err = strconv.Atoi(raw)
		}
	}
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	status := Status(c.Query("status"))
	if status != "" && !validStatus(status) {
		response.Fail(c, apperror.InvalidRequest(fmt.Errorf("status is invalid")))
		return
	}
	rows, total, err := h.service.List(c.Request.Context(), status, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	list := make([]taskListItemResponse, 0, len(rows))
	for _, row := range rows {
		list = append(list, taskListDTO(row))
	}
	response.OK(c, http.StatusOK, taskListResponse{list, total, page, size})
}
func (h *Handler) Detail(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	row, err := h.service.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, taskDTO(row))
}
func (h *Handler) Create(c *gin.Context) {
	user, ok := h.user(c)
	if !ok {
		return
	}
	input, ok := bindDraft(c)
	if !ok {
		return
	}
	row, err := h.service.Create(c.Request.Context(), user, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, taskDTO(row))
}
func (h *Handler) Update(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	input, ok := bindDraft(c)
	if !ok {
		return
	}
	row, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, taskDTO(row))
}
func (h *Handler) Delete(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, time.Now().UTC()); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) Submit(c *gin.Context) {
	h.command(c, func(ctx context.Context, id, user int64) (Task, error) {
		return h.service.Submit(ctx, id, time.Now().UTC())
	}, false)
}
func (h *Handler) Cancel(c *gin.Context) {
	h.command(c, func(ctx context.Context, id, user int64) (Task, error) {
		return h.service.Cancel(ctx, id, time.Now().UTC())
	}, false)
}
func (h *Handler) Copy(c *gin.Context) {
	h.command(c, func(ctx context.Context, id, user int64) (Task, error) {
		return h.service.Copy(ctx, id, user, time.Now().UTC())
	}, true)
}
func (h *Handler) command(c *gin.Context, fn func(context.Context, int64, int64) (Task, error), needsUser bool) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	user := int64(0)
	if needsUser {
		user, ok = h.user(c)
		if !ok {
			return
		}
	}
	row, err := fn(c.Request.Context(), id, user)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, taskDTO(row))
}
func (h *Handler) PlatformOption(c *gin.Context) { h.options(c, "platform") }
func (h *Handler) UserOption(c *gin.Context)     { h.options(c, "user") }
func (h *Handler) RoleOption(c *gin.Context)     { h.options(c, "role") }
func (h *Handler) options(c *gin.Context, kind string) {
	query, err := parseOptionQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, next, err := h.service.Options(c.Request.Context(), kind, query.Keyword, query.AfterID, query.Limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, optionResponse{items, next})
}
func (h *Handler) user(c *gin.Context) (int64, bool) {
	id, ok := h.actor(c)
	if !ok || id <= 0 {
		response.Fail(c, apperror.Unauthorized(fmt.Errorf("identity missing")))
		return 0, false
	}
	return id, true
}
func taskID(c *gin.Context) (int64, bool) {
	id, err := validate.ParsePositiveInt64(c.Param("id"), "notification task id")
	if err != nil {
		response.Fail(c, err)
		return 0, false
	}
	return id, true
}
func bindDraft(c *gin.Context) (DraftInput, bool) {
	var request draftRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return DraftInput{}, false
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return DraftInput{}, false
	}
	return input, true
}
