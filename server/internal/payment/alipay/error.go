package alipay

import "errors"

// Validation errors identify a fixed field or rule without including credential content.
var (
	ErrAppIDInvalid              = errors.New("APPID invalid")
	ErrSellerIDInvalid           = errors.New("PID invalid")
	ErrEnvironmentInvalid        = errors.New("environment invalid")
	ErrPrivateKeyInvalid         = errors.New("private key invalid")
	ErrAppCertificateInvalid     = errors.New("application certificate invalid")
	ErrAlipayCertificateInvalid  = errors.New("Alipay certificate invalid")
	ErrRootCertificateInvalid    = errors.New("root certificate invalid")
	ErrAppCertificateIdentity    = errors.New("application certificate identity mismatch")
	ErrPrivateKeyMismatch        = errors.New("private key does not match application certificate")
	ErrAppCertificateValidity    = errors.New("application certificate validity invalid")
	ErrAlipayCertificateValidity = errors.New("Alipay certificate validity invalid")
	ErrRootCertificateValidity   = errors.New("root certificate validity invalid")
	ErrClientInitialization      = errors.New("Alipay client initialization failed")
	ErrCertificateInitialization = errors.New("Alipay certificate initialization failed")
)
