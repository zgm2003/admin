package mail

import (
	"context"
	"fmt"

	maillog "admin/server/internal/module/message/mail/log"
	policy "admin/server/internal/module/message/mail/rateLimitPolicy"
	rule "admin/server/internal/module/message/mail/recipientRule"
	"admin/server/internal/module/message/mail/template"
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

// Options only projects code-owned constants; it never reads a runtime store.
func Options(ctx context.Context) OptionsResult {
	result := OptionsResult{
		RateLimitConstraints: policy.Constraints(),
		Scenes:               make([]sceneOption, 0, 4),
		Statuses: []statusOption{
			{option.New(ctx, maillog.StatusPending, "待发送", "Pending"), "info"},
			{option.New(ctx, maillog.StatusSent, "已发送", "Sent"), "success"},
			{option.New(ctx, maillog.StatusFailed, "发送失败", "Failed"), "danger"},
		},
		RuleScopes:          []option.Option[rule.Scope]{option.New(ctx, rule.ScopeEmail, "邮箱", "Email"), option.New(ctx, rule.ScopeDomain, "域名", "Domain")},
		RuleActions:         []actionOption{{option.New(ctx, rule.ActionAllow, "允许", "Allow"), "success"}, {option.New(ctx, rule.ActionDeny, "拒绝", "Deny"), "danger"}},
		RateLimitPolicies:   make([]option.Option[string], 0, 2),
		RateLimitModes:      make([]option.Option[string], 0, 1),
		RateLimitDimensions: make([]option.Option[string], 0, 1),
	}
	result.RuleDefaults.Scope, result.RuleDefaults.Action = rule.ScopeEmail, rule.ActionDeny
	result.ImportConstraints.MaxBytes, result.ImportConstraints.MaxRows = rule.XlsxMaxBytes, rule.XlsxMaxRows
	for _, fixed := range template.FixedCatalog() {
		zh, en := fixed.Name, fixed.Scene
		switch fixed.Scene {
		case template.SceneLogin:
			zh, en = "登录验证码", "Login verification code"
		case template.SceneForget:
			en = "Password recovery"
		case template.SceneBindEmail:
			en = "Bind or change email"
		case template.SceneChangePassword:
			en = "Change password"
		}
		result.Scenes = append(result.Scenes, sceneOption{option.New(ctx, fixed.Scene, zh, en), append([]string(nil), fixed.Variables...)})
	}
	seenModes, seenDimensions := map[string]bool{}, map[string]bool{}
	for _, fixed := range policy.FixedRateLimitPolicies() {
		result.RateLimitPolicies = append(result.RateLimitPolicies, option.New(ctx, fixed.Key, fmt.Sprintf("业务邮箱发送 · %d 秒窗口", fixed.WindowSeconds), fmt.Sprintf("Business email sending · %ds window", fixed.WindowSeconds)))
		if !seenModes[fixed.Mode] {
			result.RateLimitModes = append(result.RateLimitModes, option.New(ctx, fixed.Mode, "业务发送", "Business sending"))
			seenModes[fixed.Mode] = true
		}
		if !seenDimensions[fixed.Dimension] {
			result.RateLimitDimensions = append(result.RateLimitDimensions, option.New(ctx, fixed.Dimension, "认证平台 + 邮箱", "Authentication platform + email"))
			seenDimensions[fixed.Dimension] = true
		}
	}
	return result
}

func PageInit(ctx context.Context) struct {
	Scenes []template.Fixed `json:"scenes"`
} {
	rows := template.FixedCatalog()
	for index, scene := range Options(ctx).Scenes {
		rows[index].Name = scene.Label
	}
	return struct {
		Scenes []template.Fixed `json:"scenes"`
	}{rows}
}
