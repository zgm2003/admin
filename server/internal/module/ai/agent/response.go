package agent

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}

type SafeValue struct {
	ID              int64             `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Avatar          string            `json:"avatar"`
	SystemPrompt    string            `json:"systemPrompt"`
	Scene           Scene             `json:"scene"`
	SceneName       string            `json:"sceneName"`
	Type            pricing.ModelKind `json:"type"`
	TypeName        string            `json:"typeName"`
	ProviderModelID int64             `json:"providerModelId"`
	TimeoutMS       int32             `json:"timeoutMs"`
	ProviderName    string            `json:"providerName"`
	ModelName       string            `json:"modelName"`
	ModelKey        string            `json:"modelKey"`
	ModelAvailable  bool              `json:"modelAvailable"`
	Status          yesno.Value       `json:"status"`
	StatusName      string            `json:"statusName"`
	Version         int64             `json:"version"`
	Actions         Actions           `json:"actions"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

type Options struct {
	Types    []option.Option[pricing.ModelKind] `json:"types"`
	Scenes   []option.Option[Scene]             `json:"scenes"`
	Statuses []option.Option[yesno.Value]       `json:"statuses"`
}

type ModelOption struct {
	Value     int64             `json:"value"`
	Label     string            `json:"label"`
	ModelKey  string            `json:"modelKey"`
	ModelKind pricing.ModelKind `json:"modelKind"`
}

type ProviderModelOption struct {
	Value    int64         `json:"value"`
	Label    string        `json:"label"`
	Children []ModelOption `json:"children"`
}

type ModelOptions struct {
	Providers []ProviderModelOption `json:"providers"`
}

func safeValue(ctx context.Context, row Model, ref *ModelRef) (SafeValue, error) {
	if row.ID < 1 || !yesno.IsValid(row.Status) || row.Version < 1 {
		return SafeValue{}, dependency(fmt.Errorf("agent row has invalid selectable values"))
	}
	if row.Name == "" || row.SystemPrompt == "" ||
		!row.Scene.Valid() || !row.AgentType.Valid() || row.ProviderModelID < 1 {
		return SafeValue{}, dependency(fmt.Errorf("agent row has incomplete configuration"))
	}
	statusName := option.New(ctx, row.Status, "停用", "Disabled").Label
	if row.Status == yesno.Yes {
		statusName = option.New(ctx, row.Status, "启用", "Enabled").Label
	}
	result := SafeValue{
		ID: row.ID, Name: row.Name, Description: row.Description, Avatar: row.Avatar,
		SystemPrompt: row.SystemPrompt,
		Scene:        row.Scene, SceneName: sceneName(ctx, row.Scene),
		Type: row.AgentType, TypeName: agentTypeName(ctx, row.AgentType),
		ProviderModelID: row.ProviderModelID, TimeoutMS: row.TimeoutMS,
		Status: row.Status, StatusName: statusName,
		Version: row.Version, Actions: Actions{Update: true, Status: true, Delete: true},
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if ref != nil {
		result.ProviderName = ref.ProviderName
		result.ModelName = ref.DisplayName
		if result.ModelName == "" {
			result.ModelName = ref.ModelKey
		}
		result.ModelKey = ref.ModelKey
		result.ModelAvailable = ref.Available
	}
	return result, nil
}

func modelOption(ref ModelRef) ModelOption {
	label := ref.DisplayName
	if label == "" {
		label = ref.ModelKey
	}
	if ref.ModelKey != "" && ref.ModelKey != ref.DisplayName {
		label += " (" + ref.ModelKey + ")"
	}
	return ModelOption{
		Value: ref.ID, Label: label, ModelKey: ref.ModelKey, ModelKind: ref.ModelKind,
	}
}

func sceneName(ctx context.Context, scene Scene) string {
	switch scene {
	case SceneToolGeneration:
		return option.New(ctx, scene, "工具生成", "Tool generation").Label
	case SceneImageGeneration:
		return option.New(ctx, scene, "生图", "Image generation").Label
	case SceneVideoGeneration:
		return option.New(ctx, scene, "视频生成", "Video generation").Label
	case SceneAudioGeneration:
		return option.New(ctx, scene, "音频生成", "Audio generation").Label
	default:
		return option.New(ctx, scene, "对话", "Conversation").Label
	}
}

func agentTypeName(ctx context.Context, kind pricing.ModelKind) string {
	switch kind {
	case pricing.KindImage:
		return option.New(ctx, kind, "图片", "Image").Label
	case pricing.KindVideo:
		return option.New(ctx, kind, "视频", "Video").Label
	case pricing.KindAudio:
		return option.New(ctx, kind, "音频", "Audio").Label
	default:
		return option.New(ctx, kind, "对话", "Chat").Label
	}
}
