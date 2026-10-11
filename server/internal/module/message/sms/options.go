package sms

import (
	"context"
	"fmt"
	"strconv"

	smslog "admin/server/internal/module/message/sms/log"
	policy "admin/server/internal/module/message/sms/rateLimitPolicy"
	rule "admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
)

type statusOption struct {
	option.Option[smslog.Status]
	Tone string `json:"tone"`
}
type localizedSceneOption struct {
	option.Option[string]
	VariableKeys []string `json:"variableKeys"`
}
type actionOption struct {
	option.Option[rule.Action]
	Tone string `json:"tone"`
}

type OptionsResult struct {
	RateLimitConstraints policy.InputConstraints     `json:"rateLimitConstraints"`
	Scenes               []localizedSceneOption      `json:"scenes"`
	Statuses             []statusOption              `json:"statuses"`
	RuleScopes           []option.Option[rule.Scope] `json:"ruleScopes"`
	RuleActions          []actionOption              `json:"ruleActions"`
	RuleDefaults         struct {
		Scope  rule.Scope  `json:"scope"`
		Action rule.Action `json:"action"`
	} `json:"ruleDefaults"`
	RateLimitPolicies   []option.Option[string] `json:"rateLimitPolicies"`
	RateLimitModes      []option.Option[string] `json:"rateLimitModes"`
	RateLimitDimensions []option.Option[string] `json:"rateLimitDimensions"`
}

// Options is code-only and has no PostgreSQL, Redis, or configuration dependency.
func Options(ctx context.Context) (OptionsResult, error) {
	result := OptionsResult{
		RateLimitConstraints: policy.Constraints(),
		Scenes:               make([]localizedSceneOption, 0, 4),
		Statuses:             make([]statusOption, 0, 3),
		RuleScopes:           make([]option.Option[rule.Scope], 0, 2),
		RuleActions:          make([]actionOption, 0, 2),
		RateLimitPolicies:    make([]option.Option[string], 0, 2),
		RateLimitModes:       make([]option.Option[string], 0, 1),
		RateLimitDimensions:  make([]option.Option[string], 0, 1),
	}
	for _, item := range []struct {
		value smslog.Status
		key   i18n.MessageKey
		tone  string
	}{
		{smslog.StatusPending, i18n.KeySMSOptionStatusPending, "info"},
		{smslog.StatusSent, i18n.KeySMSOptionStatusSent, "success"},
		{smslog.StatusFailed, i18n.KeySMSOptionStatusFailed, "danger"},
	} {
		localized, err := option.NewLocalized(ctx, item.value, item.key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.Statuses = append(result.Statuses, statusOption{Option: localized, Tone: item.tone})
	}
	for _, item := range []struct {
		value rule.Scope
		key   i18n.MessageKey
	}{
		{rule.ScopePhone, i18n.KeySMSOptionScopePhone},
		{rule.ScopePrefix, i18n.KeySMSOptionScopePrefix},
	} {
		localized, err := option.NewLocalized(ctx, item.value, item.key, nil)
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
		{rule.ActionAllow, i18n.KeySMSOptionActionAllow, "success"},
		{rule.ActionDeny, i18n.KeySMSOptionActionDeny, "danger"},
	} {
		localized, err := option.NewLocalized(ctx, item.value, item.key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.RuleActions = append(result.RuleActions, actionOption{Option: localized, Tone: item.tone})
	}
	localizedMode, err := option.NewLocalized(ctx, policy.ModeBusiness, i18n.KeySMSOptionRateLimitMode, nil)
	if err != nil {
		return OptionsResult{}, err
	}
	result.RateLimitModes = append(result.RateLimitModes, localizedMode)
	localizedDimension, err := option.NewLocalized(ctx, policy.DimensionPhone, i18n.KeySMSOptionRateLimitDimension, nil)
	if err != nil {
		return OptionsResult{}, err
	}
	result.RateLimitDimensions = append(result.RateLimitDimensions, localizedDimension)
	result.RuleDefaults.Scope, result.RuleDefaults.Action = rule.ScopePhone, rule.ActionDeny
	sceneKeys := map[string]i18n.MessageKey{
		template.SceneLogin:          i18n.KeySMSOptionSceneLogin,
		template.SceneForget:         i18n.KeySMSOptionSceneForget,
		template.SceneBindPhone:      i18n.KeySMSOptionSceneBindPhone,
		template.SceneChangePassword: i18n.KeySMSOptionSceneChangePassword,
	}
	for _, fixed := range template.FixedCatalog() {
		key, ok := sceneKeys[fixed.Scene]
		if !ok {
			return OptionsResult{}, fmt.Errorf("sms scene %q has no localized label key", fixed.Scene)
		}
		localized, err := option.NewLocalized(ctx, fixed.Scene, key, nil)
		if err != nil {
			return OptionsResult{}, err
		}
		result.Scenes = append(result.Scenes, localizedSceneOption{Option: localized, VariableKeys: append([]string(nil), fixed.VariableKeys...)})
	}
	for _, fixed := range policy.FixedPolicies() {
		localized, err := option.NewLocalized(ctx, fixed.Key, i18n.KeySMSOptionRateLimitPolicy, map[string]string{"windowSeconds": strconv.Itoa(fixed.WindowSeconds)})
		if err != nil {
			return OptionsResult{}, err
		}
		result.RateLimitPolicies = append(result.RateLimitPolicies, localized)
	}
	return result, nil
}
