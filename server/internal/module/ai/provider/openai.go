package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const OpenAIPricingURL = "https://developers.openai.com/api/docs/pricing.md"
const OpenAIImagePricingURL = "https://developers.openai.com/api/docs/guides/image-generation.md"
const OpenAITranscriptionPricingURL = "https://developers.openai.com/api/docs/guides/speech-to-text.md"
const OpenAISpeechPricingURL = "https://developers.openai.com/api/docs/guides/text-to-speech.md"
const OpenAIModelsURL = "https://developers.openai.com/api/reference/resources/models/methods/list"

type OpenAIRate struct {
	ModelKey, Operation, UsageKey, Unit, UnitQuantity, SourceUnitPrice string
	Selector                                                           []byte
}

func (s OpenAIPricingSnapshot) ForModel(vendor, baseURL, modelKey string) (OpenAIPricingSnapshot, error) {
	u, err := url.Parse(baseURL)
	if err != nil || vendor != "openai" || u.Scheme != "https" || u.Host != "api.openai.com" || strings.TrimRight(u.Path, "/") != "/v1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return OpenAIPricingSnapshot{}, errors.New("official OpenAI price cannot apply to compatible provider")
	}
	out := s
	out.Rates = nil
	for _, r := range s.Rates {
		if r.ModelKey == modelKey {
			out.Rates = append(out.Rates, r)
		}
	}
	if len(out.Rates) == 0 {
		return OpenAIPricingSnapshot{}, errors.New("official model price not found")
	}
	return out, nil
}

type OpenAIPricingSnapshot struct {
	Digest   string
	Payload  []byte
	SyncedAt time.Time
	Rates    []OpenAIRate
}
type OpenAISource struct{ Client *http.Client }

func (s OpenAISource) FetchPricing(ctx context.Context) (OpenAIPricingSnapshot, error) {
	client := sourceClient(s.Client)
	var body []byte
	for _, sourceURL := range []string{OpenAIPricingURL, OpenAIImagePricingURL, OpenAITranscriptionPricingURL, OpenAISpeechPricingURL} {
		part, err := fetchPricingDocument(ctx, client, sourceURL)
		if err != nil {
			return OpenAIPricingSnapshot{}, err
		}
		body = append(body, []byte("\n\n<!-- SOURCE: "+sourceURL+" -->\n\n")...)
		body = append(body, part...)
	}
	rates, err := ParsePricingMarkdown(body)
	if err != nil {
		return OpenAIPricingSnapshot{}, err
	}
	sum := sha256.Sum256(body)
	return OpenAIPricingSnapshot{Digest: hex.EncodeToString(sum[:]), Payload: body, SyncedAt: time.Now().UTC(), Rates: rates}, nil
}

