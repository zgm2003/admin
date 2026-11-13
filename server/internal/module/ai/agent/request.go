package agent

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

const (
	maxNameRunes         = 128
	maxDescriptionRunes  = 500
	maxSystemPromptRunes = 20000
	defaultTimeoutMS     = 60000
	minTimeoutMS         = 1000
	maxTimeoutMS         = 300000
)

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Type     *pricing.ModelKind
	Scene    *Scene
	Status   *yesno.Value
}

type CreateInput struct {
	Name            string
	Description     string
	Avatar          string
	SystemPrompt    string
	Scene           Scene
	AgentType       pricing.ModelKind
	ProviderModelID int64
	TimeoutMS       int32
	Status          yesno.Value
}

type UpdateInput struct {
	Name            string
	Description     string
	Avatar          *string
	SystemPrompt    string
	Scene           Scene
	AgentType       pricing.ModelKind
	ProviderModelID int64
	TimeoutMS       int32
	Status          yesno.Value
}

type StatusInput struct {
	Status yesno.Value
}

type createRequest struct {
	Name            *string      `json:"name"`
	Description     *string      `json:"description"`
	Avatar          *string      `json:"avatar"`
	SystemPrompt    *string      `json:"systemPrompt"`
	Scene           *int16       `json:"scene"`
	Type            *int16       `json:"type"`
	ProviderModelID *int64       `json:"providerModelId"`
	TimeoutMS       *int32       `json:"timeoutMs"`
	Status          *yesno.Value `json:"status"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Name == nil || r.Description == nil || r.SystemPrompt == nil || r.Scene == nil ||
		r.Type == nil || r.ProviderModelID == nil || r.Status == nil {
		return CreateInput{}, fmt.Errorf("name, description, systemPrompt, scene, type, providerModelId and status are required")
	}
	timeoutMS := int32(defaultTimeoutMS)
	if r.TimeoutMS != nil {
		timeoutMS = *r.TimeoutMS
	}
	value, err := validateInput(
		*r.Name, *r.Description, *r.SystemPrompt, *r.Scene,
		*r.ProviderModelID, *r.Type, timeoutMS, *r.Status,
	)
	if err != nil {
		return CreateInput{}, err
	}
	avatar := ""
	if r.Avatar != nil {
		avatar, err = normalizeAvatar(*r.Avatar)
		if err != nil {
			return CreateInput{}, err
		}
	}
	return CreateInput{
		Name: value.Name, Description: value.Description, Avatar: avatar,
		SystemPrompt: value.SystemPrompt, Scene: value.Scene, AgentType: value.AgentType,
		ProviderModelID: value.ProviderModelID, TimeoutMS: value.TimeoutMS, Status: value.Status,
	}, nil
}

type updateRequest struct {
	Name            *string      `json:"name"`
	Description     *string      `json:"description"`
	Avatar          *string      `json:"avatar"`
	SystemPrompt    *string      `json:"systemPrompt"`
	Scene           *int16       `json:"scene"`
	Type            *int16       `json:"type"`
	ProviderModelID *int64       `json:"providerModelId"`
	TimeoutMS       *int32       `json:"timeoutMs"`
	Status          *yesno.Value `json:"status"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.Description == nil || r.SystemPrompt == nil || r.Scene == nil ||
		r.Type == nil || r.ProviderModelID == nil || r.Status == nil {
		return UpdateInput{}, fmt.Errorf("name, description, systemPrompt, scene, type, providerModelId and status are required")
	}
	timeoutMS := int32(defaultTimeoutMS)
	if r.TimeoutMS != nil {
		timeoutMS = *r.TimeoutMS
	}
	value, err := validateInput(
		*r.Name, *r.Description, *r.SystemPrompt, *r.Scene,
		*r.ProviderModelID, *r.Type, timeoutMS, *r.Status,
	)
	if err != nil {
		return UpdateInput{}, err
	}
	var avatar *string
	if r.Avatar != nil {
		normalized, normalizeErr := normalizeAvatar(*r.Avatar)
		if normalizeErr != nil {
			return UpdateInput{}, normalizeErr
		}
		avatar = &normalized
	}
	return UpdateInput{
		Name: value.Name, Description: value.Description, Avatar: avatar,
		SystemPrompt: value.SystemPrompt, Scene: value.Scene, AgentType: value.AgentType,
		ProviderModelID: value.ProviderModelID, TimeoutMS: value.TimeoutMS, Status: value.Status,
	}, nil
}

