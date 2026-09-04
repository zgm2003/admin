package mail

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

const (
	PermissionView           = "message:mail:view"
	PermissionList           = "message:mail:list"
	PermissionDetail         = "message:mail:detail"
	PermissionConfigUpdate   = "message:mail:config:update"
	PermissionConfigDelete   = "message:mail:config:delete"
	PermissionTest           = "message:mail:test"
	PermissionTemplateUpdate = "message:mail:template:update"
	PermissionTemplateStatus = "message:mail:template:status"
	PermissionLogDelete      = "message:mail:log:delete"
	PermissionRuleCreate     = "message:mail:rule:create"
	PermissionRuleUpdate     = "message:mail:rule:update"
	PermissionRuleStatus     = "message:mail:rule:status"
	PermissionRuleDelete     = "message:mail:rule:delete"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s: s} }
func (h *Handler) PageInit(c *gin.Context) {
	response.OK(c, http.StatusOK, map[string]any{"scenes": FixedTemplates()})
}
func (h *Handler) Config(c *gin.Context) {
	id := adminPlatformID(c)
	v, e := h.s.GetConfig(c, id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) SaveConfig(c *gin.Context) {
	var r ConfigInput
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	v, e := h.s.SaveConfig(c, adminPlatformID(c), r)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) DeleteConfig(c *gin.Context) {
	if e := validate.RequireEmptyBody(c); e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.DeleteConfig(c, adminPlatformID(c)); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) Templates(c *gin.Context) {
	v, e := h.s.ListTemplates(c, adminPlatformID(c))
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) UpdateTemplate(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r TemplateUpdateInput
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	if e = h.s.UpdateTemplate(c, adminPlatformID(c), id, r); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) TemplateStatus(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r struct {
		IsEnabled *yesno.Value `json:"isEnabled"`
	}
	if e = validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	if r.IsEnabled == nil {
		response.Fail(c, invalid(fmt.Errorf("isEnabled is required")))
		return
	}
	if e = h.s.SetTemplateStatus(c, adminPlatformID(c), id, *r.IsEnabled); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{"id": id, "isEnabled": *r.IsEnabled})
}
func (h *Handler) Test(c *gin.Context) {
	var request AdminTestRequest
	if e := validate.BindJSON(c, &request); e != nil {
		response.Fail(c, e)
		return
	}
	r := AdminTestInput{ClientIP: c.ClientIP(), ToEmail: request.ToEmail, Scene: request.Scene, Variables: request.Variables}
	if id, ok := authcontext.Get(c); ok {
		r.AdminUserID = id.UserID
	}
	v, e := h.s.TestForPlatform(c, adminPlatformID(c), r)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) Logs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 || size < 1 || size > 100 {
		response.Fail(c, invalid(fmt.Errorf("invalid pagination")))
		return
	}
	rows, total, e := h.s.ListLogs(c, adminPlatformID(c), page, size)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, logListResponseFromModels(rows, total, page, size))
}
func (h *Handler) LogDetail(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	detail, e := h.s.GetLogDetail(c, adminPlatformID(c), id)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, logDetailResponseFromModel(detail))
}
func (h *Handler) DeleteLog(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.DeleteLog(c, adminPlatformID(c), id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) DeleteLogs(c *gin.Context) {
	var ids []int64
	if e := validate.BindJSON(c, &ids); e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.DeleteLogs(c, adminPlatformID(c), ids); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) Rules(c *gin.Context) {
	v, e := h.s.ListRules(c, adminPlatformID(c))
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, v)
}
func (h *Handler) CreateRule(c *gin.Context) {
	var r RuleInput
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	id, e := h.s.CreateRule(c, adminPlatformID(c), r)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusCreated, map[string]any{"id": id})
}
func (h *Handler) UpdateRule(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r RuleInput
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.UpdateRule(c, adminPlatformID(c), id, r); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func (h *Handler) RuleStatus(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	var r struct {
		IsEnabled *yesno.Value `json:"isEnabled"`
	}
	if e := validate.BindJSON(c, &r); e != nil {
		response.Fail(c, e)
		return
	}
	if r.IsEnabled == nil {
		response.Fail(c, invalid(fmt.Errorf("isEnabled is required")))
		return
	}
	if e := h.s.SetRuleStatus(c, adminPlatformID(c), id, *r.IsEnabled); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{"id": id, "isEnabled": *r.IsEnabled})
}
func (h *Handler) DeleteRule(c *gin.Context) {
	id, e := mailID(c)
	if e != nil {
		response.Fail(c, e)
		return
	}
	if e := h.s.DeleteRule(c, adminPlatformID(c), id); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, http.StatusOK, map[string]any{})
}
func adminPlatformID(c *gin.Context) int64 {
	if id, ok := authcontext.Get(c); ok && id.PlatformID > 0 {
		return id.PlatformID
	}
	return 1
}
func mailID(c *gin.Context) (int64, error) { return validate.ParsePositiveInt64(c.Param("id"), "id") }
