package provider

import (
	"context"

	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Options struct {
	Vendors   []option.Option[Vendor]      `json:"vendors"`
	Protocols []option.Option[Protocol]    `json:"protocols"`
	Statuses  []option.Option[yesno.Value] `json:"statuses"`
}

func (s *Service) Options(ctx context.Context) (Options, error) {
	vendors := []option.Option[Vendor]{
		option.New(ctx, VendorOpenAI, "OpenAI", "OpenAI"),
		option.New(ctx, VendorAnthropic, "Anthropic", "Anthropic"),
		option.New(ctx, VendorGoogle, "Google", "Google"),
		option.New(ctx, VendorVolcengine, "火山引擎", "Volcengine"),
	}
	protocols := []option.Option[Protocol]{
		option.New(ctx, ProtocolOpenAI, "OpenAI Chat 兼容", "OpenAI Chat compatible"),
		option.New(ctx, ProtocolAnthropic, "Anthropic", "Anthropic"),
		option.New(ctx, ProtocolGemini, "Gemini", "Gemini"),
		option.New(ctx, ProtocolOpenAIResponses, "OpenAI Responses", "OpenAI Responses"),
	}
	statuses := []option.Option[yesno.Value]{
		option.New(ctx, yesno.No, "停用", "Disabled"),
		option.New(ctx, yesno.Yes, "启用", "Enabled"),
	}
	return Options{Vendors: vendors, Protocols: protocols, Statuses: statuses}, nil
}
