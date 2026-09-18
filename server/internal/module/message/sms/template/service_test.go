package template

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	rows      []Model
	findErr   error
	writeErr  error
	updated   Model
	updatedID int64
	statusID  int64
	status    int16
}

func (f *fakeRepository) List(context.Context) ([]Model, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return append([]Model(nil), f.rows...), nil
}

func (f *fakeRepository) FindByID(_ context.Context, id int64) (Model, error) {
	if f.findErr != nil {
		return Model{}, f.findErr
	}
	for _, row := range f.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return Model{}, gorm.ErrRecordNotFound
}

func (f *fakeRepository) Update(_ context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if f.writeErr != nil {
		return cachegeneration.MutationResult{}, f.writeErr
	}
	f.updatedID = value.ID
	f.updated = *value
	f.updated.UpdatedAt = now
	return templateMutation(expected), nil
}

func (f *fakeRepository) UpdateStatus(_ context.Context, id int64, status int16, expected int64, _ time.Time) (cachegeneration.MutationResult, error) {
	if f.writeErr != nil {
		return cachegeneration.MutationResult{}, f.writeErr
	}
	f.statusID = id
	f.status = status
	return templateMutation(expected), nil
}

type passthroughRuntime struct{ generation int64 }

func (r *passthroughRuntime) Mutate(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if r.generation == 0 {
		r.generation = 1
	}
	result, err := change(ctx, r.generation)
	if err == nil && result.Changed {
		r.generation = result.Generation
	}
	return err
}

func templateMutation(expected int64) cachegeneration.MutationResult {
	return cachegeneration.MutationResult{Changed: true, Generation: expected + 1, OutboxID: expected}
}

func newMutationService(repository repository) *Service {
	service := NewService(repository)
	service.SetRuntimeCoordinator(&passthroughRuntime{})
	return service
}

func seededRows() []Model {
	rows := make([]Model, 0, 4)
	for index, fixed := range FixedCatalog() {
		rows = append(rows, Model{
			ID:                int64(index + 1),
			Scene:             fixed.Scene,
			Name:              fixed.Name,
			TencentTemplateID: "1000" + fixed.Scene[:1],
			Content:           "{1} 有效期 {2} 分钟",
			VariableKeys:      jsonOf(fixed.VariableKeys),
			ExampleVariables:  jsonOf(map[string]string{"code": "123456", "ttl_minutes": "5"}),
			IsEnabled:         yesno.No,
			CreatedAt:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	return rows
}

func appErrorCode(err error) int {
	var appError *apperror.Error
	if errors.As(err, &appError) {
		return appError.Code
	}
	return 0
}

func TestListReturnsTheFourCatalogScenesInOrder(t *testing.T) {
	safes, err := NewService(&fakeRepository{rows: seededRows()}).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []string{SceneLogin, SceneForget, SceneBindPhone, SceneChangePassword}
	if len(safes) != len(want) {
		t.Fatalf("templates = %d", len(safes))
	}
	for index, scene := range want {
		if safes[index].Scene != scene || safes[index].Name == "" {
			t.Fatalf("template[%d] = %+v", index, safes[index])
		}
		if strings.Join(safes[index].VariableKeys, ",") != "code,ttl_minutes" {
			t.Fatalf("parameter keys = %v", safes[index].VariableKeys)
		}
	}
}

func TestListFailsClosedWhenTheCatalogIsIncomplete(t *testing.T) {
	rows := seededRows()
	rows = rows[:2]
	_, err := NewService(&fakeRepository{rows: rows}).List(context.Background())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("List() error = %v, want dependency unavailable", err)
	}
}

func TestUpdateRejectsSceneChangesAndInvalidFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*UpdateInput)
	}{
		{name: "scene changed", mutate: func(input *UpdateInput) { input.Scene = SceneForget }},
		{name: "unknown scene", mutate: func(input *UpdateInput) { input.Scene = "test" }},
		{name: "blank name", mutate: func(input *UpdateInput) { input.Name = "   " }},
		{name: "overlong name", mutate: func(input *UpdateInput) { input.Name = strings.Repeat("名", 129) }},
		{name: "non numeric template id", mutate: func(input *UpdateInput) { input.TencentTemplateID = "abc" }},
		{name: "duplicate variable keys", mutate: func(input *UpdateInput) { input.VariableKeys = []string{"code", "code"} }},
		{name: "missing example variable", mutate: func(input *UpdateInput) { input.ExampleVariables = map[string]string{"code": "123456"} }},
		{name: "blank example variable", mutate: func(input *UpdateInput) {
			input.ExampleVariables = map[string]string{"code": "123456", "ttl_minutes": " "}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{rows: seededRows()}
			input := validUpdate()
			test.mutate(&input)
			_, err := newMutationService(repository).Update(context.Background(), 1, input)
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("Update() error = %v, want invalid request", err)
			}
			if repository.updatedID != 0 {
				t.Fatal("invalid input reached the repository")
			}
		})
	}
}

