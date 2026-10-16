package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/module/payment"
	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"
)

type API struct {
	HTTPAddr             string
	PostgresDSN          string
	RedisURL             string
	CORSOrigin           string
	AppSecret            string
	TrustedProxies       []string
	TrustedProxyMode     string
	Auth                 Auth
	Realtime             Realtime
	PaymentEnvironment   payment.Environment
	PaymentReturnOrigins []string
}

type Auth struct {
	CookieSecure bool
}

type Realtime struct {
	MaxConnections        int
	MaxConnectionsPerUser int
	ResumeConcurrency     int
}

type Worker struct {
	PostgresDSN        string
	RedisURL           string
	PaymentEnvironment payment.Environment
}

type LookupEnv func(string) (string, bool)

func LoadAPI(lookupEnv LookupEnv) (API, error) {
	httpAddr, err := required(lookupEnv, "HTTP_ADDR")
	if err != nil {
		return API{}, err
	}
	if err := validateHTTPAddr(httpAddr); err != nil {
		return API{}, fmt.Errorf("HTTP_ADDR: %w", err)
	}

	postgresDSN, err := loadPostgresDSN(lookupEnv)
	if err != nil {
		return API{}, err
	}
	redisURL, err := loadRedisURL(lookupEnv)
	if err != nil {
		return API{}, err
	}
	corsOrigin, err := required(lookupEnv, "CORS_ORIGIN")
	if err != nil {
		return API{}, err
	}
	if err := validateOrigin(corsOrigin); err != nil {
		return API{}, fmt.Errorf("CORS_ORIGIN: %w", err)
	}
	appSecret, err := loadAppSecret(lookupEnv)
	if err != nil {
		return API{}, err
	}
	trustedProxies, trustedProxyMode, err := loadTrustedProxies(lookupEnv)
	if err != nil {
		return API{}, err
	}
	auth, err := loadAuth(lookupEnv, corsOrigin)
	if err != nil {
		return API{}, err
	}
	realtime, err := loadRealtime(lookupEnv)
	if err != nil {
		return API{}, err
	}
	paymentEnvironment, err := loadPaymentEnvironment(lookupEnv)
	if err != nil {
		return API{}, err
	}
	paymentReturnOrigins, err := loadPaymentReturnOrigins(lookupEnv)
	if err != nil {
		return API{}, err
	}

	return API{
		HTTPAddr:             httpAddr,
		PostgresDSN:          postgresDSN,
		RedisURL:             redisURL,
		CORSOrigin:           corsOrigin,
		AppSecret:            appSecret,
		TrustedProxies:       trustedProxies,
		TrustedProxyMode:     trustedProxyMode,
		Auth:                 auth,
		Realtime:             realtime,
		PaymentEnvironment:   paymentEnvironment,
		PaymentReturnOrigins: paymentReturnOrigins,
	}, nil
}

func loadPaymentEnvironment(lookupEnv LookupEnv) (payment.Environment, error) {
	raw, ok := lookupEnv("PAYMENT_ENVIRONMENT")
	if !ok || raw == "" {
		return 0, fmt.Errorf("PAYMENT_ENVIRONMENT is required")
	}
	environment, err := payment.ParseEnvironment(raw)
	if err != nil {
		return 0, fmt.Errorf("PAYMENT_ENVIRONMENT: invalid value")
	}
	return environment, nil
}

