package cachegeneration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	sharedgeneration "admin/server/internal/shared/cacheGeneration"
)

type fakeRepository struct {
	items []Item
	total int64
	err   error
	query ListQuery
}

func (f *fakeRepository) List(_ context.Context, query ListQuery) ([]Item, int64, error) {
	f.query = query
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.items, f.total, nil
}

type fakeStateEntry struct {
	state sharedgeneration.State
	found bool
	err   error
}

type fakeStateStore struct {
	entries map[string]fakeStateEntry
	err     error
}

func (f *fakeStateStore) Read(_ context.Context, scope sharedgeneration.Scope) (sharedgeneration.State, bool, error) {
	if f.err != nil {
		return sharedgeneration.State{}, false, f.err
	}
	entry, ok := f.entries[scope.Namespace+"/"+scope.ScopeKey]
	if !ok {
		return sharedgeneration.State{}, false, nil
	}
	return entry.state, entry.found, entry.err
}

func readyState(generation int64) sharedgeneration.State {
	return sharedgeneration.State{SchemaVersion: sharedgeneration.SchemaVersion, State: sharedgeneration.StateReady, Generation: generation}
}

func invalidatingState(base int64) sharedgeneration.State {
	return sharedgeneration.State{
		SchemaVersion: sharedgeneration.SchemaVersion, State: sharedgeneration.StateInvalidating,
		BaseGeneration: base, MutationToken: "mutation-token",
	}
}

func TestServiceListSynthesizesStatus(t *testing.T) {
	cases := []struct {
		name     string
		item     Item
		state    sharedgeneration.State
		found    bool
		stateErr error
		want     string
	}{
		{"ready", Item{PendingCount: 0}, readyState(1), true, nil, StatusReady},
		{"pending", Item{PendingCount: 2}, readyState(1), true, nil, StatusPending},
		{"retrying", Item{PendingCount: 1, LatestAttempts: 3}, readyState(1), true, nil, StatusRetrying},
		{"ready generation mismatch", Item{Generation: 2}, readyState(1), true, nil, StatusCorrupt},
		{"invalidating", Item{PendingCount: 0}, invalidatingState(1), true, nil, StatusInvalidating},
		{"missing", Item{}, sharedgeneration.State{}, false, nil, StatusMissing},
		{"corrupt", Item{}, sharedgeneration.State{}, false, sharedgeneration.ErrStateCorrupt, StatusCorrupt},
		{"unavailable", Item{}, sharedgeneration.State{}, false, errors.New("redis down"), StatusUnavailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			item := test.item
			item.Namespace = "system.setting"
			item.ScopeKey = "global"
			if item.Generation == 0 {
				item.Generation = 1
			}
			item.UpdatedAt = time.Now().UTC()
			repository := &fakeRepository{items: []Item{item}, total: 1}
			store := &fakeStateStore{entries: map[string]fakeStateEntry{
				"system.setting/global": {state: test.state, found: test.found, err: test.stateErr},
			}}

			result, err := NewService(repository, store).List(context.Background(), ListQuery{Page: 1, PageSize: 20})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(result.Items) != 1 || result.Items[0].Status != test.want {
				t.Fatalf("status = %+v want %q", result.Items, test.want)
			}
		})
	}
}

func TestServiceListSanitizesLastErrorAndNormalizesTimes(t *testing.T) {
	longError := "line\x01\r\n" + strings.Repeat("x", 600)
	now := time.Now()
	item := Item{
		Namespace: "system.setting", ScopeKey: "global", Generation: 3,
		PendingCount: 1, LatestAttempts: 2, LastError: longError,
		UpdatedAt: now, OldestPendingAt: &now,
	}
	result, err := NewService(&fakeRepository{items: []Item{item}, total: 1}, &fakeStateStore{}).List(
		context.Background(), ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	got := result.Items[0].LastError
	if len([]rune(got)) != maxLastErrorRunes {
		t.Fatalf("last error runes = %d want %d", len([]rune(got)), maxLastErrorRunes)
	}
	if strings.ContainsAny(got, "\x01\r\n") {
		t.Fatalf("last error kept control characters: %q", got)
	}
	if result.Items[0].UpdatedAt.Location() != time.UTC || result.Items[0].OldestPendingAt.Location() != time.UTC {
		t.Fatal("times must be normalized to UTC")
	}
}

func TestServiceListRejectsInvalidQuery(t *testing.T) {
	cases := []ListQuery{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
		{Page: 1, PageSize: 20, PublishState: "unknown"},
		{Page: 1, PageSize: 20, Keyword: strings.Repeat("k", maxKeywordRunes+1)},
	}
	for _, query := range cases {
		_, err := NewService(&fakeRepository{}, &fakeStateStore{}).List(context.Background(), query)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidRequest {
			t.Fatalf("query %+v error = %v want invalid request", query, err)
		}
	}
}

func TestServiceListReturnsDependencyUnavailableWhenPostgresFails(t *testing.T) {
	_, err := NewService(&fakeRepository{err: errors.New("database down")}, &fakeStateStore{}).List(
		context.Background(), ListQuery{Page: 1, PageSize: 20})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("error = %v want dependency unavailable", err)
	}
}

func TestServiceListKeepsPostgresRowsWhenRedisIsUnavailable(t *testing.T) {
	item := Item{Namespace: "system.setting", ScopeKey: "global", Generation: 4, UpdatedAt: time.Now().UTC()}
	result, err := NewService(&fakeRepository{items: []Item{item}, total: 1}, &fakeStateStore{err: errors.New("redis down")}).List(
		context.Background(), ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Generation != 4 || result.Items[0].Status != StatusUnavailable {
		t.Fatalf("items = %+v want preserved postgres row with unavailable status", result.Items)
	}
}
