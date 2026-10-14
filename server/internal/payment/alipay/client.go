package alipay

import (
	"admin/server/internal/module/payment"
	"context"
	"crypto/x509"
	"encoding/base64"
	sdk "github.com/go-pay/gopay/alipay"
	"time"
)

// NewClient initializes certificate content mode without filesystem paths or I/O.
func NewClient(ctx context.Context, c Credentials, now time.Time) (*sdk.Client, error) {
	if _, err := ValidateCredentials(ctx, c, now); err != nil {
		return nil, err
	}
	key, err := parsePrivateKey(c.PrivateKey)
	if err != nil {
		return nil, err
	}
	client, err := sdk.NewClient(c.AppID, base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(key)), c.Environment == payment.EnvironmentProduction)
	if err != nil {
		return nil, ErrClientInitialization
	}
	client.SetSignType(sdk.RSA2).SetNotifyUrl(c.NotifyURL)
	if err = client.SetCertSnByContent([]byte(c.AppCertPEM), []byte(c.RootCertPEM), []byte(c.AlipayCertPEM)); err != nil {
		return nil, ErrCertificateInitialization
	}
	client.AutoVerifySign([]byte(c.AlipayCertPEM))
	return client, nil
}
