package run

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	aiprovider "admin/server/internal/module/ai/provider"
)

var ErrUnsupportedProtocol = errors.New("provider protocol is not supported by the chat runtime yet")

type ChatAttachment struct {
	URL      string
	IsImage  bool
	FileName string
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ChatMessage struct {
	Role        string
	Content     string
	Attachments []ChatAttachment
	ToolCalls   []ToolCall
	ToolCallID  string
	Name        string
}

type ChatUsage struct {
	UncachedInput int64
	CacheRead     int64
	CacheWrite    int64
	Output        int64
}

type StreamChunk struct {
	Delta     string
	Reasoning string
	ToolCalls []ToolCall
	Usage     *ChatUsage
	Done      bool
	Err       error
}

type ProviderRequest struct {
	Protocol        int16
	BaseURL         string
	APIKey          string
	Model           string
	SystemPrompt    string
	Messages        []ChatMessage
	Tools           []ToolDefinition
	MaxOutputTokens int
	Timeout         time.Duration
}

type ProviderClient struct {
	httpClient *http.Client
}

func NewProviderClient(httpClient *http.Client) *ProviderClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 0}
	}
	return &ProviderClient{httpClient: httpClient}
}

func (c *ProviderClient) Stream(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	switch aiprovider.Protocol(request.Protocol) {
	case aiprovider.ProtocolOpenAI:
		return c.streamOpenAI(ctx, request)
	case aiprovider.ProtocolAnthropic:
		return c.streamAnthropic(ctx, request)
	case aiprovider.ProtocolGemini:
		return c.streamGemini(ctx, request)
	case aiprovider.ProtocolOpenAIResponses:
		return c.streamOpenAIResponses(ctx, request)
	default:
		return nil, fmt.Errorf("%w: protocol %d", ErrUnsupportedProtocol, request.Protocol)
	}
}

