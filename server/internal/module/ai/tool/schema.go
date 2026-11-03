package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"
)

// Schema validation intentionally supports a bounded subset. Unsupported keywords are rejected,
// rather than silently claiming constraints were enforced.
var schemaKeywords = map[string]bool{"type": true, "properties": true, "required": true, "additionalProperties": true, "items": true, "enum": true, "description": true, "title": true, "minimum": true, "maximum": true, "minLength": true, "maxLength": true, "minItems": true, "maxItems": true}

func parseSchema(raw json.RawMessage) (map[string]any, error) {
	if len(raw) > 65536 || !json.Valid(raw) {
		return nil, errors.New("schema exceeds size limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v map[string]any
	if e := d.Decode(&v); e != nil {
		return nil, e
	}
	if v == nil {
		return nil, errors.New("schema object required")
	}
	if e := checkSchema(v, 0); e != nil {
		return nil, e
	}
	return v, nil
}
func checkSchema(s map[string]any, depth int) error {
	if depth > 16 {
		return errors.New("schema depth exceeded")
	}
	for k := range s {
		if !schemaKeywords[k] {
			return fmt.Errorf("unsupported schema keyword %s", k)
		}
	}
	if t, ok := s["type"]; ok {
		ts, ok := t.(string)
		if !ok {
			return errors.New("schema type must be string")
		}
		switch ts {
		case "object", "array", "string", "number", "integer", "boolean", "null":
		default:
			return errors.New("schema type invalid")
		}
	}
	if v, ok := s["properties"]; ok {
		p, ok := v.(map[string]any)
		if !ok || len(p) > 128 {
			return errors.New("schema properties invalid")
		}
		for _, child := range p {
			m, ok := child.(map[string]any)
			if !ok {
				return errors.New("schema property invalid")
			}
			if e := checkSchema(m, depth+1); e != nil {
				return e
			}
		}
	}
	if v, ok := s["required"]; ok {
		r, ok := v.([]any)
		if !ok {
			return errors.New("schema required invalid")
		}
		for _, x := range r {
			if _, ok := x.(string); !ok {
				return errors.New("schema required key invalid")
			}
		}
	}
	if v, ok := s["additionalProperties"]; ok {
		if _, ok := v.(bool); !ok {
			return errors.New("additionalProperties must be bool")
		}
	}
	if v, ok := s["items"]; ok {
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("items schema invalid")
		}
		if e := checkSchema(m, depth+1); e != nil {
			return e
		}
	}
	for _, k := range []string{"minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems"} {
		if v, ok := s[k]; ok {
			n, ok := v.(json.Number)
			if !ok {
				return fmt.Errorf("%s must be number", k)
			}
			f, e := n.Float64()
			if e != nil || math.IsInf(f, 0) || math.IsNaN(f) {
				return errors.New("invalid numeric constraint")
			}
			if k != "minimum" && k != "maximum" && (f < 0 || f != math.Trunc(f)) {
				return errors.New("invalid length constraint")
			}
		}
	}
	if v, ok := s["enum"]; ok {
		a, ok := v.([]any)
		if !ok || len(a) == 0 || len(a) > 128 {
			return errors.New("enum invalid")
		}
	}
	return nil
}
func validateSchemaValue(raw json.RawMessage, schema json.RawMessage) error {
	if len(raw) > 65536 || !json.Valid(raw) {
		return errors.New("tool JSON input invalid")
	}
	s, e := parseSchema(schema)
	if e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if e = d.Decode(&v); e != nil {
		return e
	}
	return validateValue(v, s, 0)
}
func validateValue(v any, s map[string]any, depth int) error {
	if depth > 32 {
		return errors.New("value depth exceeded")
	}
	typ, _ := s["type"].(string)
	if typ != "" {
		ok := false
		switch typ {
		case "object":
			_, ok = v.(map[string]any)
		case "array":
			_, ok = v.([]any)
		case "string":
			_, ok = v.(string)
		case "number":
			_, ok = v.(json.Number)
		case "integer":
			if n, yes := v.(json.Number); yes {
				_, e := n.Int64()
				ok = e == nil
			}
		case "boolean":
			_, ok = v.(bool)
		case "null":
			ok = v == nil
		}
		if !ok {
			return errors.New("schema type mismatch")
		}
	}
	if choices, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range choices {
			if reflect.DeepEqual(x, v) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("schema enum mismatch")
		}
	}
	if obj, ok := v.(map[string]any); ok {
		props, _ := s["properties"].(map[string]any)
		req, _ := s["required"].([]any)
		for _, key := range req {
			if _, ok := obj[key.(string)]; !ok {
				return errors.New("required property missing")
			}
		}
		additional, exists := s["additionalProperties"].(bool)
		for key, val := range obj {
			child, known := props[key]
			if !known {
				if exists && !additional {
					return errors.New("additional property forbidden")
				}
				continue
			}
			if e := validateValue(val, child.(map[string]any), depth+1); e != nil {
				return e
			}
		}
	}
	if a, ok := v.([]any); ok {
		if e := lengthBounds(len(a), s, "minItems", "maxItems"); e != nil {
			return e
		}
		if child, ok := s["items"].(map[string]any); ok {
			for _, x := range a {
				if e := validateValue(x, child, depth+1); e != nil {
					return e
				}
			}
		}
	}
	if text, ok := v.(string); ok {
		if e := lengthBounds(utf8.RuneCountInString(text), s, "minLength", "maxLength"); e != nil {
			return e
		}
	}
	if n, ok := v.(json.Number); ok {
		f, e := n.Float64()
		if e != nil {
			return e
		}
		if v, ok := s["minimum"].(json.Number); ok {
			x, _ := v.Float64()
			if f < x {
				return errors.New("below minimum")
			}
		}
		if v, ok := s["maximum"].(json.Number); ok {
			x, _ := v.Float64()
			if f > x {
				return errors.New("above maximum")
			}
		}
	}
	return nil
}
func lengthBounds(n int, s map[string]any, min, max string) error {
	if v, ok := s[min].(json.Number); ok {
		x, _ := v.Int64()
		if int64(n) < x {
			return errors.New("below minimum length")
		}
	}
	if v, ok := s[max].(json.Number); ok {
		x, _ := v.Int64()
		if int64(n) > x {
			return errors.New("above maximum length")
		}
	}
	return nil
}
