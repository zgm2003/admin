package officialmodel

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	microsPerUnit int64 = 1_000_000
	maxMicros     int64 = 999_999_999_999_999_999
)

// parseMicros accepts a non-negative decimal with at most six fractional
// digits. Prices are stored as integer micros so syncing/copying never
// introduces floating-point drift.
func parseMicros(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("price is required")
	}
	if value[0] == '+' {
		value = value[1:]
	}
	if value == "" || value[0] == '-' {
		return 0, fmt.Errorf("price must be non-negative")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("price is invalid")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > maxMicros/microsPerUnit {
		return 0, fmt.Errorf("price is too large")
	}
	fraction := int64(0)
	if len(parts) == 2 {
		rawFraction := parts[1]
		if rawFraction == "" || len(rawFraction) > 6 {
			return 0, fmt.Errorf("price supports at most six decimal places")
		}
		for _, character := range rawFraction {
			if character < '0' || character > '9' {
				return 0, fmt.Errorf("price is invalid")
			}
		}
		padded := rawFraction + strings.Repeat("0", 6-len(rawFraction))
		fraction, err = strconv.ParseInt(padded, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("price is invalid")
		}
	}
	if whole > (maxMicros-fraction)/microsPerUnit {
		return 0, fmt.Errorf("price is too large")
	}
	return whole*microsPerUnit + fraction, nil
}

func formatOptionalMicros(value *int64) *string {
	if value == nil {
		return nil
	}
	formatted := formatMicros(*value)
	return &formatted
}

func formatMicros(value int64) string {
	if value <= 0 {
		return "0.00"
	}
	whole := value / microsPerUnit
	fraction := value % microsPerUnit
	text := strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%06d", fraction)
	for strings.HasSuffix(text, "0") && len(text)-strings.IndexByte(text, '.')-1 > 2 {
		text = strings.TrimSuffix(text, "0")
	}
	return text
}
