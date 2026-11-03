package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
type OpenAIModelsClient struct{ Client *http.Client }

func (c OpenAIModelsClient) List(ctx context.Context, baseURL, apiKey string) ([]OpenAIModel, error) {
	return c.ListWithHeaders(ctx, baseURL, apiKey, nil)
}
func (c OpenAIModelsClient) ListWithHeaders(ctx context.Context, baseURL, apiKey string, headers map[string]string) ([]OpenAIModel, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key required")
	}
	if err := validURL(baseURL); err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/models")
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || strings.ContainsAny(u.Host, " \r\n") {
		return nil, errors.New("OpenAI models base URL invalid")
	}
	client := sourceClient(c.Client)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("create OpenAI models request")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("OpenAI models source unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI models source returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, errors.New("OpenAI models response invalid")
	}
	var payload struct {
		Data []OpenAIModel `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Data == nil || len(payload.Data) == 0 {
		return nil, errors.New("OpenAI models response invalid")
	}
	seen := map[string]struct{}{}
	for _, m := range payload.Data {
		if m.ID == "" || len(m.ID) > 200 || len(m.OwnedBy) > 200 || m.Object != "model" {
			return nil, errors.New("OpenAI model identity invalid")
		}
		if _, ok := seen[m.ID]; ok {
			return nil, errors.New("OpenAI model identity duplicated")
		}
		seen[m.ID] = struct{}{}
	}
	return payload.Data, nil
}
