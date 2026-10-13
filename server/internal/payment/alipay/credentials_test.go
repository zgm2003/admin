package alipay

import (
	"admin/server/internal/module/payment"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
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
