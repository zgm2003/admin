package sms

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the aggregate admin endpoints. The sub-resource routes
// (config, template, recipient rule, log, rate limit policy) are registered by
// their own packages under the same /message/sms group.
func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/page-init", authenticate, requirePermission(PermissionList), handler.PageInit)
	routes.POST("/test", authenticate, requirePermission(PermissionTest), handler.Test)
}