func fetchPricingDocument(ctx context.Context, client *http.Client, sourceURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, errors.New("create OpenAI pricing request")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("OpenAI pricing source unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI pricing source returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, errors.New("read OpenAI pricing source")
	}
	return body, nil
}
func ParsePricingMarkdown(body []byte) ([]OpenAIRate, error) {
	document := string(body)
	text := document
	start := strings.Index(text, "### Standard pricing data")
	if start < 0 {
		return nil, errors.New("OpenAI standard pricing table missing")
	}
	end := strings.Index(text[start+len("### Standard pricing data"):], "### ")
	if end > 0 {
		text = text[start : start+len("### Standard pricing data")+end]
	}
	var out []OpenAIRate
	var headers []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		if strings.Contains(strings.ToLower(line), "| model |") {
			headers = parts
			continue
		}
		model := strings.TrimSpace(parts[1])
		if model == "" || model == "Model" || strings.HasPrefix(model, "---") {
			continue
		}
		model = strings.TrimSpace(strings.Split(model, " ")[0])
		if len(headers) == 0 || len(parts) != len(headers) {
			return nil, errors.New("OpenAI standard pricing table shape changed")
		}
		for i := 2; i < len(parts) && i < len(headers); i++ {
			h := strings.ToLower(strings.TrimSpace(headers[i]))
			key := ""
			switch {
			case strings.Contains(h, "input") && !strings.Contains(h, "cached"):
				key = "input"
			case strings.Contains(h, "cached input"):
				key = "cacheInput"
			case strings.Contains(h, "output"):
				key = "output"
			case strings.Contains(h, "30m write"):
				key = "cacheWrite30m"
			case strings.Contains(h, "12h write"):
				key = "cacheWrite12h"
			}
			if key == "" {
				if strings.HasPrefix(strings.TrimSpace(parts[i]), "$") {
					return nil, errors.New("OpenAI standard pricing dimension unsupported")
				}
				continue
			}
			price := strings.TrimSpace(parts[i])
			if price == "-" {
				continue
			}
			if !strings.HasPrefix(price, "$") {
				return nil, errors.New("OpenAI standard pricing contains unsupported dollar price")
			}
			price = strings.TrimPrefix(price, "$")
			if !regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,12})?$`).MatchString(price) {
				return nil, errors.New("OpenAI standard pricing contains malformed dollar price")
			}
			selector := `{}`
			if strings.Contains(h, "short context") {
				selector = `{"context":"short"}`
			}
			if strings.Contains(h, "long context") {
				selector = `{"context":"long"}`
			}
			out = append(out, OpenAIRate{ModelKey: model, Operation: "text.generate", UsageKey: key, Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: price, Selector: []byte(selector)})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("OpenAI standard pricing has no token rows")
	}
	imageRates, err := parseImagePricing(document)
	if err != nil {
		return nil, err
	}
	audioRates, err := parseAudioPricing(document)
	if err != nil {
		return nil, err
	}
	out = append(out, imageRates...)
	out = append(out, audioRates...)
	// Token-level image estimates remain in the immutable source payload.
	// This release charges image output exclusively by quality/size per image.
	return out, nil
}

var htmlCellPattern = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
var htmlRowPattern = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
var htmlTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)

func htmlCellText(raw string) string {
	raw = htmlTagPattern.ReplaceAllString(raw, " ")
	raw = strings.ReplaceAll(raw, "&times;", "x")
	raw = strings.ReplaceAll(raw, "×", "x")
	return strings.Join(strings.Fields(raw), " ")
}

func parseImagePricing(text string) ([]OpenAIRate, error) {
	marker := strings.Index(text, "per-image output pricing table")
	if marker < 0 {
		return nil, nil
	}
	tableEnd := strings.Index(text[marker:], "</table>")
	if tableEnd < 0 {
		return nil, errors.New("OpenAI image pricing table shape changed")
	}
	table := text[marker : marker+tableEnd]
	model := ""
	out := make([]OpenAIRate, 0, 24)
	for _, rawRow := range htmlRowPattern.FindAllStringSubmatch(table, -1) {
		cells := htmlCellPattern.FindAllStringSubmatch(rawRow[1], -1)
		values := make([]string, 0, len(cells))
		for _, cell := range cells {
			values = append(values, htmlCellText(cell[1]))
		}
		if len(values) == 0 || strings.EqualFold(values[0], "Model") {
			continue
		}
		if len(values) == 5 {
			model = normalizeImageModel(values[0])
			values = values[1:]
		}
		if model == "" || len(values) != 4 {
			continue
		}
		quality := strings.ToLower(values[0])
		if quality != "low" && quality != "medium" && quality != "high" {
			return nil, errors.New("OpenAI image pricing quality unsupported")
		}
		for _, sizePrice := range []struct {
			size, price string
		}{
			{"1024x1024", values[1]}, {"1024x1536", values[2]}, {"1536x1024", values[3]},
		} {
			price, ok := dollarNumber(sizePrice.price)
			if !ok {
				return nil, errors.New("OpenAI image pricing value malformed")
			}
			out = append(out, OpenAIRate{ModelKey: model, Operation: "image.generate", UsageKey: "output", Unit: "image", UnitQuantity: "1", SourceUnitPrice: price, Selector: []byte(fmt.Sprintf(`{"billing":"image","quality":%q,"size":%q}`, quality, sizePrice.size))})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("OpenAI image pricing has no per-image rows")
	}
	return out, nil
}

func normalizeImageModel(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.TrimPrefix(v, "gpt ")
	v = strings.ReplaceAll(v, " ", "-")
	return "gpt-" + v
}

func dollarNumber(value string) (string, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "$"))
	if slash := strings.Index(value, " "); slash >= 0 {
		value = value[:slash]
	}
	return value, regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,12})?$`).MatchString(value)
}