// loadPaymentReturnOrigins accepts an empty value so a deployment can keep
// read-only wallet/history and callback handling available while purchases are
// explicitly unavailable until a browser origin is configured.
func loadPaymentReturnOrigins(lookupEnv LookupEnv) ([]string, error) {
	raw, ok := lookupEnv("PAYMENT_RETURN_ORIGINS")
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || len(value) > 2048 || strings.ContainsAny(value, "?#") {
			return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: invalid origin")
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" {
			return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: invalid origin")
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" || strings.ContainsAny(parsed.Host, "\\/\t\r\n @*") {
			return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: invalid origin")
		}
		hostname := parsed.Hostname()
		if hostname == "" || strings.HasSuffix(parsed.Host, ":") {
			return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: invalid origin")
		}
		if port := parsed.Port(); port != "" {
			portNumber, portErr := strconv.Atoi(port)
			if portErr != nil || portNumber < 1 || portNumber > 65535 {
				return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: invalid origin")
			}
		}
		origin := scheme + "://" + strings.ToLower(parsed.Host)
		if _, exists := seen[origin]; exists {
			return nil, fmt.Errorf("PAYMENT_RETURN_ORIGINS: duplicate origin")
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
}

func LoadWorker(lookupEnv LookupEnv) (Worker, error) {
	postgresDSN, err := loadPostgresDSN(lookupEnv)
	if err != nil {
		return Worker{}, err
	}
	redisURL, err := loadRedisURL(lookupEnv)
	if err != nil {
		return Worker{}, err
	}
	paymentEnvironment, err := loadPaymentEnvironment(lookupEnv)
	if err != nil {
		return Worker{}, err
	}

	return Worker{PostgresDSN: postgresDSN, RedisURL: redisURL, PaymentEnvironment: paymentEnvironment}, nil
}

func loadPostgresDSN(lookupEnv LookupEnv) (string, error) {
	value, err := required(lookupEnv, "POSTGRES_DSN")
	if err != nil {
		return "", err
	}
	if _, err := pgx.ParseConfig(value); err != nil {
		return "", fmt.Errorf("POSTGRES_DSN: %w", err)
	}
	return value, nil
}

func loadRedisURL(lookupEnv LookupEnv) (string, error) {
	value, err := required(lookupEnv, "REDIS_URL")
	if err != nil {
		return "", err
	}
	if _, err := goredis.ParseURL(value); err != nil {
		return "", fmt.Errorf("REDIS_URL: %w", err)
	}
	return value, nil
}

func loadAppSecret(lookupEnv LookupEnv) (string, error) {
	value, err := required(lookupEnv, "APP_SECRET")
	if err != nil {
		return "", err
	}
	if err := validateAppSecret(value); err != nil {
		return "", fmt.Errorf("APP_SECRET: %w", err)
	}
	return value, nil
}

func loadTrustedProxies(lookupEnv LookupEnv) ([]string, string, error) {
	raw, err := required(lookupEnv, "TRUSTED_PROXIES")
	if err != nil {
		return nil, "", err
	}
	if raw == "none" {
		return []string{}, "none", nil
	}

	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, rawEntry := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(rawEntry)
		if entry == "" || entry == "none" {
			return nil, "", fmt.Errorf("TRUSTED_PROXIES: unsafe entry %q", entry)
		}
		if net.ParseIP(entry) == nil {
			_, network, parseErr := net.ParseCIDR(entry)
			if parseErr != nil {
				return nil, "", fmt.Errorf("TRUSTED_PROXIES: invalid entry %q", entry)
			}
			ones, _ := network.Mask.Size()
			if ones == 0 {
				return nil, "", fmt.Errorf("TRUSTED_PROXIES: unsafe entry %q", entry)
			}
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		values = append(values, entry)
	}
	if len(values) == 0 {
		return nil, "", fmt.Errorf("TRUSTED_PROXIES: allowlist is empty")
	}
	return values, "allowlist", nil
}

func loadAuth(lookupEnv LookupEnv, corsOrigin string) (Auth, error) {
	value, err := required(lookupEnv, "AUTH_COOKIE_SECURE")
	if err != nil {
		return Auth{}, err
	}
	if value != "0" && value != "1" {
		return Auth{}, fmt.Errorf("AUTH_COOKIE_SECURE: must be 0 or 1")
	}

	origin, err := url.Parse(corsOrigin)
	if err != nil {
		return Auth{}, fmt.Errorf("CORS_ORIGIN: invalid origin: %w", err)
	}
	wantSecure := origin.Scheme == "https"
	cookieSecure := value == "1"
	if cookieSecure != wantSecure {
		return Auth{}, fmt.Errorf("AUTH_COOKIE_SECURE: must be %d when CORS_ORIGIN uses %s", boolCode(wantSecure), origin.Scheme)
	}
	return Auth{CookieSecure: cookieSecure}, nil
}

func loadRealtime(lookupEnv LookupEnv) (Realtime, error) {
	maxConnections, err := optionalBoundedInt(lookupEnv, "REALTIME_MAX_CONNECTIONS", 20000, 1, 200000)
	if err != nil {
		return Realtime{}, err
	}
	maxConnectionsPerUser, err := optionalBoundedInt(lookupEnv, "REALTIME_MAX_CONNECTIONS_PER_USER", 32, 1, maxConnections)
	if err != nil {
		return Realtime{}, err
	}
	resumeConcurrency, err := optionalBoundedInt(lookupEnv, "REALTIME_RESUME_CONCURRENCY", 32, 1, 1024)
	if err != nil {
		return Realtime{}, err
	}
	return Realtime{
		MaxConnections:        maxConnections,
		MaxConnectionsPerUser: maxConnectionsPerUser,
		ResumeConcurrency:     resumeConcurrency,
	}, nil
}

func optionalBoundedInt(lookupEnv LookupEnv, key string, defaultValue, minimum, maximum int) (int, error) {
	raw, ok := lookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s: must be an integer between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func validateAppSecret(value string) error {
	const placeholder = "replace_with_at_least_64_random_characters_before_running_api_server"
	if value == placeholder {
		return fmt.Errorf("placeholder value is not allowed")
	}
	if len(value) < 64 {
		return fmt.Errorf("must contain at least 64 ASCII characters")
	}
	for _, character := range []byte(value) {
		if character > 0x7f {
			return fmt.Errorf("must contain only ASCII characters")
		}
	}
	return nil
}

func boolCode(value bool) int {
	if value {
		return 1
	}
	return 0
}

func required(lookupEnv LookupEnv, key string) (string, error) {
	value, ok := lookupEnv(key)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func validateHTTPAddr(value string) error {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

func validateOrigin(value string) error {
	origin, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid origin: %w", err)
	}
	if (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" {
		return fmt.Errorf("origin must use http or https with a host")
	}
	if origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("origin must not contain credentials, path, query, or fragment")
	}
	return nil
}
