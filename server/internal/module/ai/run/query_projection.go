package run

import (
	"bytes"
	"encoding/json"
	"errors"
)

type publicTextItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Native provider items may contain function arguments, reasoning, upstream
// signed URLs or unrecognized extensions. Only display text is public.
func publicProviderItems(raw json.RawMessage) (json.RawMessage, error) {
	out := make([]publicTextItem, 0)
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return json.RawMessage(`[]`), nil
	}
	var items []json.RawMessage
	if e := json.Unmarshal(raw, &items); e != nil {
		return nil, e
	}
	for _, item := range items {
		var value struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Text    string          `json:"text"`
			Refusal string          `json:"refusal"`
			Content json.RawMessage `json:"content"`
		}
		if e := json.Unmarshal(item, &value); e != nil {
			return nil, e
		}
		if value.Type == "function_call" || value.Type == "function_call_output" || value.Type == "reasoning" || value.Role == "tool" {
			continue
		}
		if value.Type == "output_text" || value.Type == "text" {
			out = append(out, publicTextItem{"output_text", value.Text})
			continue
		}
		if value.Type == "refusal" {
			out = append(out, publicTextItem{"refusal", value.Refusal})
			continue
		}
		if value.Type == "message" || value.Role == "assistant" {
			var parts []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			}
			if len(value.Content) == 0 {
				continue
			}
			if value.Content[0] == '"' {
				var text string
				if e := json.Unmarshal(value.Content, &text); e != nil {
					return nil, e
				}
				out = append(out, publicTextItem{"output_text", text})
				continue
			}
			if e := json.Unmarshal(value.Content, &parts); e != nil {
				return nil, e
			}
			for _, part := range parts {
				if part.Type == "output_text" || part.Type == "text" {
					out = append(out, publicTextItem{"output_text", part.Text})
				}
				if part.Type == "refusal" {
					out = append(out, publicTextItem{"refusal", part.Refusal})
				}
			}
			continue
		}
		if value.Type == "" && value.Text != "" {
			out = append(out, publicTextItem{"output_text", value.Text})
		}
	}
	b, e := json.Marshal(out)
	return b, e
}
func publicProviderUsage(raw json.RawMessage) (json.RawMessage, error) {
	var value struct {
		Known             bool   `json:"known"`
		Basis             string `json:"basis,omitempty"`
		InputTokens       *int64 `json:"inputTokens,omitempty"`
		CachedInputTokens *int64 `json:"cachedInputTokens,omitempty"`
		OutputTokens      *int64 `json:"outputTokens,omitempty"`
		ImageCount        *int64 `json:"imageCount,omitempty"`
		DurationSeconds   string `json:"durationSeconds,omitempty"`
		AudioInputTokens  *int64 `json:"audioInputTokens,omitempty"`
		TextInputTokens   *int64 `json:"textInputTokens,omitempty"`
	}
	if len(raw) == 0 {
		return nil, errors.New("missing AI provider usage")
	}
	if e := json.Unmarshal(raw, &value); e != nil {
		return nil, e
	}
	b, e := json.Marshal(value)
	return b, e
}
func publicResult(raw json.RawMessage) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, errors.New("missing AI result")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw, nil
	}
	if len(raw) > 0 && bytes.TrimSpace(raw)[0] == '[' {
		return publicProviderItems(raw)
	}
	var value struct {
		Text string `json:"text"`
	}
	if e := json.Unmarshal(raw, &value); e != nil {
		return nil, e
	}
	b, e := json.Marshal(value)
	return b, e
}
