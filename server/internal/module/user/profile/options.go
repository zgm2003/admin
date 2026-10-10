package profile

import (
	"context"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

type Option struct {
	Value int16  `json:"value"`
	Label string `json:"label"`
}
type Options struct {
	Genders []Option `json:"genders"`
}

// Options are code-owned presets; this path has no database, cache or SDK I/O.
func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	result := Options{Genders: make([]Option, 0, 3)}
	for _, value := range []int16{GenderUnknown, GenderMale, GenderFemale} {
		label, err := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.MessageKey("options.userGender."+genderKey(value)), nil)
		if err != nil {
			return Options{}, apperror.DependencyUnavailable(err)
		}
		result.Genders = append(result.Genders, Option{Value: value, Label: label})
	}

	return result, nil
}

const (
	GenderUnknown int16 = iota
	GenderMale
	GenderFemale
)

func validGender(value int16) bool {
	return value == GenderUnknown || value == GenderMale || value == GenderFemale
}
func genderKey(value int16) string {
	switch value {
	case GenderMale:
		return "1"
	case GenderFemale:
		return "2"
	default:
		return "0"
	}
}
