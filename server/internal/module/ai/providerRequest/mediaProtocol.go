package providerrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

type MediaUsage struct {
	Known            bool            `json:"known"`
	Basis            string          `json:"basis"`
	ImageCount       int64           `json:"imageCount"`
	DurationSeconds  string          `json:"durationSeconds"`
	InputTokens      *int64          `json:"inputTokens"`
	OutputTokens     *int64          `json:"outputTokens"`
	AudioInputTokens *int64          `json:"audioInputTokens"`
	TextInputTokens  *int64          `json:"textInputTokens"`
	Raw              json.RawMessage `json:"raw"`
}
type ImageRequest struct {
	Config       FrozenConfig
	Prompt       string
	N            int
	Size         string
	Quality      string
	OutputFormat string
	Params       json.RawMessage
}
type ImageEditRequest struct {
	MaxBytes int64
	Config   FrozenConfig
	Prompt   string
	N        int
	Size     string
	Quality  string
	Image    io.Reader
	Filename string
	MIME     string
	Params   json.RawMessage
}
type ImageItem struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"-"`
	Ordinal       int    `json:"ordinal"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
}
type ImageResponse struct {
	Created   int64           `json:"created"`
	Items     []ImageItem     `json:"items"`
	RequestID string          `json:"requestId"`
	Usage     MediaUsage      `json:"usage"`
	Raw       json.RawMessage `json:"raw"`
}
type TranscriptionRequest struct {
	MaxBytes       int64
	Config         FrozenConfig
	Audio          io.Reader
	Filename       string
	MIME           string
	Prompt         string
	Language       string
	ResponseFormat string
	Params         json.RawMessage
}
type TranscriptionResponse struct {
	Text            string          `json:"text"`
	DurationSeconds string          `json:"durationSeconds"`
	Usage           MediaUsage      `json:"usage"`
	RequestID       string          `json:"requestId"`
	Raw             json.RawMessage `json:"raw"`
}
type SpeechRequest struct {
	Config         FrozenConfig
	Input          string
	Voice          string
	ResponseFormat string
	Instructions   string
	Params         json.RawMessage
}
type MediaStream struct {
	Body          io.ReadCloser
	MIME          string
	ContentLength int64
	RequestID     string
	Usage         MediaUsage
}
type VideoRequest struct {
	Config   FrozenConfig
	Prompt   string
	Input    io.Reader
	Filename string
	MIME     string
	Seconds  string
	Size     string
	Params   json.RawMessage
}
type VideoResource struct {
	ID        string          `json:"id"`
	Status    string          `json:"status"`
	Progress  int             `json:"progress"`
	URL       string          `json:"url,omitempty"`
	Error     string          `json:"error,omitempty"`
	RequestID string          `json:"requestId"`
	Usage     MediaUsage      `json:"usage"`
	Raw       json.RawMessage `json:"raw"`
}

type ImagesClient struct{ client *http.Client }

func NewImagesClient(c *http.Client) *ImagesClient { return &ImagesClient{client: c} }

type AudioClient struct{ client *http.Client }

func NewAudioClient(c *http.Client) *AudioClient { return &AudioClient{client: c} }

type VideoClient struct{ client *http.Client }

func NewVideoClient(c *http.Client) *VideoClient { return &VideoClient{client: c} }

