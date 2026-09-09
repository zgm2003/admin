package config

import (
	"context"
	"time"

	"admin/server/internal/shared/yesno"
)

const (
	PermissionUpdate = "message:mail:config:update"
	PermissionDelete = "message:mail:config:delete"
)

type Input struct {
	SecretID   string      `json:"secretId"`
	SecretKey  string      `json:"secretKey"`
	Region     string      `json:"region"`
	FromEmail  string      `json:"fromEmail"`
	FromName   string      `json:"fromName"`
	ReplyTo    string      `json:"replyTo"`
	Endpoint   string      `json:"endpoint"`
	TTLMinutes int         `json:"ttlMinutes"`
	IsEnabled  yesno.Value `json:"isEnabled"`
}

type Safe struct {
	Configured    bool        `json:"configured"`
	Region        string      `json:"region"`
	Endpoint      string      `json:"endpoint"`
	FromEmail     string      `json:"fromEmail"`
	FromName      string      `json:"fromName"`
	ReplyTo       string      `json:"replyTo"`
	TTLMinutes    int         `json:"ttlMinutes"`
	IsEnabled     yesno.Value `json:"isEnabled"`
	LastTestAt    *time.Time  `json:"lastTestAt"`
	LastTestError string      `json:"lastTestError"`
}

type ReadinessCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}
