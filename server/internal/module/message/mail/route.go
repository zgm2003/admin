package mail

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/message/mail")
	g.GET("/page-init", auth, req(PermissionList), h.PageInit)
	g.GET("/config", auth, req(PermissionList), h.Config)
	g.PUT("/config", auth, req(PermissionConfigUpdate), h.SaveConfig)
	g.DELETE("/config", auth, req(PermissionConfigDelete), h.DeleteConfig)
	g.POST("/test", auth, req(PermissionTest), h.Test)
	g.GET("/template", auth, req(PermissionList), h.Templates)
	g.PUT("/template/:id", auth, req(PermissionTemplateUpdate), h.UpdateTemplate)
	g.PATCH("/template/:id/status", auth, req(PermissionTemplateStatus), h.TemplateStatus)
	g.GET("/log", auth, req(PermissionList), h.Logs)
	g.GET("/log/:id", auth, req(PermissionDetail), h.LogDetail)
	g.DELETE("/log/:id", auth, req(PermissionLogDelete), h.DeleteLog)
	g.DELETE("/log", auth, req(PermissionLogDelete), h.DeleteLogs)
	g.GET("/recipient-rule", auth, req(PermissionList), h.Rules)
	g.POST("/recipient-rule", auth, req(PermissionRuleCreate), h.CreateRule)
	g.PUT("/recipient-rule/:id", auth, req(PermissionRuleUpdate), h.UpdateRule)
	g.PATCH("/recipient-rule/:id/status", auth, req(PermissionRuleStatus), h.RuleStatus)
	g.DELETE("/recipient-rule/:id", auth, req(PermissionRuleDelete), h.DeleteRule)
	g.GET("/rate-limit-policy", auth, req(PermissionList), h.RateLimitPolicies)
	g.PUT("/rate-limit-policy/:key", auth, req(PermissionRateLimitUpdate), h.UpdateRateLimitPolicy)
}
