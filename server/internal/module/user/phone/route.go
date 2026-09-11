package phone

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	userRoutes := routes.Group("/user")
	userRoutes.POST("/phone/send-code", authenticate, requirePermission(PermissionUpdate), handler.SendCode)
	userRoutes.PUT("/phone", authenticate, requirePermission(PermissionUpdate), handler.BindOrChange)
}
