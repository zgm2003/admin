package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type repository interface {
	Create(context.Context, Model) (int64, error)
	FindByID(context.Context, int64) (Model, error)
	List(context.Context, ListQuery) ([]Model, int64, error)
	Update(context.Context, int64, UpdateValues) error
	UpdateStatus(context.Context, int64, yesno.Value, time.Time) error
	Delete(context.Context, int64) error
	ModelRefsByIDs(context.Context, []int64) (map[int64]ModelRef, error)
	ListUsableModelOptions(context.Context) ([]ModelRef, error)
	FindUsableModel(context.Context, int64, pricing.ModelKind) (ModelRef, error)
}

type Service struct {
	repo repository
	now  func() time.Time
}

func NewService(repo repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if s == nil || s.repo == nil {
		return pagination.Result[SafeValue]{}, dependency(fmt.Errorf("agent repository missing"))
	}
	rows, total, err := s.repo.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProviderModelID)
	}
	refs, err := s.repo.ModelRefsByIDs(ctx, ids)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	items := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		ref, ok := refs[row.ProviderModelID]
		var refPointer *ModelRef
		if ok {
			refPointer = &ref
		}
		item, err := safeValue(ctx, row, refPointer)
		if err != nil {
			return pagination.Result[SafeValue]{}, err
		}
		items = append(items, item)
	}
	return pagination.Result[SafeValue]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if s == nil || s.repo == nil {
		return SafeValue{}, dependency(fmt.Errorf("agent repository missing"))
	}
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	refs, err := s.repo.ModelRefsByIDs(ctx, []int64{row.ProviderModelID})
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	ref, ok := refs[row.ProviderModelID]
	if !ok {
		return safeValue(ctx, row, nil)
	}
	return safeValue(ctx, row, &ref)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, dependency(fmt.Errorf("agent service is not configured"))
	}
	values, err := validateInput(
		input.Name, input.Description, input.SystemPrompt, int16(input.Scene),
		input.ProviderModelID, int16(input.AgentType), input.Status,
	)
	if err != nil {
		return 0, invalid(err)
	}
	if _, err := s.repo.FindUsableModel(ctx, values.ProviderModelID, values.AgentType); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, invalid(fmt.Errorf("providerModelId does not reference an enabled model of the agent type"))
		}
		return 0, mapError(err)
	}
	now := s.now().UTC()
	id, err := s.repo.Create(ctx, Model{
		Name: values.Name, Description: values.Description, SystemPrompt: values.SystemPrompt,
		Scene: values.Scene, AgentType: values.AgentType,
		ProviderModelID: values.ProviderModelID, Status: values.Status,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return 0, mapError(err)
	}
	return id, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if s == nil || s.repo == nil {
		return dependency(fmt.Errorf("agent repository missing"))
	}
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return mapError(err)
	}
	values, err := validateInput(
		input.Name, input.Description, input.SystemPrompt, int16(input.Scene),
		input.ProviderModelID, int16(input.AgentType), input.Status,
	)
	if err != nil {
		return invalid(err)
	}
	if _, err := s.repo.FindUsableModel(ctx, values.ProviderModelID, values.AgentType); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid(fmt.Errorf("providerModelId does not reference an enabled model of the agent type"))
		}
		return mapError(err)
	}
	if err := s.repo.Update(ctx, id, UpdateValues{
		Name: values.Name, Description: values.Description, SystemPrompt: values.SystemPrompt,
		Scene: values.Scene, AgentType: values.AgentType,
		ProviderModelID: values.ProviderModelID, Status: values.Status, UpdatedAt: s.now().UTC(),
	}); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, input StatusInput) error {
	if s == nil || s.repo == nil {
		return dependency(fmt.Errorf("agent repository missing"))
	}
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if !yesno.IsValid(input.Status) {
		return invalid(fmt.Errorf("status is invalid"))
	}
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return mapError(err)
	}
	if input.Status == yesno.Yes {
		if _, err := s.repo.FindUsableModel(ctx, row.ProviderModelID, row.AgentType); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return invalid(fmt.Errorf("bound provider model is unavailable"))
			}
			return mapError(err)
		}
	}
	if err := s.repo.UpdateStatus(ctx, id, input.Status, s.now().UTC()); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return dependency(fmt.Errorf("agent repository missing"))
	}
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) Options(ctx context.Context) Options {
	return Options{
		Types: []option.Option[pricing.ModelKind]{
			option.New(ctx, pricing.KindChat, "对话", "Chat"),
			option.New(ctx, pricing.KindImage, "图片", "Image"),
			option.New(ctx, pricing.KindVideo, "视频", "Video"),
			option.New(ctx, pricing.KindAudio, "音频", "Audio"),
		},
		Scenes: []option.Option[Scene]{
			option.New(ctx, SceneConversation, "对话", "Conversation"),
			option.New(ctx, SceneToolGeneration, "工具生成", "Tool generation"),
			option.New(ctx, SceneImageGeneration, "生图", "Image generation"),
			option.New(ctx, SceneVideoGeneration, "视频生成", "Video generation"),
			option.New(ctx, SceneAudioGeneration, "音频生成", "Audio generation"),
		},
		Statuses: []option.Option[yesno.Value]{
			option.New(ctx, yesno.No, "停用", "Disabled"),
			option.New(ctx, yesno.Yes, "启用", "Enabled"),
		},
	}
}

func (s *Service) ModelOptions(ctx context.Context) (ModelOptions, error) {
	if s == nil || s.repo == nil {
		return ModelOptions{}, dependency(fmt.Errorf("agent repository missing"))
	}
	refs, err := s.repo.ListUsableModelOptions(ctx)
	if err != nil {
		return ModelOptions{}, mapError(err)
	}
	providers := make([]ProviderModelOption, 0)
	indexByProvider := make(map[int64]int)
	for _, ref := range refs {
		index, exists := indexByProvider[ref.ProviderID]
		if !exists {
			index = len(providers)
			indexByProvider[ref.ProviderID] = index
			providers = append(providers, ProviderModelOption{
				Value: ref.ProviderID, Label: ref.ProviderName, Children: []ModelOption{},
			})
		}
		providers[index].Children = append(providers[index].Children, modelOption(ref))
	}
	return ModelOptions{Providers: providers}, nil
}
