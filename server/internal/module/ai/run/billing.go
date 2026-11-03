package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/usage"
)

type ChargeLine struct {
	RequestID      int64
	Rate           RuntimeRate
	RawUsageDigest string
	Calculation    usage.LineInput
}

func textUsageLines(s RuntimeSnapshot, requestID int64, u pr.TokenUsage) ([]ChargeLine, error) {
	if !u.Known || requestID < 1 || u.InputTokens < 0 || u.CachedInputTokens < 0 || u.CachedInputTokens > u.InputTokens || u.OutputTokens < 0 {
		return nil, usage.ErrUsageUnknown
	}
	var config RuntimeConfig
	if json.Unmarshal(s.Config, &config) != nil {
		return nil, ErrInvalidSnapshot
	}
	rates := map[string]RuntimeRate{}
	for _, r := range config.Rates {
		if r.Operation != "text.generate" {
			continue
		}
		selector, e := normalizeObject(r.Selector)
		if e != nil || string(selector) != "{}" || r.ID < 1 || r.Unit != "token" || r.BillingUnitPriceUnits < 0 {
			return nil, ErrInvalidSnapshot
		}
		if _, ok := rates[r.UsageKey]; ok {
			return nil, ErrInvalidSnapshot
		}
		if r.UsageKey != "input" && r.UsageKey != "cacheInput" && r.UsageKey != "output" {
			return nil, ErrInvalidSnapshot
		}
		rates[r.UsageKey] = r
	}
	if _, ok := rates["input"]; !ok {
		return nil, ErrInvalidSnapshot
	}
	if _, ok := rates["output"]; !ok {
		return nil, ErrInvalidSnapshot
	}
	if _, ok := rates["cacheInput"]; !ok && u.CachedInputTokens != 0 {
		return nil, errors.New("AI cache input price missing")
	}
	digest, e := usage.UsageDigest(u)
	if e != nil {
		return nil, e
	}
	out := make([]ChargeLine, 0, 3)
	for _, key := range []string{"input", "cacheInput", "output"} {
		r, ok := rates[key]
		if !ok {
			continue
		}
		q := u.OutputTokens
		if key == "input" {
			q = u.InputTokens - u.CachedInputTokens
		}
		if key == "cacheInput" {
			q = u.CachedInputTokens
		}
		out = append(out, ChargeLine{RequestID: requestID, Rate: r, RawUsageDigest: digest, Calculation: usage.LineInput{ItemKey: fmt.Sprintf("%d:%s", requestID, key), PriceUnits: r.BillingUnitPriceUnits, Quantity: strconv.FormatInt(q, 10), UnitQuantity: r.UnitQuantity, MultiplierPPM: s.MultiplierPPM}})
	}
	return out, nil
}

func mediaUsageLines(s RuntimeSnapshot, requestID int64, u pr.MediaUsage, operation string, input RunInput) ([]ChargeLine, error) {
	if !u.Known || requestID < 1 {
		return nil, usage.ErrUsageUnknown
	}
	var config RuntimeConfig
	if json.Unmarshal(s.Config, &config) != nil {
		return nil, ErrInvalidSnapshot
	}
	digest, err := usage.UsageDigest(u)
	if err != nil {
		return nil, err
	}
	lines := make([]ChargeLine, 0)
	seen := make(map[string]bool)
	image := operation == "image.generate" || operation == "image.edit"
	for _, rate := range config.Rates {
		if rate.Operation != operation {
			continue
		}
		if rate.ID < 1 || rate.BillingUnitPriceUnits < 0 {
			return nil, ErrInvalidSnapshot
		}
		matched, err := mediaRateMatches(rate, input)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		if image {
			if rate.Unit != "image" || (rate.UsageKey != "output" && rate.UsageKey != "image") {
				return nil, ErrInvalidSnapshot
			}
		}
		quantity, ok := mediaQuantity(u, rate.UsageKey, rate.Unit)
		if !ok || strings.TrimSpace(quantity) == "" {
			return nil, usage.ErrUsageUnknown
		}
		if seen[rate.UsageKey] {
			return nil, ErrInvalidSnapshot
		}
		seen[rate.UsageKey] = true
		lines = append(lines, ChargeLine{RequestID: requestID, Rate: rate, RawUsageDigest: digest, Calculation: usage.LineInput{ItemKey: fmt.Sprintf("%d:%s", requestID, rate.UsageKey), PriceUnits: rate.BillingUnitPriceUnits, Quantity: quantity, UnitQuantity: rate.UnitQuantity, MultiplierPPM: s.MultiplierPPM}})
	}
	if len(lines) == 0 || (image && len(lines) != 1) {
		return nil, usage.ErrUsageUnknown
	}
	check := make([]usage.LineInput, 0, len(lines))
	for _, line := range lines {
		check = append(check, line.Calculation)
	}
	if _, err := usage.Calculate(check); err != nil {
		return nil, err
	}
	return lines, nil
}

// Supplier selectors include accounting basis metadata as well as customer
// choices. Keep those identities while matching only supported combinations.
func mediaRateMatches(rate RuntimeRate, input RunInput) (bool, error) {
	var selector map[string]string
	if json.Unmarshal(rate.Selector, &selector) != nil || selector == nil {
		return false, ErrInvalidSnapshot
	}
	if rate.Operation == "image.generate" || rate.Operation == "image.edit" {
		if billing, ok := selector["billing"]; ok {
			if billing != "image" {
				return false, ErrInvalidSnapshot
			}
			delete(selector, "billing")
		}
		if len(selector) != 2 || selector["quality"] == "" || selector["size"] == "" {
			return false, ErrInvalidSnapshot
		}
		return selector["quality"] == input.Quality && selector["size"] == input.Size, nil
	}
	if len(selector) == 0 {
		return true, nil
	}
	if len(selector) != 1 {
		return false, ErrInvalidSnapshot
	}
	if billing, ok := selector["billing"]; ok {
		if (billing == "token" && rate.Unit == "token") || (billing == "audio" && rate.Unit == "second") {
			return true, nil
		}
	}
	if modality, ok := selector["modality"]; ok && rate.Unit == "token" {
		if modality == "text" && (rate.UsageKey == "input" || rate.UsageKey == "textInput") {
			return true, nil
		}
		if modality == "audio" && (rate.UsageKey == "output" || rate.UsageKey == "audioInput") {
			return true, nil
		}
	}
	return false, ErrInvalidSnapshot
}

func mediaQuantity(u pr.MediaUsage, key, unit string) (string, bool) {
	if unit == "image" {
		if key == "output" || key == "image" {
			if u.ImageCount >= 0 {
				return strconv.FormatInt(u.ImageCount, 10), true
			}
		}
		return "", false
	}
	if unit == "second" {
		if key != "audio" && key != "duration" {
			return "", false
		}
		return strings.TrimSpace(u.DurationSeconds), u.DurationSeconds != ""
	}
	if unit == "character" {
		if u.TextInputTokens != nil && *u.TextInputTokens >= 0 {
			return strconv.FormatInt(*u.TextInputTokens, 10), true
		}
		return "", false
	}
	if unit == "request" {
		return "1", true
	}
	if unit != "token" {
		return "", false
	}
	var value *int64
	switch key {
	case "input":
		value = u.InputTokens
	case "output":
		value = u.OutputTokens
	case "audioInput":
		value = u.AudioInputTokens
	case "textInput":
		value = u.TextInputTokens
	}
	if value == nil || *value < 0 {
		return "", false
	}
	return strconv.FormatInt(*value, 10), true
}