func (c *ImagesClient) Generate(ctx context.Context, in ImageRequest) (ImageResponse, error) {
	var out ImageResponse
	if !validMediaConfig(in.Config) || strings.TrimSpace(in.Prompt) == "" || in.N < 1 || in.N > 10 {
		return out, ErrRequestInvalid
	}
	body := map[string]any{"model": in.Config.Model, "prompt": in.Prompt, "n": in.N}
	if in.Size != "" {
		body["size"] = in.Size
	}
	if in.Quality != "" {
		body["quality"] = in.Quality
	}
	if in.OutputFormat != "" {
		body["output_format"] = in.OutputFormat
	}
	if len(in.Params) > 0 {
		var extra map[string]json.RawMessage
		if !jsonObject(in.Params) || json.Unmarshal(in.Params, &extra) != nil {
			return out, ErrRequestInvalid
		}
		for k, v := range extra {
			if _, ok := body[k]; ok {
				return out, ErrRequestInvalid
			}
			body[k] = v
		}
	}
	raw, id, status, err := mediaJSON(ctx, c.client, in.Config, http.MethodPost, "images/generations", body)
	if err != nil {
		return out, err
	}
	out.RequestID = id
	out.Raw = append(json.RawMessage(nil), raw...)
	var wire struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &wire) != nil || len(wire.Data) == 0 || len(wire.Data) > in.N {
		return out, requestFailure("protocol", status, id, true, ErrResponseInvalid)
	}
	out.Created = wire.Created
	out.Items = make([]ImageItem, len(wire.Data))
	for i, v := range wire.Data {
		out.Items[i] = ImageItem{URL: v.URL, B64JSON: v.B64JSON, Ordinal: i, RevisedPrompt: v.RevisedPrompt}
	}
	out.Usage = MediaUsage{Known: true, Basis: "image", ImageCount: int64(len(out.Items))}
	return out, nil
}

