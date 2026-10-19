package rechargepackage

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "payment:rechargePackage:view"
	PermissionList   = "payment:rechargePackage:list"
	PermissionDetail = "payment:rechargePackage:detail"
	PermissionCreate = "payment:rechargePackage:create"
	PermissionUpdate = "payment:rechargePackage:update"
	PermissionStatus = "payment:rechargePackage:status"
	PermissionDelete = "payment:rechargePackage:delete"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/payment/rechargepackage")
	group.GET("/options", authenticate, handler.Options)
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.GET("/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
	group.POST("", authenticate, requirePermission(PermissionCreate), handler.Create)
	group.PUT("/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/:id/status", authenticate, requirePermission(PermissionStatus), handler.Status)
	group.DELETE("/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
