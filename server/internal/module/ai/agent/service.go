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
	Create(context.Context, Model, []int64) (int64, error)
	FindByID(context.Context, int64) (Model, error)
	List(context.Context, ListQuery) ([]Model, int64, error)
	Update(context.Context, int64, UpdateValues, []int64, bool) error
	UpdateStatus(context.Context, int64, yesno.Value, time.Time) error
	Delete(context.Context, int64) error
	ModelRefsByIDs(context.Context, []int64) (map[int64]ModelRef, error)
	ListUsableModelOptions(context.Context) ([]ModelRef, error)
	FindUsableModel(context.Context, int64, pricing.ModelKind) (ModelRef, error)
	ToolIDsByAgentIDs(context.Context, []int64) (map[int64][]int64, error)
	ToolRefs(context.Context) ([]ToolRef, error)
	ExistingToolIDs(context.Context, []int64) (map[int64]struct{}, error)
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
	agentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		agentIDs = append(agentIDs, row.ID)
	}
	toolIDs, err := s.repo.ToolIDsByAgentIDs(ctx, agentIDs)
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
		item.ToolIDs = append([]int64{}, toolIDs[row.ID]...)
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
	toolIDs, err := s.repo.ToolIDsByAgentIDs(ctx, []int64{row.ID})
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	var result SafeValue
	ref, ok := refs[row.ProviderModelID]
	if !ok {
		result, err = safeValue(ctx, row, nil)
	} else {
		result, err = safeValue(ctx, row, &ref)
	}
	if err != nil {
		return SafeValue{}, err
	}
	result.ToolIDs = append([]int64{}, toolIDs[row.ID]...)
	return result, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, dependency(fmt.Errorf("agent service is not configured"))
	}
	values, err := validateInput(
		input.Name, input.Description, input.SystemPrompt, int16(input.Scene),
		input.ProviderModelID, int16(input.AgentType), input.TimeoutMS, input.Status,
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
	toolIDs, err := s.validateToolIDs(ctx, input.ToolIDs)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	id, err := s.repo.Create(ctx, Model{
		Name: values.Name, Description: values.Description, Avatar: input.Avatar, SystemPrompt: values.SystemPrompt,
		Scene: values.Scene, AgentType: values.AgentType,
		ProviderModelID: values.ProviderModelID, TimeoutMS: values.TimeoutMS,
		Status: values.Status, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, toolIDs)
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
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return mapError(err)
	}
	values, err := validateInput(
		input.Name, input.Description, input.SystemPrompt, int16(input.Scene),
		input.ProviderModelID, int16(input.AgentType), input.TimeoutMS, input.Status,
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
	avatar := existing.Avatar
	if input.Avatar != nil {
		avatar = *input.Avatar
	}
	var toolIDs []int64
	if input.ToolIDsSet {
		toolIDs, err = s.validateToolIDs(ctx, input.ToolIDs)
		if err != nil {
			return err
		}
	}
	if err := s.repo.Update(ctx, id, UpdateValues{
		Name: values.Name, Description: values.Description, Avatar: avatar,
		SystemPrompt: values.SystemPrompt,
		Scene:        values.Scene, AgentType: values.AgentType,
		ProviderModelID: values.ProviderModelID, TimeoutMS: values.TimeoutMS,
		Status: values.Status, UpdatedAt: s.now().UTC(),
	}, toolIDs, input.ToolIDsSet); err != nil {
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

func (s *Service) Options(ctx context.Context) (Options, error) {
	if s == nil || s.repo == nil {
		return Options{}, dependency(fmt.Errorf("agent repository missing"))
	}
	refs, err := s.repo.ToolRefs(ctx)
	if err != nil {
		return Options{}, mapError(err)
	}
	tools := make([]ToolOption, 0, len(refs))
	for _, ref := range refs {
		label := ref.Name
		if ref.Code != "" {
			label += " · " + ref.Code
		}
		tools = append(tools, ToolOption{
			Value: ref.ID, Label: label, Code: ref.Code, Description: ref.Description,
			Category: ref.Category, Status: ref.Status,
		})
	}
	return Options{
		Types: []option.Option[pricing.ModelKind]{
			option.New(ctx, pricing.KindChat, "对话", "Chat"),
			option.New(ctx, pricing.KindImage, "图片", "Image"),
			option.New(ctx, pricing.KindVideo, "视频", "Video"),
		},
		Scenes: []option.Option[Scene]{
			option.New(ctx, SceneConversation, "对话", "Conversation"),
			option.New(ctx, SceneToolGeneration, "工具生成", "Tool generation"),
			option.New(ctx, SceneImageGeneration, "生图", "Image generation"),
			option.New(ctx, SceneVideoGeneration, "视频生成", "Video generation"),
		},
		Statuses: []option.Option[yesno.Value]{
			option.New(ctx, yesno.No, "停用", "Disabled"),
			option.New(ctx, yesno.Yes, "启用", "Enabled"),
		},
		Tools: tools,
	}, nil
}

func (s *Service) validateToolIDs(ctx context.Context, input []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(input))
	ids := make([]int64, 0, len(input))
	for _, id := range input {
		if id < 1 {
			return nil, invalid(fmt.Errorf("toolIds contains an invalid id"))
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ids, nil
	}
	existing, err := s.repo.ExistingToolIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	if len(existing) != len(ids) {
		return nil, invalid(fmt.Errorf("toolIds contains a tool that does not exist"))
	}
	return ids, nil
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
