package config

import (
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
	"net/http"
)

func invalid(err error) error {
	var public *apperror.Error
	if errors.As(err, &public) {
		return err
	}
	key := i18n.KeyInvalidRequest
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return dependency(err)
	case errors.Is(err, errNotifyURLInvalid):
		key = i18n.KeyPaymentConfigNotifyURLInvalid
	case errors.Is(err, alipay.ErrAppIDInvalid):
		key = i18n.KeyPaymentConfigAppIDInvalid
	case errors.Is(err, alipay.ErrSellerIDInvalid):
		key = i18n.KeyPaymentConfigSellerIDInvalid
	case errors.Is(err, alipay.ErrEnvironmentInvalid):
		key = i18n.KeyPaymentConfigEnvironmentInvalid
	case errors.Is(err, alipay.ErrPrivateKeyInvalid):
		key = i18n.KeyPaymentConfigPrivateKeyInvalid
	case errors.Is(err, alipay.ErrAppCertificateInvalid):
		key = i18n.KeyPaymentConfigAppCertInvalid
	case errors.Is(err, alipay.ErrAlipayCertificateInvalid):
		key = i18n.KeyPaymentConfigAlipayCertInvalid
	case errors.Is(err, alipay.ErrRootCertificateInvalid):
		key = i18n.KeyPaymentConfigRootCertInvalid
	case errors.Is(err, alipay.ErrAppCertificateIdentity):
		key = i18n.KeyPaymentConfigAppCertIdentityMismatch
	case errors.Is(err, alipay.ErrPrivateKeyMismatch):
		key = i18n.KeyPaymentConfigPrivateKeyMismatch
	case errors.Is(err, alipay.ErrAppCertificateValidity):
		key = i18n.KeyPaymentConfigAppCertValidityInvalid
	case errors.Is(err, alipay.ErrAlipayCertificateValidity):
		key = i18n.KeyPaymentConfigAlipayCertValidityInvalid
	case errors.Is(err, alipay.ErrRootCertificateValidity):
		key = i18n.KeyPaymentConfigRootCertValidityInvalid
	case errors.Is(err, alipay.ErrClientInitialization), errors.Is(err, alipay.ErrCertificateInitialization):
		key = i18n.KeyPaymentConfigCredentialInitializationFailed
	}
	return &apperror.Error{HTTPStatus: http.StatusBadRequest, Code: apperror.CodeInvalidRequest, MessageKey: key, Cause: err}
}