func TestUpdatePersistsAllowedFieldsAndTrims(t *testing.T) {
	repository := &fakeRepository{rows: seededRows()}
	input := validUpdate()
	input.Name = "  登录短信验证码  "
	safe, err := newMutationService(repository).Update(context.Background(), 1, input)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repository.updated.Scene != SceneLogin || repository.updated.Name != "登录短信验证码" ||
		repository.updated.TencentTemplateID != "1234567" {
		t.Fatalf("updated = %+v", repository.updated)
	}
	if safe.Name != "登录短信验证码" || safe.TencentTemplateID != "1234567" || safe.Scene != SceneLogin {
		t.Fatalf("safe = %+v", safe)
	}
}

func TestUpdateRequiresAValidTemplateBeforeStayingEnabled(t *testing.T) {
	rows := seededRows()
	rows[0].IsEnabled = yesno.Yes
	repository := &fakeRepository{rows: rows}
	input := validUpdate()
	input.TencentTemplateID = ""
	if _, err := newMutationService(repository).Update(context.Background(), 1, input); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("Update() error = %v, want invalid request", err)
	}
}

func TestUpdateReportsMissingTemplateAsNotFound(t *testing.T) {
	_, err := newMutationService(&fakeRepository{rows: seededRows()}).Update(context.Background(), 999, validUpdate())
	if appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Update() error = %v, want not found", err)
	}
}

func TestStatusEnableRequiresNumericIDAndCompleteExampleVariables(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]Model)
	}{
		{name: "empty template id", mutate: func(rows []Model) { rows[0].TencentTemplateID = "" }},
		{name: "non numeric template id", mutate: func(rows []Model) { rows[0].TencentTemplateID = "unknown" }},
		{name: "incomplete example variables", mutate: func(rows []Model) { rows[0].ExampleVariables = jsonOf(map[string]string{"code": "1"}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := seededRows()
			test.mutate(rows)
			repository := &fakeRepository{rows: rows}
			err := newMutationService(repository).UpdateStatus(context.Background(), 1, yesno.Yes)
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("UpdateStatus() error = %v, want invalid request", err)
			}
			if repository.statusID != 0 {
				t.Fatal("invalid status reached the repository")
			}
		})
	}
}

func TestStatusDisableIsAlwaysAllowed(t *testing.T) {
	rows := seededRows()
	rows[0].TencentTemplateID = ""
	repository := &fakeRepository{rows: rows}
	if err := newMutationService(repository).UpdateStatus(context.Background(), 1, yesno.No); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if repository.statusID != 1 || repository.status != 0 {
		t.Fatalf("status write = %d/%d", repository.statusID, repository.status)
	}
}

func TestUpdateMapsRepositoryFailureToDependencyUnavailable(t *testing.T) {
	repository := &fakeRepository{rows: seededRows(), writeErr: errors.New("database unavailable")}
	_, err := newMutationService(repository).Update(context.Background(), 1, validUpdate())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Update() error = %v, want dependency unavailable", err)
	}
}

func validUpdate() UpdateInput {
	return UpdateInput{
		Scene:             SceneLogin,
		Name:              "登录验证码",
		TencentTemplateID: "1234567",
		Content:           "{1} 有效期 {2} 分钟",
		VariableKeys:      []string{"code", "ttl_minutes"},
		ExampleVariables:  map[string]string{"code": "123456", "ttl_minutes": "5"},
	}
}
