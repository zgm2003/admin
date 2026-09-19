package notification

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
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

func TestMailboxServiceMapsPublicErrors(t *testing.T) {
	dependencyErr := errors.New("database unavailable")
	validSetting := mailboxServiceSettingStub{record: sharedsetting.Record{
		Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "180", ValueType: sharedsetting.ValueTypeNumber,
		IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes,
	}}
	tests := []struct {
		name       string
		service    *Service
		call       func(*Service) error
		wantStatus int
	}{
		{
			name:       "invalid mailbox query",
			service:    NewService(&mailboxServiceRepositoryStub{}, validSetting),
			call:       func(service *Service) error { _, err := service.List(context.Background(), MailboxQuery{}); return err },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "retention setting unavailable",
			service:    NewService(&mailboxServiceRepositoryStub{}, mailboxServiceSettingStub{err: dependencyErr}),
			call:       func(service *Service) error { _, err := service.Summary(context.Background(), 1, 1); return err },
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:    "mailbox query unavailable",
			service: NewService(&mailboxServiceRepositoryStub{listErr: dependencyErr}, validSetting),
			call: func(service *Service) error {
				_, err := service.List(context.Background(), MailboxQuery{PlatformID: 1, UserID: 1, Limit: 20})
				return err
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "notification missing",
			service:    NewService(&mailboxServiceRepositoryStub{mutationErr: gorm.ErrRecordNotFound}, validSetting),
			call:       func(service *Service) error { return service.Read(context.Background(), 1, 1, 99) },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "notification mutation unavailable",
			service:    NewService(&mailboxServiceRepositoryStub{mutationErr: dependencyErr}, validSetting),
			call:       func(service *Service) error { return service.Delete(context.Background(), 1, 1, 99) },
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "read all unavailable",
			service:    NewService(&mailboxServiceRepositoryStub{readAllErr: dependencyErr}, validSetting),
			call:       func(service *Service) error { return service.ReadAll(context.Background(), 1, 1) },
			wantStatus: http.StatusServiceUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call(test.service)
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) || applicationError.HTTPStatus != test.wantStatus {
				t.Fatalf("error=%v status=%v want=%d", err, applicationError, test.wantStatus)
			}
		})
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

type mailboxServiceRepositoryStub struct {
	listErr     error
	mutationErr error
	readAllErr  error
}

func (s *mailboxServiceRepositoryStub) CreateForUsers(context.Context, CreateForUsersInput) (Notification, error) {
	return Notification{}, nil
}
func (s *mailboxServiceRepositoryStub) ListMailbox(context.Context, MailboxQuery, time.Time) (MailboxPage, error) {
	return MailboxPage{}, s.listErr
}
func (s *mailboxServiceRepositoryStub) SummaryMailbox(context.Context, int64, int64, time.Time) (MailboxSummary, error) {
	return MailboxSummary{}, s.listErr
}
func (s *mailboxServiceRepositoryStub) ReadNotification(context.Context, int64, int64, int64, time.Time) (bool, error) {
	return false, s.mutationErr
}
func (s *mailboxServiceRepositoryStub) ReadAllNotifications(context.Context, int64, int64, time.Time, time.Time) (bool, error) {
	return false, s.readAllErr
}
func (s *mailboxServiceRepositoryStub) DeleteNotification(context.Context, int64, int64, int64, time.Time) (bool, error) {
	return false, s.mutationErr
}

type mailboxServiceSettingStub struct {
	record sharedsetting.Record
	err    error
}

func (s mailboxServiceSettingStub) FindByKey(context.Context, string) (sharedsetting.Record, error) {
	return s.record, s.err
}

func (s *notificationRepositoryStub) CreateForUsers(_ context.Context, input CreateForUsersInput) (Notification, error) {
	s.calls++
	s.input = input
	if s.err != nil {
		return Notification{}, s.err
	}
	return Notification{ID: 99}, nil
}
