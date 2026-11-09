package origins

import (
	"fmt"
	"net/url"
	"strings"
)

// Parse validates a comma-separated, exact-origin allowlist. Wildcards are
// intentionally rejected; each entry must be scheme://host[:port] without a
// path, query, fragment, or credentials.
func Parse(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			return nil, fmt.Errorf("origin must not be empty")
		}
		if origin == "*" || strings.Contains(origin, "*") {
			return nil, fmt.Errorf("wildcard origins are not supported")
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return nil, fmt.Errorf("invalid origin %q: %w", origin, err)
		}
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("origin %q must use http or https with a host", origin)
		}
		if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("origin %q must not contain credentials, path, query, or fragment", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("at least one origin is required")
	}
	return result, nil
}

func Contains(allowed []string, candidate string) bool {
	for _, origin := range allowed {
		if candidate == origin {
			return true
		}
	}
	return false
}
