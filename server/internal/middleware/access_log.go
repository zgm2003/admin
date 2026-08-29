package middleware

import (
	"errors"
	"log/slog"
	"time"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

const (
	accessLogOperationKey = "access-log-operation"
	authenticationLogKey  = "access-log-authentication"
)

type accessLogOperation struct {
	operation    string
	actorUserID  int64
	targetUserID int64
}

type authenticationLog struct {
	platformID    int64
	platform      string
	userID        int64
	sessionID     int64
	cacheKind     string
	cacheResult   string
	accessVersion int64
}

type AuthenticationLogInfo struct {
	PlatformID int64
	Platform   string
	UserID     int64
	SessionID  int64
}

type AccessLogOperationInfo struct {
	Operation    string
	ActorUserID  int64
	TargetUserID int64
}

func GetAuthenticationLog(context *gin.Context) (AuthenticationLogInfo, bool) {
	value, ok := authenticationLogFromContext(context)
	if !ok {
		return AuthenticationLogInfo{}, false
	}
	return AuthenticationLogInfo{PlatformID: value.platformID, Platform: value.platform, UserID: value.userID, SessionID: value.sessionID}, true
}

func GetAccessLogOperation(context *gin.Context) (AccessLogOperationInfo, bool) {
	value, exists := context.Get(accessLogOperationKey)
	if !exists {
		return AccessLogOperationInfo{}, false
	}
	operation, ok := value.(accessLogOperation)
	if !ok {
		return AccessLogOperationInfo{}, false
	}
	return AccessLogOperationInfo{Operation: operation.operation, ActorUserID: operation.actorUserID, TargetUserID: operation.targetUserID}, true
}

func SetAccessLogOperation(context *gin.Context, operation string, actorUserID, targetUserID int64) {
	context.Set(accessLogOperationKey, accessLogOperation{operation: operation, actorUserID: actorUserID, targetUserID: targetUserID})
}

func SetAuthenticationLog(context *gin.Context, platformID int64, platform string, userID, sessionID int64) {
	value := authenticationLog{platformID: platformID, platform: platform, userID: userID, sessionID: sessionID}
	if current, ok := authenticationLogFromContext(context); ok {
		value.cacheKind = current.cacheKind
		value.cacheResult = current.cacheResult
		value.accessVersion = current.accessVersion
	}
	context.Set(authenticationLogKey, value)
}

func SetCacheLog(context *gin.Context, kind, result string, accessVersion int64) {
	value := authenticationLog{cacheKind: kind, cacheResult: result, accessVersion: accessVersion}
	if current, ok := authenticationLogFromContext(context); ok {
		value.platformID = current.platformID
		value.platform = current.platform
		value.userID = current.userID
		value.sessionID = current.sessionID
	}
	context.Set(authenticationLogKey, value)
}

func authenticationLogFromContext(context *gin.Context) (authenticationLog, bool) {
	value, exists := context.Get(authenticationLogKey)
	if !exists {
		return authenticationLog{}, false
	}
	logContext, ok := value.(authenticationLog)
	return logContext, ok
}

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(context *gin.Context) {
		started := time.Now()
		context.Next()

		route := context.FullPath()
		if route == "" {
			route = "unmatched"
		}

		attributes := []any{
			"requestId", GetRequestID(context),
			"method", context.Request.Method,
			"route", route,
			"status", context.Writer.Status(),
			"latencyMs", float64(time.Since(started).Microseconds()) / 1000,
		}
		operation, hasOperation := context.Get(accessLogOperationKey)
		operationContext, operationValid := operation.(accessLogOperation)
		if hasOperation && operationValid {
			attributes = append(attributes, "operation", operationContext.operation, "actorUserId", operationContext.actorUserID, "targetUserId", operationContext.targetUserID)
		}
		if authContext, ok := authenticationLogFromContext(context); ok {
			if authContext.platform != "" {
				attributes = append(attributes, "authPlatform", authContext.platform)
			}
			if authContext.userID > 0 {
				attributes = append(attributes, "userID", authContext.userID)
			}
			if authContext.sessionID > 0 {
				attributes = append(attributes, "sessionID", authContext.sessionID)
			}
			if authContext.cacheKind != "" {
				attributes = append(attributes, "cacheKind", authContext.cacheKind)
			}
			if authContext.cacheResult != "" {
				attributes = append(attributes, "cacheResult", authContext.cacheResult)
			}
			if authContext.accessVersion > 0 {
				attributes = append(attributes, "accessVersion", authContext.accessVersion)
			}
		}
		if lastError := context.Errors.Last(); lastError != nil {
			var appErr *apperror.Error
			if errors.As(lastError.Err, &appErr) {
				attributes = append(attributes, "errorCode", appErr.Code)
				if (appErr.HTTPStatus >= 500 || appErr.Code == apperror.CodeDependencyUnavailable || operationValid) && appErr.Cause != nil {
					attributes = append(attributes, "error", appErr.Cause)
				}
			}
		}
		logger.InfoContext(context.Request.Context(), "http request", attributes...)
	}
}