func (c *ImagesClient) Edit(ctx context.Context, in ImageEditRequest) (ImageResponse, error) {
	var out ImageResponse
	if in.Image == nil || !validMediaConfig(in.Config) || strings.TrimSpace(in.Prompt) == "" || in.N < 1 || in.N > 10 {
		return out, ErrRequestInvalid
	}
	work, cancel := context.WithTimeout(ctx, in.Config.Timeout)
	defer cancel()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	name := in.Filename
	if name == "" {
		name = "image.png"
	}
	part, err := mediaFormFile(mw, "image", name, in.MIME)
	if err != nil {
		return out, ErrRequestInvalid
	}
	if err = copyMediaBounded(part, in.Image, mediaInputLimit(in.MaxBytes, 10<<20)); err != nil {
		return out, ErrRequestInvalid
	}
	if err = mw.WriteField("model", in.Config.Model); err == nil {
		err = mw.WriteField("prompt", in.Prompt)
	}
	if err == nil {
		err = mw.WriteField("n", fmt.Sprintf("%d", in.N))
	}
	if err == nil && in.Size != "" {
		err = mw.WriteField("size", in.Size)
	}
	if err == nil && in.Quality != "" {
		err = mw.WriteField("quality", in.Quality)
	}
	if err == nil {
		err = writeMediaParams(mw, in.Params, []string{"model", "prompt", "n", "size", "quality", "image"})
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		return out, ErrRequestInvalid
	}
	target, err := endpoint(in.Config.BaseURL, "images/edits")
	if err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(work, http.MethodPost, target, &body)
	if err != nil || !applyMediaHeaders(req, in.Config) {
		return out, ErrRequestInvalid
	}
	req.Header.Set("Authorization", "Bearer "+in.Config.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := cloneMediaClient(c.client).Do(req)
	if err != nil {
		return out, requestFailure("transport", 0, "", true, err)
	}
	defer resp.Body.Close()
	id := safeRequestID(resp.Header.Get("x-request-id"), in.Config.APIKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, requestFailure("http", resp.StatusCode, id, resp.StatusCode >= 500, ErrUpstreamRejected)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil || !json.Valid(raw) {
		return out, requestFailure("protocol", resp.StatusCode, id, true, ErrResponseInvalid)
	}
	var wire struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &wire) != nil || len(wire.Data) == 0 || len(wire.Data) > in.N {
		return out, requestFailure("protocol", resp.StatusCode, id, true, ErrResponseInvalid)
	}
	out.Created, out.RequestID, out.Raw = wire.Created, id, raw
	out.Items = make([]ImageItem, len(wire.Data))
	for i, item := range wire.Data {
		out.Items[i] = ImageItem{URL: item.URL, B64JSON: item.B64JSON, Ordinal: i, RevisedPrompt: item.RevisedPrompt}
	}
	out.Usage = MediaUsage{Known: true, Basis: "image", ImageCount: int64(len(out.Items))}
	return out, nil
}

func (c *AudioClient) Transcribe(ctx context.Context, in TranscriptionRequest) (TranscriptionResponse, error) {
	var out TranscriptionResponse
	if in.Audio == nil || !validMediaConfig(in.Config) {
		return out, ErrRequestInvalid
	}
	target, err := endpoint(in.Config.BaseURL, "audio/transcriptions")
	if err != nil {
		return out, err
	}
	work, cancel := context.WithTimeout(ctx, in.Config.Timeout)
	defer cancel()
	pr, pw := io.Pipe()
	defer pr.Close()
	mw := multipart.NewWriter(pw)
	done := make(chan error, 1)
	go func() {
		defer close(done)

		fn := in.Filename
		if fn == "" {
			fn = "audio.bin"
		}
		part, e := mediaFormFile(mw, "file", fn, in.MIME)
		if e == nil {
			e = copyMediaBounded(part, in.Audio, mediaInputLimit(in.MaxBytes, 100<<20))
		}
		if e == nil {
			e = mw.WriteField("model", in.Config.Model)
			if in.ResponseFormat != "" {
				e = mw.WriteField("response_format", in.ResponseFormat)
			}
			if e == nil && in.Prompt != "" {
				e = mw.WriteField("prompt", in.Prompt)
			}
			if e == nil && in.Language != "" {
				e = mw.WriteField("language", in.Language)
			}
		}
		if e == nil {
			e = writeMediaParams(mw, in.Params, []string{"model", "file", "prompt", "language", "response_format"})
		}
		if e == nil {
			e = mw.Close()
		}
		_ = pw.CloseWithError(e)
		done <- e
	}()
	req, e := http.NewRequestWithContext(work, http.MethodPost, target, pr)
	if e != nil {
		return out, ErrRequestInvalid
	}
	req.GetBody = nil
	if !applyMediaHeaders(req, in.Config) {
		return out, ErrRequestInvalid
	}
	req.Header.Set("Authorization", "Bearer "+in.Config.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	client := cloneMediaClient(c.client)
	resp, e := client.Do(req)
	if e != nil {
		return out, requestFailure("transport", 0, "", true, e)
	}
	defer resp.Body.Close()
	id := safeRequestID(resp.Header.Get("x-request-id"), in.Config.APIKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, requestFailure("http", resp.StatusCode, id, resp.StatusCode >= 500, ErrUpstreamRejected)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if e != nil || !json.Valid(raw) {
		return out, requestFailure("protocol", resp.StatusCode, id, true, ErrResponseInvalid)
	}
	if e := <-done; e != nil {
		return out, requestFailure("transport", resp.StatusCode, id, true, e)
	}
	out.RequestID = id
	out.Raw = raw
	var w struct {
		Text     string          `json:"text"`
		Duration json.Number     `json:"duration"`
		Usage    json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &w) != nil {
		return out, requestFailure("protocol", resp.StatusCode, id, true, ErrResponseInvalid)
	}
	out.Text = w.Text
	if w.Duration != "" {
		out.DurationSeconds = w.Duration.String()
	}
	out.Usage = parseAudioUsage(w.Usage, out.DurationSeconds)
	return out, nil
}

func (c *AudioClient) Synthesize(ctx context.Context, in SpeechRequest) (MediaStream, error) {
	if strings.TrimSpace(in.Input) == "" || !validMediaConfig(in.Config) || strings.TrimSpace(in.Voice) == "" {
		return MediaStream{}, ErrRequestInvalid
	}
	body := map[string]any{"model": in.Config.Model, "input": in.Input, "voice": in.Voice}
	if in.ResponseFormat != "" {
		body["response_format"] = in.ResponseFormat
	}
	if in.Instructions != "" {
		body["instructions"] = in.Instructions
	}
	if len(in.Params) > 0 {
		var extra map[string]json.RawMessage
		if !jsonObject(in.Params) || json.Unmarshal(in.Params, &extra) != nil {
			return MediaStream{}, ErrRequestInvalid
		}
		for k, v := range extra {
			switch k {
			case "model", "input", "voice", "response_format", "instructions":
				return MediaStream{}, ErrRequestInvalid
			}
			body[k] = v
		}
	}
	var format string
	if raw, ok := body["stream_format"].(json.RawMessage); ok {
		if json.Unmarshal(raw, &format) != nil || (format != "audio" && format != "sse") {
			return MediaStream{}, ErrRequestInvalid
		}
	}
	if format == "sse" && (in.Config.Model == "tts-1" || in.Config.Model == "tts-1-hd") {
		return MediaStream{}, ErrRequestInvalid
	}
	stream, err := c.stream(ctx, in.Config, "audio/speech", body)
	if err != nil {
		return stream, err
	}
	if format == "sse" {
		if !strings.HasPrefix(strings.ToLower(stream.MIME), "text/event-stream") {
			stream.Body.Close()
			return MediaStream{}, requestFailure("protocol", 200, stream.RequestID, true, ErrResponseInvalid)
		}
		stream.Body = newSpeechSSEBody(stream.Body)
		stream.MIME = speechMIME(in.ResponseFormat)
		stream.ContentLength = -1
	}
	return stream, nil
}
func (c *AudioClient) stream(ctx context.Context, cfg FrozenConfig, path string, body map[string]any) (MediaStream, error) {
	var out MediaStream
	raw, e := json.Marshal(body)
	if e != nil {
		return out, e
	}
	target, e := endpoint(cfg.BaseURL, path)
	if e != nil {
		return out, e
	}
	work, cancel := context.WithTimeout(ctx, cfg.Timeout)
	req, e := http.NewRequestWithContext(work, http.MethodPost, target, strings.NewReader(string(raw)))
	if e != nil {
		cancel()
		return out, e
	}
	req.GetBody = nil
	if !applyMediaHeaders(req, cfg) {
		cancel()
		return out, ErrRequestInvalid
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, e := cloneMediaClient(c.client).Do(req)
	if e != nil {
		cancel()
		return out, requestFailure("transport", 0, "", true, e)
	}
	id := safeRequestID(resp.Header.Get("x-request-id"), cfg.APIKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		cancel()
		return out, requestFailure("http", resp.StatusCode, id, resp.StatusCode >= 500, ErrUpstreamRejected)
	}
	out = MediaStream{Body: &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}, MIME: resp.Header.Get("content-type"), ContentLength: resp.ContentLength, RequestID: id}
	return out, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error { e := c.ReadCloser.Close(); c.cancel(); return e }

func (c *VideoClient) Submit(ctx context.Context, in VideoRequest) (VideoResource, error) {
	var out VideoResource
	if !validMediaConfig(in.Config) || strings.TrimSpace(in.Prompt) == "" {
		return out, ErrRequestInvalid
	}
	body := map[string]any{"model": in.Config.Model, "prompt": in.Prompt}
	if in.Seconds != "" {
		body["seconds"] = in.Seconds
	}
	if in.Size != "" {
		body["size"] = in.Size
	}
	raw, id, status, e := mediaJSON(ctx, c.client, in.Config, http.MethodPost, "videos", body)
	if e != nil {
		return out, e
	}
	out.RequestID = id
	out.Raw = raw
	if json.Unmarshal(raw, &out) != nil || out.ID == "" {
		return out, requestFailure("protocol", status, id, true, ErrResponseInvalid)
	}
	return out, nil
}
func (c *VideoClient) Poll(ctx context.Context, cfg FrozenConfig, taskID string) (VideoResource, error) {
	if !validMediaConfig(cfg) || strings.TrimSpace(taskID) == "" {
		return VideoResource{}, ErrRequestInvalid
	}
	return c.get(ctx, cfg, "videos/"+url.PathEscape(taskID))
}
func (c *VideoClient) Content(ctx context.Context, cfg FrozenConfig, taskID string) (MediaStream, error) {
	if !validMediaConfig(cfg) || strings.TrimSpace(taskID) == "" {
		return MediaStream{}, ErrRequestInvalid
	}
	return c.streamGet(ctx, cfg, "videos/"+url.PathEscape(taskID)+"/content")
}
func (c *VideoClient) get(ctx context.Context, cfg FrozenConfig, path string) (VideoResource, error) {
	var out VideoResource
	raw, id, status, e := mediaJSON(ctx, c.client, cfg, http.MethodGet, path, nil)
	if e != nil {
		return out, e
	}
	out.RequestID = id
	out.Raw = raw
	if json.Unmarshal(raw, &out) != nil {
		return out, requestFailure("protocol", status, id, true, ErrResponseInvalid)
	}
	return out, nil
}
func (c *VideoClient) streamGet(ctx context.Context, cfg FrozenConfig, path string) (MediaStream, error) {
	target, e := endpoint(cfg.BaseURL, path)
	if e != nil {
		return MediaStream{}, e
	}
	work, cancel := context.WithTimeout(ctx, cfg.Timeout)
	req, e := http.NewRequestWithContext(work, http.MethodGet, target, nil)
	if e != nil {
		cancel()
		return MediaStream{}, e
	}
	if !applyMediaHeaders(req, cfg) {
		cancel()
		return MediaStream{}, ErrRequestInvalid
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, e := cloneMediaClient(c.client).Do(req)
	if e != nil {
		cancel()
		return MediaStream{}, requestFailure("transport", 0, "", true, e)
	}
	id := safeRequestID(resp.Header.Get("x-request-id"), cfg.APIKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		cancel()
		return MediaStream{}, requestFailure("http", resp.StatusCode, id, resp.StatusCode >= 500, ErrUpstreamRejected)
	}
	return MediaStream{Body: &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}, MIME: resp.Header.Get("content-type"), ContentLength: resp.ContentLength, RequestID: id}, nil
}

func cloneMediaClient(in *http.Client) *http.Client {
	out := http.Client{}
	if in != nil {
		out = *in
	}
	out.Jar = nil
	out.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &out
}
func mediaJSON(ctx context.Context, client *http.Client, cfg FrozenConfig, method, path string, body any) ([]byte, string, int, error) {
	target, e := endpoint(cfg.BaseURL, path)
	if e != nil {
		return nil, "", 0, e
	}
	var rd io.Reader
	if body != nil {
		raw, e := json.Marshal(body)
		if e != nil {
			return nil, "", 0, e
		}
		rd = strings.NewReader(string(raw))
	}
	work, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	req, e := http.NewRequestWithContext(work, method, target, rd)
	if e != nil {
		return nil, "", 0, e
	}
	req.GetBody = nil
	if !applyMediaHeaders(req, cfg) {
		return nil, "", 0, ErrRequestInvalid
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := cloneMediaClient(client).Do(req)
	if e != nil {
		return nil, "", 0, requestFailure("transport", 0, "", true, e)
	}
	defer resp.Body.Close()
	id := safeRequestID(resp.Header.Get("x-request-id"), cfg.APIKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, id, resp.StatusCode, requestFailure("http", resp.StatusCode, id, resp.StatusCode >= 500, ErrUpstreamRejected)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if e != nil || !json.Valid(raw) {
		return nil, id, resp.StatusCode, requestFailure("protocol", resp.StatusCode, id, true, ErrResponseInvalid)
	}
	return raw, id, resp.StatusCode, nil
}

func validMediaConfig(cfg FrozenConfig) bool {
	return strings.TrimSpace(cfg.APIKey) != "" && validHeaderValue(cfg.APIKey) && strings.TrimSpace(cfg.Model) != "" && cfg.Timeout > 0
}

func applyMediaHeaders(req *http.Request, cfg FrozenConfig) bool {
	for k, v := range cfg.Headers {
		if !validHeaderName(k) || !validHeaderValue(v) || reservedHeader(k) {
			return false
		}
		req.Header.Set(k, v)
	}
	return true
}

func mediaFormFile(w *multipart.Writer, field, name, contentType string) (io.Writer, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil || !validHeaderValue(contentType) {
		return nil, ErrRequestInvalid
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": field, "filename": name}))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}
func copyMediaBounded(dst io.Writer, src io.Reader, maximum int64) error {
	n, err := io.Copy(dst, io.LimitReader(src, maximum+1))
	if err != nil {
		return err
	}
	if n == 0 || n > maximum {
		return ErrRequestInvalid
	}
	return nil
}
func writeMediaParams(w *multipart.Writer, raw json.RawMessage, reserved []string) error {
	if len(raw) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if !jsonObject(raw) || json.Unmarshal(raw, &fields) != nil {
		return ErrRequestInvalid
	}
	for k, v := range fields {
		for _, r := range reserved {
			if k == r {
				return ErrRequestInvalid
			}
		}
		var str string
		if json.Unmarshal(v, &str) != nil {
			str = string(v)
		}
		if err := w.WriteField(k, str); err != nil {
			return err
		}
	}
	return nil
}

func mediaInputLimit(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}
