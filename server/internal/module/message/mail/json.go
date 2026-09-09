package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("mail snapshot must be a JSON object")
	}
	return scanJSONObject(decoder, map[string]struct{}{})
}

func scanJSONObject(decoder *json.Decoder, seen map[string]struct{}) error {
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("mail snapshot key is invalid")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("mail snapshot contains duplicate key %q", key)
		}
		seen[key] = struct{}{}
		if err := scanJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
		case '{':
			return scanJSONObject(decoder, map[string]struct{}{})
		case '[':
			for decoder.More() {
				if err := scanJSONValue(decoder); err != nil {
					return err
				}
			}
			_, err := decoder.Token()
			return err
		default:
			return fmt.Errorf("mail snapshot contains invalid nesting")
		}
	}
	return nil
}
