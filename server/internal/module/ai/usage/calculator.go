package usage

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

const (
	maxDecimalDigits = 24
	maxDecimalPlaces = 8
	partsPerMillion  = int64(1_000_000)
)

type LineInput struct {
	ItemKey       string
	PriceUnits    int64
	Quantity      string
	UnitQuantity  string
	MultiplierPPM int64
}

type CalculatedItem struct {
	ItemKey     string
	AmountUnits int64
}

type CalculatedCharge struct {
	TotalUnits int64
	Items      []CalculatedItem
}

type calculatedLine struct {
	key   string
	floor *big.Int
	rem   *big.Rat
}

func Calculate(lines []LineInput) (CalculatedCharge, error) {
	if len(lines) == 0 {
		return CalculatedCharge{}, errors.New("AI charge requires at least one line")
	}
	seen := make(map[string]struct{}, len(lines))
	calculated := make([]calculatedLine, 0, len(lines))
	total := new(big.Rat)
	for _, line := range lines {
		if line.ItemKey == "" || len(line.ItemKey) > 200 {
			return CalculatedCharge{}, errors.New("AI charge item key must contain 1 to 200 bytes")
		}
		if _, ok := seen[line.ItemKey]; ok {
			return CalculatedCharge{}, fmt.Errorf("duplicate AI charge item key %q", line.ItemKey)
		}
		seen[line.ItemKey] = struct{}{}
		if line.PriceUnits < 0 || line.MultiplierPPM < 1 || line.MultiplierPPM > 100_000_000 {
			return CalculatedCharge{}, errors.New("AI charge rate is invalid")
		}
		quantity, err := parseDecimal(line.Quantity)
		if err != nil {
			return CalculatedCharge{}, fmt.Errorf("AI charge quantity is invalid: %w", err)
		}
		unitQuantity, err := parseDecimal(line.UnitQuantity)
		if err != nil {
			return CalculatedCharge{}, fmt.Errorf("AI charge unit quantity is invalid: %w", err)
		}
		if unitQuantity.Sign() <= 0 {
			return CalculatedCharge{}, errors.New("AI charge unit quantity must be positive")
		}
		amount := new(big.Rat).SetInt64(line.PriceUnits)
		amount.Mul(amount, quantity)
		amount.Quo(amount, unitQuantity)
		amount.Mul(amount, new(big.Rat).SetFrac64(line.MultiplierPPM, partsPerMillion))
		floor, remainder := floorAndRemainder(amount)
		calculated = append(calculated, calculatedLine{key: line.ItemKey, floor: floor, rem: remainder})
		total.Add(total, amount)
	}

	totalFloor, totalRemainder := floorAndRemainder(total)
	if totalRemainder.Sign() > 0 {
		totalFloor.Add(totalFloor, big.NewInt(1))
	}
	if !totalFloor.IsInt64() {
		return CalculatedCharge{}, errors.New("AI charge exceeds int64 range")
	}
	for _, line := range calculated {
		if !line.floor.IsInt64() {
			return CalculatedCharge{}, errors.New("AI charge item exceeds int64 range")
		}
	}
	remaining := new(big.Int).Sub(totalFloor, sumFloors(calculated))
	if remaining.Sign() < 0 || remaining.Cmp(big.NewInt(int64(len(calculated)))) > 0 {
		return CalculatedCharge{}, errors.New("AI charge rounding allocation is invalid")
	}
	sort.SliceStable(calculated, func(i, j int) bool {
		cmp := calculated[i].rem.Cmp(calculated[j].rem)
		if cmp != 0 {
			return cmp > 0
		}
		return calculated[i].key < calculated[j].key
	})
	for index := int64(0); index < remaining.Int64(); index++ {
		calculated[index].floor.Add(calculated[index].floor, big.NewInt(1))
	}
	result := make([]CalculatedItem, 0, len(calculated))
	for _, line := range calculated {
		result = append(result, CalculatedItem{ItemKey: line.key, AmountUnits: line.floor.Int64()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ItemKey < result[j].ItemKey })
	return CalculatedCharge{TotalUnits: totalFloor.Int64(), Items: result}, nil
}

func sumFloors(lines []calculatedLine) *big.Int {
	total := new(big.Int)
	for _, line := range lines {
		total.Add(total, line.floor)
	}
	return total
}

func floorAndRemainder(value *big.Rat) (*big.Int, *big.Rat) {
	floor := new(big.Int).Quo(value.Num(), value.Denom())
	remainder := new(big.Rat).Sub(value, new(big.Rat).SetInt(floor))
	return floor, remainder
}

func parseDecimal(raw string) (*big.Rat, error) {
	if raw == "" || len(raw) > 64 || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		return nil, errors.New("decimal syntax is invalid")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return nil, errors.New("decimal syntax is invalid")
	}
	if len(parts) == 2 && len(parts[1]) > maxDecimalPlaces {
		return nil, errors.New("decimal precision is invalid")
	}
	if len(strings.TrimLeft(parts[0], "0")) > maxDecimalDigits-maxDecimalPlaces {
		return nil, errors.New("decimal integer range is invalid")
	}
	digits := len(parts[0])
	if len(parts) == 2 {
		digits += len(parts[1])
	}
	if digits == 0 || digits > maxDecimalDigits {
		return nil, errors.New("decimal range is invalid")
	}
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, errors.New("decimal syntax is invalid")
			}
		}
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, errors.New("decimal syntax is invalid")
	}
	return value, nil
}
