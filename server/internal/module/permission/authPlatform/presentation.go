package authplatform

import (
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
	"fmt"
)

type platformActions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}

type platformPresentation struct {
	LoginTypes       []option.Option[LoginType] `json:"loginTypes"`
	MaxSessionsLabel string                     `json:"maxSessionsLabel"`
	DeleteReason     string                     `json:"deleteReason"`
}

func newPlatformPresentation(ctx context.Context, value Platform, loginTypes []LoginType) platformPresentation {
	labels := make([]option.Option[LoginType], 0, len(loginTypes))
	options := loginTypeOptions(ctx)
	for _, loginType := range loginTypes {
		label := string(loginType)
		for _, candidate := range options {
			if candidate.Value == loginType {
				label = candidate.Label
				break
			}
		}
		labels = append(labels, option.Option[LoginType]{Value: loginType, Label: label})
	}
	sessionLabel := fmt.Sprint(value.MaxSessions)
	switch {
	case value.MaxSessions == 0:
		sessionLabel = option.New(ctx, 0, "不限", "Unlimited").Label
	case value.MaxSessions == 1:
		sessionLabel = option.New(ctx, 1, "单会话", "Single session").Label
	case value.MaxSessions > 1:
		sessionLabel = option.New(ctx, 2, fmt.Sprintf("最多 %d 个", value.MaxSessions), fmt.Sprintf("Up to %d", value.MaxSessions)).Label
	}
	reason := ""
	if value.IsBuiltin == yesno.Yes {
		reason = option.New(ctx, 1, "内置认证平台不可删除", "Built-in authentication platforms cannot be deleted").Label
	}
	return platformPresentation{LoginTypes: labels, MaxSessionsLabel: sessionLabel, DeleteReason: reason}
}
