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

func TestConfigStatusActionDoesNotUseCertificateDates(t *testing.T) {
	fixture := validInput(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name        string
		environment payment.Environment
		field       string
		start, end  time.Time
	}{
		{"expired Alipay production", payment.EnvironmentProduction, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour)},
		{"future application production", payment.EnvironmentProduction, "app", now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"expired root production", payment.EnvironmentProduction, "root", now.Add(-2 * time.Hour), now.Add(-time.Hour)},
		{"future Alipay sandbox", payment.EnvironmentSandbox, "alipay", now.Add(time.Hour), now.Add(2 * time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := Current{
				Config:     Model{ID: 1, Provider: payment.ProviderAlipay, Environment: test.environment, IsEnabled: yesno.No},
				Credential: Version{Version: 1, AppCertPEM: fixture.AppCertPEM, AlipayCertPEM: fixture.AlipayCertPEM, RootCertPEM: fixture.RootCertPEM, NotifyURL: "https://pay.example.test/api/admin/v1/payment/callback/alipay/1/1", PayMethods: PayMethods{payment.PayMethodWeb}},
			}
			field := &current.Credential.AlipayCertPEM
			switch test.field {
			case "app":
				field = &current.Credential.AppCertPEM
			case "root":
				field = &current.Credential.RootCertPEM
			}
			*field = configCertificateWithWindow(t, *field, fixture.PrivateKey, test.start, test.end)
			got, err := safeValue(context.Background(), current)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Actions.Status {
				t.Fatal("certificate date disabled the status action")
			}
			if test.field == "alipay" && !got.Certificates.Alipay.NotAfter.Equal(test.end) {
				t.Fatal("management DTO changed the original certificate validity")
			}
		})
	}
}
