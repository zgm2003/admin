package config

import (
	"context"
	"regexp"
	"time"

	"admin/server/internal/shared/yesno"
)

const (
	PermissionList   = "message:sms:list"
	PermissionUpdate = "message:sms:config:update"
	PermissionDelete = "message:sms:config:delete"
)

var (
	regionPattern   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	endpointPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
)

// Safe is the only configuration shape exposed over HTTP. It never carries
// ciphertext or a plaintext credential.
type Safe struct {
	Configured    bool        `json:"configured"`
	SDKAppID      string      `json:"smsSdkAppId"`
	SignName      string      `json:"signName"`
	Region        string      `json:"region"`
	Endpoint      string      `json:"endpoint"`
	TTLMinutes    int         `json:"ttlMinutes"`
	IsEnabled     yesno.Value `json:"isEnabled"`
	LastTestAt    *string     `json:"lastTestAt"`
	LastTestError string      `json:"lastTestError"`
}

type Input struct {
	SecretID   string
	SecretKey  string
	SDKAppID   string
	SignName   string
	Region     string
	Endpoint   string
	TTLMinutes int
	IsEnabled  yesno.Value
}

type Credentials struct {
	SecretID   string
	SecretKey  string
	SDKAppID   string
	SignName   string
	Region     string
	Endpoint   string
	TTLMinutes int
}

// RuntimeCoordinator invalidates the SMS runtime snapshot after a write.
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}

type repository interface {
	FindActive(context.Context) (Model, error)
	Create(context.Context, *Model) error
	Update(context.Context, *Model, time.Time) error
	UpdateTestResult(context.Context, int64, time.Time, string) error
	Delete(context.Context, int64) error
}
