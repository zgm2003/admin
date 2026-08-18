package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"admin/server/internal/module/auth"
)

func TestLoadBootstrapSettingsRequiresEveryValue(t *testing.T) {
	valid := map[string]string{
		"POSTGRES_DSN":             "host=127.0.0.1 user=postgres password=postgres dbname=admin port=5432 sslmode=disable",
		"BOOTSTRAP_ADMIN_USERNAME": "admin",
		"BOOTSTRAP_ADMIN_EMAIL":    "admin@example.com",
		"BOOTSTRAP_ADMIN_PASSWORD": "password",
	}
	if _, err := loadBootstrapSettings(mapLookup(valid)); err != nil {
		t.Fatalf("valid settings error = %v", err)
	}
	for key := range valid {
		values := make(map[string]string, len(valid))
		for name, value := range valid {
			values[name] = value
		}
		delete(values, key)
		if _, err := loadBootstrapSettings(mapLookup(values)); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("missing %s error = %v", key, err)
		}
	}
}

func TestExecutePrintsOnlyCreatedIdentity(t *testing.T) {
	creator := &fakeAdminCreator{created: auth.Registered{UserID: 12, Username: "admin", Email: "admin@example.com"}}
	settings := bootstrapSettings{Username: "admin", Email: "admin@example.com", Password: "secret-password"}
	var output bytes.Buffer
	if err := execute(context.Background(), creator, settings, &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "12") || !strings.Contains(text, "admin") {
		t.Fatalf("output = %q", text)
	}
	for _, forbidden := range []string{"secret-password", "admin@example.com", "$2a$", "$2b$"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("output %q contains %q", text, forbidden)
		}
	}
}

type fakeAdminCreator struct {
	created auth.Registered
	err     error
}

func (f *fakeAdminCreator) Create(context.Context, auth.BootstrapAdminInput) (auth.Registered, error) {
	return f.created, f.err
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
