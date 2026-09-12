package setting

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"admin/server/internal/shared/yesno"
)

const (
	ValueTypeString = 1
	ValueTypeNumber = 2
	ValueTypeBool   = 3
	ValueTypeJSON   = 4

	AuthCaptchaTTLKey          = "auth.captcha.ttl_minutes"
	AuthCaptchaSlidePaddingKey = "auth.captcha.slide_padding"
)

type Record struct {
	Key         string
	Value       string
	ValueType   int
	Description string
	IsEnabled   yesno.Value
	IsBuiltin   yesno.Value
}

type Reader interface {
	FindByKey(context.Context, string) (Record, error)
}

func (r Record) Number() (int, error) {
	if r.ValueType != ValueTypeNumber {
		return 0, errors.New("setting is not numeric")
	}
	return strconv.Atoi(strings.TrimSpace(r.Value))
}
