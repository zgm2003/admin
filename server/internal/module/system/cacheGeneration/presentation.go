package cachegeneration

import (
	"admin/server/internal/shared/option"
	"context"
	"fmt"
	"strconv"
)

type adminOptions struct {
	PublishStates []option.Option[string] `json:"publishStates"`
}

func formOptions(ctx context.Context) adminOptions {
	return adminOptions{PublishStates: []option.Option[string]{
		option.New(ctx, PublishStateReady, "已发布", "Published"),
		option.New(ctx, PublishStatePending, "待发布", "Pending"),
		option.New(ctx, PublishStateRetrying, "重试中", "Retrying"),
	}}
}

func namespaceLabel(ctx context.Context, value string) string {
	switch value {
	case "system.setting":
		return option.New(ctx, value, "系统设置", "System settings").Label
	case "message.mail":
		return option.New(ctx, value, "邮件服务", "Mail service").Label
	case "message.sms":
		return option.New(ctx, value, "短信服务", "SMS service").Label
	case "storage.cosconfig":
		return option.New(ctx, value, "对象存储配置", "Object storage configuration").Label
	default:
		return value
	}
}

func scopeLabel(ctx context.Context, namespace, scope string) string {
	if scope == "global" {
		return option.New(ctx, scope, "全局", "Global").Label
	}
	if namespace == "storage.cosconfig" {
		if id, err := strconv.ParseInt(scope, 10, 64); err == nil && id > 0 {
			return fmt.Sprintf(option.New(ctx, scope, "对象存储配置 %s", "Object storage configuration %s").Label, scope)
		}
	}
	return fmt.Sprintf(option.New(ctx, scope, "其他范围（%s）", "Other scope (%s)").Label, scope)
}

func statusDisplay(ctx context.Context, value string) (label, tone, hint string) {
	tone = "info"
	switch value {
	case StatusReady:
		label = option.New(ctx, value, "就绪", "Ready").Label
		tone = "success"
	case StatusPending:
		label = option.New(ctx, value, "待发布", "Pending").Label
		tone = "warning"
	case StatusRetrying:
		label = option.New(ctx, value, "重试中", "Retrying").Label
		tone = "danger"
	case StatusInvalidating:
		label = option.New(ctx, value, "变更中", "Invalidating").Label
		tone = "warning"
	case StatusMissing:
		label = option.New(ctx, value, "等待缓存重建", "Waiting for cache rebuild").Label
		tone = "warning"
		hint = option.New(ctx, value, "Redis 状态为空，访问配置时会按现有协议自动重建。", "Redis state is empty; configuration access rebuilds it using the existing protocol.").Label
	case StatusCorrupt:
		label = option.New(ctx, value, "状态损坏", "Corrupt").Label
		tone = "danger"
	case StatusUnavailable:
		label = option.New(ctx, value, "Redis 不可用", "Redis unavailable").Label
		tone = "danger"
	default:
		label = value
	}
	return
}
