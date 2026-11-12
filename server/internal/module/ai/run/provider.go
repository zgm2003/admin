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
	"strings"
	"time"
)

var ErrUnsupportedProtocol = errors.New("provider protocol is not supported by the chat runtime yet")

type ChatMessage struct {
	Role    string
	Content string
}

type ChatUsage struct {
	UncachedInput int64
	CacheRead     int64
	CacheWrite    int64
	Output        int64
}

type StreamChunk struct {
	Delta string
	Usage *ChatUsage
	Done  bool
	Err   error
}

type ProviderRequest struct {
	Protocol     int16
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	Messages     []ChatMessage
	Timeout      time.Duration
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
	switch request.Protocol {
	case 1:
		return c.streamOpenAI(ctx, request)
	default:
		return nil, fmt.Errorf("%w: protocol %d", ErrUnsupportedProtocol, request.Protocol)
	}
}

func (c *ProviderClient) streamOpenAI(ctx context.Context, request ProviderRequest) (<-chan StreamChunk, error) {
	messages := make([]map[string]string, 0, len(request.Messages)+1)
	if strings.TrimSpace(request.SystemPrompt) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": request.SystemPrompt})
	}
	for _, message := range request.Messages {
		role := message.Role
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, map[string]string{"role": role, "content": message.Content})
	}
	body, err := json.Marshal(map[string]any{
		"model": request.Model, "messages": messages, "stream": true,
		"stream_options": map[string]bool{"include_usage": true},
	})
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(request.BaseURL, "/") + "/chat/completions"
	response, err := c.doOpenAI(ctx, endpoint, request.APIKey, body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnprocessableEntity {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if bytes.Contains(bytes.ToLower(raw), []byte("stream_options")) {
			body, _ = json.Marshal(map[string]any{
				"model": request.Model, "messages": messages, "stream": true,
			})
			response, err = c.doOpenAI(ctx, endpoint, request.APIKey, body)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, upstreamError(response.StatusCode, raw)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		return nil, upstreamError(response.StatusCode, raw)
	}

	chunks := make(chan StreamChunk, 32)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				select {
				case chunks <- StreamChunk{Done: true}:
				case <-ctx.Done():
				}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens        int64 `json:"prompt_tokens"`
					CompletionTokens    int64 `json:"completion_tokens"`
					PromptTokensDetails *struct {
						CachedTokens int64 `json:"cached_tokens"`
					} `json:"prompt_tokens_details"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			event := StreamChunk{}
			if len(chunk.Choices) > 0 {
				event.Delta = chunk.Choices[0].Delta.Content
			}
			if chunk.Usage != nil {
				cached := int64(0)
				if chunk.Usage.PromptTokensDetails != nil {
					cached = chunk.Usage.PromptTokensDetails.CachedTokens
				}
				if cached < 0 || cached > chunk.Usage.PromptTokens {
					cached = 0
				}
				event.Usage = &ChatUsage{
					UncachedInput: chunk.Usage.PromptTokens - cached,
					CacheRead:     cached,
					Output:        chunk.Usage.CompletionTokens,
				}
			}
			if event.Delta != "" || event.Usage != nil {
				select {
				case chunks <- event:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			select {
			case chunks <- StreamChunk{Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return chunks, nil
}

func (c *ProviderClient) doOpenAI(ctx context.Context, endpoint, apiKey string, body []byte) (*http.Response, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	return c.httpClient.Do(httpRequest)
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
