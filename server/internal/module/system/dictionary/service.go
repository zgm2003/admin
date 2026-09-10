package dictionary

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)

type repository interface {
	List(context.Context, ListQuery) ([]ListItem, int64, error)
	Find(context.Context, int64) (Dictionary, error)
	FindByCode(context.Context, string) (Dictionary, error)
	Items(context.Context, int64, bool) ([]Item, error)
	Create(context.Context, *Dictionary) error
	Update(context.Context, int64, UpdateInput, time.Time) error
	UpdateStatus(context.Context, int64, int16, time.Time) error
	Delete(context.Context, int64) error
	CreateItem(context.Context, Item) error
	FindItem(context.Context, int64, int64) (Item, error)
	UpdateItem(context.Context, int64, int64, UpdateItemInput, time.Time) error
	UpdateItemStatus(context.Context, int64, int64, int16, time.Time) error
	DeleteItem(context.Context, int64, int64) error
	CountItems(context.Context, int64) (int64, error)
}

type Service struct {
	repository repository
	cache      *optionsCache
}

func NewService(repository repository) *Service { return &Service{repository: repository} }

func (s *Service) SetCache(cache *optionsCache) { s.cache = cache }

func (s *Service) mutate(ctx context.Context, change func(context.Context) error) error {
	if s.cache == nil {
		return change(ctx)
	}
	if err := s.cache.Mutate(ctx, change); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Detail, error) {
	value, err := s.repository.Find(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Detail{}, apperror.NotFound(err)
	}
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(err)
	}
	items, err := s.repository.Items(ctx, id, false)
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(err)
	}
	return Detail{Dictionary: value, Items: items}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.NameZH = strings.TrimSpace(input.NameZH)
	input.NameEN = strings.TrimSpace(input.NameEN)
	if !codePattern.MatchString(input.Code) || len(input.Code) > 128 || input.NameZH == "" || input.NameEN == "" {
		return 0, apperror.InvalidRequest(fmt.Errorf("dictionary input is invalid"))
	}
	if _, err := s.repository.FindByCode(ctx, input.Code); err == nil {
		return 0, apperror.Conflict("error.conflict", nil, fmt.Errorf("dictionary code already exists"))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, apperror.DependencyUnavailable(err)
	}
	now := time.Now().UTC()
	value := Dictionary{Code: input.Code, NameZH: input.NameZH, NameEN: input.NameEN, Description: input.Description, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.mutate(ctx, func(writeContext context.Context) error { return s.repository.Create(writeContext, &value) }); err != nil {
		return 0, err
	}
	return value.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	current, err := s.repository.Find(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if current.IsBuiltin == yesno.Yes { /* labels remain editable */
	}
	input.NameZH = strings.TrimSpace(input.NameZH)
	input.NameEN = strings.TrimSpace(input.NameEN)
	if input.NameZH == "" || input.NameEN == "" {
		return apperror.InvalidRequest(fmt.Errorf("dictionary names are required"))
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, id, input, time.Now().UTC())
	}); err != nil {
		return err
	}
	return nil
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	if _, err := s.repository.Find(ctx, id); errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.UpdateStatus(writeContext, id, int16(status), time.Now().UTC())
	}); err != nil {
		return err
	}
	return nil
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	current, err := s.repository.Find(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if current.IsBuiltin == yesno.Yes {
		return apperror.Conflict("error.conflict", nil, fmt.Errorf("builtin dictionary cannot be deleted"))
	}
	count, err := s.repository.CountItems(ctx, id)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if count > 0 {
		return apperror.Conflict("error.conflict", nil, fmt.Errorf("dictionary has items"))
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error { return s.repository.Delete(writeContext, id) }); err != nil {
		return err
	}
	return nil
}

