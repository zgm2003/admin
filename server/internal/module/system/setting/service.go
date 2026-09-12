package setting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type repository interface {
	List(context.Context, ListQuery) ([]Record, int64, error)
	Find(context.Context, string) (Record, error)
	Create(context.Context, *Record) error
	Update(context.Context, string, Record) error
	UpdateStatus(context.Context, string, yesno.Value, time.Time) error
	Delete(context.Context, string) error
}

type settingCache interface {
	Get(context.Context, string) (Record, bool, error)
	Set(context.Context, Record) error
	Delete(context.Context, string) error
}

type Service struct {
	repository repository
	cache      settingCache
}

func NewService(repository repository) *Service { return &Service{repository: repository} }

func (s *Service) SetCache(cache settingCache) { s.cache = cache }

func (s *Service) Find(ctx context.Context, key string) (Record, error) {
	key = strings.TrimSpace(key)
	if s.cache != nil {
		if cached, found, err := s.cache.Get(ctx, key); err != nil {
			return Record{}, err
		} else if found {
			return cached, nil
		}
	}
	row, err := s.repository.Find(ctx, key)
	if err != nil {
		return Record{}, err
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, row); err != nil {
			return Record{}, err
		}
	}
	return row, nil
}

func (s *Service) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	row, err := s.Find(ctx, key)
	if err != nil {
		return sharedsetting.Record{}, err
	}
	return sharedsetting.Record{Key: row.Key, Value: row.Value, ValueType: row.ValueType, Description: row.Description, IsEnabled: row.IsEnabled, IsBuiltin: row.IsBuiltin}, nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	rows, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{Items: rows, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	input.Key, input.Value, input.Description = strings.TrimSpace(input.Key), strings.TrimSpace(input.Value), strings.TrimSpace(input.Description)
	if err := validateInput(input.Key, input.Value, input.ValueType, input.Description); err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	if row, err := s.repository.Find(ctx, input.Key); err == nil && row.Key != "" {
		return 0, apperror.Conflict("error.conflict", nil, ErrConflict)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return 0, apperror.DependencyUnavailable(err)
	}
	now := time.Now().UTC()
	row := Record{Key: input.Key, Value: input.Value, ValueType: input.ValueType, Description: input.Description, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.Create(ctx, &row); err != nil {
		if errors.Is(err, ErrConflict) {
			return 0, apperror.Conflict("error.conflict", nil, err)
		}
		return 0, apperror.DependencyUnavailable(err)
	}
	return row.ID, nil
}

func (s *Service) Update(ctx context.Context, key string, input UpdateInput) error {
	key = strings.TrimSpace(key)
	current, err := s.repository.Find(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	input.Value, input.Description = strings.TrimSpace(input.Value), strings.TrimSpace(input.Description)
	if err := validateInput(key, input.Value, input.ValueType, input.Description); err != nil {
		return apperror.InvalidRequest(err)
	}
	current.Value, current.ValueType, current.Description, current.UpdatedAt = input.Value, input.ValueType, input.Description, time.Now().UTC()
	if err := s.repository.Update(ctx, key, current); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if s.cache != nil {
		if err := s.cache.Delete(ctx, key); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, key string, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	if _, err := s.repository.Find(ctx, strings.TrimSpace(key)); errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.repository.UpdateStatus(ctx, strings.TrimSpace(key), status, time.Now().UTC()); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if s.cache != nil {
		if err := s.cache.Delete(ctx, strings.TrimSpace(key)); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	row, err := s.repository.Find(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if row.IsBuiltin == yesno.Yes {
		return apperror.Conflict("error.conflict", nil, fmt.Errorf("builtin setting cannot be deleted"))
	}
	if err := s.repository.Delete(ctx, key); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if s.cache != nil {
		if err := s.cache.Delete(ctx, key); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
}

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

func validateInput(key, value string, valueType int, description string) error {
	if !keyPattern.MatchString(key) || utf8.RuneCountInString(key) > 128 || utf8.RuneCountInString(description) > 512 || !validSettingValue(value, valueType) {
		return fmt.Errorf("setting input is invalid")
	}
	return nil
}
func validSettingValue(value string, valueType int) bool {
	switch valueType {
	case ValueTypeString:
		return utf8.RuneCountInString(value) <= 4096
	case ValueTypeNumber:
		_, err := strconv.Atoi(value)
		return err == nil
	case ValueTypeBool:
		return value == "0" || value == "1" || value == "true" || value == "false"
	case ValueTypeJSON:
		return json.Valid([]byte(value))
	default:
		return false
	}
}
