package dictionary

import (
	"context"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	dictionaries map[string]Dictionary
	items        map[int64][]Item
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]ListItem, int64, error) {
	return []ListItem{}, 0, nil
}
func (f *fakeRepository) Find(_ context.Context, id int64) (Dictionary, error) {
	for _, v := range f.dictionaries {
		if v.ID == id {
			return v, nil
		}
	}
	return Dictionary{}, gorm.ErrRecordNotFound
}
func (f *fakeRepository) FindByCode(_ context.Context, code string) (Dictionary, error) {
	v, ok := f.dictionaries[code]
	if !ok {
		return Dictionary{}, gorm.ErrRecordNotFound
	}
	return v, nil
}
func (f *fakeRepository) Items(_ context.Context, id int64, enabled bool) ([]Item, error) {
	result := []Item{}
	for _, v := range f.items[id] {
		if !enabled || v.IsEnabled == yesno.Yes {
			result = append(result, v)
		}
	}
	return result, nil
}
func (*fakeRepository) Create(context.Context, *Dictionary) error                   { return nil }
func (*fakeRepository) Update(context.Context, int64, UpdateInput, time.Time) error { return nil }
func (*fakeRepository) UpdateStatus(context.Context, int64, int16, time.Time) error { return nil }
func (*fakeRepository) Delete(context.Context, int64) error                         { return nil }
func (*fakeRepository) CreateItem(context.Context, Item) error                      { return nil }
func (*fakeRepository) FindItem(context.Context, int64, int64) (Item, error) {
	return Item{}, gorm.ErrRecordNotFound
}
func (*fakeRepository) UpdateItem(context.Context, int64, int64, UpdateItemInput, time.Time) error {
	return nil
}
func (*fakeRepository) UpdateItemStatus(context.Context, int64, int64, int16, time.Time) error {
	return nil
}
func (*fakeRepository) DeleteItem(context.Context, int64, int64) error   { return nil }
func (*fakeRepository) CountItems(context.Context, int64) (int64, error) { return 0, nil }

func TestOptionsUsesRequestedLanguageAndChineseFallback(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"user.gender": {ID: 1, Code: "user.gender", IsEnabled: yesno.Yes}}, items: map[int64][]Item{1: {{Value: "male", LabelZH: "男", LabelEN: "Male", IsEnabled: yesno.Yes}, {Value: "unknown", LabelZH: "未知", LabelEN: "", IsEnabled: yesno.Yes}}}}
	result, err := NewService(repo).Options(context.Background(), []string{"user.gender"}, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if result["user.gender"][0].Label != "Male" || result["user.gender"][1].Label != "未知" {
		t.Fatalf("options=%+v", result)
	}
}

func TestOptionsRejectsDisabledAndDuplicateCodes(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"user.gender": {ID: 1, Code: "user.gender", IsEnabled: yesno.No}}, items: map[int64][]Item{}}
	service := NewService(repo)
	if _, err := service.Options(context.Background(), []string{"user.gender"}, "zh-CN"); err == nil {
		t.Fatal("disabled dictionary was accepted")
	}
	repo.dictionaries["user.gender"] = Dictionary{ID: 1, Code: "user.gender", IsEnabled: yesno.Yes}
	if _, err := service.Options(context.Background(), []string{"user.gender", "user.gender"}, "zh-CN"); err == nil {
		t.Fatal("duplicate codes were accepted")
	}
}

func TestDeleteProtectsBuiltinDictionary(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"system.fixed": {ID: 1, Code: "system.fixed", IsBuiltin: yesno.Yes}}, items: map[int64][]Item{}}
	if err := NewService(repo).Delete(context.Background(), 1); err == nil {
		t.Fatal("builtin dictionary was deletable")
	}
}
