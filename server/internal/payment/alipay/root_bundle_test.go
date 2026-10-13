package alipay

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	sdk "github.com/go-pay/gopay/alipay"
	"os"
	"testing"
	"time"
)

func TestAlipayOfficialCertificateBundlesUseSDKRSA2Selection(t *testing.T) {
	roots, err := os.ReadFile("testdata/alipayRootCert.crt")
	if err != nil {
		t.Fatal(err)
	}
	ali, err := os.ReadFile("testdata/alipayPublicCert.crt")
	if err != nil {
		t.Fatal(err)
	}
	c := certificateFixture(t)
	summary, err := InspectCertificates(c.AppCertPEM, string(ali), string(roots))
	if err != nil || len(summary.Roots) != 2 {
		t.Fatalf("official mixed root bundle and intermediate: %+v %v", summary, err)
	}
	c.RootCertPEM = string(roots)
	client, err := NewClient(context.Background(), c, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want, err := sdk.GetRootCertSN(roots)
	if err != nil || client.AliPayRootCertSN != want {
		t.Fatalf("SDK root selection: %q %q %v", client.AliPayRootCertSN, want, err)
	}
	// Historical SDK leaves prove format, not current validity.
	c.AlipayCertPEM = string(ali)
	if _, err := ValidateCredentials(context.Background(), c, time.Now()); err == nil {
		t.Fatal("expired public fixture accepted as current")
	}
}

func TestPrivateKeyAcceptsReferencePKCS1AndPKCS8Base64(t *testing.T) {
	c := certificateFixture(t)
	key, err := parsePrivateKey(c.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(key)), base64.StdEncoding.EncodeToString(pkcs8)} {
		c.PrivateKey = raw
		if _, err := NewClient(context.Background(), c, time.Now()); err != nil {
			t.Fatalf("reference base64 key rejected: %v", err)
		}
	}
}
