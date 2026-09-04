package i18n_test

import (
	"context"
	"testing"

	"admin/server/internal/shared/i18n"
)

func TestCatalogsHaveTheSameKeysAndParameters(t *testing.T) {
	if err := i18n.ValidateCatalogs(); err != nil {
		t.Fatalf("ValidateCatalogs() error = %v", err)
	}
}

func TestCatalogsTranslateMenuErrorsWithExactParameters(t *testing.T) {
	tests := []struct {
		key    i18n.MessageKey
		params map[string]string
	}{
		{key: i18n.KeyMenuTreeInvalid},
		{key: i18n.KeyMenuNotFound},
		{key: i18n.KeyMenuCodeConflict, params: map[string]string{"code": "reports"}},
		{key: i18n.KeyMenuPathConflict, params: map[string]string{"path": "/reports"}},
		{key: i18n.KeyMenuInvalidParent},
		{key: i18n.KeyMenuCycleDetected},
		{key: i18n.KeyMenuParentDisabled, params: map[string]string{"code": "reports"}},
		{key: i18n.KeyMenuStructureConflict, params: map[string]string{"code": "reports"}},
		{key: i18n.KeyMenuInvalidFields},
	}

	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, test := range tests {
			message, err := i18n.Translate(locale, test.key, test.params)
			if err != nil {
				t.Errorf("Translate(%q, %q) error = %v", locale, test.key, err)
			}
			if message == "" {
				t.Errorf("Translate(%q, %q) returned an empty message", locale, test.key)
			}
		}
	}
}

func TestCatalogsTranslateRoleErrorsWithExactParameters(t *testing.T) {
	tests := []struct {
		key    i18n.MessageKey
		params map[string]string
	}{
		{key: i18n.KeyRoleNotFound},
		{key: i18n.KeyRoleCodeConflict, params: map[string]string{"code": "ai_tester"}},
		{key: i18n.KeyRoleNameConflict, params: map[string]string{"name": "AI Tester"}},
		{key: i18n.KeyRoleSystemProtected, params: map[string]string{"code": "super_admin"}},
		{key: i18n.KeyRoleDefaultProtected, params: map[string]string{"code": "member"}},
		{key: i18n.KeyRoleUsersAttached, params: map[string]string{"code": "member"}},
		{key: i18n.KeyRoleInvalidState},
		{key: i18n.KeyRoleInvalidPermission},
		{key: i18n.KeyRoleSuperAdminAuthorization},
		{key: i18n.KeyRoleDataInvalid},
	}

	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, test := range tests {
			message, err := i18n.Translate(locale, test.key, test.params)
			if err != nil || message == "" {
				t.Errorf("Translate(%q, %q) = %q,%v", locale, test.key, message, err)
			}
		}
	}
}

func TestCatalogsTranslateMailErrors(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		message, err := i18n.Translate(locale, i18n.KeyMailRecipientDenied, nil)
		if err != nil || message == "" {
			t.Errorf("Translate(%q, %q) = %q,%v", locale, i18n.KeyMailRecipientDenied, message, err)
		}
	}
}

func TestCatalogsTranslateUserErrorsWithExactParameters(t *testing.T) {
	keys := []i18n.MessageKey{
		i18n.KeyUserNotFound,
		i18n.KeyUserUsernameConflict,
		i18n.KeyUserPhoneConflict,
		i18n.KeyUserSelfOperation,
		i18n.KeyUserSuperAdminProtected,
		i18n.KeyUserLastSuperAdmin,
		i18n.KeyUserInvalidRoles,
		i18n.KeyUserRoleNotFound,
		i18n.KeyUserDataInvalid,
	}
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, key := range keys {
			message, err := i18n.Translate(locale, key, nil)
			if err != nil || message == "" {
				t.Errorf("Translate(%q, %q) = %q,%v", locale, key, message, err)
			}
		}
	}
}

func TestCatalogsTranslateSessionAdministrationErrors(t *testing.T) {
	keys := []i18n.MessageKey{
		i18n.KeySessionNotFound,
		i18n.KeySessionCurrentProtected,
	}
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, key := range keys {
			message, err := i18n.Translate(locale, key, nil)
			if err != nil || message == "" {
				t.Errorf("Translate(%q, %q) = %q,%v", locale, key, message, err)
			}
		}
	}
}

func TestTranslateUsesTheRequestedLocale(t *testing.T) {
	for _, test := range []struct {
		locale i18n.Locale
		want   string
	}{
		{locale: i18n.ZhCN, want: "请求参数错误"},
		{locale: i18n.EnUS, want: "Invalid request"},
	} {
		got, err := i18n.Translate(test.locale, i18n.KeyInvalidRequest, nil)
		if err != nil || got != test.want {
			t.Fatalf("Translate(%q) = %q,%v, want %q", test.locale, got, err, test.want)
		}
	}
}

func TestTranslateInterpolatesExactParameters(t *testing.T) {
	got, err := i18n.Translate(i18n.EnUS, i18n.KeyPermissionDenied, map[string]string{
		"permission": "account:user:create",
	})
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if got != "Permission denied: account:user:create" {
		t.Fatalf("Translate() = %q", got)
	}
}

func TestTranslateRejectsUnexpectedParameters(t *testing.T) {
	_, missingErr := i18n.Translate(i18n.ZhCN, i18n.KeyPermissionDenied, nil)
	if missingErr == nil {
		t.Fatal("Translate accepted missing interpolation parameters")
	}

	_, extraErr := i18n.Translate(i18n.ZhCN, i18n.KeyInvalidRequest, map[string]string{"field": "email"})
	if extraErr == nil {
		t.Fatal("Translate accepted an unexpected interpolation parameter")
	}
}

func TestTranslateRejectsUnknownLocaleAndKey(t *testing.T) {
	if _, err := i18n.Translate(i18n.Locale("fr-FR"), i18n.KeyInvalidRequest, nil); err == nil {
		t.Fatal("Translate accepted an unsupported locale")
	}
	if _, err := i18n.Translate(i18n.ZhCN, i18n.MessageKey("unknown.key"), nil); err == nil {
		t.Fatal("Translate accepted an unknown message key")
	}
}

func TestParseLocaleAcceptsOnlySupportedExactValues(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		got, err := i18n.ParseLocale(string(locale))
		if err != nil || got != locale {
			t.Fatalf("ParseLocale(%q) = %q,%v", locale, got, err)
		}
	}

	for _, value := range []string{"", "zh", "en", "ZH-CN", "fr-FR"} {
		if _, err := i18n.ParseLocale(value); err == nil {
			t.Fatalf("ParseLocale(%q) accepted an unsupported value", value)
		}
	}
}

func TestLocaleContextDefaultsToChinese(t *testing.T) {
	if got := i18n.LocaleFromContext(context.Background()); got != i18n.ZhCN {
		t.Fatalf("LocaleFromContext(background) = %q, want %q", got, i18n.ZhCN)
	}

	ctx := i18n.WithLocale(context.Background(), i18n.EnUS)
	if got := i18n.LocaleFromContext(ctx); got != i18n.EnUS {
		t.Fatalf("LocaleFromContext(English) = %q, want %q", got, i18n.EnUS)
	}
}
