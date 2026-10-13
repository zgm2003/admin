// Package payment owns stable numeric identities shared by payment resources.
package payment

import (
	"fmt"
	"sort"
)

type Provider int16

const (
	ProviderAlipay Provider = 1
	ProviderWechat Provider = 2
)

func (v Provider) Supported() bool { return v == ProviderAlipay }

type Environment int16

const (
	EnvironmentSandbox    Environment = 1
	EnvironmentProduction Environment = 2
)

func (v Environment) Valid() bool { return v == EnvironmentSandbox || v == EnvironmentProduction }
func ParseEnvironment(raw string) (Environment, error) {
	switch raw {
	case "1":
		return EnvironmentSandbox, nil
	case "2":
		return EnvironmentProduction, nil
	default:
		return 0, fmt.Errorf("payment environment must be 1 or 2")
	}
}

type PayMethod int16

const (
	PayMethodWeb PayMethod = 1
	PayMethodH5  PayMethod = 2
)

func NormalizePayMethods(values []PayMethod) ([]PayMethod, error) {
	if len(values) < 1 || len(values) > 2 {
		return nil, fmt.Errorf("payMethods must be a nonempty set")
	}
	result := append([]PayMethod(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	for i, v := range result {
		if (v != PayMethodWeb && v != PayMethodH5) || (i > 0 && result[i-1] == v) {
			return nil, fmt.Errorf("payMethods contains an unknown or duplicate method")
		}
	}
	return result, nil
}
