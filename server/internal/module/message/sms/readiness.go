package sms

import (
	"fmt"

	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/secretkey"
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

// evaluateRules decrypts the runtime rule ciphertexts and delegates to the
// single precedence implementation shared with the management module.
func evaluateRules(keys *secretkey.KeyRing, rules []RuleFact, toPhone string) (recipientRule.Decision, error) {
	if keys == nil {
		return recipientRule.Decision{}, fmt.Errorf("sms keys are unavailable")
	}
	patterns := make([]recipientRule.RulePattern, 0, len(rules))
	for _, rule := range rules {
		if rule.IsEnabled != yesno.Yes {
			continue
		}
		pattern, err := secretkey.DecryptSMSValue(keys.SMSEncryptionKey(), rule.PatternCiphertext)
		if err != nil {
			return recipientRule.Decision{}, fmt.Errorf("decrypt sms recipient rule pattern: %w", err)
		}
		patterns = append(patterns, recipientRule.RulePattern{ID: rule.ID, Scope: rule.Scope, Action: rule.Action, Pattern: pattern})
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
