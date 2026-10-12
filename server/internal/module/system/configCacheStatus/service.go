package cachegeneration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	sharedgeneration "admin/server/internal/shared/cacheGeneration"
)

type repository interface {
	List(context.Context, ListQuery) ([]Item, int64, error)
}

type stateStore interface {
	Read(context.Context, sharedgeneration.Scope) (sharedgeneration.State, bool, error)
}

type Service struct {
	repository repository
	states     stateStore
}

func NewService(repository repository, states stateStore) *Service {
	return &Service{repository: repository, states: states}
}

// List 以 PostgreSQL 分页结果为准，再按当页每个 scope 合成 Redis state；
// Redis 不可用不隐藏 PostgreSQL 数据，只标记 unavailable。
func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if err := validateListQuery(query); err != nil {
		return ListResult{}, apperror.InvalidRequest(err)
	}
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	result := ListResult{Items: make([]Item, 0, len(items)), Total: total, Page: query.Page, PageSize: query.PageSize}
	for _, item := range items {
		item.Status = s.resolveStatus(ctx, item)
		item.LastError = sanitizeLastError(item.LastError)
		item.UpdatedAt = item.UpdatedAt.UTC()
		if item.OldestPendingAt != nil {
			utc := item.OldestPendingAt.UTC()
			item.OldestPendingAt = &utc
		}
		if item.LatestPublishedAt != nil {
			utc := item.LatestPublishedAt.UTC()
			item.LatestPublishedAt = &utc
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Service) resolveStatus(ctx context.Context, item Item) string {
	publishState := publishStateOf(item)
	if s.states == nil {
		return StatusUnavailable
	}
	scope, err := sharedgeneration.NewScope(item.Namespace, item.ScopeKey)
	if err != nil {
		return StatusUnavailable
	}
	state, found, err := s.states.Read(ctx, scope)
	if err != nil {
		if errors.Is(err, sharedgeneration.ErrStateCorrupt) {
			return StatusCorrupt
		}
		return StatusUnavailable
	}
	if !found {
		return StatusMissing
	}
	switch state.State {
	case sharedgeneration.StateInvalidating:
		return StatusInvalidating
	case sharedgeneration.StateReady:
		if state.Generation != item.Generation {
			return StatusCorrupt
		}
		return publishState
	default:
		return StatusCorrupt
	}
}

func publishStateOf(item Item) string {
	if item.PendingCount == 0 {
		return PublishStateReady
	}
	if item.LatestAttempts > 0 {
		return PublishStateRetrying
	}
	return PublishStatePending
}

func validateListQuery(query ListQuery) error {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return fmt.Errorf("pagination is invalid")
	}
	if utf8.RuneCountInString(query.Keyword) > maxKeywordRunes {
		return fmt.Errorf("keyword is invalid")
	}
	if query.PublishState != "" && !IsPublishState(query.PublishState) {
		return fmt.Errorf("publishState is invalid")
	}
	return nil
}

// sanitizeLastError 只保留可打印字符并限制为 512 字符；写入端已脱敏。
func sanitizeLastError(message string) string {
	cleaned := make([]rune, 0, len(message))
	for _, character := range message {
		if character < 0x20 || character == 0x7f {
			character = ' '
		}
		cleaned = append(cleaned, character)
	}
	text := strings.TrimSpace(string(cleaned))
	runes := []rune(text)
	if len(runes) > maxLastErrorRunes {
		text = string(runes[:maxLastErrorRunes])
	}
	return text
}