func parseAudioPricing(text string) ([]OpenAIRate, error) {
	out := make([]OpenAIRate, 0, 16)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}
		model, modality := strings.TrimSpace(parts[1]), strings.ToLower(strings.TrimSpace(parts[2]))
		if model == "Model" || strings.Contains(model, "---") {
			continue
		}
		input, output, cached := strings.TrimSpace(parts[3]), strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5])
		if strings.HasPrefix(model, "gpt-audio") || strings.HasPrefix(model, "gpt-realtime") || strings.Contains(modality, "live") {
			continue
		}
		audioRow := strings.Contains(modality, "transcription") || strings.Contains(modality, "translation") || strings.Contains(model, "tts") || strings.HasPrefix(model, "tts-") || strings.HasPrefix(model, "gpt-audio") || strings.HasPrefix(model, "gpt-realtime")
		if !audioRow {
			continue
		}

		if strings.Contains(modality, "transcription") {
			// The final column is an estimated per-minute cost for token-priced
			// models. It is an alternative basis, never another additive item.
			hasInput, hasOutput := input != "-", output != "-"
			if hasInput || hasOutput {
				inputPrice, inputOK := dollarNumber(input)
				outputPrice, outputOK := dollarNumber(output)
				if !hasInput || !hasOutput || !inputOK || !outputOK {
					return nil, fmt.Errorf("OpenAI transcription token pricing incomplete for %s", model)
				}
				for _, item := range []struct{ key, price string }{{"input", inputPrice}, {"output", outputPrice}} {
					out = append(out, OpenAIRate{ModelKey: model, Operation: "audio.transcribe", UsageKey: item.key, Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: item.price, Selector: []byte(`{"billing":"token"}`)})
				}
			} else {
				if !strings.HasSuffix(cached, "/ minute") {
					return nil, fmt.Errorf("OpenAI audio pricing unit unsupported for %s", model)
				}
				price, ok := dollarNumber(strings.TrimSpace(strings.TrimSuffix(cached, "/ minute")))
				if !ok {
					return nil, fmt.Errorf("OpenAI audio pricing value malformed for %s", model)
				}
				out = append(out, OpenAIRate{ModelKey: model, Operation: "audio.transcribe", UsageKey: "audio", Unit: "second", UnitQuantity: "60", SourceUnitPrice: price, Selector: []byte(`{"billing":"audio"}`)})
			}
		}

		if model == "gpt-4o-mini-tts" {
			key, raw := "input", input
			if modality == "audio" {
				key, raw = "output", output
			}
			if price, ok := dollarNumber(raw); ok {
				out = append(out, OpenAIRate{ModelKey: model, Operation: "audio.speech", UsageKey: key, Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: price, Selector: []byte(fmt.Sprintf(`{"modality":%q}`, modality))})
			}
		} else if strings.HasPrefix(model, "tts-") {
			if !strings.Contains(input, "/ 1M characters") || modality != "text" {
				return nil, errors.New("OpenAI speech pricing unit unsupported")
			}
			if price, ok := dollarNumber(input); ok {
				out = append(out, OpenAIRate{ModelKey: model, Operation: "audio.speech", UsageKey: "input", Unit: "character", UnitQuantity: "1000000", SourceUnitPrice: price, Selector: []byte(`{}`)})
			} else {
				return nil, errors.New("OpenAI speech pricing malformed")
			}
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
