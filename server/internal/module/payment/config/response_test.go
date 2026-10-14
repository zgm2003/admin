package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/yesno"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func configCertificateWithWindow(t *testing.T, raw, privateKey string, start, end time.Time) string {
	t.Helper()
	block, _ := pem.Decode([]byte(raw))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	block, _ = pem.Decode([]byte(privateKey))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	cert.NotBefore, cert.NotAfter = start, end
	parent := *cert
	parent.PublicKey = &key.PublicKey
	der, err := x509.CreateCertificate(rand.Reader, cert, &parent, cert.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestConfigStatusActionUsesEnvironmentCertificateValidity(t *testing.T) {
	fixture := validInput(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name        string
		environment payment.Environment
		field       string
		start, end  time.Time
		enabled     yesno.Value
		want        bool
	}{
		{"sandbox expired Alipay can enable", payment.EnvironmentSandbox, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour), yesno.No, true},
		{"production expired Alipay cannot enable", payment.EnvironmentProduction, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour), yesno.No, false},
		{"production expired enabled can disable", payment.EnvironmentProduction, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour), yesno.Yes, true},
		{"sandbox Alipay expiry boundary can enable", payment.EnvironmentSandbox, "alipay", now.Add(-time.Hour), now, yesno.No, true},
		{"production Alipay expiry boundary cannot enable", payment.EnvironmentProduction, "alipay", now.Add(-time.Hour), now, yesno.No, false},
		{"sandbox future Alipay cannot enable", payment.EnvironmentSandbox, "alipay", now.Add(time.Hour), now.Add(2 * time.Hour), yesno.No, false},
		{"sandbox future application cannot enable", payment.EnvironmentSandbox, "app", now.Add(time.Hour), now.Add(2 * time.Hour), yesno.No, false},
		{"sandbox future root cannot enable", payment.EnvironmentSandbox, "root", now.Add(time.Hour), now.Add(2 * time.Hour), yesno.No, false},
		{"sandbox expired application cannot enable", payment.EnvironmentSandbox, "app", now.Add(-2 * time.Hour), now.Add(-time.Hour), yesno.No, false},
		{"sandbox expired root cannot enable", payment.EnvironmentSandbox, "root", now.Add(-2 * time.Hour), now.Add(-time.Hour), yesno.No, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := Current{
				Config:     Model{ID: 1, Provider: payment.ProviderAlipay, Environment: test.environment, IsEnabled: test.enabled},
				Credential: Version{Version: 1, AppCertPEM: fixture.AppCertPEM, AlipayCertPEM: fixture.AlipayCertPEM, RootCertPEM: fixture.RootCertPEM, NotifyURL: "https://pay.example.test/api/v1/payment/callback/alipay/1/1", PayMethods: PayMethods{payment.PayMethodWeb}},
			}
			field := &current.Credential.AlipayCertPEM
			switch test.field {
			case "app":
				field = &current.Credential.AppCertPEM
			case "root":
				field = &current.Credential.RootCertPEM
			}
			*field = configCertificateWithWindow(t, *field, fixture.PrivateKey, test.start, test.end)
			got, err := safeValue(context.Background(), current, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.Actions.Status != test.want {
				t.Fatalf("status action: got %v, want %v", got.Actions.Status, test.want)
			}
			if test.field == "alipay" && !got.Certificates.Alipay.NotAfter.Equal(test.end) {
				t.Fatal("management DTO changed the original certificate validity")
			}
		})
	}
}
