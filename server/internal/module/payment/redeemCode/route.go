package redeemcode

import "github.com/gin-gonic/gin"

const (
	PermissionView     = "payment:redeemCode:view"
	PermissionList     = "payment:redeemCode:list"
	PermissionDetail   = "payment:redeemCode:detail"
	PermissionGenerate = "payment:redeemCode:generate"
	PermissionVoid     = "payment:redeemCode:void"
	PermissionExport   = "payment:redeemCode:export"
	PersonalRedeem     = "payment:myWallet:redeem"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, h *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	g := routes.Group("/payment/redeemcode")
	g.GET("/options", authenticate, requirePermission(PermissionView), h.Options)
	g.GET("", authenticate, requirePermission(PermissionList), h.List)
	g.GET("/:id", authenticate, requirePermission(PermissionDetail), h.Get)
	g.POST("/batch", authenticate, requirePermission(PermissionGenerate), h.Generate)
	g.POST("/lookup", authenticate, requirePermission(PermissionDetail), h.Lookup)
	g.POST("/export", authenticate, requirePermission(PermissionExport), h.Export)
	g.PATCH("/void", authenticate, requirePermission(PermissionVoid), h.Void)

	routes.POST("/payment/wallet/redemption", authenticate, requirePermission(PersonalRedeem), h.Redeem)
}
