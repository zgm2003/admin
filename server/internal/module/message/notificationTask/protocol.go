package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"errors"
	"sort"
	"strings"
	"time"
)

type AudienceType string

const (
	AudienceUser     AudienceType = "user"
	AudienceRole     AudienceType = "role"
	AudiencePlatform AudienceType = "platform"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusScheduled  Status = "scheduled"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

var (
	ErrNotDraft          = errors.New("notification task is not draft")
	ErrInvalidTransition = errors.New("notification task transition is invalid")
	ErrInvalidFacts      = errors.New("notification task facts are invalid")
)

type DraftInput struct {
	PlatformID   int64
	Title        string
	ContentHTML  string
	Variant      notification.Variant
	Priority     notification.Priority
	LinkType     notification.LinkType
	Link         string
	AudienceType AudienceType
	TargetIDs    []int64
	ScheduledAt  *time.Time
}

type ListQuery struct {
	Page         int
	PageSize     int
	PlatformID   *int64
	Status       Status
	AudienceType AudienceType
	Keyword      string
	From         *time.Time
	To           *time.Time
}

func NormalizeDraft(input DraftInput) (DraftInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	cleaned, err := notification.SanitizeContent(input.ContentHTML)
	if err != nil {
		return DraftInput{}, err
	}
	input.ContentHTML = cleaned
	summary, err := notification.SummaryFromHTML(cleaned)
	if err != nil {
		return DraftInput{}, err
	}
	if input.PlatformID <= 0 {
		return DraftInput{}, errors.New("platform is invalid")
	}
	if err := notification.ValidateContent(notification.Content{Title: input.Title, ContentHTML: cleaned, Summary: summary, Variant: input.Variant, Priority: input.Priority, LinkType: input.LinkType, Link: input.Link}); err != nil {
		return DraftInput{}, err
	}
	sort.Slice(input.TargetIDs, func(i, j int) bool { return input.TargetIDs[i] < input.TargetIDs[j] })
	targets := input.TargetIDs[:0]
	for _, id := range input.TargetIDs {
		if id <= 0 {
			return DraftInput{}, errors.New("target id is invalid")
		}
		if len(targets) == 0 || targets[len(targets)-1] != id {
			targets = append(targets, id)
		}
	}
	input.TargetIDs = targets
	switch input.AudienceType {
	case AudiencePlatform:
		if len(targets) != 0 {
			return DraftInput{}, errors.New("platform audience cannot have targets")
		}
	case AudienceUser, AudienceRole:
		if len(targets) < 1 || len(targets) > 1000 {
			return DraftInput{}, errors.New("target count must be 1..1000")
		}
	default:
		return DraftInput{}, errors.New("audience type is invalid")
	}
	return input, nil
}
