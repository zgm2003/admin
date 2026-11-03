package providerrequest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// ResponsesClient implements non-streaming Responses. Complete output items,
// including opaque reasoning, are retained for the caller's next request.
// Protocol: https://developers.openai.com/api/docs/guides/function-calling
type ResponsesClient struct{ client *http.Client }

func NewResponsesClient(client *http.Client) *ResponsesClient {
	return &ResponsesClient{client: client}
}

func (c *ResponsesClient) Generate(ctx context.Context, in TextRequest) (TextResponse, error) {
	if err := ctx.Err(); err != nil {
		return TextResponse{}, requestFailure("canceled", 0, "", false, err)
	}
	if c == nil {
		return TextResponse{}, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
	}
	body, err := validateRequest(in)
	if err != nil {
		return TextResponse{}, requestFailure("invalid", 0, "", false, err)
	}
	for key, value := range map[string]any{"model": in.Config.Model, "input": in.Input, "instructions": in.Prompt, "max_output_tokens": in.MaxOutputTokens, "stream": false, "store": false, "background": false} {
		if err := putJSON(body, key, value); err != nil {
			return TextResponse{}, requestFailure("invalid", 0, "", false, err)
		}
	}
	var include []string
	if raw, ok := body["include"]; ok {
		if err := json.Unmarshal(raw, &include); err != nil {
			return TextResponse{}, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
		}
	}
	found := false
	for _, value := range include {
		if value == "reasoning.encrypted_content" {
			found = true
		}
	}
	if !found {
		include = append(include, "reasoning.encrypted_content")
	}
	_ = putJSON(body, "include", include)
	if len(in.Tools) > 0 {
		tools := make([]map[string]any, 0, len(in.Tools))
		for _, tool := range in.Tools {
			tools = append(tools, map[string]any{"type": "function", "name": tool.Name, "description": tool.Description, "parameters": tool.Parameters, "strict": tool.Strict})
		}
		_ = putJSON(body, "tools", tools)
	}
	if in.OnTextDelta != nil {
		_ = putJSON(body, "stream", true)
		return streamText(ctx, c.client, in, "responses", body, false)
	}
	raw, id, status, err := postJSON(ctx, c.client, in, "responses", body)
	if err != nil {
		return TextResponse{RequestID: id}, err
	}
	response, err := parseResponse(raw)
	response.RequestID = id
	terminal := response.Status == "completed" || response.Status == "incomplete" || response.Status == "failed" || response.Status == "cancelled"
	if err != nil {
		return response, requestFailure("protocol", status, id, response.ID == "" || !terminal, err)
	}
	if response.Status != "completed" {
		return response, requestFailure("incomplete", status, id, !terminal, ErrResponseIncomplete)
	}
	return response, nil
}

func parseResponse(raw []byte) (TextResponse, error) {
	response := TextResponse{Raw: append(json.RawMessage(nil), raw...), Calls: []FunctionCall{}}
	var wire struct {
		ID     string          `json:"id"`
		Status string          `json:"status"`
		Output json.RawMessage `json:"output"`
		Usage  json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return response, ErrResponseInvalid
	}
	response.ID, response.Status = wire.ID, wire.Status
	response.Usage = parseTokenUsage(wire.Usage, false)
	if wire.ID == "" || wire.Status == "" || len(wire.Output) == 0 || string(wire.Output) == "null" || json.Unmarshal(wire.Output, &response.OutputItems) != nil {
		return response, ErrResponseInvalid
	}
	var text, refusal strings.Builder
	for _, item := range response.OutputItems {
		var row struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(item, &row); err != nil || row.Type == "" {
			return response, ErrResponseInvalid
		}
		switch row.Type {
		case "function_call":
			var call struct {
				CallID    string  `json:"call_id"`
				Name      string  `json:"name"`
				Arguments *string `json:"arguments"`
			}
			if json.Unmarshal(item, &call) != nil || call.CallID == "" || call.Name == "" || call.Arguments == nil {
				return response, ErrResponseInvalid
			}
			response.Calls = append(response.Calls, FunctionCall{CallID: call.CallID, Name: call.Name, Arguments: *call.Arguments})
		case "message":
			var message struct {
				Content []struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					Refusal string `json:"refusal"`
				} `json:"content"`
			}
			if json.Unmarshal(item, &message) != nil {
				return response, ErrResponseInvalid
			}
			for _, part := range message.Content {
				switch part.Type {
				case "output_text":
					text.WriteString(part.Text)
				case "refusal":
					refusal.WriteString(part.Refusal)
				}
			}
		}
	}
	response.Text, response.Refusal = text.String(), refusal.String()
	return response, nil
}
