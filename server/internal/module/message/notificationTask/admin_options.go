package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/i18n"
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

func adminOptions(ctx context.Context) (taskAdminOptions, error) {
	statuses, err := option.ManyLocalized(ctx, []option.LocalizedEntry[Status]{
		{Value: StatusDraft, Key: i18n.KeyNotificationTaskStatusDraft}, {Value: StatusScheduled, Key: i18n.KeyNotificationTaskStatusScheduled}, {Value: StatusQueued, Key: i18n.KeyNotificationTaskStatusQueued}, {Value: StatusProcessing, Key: i18n.KeyNotificationTaskStatusProcessing}, {Value: StatusCompleted, Key: i18n.KeyNotificationTaskStatusCompleted}, {Value: StatusFailed, Key: i18n.KeyNotificationTaskStatusFailed}, {Value: StatusCanceled, Key: i18n.KeyNotificationTaskStatusCanceled},
	})
	if err != nil {
		return taskAdminOptions{}, err
	}
	audiences, err := option.ManyLocalized(ctx, []option.LocalizedEntry[AudienceType]{
		{Value: AudienceUser, Key: i18n.KeyNotificationTaskAudienceUser}, {Value: AudienceRole, Key: i18n.KeyNotificationTaskAudienceRole}, {Value: AudiencePlatform, Key: i18n.KeyNotificationTaskAudiencePlatform},
	})
	if err != nil {
		return taskAdminOptions{}, err
	}
	variants, err := option.ManyLocalized(ctx, []option.LocalizedEntry[notification.Variant]{
		{Value: notification.VariantInfo, Key: i18n.KeyNotificationVariantInfo}, {Value: notification.VariantSuccess, Key: i18n.KeyNotificationVariantSuccess}, {Value: notification.VariantWarning, Key: i18n.KeyNotificationVariantWarning}, {Value: notification.VariantError, Key: i18n.KeyNotificationVariantError},
	})
	if err != nil {
		return taskAdminOptions{}, err
	}
	priorities, err := option.ManyLocalized(ctx, []option.LocalizedEntry[notification.Priority]{
		{Value: notification.PriorityNormal, Key: i18n.KeyNotificationPriorityNormal}, {Value: notification.PriorityUrgent, Key: i18n.KeyNotificationPriorityUrgent},
	})
	if err != nil {
		return taskAdminOptions{}, err
	}
	links, err := option.ManyLocalized(ctx, []option.LocalizedEntry[notification.LinkType]{
		{Value: notification.LinkNone, Key: i18n.KeyNotificationTaskLinkNone}, {Value: notification.LinkInternal, Key: i18n.KeyNotificationTaskLinkInternal}, {Value: notification.LinkExternal, Key: i18n.KeyNotificationTaskLinkExternal},
	})
	if err != nil {
		return taskAdminOptions{}, err
	}
	return taskAdminOptions{Defaults: taskFormDefaults{notification.VariantInfo, notification.PriorityNormal, notification.LinkNone, AudiencePlatform}, Constraints: taskFormConstraints{TitleMaxLength: notification.TitleMaxLength}, Statuses: statuses, Audiences: audiences, Variants: variants, Priorities: priorities, LinkTypes: links}, nil
}
