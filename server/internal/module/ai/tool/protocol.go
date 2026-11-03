package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

type Type int16

const (
	TypeHTTP    Type = 1
	TypeBuiltin Type = 2
)

type Status int16

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

type ToolInput struct {
	Code    string       `json:"code"`
	Name    string       `json:"name"`
	Status  Status       `json:"status"`
	ActorID int64        `json:"-"`
	Version VersionInput `json:"version"`
}
type UpdateInput struct {
	Name    string       `json:"name"`
	ActorID int64        `json:"-"`
	Version VersionInput `json:"version"`
}
type VersionInput struct {
	Type             Type              `json:"type"`
	Description      string            `json:"description"`
	InputSchema      json.RawMessage   `json:"inputSchema"`
	OutputSchema     json.RawMessage   `json:"outputSchema"`
	BuiltinKey       string            `json:"builtinKey"`
	HTTPDefinition   json.RawMessage   `json:"httpDefinition"`
	SecretCiphertext string            `json:"-"`
	SecretHeaders    map[string]string `json:"secretHeaders"`
	EncryptionKeyID  string            `json:"-"`
	TimeoutMS        int               `json:"timeoutMs"`
	OutputLimitBytes int               `json:"outputLimitBytes"`
	HasSideEffect    int16             `json:"hasSideEffect"`
	IdempotencyMode  int16             `json:"idempotencyMode"`
}

func schemaObject(raw json.RawMessage) error { _, err := parseSchema(raw); return err }
func publicIP(ip net.IP) bool {
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil && ((ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127) || (ip4[0] == 169 && ip4[1] == 254)) {
		return false
	}
	return true
}
func safeHTTP(raw json.RawMessage) error { _, e := parseHTTPDefinition(raw); return e }
func (v VersionInput) Validate() error {
	if v.Type != TypeHTTP && v.Type != TypeBuiltin {
		return errors.New("tool type invalid")
	}
	if len(v.Description) > 16384 {
		return errors.New("description too long")
	}
	if err := schemaObject(v.InputSchema); err != nil {
		return err
	}
	if err := schemaObject(v.OutputSchema); err != nil {
		return err
	}
	if v.TimeoutMS < 100 || v.TimeoutMS > 30000 {
		return errors.New("timeout must be 100..30000ms")
	}
	if v.OutputLimitBytes < 1 || v.OutputLimitBytes > 65536 {
		return errors.New("output limit must be 1..65536")
	}
	if v.HasSideEffect < 0 || v.HasSideEffect > 1 || v.IdempotencyMode < 0 || v.IdempotencyMode > 3 {
		return errors.New("side effect policy invalid")
	}
	if (v.HasSideEffect == 0 && v.IdempotencyMode != 0) || (v.HasSideEffect == 1 && v.IdempotencyMode == 0) {
		return errors.New("idempotency mode invalid")
	}
	if v.Type == TypeBuiltin {
		if v.BuiltinKey != "currentTime" && v.BuiltinKey != "calculator" {
			return errors.New("builtin key is not allowed")
		}
		if (len(v.HTTPDefinition) > 0 && string(v.HTTPDefinition) != "null") || v.SecretCiphertext != "" || v.EncryptionKeyID != "" || len(v.SecretHeaders) > 0 || v.HasSideEffect != 0 {
			return errors.New("builtin HTTP fields are not allowed")
		}
	} else {
		if len(v.HTTPDefinition) == 0 {
			return errors.New("HTTP definition required")
		}
		if err := safeHTTP(v.HTTPDefinition); err != nil {
			return err
		}
		if v.BuiltinKey != "" {
			return errors.New("HTTP builtin key is not allowed")
		}
		for k, val := range v.SecretHeaders {
			if !allowedHeader(k) || len(val) == 0 || len(val) > 4096 || strings.ContainsAny(val, "\r\n") {
				return errors.New("secret header invalid")
			}
		}
		if (v.SecretCiphertext == "") != (v.EncryptionKeyID == "") {
			return errors.New("secret and encryption key must be paired")
		}
	}
	return nil
}
func (i ToolInput) Validate() error {
	if i.Code == "" || len(i.Code) > 64 {
		return fmt.Errorf("code is required")
	}
	if i.Name == "" || len(i.Name) > 100 {
		return fmt.Errorf("name is required")
	}
	if i.Status != StatusDisabled && i.Status != StatusEnabled {
		return errors.New("status invalid")
	}
	return i.Version.Validate()
}
func (i UpdateInput) Validate() error {
	if i.Name == "" || len(i.Name) > 100 {
		return errors.New("name is required")
	}
	return i.Version.Validate()
}
