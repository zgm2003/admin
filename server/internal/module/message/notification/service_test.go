package notification

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCreateForUsersNormalizesContentAndSortedUniqueUsers(t *testing.T) {
	repository := &notificationRepositoryStub{}
	service := NewService(repository)
	now := time.Now().UTC()
	result, err := service.CreateForUsers(context.Background(), CreateForUsersInput{
		PlatformID: 1, SourceType: "system.test", SourceKey: "one", UserIDs: []int64{3, 1, 2, 2},
		Title: " title ", ContentHTML: `<p>Hello   世界<script>bad()</script></p>`, Variant: VariantInfo,
		Priority: PriorityNormal, LinkType: LinkNone, PublishedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 99 || !reflect.DeepEqual(repository.input.UserIDs, []int64{1, 2, 3}) {
		t.Fatalf("result=%+v users=%v", result, repository.input.UserIDs)
	}
	if repository.input.Title != "title" || repository.input.ContentHTML != "<p>Hello   世界</p>" || repository.input.Summary != "Hello 世界" {
		t.Fatalf("normalized input=%+v", repository.input)
	}
}

func TestCreateForUsersRejectsInvalidIdentitySourceAndBounds(t *testing.T) {
	base := CreateForUsersInput{PlatformID: 1, SourceType: "system.test", SourceKey: "one", UserIDs: []int64{1}, Title: "title", ContentHTML: "<p>content</p>", Variant: VariantInfo, Priority: PriorityNormal, LinkType: LinkNone, PublishedAt: time.Now().UTC()}
	tests := []CreateForUsersInput{
		withCreateChange(base, func(value *CreateForUsersInput) { value.PlatformID = 0 }),
		withCreateChange(base, func(value *CreateForUsersInput) { value.SourceType = "Bad_type" }),
		withCreateChange(base, func(value *CreateForUsersInput) { value.SourceKey = "" }),
		withCreateChange(base, func(value *CreateForUsersInput) { value.UserIDs = nil }),
		withCreateChange(base, func(value *CreateForUsersInput) { value.UserIDs = []int64{0} }),
		withCreateChange(base, func(value *CreateForUsersInput) {
			value.UserIDs = make([]int64, 501)
			for i := range value.UserIDs {
				value.UserIDs[i] = int64(i + 1)
			}
		}),
		withCreateChange(base, func(value *CreateForUsersInput) { value.PublishedAt = time.Time{} }),
	}
	for _, input := range tests {
		repository := &notificationRepositoryStub{}
		if _, err := NewService(repository).CreateForUsers(context.Background(), input); err == nil {
			t.Fatalf("CreateForUsers(%+v) error=nil", input)
		}
		if repository.calls != 0 {
			t.Fatal("invalid input reached repository")
		}
	}
}

func withCreateChange(base CreateForUsersInput, change func(*CreateForUsersInput)) CreateForUsersInput {
	change(&base)
	return base
}

type notificationRepositoryStub struct {
	input CreateForUsersInput
	calls int
	err   error
}

func (s *notificationRepositoryStub) CreateForUsers(_ context.Context, input CreateForUsersInput) (Notification, error) {
	s.calls++
	s.input = input
	if s.err != nil {
		return Notification{}, s.err
	}
	return Notification{ID: 99}, nil
}
