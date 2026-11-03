package providerrequest

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"strings"
)

// speechSSEBody exposes decoded audio only. Usage is final only after an explicit
// speech.audio.done frame; EOF without that frame is an unknown provider result.
type speechSSEBody struct {
	source  io.ReadCloser
	scanner *bufio.Scanner
	pending []byte
	usage   MediaUsage
	done    bool
}

func newSpeechSSEBody(source io.ReadCloser) *speechSSEBody {
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	return &speechSSEBody{source: source, scanner: scanner}
}
func (s *speechSSEBody) Close() error      { return s.source.Close() }
func (s *speechSSEBody) Usage() MediaUsage { return s.usage }
func (s *speechSSEBody) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	for len(s.pending) == 0 {
		if s.done {
			return 0, io.EOF
		}
		var data strings.Builder
		for s.scanner.Scan() {
			line := s.scanner.Text()
			if line == "" {
				if data.Len() > 0 {
					break
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				value := strings.TrimPrefix(line, "data:")
				if data.Len()+len(value) > 4<<20 {
					return 0, ErrResponseInvalid
				}
				data.WriteString(strings.TrimSpace(value))
			}
		}
		if data.Len() == 0 {
			if err := s.scanner.Err(); err != nil {
				return 0, err
			}
			return 0, io.ErrUnexpectedEOF
		}
		var event struct {
			Type  string          `json:"type"`
			Audio string          `json:"audio"`
			Usage json.RawMessage `json:"usage"`
		}
		if json.Unmarshal([]byte(data.String()), &event) != nil {
			return 0, ErrResponseInvalid
		}
		switch event.Type {
		case "speech.audio.delta":
			if event.Audio == "" {
				return 0, ErrResponseInvalid
			}
			decoded, err := base64.StdEncoding.DecodeString(event.Audio)
			if err != nil || len(decoded) == 0 {
				return 0, ErrResponseInvalid
			}
			s.pending = decoded
		case "speech.audio.done":
			s.usage = parseAudioUsage(event.Usage, "")
			if !s.usage.Known || s.usage.Basis != "token" {
				return 0, ErrResponseInvalid
			}
			if s.usage.TextInputTokens == nil {
				s.usage.TextInputTokens = s.usage.InputTokens
			}
			s.done = true
		default:
			return 0, ErrResponseInvalid
		}
	}
	n := copy(dst, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

// Official token and duration usages are distinct facts. No token counter is
// inferred from elapsed seconds, returned text, audio bytes, or input length.
func parseAudioUsage(raw json.RawMessage, legacyDuration string) MediaUsage {
	usage := MediaUsage{Raw: append(json.RawMessage(nil), raw...)}
	var wire struct {
		Type         string      `json:"type"`
		InputTokens  *int64      `json:"input_tokens"`
		OutputTokens *int64      `json:"output_tokens"`
		TotalTokens  *int64      `json:"total_tokens"`
		Seconds      json.Number `json:"seconds"`
		InputDetails struct {
			Audio *int64 `json:"audio_tokens"`
			Text  *int64 `json:"text_tokens"`
		} `json:"input_token_details"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if json.Unmarshal(raw, &wire) != nil {
			return usage
		}
		if wire.Type == "tokens" || wire.InputTokens != nil || wire.OutputTokens != nil {
			if wire.InputTokens == nil || wire.OutputTokens == nil || *wire.InputTokens < 0 || *wire.OutputTokens < 0 || *wire.InputTokens > math.MaxInt64-*wire.OutputTokens {
				return usage
			}
			if wire.TotalTokens != nil && *wire.TotalTokens != *wire.InputTokens+*wire.OutputTokens {
				return usage
			}
			for _, v := range []*int64{wire.InputDetails.Audio, wire.InputDetails.Text} {
				if v != nil && (*v < 0 || *v > *wire.InputTokens) {
					return usage
				}
			}
			if wire.InputDetails.Audio != nil && wire.InputDetails.Text != nil && *wire.InputDetails.Audio != *wire.InputTokens-*wire.InputDetails.Text {
				return usage
			}
			usage.Known = true
			usage.Basis = "token"
			usage.InputTokens = wire.InputTokens
			usage.OutputTokens = wire.OutputTokens
			usage.AudioInputTokens = wire.InputDetails.Audio
			usage.TextInputTokens = wire.InputDetails.Text
			return usage
		}
		if wire.Type != "duration" || wire.Seconds == "" {
			return usage
		}
		legacyDuration = wire.Seconds.String()
	}
	if legacyDuration != "" {
		var seconds json.Number
		if json.Unmarshal([]byte(legacyDuration), &seconds) != nil {
			return usage
		}
		n, err := seconds.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return usage
		}
		usage.Known = true
		usage.Basis = "duration"
		usage.DurationSeconds = legacyDuration
	}
	return usage
}
func speechMIME(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/mpeg"
	}
}
