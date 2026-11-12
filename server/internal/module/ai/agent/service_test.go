package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	row       Model
	rows      []Model
	refs      map[int64]ModelRef
	usable    map[int64]ModelRef
	created   Model
	updated   UpdateValues
	status    yesno.Value
	statusSet bool
	deleted   bool
	createID  int64
	listErr   error
	usableErr error
}

func (f *fakeRepository) Create(_ context.Context, row Model) (int64, error) {
	f.created = row
	return f.createID, nil
}
func (f *fakeRepository) FindByID(context.Context, int64) (Model, error) {
	return f.row, nil
}
func (f *fakeRepository) List(context.Context, ListQuery) ([]Model, int64, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.rows, int64(len(f.rows)), nil
}
func (f *fakeRepository) Update(_ context.Context, _ int64, values UpdateValues) error {
	f.updated = values
	return nil
}
func (f *fakeRepository) UpdateStatus(_ context.Context, _ int64, value yesno.Value, _ time.Time) error {
	f.status = value
	f.statusSet = true
	return nil
}
func (f *fakeRepository) Delete(context.Context, int64) error {
	f.deleted = true
	return nil
}
func (f *fakeRepository) ModelRefsByIDs(_ context.Context, ids []int64) (map[int64]ModelRef, error) {
	result := make(map[int64]ModelRef, len(ids))
	for _, id := range ids {
		if ref, ok := f.refs[id]; ok {
			result[id] = ref
		}
	}
	return result, nil
}
func (f *fakeRepository) ListUsableModelOptions(context.Context) ([]ModelRef, error) {
	result := make([]ModelRef, 0, len(f.usable))
	for _, ref := range f.usable {
		result = append(result, ref)
	}
	return result, nil
}
func (f *fakeRepository) FindUsableModel(_ context.Context, id int64, kind pricing.ModelKind) (ModelRef, error) {
	if f.usableErr != nil {
		return ModelRef{}, f.usableErr
	}
	ref, ok := f.usable[id]
	if !ok || ref.ModelKind != kind {
		return ModelRef{}, gorm.ErrRecordNotFound
	}
	return ref, nil
}

func testRef(id int64) ModelRef {
	return ModelRef{
		ID: id, ProviderID: 3, ProviderName: "OpenAI", ProviderStatus: yesno.Yes,
		ModelKey: "gpt-test", DisplayName: "GPT Test", ModelKind: pricing.KindChat,
		PricingMode: pricing.ModeOfficialToken, Status: yesno.Yes, Available: true,
	}
}

func newTestService(repo *fakeRepository) *Service {
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }
	return service
}

func TestCreateStoresAgentAndNormalizesInput(t *testing.T) {
	repo := &fakeRepository{createID: 7, usable: map[int64]ModelRef{9: testRef(9)}}
	service := newTestService(repo)
	id, err := service.Create(context.Background(), CreateInput{
		Name: "  Support Agent  ", Description: "  helps users  ", SystemPrompt: "  be helpful  ",
		Scene: SceneConversation, AgentType: pricing.KindChat,
		ProviderModelID: 9, Status: yesno.No,
	})
	if err != nil || id != 7 {
		t.Fatalf("Create() = %d, %v", id, err)
	}
	if repo.created.Name != "Support Agent" || repo.created.Description != "helps users" ||
		repo.created.SystemPrompt != "be helpful" || repo.created.Scene != SceneConversation ||
		repo.created.AgentType != pricing.KindChat || repo.created.ProviderModelID != 9 {
		t.Fatalf("created = %+v", repo.created)
	}
	if repo.created.Version != 1 || repo.created.CreatedAt.IsZero() {
		t.Fatalf("created metadata = %+v", repo.created)
	}
}

func TestCreateRejectsUnavailableModel(t *testing.T) {
	repo := &fakeRepository{usable: map[int64]ModelRef{}}
	service := newTestService(repo)
	_, err := service.Create(context.Background(), CreateInput{
		Name: "Support", SystemPrompt: "help", Scene: SceneConversation,
		AgentType: pricing.KindChat, ProviderModelID: 9, Status: yesno.No,
	})
	if err == nil {
		t.Fatal("unavailable model was accepted")
	}
}

