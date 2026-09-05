package mail

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/mail")
	g.GET("/page-init", auth, req(PermissionList), h.PageInit)
	g.GET("/config", auth, req(PermissionList), h.Config)
	g.PUT("/config", auth, req(PermissionConfigUpdate), h.SaveConfig)
	g.DELETE("/config", auth, req(PermissionConfigDelete), h.DeleteConfig)
	g.POST("/test", auth, req(PermissionTest), h.Test)
	g.GET("/templates", auth, req(PermissionList), h.Templates)
	g.PUT("/templates/:id", auth, req(PermissionTemplateUpdate), h.UpdateTemplate)
	g.PATCH("/templates/:id/status", auth, req(PermissionTemplateStatus), h.TemplateStatus)
	g.GET("/logs", auth, req(PermissionList), h.Logs)
	g.GET("/logs/:id", auth, req(PermissionDetail), h.LogDetail)
	g.DELETE("/logs/:id", auth, req(PermissionLogDelete), h.DeleteLog)
	g.DELETE("/logs", auth, req(PermissionLogDelete), h.DeleteLogs)
	g.GET("/recipient-rules", auth, req(PermissionList), h.Rules)
	g.POST("/recipient-rules", auth, req(PermissionRuleCreate), h.CreateRule)
	g.PUT("/recipient-rules/:id", auth, req(PermissionRuleUpdate), h.UpdateRule)
	g.PATCH("/recipient-rules/:id/status", auth, req(PermissionRuleStatus), h.RuleStatus)
	g.DELETE("/recipient-rules/:id", auth, req(PermissionRuleDelete), h.DeleteRule)
	g.GET("/rate-limit-policies", auth, req(PermissionList), h.RateLimitPolicies)
	g.PUT("/rate-limit-policies/:key", auth, req(PermissionRateLimitUpdate), h.UpdateRateLimitPolicy)
}
