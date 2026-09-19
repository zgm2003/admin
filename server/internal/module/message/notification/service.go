package notification

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

var sourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)

type CreateForUsersInput struct {
	PlatformID   int64
	SourceTaskID *int64
	SourceType   string
	SourceKey    string
	UserIDs      []int64
	Title        string
	ContentHTML  string
	Summary      string
	Variant      Variant
	Priority     Priority
	LinkType     LinkType
	Link         string
	PublishedAt  time.Time
}

type createForUsersRepository interface {
	CreateForUsers(context.Context, CreateForUsersInput) (Notification, error)
}

type Service struct {
	repository createForUsersRepository
	mailbox    mailboxRepository
	settings   sharedsetting.Reader
}

func NewService(repository createForUsersRepository, settings ...sharedsetting.Reader) *Service {
	service := &Service{repository: repository}
	if mailbox, ok := repository.(mailboxRepository); ok {
		service.mailbox = mailbox
	}
	if len(settings) > 0 {
		service.settings = settings[0]
	}
	return service
}

func (s *Service) CreateForUsers(ctx context.Context, input CreateForUsersInput) (Notification, error) {
	if s == nil || s.repository == nil {
		return Notification{}, errors.New("notification repository is unavailable")
	}
	input.Title = strings.TrimSpace(input.Title)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	if input.PlatformID <= 0 || !sourceTypePattern.MatchString(input.SourceType) || input.SourceKey == "" || len(input.SourceKey) > 128 || input.PublishedAt.IsZero() {
		return Notification{}, errors.New("notification source metadata is invalid")
	}
	if input.SourceTaskID != nil && *input.SourceTaskID <= 0 {
		return Notification{}, errors.New("notification source task is invalid")
	}
	input.UserIDs = normalizeUserIDs(input.UserIDs)
	if len(input.UserIDs) < 1 || len(input.UserIDs) > 500 {
		return Notification{}, errors.New("notification user count must be 1..500")
	}
	cleaned, err := SanitizeContent(input.ContentHTML)
	if err != nil {
		return Notification{}, err
	}
	input.ContentHTML = cleaned
	input.Summary, err = SummaryFromHTML(cleaned)
	if err != nil {
		return Notification{}, err
	}
	if err := ValidateContent(Content{Title: input.Title, ContentHTML: input.ContentHTML, Summary: input.Summary, Variant: input.Variant, Priority: input.Priority, LinkType: input.LinkType, Link: input.Link}); err != nil {
		return Notification{}, err
	}
	return s.repository.CreateForUsers(ctx, input)
}

type MailboxFilter string

const (
	MailboxAll    MailboxFilter = "all"
	MailboxUnread MailboxFilter = "unread"
)

type MailboxQuery struct {
	PlatformID int64
	UserID     int64
	BeforeID   int64
	Limit      int
	Filter     MailboxFilter
	Variant    Variant
	Priority   Priority
}

type MailboxItem struct {
	Notification
	IsRead bool
}

type MailboxPage struct {
	Items        []MailboxItem
	NextBeforeID *int64
}

type SummaryItem struct {
	ID          int64
	Title       string
	Summary     string
	Variant     Variant
	Priority    Priority
	LinkType    LinkType
	Link        string
	PublishedAt time.Time
	IsRead      bool
}

type MailboxSummary struct {
	UnreadCount int64
	Recent      []SummaryItem
}

type mailboxRepository interface {
	ListMailbox(context.Context, MailboxQuery, time.Time) (MailboxPage, error)
	SummaryMailbox(context.Context, int64, int64, time.Time) (MailboxSummary, error)
	ReadNotification(context.Context, int64, int64, int64, time.Time) (bool, error)
	ReadAllNotifications(context.Context, int64, int64, time.Time, time.Time) (bool, error)
	DeleteNotification(context.Context, int64, int64, int64, time.Time) (bool, error)
}

