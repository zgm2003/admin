// Package option defines the transport shape for code-owned business options.
// Values stay with their module; labels are projected from the shared i18n catalog.
package option

import (
	"admin/server/internal/shared/i18n"
	"context"
)

type Option[T comparable] struct {
	Value T      `json:"value"`
	Label string `json:"label"`
}

type LocalizedEntry[T comparable] struct {
	Value  T
	Key    i18n.MessageKey
	Params map[string]string
}

func New[T comparable](ctx context.Context, value T, zh, en string) Option[T] {
	label := zh
	if i18n.LocaleFromContext(ctx) == i18n.EnUS {
		label = en
	}
	return Option[T]{Value: value, Label: label}
}

// NewLocalized projects a code-owned catalog key into the requested locale.
// Unlike New, it returns catalog errors instead of silently returning a fallback label.
func NewLocalized[T comparable](ctx context.Context, value T, key i18n.MessageKey, params map[string]string) (Option[T], error) {
	label, err := i18n.Translate(i18n.LocaleFromContext(ctx), key, params)
	if err != nil {
		return Option[T]{}, err
	}
	return Option[T]{Value: value, Label: label}, nil
}

func ManyLocalized[T comparable](ctx context.Context, entries []LocalizedEntry[T]) ([]Option[T], error) {
	options := make([]Option[T], 0, len(entries))
	for _, entry := range entries {
		localized, err := NewLocalized(ctx, entry.Value, entry.Key, entry.Params)
		if err != nil {
			return nil, err
		}
		options = append(options, localized)
	}
	return options, nil
}
