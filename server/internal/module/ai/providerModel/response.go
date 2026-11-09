package providermodel

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update bool `json:"update"`
}

type PriceValue struct {
	ID        int64       `json:"id"`
	TierKey   string      `json:"tierKey"`
	TierLabel string      `json:"tierLabel"`
	PriceCNY  string      `json:"priceCny"`
	Status    yesno.Value `json:"status"`
	Version   int64       `json:"version"`
}

type OfficialValue struct {
	ID            int64             `json:"id"`
	ModelKey      string            `json:"modelKey"`
	DisplayName   string            `json:"displayName"`
	ModelKind     pricing.ModelKind `json:"modelKind"`
	KindName      string            `json:"kindName"`
	InputCNY      string            `json:"inputCnyPerMillion"`
	OutputCNY     string            `json:"outputCnyPerMillion"`
	CacheReadCNY  *string           `json:"cacheReadCnyPerMillion"`
	CacheWriteCNY *string           `json:"cacheWriteCnyPerMillion"`
}

type SafeValue struct {
	ID            int64               `json:"id"`
	ProviderID    int64               `json:"providerId"`
	UpstreamModel string              `json:"upstreamModel"`
	ModelKey      string              `json:"modelKey"`
	DisplayName   string              `json:"displayName"`
	ModelKind     pricing.ModelKind   `json:"modelKind"`
	KindName      string              `json:"kindName"`
	PricingMode   pricing.PricingMode `json:"pricingMode"`
	PricingName   string              `json:"pricingName"`
	PriceUnit     pricing.PriceUnit   `json:"priceUnit"`
	UnitName      string              `json:"unitName"`
	Source        Source              `json:"source"`
	SourceName    string              `json:"sourceName"`
	Status        yesno.Value         `json:"status"`
	StatusName    string              `json:"statusName"`
	OfficialModel *OfficialValue      `json:"officialModel"`
	Prices        []PriceValue        `json:"prices"`
	LastSeenAt    *time.Time          `json:"lastSeenAt"`
	Version       int64               `json:"version"`
	Actions       Actions             `json:"actions"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

func safeValue(ctx context.Context, row Model, prices []Price, official *Official) (SafeValue, error) {
	if row.ID < 1 || row.ProviderID < 1 || row.UpstreamModel == "" || row.ModelKey == "" ||
		!row.ModelKind.Valid() || !row.PricingMode.Valid() || !row.PriceUnit.Valid() ||
		!row.Source.Valid() || !yesno.IsValid(row.Status) || row.Version < 1 {
		return SafeValue{}, dependency(fmt.Errorf("provider model row is invalid"))
	}
	if row.Status == yesno.Yes && row.PricingMode == pricing.ModeOfficialToken && row.OfficialModelID == nil {
		return SafeValue{}, dependency(fmt.Errorf("enabled token provider model has no official model"))
	}
	status := option.New(ctx, row.Status, "停用", "Disabled")
	if row.Status == yesno.Yes {
		status = option.New(ctx, row.Status, "启用", "Enabled")
	}
	priceValues := make([]PriceValue, 0, len(prices))
	for _, price := range prices {
		if price.ID < 1 || price.TierKey == "" || !yesno.IsValid(price.Status) {
			return SafeValue{}, dependency(fmt.Errorf("provider model price row is invalid"))
		}
		priceValues = append(priceValues, PriceValue{
			ID: price.ID, TierKey: price.TierKey, TierLabel: price.TierLabel,
			PriceCNY: formatCNYMicros(price.PriceCNYMicros), Status: price.Status, Version: price.Version,
		})
	}
	var officialValue *OfficialValue
	if official != nil {
		value := OfficialValue{
			ID: official.ID, ModelKey: official.ModelKey, DisplayName: official.DisplayName,
			ModelKind: official.ModelKind, KindName: kindLabel(ctx, official.ModelKind),
			InputCNY: formatCNYMicros(official.InputCNY), OutputCNY: formatCNYMicros(official.OutputCNY),
		}
		if official.CacheRead != nil {
			formatted := formatCNYMicros(*official.CacheRead)
			value.CacheReadCNY = &formatted
		}
		if official.CacheWrite != nil {
			formatted := formatCNYMicros(*official.CacheWrite)
			value.CacheWriteCNY = &formatted
		}
		officialValue = &value
	}
	sourceName := "手工"
	if row.Source == SourceSync {
		sourceName = "上游同步"
	}
	return SafeValue{
		ID: row.ID, ProviderID: row.ProviderID,
		UpstreamModel: row.UpstreamModel, ModelKey: row.ModelKey, DisplayName: row.DisplayName,
		ModelKind: row.ModelKind, KindName: kindLabel(ctx, row.ModelKind),
		PricingMode: row.PricingMode, PricingName: pricingModeLabel(ctx, row.PricingMode),
		PriceUnit: row.PriceUnit, UnitName: priceUnitLabel(ctx, row.PriceUnit),
		Source: row.Source, SourceName: sourceName,
		Status: row.Status, StatusName: status.Label,
		OfficialModel: officialValue, Prices: priceValues,
		LastSeenAt: row.LastSeenAt, Version: row.Version,
		Actions: Actions{Update: true}, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

type OfficialOption struct {
	ID          int64             `json:"id"`
	ModelKey    string            `json:"modelKey"`
	DisplayName string            `json:"displayName"`
	ModelKind   pricing.ModelKind `json:"modelKind"`
	KindName    string            `json:"kindName"`
}

type Options struct {
	ModelKinds     []option.Option[pricing.ModelKind]   `json:"modelKinds"`
	PricingModes   []option.Option[pricing.PricingMode] `json:"pricingModes"`
	PriceUnits     []option.Option[pricing.PriceUnit]   `json:"priceUnits"`
	OfficialModels []OfficialOption                     `json:"officialModels"`
}

func (s *Service) Options(ctx context.Context, providerID int64) (Options, error) {
	provider, err := s.providerSource.FindByID(ctx, providerID)
	if err != nil {
		return Options{}, mapError(err)
	}
	officials, err := s.repo.OfficialByVendor(ctx, int16(provider.Vendor))
	if err != nil {
		return Options{}, mapError(err)
	}
	officialOptions := make([]OfficialOption, 0, len(officials))
	for _, official := range officials {
		officialOptions = append(officialOptions, OfficialOption{
			ID: official.ID, ModelKey: official.ModelKey, DisplayName: official.DisplayName,
			ModelKind: official.ModelKind, KindName: kindLabel(ctx, official.ModelKind),
		})
	}
	return Options{
		ModelKinds: []option.Option[pricing.ModelKind]{
			option.New(ctx, pricing.KindChat, "聊天", "Chat"),
			option.New(ctx, pricing.KindImage, "图片", "Image"),
			option.New(ctx, pricing.KindVideo, "视频", "Video"),
			option.New(ctx, pricing.KindAudio, "音频", "Audio"),
		},
		PricingModes: []option.Option[pricing.PricingMode]{
			option.New(ctx, pricing.ModeOfficialToken, "官方 Token 倍率", "Official token multiplier"),
			option.New(ctx, pricing.ModeProviderUnit, "供应商单位价格", "Provider unit price"),
		},
		PriceUnits: []option.Option[pricing.PriceUnit]{
			option.New(ctx, pricing.UnitMillionTokens, "百万 Token", "Million tokens"),
			option.New(ctx, pricing.UnitImage, "张", "Image"),
			option.New(ctx, pricing.UnitSecond, "秒", "Second"),
			option.New(ctx, pricing.UnitMinute, "分钟", "Minute"),
			option.New(ctx, pricing.UnitMillionCharacters, "百万字符", "Million characters"),
			option.New(ctx, pricing.UnitRequest, "次", "Request"),
		},
		OfficialModels: officialOptions,
	}, nil
}

func kindLabel(ctx context.Context, kind pricing.ModelKind) string {
	switch kind {
	case pricing.KindChat:
		return option.New(ctx, kind, "聊天", "Chat").Label
	case pricing.KindImage:
		return option.New(ctx, kind, "图片", "Image").Label
	case pricing.KindVideo:
		return option.New(ctx, kind, "视频", "Video").Label
	case pricing.KindAudio:
		return option.New(ctx, kind, "音频", "Audio").Label
	default:
		return ""
	}
}

func pricingModeLabel(ctx context.Context, mode pricing.PricingMode) string {
	switch mode {
	case pricing.ModeOfficialToken:
		return option.New(ctx, mode, "官方 Token 倍率", "Official token multiplier").Label
	case pricing.ModeProviderUnit:
		return option.New(ctx, mode, "供应商单位价格", "Provider unit price").Label
	default:
		return ""
	}
}

func priceUnitLabel(ctx context.Context, unit pricing.PriceUnit) string {
	switch unit {
	case pricing.UnitMillionTokens:
		return option.New(ctx, unit, "百万 Token", "Million tokens").Label
	case pricing.UnitImage:
		return option.New(ctx, unit, "张", "Image").Label
	case pricing.UnitSecond:
		return option.New(ctx, unit, "秒", "Second").Label
	case pricing.UnitMinute:
		return option.New(ctx, unit, "分钟", "Minute").Label
	case pricing.UnitMillionCharacters:
		return option.New(ctx, unit, "百万字符", "Million characters").Label
	case pricing.UnitRequest:
		return option.New(ctx, unit, "次", "Request").Label
	default:
		return ""
	}
}
