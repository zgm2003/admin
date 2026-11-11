package providermodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
)

const testPrompt = "Reply with OK only."

type PreviewModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type ModelPreviewResult struct {
	OK        bool           `json:"ok"`
	Models    []PreviewModel `json:"models"`
	Error     string         `json:"error"`
	CheckedAt time.Time      `json:"checkedAt"`
}

type TestResult struct {
	OK          bool      `json:"ok"`
	Model       string    `json:"model"`
	DisplayName string    `json:"displayName"`
	HTTPStatus  int       `json:"httpStatus"`
	LatencyMS   int64     `json:"latencyMs"`
	Reply       string    `json:"reply"`
	Error       string    `json:"error"`
	CheckedAt   time.Time `json:"checkedAt"`
}

func (s *Service) PreviewModels(ctx context.Context, input TestDraftInput) (ModelPreviewResult, error) {
	provider, apiKey, err := s.resolveDraftProvider(ctx, input)
	if err != nil {
		return ModelPreviewResult{}, err
	}
	models, err := s.listUpstreamModels(ctx, provider, apiKey)
	if err != nil {
		return ModelPreviewResult{
			OK:        false,
			Error:     "拉取模型失败: " + sanitizeTestText(testErrorDetail(err), apiKey, 300),
			CheckedAt: s.checkedAt(),
		}, nil
	}
	if len(models) == 0 {
		return ModelPreviewResult{
			OK: false, Error: "上游没有返回模型", CheckedAt: s.checkedAt(),
		}, nil
	}
	items := make([]PreviewModel, 0, len(models))
	for _, model := range models {
		items = append(items, PreviewModel{ID: model.ID, DisplayName: model.DisplayName})
	}
	return ModelPreviewResult{OK: true, Models: items, CheckedAt: s.checkedAt()}, nil
}

func (s *Service) TestDraft(ctx context.Context, input TestDraftInput) (TestResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return TestResult{}, invalid(fmt.Errorf("model is required"))
	}
	provider, apiKey, err := s.resolveDraftProvider(ctx, input)
	if err != nil {
		return TestResult{}, err
	}
	return s.probeHello(ctx, provider, Model{
		UpstreamModel: model, DisplayName: model,
	}, apiKey), nil
}

func (s *Service) resolveDraftProvider(
	ctx context.Context,
	input TestDraftInput,
) (aiprovider.Model, string, error) {
	if s == nil || s.providerSource == nil || s.keys == nil {
		return aiprovider.Model{}, "", dependency(fmt.Errorf("provider test service is not configured"))
	}
	if !input.Protocol.Valid() || strings.TrimSpace(input.BaseURL) == "" {
		return aiprovider.Model{}, "", invalid(fmt.Errorf("protocol or baseUrl is invalid"))
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		if input.ProviderID == nil {
			return aiprovider.Model{}, "", invalid(fmt.Errorf("apiKey is required"))
		}
		saved, err := s.providerSource.FindByID(ctx, *input.ProviderID)
		if err != nil {
			return aiprovider.Model{}, "", mapError(err)
		}
		apiKey, err = secretkey.DecryptProviderValue(
			s.keys.ProviderEncryptionKey(), saved.APIKeyCiphertext,
		)
		if err != nil {
			return aiprovider.Model{}, "", dependency(fmt.Errorf("decrypt provider API key: %w", err))
		}
	}
	timeout := input.TimeoutMS
	if timeout < 1000 || timeout > 30000 {
		timeout = 15000
	}
	return aiprovider.Model{
		Protocol: input.Protocol, BaseURL: strings.TrimSpace(input.BaseURL), TimeoutMS: timeout,
	}, apiKey, nil
}

func testErrorDetail(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) && appErr.Cause != nil {
		return appErr.Cause.Error()
	}
	return err.Error()
}

func (s *Service) probeHello(
	ctx context.Context,
	provider aiprovider.Model,
	model Model,
	apiKey string,
) TestResult {
	result := TestResult{
		Model: model.UpstreamModel, DisplayName: model.DisplayName, CheckedAt: s.checkedAt(),
	}
	protocol := provider.Protocol
	if !protocol.Valid() {
		result.Error = "不支持的供应商协议"
		return result
	}
	payload, err := helloPayload(protocol, model.UpstreamModel)
	if err != nil {
		result.Error = "构造测试请求失败: " + err.Error()
		return result
	}
	endpoint, err := helloEndpoint(protocol, provider.BaseURL, model.UpstreamModel)
	if err != nil {
		result.Error = "构造测试地址失败: " + err.Error()
		return result
	}
	requestCtx, cancel := context.WithTimeout(ctx, testTimeout(provider.TimeoutMS))
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestCtx, http.MethodPost, endpoint, strings.NewReader(string(payload)),
	)
	if err != nil {
		result.Error = "构造测试请求失败: " + sanitizeTestText(err.Error(), apiKey, 300)
		return result
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "admin-ai-provider-test/1.0")
	switch protocol {
	case aiprovider.ProtocolOpenAI, aiprovider.ProtocolOpenAIResponses:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	case aiprovider.ProtocolAnthropic:
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	case aiprovider.ProtocolGemini:
		request.Header.Set("x-goog-api-key", apiKey)
	}

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	startedAt := time.Now()
	response, err := client.Do(request)
	result.LatencyMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			result.Error = "请求超时"
			return result
		}
		result.Error = "请求失败: " + sanitizeTestText(err.Error(), apiKey, 300)
		return result
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	result.HTTPStatus = response.StatusCode
	if readErr != nil {
		result.Error = "读取上游响应失败: " + sanitizeTestText(readErr.Error(), apiKey, 300)
		return result
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result.Error = upstreamTestError(response.StatusCode, body, apiKey)
		return result
	}
	reply, err := parseHelloReply(protocol, body)
	if err != nil {
		result.Error = "解析上游响应失败: " + err.Error()
		return result
	}
	reply = sanitizeTestText(reply, apiKey, 500)
	if reply == "" {
		result.Error = "上游返回空回复"
		return result
	}
	result.OK = true
	result.Reply = reply
	return result
}

