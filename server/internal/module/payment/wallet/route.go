package wallet

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "payment:wallet:view"
	PermissionList   = "payment:wallet:list"
	PermissionDetail = "payment:wallet:detail"
	PersonalDetail   = "payment:myWallet:detail"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/payment/wallet")
	group.GET("", authenticate, requirePermission(PermissionList), handler.AdminList)
	group.GET("/:userId", authenticate, requirePermission(PermissionDetail), handler.AdminDetail)
}

func RegisterPersonalRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/payment/wallet", authenticate, requirePermission(PersonalDetail), handler.Current)
}
