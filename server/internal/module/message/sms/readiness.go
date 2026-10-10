package sms

import (
	"fmt"

	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/shared/yesno"
)

// readinessOf derives one scene's readiness from the runtime facts. A missing
// configuration or template is a dependency failure, never a silent not-ready.
func readinessOf(facts RuntimeFacts, scene string) (VerifyCodeReadiness, error) {
	if err := validScene(scene); err != nil {
		return VerifyCodeReadiness{}, err
	}
	if !facts.Configured || facts.Config.IsEnabled != yesno.Yes {
		return VerifyCodeReadiness{}, ErrConfigurationMissing
	}
	template, found := facts.Templates[scene]
	if !found {
		return VerifyCodeReadiness{}, fmt.Errorf("sms template %q is missing", scene)
	}
	if template.IsEnabled != yesno.Yes || template.TencentTemplateID == "" {
		return VerifyCodeReadiness{Ready: false}, nil
	}
	return VerifyCodeReadiness{Ready: true, TTLMinutes: facts.Config.TTLMinutes}, nil
}

// evaluateRules delegates plaintext patterns to the single precedence
// implementation shared with the management module.
func evaluateRules(rules []RuleFact, toPhone string) (recipientRule.Decision, error) {
	patterns := make([]recipientRule.RulePattern, 0, len(rules))
	for _, rule := range rules {
		if rule.IsEnabled != yesno.Yes {
			continue
		}
		patterns = append(patterns, recipientRule.RulePattern{ID: rule.ID, Scope: rule.Scope, Action: rule.Action, Pattern: rule.Pattern})
	}
	return recipientRule.Match(toPhone, patterns), nil
}

func validScene(scene string) error {
	if _, found := sceneOf(scene); !found {
		return invalidRequest(ErrInvalidScene)
	}
	return nil
}

func sceneOf(scene string) (struct{}, bool) {
	for _, value := range []string{"login", "forget", "bind_phone", "change_password"} {
		if scene == value {
			return struct{}{}, true
		}
	}
	return struct{}{}, false
}