func (s *Service) List(ctx context.Context, query MailboxQuery) (MailboxPage, error) {
	if query.PlatformID <= 0 || query.UserID <= 0 || query.BeforeID < 0 || query.Limit < 1 || query.Limit > 50 {
		return MailboxPage{}, apperror.InvalidRequest(errors.New("invalid notification mailbox query"))
	}
	if query.Filter == "" {
		query.Filter = MailboxAll
	}
	if query.Filter != MailboxAll && query.Filter != MailboxUnread {
		return MailboxPage{}, apperror.InvalidRequest(errors.New("invalid notification mailbox filter"))
	}
	cutoff, err := s.retentionCutoff(ctx)
	if err != nil {
		return MailboxPage{}, mailboxDependencyError(err)
	}
	page, err := s.mailbox.ListMailbox(ctx, query, cutoff)
	return page, mailboxDependencyError(err)
}

func (s *Service) Summary(ctx context.Context, platformID, userID int64) (MailboxSummary, error) {
	if platformID <= 0 || userID <= 0 {
		return MailboxSummary{}, apperror.InvalidRequest(errors.New("invalid notification mailbox identity"))
	}
	cutoff, err := s.retentionCutoff(ctx)
	if err != nil {
		return MailboxSummary{}, mailboxDependencyError(err)
	}
	result, err := s.mailbox.SummaryMailbox(ctx, platformID, userID, cutoff)
	return result, mailboxDependencyError(err)
}

func (s *Service) Read(ctx context.Context, platformID, userID, notificationID int64) error {
	if platformID <= 0 || userID <= 0 || notificationID <= 0 {
		return apperror.InvalidRequest(errors.New("invalid notification read"))
	}
	if s == nil || s.mailbox == nil {
		return apperror.DependencyUnavailable(errors.New("notification mailbox dependencies are unavailable"))
	}
	_, err := s.mailbox.ReadNotification(ctx, platformID, userID, notificationID, time.Now().UTC())
	return mailboxMutationError(err)
}

func (s *Service) ReadAll(ctx context.Context, platformID, userID int64) error {
	if platformID <= 0 || userID <= 0 {
		return apperror.InvalidRequest(errors.New("invalid notification read all"))
	}
	cutoff, err := s.retentionCutoff(ctx)
	if err != nil {
		return mailboxDependencyError(err)
	}
	_, err = s.mailbox.ReadAllNotifications(ctx, platformID, userID, cutoff, time.Now().UTC())
	return mailboxDependencyError(err)
}

func (s *Service) Delete(ctx context.Context, platformID, userID, notificationID int64) error {
	if platformID <= 0 || userID <= 0 || notificationID <= 0 {
		return apperror.InvalidRequest(errors.New("invalid notification delete"))
	}
	if s == nil || s.mailbox == nil {
		return apperror.DependencyUnavailable(errors.New("notification mailbox dependencies are unavailable"))
	}
	_, err := s.mailbox.DeleteNotification(ctx, platformID, userID, notificationID, time.Now().UTC())
	return mailboxMutationError(err)
}

func mailboxMutationError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	return mailboxDependencyError(err)
}

func mailboxDependencyError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	return apperror.DependencyUnavailable(err)
}

func (s *Service) retentionCutoff(ctx context.Context) (time.Time, error) {
	if s.mailbox == nil || s.settings == nil {
		return time.Time{}, errors.New("notification mailbox dependencies are unavailable")
	}
	record, err := s.settings.FindByKey(ctx, sharedsetting.MessageNotificationRetentionDaysKey)
	if err != nil {
		return time.Time{}, err
	}
	if record.Key != sharedsetting.MessageNotificationRetentionDaysKey || record.IsEnabled != yesno.Yes || record.IsBuiltin != yesno.Yes {
		return time.Time{}, errors.New("notification retention setting must be enabled and builtin")
	}
	days, err := record.Number()
	if err != nil || days < 30 || days > 3650 {
		return time.Time{}, errors.New("notification retention setting must be an integer from 30 to 3650")
	}
	return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
}

func normalizeUserIDs(values []int64) []int64 {
	result := append([]int64(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	compacted := result[:0]
	for _, value := range result {
		if value <= 0 {
			return nil
		}
		if len(compacted) == 0 || compacted[len(compacted)-1] != value {
			compacted = append(compacted, value)
		}
	}
	return compacted
}
