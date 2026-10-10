package notification

import (
	"context"

	"admin/server/internal/shared/option"
)

type notificationOptionResponse struct {
	Variants   []option.Option[Variant]  `json:"variants"`
	Priorities []option.Option[Priority] `json:"priorities"`
}

func notificationOptions(ctx context.Context) notificationOptionResponse {
	return notificationOptionResponse{
		Variants: []option.Option[Variant]{
			option.New(ctx, VariantInfo, "信息", "Info"),
			option.New(ctx, VariantSuccess, "成功", "Success"),
			option.New(ctx, VariantWarning, "警告", "Warning"),
			option.New(ctx, VariantError, "错误", "Error"),
		},
		Priorities: []option.Option[Priority]{
			option.New(ctx, PriorityNormal, "普通", "Normal"),
			option.New(ctx, PriorityUrgent, "紧急", "Urgent"),
		},
	}
}
