package mail

import (
	"admin/server/internal/shared/yesno"
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"sort"
	"strings"
)

var domainLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func NormalizeRecipient(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || !strings.Contains(value, "@") {
		return "", fmt.Errorf("invalid recipient email")
	}
	parts := strings.Split(value, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid recipient email")
	}
	return value, nil
}
func NormalizeRule(scope, pattern string) (string, error) {
	pattern = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(pattern, ".")))
	if scope == RuleScopeEmail {
		return NormalizeRecipient(pattern)
	}
	if scope != RuleScopeDomain || len(pattern) > 253 {
		return "", fmt.Errorf("invalid recipient rule scope")
	}
	labels := strings.Split(pattern, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("invalid domain")
	}
	for _, label := range labels {
		if !domainLabel.MatchString(label) {
			return "", fmt.Errorf("invalid domain")
		}
	}
	return pattern, nil
}

type RuleService struct{ repository *Repository }

func NewRuleService(r *Repository) *RuleService { return &RuleService{repository: r} }
func (s *RuleService) Evaluate(ctx context.Context, platformID int64, email string, mode SendMode) (RuleDecision, error) {
	email, err := NormalizeRecipient(email)
	if err != nil {
		return RuleDecision{}, err
	}
	rows, err := s.repository.ListRules(ctx, platformID)
	if err != nil {
		return RuleDecision{}, err
	}
	var exact, domain []RecipientRule
	for _, row := range rows {
		if row.IsEnabled != yesno.Yes {
			continue
		}
		if row.Scope == RuleScopeEmail && row.Pattern == email {
			exact = append(exact, row)
		}
		if row.Scope == RuleScopeDomain && matchesDomain(email, row.Pattern) {
			domain = append(domain, row)
		}
	}
	sort.Slice(exact, func(i, j int) bool { return exact[i].ID < exact[j].ID })
	sort.Slice(domain, func(i, j int) bool { return domain[i].ID < domain[j].ID })
	return chooseRule(exact, domain), nil
}
func chooseRule(groups ...[]RecipientRule) RuleDecision {
	for _, group := range groups {
		for _, action := range []string{RuleActionDeny, RuleActionAllow} {
			for _, r := range group {
				if r.Action == action {
					return RuleDecision{Allowed: action == RuleActionAllow, RuleID: r.ID, Reason: action}
				}
			}
		}
	}
	return RuleDecision{Allowed: true, Reason: "default_allow"}
}
func matchesDomain(email, pattern string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	host := strings.ToLower(parts[1])
	return host == pattern || strings.HasSuffix(host, "."+pattern)
}
