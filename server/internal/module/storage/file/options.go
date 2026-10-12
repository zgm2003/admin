package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
)

type CategoryOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type OptionsResult struct {
	Categories  []CategoryOption              `json:"categories"`
	UploadRules []uploadrule.FileUploadOption `json:"uploadRules"`
}

func (s *Service) Options(ctx context.Context, id auth.Identity, purpose string) (OptionsResult, error) {
	if !validIdentity(id) || purpose != "" && purpose != "avatar" {
		return OptionsResult{}, apperror.InvalidRequest(errors.New("file options invalid"))
	}
	rules, e := s.rules.FileUploadOptions(ctx, id.PlatformID, purpose)
	if e != nil {
		return OptionsResult{}, mapError(e)
	}
	categories := make([]CategoryOption, 0, 5)
	for _, v := range []string{"image", "video", "audio", "document", "other"} {
		label, e := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.MessageKey("storage.file.category."+v), nil)
		if e != nil {
			return OptionsResult{}, mapError(e)
		}
		categories = append(categories, CategoryOption{v, label})
	}
	return OptionsResult{categories, rules}, nil
}
