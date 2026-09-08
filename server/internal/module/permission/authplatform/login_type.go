package authplatform

import (
	"encoding/json"
	"fmt"
)

// LoginType is a single supported authentication login method.
type LoginType string

const (
	LoginTypeEmail    LoginType = "email"
	LoginTypePhone    LoginType = "phone"
	LoginTypePassword LoginType = "password"
)

// LoginTypeOption pairs a login type with its localized label.
type LoginTypeOption struct {
	Value LoginType `json:"value"`
	Label string    `json:"label"`
}

// LoginConfig is the effective, channel-filtered login configuration exposed
// to the public login endpoint.
type LoginConfig struct {
	LoginTypes    []LoginTypeOption
	AllowRegister bool
}

var loginTypeCanonicalOrder = []LoginType{LoginTypeEmail, LoginTypePhone, LoginTypePassword}

// normalizeLoginTypes rejects invalid, duplicate, empty and over-long sets and
// returns the values in the fixed canonical order email -> phone -> password.
func normalizeLoginTypes(types []LoginType) ([]LoginType, error) {
	if len(types) < 1 || len(types) > 3 {
		return nil, fmt.Errorf("login types must contain 1 to 3 values")
	}
	seen := make(map[LoginType]struct{}, len(types))
	for _, item := range types {
		switch item {
		case LoginTypeEmail, LoginTypePhone, LoginTypePassword:
		default:
			return nil, fmt.Errorf("login type %q is invalid", item)
		}
		if _, exists := seen[item]; exists {
			return nil, fmt.Errorf("login type %q is duplicated", item)
		}
		seen[item] = struct{}{}
	}
	result := make([]LoginType, 0, len(types))
	for _, item := range loginTypeCanonicalOrder {
		if _, exists := seen[item]; exists {
			result = append(result, item)
		}
	}
	return result, nil
}

// parseLoginTypes decodes the JSONB column value and normalizes it.
func parseLoginTypes(raw json.RawMessage) ([]LoginType, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("login types are missing")
	}
	var types []LoginType
	if err := json.Unmarshal(raw, &types); err != nil {
		return nil, fmt.Errorf("decode login types: %w", err)
	}
	return normalizeLoginTypes(types)
}

// marshalLoginTypes normalizes and encodes a login type set for storage.
func marshalLoginTypes(types []LoginType) (json.RawMessage, error) {
	normalized, err := normalizeLoginTypes(types)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode login types: %w", err)
	}
	return raw, nil
}
