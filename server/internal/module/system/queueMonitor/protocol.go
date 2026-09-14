package queuemonitor

import (
	"context"
	"time"
)

const (
	PermissionView = "system:queueMonitor:view"
	PermissionList = "system:queueMonitor:list"
	GrantTTL       = 60 * time.Second
	CookieName     = "admin_queue_monitor_grant"
	UIPath         = "/api/admin/v1/system/queuemonitor/ui"
	grantKeyPrefix = "system:queue-monitor:grant:v1:"
)

var ErrGrantInvalid = grantInvalidError{}

type grantInvalidError struct{}

func (grantInvalidError) Error() string { return "queue monitor grant is invalid" }

type Subject struct {
	UserID     int64
	PlatformID int64
}

type IssuedGrant struct {
	credential string
	ExpiresAt  time.Time
}

type GrantRecord struct {
	UserID         int64     `json:"userId"`
	PlatformID     int64     `json:"platformId"`
	PermissionCode string    `json:"permissionCode"`
	IssuedAt       time.Time `json:"issuedAt"`
}

type redisGrantClient interface {
	SetString(context.Context, string, string, time.Duration) error
	GetString(context.Context, string) (string, bool, error)
}
