package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/option"
	"context"
)

type taskAdminOptions struct {
	Statuses    []option.Option[Status]                `json:"statuses"`
	Audiences   []option.Option[AudienceType]          `json:"audiences"`
	Variants    []option.Option[notification.Variant]  `json:"variants"`
	Priorities  []option.Option[notification.Priority] `json:"priorities"`
	LinkTypes   []option.Option[notification.LinkType] `json:"linkTypes"`
	Defaults    taskFormDefaults                       `json:"defaults"`
	Constraints taskFormConstraints                    `json:"constraints"`
}

type taskFormDefaults struct {
	Variant      notification.Variant  `json:"variant"`
	Priority     notification.Priority `json:"priority"`
	LinkType     notification.LinkType `json:"linkType"`
	AudienceType AudienceType          `json:"audienceType"`
}

type taskFormConstraints struct {
	TitleMaxLength int `json:"titleMaxLength"`
}

func adminOptions(ctx context.Context) taskAdminOptions {
	return taskAdminOptions{
		Defaults:    taskFormDefaults{notification.VariantInfo, notification.PriorityNormal, notification.LinkNone, AudiencePlatform},
		Constraints: taskFormConstraints{TitleMaxLength: notification.TitleMaxLength},
		Statuses:    []option.Option[Status]{option.New(ctx, StatusDraft, "草稿", "Draft"), option.New(ctx, StatusScheduled, "待定时发送", "Scheduled"), option.New(ctx, StatusQueued, "已排队", "Queued"), option.New(ctx, StatusProcessing, "发送中", "Processing"), option.New(ctx, StatusCompleted, "已完成", "Completed"), option.New(ctx, StatusFailed, "失败", "Failed"), option.New(ctx, StatusCanceled, "已取消", "Canceled")},
		Audiences:   []option.Option[AudienceType]{option.New(ctx, AudienceUser, "指定用户", "Users"), option.New(ctx, AudienceRole, "指定角色", "Roles"), option.New(ctx, AudiencePlatform, "整个平台", "Platform")},
		Variants:    []option.Option[notification.Variant]{option.New(ctx, notification.VariantInfo, "信息", "Info"), option.New(ctx, notification.VariantSuccess, "成功", "Success"), option.New(ctx, notification.VariantWarning, "警告", "Warning"), option.New(ctx, notification.VariantError, "错误", "Error")},
		Priorities:  []option.Option[notification.Priority]{option.New(ctx, notification.PriorityNormal, "普通", "Normal"), option.New(ctx, notification.PriorityUrgent, "紧急", "Urgent")},
		LinkTypes:   []option.Option[notification.LinkType]{option.New(ctx, notification.LinkNone, "无链接", "No link"), option.New(ctx, notification.LinkInternal, "站内链接", "Internal"), option.New(ctx, notification.LinkExternal, "外部链接", "External")},
	}
}
