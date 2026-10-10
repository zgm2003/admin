package sms

import (
	"context"
	"fmt"

	smslog "admin/server/internal/module/message/sms/log"
	policy "admin/server/internal/module/message/sms/rateLimitPolicy"
	rule "admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
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
func Options(ctx context.Context) OptionsResult {
	result := OptionsResult{
		RateLimitConstraints: policy.Constraints(),
		Scenes:               make([]localizedSceneOption, 0, 4),
		Statuses: []statusOption{
			{option.New(ctx, smslog.StatusPending, "待发送", "Pending"), "info"},
			{option.New(ctx, smslog.StatusSent, "已发送", "Sent"), "success"},
			{option.New(ctx, smslog.StatusFailed, "发送失败", "Failed"), "danger"},
		},
		RuleScopes:          []option.Option[rule.Scope]{option.New(ctx, rule.ScopePhone, "手机号", "Phone number"), option.New(ctx, rule.ScopePrefix, "号段", "Phone prefix")},
		RuleActions:         []actionOption{{option.New(ctx, rule.ActionAllow, "允许", "Allow"), "success"}, {option.New(ctx, rule.ActionDeny, "拒绝", "Deny"), "danger"}},
		RateLimitPolicies:   make([]option.Option[string], 0, 2),
		RateLimitModes:      []option.Option[string]{option.New(ctx, policy.ModeBusiness, "业务发送", "Business sending")},
		RateLimitDimensions: []option.Option[string]{option.New(ctx, policy.DimensionPhone, "认证平台 + 手机号", "Authentication platform + phone number")},
	}
	result.RuleDefaults.Scope, result.RuleDefaults.Action = rule.ScopePhone, rule.ActionDeny
	for _, fixed := range template.FixedCatalog() {
		en := fixed.Scene
		switch fixed.Scene {
		case template.SceneLogin:
			en = "Login verification code"
		case template.SceneForget:
			en = "Password recovery"
		case template.SceneBindPhone:
			en = "Bind or change phone"
		case template.SceneChangePassword:
			en = "Change password"
		}
		result.Scenes = append(result.Scenes, localizedSceneOption{option.New(ctx, fixed.Scene, fixed.Name, en), append([]string(nil), fixed.VariableKeys...)})
	}
	for _, fixed := range policy.FixedPolicies() {
		result.RateLimitPolicies = append(result.RateLimitPolicies, option.New(ctx, fixed.Key, fmt.Sprintf("业务手机号发送 · %d 秒窗口", fixed.WindowSeconds), fmt.Sprintf("Business phone sending · %ds window", fixed.WindowSeconds)))
	}
	return result
}
