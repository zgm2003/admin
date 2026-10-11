package authplatform

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
	"fmt"
)

type platformActions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}

type platformPresentation struct {
	LoginTypes       []option.Option[LoginType] `json:"loginTypes"`
	MaxSessionsLabel string                     `json:"maxSessionsLabel"`
	DeleteReason     string                     `json:"deleteReason"`
}

func newPlatformPresentation(ctx context.Context, value Platform, loginTypes []LoginType) (platformPresentation, error) {
	labels := make([]option.Option[LoginType], 0, len(loginTypes))
	options, err := loginTypeOptions(ctx)
	if err != nil {
		return platformPresentation{}, err
	}
	for _, loginType := range loginTypes {
		label := string(loginType)
		for _, candidate := range options {
			if candidate.Value == loginType {
				label = candidate.Label
				break
			}
		}
		labels = append(labels, option.Option[LoginType]{Value: loginType, Label: label})
	}
	sessionLabel := fmt.Sprint(value.MaxSessions)
	switch {
	case value.MaxSessions == 0:
		sessionLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyAuthPlatformMaxSessionsUnlimited, nil)
		if err != nil {
			return platformPresentation{}, err
		}
	case value.MaxSessions == 1:
		sessionLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyAuthPlatformMaxSessionsSingle, nil)
		if err != nil {
			return platformPresentation{}, err
		}
	case value.MaxSessions > 1:
		sessionLabel, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyAuthPlatformMaxSessionsMany, map[string]string{"count": fmt.Sprintf("%d", value.MaxSessions)})
		if err != nil {
			return platformPresentation{}, err
		}
	}
	reason := ""
	if value.IsBuiltin == yesno.Yes {
		reason, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyAuthPlatformDeleteReasonBuiltin, nil)
		if err != nil {
			return platformPresentation{}, err
		}
	}
	return platformPresentation{LoginTypes: labels, MaxSessionsLabel: sessionLabel, DeleteReason: reason}, nil
}
