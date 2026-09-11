package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"admin/server/internal/module/message/sms/config"
	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

// Stores is the explicit composition of everything the sending service needs.
// Every dependency is a narrow interface owned by this package; there is no
// registry, factory or shared Mail abstraction.
type Stores struct {
	Runtime      runtimeStore
	Config       configSource
	Template     templateSource
	Rule         ruleSource
	Log          logStore
	Verification verificationStore
	Policy       policyStore
	Limiter      Limiter
	Sender       Sender
}

type runtimeStore interface {
	Load(context.Context, func(context.Context) (RuntimeFacts, error)) (RuntimeFacts, error)
	LoadReadiness(context.Context, string, func(context.Context) (VerifyCodeReadiness, error)) (VerifyCodeReadiness, error)
}

type configSource interface {
	FindActive(context.Context) (config.Model, error)
}

// notConfigured reports that no active SMS configuration row exists. The runtime
// snapshot represents this as a valid unconfigured state instead of failing.
func notConfigured(err error) bool {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	var appError *apperror.Error
	return errors.As(err, &appError) && appError.Code == apperror.CodeNotFound
}

type templateSource interface {
	List(context.Context) ([]template.Model, error)
}

type ruleSource interface {
	List(context.Context) ([]recipientRule.Model, error)
}

// runtimeLoader builds the runtime facts from PostgreSQL.
func (s Stores) runtimeLoader() func(context.Context) (RuntimeFacts, error) {
	return func(ctx context.Context) (RuntimeFacts, error) {
		configured, err := s.Config.FindActive(ctx)
		if err == nil {
			endpoint := ""
			if configured.Endpoint != nil {
				endpoint = *configured.Endpoint
			}
			facts := RuntimeFacts{
				Configured: true,
				Config: ConfiguredConfig{
					SecretIDCiphertext: configured.SecretIDCiphertext, SecretKeyCiphertext: configured.SecretKeyCiphertext,
					SDKAppID: configured.SDKAppID, SignName: configured.SignName,
					Region: configured.Region, Endpoint: endpoint,
					TTLMinutes: int(configured.TTLMinutes), IsEnabled: configured.IsEnabled,
				},
				Templates: make(map[string]TemplateFact),
			}
			if _, err := s.loadTemplatesInto(ctx, &facts); err != nil {
				return RuntimeFacts{}, err
			}
			if _, err := s.loadRulesInto(ctx, &facts); err != nil {
				return RuntimeFacts{}, err
			}
			return facts, nil
		}
		// A missing configuration is a valid runtime state (not yet configured).
		if notConfigured(err) {
			facts := RuntimeFacts{Configured: false, Templates: make(map[string]TemplateFact)}
			if _, err := s.loadTemplatesInto(ctx, &facts); err != nil {
				return RuntimeFacts{}, err
			}
			if _, err := s.loadRulesInto(ctx, &facts); err != nil {
				return RuntimeFacts{}, err
			}
			return facts, nil
		}
		return RuntimeFacts{}, err
	}
}

func (s Stores) loadTemplatesInto(ctx context.Context, facts *RuntimeFacts) (int, error) {
	rows, err := s.Template.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("sms template source: %w", err)
	}
	for _, row := range rows {
		fact := TemplateFact{Name: row.Name, TencentTemplateID: row.TencentTemplateID, IsEnabled: row.IsEnabled}
		if err := unmarshalJSON(row.ParameterKeys, &fact.ParameterKeys); err != nil {
			return 0, err
		}
		if err := unmarshalJSON(row.ExampleVariables, &fact.ExampleVariables); err != nil {
			return 0, err
		}
		facts.Templates[row.Scene] = fact
	}
	return len(rows), nil
}

func (s Stores) loadRulesInto(ctx context.Context, facts *RuntimeFacts) (int, error) {
	rows, err := s.Rule.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("sms rule source: %w", err)
	}
	for _, row := range rows {
		facts.Rules = append(facts.Rules, RuleFact{
			ID: row.ID, Scope: row.Scope, Action: row.Action,
			PatternCiphertext: row.PatternCiphertext, PatternHint: row.PatternHint,
			IsEnabled: row.IsEnabled,
		})
	}
	return len(rows), nil
}

func unmarshalJSON(raw []byte, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("json payload is missing")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode sms runtime payload: %w", err)
	}
	return nil
}
