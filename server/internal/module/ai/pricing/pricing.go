package pricing

import (
	"errors"
	"fmt"
	"math/big"
)

type ModelKind int16

const (
	KindChat  ModelKind = 1
	KindImage ModelKind = 2
	KindVideo ModelKind = 3
	KindAudio ModelKind = 4
)

func (kind ModelKind) Valid() bool { return kind >= KindChat && kind <= KindAudio }

type PricingMode int16

const (
	ModeOfficialToken PricingMode = 1
	ModeProviderUnit  PricingMode = 2
)

func (mode PricingMode) Valid() bool { return mode == ModeOfficialToken || mode == ModeProviderUnit }

type PriceUnit int16

const (
	UnitMillionTokens     PriceUnit = 1
	UnitImage             PriceUnit = 2
	UnitSecond            PriceUnit = 3
	UnitMinute            PriceUnit = 4
	UnitMillionCharacters PriceUnit = 5
	UnitRequest           PriceUnit = 6
)

func (unit PriceUnit) Valid() bool { return unit >= UnitMillionTokens && unit <= UnitRequest }

var (
	ErrInvalidUsage = errors.New("invalid usage")
	ErrMissingPrice = errors.New("price is not configured")
	ErrOverflow     = errors.New("amount overflow")
)

// TokenPrices 的单位是 CNY micros / 1M tokens。缓存价格用指针区分“免费”和“未配置”。
type TokenPrices struct {
	Input      int64
	Output     int64
	CacheRead  *int64
	CacheWrite *int64
}

type TokenUsage struct {
	UncachedInput int64
	CacheRead     int64
	CacheWrite    int64
	Output        int64
}

// CalculateToken 先按官方 token 单价计算基础费用，再乘供应商 token 倍率。
func CalculateToken(usage TokenUsage, prices TokenPrices, ratePPM int64) (int64, error) {
	if usage.UncachedInput < 0 || usage.CacheRead < 0 || usage.CacheWrite < 0 || usage.Output < 0 {
		return 0, ErrInvalidUsage
	}
	if prices.Input < 0 || prices.Output < 0 || ratePPM < 0 || ratePPM > 100000000 {
		return 0, ErrInvalidUsage
	}
	if (prices.CacheRead != nil && *prices.CacheRead < 0) ||
		(prices.CacheWrite != nil && *prices.CacheWrite < 0) {
		return 0, ErrInvalidUsage
	}
	if usage.CacheRead > 0 && prices.CacheRead == nil {
		return 0, fmt.Errorf("%w: cache read", ErrMissingPrice)
	}
	if usage.CacheWrite > 0 && prices.CacheWrite == nil {
		return 0, fmt.Errorf("%w: cache write", ErrMissingPrice)
	}

	numerator := new(big.Int)
	addProduct := func(tokens, price int64) {
		if tokens == 0 || price == 0 {
			return
		}
		product := new(big.Int).Mul(big.NewInt(tokens), big.NewInt(price))
		numerator.Add(numerator, product)
	}
	addProduct(usage.UncachedInput, prices.Input)
	addProduct(usage.Output, prices.Output)
	if prices.CacheRead != nil {
		addProduct(usage.CacheRead, *prices.CacheRead)
	}
	if prices.CacheWrite != nil {
		addProduct(usage.CacheWrite, *prices.CacheWrite)
	}

	// 官方价格是每百万 token，先得到 micros；再应用百万分比倍率。
	if !numerator.IsInt64() {
		return 0, ErrOverflow
	}
	base := roundDivide(numerator.Int64(), 1000000)
	if base < 0 {
		return 0, ErrOverflow
	}
	actual := applyPPM(base, ratePPM)
	if actual < 0 {
		return 0, ErrOverflow
	}
	return actual, nil
}

// CalculateUnit 计算媒体模型的实际费用；单位价格已经是 CNY micros / priceUnit。
func CalculateUnit(unit PriceUnit, quantity, unitPriceMicros int64) (int64, error) {
	if !unit.Valid() || quantity < 0 || unitPriceMicros < 0 {
		return 0, ErrInvalidUsage
	}
	if quantity == 0 || unitPriceMicros == 0 {
		return 0, nil
	}
	numerator := new(big.Int).Mul(big.NewInt(quantity), big.NewInt(unitPriceMicros))
	divisor := int64(1)
	switch unit {
	case UnitMillionTokens, UnitMillionCharacters:
		divisor = 1000000
	}
	if !numerator.IsInt64() {
		return 0, ErrOverflow
	}
	return roundDivide(numerator.Int64(), divisor), nil
}

func applyPPM(value, ppm int64) int64 {
	if value == 0 || ppm == 0 {
		return 0
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(ppm))
	if !product.IsInt64() {
		return -1
	}
	return roundDivide(product.Int64(), 1000000)
}

func roundDivide(value, divisor int64) int64 {
	if divisor <= 0 {
		return -1
	}
	if value < 0 {
		return -1
	}
	return (value + divisor/2) / divisor
}
