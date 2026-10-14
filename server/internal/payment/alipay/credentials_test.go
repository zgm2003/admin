package alipay

import (
	"admin/server/internal/module/payment"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/go-pay/gopay"
)

func certificateFixture(t *testing.T) Credentials {
	t.Helper()
	now := time.Now()
	rootKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	appKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	aliKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Local root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SignatureAlgorithm: x509.SHA256WithRSA}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	leaf := func(serial int64, name string, key *rsa.PrivateKey) string {
		c := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, SignatureAlgorithm: x509.SHA256WithRSA}
		der, err := x509.CreateCertificate(rand.Reader, c, root, &key.PublicKey, rootKey)
		if err != nil {
			t.Fatal(err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	return Credentials{AppID: "20260001", SellerID: "20880001", Environment: payment.EnvironmentSandbox, PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(appKey)})), AppCertPEM: leaf(2, "20260001", appKey), AlipayCertPEM: leaf(3, "Local Alipay", aliKey), RootCertPEM: rootPEM}
}
func TestLocalValidationAndSDKContentMode(t *testing.T) {
	c := certificateFixture(t)
	summaries, err := ValidateCredentials(context.Background(), c, time.Now())
	if err != nil || summaries.App.SerialNumber != "2" {
		t.Fatalf("validate: %+v %v", summaries, err)
	}
	client, err := NewClient(context.Background(), c, time.Now())
	if err != nil || client.IsProd || client.AppCertSN == "" || client.AliPayRootCertSN == "" {
		t.Fatalf("SDK content mode failed: %v", err)
	}
	c.Environment = payment.EnvironmentProduction
	client, err = NewClient(context.Background(), c, time.Now())
	if err != nil || !client.IsProd {
		t.Fatal("wrong SDK environment")
	}
	for name, change := range map[string]func(*Credentials){"malformed": func(c *Credentials) { c.AppCertPEM = "not a cert" }, "extra PEM": func(c *Credentials) { c.AppCertPEM += c.AppCertPEM }, "mismatched key": func(c *Credentials) { c.AppCertPEM = c.AlipayCertPEM }, "unknown environment": func(c *Credentials) { c.Environment = 0 }, "app identity": func(c *Credentials) { c.AppID = "20269999" }, "bad seller": func(c *Credentials) { c.SellerID = "merchant" }, "no roots": func(c *Credentials) { c.RootCertPEM = "" }, "oversize": func(c *Credentials) { c.PrivateKey = string(make([]byte, 16385)) }} {
		t.Run(name, func(t *testing.T) {
			bad := c
			change(&bad)
			if _, err := ValidateCredentials(context.Background(), bad, time.Now()); err == nil {
				t.Fatal("invalid credentials accepted")
			}
		})
	}
	if _, err := ValidateCredentials(context.Background(), c, time.Now().Add(72*time.Hour)); err == nil {
		t.Fatal("expired certificate accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewClient(ctx, c, time.Now()); err == nil {
		t.Fatal("canceled input accepted")
	}
}

func TestSDKPaymentURLsUseConfiguredEnvironment(t *testing.T) {
	credentials := certificateFixture(t)
	for _, environment := range []struct {
		name  string
		value payment.Environment
		host  string
	}{
		{"sandbox", payment.EnvironmentSandbox, "openapi-sandbox.dl.alipaydev.com"},
		{"production", payment.EnvironmentProduction, "openapi.alipay.com"},
	} {
		t.Run(environment.name, func(t *testing.T) {
			input := credentials
			input.Environment = environment.value
			client, err := NewClient(context.Background(), input, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"web", "h5"} {
				t.Run(method, func(t *testing.T) {
					body := gopay.BodyMap{"out_trade_no": "LOCAL_FIXTURE", "total_amount": "1.00", "subject": "Local fixture"}
					// These two SDK methods generate a signed URL locally; they do not send it.
					var raw string
					var err error
					if method == "web" {
						raw, err = client.TradePagePay(context.Background(), body)
					} else {
						raw, err = client.TradeWapPay(context.Background(), body)
					}
					if err != nil {
						t.Fatal(err)
					}
					gateway, err := url.Parse(raw)
					if err != nil {
						t.Fatal(err)
					}
					if gateway.Scheme != "https" || gateway.Host != environment.host || gateway.Path != "/gateway.do" {
						t.Fatalf("wrong payment gateway: %s://%s%s", gateway.Scheme, gateway.Host, gateway.Path)
					}
				})
			}
		})
	}
}

func certificateWithWindow(t *testing.T, raw, privateKey string, start, end time.Time) string {
	t.Helper()
	block, _ := pem.Decode([]byte(raw))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	key, err := parsePrivateKey(privateKey)
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

func TestSandboxCompatibilityOnlyAllowsExpiredAlipayCertificate(t *testing.T) {
	fixture := certificateFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name        string
		environment payment.Environment
		field       string
		start, end  time.Time
		want        error
	}{
		{"sandbox expired Alipay", payment.EnvironmentSandbox, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour), nil},
		{"production expired Alipay", payment.EnvironmentProduction, "alipay", now.Add(-2 * time.Hour), now.Add(-time.Hour), ErrAlipayCertificateValidity},
		{"sandbox Alipay expiry boundary", payment.EnvironmentSandbox, "alipay", now.Add(-time.Hour), now, nil},
		{"production Alipay expiry boundary", payment.EnvironmentProduction, "alipay", now.Add(-time.Hour), now, ErrAlipayCertificateValidity},
		{"sandbox future Alipay", payment.EnvironmentSandbox, "alipay", now.Add(time.Hour), now.Add(2 * time.Hour), ErrAlipayCertificateValidity},
		{"sandbox Alipay activation boundary", payment.EnvironmentSandbox, "alipay", now, now.Add(time.Hour), nil},
		{"sandbox invalid Alipay window", payment.EnvironmentSandbox, "alipay", now.Add(-time.Hour), now.Add(-2 * time.Hour), ErrAlipayCertificateValidity},
		{"sandbox expired application", payment.EnvironmentSandbox, "app", now.Add(-2 * time.Hour), now.Add(-time.Hour), ErrAppCertificateValidity},
		{"sandbox future application", payment.EnvironmentSandbox, "app", now.Add(time.Hour), now.Add(2 * time.Hour), ErrAppCertificateValidity},
		{"sandbox expired root", payment.EnvironmentSandbox, "root", now.Add(-2 * time.Hour), now.Add(-time.Hour), ErrRootCertificateValidity},
		{"sandbox future root", payment.EnvironmentSandbox, "root", now.Add(time.Hour), now.Add(2 * time.Hour), ErrRootCertificateValidity},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := fixture
			input.Environment = test.environment
			field := &input.AlipayCertPEM
			switch test.field {
			case "app":
				field = &input.AppCertPEM
			case "root":
				field = &input.RootCertPEM
			}
			*field = certificateWithWindow(t, *field, input.PrivateKey, test.start, test.end)
			summary, err := ValidateCredentials(context.Background(), input, now)
			if !errors.Is(err, test.want) {
				t.Fatalf("validation: got %v, want %v", err, test.want)
			}
			client, err := NewClient(context.Background(), input, now)
			if !errors.Is(err, test.want) {
				t.Fatalf("SDK initialization: got %v, want %v", err, test.want)
			}
			if test.want == nil {
				if client.IsProd || !summary.Alipay.NotAfter.Equal(test.end) {
					t.Fatal("sandbox environment or original certificate validity was changed")
				}
				if client.AppCertSN == "" || client.AliPayRootCertSN == "" {
					t.Fatal("certificate content mode was not initialized")
				}
			}
		})
	}
}
