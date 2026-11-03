package providerrequest

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
)

func parseTokenUsage(raw json.RawMessage, chat bool) TokenUsage {
	result := TokenUsage{Raw: append(json.RawMessage(nil), raw...)}
	fields, valid := usageObject(raw)
	if !valid {
		return result
	}
	input, output, details := "input_tokens", "output_tokens", "input_tokens_details"
	if chat {
		input, output, details = "prompt_tokens", "completion_tokens", "prompt_tokens_details"
	}
	read := func(value json.RawMessage) (int64, bool) {
		var number *int64
		if len(value) == 0 || json.Unmarshal(value, &number) != nil || number == nil || *number < 0 {
			return 0, false
		}
		return *number, true
	}
	in, ok := read(fields[input])
	if !ok {
		return result
	}
	out, ok := read(fields[output])
	if !ok {
		return result
	}
	detail, valid := usageObject(fields[details])
	if !valid {
		return result
	}
	cached, ok := read(detail["cached_tokens"])
	if !ok || cached > in {
		return result
	}
	if total, exists := fields["total_tokens"]; exists {
		n, ok := read(total)
		if !ok || in > math.MaxInt64-out || n != in+out {
			return result
		}
	}
	result.Known = true
	result.InputTokens = in
	result.CachedInputTokens = cached
	result.OutputTokens = out
	return result
}

// Reject duplicate counters rather than choosing whichever value a generic
// map decoder happens to retain. The untouched raw usage is still auditable.
func usageObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		key, ok := token.(string)
		if !ok {
			return nil, false
		}
		if _, exists := fields[key]; exists {
			return nil, false
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		fields[key] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, false
	}
	var extra json.RawMessage
	if decoder.Decode(&extra) != io.EOF {
		return nil, false
	}
	return fields, true
}
