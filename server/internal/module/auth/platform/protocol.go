package authplatform

import (
	"fmt"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/shared/yesno"
)

const (
	PermissionView   = "auth:platform:view"
	PermissionList   = "auth:platform:list"
	PermissionCreate = "auth:platform:create"
	PermissionUpdate = "auth:platform:update"
	PermissionStatus = "auth:platform:status"
	PermissionDelete = "auth:platform:delete"

	BuiltinAdminCode              = "admin"
	BuiltinCanvasCode             = "canvas"
	MinimumAccessTTLSeconds       = 60
	MaximumAccessTTLSeconds       = 2_592_000
	MinimumRefreshTTLSeconds      = 60
	MaximumRefreshTTLSeconds      = 31_536_000
	MinimumSessionCacheTTLSeconds = 60
	MaximumSessionCacheTTLSeconds = 86_400
	MinimumAccessCacheTTLSeconds  = 60
	MaximumAccessCacheTTLSeconds  = 86_400
	MaximumSessions               = 100
)

func ValidateCode(code string) error {
	return authclient.ValidatePlatform(code)
}

func ValidatePlatform(value Platform) error {
	if err := ValidateCode(value.Code); err != nil {
		return err
	}
	if value.Name == "" || len(value.Name) > 64 {
		return fmt.Errorf("platform name must contain 1 to 64 bytes")
	}
	if value.PolicyVersion < 1 {
		return fmt.Errorf("policy version must be at least 1")
	}
	if value.AccessTTLSeconds < MinimumAccessTTLSeconds || value.AccessTTLSeconds > MaximumAccessTTLSeconds {
		return fmt.Errorf("access TTL is outside the allowed range")
	}
	if value.RefreshTTLSeconds < MinimumRefreshTTLSeconds || value.RefreshTTLSeconds > MaximumRefreshTTLSeconds {
		return fmt.Errorf("refresh TTL is outside the allowed range")
	}
	if value.SessionCacheTTLSeconds < MinimumSessionCacheTTLSeconds || value.SessionCacheTTLSeconds > MaximumSessionCacheTTLSeconds {
		return fmt.Errorf("session cache TTL is outside the allowed range")
	}
	if value.AccessCacheTTLSeconds < MinimumAccessCacheTTLSeconds || value.AccessCacheTTLSeconds > MaximumAccessCacheTTLSeconds {
		return fmt.Errorf("access cache TTL is outside the allowed range")
	}
	if !yesno.IsValid(value.BindDevice) || !yesno.IsValid(value.BindIP) || !yesno.IsValid(value.AllowRegister) || !yesno.IsValid(value.IsEnabled) || !yesno.IsValid(value.IsBuiltin) {
		return fmt.Errorf("platform Yes/No value is invalid")
	}
	if value.MaxSessions < 0 || value.MaxSessions > MaximumSessions {
		return fmt.Errorf("max sessions must be between 0 and %d", MaximumSessions)
	}
	return nil
}