func (s *Service) checkedAt() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func testTimeout(milliseconds int32) time.Duration {
	if milliseconds >= 1000 && milliseconds <= 30000 {
		return time.Duration(milliseconds) * time.Millisecond
	}
	return 20 * time.Second
}

func helloEndpoint(protocol aiprovider.Protocol, baseURL, model string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "", fmt.Errorf("base URL is empty")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL is invalid")
	}
	switch protocol {
	case aiprovider.ProtocolOpenAI:
		return base + "/chat/completions", nil
	case aiprovider.ProtocolOpenAIResponses:
		return base + "/responses", nil
	case aiprovider.ProtocolAnthropic:
		return base + "/messages", nil
	case aiprovider.ProtocolGemini:
		model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
		if model == "" {
			return "", fmt.Errorf("model is empty")
		}
		return base + "/models/" + url.PathEscape(model) + ":generateContent", nil
	default:
		return "", fmt.Errorf("unsupported provider protocol")
	}
}

func helloPayload(protocol aiprovider.Protocol, model string) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is empty")
	}
	var payload any
	switch protocol {
	case aiprovider.ProtocolOpenAI:
		payload = map[string]any{
			"model":    model,
			"messages": []map[string]string{{"role": "user", "content": testPrompt}},
			"stream":   false,
		}
	case aiprovider.ProtocolOpenAIResponses:
		payload = map[string]any{
			"model": model,
			"input": testPrompt,
		}
	case aiprovider.ProtocolAnthropic:
		payload = map[string]any{
			"model":      model,
			"max_tokens": 32,
			"messages":   []map[string]string{{"role": "user", "content": testPrompt}},
		}
	case aiprovider.ProtocolGemini:
		payload = map[string]any{
			"contents": []map[string]any{{
				"role":  "user",
				"parts": []map[string]string{{"text": testPrompt}},
			}},
			"generationConfig": map[string]any{
				"temperature":     0,
				"maxOutputTokens": 32,
			},
		}
	default:
		return nil, fmt.Errorf("unsupported provider protocol")
	}
	return json.Marshal(payload)
}

func parseHelloReply(protocol aiprovider.Protocol, body []byte) (string, error) {
	switch protocol {
	case aiprovider.ProtocolOpenAI:
		var payload struct {
			Choices []struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		if len(payload.Choices) == 0 {
			return "", nil
		}
		return openAIContentText(payload.Choices[0].Message.Content), nil
	case aiprovider.ProtocolOpenAIResponses:
		var payload struct {
			OutputText string `json:"output_text"`
			Output     []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		if strings.TrimSpace(payload.OutputText) != "" {
			return payload.OutputText, nil
		}
		parts := make([]string, 0, len(payload.Output))
		for _, item := range payload.Output {
			for _, content := range item.Content {
				if strings.TrimSpace(content.Text) != "" {
					parts = append(parts, content.Text)
				}
			}
		}
		return strings.Join(parts, "\n"), nil
	case aiprovider.ProtocolAnthropic:
		var payload struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		parts := make([]string, 0, len(payload.Content))
		for _, item := range payload.Content {
			if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.Join(parts, "\n"), nil
	case aiprovider.ProtocolGemini:
		var payload struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		parts := make([]string, 0, len(payload.Candidates))
		for _, candidate := range payload.Candidates {
			for _, part := range candidate.Content.Parts {
				if strings.TrimSpace(part.Text) != "" {
					parts = append(parts, part.Text)
				}
			}
		}
		return strings.Join(parts, "\n"), nil
	default:
		return "", fmt.Errorf("unsupported provider protocol")
	}
}

func openAIContentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func upstreamTestError(status int, body []byte, apiKey string) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	message := ""
	if err := json.Unmarshal(body, &payload); err == nil && len(payload.Error) > 0 {
		var object struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(payload.Error, &object); err == nil {
			message = object.Message
		}
		if message == "" {
			var text string
			if err := json.Unmarshal(payload.Error, &text); err == nil {
				message = text
			}
		}
	}
	if message == "" {
		message = string(body)
	}
	message = sanitizeTestText(message, apiKey, 400)
	if message == "" {
		return fmt.Sprintf("上游返回 HTTP %d", status)
	}
	return fmt.Sprintf("上游返回 HTTP %d: %s", status, message)
}

func sanitizeTestText(value, apiKey string, limit int) string {
	value = strings.TrimSpace(value)
	if apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "[redacted]")
	}
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return value
}
