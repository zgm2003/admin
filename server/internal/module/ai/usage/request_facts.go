package usage

import (
	"encoding/json"
	"fmt"
	"strconv"

	pr "admin/server/internal/module/ai/providerRequest"
)

// attachRequestFacts independently derives every required quantity from durable
// provider facts. An amount-only check cannot detect omitted zero-priced items.
func attachRequestFacts(facts *SettlementFacts, operation string, input json.RawMessage, requests []pr.Fact) error {
	facts.UsageDigests = make(map[int64]string, len(requests))
	facts.Quantities = make(map[string]string)
	facts.ExpectedRates = make(map[string]int64)
	facts.RequiredItemKeys = make([]string, 0)
	facts.ProviderRequestIDs = make([]int64, 0, len(requests))
	var selection struct{ Quality, Size string }
	if json.Unmarshal(input, &selection) != nil {
		return ErrFrozenSnapshotMismatch
	}
	for _, request := range requests {
		if request.Status == 1 || request.Status == 4 || request.UsageStatus != 1 || request.RunID != facts.RunID {
			return ErrUsageUnknown
		}
		var value any
		quantities := map[string]string{}
		if operation == "text.generate" {
			var u pr.TokenUsage
			if json.Unmarshal(request.Usage, &u) != nil || !u.Known || u.InputTokens < 0 || u.CachedInputTokens < 0 || u.CachedInputTokens > u.InputTokens || u.OutputTokens < 0 {
				return ErrUsageUnknown
			}
			value = u
			quantities["input"] = strconv.FormatInt(u.InputTokens-u.CachedInputTokens, 10)
			quantities["cacheInput"] = strconv.FormatInt(u.CachedInputTokens, 10)
			quantities["output"] = strconv.FormatInt(u.OutputTokens, 10)
		} else {
			var u pr.MediaUsage
			if json.Unmarshal(request.Usage, &u) != nil || !u.Known {
				return ErrUsageUnknown
			}
			value = u
			for _, rate := range facts.Rates {
				if rate.Operation != operation {
					continue
				}
				switch rate.Unit {
				case "image":
					if u.ImageCount < 0 {
						return ErrUsageUnknown
					}
					quantities[rate.UsageKey] = strconv.FormatInt(u.ImageCount, 10)
				case "second":
					if u.DurationSeconds != "" {
						quantities[rate.UsageKey] = u.DurationSeconds
					}
				case "character":
					if u.TextInputTokens != nil && *u.TextInputTokens >= 0 {
						quantities[rate.UsageKey] = strconv.FormatInt(*u.TextInputTokens, 10)
					}
				case "request":
					quantities[rate.UsageKey] = "1"
				case "token":
					var value *int64
					switch rate.UsageKey {
					case "input":
						value = u.InputTokens
					case "output":
						value = u.OutputTokens
					case "audioInput":
						value = u.AudioInputTokens
					case "textInput":
						value = u.TextInputTokens
					}
					if value != nil && *value >= 0 {
						quantities[rate.UsageKey] = strconv.FormatInt(*value, 10)
					}
				}
			}
		}
		digest, err := UsageDigest(value)
		if err != nil {
			return err
		}
		facts.UsageDigests[request.ID] = digest
		facts.ProviderRequestIDs = append(facts.ProviderRequestIDs, request.ID)
		seen := map[string]bool{}
		for _, rate := range facts.Rates {
			if rate.Operation != operation {
				continue
			}
			if operation == "image.generate" || operation == "image.edit" {
				var selector map[string]string
				if json.Unmarshal(rate.Selector, &selector) != nil {
					return ErrFrozenSnapshotMismatch
				}
				if selector["quality"] != selection.Quality || selector["size"] != selection.Size {
					continue
				}
			}
			if seen[rate.UsageKey] {
				return ErrFrozenSnapshotMismatch
			}
			seen[rate.UsageKey] = true
			quantity, ok := quantities[rate.UsageKey]
			if !ok {
				return ErrUsageUnknown
			}
			key := fmt.Sprintf("%d:%s", request.ID, rate.UsageKey)
			facts.RequiredItemKeys = append(facts.RequiredItemKeys, key)
			facts.Quantities[key] = quantity
			facts.ExpectedRates[key] = rate.ID
		}
		if len(seen) == 0 {
			return ErrFrozenSnapshotMismatch
		}
		if operation == "text.generate" {
			if !seen["input"] || !seen["output"] || (!seen["cacheInput"] && quantities["cacheInput"] != "0") {
				return ErrFrozenSnapshotMismatch
			}
		}
	}
	return nil
}
