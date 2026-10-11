package mail

import (
	"context"
	"fmt"
	"strconv"

	maillog "admin/server/internal/module/message/mail/log"
	policy "admin/server/internal/module/message/mail/rateLimitPolicy"
	rule "admin/server/internal/module/message/mail/recipientRule"
	"admin/server/internal/module/message/mail/template"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
)

type statusOption struct {
	option.Option[maillog.Status]
	Tone string `json:"tone"`
}

type sceneOption struct {
	option.Option[string]
	VariableKeys []string `json:"variableKeys"`
}

type actionOption struct {
	option.Option[rule.Action]
	Tone string `json:"tone"`
}

type OptionsResult struct {
	RateLimitConstraints policy.InputConstraints `json:"rateLimitConstraints"`
	ImportConstraints    struct {
		MaxBytes int `json:"maxBytes"`
		MaxRows  int `json:"maxRows"`
	} `json:"importConstraints"`
	Scenes       []sceneOption               `json:"scenes"`
	Statuses     []statusOption              `json:"statuses"`
	RuleScopes   []option.Option[rule.Scope] `json:"ruleScopes"`
	RuleActions  []actionOption              `json:"ruleActions"`
	RuleDefaults struct {
		Scope  rule.Scope  `json:"scope"`
		Action rule.Action `json:"action"`
	} `json:"ruleDefaults"`
	RateLimitPolicies   []option.Option[string] `json:"rateLimitPolicies"`
	RateLimitModes      []option.Option[string] `json:"rateLimitModes"`
	RateLimitDimensions []option.Option[string] `json:"rateLimitDimensions"`
}

type PageInitResult struct {
	Scenes []template.Fixed `json:"scenes"`
}

func localizedOption[T comparable](ctx context.Context, value T, key i18n.MessageKey, params map[string]string) (option.Option[T], error) {
	return option.NewLocalized(ctx, value, key, params)
}

// Options only projects code-owned constants; it never reads a runtime store.
func Options(ctx context.Context) (OptionsResult, error) {
	result := OptionsResult{
		RateLimitConstraints: policy.Constraints(),
		Scenes:               make([]sceneOption, 0, 4),
		Statuses:             make([]statusOption, 0, 3),
		RuleScopes:           make([]option.Option[rule.Scope], 0, 2),
		RuleActions:          make([]actionOption, 0, 2),
		RateLimitPolicies:    make([]option.Option[string], 0, 2),
		RateLimitModes:       make([]option.Option[string], 0, 1),
		RateLimitDimensions:  make([]option.Option[string], 0, 1),
	}
	for _, item := range []struct {
		value maillog.Status
		key   i18n.MessageKey
		tone  string
	}{
		{maillog.StatusPending, i18n.KeyMailOptionStatusPending, "info"},
		{maillog.StatusSent, i18n.KeyMailOptionStatusSent, "success"},
		{maillog.StatusFailed, i18n.KeyMailOptionStatusFailed, "danger"},
	} {
		localized, err := localizedOption(ctx, item.value, item.key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.Statuses = append(result.Statuses, statusOption{Option: localized, Tone: item.tone})
	}
	for _, item := range []struct {
		value rule.Scope
		key   i18n.MessageKey
	}{
		{rule.ScopeEmail, i18n.KeyMailOptionScopeEmail},
		{rule.ScopeDomain, i18n.KeyMailOptionScopeDomain},
	} {
		localized, err := localizedOption(ctx, item.value, item.key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.RuleScopes = append(result.RuleScopes, localized)
	}
	for _, item := range []struct {
		value rule.Action
		key   i18n.MessageKey
		tone  string
	}{
		{rule.ActionAllow, i18n.KeyMailOptionActionAllow, "success"},
		{rule.ActionDeny, i18n.KeyMailOptionActionDeny, "danger"},
	} {
		localized, err := localizedOption(ctx, item.value, item.key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.RuleActions = append(result.RuleActions, actionOption{Option: localized, Tone: item.tone})
	}
	result.RuleDefaults.Scope, result.RuleDefaults.Action = rule.ScopeEmail, rule.ActionDeny
	result.ImportConstraints.MaxBytes, result.ImportConstraints.MaxRows = rule.XlsxMaxBytes, rule.XlsxMaxRows
	sceneKeys := map[string]i18n.MessageKey{
		template.SceneLogin:          i18n.KeyMailOptionSceneLogin,
		template.SceneForget:         i18n.KeyMailOptionSceneForget,
		template.SceneBindEmail:      i18n.KeyMailOptionSceneBindEmail,
		template.SceneChangePassword: i18n.KeyMailOptionSceneChangePassword,
	}
	for _, fixed := range template.FixedCatalog() {
		key, ok := sceneKeys[fixed.Scene]
		if !ok {
			return OptionsResult{}, fmt.Errorf("mail scene %q has no localized label key", fixed.Scene)
		}
		localized, err := localizedOption(ctx, fixed.Scene, key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.Scenes = append(result.Scenes, sceneOption{Option: localized, VariableKeys: append([]string(nil), fixed.Variables...)})
	}
	seenModes, seenDimensions := map[string]bool{}, map[string]bool{}
	for _, fixed := range policy.FixedRateLimitPolicies() {
		localized, err := localizedOption(ctx, fixed.Key, i18n.KeyMailOptionRateLimitPolicy, map[string]string{"windowSeconds": strconv.Itoa(fixed.WindowSeconds)})
		if err != nil {
			return OptionsResult{}, err
		}
		result.RateLimitPolicies = append(result.RateLimitPolicies, localized)
		if !seenModes[fixed.Mode] {
			localized, err := localizedOption(ctx, fixed.Mode, i18n.KeyMailOptionRateLimitMode, nil)
			if err != nil {
				return OptionsResult{}, err
			}
			result.RateLimitModes = append(result.RateLimitModes, localized)
			seenModes[fixed.Mode] = true
		}
		if !seenDimensions[fixed.Dimension] {
			localized, err := localizedOption(ctx, fixed.Dimension, i18n.KeyMailOptionRateLimitDimension, nil)
			if err != nil {
				return OptionsResult{}, err
			}
			result.RateLimitDimensions = append(result.RateLimitDimensions, localized)
			seenDimensions[fixed.Dimension] = true
		}
	}
	return result, nil
}

func PageInit(ctx context.Context) (PageInitResult, error) {
	options, err := Options(ctx)
	if err != nil {
		return PageInitResult{}, err
	}
	rows := template.FixedCatalog()
	subjectKeys := map[string]i18n.MessageKey{
		template.SceneLogin:          i18n.KeyMailOptionSubjectLogin,
		template.SceneForget:         i18n.KeyMailOptionSubjectForget,
		template.SceneBindEmail:      i18n.KeyMailOptionSubjectBindEmail,
		template.SceneChangePassword: i18n.KeyMailOptionSubjectChangePassword,
	}
	for index, scene := range options.Scenes {
		rows[index].Name = scene.Label
		subject, err := option.NewLocalized(ctx, scene.Value, subjectKeys[scene.Value], nil)
		if err != nil {
			return PageInitResult{}, err
		}
		rows[index].Subject = subject.Label
	}
	return PageInitResult{Scenes: rows}, nil
}
