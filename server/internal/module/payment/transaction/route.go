package transaction

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "payment:transaction:view"
	PermissionList   = "payment:transaction:list"
	PermissionDetail = "payment:transaction:detail"
	PersonalList     = "payment:myWallet:list"
)

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/payment/transaction")
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.GET("/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
}

func RegisterPersonalRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/payment/wallet/transaction", authenticate, requirePermission(PersonalList), handler.PersonalList)
}
