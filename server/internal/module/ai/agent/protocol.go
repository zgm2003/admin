package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const maxPromptBytes = 65536

var validCapability = map[string]bool{"text": true, "image": true, "video": true, "audio": true}

type BindingInput struct {
	ID              int64           `json:"id,string,omitempty"`
	ProviderModelID int64           `json:"providerModelId,string"`
	TextProtocol    int16           `json:"textProtocol"`
	Parameters      json.RawMessage `json:"parameters"`
	Status          int16           `json:"status"`
}
type ToolBindingInput struct {
	ToolID               int64           `json:"toolId,string"`
	ExposedName          string          `json:"exposedName"`
	ParameterConstraints json.RawMessage `json:"parameterConstraints"`
	Status               int16           `json:"status"`
}
type CreateInput struct {
	ActiveProviderModelID *int64             `json:"activeProviderModelId,string"`
	Code                  string             `json:"code"`
	Name                  string             `json:"name"`
	Description           string             `json:"description"`
	AvatarFileID          *int64             `json:"avatarFileId,string"`
	Capabilities          []string           `json:"capabilities"`
	SystemPrompt          string             `json:"systemPrompt"`
	MultiplierPPM         int64              `json:"multiplierPpm"`
	GenerationParams      json.RawMessage    `json:"generationParams"`
	Status                int16              `json:"status"`
	ActiveBindingID       *int64             `json:"activeBindingId,string"`
	Bindings              []BindingInput     `json:"bindings"`
	Tools                 []ToolBindingInput `json:"tools"`
	ActorID               int64              `json:"-"`
}
type UpdateInput struct {
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	AvatarFileID     *int64             `json:"avatarFileId,string"`
	Capabilities     []string           `json:"capabilities"`
	SystemPrompt     string             `json:"systemPrompt"`
	MultiplierPPM    int64              `json:"multiplierPpm"`
	GenerationParams json.RawMessage    `json:"generationParams"`
	Bindings         []BindingInput     `json:"bindings"`
	Tools            []ToolBindingInput `json:"tools"`
	ActorID          int64              `json:"-"`
}
type CurrentBindingInput struct {
	BindingID int64 `json:"bindingId,string"`
}

func object(raw json.RawMessage) error {
	b := bytes.TrimSpace(raw)
	if len(b) < 2 || len(b) > 65536 || !json.Valid(b) || b[0] != '{' || b[len(b)-1] != '}' {
		return errors.New("JSON object required")
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return err
	}
	if _, ok := v.(map[string]any); !ok {
		return errors.New("JSON object required")
	}
	return checkDepth(v, 0)
}
func checkDepth(v any, depth int) error {
	if depth > 16 {
		return errors.New("JSON depth exceeded")
	}
	switch x := v.(type) {
	case map[string]any:
		for _, e := range x {
			if er := checkDepth(e, depth+1); er != nil {
				return er
			}
		}
	case []any:
		for _, e := range x {
			if er := checkDepth(e, depth+1); er != nil {
				return er
			}
		}
	}
	return nil
}
func baseValidate(name, desc string, caps []string, prompt string, mult int64, params json.RawMessage) error {
	if strings.TrimSpace(name) == "" || len(name) > 100 {
		return errors.New("name is required")
	}
	if len(desc) > 1000 {
		return errors.New("description too long")
	}
	if len(caps) == 0 {
		return errors.New("capabilities required")
	}
	seen := map[string]bool{}
	for _, c := range caps {
		if !validCapability[c] || seen[c] {
			return fmt.Errorf("invalid capability %q", c)
		}
		seen[c] = true
	}
	if len(prompt) > maxPromptBytes {
		return errors.New("system prompt too long")
	}
	if mult < 1 || mult > 100000000 {
		return errors.New("multiplier is invalid")
	}
	return object(params)
}
func (i CreateInput) Validate() error {
	if i.Code == "" || len(i.Code) > 64 {
		return errors.New("code is required")
	}
	if err := baseValidate(i.Name, i.Description, i.Capabilities, i.SystemPrompt, i.MultiplierPPM, i.GenerationParams); err != nil {
		return err
	}
	if i.ActiveProviderModelID != nil && i.ActiveBindingID != nil {
		return errors.New("one initial current selection required")
	}
	if i.Status != 0 && i.Status != 1 {
		return errors.New("status invalid")
	}
	return validateRelations(i.Bindings, i.Tools)
}
func (i UpdateInput) Validate() error {
	if err := baseValidate(i.Name, i.Description, i.Capabilities, i.SystemPrompt, i.MultiplierPPM, i.GenerationParams); err != nil {
		return err
	}
	return validateRelations(i.Bindings, i.Tools)
}

var exposedNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

func validateRelations(bs []BindingInput, ts []ToolBindingInput) error {
	if len(bs) > 100 || len(ts) > 16 {
		return errors.New("binding limit exceeded")
	}
	seenModels := map[int64]bool{}
	for _, b := range bs {
		if b.ProviderModelID < 1 || (b.Status != 0 && b.Status != 1) || seenModels[b.ProviderModelID] || (b.TextProtocol != 0 && b.TextProtocol != 1 && b.TextProtocol != 2) {
			return errors.New("binding invalid")
		}
		seenModels[b.ProviderModelID] = true
		if b.Parameters != nil {
			if e := object(b.Parameters); e != nil {
				return e
			}
		}
	}
	seenTools := map[int64]bool{}
	seenNames := map[string]bool{}
	for _, v := range ts {
		if v.ToolID < 1 || !exposedNamePattern.MatchString(v.ExposedName) || strings.HasPrefix(v.ExposedName, "__admin_") || (v.Status != 0 && v.Status != 1) || seenTools[v.ToolID] || seenNames[v.ExposedName] {
			return errors.New("tool binding invalid")
		}
		seenTools[v.ToolID] = true
		seenNames[v.ExposedName] = true
		if e := object(v.ParameterConstraints); e != nil {
			return e
		}
		var constraints struct {
			MaxCalls          int            `json:"maxCalls"`
			AllowedParameters map[string]any `json:"allowedParameters"`
		}
		d := json.NewDecoder(bytes.NewReader(v.ParameterConstraints))
		d.DisallowUnknownFields()
		if e := d.Decode(&constraints); e != nil {
			return e
		}
		if constraints.MaxCalls < 1 || constraints.MaxCalls > 16 {
			return errors.New("maxCalls must be 1..16")
		}
	}
	return nil
}
func DecodeUpdate(payload map[string]any) (UpdateInput, error) {
	for _, key := range []string{"status", "activeBindingId"} {
		if _, ok := payload[key]; ok {
			return UpdateInput{}, fmt.Errorf("%s is not accepted on update", key)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return UpdateInput{}, err
	}
	var out UpdateInput
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return UpdateInput{}, err
	}
	return out, out.Validate()
}
