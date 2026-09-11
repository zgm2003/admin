package recipientRule

import "strings"

// RulePattern is one decrypted rule for the runtime matcher.
type RulePattern struct {
	ID      int64
	Scope   string
	Action  string
	Pattern string
}

// Match resolves one phone deterministically: an exact phone rule outranks every
// prefix, a longer prefix outranks a shorter one, and deny outranks allow at the
// same level. No match means the default allow.
func Match(toPhone string, patterns []RulePattern) Decision {
	decision := Decision{Allowed: true, Reason: ReasonDefaultAllow}
	matched := false
	var bestExact, bestDeny bool
	var bestLength int
	for _, pattern := range patterns {
		exact := pattern.Scope == ScopePhone && pattern.Pattern == toPhone
		prefix := pattern.Scope == ScopePrefix && strings.HasPrefix(toPhone, pattern.Pattern)
		if !exact && !prefix {
			continue
		}
		deny := pattern.Action == ActionDeny
		length := len(pattern.Pattern)
		switch {
		case !matched:
		case exact != bestExact:
			if !exact {
				continue
			}
		case length != bestLength:
			if length < bestLength {
				continue
			}
		case deny == bestDeny:
			if !deny {
				continue
			}
		case deny:
		default:
			continue
		}
		matched = true
		bestExact, bestLength, bestDeny = exact, length, deny
		reason := ReasonPrefixMatch
		if exact {
			reason = ReasonPhoneExact
		}
		decision = Decision{Allowed: !deny, RuleID: pattern.ID, Reason: reason}
	}
	return decision
}
