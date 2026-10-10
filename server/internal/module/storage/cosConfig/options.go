package cosconfig

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
	Regions    []Option `json:"regions"`
	Extensions []Option `json:"extensions"`
	MimeTypes  []Option `json:"mimeTypes"`
}

// Options are code-owned presets; this path has no database, cache or SDK I/O.
func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	result := Options{Regions: make([]Option, 0, 12)}
	for _, value := range []string{"ap-guangzhou", "ap-shanghai", "ap-nanjing", "ap-beijing", "ap-chengdu", "ap-chongqing", "ap-hongkong", "ap-singapore", "ap-tokyo", "ap-seoul", "eu-frankfurt", "na-siliconvalley"} {
		label, err := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.MessageKey("options.cosRegion."+value), nil)
		if err != nil {
			return Options{}, apperror.DependencyUnavailable(err)
		}
		result.Regions = append(result.Regions, Option{Value: value, Label: label})
	}
	result.Extensions = make([]Option, 0, 11)
	for _, value := range []string{"jpg", "jpeg", "png", "gif", "webp", "pdf", "doc", "docx", "xls", "xlsx", "zip"} {
		result.Extensions = append(result.Extensions, Option{Value: value, Label: value})
	}
	result.MimeTypes = make([]Option, 0, 6)
	for _, value := range []string{"image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "application/zip"} {
		result.MimeTypes = append(result.MimeTypes, Option{Value: value, Label: value})
	}

	return result, nil
}
