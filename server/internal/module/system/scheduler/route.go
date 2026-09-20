package scheduler

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	routes.GET("/system/scheduler/options", authenticate, require(PermissionList), handler.Options)
	routes.GET("/system/scheduler/schedule", authenticate, require(PermissionList), handler.ListSchedule)
	routes.GET("/system/scheduler/schedule/:id", authenticate, require(PermissionDetail), handler.GetSchedule)
	routes.POST("/system/scheduler/schedule", authenticate, require(PermissionCreate), handler.CreateSchedule)
	routes.PUT("/system/scheduler/schedule/:id", authenticate, require(PermissionUpdate), handler.UpdateSchedule)
	routes.PATCH("/system/scheduler/schedule/:id/status", authenticate, require(PermissionStatus), handler.Status)
	routes.DELETE("/system/scheduler/schedule/:id", authenticate, require(PermissionDelete), handler.Delete)
	routes.POST("/system/scheduler/schedule/:id/execute", authenticate, require(PermissionExecute), handler.Execute)
	routes.GET("/system/scheduler/job", authenticate, require(PermissionList), handler.ListJobs)
	routes.GET("/system/scheduler/job/:id", authenticate, require(PermissionDetail), handler.GetJob)
	routes.GET("/system/scheduler/job/:id/run", authenticate, require(PermissionDetail), handler.ListRuns)
	routes.POST("/system/scheduler/job/:id/retry", authenticate, require(PermissionRetry), handler.Retry)
}