func (s *Service) Options(ctx context.Context, codes []string, language string) (OptionResult, error) {
	if len(codes) == 0 || len(codes) > 50 {
		return nil, apperror.InvalidRequest(fmt.Errorf("codes are invalid"))
	}
	seen := make(map[string]struct{}, len(codes))
	normalizedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if !codePattern.MatchString(code) || len(code) > 128 {
			return nil, apperror.InvalidRequest(fmt.Errorf("dictionary code is invalid"))
		}
		if _, ok := seen[code]; ok {
			return nil, apperror.InvalidRequest(fmt.Errorf("codes contain duplicate"))
		}
		seen[code] = struct{}{}
		normalizedCodes = append(normalizedCodes, code)
	}
	codes = normalizedCodes
	var fill *cachefill.Lease
	if s.cache != nil {
		defer func() { _ = fill.Release(ctx) }()
		for admission := 0; admission < 25; admission++ {
			if cached, found, err := s.cache.Get(ctx, codes, language); err != nil {
				return nil, apperror.DependencyUnavailable(err)
			} else if found {
				return cached, nil
			}
			var err error
			fill, err = s.cache.acquireFill(ctx, codes, language)
			if err != nil {
				return nil, apperror.DependencyUnavailable(err)
			}
			if fill != nil {
				workContext, cancel := fill.WorkContext(ctx)
				defer cancel()
				ctx = workContext
				break
			}
			if err := cachefill.Wait(ctx); err != nil {
				return nil, apperror.DependencyUnavailable(err)
			}
		}
		if fill == nil {
			return nil, apperror.DependencyUnavailable(fmt.Errorf("dictionary options cache rebuild busy"))
		}
	}
	result := make(OptionResult, len(codes))
	for _, code := range codes {
		dictionary, err := s.repository.FindByCode(ctx, code)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound(err)
		}
		if err != nil {
			return nil, apperror.DependencyUnavailable(err)
		}
		if dictionary.IsEnabled != yesno.Yes {
			return nil, apperror.Conflict("error.conflict", nil, fmt.Errorf("dictionary is disabled"))
		}
		items, err := s.repository.Items(ctx, dictionary.ID, true)
		if err != nil {
			return nil, apperror.DependencyUnavailable(err)
		}
		options := make([]OptionsItem, 0, len(items))
		for _, item := range items {
			label := item.LabelZH
			if strings.HasPrefix(strings.ToLower(language), "en") {
				label = item.LabelEN
			}
			if label == "" {
				label = item.LabelZH
			}
			if label == "" {
				return nil, apperror.DependencyUnavailable(fmt.Errorf("dictionary item label is missing"))
			}
			options = append(options, OptionsItem{Label: label, Value: item.Value})
		}
		result[code] = options
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, codes, language, result); err != nil {
			return nil, apperror.DependencyUnavailable(err)
		}
	}
	return result, nil
}

func (s *Service) CreateItem(ctx context.Context, dictionaryID int64, input CreateItemInput) (int64, error) {
	dictionary, err := s.repository.Find(ctx, dictionaryID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, apperror.NotFound(err)
	}
	if err != nil {
		return 0, apperror.DependencyUnavailable(err)
	}
	if dictionary.IsEnabled != yesno.Yes {
		return 0, apperror.Conflict("error.conflict", nil, fmt.Errorf("dictionary is disabled"))
	}
	input.Value = strings.TrimSpace(input.Value)
	if input.Value == "" || input.LabelZH == "" || input.LabelEN == "" || input.Sort < 0 {
		return 0, apperror.InvalidRequest(fmt.Errorf("dictionary item input is invalid"))
	}
	now := time.Now().UTC()
	value := Item{DictionaryID: dictionaryID, Value: input.Value, LabelZH: input.LabelZH, LabelEN: input.LabelEN, Sort: input.Sort, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.mutate(ctx, func(writeContext context.Context) error { return s.repository.CreateItem(writeContext, value) }); err != nil {
		return 0, err
	}
	return value.ID, nil
}
func (s *Service) UpdateItem(ctx context.Context, dictionaryID, itemID int64, input UpdateItemInput) error {
	if input.LabelZH == "" || input.LabelEN == "" || input.Sort < 0 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary item input is invalid"))
	}
	if _, err := s.repository.FindItem(ctx, dictionaryID, itemID); errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.UpdateItem(writeContext, dictionaryID, itemID, input, time.Now().UTC())
	}); err != nil {
		return err
	}
	return nil
}
func (s *Service) UpdateItemStatus(ctx context.Context, dictionaryID, itemID int64, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	if _, err := s.repository.FindItem(ctx, dictionaryID, itemID); errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.UpdateItemStatus(writeContext, dictionaryID, itemID, int16(status), time.Now().UTC())
	}); err != nil {
		return err
	}
	return nil
}
func (s *Service) DeleteItem(ctx context.Context, dictionaryID, itemID int64) error {
	item, err := s.repository.FindItem(ctx, dictionaryID, itemID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if item.IsBuiltin == yesno.Yes {
		return apperror.Conflict("error.conflict", nil, fmt.Errorf("builtin dictionary item cannot be deleted"))
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.DeleteItem(writeContext, dictionaryID, itemID)
	}); err != nil {
		return err
	}
	return nil
}
