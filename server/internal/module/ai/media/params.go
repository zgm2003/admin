package media

import (
	"encoding/json"
	"errors"

	airun "admin/server/internal/module/ai/run"
)

var errFrozenMediaParams = errors.New("AI media parameters are invalid or reserved")

// FrozenMediaParams combines the immutable Agent and binding parameter objects.
// Provider request identity fields are owned by RunInput and can never be
// overridden by a parameter object.
func FrozenMediaParams(config airun.RuntimeConfig) (json.RawMessage, error) {
	out := make(map[string]json.RawMessage)
	for _, raw := range []json.RawMessage{config.GenerationParams, config.Parameters} {
		if len(raw) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return nil, errFrozenMediaParams
		}
		for key, value := range fields {
			if mediaReservedParameter(key) {
				return nil, errFrozenMediaParams
			}
			out[key] = value
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, errFrozenMediaParams
	}
	return encoded, nil
}

func mediaReservedParameter(key string) bool {
	switch key {
	case "model", "prompt", "n", "size", "quality", "output_format", "image", "file", "input", "voice", "response_format", "language", "instructions":
		return true
	default:
		return false
	}
}
