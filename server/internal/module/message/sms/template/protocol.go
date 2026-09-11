package template

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"admin/server/internal/shared/yesno"
)

const (
	PermissionList   = "message:sms:list"
	PermissionUpdate = "message:sms:template:update"
	PermissionStatus = "message:sms:template:status"

	maxTemplateNameLength = 128
	maxTemplateIDLength   = 64
	maxVariableLength     = 64
)

var numericTemplatePattern = regexp.MustCompile(`^[0-9]{1,64}$`)

type Safe struct {
	ID                int64             `json:"id"`
	Scene             string            `json:"scene"`
	Name              string            `json:"name"`
	TencentTemplateID string            `json:"tencentTemplateId"`
	ParameterKeys     []string          `json:"parameterKeys"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
	IsEnabled         yesno.Value       `json:"isEnabled"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
}

type ListResponse struct {
	List []Safe `json:"list"`
}

type UpdateInput struct {
	Scene             string
	Name              string
	TencentTemplateID string
	ParameterKeys     []string
	ExampleVariables  map[string]string
}

type repository interface {
	List(context.Context) ([]Model, error)
	FindByID(context.Context, int64) (Model, error)
	Update(context.Context, *Model, time.Time) error
	UpdateStatus(context.Context, int64, int16, time.Time) error
}

// RuntimeCoordinator invalidates the SMS runtime snapshot after a write.
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}

func decodeJSON(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("json payload is missing")
	}
	decoder := json.NewDecoder(bytes.NewBuffer(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json payload: %w", err)
	}
	return nil
}

func safeOf(value Model) (Safe, error) {
	var keys []string
	if err := json.Unmarshal(value.ParameterKeys, &keys); err != nil {
		return Safe{}, fmt.Errorf("decode sms template parameter keys: %w", err)
	}
	var variables map[string]string
	if err := json.Unmarshal(value.ExampleVariables, &variables); err != nil {
		return Safe{}, fmt.Errorf("decode sms template example variables: %w", err)
	}
	return Safe{
		ID:                value.ID,
		Scene:             value.Scene,
		Name:              value.Name,
		TencentTemplateID: value.TencentTemplateID,
		ParameterKeys:     keys,
		ExampleVariables:  variables,
		IsEnabled:         value.IsEnabled,
		CreatedAt:         value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:         value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}
