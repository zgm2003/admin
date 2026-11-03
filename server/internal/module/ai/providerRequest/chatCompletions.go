package providerrequest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// ChatCompletionsClient is an explicit compatibility path, never a fallback
// after a Responses POST. Its native assistant/tool messages are preserved.
type ChatCompletionsClient struct{ client *http.Client }

func NewChatCompletionsClient(client *http.Client) *ChatCompletionsClient {
	return &ChatCompletionsClient{client: client}
}

func (c *ChatCompletionsClient) Generate(ctx context.Context, in TextRequest) (TextResponse, error) {
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
	messages := make([]json.RawMessage, 0, len(in.Input)+1)
	if in.Prompt != "" {
		prompt, _ := json.Marshal(map[string]string{"role": "system", "content": in.Prompt})
		messages = append(messages, prompt)
	}
	messages = append(messages, in.Input...)
	for key, value := range map[string]any{"model": in.Config.Model, "messages": messages, "max_completion_tokens": in.MaxOutputTokens, "stream": false, "store": false, "n": 1} {
		if err := putJSON(body, key, value); err != nil {
			return TextResponse{}, requestFailure("invalid", 0, "", false, err)
		}
	}
	if len(in.Tools) > 0 {
		tools := make([]map[string]any, 0, len(in.Tools))
		for _, tool := range in.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.Parameters, "strict": tool.Strict}})
		}
		_ = putJSON(body, "tools", tools)
	}
	if in.OnTextDelta != nil {
		_ = putJSON(body, "stream", true)
		_ = putJSON(body, "stream_options", map[string]bool{"include_usage": true})
		return streamText(ctx, c.client, in, "chat/completions", body, true)
	}
	raw, id, status, err := postJSON(ctx, c.client, in, "chat/completions", body)
	if err != nil {
		return TextResponse{RequestID: id}, err
	}
	response, err := parseChatResponse(raw)
	response.RequestID = id
	if err != nil {
		return response, requestFailure("protocol", status, id, response.ID == "" || response.Status == "", err)
	}
	if response.Status != "completed" {
		return response, requestFailure("incomplete", status, id, false, ErrResponseIncomplete)
	}
	return response, nil
}

func parseChatResponse(raw []byte) (TextResponse, error) {
	response := TextResponse{Raw: append(json.RawMessage(nil), raw...), Calls: []FunctionCall{}}
	var wire struct {
		ID      string `json:"id"`
		Choices []struct {
			Message      json.RawMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &wire) != nil {
		return response, ErrResponseInvalid
	}
	response.ID = wire.ID
	response.Usage = parseTokenUsage(wire.Usage, true)
	if wire.ID == "" || len(wire.Choices) != 1 {
		return response, ErrResponseInvalid
	}
	choice := wire.Choices[0]
	response.FinishReason = choice.FinishReason
	if choice.FinishReason == "" {
		return response, ErrResponseInvalid
	}
	response.Status = "completed"
	if choice.FinishReason != "stop" && choice.FinishReason != "tool_calls" {
		response.Status = "incomplete"
	}
	var message struct {
		Role      string          `json:"role"`
		Content   json.RawMessage `json:"content"`
		Refusal   string          `json:"refusal"`
		ToolCalls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	}
	if json.Unmarshal(choice.Message, &message) != nil || message.Role != "assistant" {
		return response, ErrResponseInvalid
	}
	response.OutputItems = []json.RawMessage{append(json.RawMessage(nil), choice.Message...)}
	response.Refusal = message.Refusal
	if len(message.Content) > 0 && string(message.Content) != "null" {
		if json.Unmarshal(message.Content, &response.Text) != nil {
			var parts []struct{ Type, Text string }
			if json.Unmarshal(message.Content, &parts) != nil {
				return response, ErrResponseInvalid
			}
			var text strings.Builder
			for _, part := range parts {
				if part.Type == "text" {
					text.WriteString(part.Text)
				}
			}
			response.Text = text.String()
		}
	}
	for _, call := range message.ToolCalls {
		if call.ID == "" || call.Type != "function" || call.Function.Name == "" {
			return response, ErrResponseInvalid
		}
		response.Calls = append(response.Calls, FunctionCall{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return response, nil
}