func TestCreateRejectsModelKindMismatch(t *testing.T) {
	repo := &fakeRepository{usable: map[int64]ModelRef{9: testRef(9)}}
	service := newTestService(repo)
	_, err := service.Create(context.Background(), CreateInput{
		Name: "Poster", SystemPrompt: "draw", Scene: SceneImageGeneration,
		AgentType: pricing.KindImage, ProviderModelID: 9, Status: yesno.No,
	})
	if err == nil {
		t.Fatal("chat model was accepted for an image agent")
	}
}

func TestUpdateStatusRequiresUsableModelWhenEnabling(t *testing.T) {
	repo := &fakeRepository{
		row:    Model{ID: 5, ProviderModelID: 9, AgentType: pricing.KindChat, Status: yesno.No},
		usable: map[int64]ModelRef{},
	}
	service := newTestService(repo)
	if err := service.UpdateStatus(context.Background(), 5, StatusInput{Status: yesno.Yes}); err == nil {
		t.Fatal("enabling with an unavailable model was accepted")
	}
	if repo.statusSet {
		t.Fatalf("status changed to %v", repo.status)
	}
	if err := service.UpdateStatus(context.Background(), 5, StatusInput{Status: yesno.No}); err != nil {
		t.Fatal(err)
	}
	if !repo.statusSet || repo.status != yesno.No {
		t.Fatal("disable was not written")
	}
}

func TestListProjectsBoundModelFields(t *testing.T) {
	row := Model{
		ID: 5, Name: "Support", SystemPrompt: "help", Scene: SceneConversation,
		AgentType: pricing.KindChat, ProviderModelID: 9,
		Status: yesno.Yes, Version: 2, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repo := &fakeRepository{
		rows: []Model{row},
		refs: map[int64]ModelRef{9: testRef(9)},
	}
	service := newTestService(repo)
	result, err := service.List(context.Background(), ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 1 {
		t.Fatalf("list = %+v", result)
	}
	item := result.List[0]
	if item.ProviderName != "OpenAI" || item.ModelName != "GPT Test" ||
		item.ModelKey != "gpt-test" || !item.ModelAvailable ||
		item.Scene != SceneConversation || item.SceneName == "" ||
		item.Type != pricing.KindChat || item.TypeName == "" {
		t.Fatalf("item = %+v", item)
	}
}

func TestModelOptionsGroupsByProvider(t *testing.T) {
	second := testRef(10)
	second.ProviderID = 4
	second.ProviderName = "Anthropic"
	second.ModelKey = "claude-test"
	second.DisplayName = "Claude Test"
	repo := &fakeRepository{usable: map[int64]ModelRef{
		9: testRef(9), 10: second,
	}}
	service := newTestService(repo)
	result, err := service.ModelOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Providers) != 2 {
		t.Fatalf("providers = %+v", result.Providers)
	}
	byName := make(map[string]ProviderModelOption, len(result.Providers))
	for _, provider := range result.Providers {
		byName[provider.Label] = provider
	}
	openai, ok := byName["OpenAI"]
	if !ok || len(openai.Children) != 1 ||
		!strings.Contains(openai.Children[0].Label, "GPT Test") ||
		!strings.Contains(openai.Children[0].Label, "gpt-test") {
		t.Fatalf("OpenAI group = %+v", openai)
	}
	if _, ok := byName["Anthropic"]; !ok {
		t.Fatalf("Anthropic group missing: %+v", result.Providers)
	}
}

func TestCreateMapsRepositoryFailure(t *testing.T) {
	repo := &fakeRepository{usable: map[int64]ModelRef{9: testRef(9)}, createID: 1}
	repo.usableErr = errors.New("db down")
	service := newTestService(repo)
	if _, err := service.Create(context.Background(), CreateInput{
		Name: "Support", SystemPrompt: "help", Scene: SceneConversation,
		AgentType: pricing.KindChat, ProviderModelID: 9,
	}); err == nil {
		t.Fatal("repository failure was ignored")
	}
}
