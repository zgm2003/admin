package configcachestatus

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
	"fmt"
	"strconv"
)

type adminOptions struct {
	PublishStates []option.Option[string] `json:"publishStates"`
}

func formOptions(ctx context.Context) (adminOptions, error) {
	states, err := option.ManyLocalized(ctx, []option.LocalizedEntry[string]{
		{Value: PublishStateReady, Key: i18n.KeyCachePublishReady}, {Value: PublishStatePending, Key: i18n.KeyCachePublishPending}, {Value: PublishStateRetrying, Key: i18n.KeyCachePublishRetrying},
	})
	if err != nil {
		return adminOptions{}, err
	}
	return adminOptions{PublishStates: states}, nil
}

func namespaceLabel(ctx context.Context, value string) (string, error) {
	key := i18n.MessageKey("")
	switch value {
	case "system.setting":
		key = i18n.KeyCacheNamespaceSystem
	case "message.mail":
		key = i18n.KeyCacheNamespaceMail
	case "message.sms":
		key = i18n.KeyCacheNamespaceSMS
	case "storage.cosconfig":
		key = i18n.KeyCacheNamespaceCOS
	case "payment.config":
		key = i18n.KeyCacheNamespacePayment
	default:
		return value, nil
	}
	return i18n.Translate(i18n.LocaleFromContext(ctx), key, nil)
}

func scopeLabel(ctx context.Context, namespace, scope string) (string, error) {
	if scope == "global" {
		return i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheScopeGlobal, nil)
	}
	if namespace == "storage.cosconfig" {
		if id, err := strconv.ParseInt(scope, 10, 64); err == nil && id > 0 {
			return i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheScopeCOS, map[string]string{"scope": scope})
		}
	}
	label, err := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheScopeOther, map[string]string{"scope": scope})
	return fmt.Sprintf("%s", label), err
}

func statusDisplay(ctx context.Context, value string) (label, tone, hint string, err error) {
	tone = "info"
	switch value {
	case StatusReady:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusReady, nil)
		tone = "success"
	case StatusPending:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusPending, nil)
		tone = "warning"
	case StatusRetrying:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusRetrying, nil)
		tone = "danger"
	case StatusInvalidating:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusInvalidating, nil)
		tone = "warning"
	case StatusMissing:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusMissing, nil)
		tone = "warning"
		hint, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusMissingHint, nil)
	case StatusCorrupt:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusCorrupt, nil)
		tone = "danger"
	case StatusUnavailable:
		label, err = i18n.Translate(i18n.LocaleFromContext(ctx), i18n.KeyCacheStatusUnavailable, nil)
		tone = "danger"
	default:
		label = value
	}
	return
}
