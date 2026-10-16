package config

import (
	"admin/server/internal/module/payment"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAPIParsesTypedPaymentEnvironmentAndOptionalOrigins(t *testing.T) {
	for raw, want := range map[string]payment.Environment{"1": payment.EnvironmentSandbox, "2": payment.EnvironmentProduction} {
		values := validAPIValues()
		values["PAYMENT_ENVIRONMENT"] = raw
		for _, empty := range []string{"", " "} {
			values["PAYMENT_RETURN_ORIGINS"] = empty
			got, err := LoadAPI(lookup(values))
			if err != nil || got.PaymentEnvironment != want || len(got.PaymentReturnOrigins) != 0 {
				t.Fatalf("environment=%v origins=%v err=%v", got.PaymentEnvironment, got.PaymentReturnOrigins, err)
			}
		}
		values["PAYMENT_RETURN_ORIGINS"] = "https://app.example.test, http://localhost:16300,https://[::1]:443"
		got, err := LoadAPI(lookup(values))
		if err != nil || got.PaymentEnvironment != want || !reflect.DeepEqual(got.PaymentReturnOrigins, []string{"https://app.example.test", "http://localhost:16300", "https://[::1]:443"}) {
			t.Fatalf("environment=%v origins=%v err=%v", got.PaymentEnvironment, got.PaymentReturnOrigins, err)
		}
	}
}

func TestLoadAPIRequiresExplicitPaymentEnvironment(t *testing.T) {
	values := validAPIValues()
	delete(values, "PAYMENT_ENVIRONMENT")
	_, err := LoadAPI(lookup(values))
	assertErrorContains(t, err, "PAYMENT_ENVIRONMENT")
	for _, raw := range []string{"", " ", "0", "3", "01", "+1", "-1", "1.0", "sandbox", "production", " 1", "2 ", "１", "32768"} {
		t.Run(raw, func(t *testing.T) {
			values := validAPIValues()
			values["PAYMENT_ENVIRONMENT"] = raw
			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, "PAYMENT_ENVIRONMENT")
		})
	}
}

func TestLoadAPIRejectsInvalidPaymentReturnOrigins(t *testing.T) {
	for _, raw := range []string{
		"*", "https://*.example.test", "//app.example.test", "ftp://app.example.test",
		"https://user:password@app.example.test", "https://app.example.test/paid", "https://app.example.test/",
		"https://app.example.test?token=TOKEN", "https://app.example.test?", "https://app.example.test#", "https://app.example.test#paid",
		"https://", "https://:443", "https://app.example.test:", "https://app.example.test:0", "https://app.example.test:65536",
		"https://app.example.test:PORT", "https://app.example.test%2f.evil.test", "https://app.example.test\\evil",
		",https://app.example.test", "https://app.example.test,", "https://app.example.test,,http://localhost:16300",
		"https://app.example.test http://localhost:16300",
		"https://app.example.test,https://app.example.test",
	} {
		t.Run(raw, func(t *testing.T) {
			values := validAPIValues()
			values["PAYMENT_RETURN_ORIGINS"] = raw
			_, err := LoadAPI(lookup(values))
			assertErrorContains(t, err, "PAYMENT_RETURN_ORIGINS")
			if err != nil && (strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "TOKEN")) {
				t.Fatal("configuration error exposed a supplied credential or query")
			}
		})
	}
}
