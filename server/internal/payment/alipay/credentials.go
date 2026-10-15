package alipay

import (
	"admin/server/internal/module/payment"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	sdk "github.com/go-pay/gopay/alipay"
	"strings"
	"time"
)

const (
	MaxCertificateBytes = 64 * 1024
	MaxPrivateKeyBytes  = 16 * 1024
)

type Credentials struct {
	AppID, SellerID, NotifyURL                         string
	Environment                                        payment.Environment
	PrivateKey, AppCertPEM, AlipayCertPEM, RootCertPEM string
}
type CertificateInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	SerialNumber string    `json:"serialNumber"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
}
type CertificateSummary struct {
	App    CertificateInfo   `json:"app"`
	Alipay CertificateInfo   `json:"alipay"`
	Roots  []CertificateInfo `json:"roots"`
}

func certificateInfo(c *x509.Certificate) CertificateInfo {
	return CertificateInfo{c.Subject.String(), c.Issuer.String(), c.SerialNumber.String(), c.NotBefore.UTC(), c.NotAfter.UTC()}
}

// Leaf files may include intermediates. Root selection follows GoPay RSA2:
// unsupported algorithms in the official mixed bundle are not a whole-file error.
func parseCertificates(raw string, leafFile bool) ([]*x509.Certificate, error) {
	if len(raw) == 0 || len(raw) > MaxCertificateBytes {
		return nil, fmt.Errorf("certificate size invalid")
	}
	rest := []byte(strings.TrimSpace(raw))
	result := []*x509.Certificate{}
	seen := map[string]bool{}
	count := 0
	for len(rest) > 0 {
		if !strings.HasPrefix(string(rest), "-----BEGIN CERTIFICATE-----") {
			return nil, fmt.Errorf("certificate PEM invalid")
		}
		block, next := pem.Decode(rest)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, fmt.Errorf("certificate PEM invalid")
		}
		c, err := x509.ParseCertificate(block.Bytes)
		count++
		if count > 16 || seen[string(block.Bytes)] {
			return nil, fmt.Errorf("certificate count or duplicate invalid")
		}
		seen[string(block.Bytes)] = true
		if err != nil && leafFile {
			return nil, fmt.Errorf("certificate DER invalid")
		}
		if err == nil {
			// Delegate the root algorithm/SN decision to the SDK, not a second list.
			if leafFile {
				result = append(result, c)
			} else if _, err := sdk.GetRootCertSN(pem.EncodeToMemory(block)); err == nil {
				result = append(result, c)
			}
		}
		rest = []byte(strings.TrimSpace(string(next)))
		if len(result) > 16 {
			return nil, fmt.Errorf("too many root certificates")
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("certificate count invalid")
	}
	return result, nil
}
func InspectCertificates(app, ali, root string) (CertificateSummary, error) {
	a, err := parseCertificates(app, true)
	if err != nil {
		return CertificateSummary{}, err
	}
	b, err := parseCertificates(ali, true)
	if err != nil {
		return CertificateSummary{}, err
	}
	rs, err := parseCertificates(root, false)
	if err != nil {
		return CertificateSummary{}, err
	}
	out := CertificateSummary{App: certificateInfo(a[0]), Alipay: certificateInfo(b[0]), Roots: make([]CertificateInfo, 0, len(rs))}
	for _, r := range rs {
		out.Roots = append(out.Roots, certificateInfo(r))
	}
	return out, nil
}

func numericIdentity(raw string) bool {
	if len(raw) == 0 || len(raw) > 64 {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	if len(raw) == 0 || len(raw) > MaxPrivateKeyBytes {
		return nil, fmt.Errorf("private key size invalid")
	}
	raw = strings.TrimSpace(raw)
	block, rest := pem.Decode([]byte(raw))
	if block == nil && !strings.Contains(raw, "-----") {
		der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(raw), ""))
		if err != nil {
			return nil, fmt.Errorf("private key base64 invalid")
		}
		block = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
		rest = nil
		if _, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			block.Type = "RSA PRIVATE KEY"
		}
	}
	if block == nil || len(strings.TrimSpace(string(rest))) > 0 || len(block.Headers) > 0 {
		return nil, fmt.Errorf("private key PEM invalid")
	}
	var key *rsa.PrivateKey
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				err = fmt.Errorf("RSA private key required")
			}
		}
	default:
		err = fmt.Errorf("RSA private key required")
	}
	if err != nil || key == nil {
		return nil, fmt.Errorf("private key invalid")
	}
	if key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, fmt.Errorf("RSA private key strength invalid")
	}
	return key, nil
}

// ValidateCredentials performs local checks only: no gateway I/O or merchant ownership proof.
func ValidateCredentials(ctx context.Context, c Credentials) (CertificateSummary, error) {
	if err := ctx.Err(); err != nil {
		return CertificateSummary{}, err
	}
	if !numericIdentity(c.AppID) {
		return CertificateSummary{}, ErrAppIDInvalid
	}
	if !numericIdentity(c.SellerID) {
		return CertificateSummary{}, ErrSellerIDInvalid
	}
	if !c.Environment.Valid() {
		return CertificateSummary{}, ErrEnvironmentInvalid
	}
	a, err := parseCertificates(c.AppCertPEM, true)
	if err != nil {
		return CertificateSummary{}, fmt.Errorf("%w: %v", ErrAppCertificateInvalid, err)
	}
	b, err := parseCertificates(c.AlipayCertPEM, true)
	if err != nil {
		return CertificateSummary{}, fmt.Errorf("%w: %v", ErrAlipayCertificateInvalid, err)
	}
	roots, err := parseCertificates(c.RootCertPEM, false)
	if err != nil {
		return CertificateSummary{}, fmt.Errorf("%w: %v", ErrRootCertificateInvalid, err)
	}
	key, err := parsePrivateKey(c.PrivateKey)
	if err != nil {
		return CertificateSummary{}, fmt.Errorf("%w: %v", ErrPrivateKeyInvalid, err)
	}
	cn := a[0].Subject.CommonName
	if cn != c.AppID && cn != c.SellerID+"-"+c.AppID {
		return CertificateSummary{}, ErrAppCertificateIdentity
	}
	public, ok := a[0].PublicKey.(*rsa.PublicKey)
	if !ok || public.N.Cmp(key.N) != 0 || public.E != key.E {
		return CertificateSummary{}, ErrPrivateKeyMismatch
	}
	// This is the reference project's local configuration check, not PKI trust
	// authentication or merchant ownership proof. The SDK uses complete bundles
	// to derive SNs; historical root self-signatures are not re-authenticated.
	summary := CertificateSummary{App: certificateInfo(a[0]), Alipay: certificateInfo(b[0]), Roots: make([]CertificateInfo, 0, len(roots))}
	for _, r := range roots {
		if !r.IsCA {
			return CertificateSummary{}, ErrRootCertificateInvalid
		}
		summary.Roots = append(summary.Roots, certificateInfo(r))
	}
	for index, leaf := range []*x509.Certificate{a[0], b[0]} {
		pk, ok := leaf.PublicKey.(*rsa.PublicKey)
		if !ok || pk.N.BitLen() < 2048 || leaf.IsCA || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			if index == 0 {
				return CertificateSummary{}, ErrAppCertificateInvalid
			}
			return CertificateSummary{}, ErrAlipayCertificateInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return CertificateSummary{}, err
	}
	return summary, nil
}
