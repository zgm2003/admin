package officialmodel

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func actor(c *gin.Context) int64 {
	identity, ok := authcontext.Get(c)
	if !ok {
		return 0
	}
	return identity.UserID
}
func fail(c *gin.Context, err error) {
	var app *apperror.Error
	if errors.As(err, &app) {
		response.Fail(c, err)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, apperror.NotFound(err))
		return
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		response.Fail(c, apperror.Conflict(i18n.KeyConflict, nil, err))
		return
	}
	response.Fail(c, apperror.DependencyUnavailable(err))
}
func page(c *gin.Context) (int, int, error) {
	p, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		return 0, 0, apperror.InvalidRequest(err)
	}
	s, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || p < 1 || p > 1000000 || s < 1 || s > 100 {
		return 0, 0, apperror.InvalidRequest(errors.New("pagination invalid"))
	}
	return p, s, nil
}
func id(c *gin.Context) (int64, error) { return validate.ParsePositiveInt64(c.Param("id"), "id") }
func (h *Handler) List(c *gin.Context) {
	p, s, err := page(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := h.service.List(c.Request.Context(), p, s)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": p, "pageSize": s})
}
func (h *Handler) Detail(c *gin.Context) {
	key, err := id(c)
	if err != nil {
		fail(c, err)
		return
	}
	out, err := h.service.Detail(c.Request.Context(), key)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, out)
}
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.service.Create(c.Request.Context(), req, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 201, out)
}
func (h *Handler) Update(c *gin.Context) {
	key, err := id(c)
	if err != nil {
		fail(c, err)
		return
	}
	var req UpdateRequest
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.service.Update(c.Request.Context(), key, req, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, out)
}
func (h *Handler) Status(c *gin.Context) {
	key, err := id(c)
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if err := h.service.SetStatus(c.Request.Context(), key, req.Status, actor(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) Prices(c *gin.Context) {
	key, err := id(c)
	if err != nil {
		fail(c, err)
		return
	}
	p, s, err := page(c)
	if err != nil {
		fail(c, err)
		return
	}
	out, total, err := h.service.Prices(c.Request.Context(), key, p, s)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"list": out, "total": total, "page": p, "pageSize": s})
}
