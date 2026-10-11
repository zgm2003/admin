package notification

import (
	"context"

	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
)

type notificationOptionResponse struct {
	Variants   []option.Option[Variant]  `json:"variants"`
	Priorities []option.Option[Priority] `json:"priorities"`
}

func notificationOptions(ctx context.Context) (notificationOptionResponse, error) {
	variants, err := option.ManyLocalized(ctx, []option.LocalizedEntry[Variant]{
		{Value: VariantInfo, Key: i18n.KeyNotificationVariantInfo}, {Value: VariantSuccess, Key: i18n.KeyNotificationVariantSuccess},
		{Value: VariantWarning, Key: i18n.KeyNotificationVariantWarning}, {Value: VariantError, Key: i18n.KeyNotificationVariantError},
	})
	if err != nil {
		return notificationOptionResponse{}, err
	}
	priorities, err := option.ManyLocalized(ctx, []option.LocalizedEntry[Priority]{
		{Value: PriorityNormal, Key: i18n.KeyNotificationPriorityNormal}, {Value: PriorityUrgent, Key: i18n.KeyNotificationPriorityUrgent},
	})
	if err != nil {
		return notificationOptionResponse{}, err
	}
	return notificationOptionResponse{Variants: variants, Priorities: priorities}, nil
}
