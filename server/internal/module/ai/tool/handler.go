package tool

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func (h *Handler) Options(c *gin.Context) {
	response.OK(c, http.StatusOK, Options(c.Request.Context()))
}

func parseID(c *gin.Context) (int64, error) { return validate.ParsePositiveInt64(c.Param("id"), "id") }
func (h *Handler) List(c *gin.Context) {
	q, e := parseListQuery(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	rows, total, e := h.service.ListFiltered(c.Request.Context(), q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (h *Handler) Detail(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	m, e := h.service.Find(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.service.CurrentVersion(c.Request.Context(), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{"tool": m, "version": v})
}
func (h *Handler) Create(c *gin.Context) {
	var in ToolInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e := validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	in.ActorID = actor(c)
	if id, e := h.service.Create(c.Request.Context(), in); e != nil {
		response.Fail(c, e)
	} else {
		response.OK(c, 201, map[string]string{"id": strconv.FormatInt(id, 10)})
	}
}
func (h *Handler) Update(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in UpdateInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	in.ActorID = actor(c)
	if e = h.service.Update(c.Request.Context(), id, in); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) Status(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var in struct {
		Status  *Status `json:"status"`
		ActorID int64   `json:"-"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	if e = validate.BindJSON(c, &in); e != nil {
		response.Fail(c, e)
		return
	}
	if in.Status == nil {
		response.Fail(c, apperror.InvalidRequest(nil))
		return
	}
	if e = h.service.UpdateStatus(c.Request.Context(), id, *in.Status, actor(c)); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := parseID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.service.Delete(c.Request.Context(), id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func RegisterAdminRoutes(routes *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	routes.GET("/ai/tool/options", auth, h.Options)
	routes.GET("/ai/tool", auth, require(PermissionList), h.List)
	routes.GET("/ai/tool/:id", auth, require(PermissionDetail), h.Detail)
	routes.POST("/ai/tool", auth, require(PermissionCreate), h.Create)
	routes.PUT("/ai/tool/:id", auth, require(PermissionUpdate), h.Update)
	routes.PATCH("/ai/tool/:id/status", auth, require(PermissionStatus), h.Status)
	routes.DELETE("/ai/tool/:id", auth, require(PermissionDelete), h.Delete)
}

const (
	PermissionList   = "ai:tool:list"
	PermissionDetail = "ai:tool:detail"
	PermissionCreate = "ai:tool:create"
	PermissionUpdate = "ai:tool:update"
	PermissionStatus = "ai:tool:status"
	PermissionDelete = "ai:tool:delete"
)

func actor(c *gin.Context) int64 {
	if id, ok := authcontext.Get(c); ok {
		return id.UserID
	}
	return 0
}
