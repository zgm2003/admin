package template

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
)

const (
	PermissionUpdate = "message:mail:template:update"
	PermissionStatus = "message:mail:template:status"
)

type UpdateInput struct {
	Scene             string            `json:"scene"`
	Name              string            `json:"name"`
	Subject           string            `json:"subject"`
	TencentTemplateID *int              `json:"tencentTemplateId"`
	Content           string            `json:"content"`
	VariableKeys      []string          `json:"variableKeys"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
}

type Safe struct {
	ID                int64             `json:"id"`
	Scene             string            `json:"scene"`
	Name              string            `json:"name"`
	Subject           string            `json:"subject"`
	TencentTemplateID *int              `json:"tencentTemplateId"`
	Content           string            `json:"content"`
	VariableKeys      []string          `json:"variableKeys"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
	IsEnabled         yesno.Value       `json:"isEnabled"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
}

func safeOf(value Model) (Safe, error) {
	var keys []string
	var examples map[string]string
	if err := json.Unmarshal(value.VariableKeys, &keys); err != nil {
		return Safe{}, fmt.Errorf("decode mail variable keys: %w", err)
	}
	if err := json.Unmarshal(value.ExampleVariables, &examples); err != nil {
		return Safe{}, fmt.Errorf("decode mail example variables: %w", err)
	}
	return Safe{ID: value.ID, Scene: value.Scene, Name: value.Name, Subject: value.Subject, TencentTemplateID: value.TencentTemplateID, Content: value.Content, VariableKeys: keys, ExampleVariables: examples, IsEnabled: value.IsEnabled, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano)}, nil
}

type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (cachegeneration.MutationResult, error)) error
}
