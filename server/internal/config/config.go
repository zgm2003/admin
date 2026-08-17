package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"
)

type API struct {
	HTTPAddr    string
	PostgresDSN string
	RedisURL    string
	CORSOrigin  string
}

type Worker struct {
	PostgresDSN string
	RedisURL    string
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

	return API{
		HTTPAddr:    httpAddr,
		PostgresDSN: postgresDSN,
		RedisURL:    redisURL,
		CORSOrigin:  corsOrigin,
	}, nil
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

	return Worker{PostgresDSN: postgresDSN, RedisURL: redisURL}, nil
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
