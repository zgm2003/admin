package config

import (
	"strings"
	"testing"
)

const (
	validPostgresDSN = "host=127.0.0.1 user=postgres password=postgres dbname=admin port=5432 sslmode=disable"
	validRedisURL    = "redis://127.0.0.1:6379/0"
)

func TestLoadAPI(t *testing.T) {
	t.Run("loads every required value", func(t *testing.T) {
		got, err := LoadAPI(lookup(validAPIValues()))
		if err != nil {
			t.Fatalf("LoadAPI() error = %v", err)
		}

		if got.HTTPAddr != ":16301" || got.PostgresDSN != validPostgresDSN || got.RedisURL != validRedisURL || got.CORSOrigin != "http://localhost:16300" || got.AppSecret != strings.Repeat("s", 64) || got.Auth.CookieSecure {
			t.Fatalf("LoadAPI() = %+v", got)
		}
	})

	for _, key := range []string{"HTTP_ADDR", "POSTGRES_DSN", "REDIS_URL", "CORS_ORIGIN", "APP_SECRET", "AUTH_COOKIE_SECURE"} {
		t.Run("rejects missing "+key, func(t *testing.T) {
			values := validAPIValues()
			delete(values, key)

			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, key)
		})

		t.Run("rejects empty "+key, func(t *testing.T) {
			values := validAPIValues()
			values[key] = "  "

			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, key)
		})
	}

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid HTTP address", key: "HTTP_ADDR", value: "16301"},
		{name: "invalid PostgreSQL DSN", key: "POSTGRES_DSN", value: "postgres://%zz"},
		{name: "invalid Redis URL", key: "REDIS_URL", value: "http://127.0.0.1:6379"},
		{name: "wildcard CORS origin", key: "CORS_ORIGIN", value: "*"},
		{name: "CORS origin with path", key: "CORS_ORIGIN", value: "http://localhost:16300/admin"},
	}

	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			values := validAPIValues()
			values[tt.key] = tt.value

			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, tt.key)
		})
	}
}

func TestLoadAPIRejectsInvalidAuthConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "short application secret", key: "APP_SECRET", value: "short"},
		{name: "non-ASCII application secret", key: "APP_SECRET", value: strings.Repeat("界", 64)},
		{name: "placeholder application secret", key: "APP_SECRET", value: "replace_with_at_least_64_random_characters_before_running_api_server"},
		{name: "invalid cookie secure", key: "AUTH_COOKIE_SECURE", value: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := validAPIValues()
			values[tt.key] = tt.value

			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, tt.key)
		})
	}
}

func TestLoadAPIRejectsCORSAndCookieSecurityMismatch(t *testing.T) {
	tests := []struct {
		name         string
		corsOrigin   string
		cookieSecure string
	}{
		{name: "HTTP with secure cookie", corsOrigin: "http://localhost:16300", cookieSecure: "1"},
		{name: "HTTPS with insecure cookie", corsOrigin: "https://admin.example.com", cookieSecure: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := validAPIValues()
			values["CORS_ORIGIN"] = tt.corsOrigin
			values["AUTH_COOKIE_SECURE"] = tt.cookieSecure

			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, "AUTH_COOKIE_SECURE")
		})
	}
}

func TestLoadWorker(t *testing.T) {
	t.Run("loads only PostgreSQL and Redis", func(t *testing.T) {
		got, err := LoadWorker(lookup(map[string]string{
			"POSTGRES_DSN": validPostgresDSN,
			"REDIS_URL":    validRedisURL,
		}))
		if err != nil {
			t.Fatalf("LoadWorker() error = %v", err)
		}

		if got.PostgresDSN != validPostgresDSN || got.RedisURL != validRedisURL {
			t.Fatalf("LoadWorker() = %+v", got)
		}
	})

	for _, key := range []string{"POSTGRES_DSN", "REDIS_URL"} {
		t.Run("rejects missing "+key, func(t *testing.T) {
			values := map[string]string{
				"POSTGRES_DSN": validPostgresDSN,
				"REDIS_URL":    validRedisURL,
			}
			delete(values, key)

			_, err := LoadWorker(lookup(values))
			assertErrorContains(t, err, key)
		})
	}
}

func validAPIValues() map[string]string {
	return map[string]string{
		"HTTP_ADDR":          ":16301",
		"POSTGRES_DSN":       validPostgresDSN,
		"REDIS_URL":          validRedisURL,
		"CORS_ORIGIN":        "http://localhost:16300",
		"APP_SECRET":         strings.Repeat("s", 64),
		"AUTH_COOKIE_SECURE": "0",
	}
}

func lookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}
