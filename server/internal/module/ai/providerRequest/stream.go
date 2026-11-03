package providerrequest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
)

var errStreamTerminal = errors.New("provider stream terminal")

func readSSE(source io.Reader, receive func([]byte) error) error {
	reader := &io.LimitedReader{R: source, N: maxProtocolBytes + 1}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxProtocolBytes+1)
	var data bytes.Buffer
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		payload := append([]byte(nil), data.Bytes()...)
		data.Reset()
		return receive(bytes.TrimSuffix(payload, []byte("\n")))
	}
	for scanner.Scan() {
		if reader.N <= 0 {
			return ErrResponseInvalid
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				if errors.Is(err, errStreamTerminal) {
					return nil
				}
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data.WriteString(value)
			data.WriteByte('\n')
		}
		if reader.N <= 0 {
			return ErrResponseInvalid
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if reader.N <= 0 {
		return ErrResponseInvalid
	}
	if err := flush(); err != nil && !errors.Is(err, errStreamTerminal) {
		return err
	}
	return nil
}
func streamText(ctx context.Context, client *http.Client, in TextRequest, path string, body map[string]json.RawMessage, chat bool) (TextResponse, error) {
	target, e := endpoint(in.Config.BaseURL, path)
	if e != nil {
		return TextResponse{}, requestFailure("invalid", 0, "", false, e)
	}
	raw, e := json.Marshal(body)
	if e != nil || len(raw) > maxProtocolBytes {
		return TextResponse{}, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, in.Config.Timeout)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if e != nil {
		return TextResponse{}, requestFailure("invalid", 0, "", false, e)
	}
	req.GetBody = nil
	for key, value := range in.Config.Headers {
		if !validHeaderName(key) || !validHeaderValue(value) || reservedHeader(key) {
			return TextResponse{}, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
		}
		req.Header.Set(key, value)
	}
	req.Header.Set("Authorization", "Bearer "+in.Config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	owned := http.Client{}
	if client != nil {
		owned = *client
	}
	owned.Jar = nil
	owned.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, e := owned.Do(req)
	if e != nil {
		return TextResponse{}, requestFailure("stream", 0, "", true, e)
	}
	defer response.Body.Close()
	id := safeRequestID(response.Header.Get("x-request-id"), in.Config.APIKey)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		unknown := true
		switch response.StatusCode {
		case 400, 401, 403, 404, 405, 413, 415, 422, 429:
			unknown = false
		}
		return TextResponse{RequestID: id}, requestFailure("http", response.StatusCode, id, unknown, ErrUpstreamRejected)
	}
	var result TextResponse
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		if chat {
			result, e = readChatStream(response.Body, in.OnTextDelta)
		} else {
			result, e = readResponseStream(response.Body, in.OnTextDelta)
		}
	} else {
		var payload []byte
		payload, e = io.ReadAll(io.LimitReader(response.Body, maxProtocolBytes+1))
		if e == nil && len(payload) <= maxProtocolBytes {
			if chat {
				result, e = parseChatResponse(payload)
			} else {
				result, e = parseResponse(payload)
			}
		} else if e == nil {
			e = ErrResponseInvalid
		}
	}
	result.RequestID = id
	if e != nil {
		return result, requestFailure("stream", response.StatusCode, id, true, e)
	}
	if result.Status != "completed" {
		return result, requestFailure("incomplete", response.StatusCode, id, false, ErrResponseIncomplete)
	}
	return result, nil
}
func readResponseStream(source io.Reader, onText func(string) error) (TextResponse, error) {
	var result TextResponse
	terminal := false
	var partial strings.Builder
	e := readSSE(source, func(raw []byte) error {
		if bytes.Equal(raw, []byte("[DONE]")) {
			return ErrResponseInvalid
		}
		var event struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return ErrResponseInvalid
		}
		switch event.Type {
		case "response.output_text.delta":
			partial.WriteString(event.Delta)
			if partial.Len() > maxProtocolBytes {
				return ErrResponseInvalid
			}
			if event.Delta != "" {
				return onText(event.Delta)
			}
		case "response.completed", "response.incomplete", "response.failed", "response.cancelled":
			var err error
			result, err = parseResponse(event.Response)
			if err != nil {
				return err
			}
			terminal = true
			return errStreamTerminal
		case "error", "response.error":
			return ErrResponseInvalid
		}
		return nil
	})
	if e != nil {
		if result.Text == "" {
			result.Text = partial.String()
		}
		return result, e
	}
	if !terminal {
		result.Text = partial.String()
		return result, ErrResponseIncomplete
	}
	return result, nil
}

type streamedCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func readChatStream(source io.Reader, onText func(string) error) (TextResponse, error) {
	id, finish := "", ""
	var text, refusal strings.Builder
	usage := json.RawMessage(`null`)
	calls := map[int]*streamedCall{}
	done := false
	e := readSSE(source, func(raw []byte) error {
		if bytes.Equal(raw, []byte("[DONE]")) {
			done = true
			return errStreamTerminal
		}
		var event struct {
			ID      string          `json:"id"`
			Error   json.RawMessage `json:"error"`
			Usage   json.RawMessage `json:"usage"`
			Choices []struct {
				Index  int     `json:"index"`
				Finish *string `json:"finish_reason"`
				Delta  struct {
					Role    string  `json:"role"`
					Content *string `json:"content"`
					Refusal *string `json:"refusal"`
					Tools   []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal(raw, &event) != nil || len(event.Error) > 0 && string(event.Error) != "null" {
			return ErrResponseInvalid
		}
		if event.ID != "" {
			if id != "" && id != event.ID {
				return ErrResponseInvalid
			}
			id = event.ID
		}
		if len(event.Usage) > 0 && string(event.Usage) != "null" {
			usage = event.Usage
		}
		if len(event.Choices) > 1 {
			return ErrResponseInvalid
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 || choice.Delta.Role != "" && choice.Delta.Role != "assistant" {
				return ErrResponseInvalid
			}
			if choice.Delta.Content != nil {
				text.WriteString(*choice.Delta.Content)
				if text.Len() > maxProtocolBytes {
					return ErrResponseInvalid
				}
				if *choice.Delta.Content != "" {
					if err := onText(*choice.Delta.Content); err != nil {
						return err
					}
				}
			}
			if choice.Delta.Refusal != nil {
				refusal.WriteString(*choice.Delta.Refusal)
			}
			for _, piece := range choice.Delta.Tools {
				if piece.Index < 0 || piece.Index >= 64 {
					return ErrResponseInvalid
				}
				call := calls[piece.Index]
				if call == nil {
					call = &streamedCall{}
					calls[piece.Index] = call
				}
				if piece.ID != "" {
					if call.ID != "" && call.ID != piece.ID {
						return ErrResponseInvalid
					}
					call.ID = piece.ID
				}
				if piece.Type != "" {
					if piece.Type != "function" {
						return ErrResponseInvalid
					}
					call.Type = piece.Type
				}
				call.Function.Name += piece.Function.Name
				call.Function.Arguments += piece.Function.Arguments
				if len(call.Function.Arguments) > 65536 || len(call.Function.Name) > 64 {
					return ErrResponseInvalid
				}
			}
			if choice.Finish != nil {
				finish = *choice.Finish
			}
		}
		return nil
	})
	if e != nil {
		return TextResponse{ID: id, Text: text.String()}, e
	}
	if !done || finish == "" || id == "" {
		return TextResponse{ID: id, Text: text.String()}, ErrResponseIncomplete
	}
	ordered := make([]int, 0, len(calls))
	for index := range calls {
		ordered = append(ordered, index)
	}
	sort.Ints(ordered)
	toolCalls := make([]*streamedCall, 0, len(calls))
	for _, index := range ordered {
		toolCalls = append(toolCalls, calls[index])
	}
	message := map[string]any{"role": "assistant", "content": text.String(), "refusal": refusal.String()}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	payload, e := json.Marshal(map[string]any{"id": id, "choices": []map[string]any{{"message": message, "finish_reason": finish}}, "usage": usage})
	if e != nil {
		return TextResponse{}, e
	}
	return parseChatResponse(payload)
}
