package config

import (
	"context"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type Options struct {
	Regions     []Option          `json:"regions"`
	Constraints ConfigConstraints `json:"constraints"`
}

const (
	minTTLMinutes = 1
	maxTTLMinutes = 60
)

type ConfigConstraints struct {
	MinTTLMinutes int `json:"minTTLMinutes"`
	MaxTTLMinutes int `json:"maxTTLMinutes"`
}

// Options are static presets and never read configuration or cache state.
func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	result := Options{Regions: make([]Option, 0, 2), Constraints: ConfigConstraints{minTTLMinutes, maxTTLMinutes}}
	for _, value := range []string{"ap-guangzhou", "ap-hongkong"} {
		label, err := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.MessageKey("options.mailRegion."+value), nil)
		if err != nil {
			return Options{}, apperror.DependencyUnavailable(err)
		}
		result.Regions = append(result.Regions, Option{Value: value, Label: label})
	}
	return result, nil
}
