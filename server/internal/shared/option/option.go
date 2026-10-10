// Package option defines a transport shape, not a business catalog or registry.
// Each business module owns its values and localized labels in code.
package option

import (
	"admin/server/internal/shared/i18n"
	"context"
)

type Option[T comparable] struct {
	Value T      `json:"value"`
	Label string `json:"label"`
}

func New[T comparable](ctx context.Context, value T, zh, en string) Option[T] {
	label := zh
	if i18n.LocaleFromContext(ctx) == i18n.EnUS {
		label = en
	}
	return Option[T]{Value: value, Label: label}
}