func normalizeAvatar(raw string) (string, error) {
	avatar := strings.TrimSpace(raw)
	if utf8.RuneCountInString(avatar) > 512 {
		return "", fmt.Errorf("avatar is too long")
	}
	for _, character := range avatar {
		if character < 0x20 || character == 0x7f {
			return "", fmt.Errorf("avatar contains control characters")
		}
	}
	return avatar, nil
}

type inputValues struct {
	Name            string
	Description     string
	SystemPrompt    string
	Scene           Scene
	AgentType       pricing.ModelKind
	ProviderModelID int64
	TimeoutMS       int32
	Status          yesno.Value
}

func validateInput(
	name, description, systemPrompt string,
	scene int16,
	providerModelID int64,
	agentType int16,
	timeoutMS int32,
	status yesno.Value,
) (inputValues, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	systemPrompt = strings.TrimSpace(systemPrompt)
	if timeoutMS == 0 {
		timeoutMS = defaultTimeoutMS
	}
	modelKind := pricing.ModelKind(agentType)
	sceneValue := Scene(scene)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		return inputValues{}, fmt.Errorf("name must be between 1 and %d characters", maxNameRunes)
	}
	if utf8.RuneCountInString(description) > maxDescriptionRunes {
		return inputValues{}, fmt.Errorf("description must be at most %d characters", maxDescriptionRunes)
	}
	if systemPrompt == "" || utf8.RuneCountInString(systemPrompt) > maxSystemPromptRunes {
		return inputValues{}, fmt.Errorf("systemPrompt must be between 1 and %d characters", maxSystemPromptRunes)
	}
	if !sceneValue.Valid() {
		return inputValues{}, fmt.Errorf("scene is invalid")
	}
	if !modelKind.Valid() {
		return inputValues{}, fmt.Errorf("type is invalid")
	}
	if providerModelID < 1 {
		return inputValues{}, fmt.Errorf("providerModelId is invalid")
	}
	if timeoutMS < minTimeoutMS || timeoutMS > maxTimeoutMS {
		return inputValues{}, fmt.Errorf("timeoutMs must be between %d and %d", minTimeoutMS, maxTimeoutMS)
	}
	if !yesno.IsValid(status) {
		return inputValues{}, fmt.Errorf("status is invalid")
	}
	return inputValues{
		Name: name, Description: description, SystemPrompt: systemPrompt,
		Scene: sceneValue, AgentType: modelKind,
		ProviderModelID: providerModelID, TimeoutMS: timeoutMS, Status: status,
	}, nil
}

type statusRequest struct {
	Status *yesno.Value `json:"status"`
}

func (r statusRequest) input() (StatusInput, error) {
	if r.Status == nil || !yesno.IsValid(*r.Status) {
		return StatusInput{}, fmt.Errorf("status is invalid")
	}
	return StatusInput{Status: *r.Status}, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" &&
			key != "status" && key != "type" && key != "scene") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{
		Page: request.Page, PageSize: request.PageSize,
		Keyword: strings.TrimSpace(values.Get("keyword")),
	}
	if utf8.RuneCountInString(query.Keyword) > 200 {
		return ListQuery{}, fmt.Errorf("keyword is too long")
	}
	if raw := strings.TrimSpace(values.Get("type")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return ListQuery{}, fmt.Errorf("type is invalid")
		}
		modelKind := pricing.ModelKind(value)
		if !modelKind.Valid() {
			return ListQuery{}, fmt.Errorf("type is invalid")
		}
		query.Type = &modelKind
	}
	if raw := strings.TrimSpace(values.Get("scene")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return ListQuery{}, fmt.Errorf("scene is invalid")
		}
		scene := Scene(value)
		if !scene.Valid() {
			return ListQuery{}, fmt.Errorf("scene is invalid")
		}
		query.Scene = &scene
	}
	if raw := strings.TrimSpace(values.Get("status")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		status := yesno.Value(value)
		if !yesno.IsValid(status) {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		query.Status = &status
	}
	return query, nil
}