func (c *ProviderClient) streamOpenAI(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	messages := openAIMessages(request)
	payload := map[string]any{
		"model": request.Model, "messages": messages, "stream": true,
		"stream_options": map[string]bool{"include_usage": true},
	}
	if len(request.Tools) > 0 {
		payload["tools"] = openAITools(request.Tools)
		payload["tool_choice"] = "auto"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(request.BaseURL, "/") + "/chat/completions"
	response, err := c.doJSONRequest(ctx, endpoint, request.APIKey, body, openAIHeaders)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnprocessableEntity {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if bytes.Contains(bytes.ToLower(raw), []byte("stream_options")) {
			delete(payload, "stream_options")
			body, _ = json.Marshal(payload)
			response, err = c.doJSONRequest(ctx, endpoint, request.APIKey, body, openAIHeaders)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, upstreamError(response.StatusCode, raw)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}

	chunks := make(chan StreamChunk, 32)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		toolCalls := make(map[int]*toolCallAccumulator)
		var usage *ChatUsage
		emitDone := func() {
			emitChunk(ctx, chunks, StreamChunk{
				ToolCalls: accumulatorsToCalls(toolCalls), Usage: usage, Done: true,
			})
		}
		err := scanSSE(response.Body, func(_, data string) error {
			if data == "[DONE]" {
				emitDone()
				return errStopSSE
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
						ToolCalls        []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens        int64 `json:"prompt_tokens"`
					CompletionTokens    int64 `json:"completion_tokens"`
					PromptTokensDetails *struct {
						CachedTokens int64 `json:"cached_tokens"`
					} `json:"prompt_tokens_details"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				for _, item := range delta.ToolCalls {
					call := toolCalls[item.Index]
					if call == nil {
						call = &toolCallAccumulator{index: item.Index}
						toolCalls[item.Index] = call
					}
					if item.ID != "" {
						call.ID = item.ID
					}
					if item.Function.Name != "" {
						call.Name += item.Function.Name
					}
					call.Arguments.WriteString(item.Function.Arguments)
				}
				event := StreamChunk{Delta: delta.Content}
				event.Reasoning = delta.ReasoningContent
				if event.Reasoning == "" {
					event.Reasoning = delta.Reasoning
				}
				if event.Delta != "" || event.Reasoning != "" {
					emitChunk(ctx, chunks, event)
				}
			}
			if chunk.Usage != nil {
				cached := int64(0)
				if chunk.Usage.PromptTokensDetails != nil {
					cached = chunk.Usage.PromptTokensDetails.CachedTokens
				}
				if cached < 0 || cached > chunk.Usage.PromptTokens {
					cached = 0
				}
				usage = &ChatUsage{
					UncachedInput: chunk.Usage.PromptTokens - cached,
					CacheRead:     cached, Output: chunk.Usage.CompletionTokens,
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopSSE) && !errors.Is(err, context.Canceled) {
			emitChunk(ctx, chunks, StreamChunk{Err: err})
			return
		}
		if !errors.Is(err, errStopSSE) {
			emitDone()
		}
	}()
	return chunks, nil
}

func (c *ProviderClient) streamAnthropic(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	messages := anthropicMessages(request.Messages)
	maxTokens := request.MaxOutputTokens
	if maxTokens < 1 {
		maxTokens = 4096
	}
	payload := map[string]any{
		"model": request.Model, "max_tokens": maxTokens, "stream": true,
		"messages": messages,
	}
	if strings.TrimSpace(request.SystemPrompt) != "" {
		payload["system"] = request.SystemPrompt
	}
	if len(request.Tools) > 0 {
		payload["tools"] = anthropicTools(request.Tools)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(request.BaseURL, "/") + "/messages"
	response, err := c.doJSONRequest(ctx, endpoint, request.APIKey, body, anthropicHeaders)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}

	chunks := make(chan StreamChunk, 32)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		tools := make(map[int]*toolCallAccumulator)
		usage := &ChatUsage{}
		emitDone := func() {
			emitChunk(ctx, chunks, StreamChunk{ToolCalls: accumulatorsToCalls(tools), Usage: usage, Done: true})
		}
		err := scanSSE(response.Body, func(event, data string) error {
			if event == "error" {
				return errors.New(cleanUpstreamMessage(data))
			}
			var chunk struct {
				Type         string `json:"type"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					Thinking    string `json:"thinking"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
				Usage *struct {
					InputTokens          int64 `json:"input_tokens"`
					OutputTokens         int64 `json:"output_tokens"`
					CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
					CacheCreationTokens  int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
				Message *struct {
					Usage *struct {
						InputTokens          int64 `json:"input_tokens"`
						CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
						CacheCreationTokens  int64 `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil
			}
			if chunk.Type == "message_start" && chunk.Message != nil && chunk.Message.Usage != nil {
				usage.UncachedInput = chunk.Message.Usage.InputTokens
				usage.CacheRead = chunk.Message.Usage.CacheReadInputTokens
				usage.CacheWrite = chunk.Message.Usage.CacheCreationTokens
			}
			if chunk.Usage != nil {
				if chunk.Usage.InputTokens > 0 {
					usage.UncachedInput = chunk.Usage.InputTokens
				}
				usage.CacheRead = chunk.Usage.CacheReadInputTokens
				usage.CacheWrite = chunk.Usage.CacheCreationTokens
				usage.Output = chunk.Usage.OutputTokens
			}
			if chunk.Type == "content_block_start" && chunk.ContentBlock.Type == "tool_use" {
				tools[chunk.Index] = &toolCallAccumulator{
					index: chunk.Index, ID: chunk.ContentBlock.ID, Name: chunk.ContentBlock.Name,
				}
			}
			if chunk.Type == "content_block_delta" {
				switch chunk.Delta.Type {
				case "text_delta":
					if chunk.Delta.Text != "" {
						emitChunk(ctx, chunks, StreamChunk{Delta: chunk.Delta.Text})
					}
				case "thinking_delta":
					if chunk.Delta.Thinking != "" {
						emitChunk(ctx, chunks, StreamChunk{Reasoning: chunk.Delta.Thinking})
					}
				case "input_json_delta":
					if call := tools[chunk.Index]; call != nil {
						call.Arguments.WriteString(chunk.Delta.PartialJSON)
					}
				}
			}
			if chunk.Type == "message_stop" {
				emitDone()
				return errStopSSE
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopSSE) && !errors.Is(err, context.Canceled) {
			emitChunk(ctx, chunks, StreamChunk{Err: err})
			return
		}
		if !errors.Is(err, errStopSSE) {
			emitDone()
		}
	}()
	return chunks, nil
}

func (c *ProviderClient) streamGemini(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	contents := geminiContents(request.Messages)
	payload := map[string]any{"contents": contents}
	if request.MaxOutputTokens > 0 {
		payload["generationConfig"] = map[string]any{"maxOutputTokens": request.MaxOutputTokens}
	}
	if strings.TrimSpace(request.SystemPrompt) != "" {
		payload["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": request.SystemPrompt}}}
	}
	if len(request.Tools) > 0 {
		payload["tools"] = []map[string]any{{"functionDeclarations": geminiTools(request.Tools)}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	model := strings.TrimPrefix(strings.TrimSpace(request.Model), "models/")
	endpoint := strings.TrimRight(request.BaseURL, "/") + "/models/" + url.PathEscape(model) + ":streamGenerateContent?alt=sse"
	response, err := c.doJSONRequest(ctx, endpoint, request.APIKey, body, geminiHeaders)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}

	chunks := make(chan StreamChunk, 32)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		var calls []ToolCall
		var usage *ChatUsage
		emitDone := func() {
			emitChunk(ctx, chunks, StreamChunk{ToolCalls: calls, Usage: usage, Done: true})
		}
		err := scanSSE(response.Body, func(_, data string) error {
			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text         string         `json:"text"`
							Thought      bool           `json:"thought"`
							FunctionCall map[string]any `json:"functionCall"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
				UsageMetadata *struct {
					PromptTokenCount        int64 `json:"promptTokenCount"`
					CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
					CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
				} `json:"usageMetadata"`
				PromptFeedback *struct {
					BlockReason string `json:"blockReason"`
				} `json:"promptFeedback"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil
			}
			if chunk.PromptFeedback != nil && chunk.PromptFeedback.BlockReason != "" {
				return fmt.Errorf("gemini blocked prompt: %s", chunk.PromptFeedback.BlockReason)
			}
			for _, candidate := range chunk.Candidates {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						if part.Thought {
							emitChunk(ctx, chunks, StreamChunk{Reasoning: part.Text})
						} else {
							emitChunk(ctx, chunks, StreamChunk{Delta: part.Text})
						}
					}
					if part.FunctionCall != nil {
						name, _ := part.FunctionCall["name"].(string)
						args := "{}"
						if raw, ok := part.FunctionCall["args"]; ok {
							if encoded, marshalErr := json.Marshal(raw); marshalErr == nil {
								args = string(encoded)
							}
						}
						calls = append(calls, ToolCall{
							ID:   fmt.Sprintf("gemini_%d_%d", time.Now().UnixNano(), len(calls)),
							Name: name, Arguments: args,
						})
					}
				}
			}
			if chunk.UsageMetadata != nil {
				cached := chunk.UsageMetadata.CachedContentTokenCount
				if cached < 0 || cached > chunk.UsageMetadata.PromptTokenCount {
					cached = 0
				}
				usage = &ChatUsage{
					UncachedInput: chunk.UsageMetadata.PromptTokenCount - cached,
					CacheRead:     cached, Output: chunk.UsageMetadata.CandidatesTokenCount,
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			emitChunk(ctx, chunks, StreamChunk{Err: err})
			return
		}
		emitDone()
	}()
	return chunks, nil
}

func (c *ProviderClient) streamOpenAIResponses(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	input, err := responsesInput(request.Messages)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model": request.Model, "input": input, "stream": true, "store": false,
	}
	if request.MaxOutputTokens > 0 {
		payload["max_output_tokens"] = request.MaxOutputTokens
	}
	if strings.TrimSpace(request.SystemPrompt) != "" {
		payload["instructions"] = request.SystemPrompt
	}
	if len(request.Tools) > 0 {
		payload["tools"] = responsesTools(request.Tools)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(request.BaseURL, "/") + "/responses"
	response, err := c.doJSONRequest(ctx, endpoint, request.APIKey, body, openAIHeaders)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}

	chunks := make(chan StreamChunk, 32)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		tools := make(map[string]*toolCallAccumulator)
		var usage *ChatUsage
		completed := false
		emitDone := func() {
			calls := make([]ToolCall, 0, len(tools))
			keys := make([]string, 0, len(tools))
			for key := range tools {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				call := tools[key]
				calls = append(calls, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments.String()})
			}
			emitChunk(ctx, chunks, StreamChunk{ToolCalls: calls, Usage: usage, Done: true})
		}
		err := scanSSE(response.Body, func(_, data string) error {
			var event struct {
				Type   string `json:"type"`
				Delta  string `json:"delta"`
				ItemID string `json:"item_id"`
				Item   struct {
					Type      string `json:"type"`
					ID        string `json:"id"`
					CallID    string `json:"call_id"`
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"item"`
				Response *struct {
					Output []struct {
						Type      string `json:"type"`
						ID        string `json:"id"`
						CallID    string `json:"call_id"`
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"output"`
					Usage *struct {
						InputTokens        int64 `json:"input_tokens"`
						OutputTokens       int64 `json:"output_tokens"`
						InputTokensDetails *struct {
							CachedTokens int64 `json:"cached_tokens"`
						} `json:"input_tokens_details"`
					} `json:"usage"`
				} `json:"response"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return nil
			}
			if event.Error != nil && event.Error.Message != "" {
				return errors.New(event.Error.Message)
			}
			switch event.Type {
			case "response.output_text.delta", "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
				if event.Delta != "" {
					if strings.Contains(event.Type, "reasoning") {
						emitChunk(ctx, chunks, StreamChunk{Reasoning: event.Delta})
					} else {
						emitChunk(ctx, chunks, StreamChunk{Delta: event.Delta})
					}
				}
			case "response.output_item.added":
				if event.Item.Type == "function_call" {
					key := event.Item.ID
					if key == "" {
						key = event.Item.CallID
					}
					tools[key] = &toolCallAccumulator{ID: event.Item.CallID, Name: event.Item.Name}
				}
			case "response.function_call_arguments.delta":
				key := event.ItemID
				if call := tools[key]; call != nil {
					call.Arguments.WriteString(event.Delta)
				}
			case "response.output_item.done":
				if event.Item.Type == "function_call" {
					key := event.Item.ID
					if key == "" {
						key = event.Item.CallID
					}
					call := tools[key]
					if call == nil {
						call = &toolCallAccumulator{}
						tools[key] = call
					}
					if event.Item.CallID != "" {
						call.ID = event.Item.CallID
					}
					if event.Item.Name != "" {
						call.Name = event.Item.Name
					}
					if event.Item.Arguments != "" {
						call.Arguments.Reset()
						call.Arguments.WriteString(event.Item.Arguments)
					}
				}
			case "response.completed":
				completed = true
				if event.Response != nil {
					for _, item := range event.Response.Output {
						if item.Type != "function_call" {
							continue
						}
						call := tools[item.ID]
						if call == nil {
							call = &toolCallAccumulator{}
							tools[item.ID] = call
						}
						call.ID, call.Name = item.CallID, item.Name
						if item.Arguments != "" {
							call.Arguments.Reset()
							call.Arguments.WriteString(item.Arguments)
						}
					}
					if event.Response.Usage != nil {
						cached := int64(0)
						if event.Response.Usage.InputTokensDetails != nil {
							cached = event.Response.Usage.InputTokensDetails.CachedTokens
						}
						usage = &ChatUsage{
							UncachedInput: event.Response.Usage.InputTokens - cached,
							CacheRead:     cached, Output: event.Response.Usage.OutputTokens,
						}
					}
				}
				emitDone()
				return errStopSSE
			case "response.failed", "error":
				return errors.New("upstream response failed")
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopSSE) && !errors.Is(err, context.Canceled) {
			emitChunk(ctx, chunks, StreamChunk{Err: err})
			return
		}
		if !completed && !errors.Is(err, errStopSSE) {
			emitDone()
		}
	}()
	return chunks, nil
}

func openAIMessages(request ProviderRequest) []map[string]any {
	messages := make([]map[string]any, 0, len(request.Messages)+1)
	if strings.TrimSpace(request.SystemPrompt) != "" {
		messages = append(messages, map[string]any{"role": "system", "content": request.SystemPrompt})
	}
	for _, message := range request.Messages {
		role := message.Role
		if role == "" {
			role = "user"
		}
		switch role {
		case "assistant":
			item := map[string]any{"role": "assistant"}
			if message.Content != "" {
				item["content"] = message.Content
			} else {
				item["content"] = nil
			}
			if len(message.ToolCalls) > 0 {
				toolCalls := make([]map[string]any, 0, len(message.ToolCalls))
				for _, call := range message.ToolCalls {
					toolCalls = append(toolCalls, map[string]any{
						"id": call.ID, "type": "function",
						"function": map[string]any{"name": call.Name, "arguments": call.Arguments},
					})
				}
				item["tool_calls"] = toolCalls
			}
			messages = append(messages, item)
		case "tool":
			item := map[string]any{"role": "tool", "tool_call_id": message.ToolCallID, "content": message.Content}
			if message.Name != "" {
				item["name"] = message.Name
			}
			messages = append(messages, item)
		default:
			item := map[string]any{"role": "user", "content": message.Content}
			if len(message.Attachments) > 0 {
				parts := make([]map[string]any, 0, len(message.Attachments)+1)
				if message.Content != "" {
					parts = append(parts, map[string]any{"type": "text", "text": message.Content})
				}
				for _, attachment := range message.Attachments {
					if attachment.IsImage {
						parts = append(parts, map[string]any{
							"type": "image_url", "image_url": map[string]string{"url": attachment.URL},
						})
					}
				}
				if len(parts) > 0 {
					item["content"] = parts
				}
			}
			messages = append(messages, item)
		}
	}
	return messages
}

func openAITools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"type": "function", "function": map[string]any{
				"name": tool.Name, "description": tool.Description, "parameters": tool.Parameters,
			},
		})
	}
	return result
}

func anthropicHeaders(request *http.Request, apiKey string) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("x-api-key", apiKey)
	request.Header.Set("anthropic-version", "2023-06-01")
}

func anthropicTools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"name": tool.Name, "description": tool.Description, "input_schema": tool.Parameters,
		})
	}
	return result
}

func anthropicMessages(messages []ChatMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for index := 0; index < len(messages); index++ {
		message := messages[index]
		if message.Role == "tool" {
			blocks := make([]map[string]any, 0, 2)
			for index < len(messages) && messages[index].Role == "tool" {
				item := messages[index]
				blocks = append(blocks, map[string]any{
					"type": "tool_result", "tool_use_id": item.ToolCallID,
					"content": item.Content,
				})
				index++
			}
			index--
			result = append(result, map[string]any{"role": "user", "content": blocks})
			continue
		}
		role := "user"
		if message.Role == "assistant" {
			role = "assistant"
		}
		blocks := make([]map[string]any, 0, len(message.ToolCalls)+len(message.Attachments)+1)
		if message.Content != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": message.Content})
		}
		for _, attachment := range message.Attachments {
			if attachment.IsImage {
				blocks = append(blocks, map[string]any{
					"type": "image", "source": map[string]any{"type": "url", "url": attachment.URL},
				})
			}
		}
		for _, call := range message.ToolCalls {
			var input any = map[string]any{}
			if strings.TrimSpace(call.Arguments) != "" {
				if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
					input = map[string]any{"raw": call.Arguments}
				}
			}
			blocks = append(blocks, map[string]any{
				"type": "tool_use", "id": call.ID, "name": call.Name, "input": input,
			})
		}
		if len(blocks) == 0 {
			blocks = append(blocks, map[string]any{"type": "text", "text": ""})
		}
		result = append(result, map[string]any{"role": role, "content": blocks})
	}
	return result
}

func geminiHeaders(request *http.Request, apiKey string) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("x-goog-api-key", apiKey)
}

func geminiTools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"name": tool.Name, "description": tool.Description, "parameters": tool.Parameters,
		})
	}
	return result
}

func geminiContents(messages []ChatMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for index := 0; index < len(messages); index++ {
		message := messages[index]
		if message.Role == "tool" {
			parts := make([]map[string]any, 0, 2)
			for index < len(messages) && messages[index].Role == "tool" {
				item := messages[index]
				parts = append(parts, map[string]any{
					"functionResponse": map[string]any{
						"name": item.Name, "response": map[string]any{"content": item.Content},
					},
				})
				index++
			}
			index--
			result = append(result, map[string]any{"role": "user", "parts": parts})
			continue
		}
		role := "user"
		if message.Role == "assistant" {
			role = "model"
		}
		parts := make([]map[string]any, 0, len(message.ToolCalls)+len(message.Attachments)+1)
		if message.Content != "" {
			parts = append(parts, map[string]any{"text": message.Content})
		}
		for _, attachment := range message.Attachments {
			if attachment.IsImage {
				parts = append(parts, map[string]any{
					"fileData": map[string]any{"mimeType": "image/*", "fileUri": attachment.URL},
				})
			}
		}
		for _, call := range message.ToolCalls {
			var args any = map[string]any{}
			if strings.TrimSpace(call.Arguments) != "" {
				if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
					args = map[string]any{}
				}
			}
			parts = append(parts, map[string]any{"functionCall": map[string]any{"name": call.Name, "args": args}})
		}
		if len(parts) == 0 {
			parts = append(parts, map[string]any{"text": ""})
		}
		result = append(result, map[string]any{"role": role, "parts": parts})
	}
	return result
}

func responsesTools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"type": "function", "name": tool.Name, "description": tool.Description,
			"parameters": tool.Parameters,
		})
	}
	return result
}

func responsesInput(messages []ChatMessage) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "tool":
			result = append(result, map[string]any{
				"type": "function_call_output", "call_id": message.ToolCallID, "output": message.Content,
			})
		case "assistant":
			if message.Content != "" {
				result = append(result, map[string]any{"role": "assistant", "content": message.Content})
			}
			for _, call := range message.ToolCalls {
				result = append(result, map[string]any{
					"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.Arguments,
				})
			}
		default:
			content := any(message.Content)
			if len(message.Attachments) > 0 {
				parts := make([]map[string]any, 0, len(message.Attachments)+1)
				if message.Content != "" {
					parts = append(parts, map[string]any{"type": "input_text", "text": message.Content})
				}
				for _, attachment := range message.Attachments {
					if attachment.IsImage {
						parts = append(parts, map[string]any{"type": "input_image", "image_url": attachment.URL})
					}
				}
				if len(parts) > 0 {
					content = parts
				}
			}
			result = append(result, map[string]any{"role": "user", "content": content})
		}
	}
	return result, nil
}

func geminiErrorMessage(data string) string {
	return cleanUpstreamMessage(data)
}

func cleanUpstreamMessage(data string) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(data), &payload) == nil && payload.Error.Message != "" {
		return payload.Error.Message
	}
	message := strings.TrimSpace(data)
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

type toolCallAccumulator struct {
	index     int
	ID        string
	Name      string
	Arguments strings.Builder
}

func accumulatorsToCalls(values map[int]*toolCallAccumulator) []ToolCall {
	indexes := make([]int, 0, len(values))
	for index := range values {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	result := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := values[index]
		if call.Name == "" {
			continue
		}
		result = append(result, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments.String()})
	}
	return result
}

func (c *ProviderClient) doJSONRequest(
	ctx context.Context, endpoint, apiKey string, body []byte, setHeaders func(*http.Request, string),
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setHeaders(request, apiKey)
	return c.httpClient.Do(request)
}

func openAIHeaders(request *http.Request, apiKey string) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", "Bearer "+apiKey)
}

func responseError(response *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	return upstreamError(response.StatusCode, raw)
}

func upstreamError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("upstream chat request failed (%d): %s", status, message)
}

var errStopSSE = errors.New("stop sse")

func scanSSE(body io.Reader, handler func(event, data string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	event, data := "", ""
	dispatch := func() error {
		if data == "" {
			return nil
		}
		payload := strings.TrimSpace(data)
		data = ""
		if payload == "" {
			return nil
		}
		eventName := event
		event = ""
		if err := handler(eventName, payload); err != nil {
			return err
		}
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			part := strings.TrimPrefix(line, "data:")
			part = strings.TrimPrefix(part, " ")
			if data != "" {
				data += "\n"
			}
			data += part
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func emitChunk(ctx context.Context, chunks chan<- StreamChunk, chunk StreamChunk) {
	select {
	case chunks <- chunk:
	case <-ctx.Done():
	}
}
