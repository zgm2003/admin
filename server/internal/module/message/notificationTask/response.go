package notificationtask

import "time"

type taskResponse struct {
	ID                int64        `json:"id"`
	PlatformID        int64        `json:"platformId"`
	NotificationID    *int64       `json:"notificationId"`
	Title             string       `json:"title"`
	ContentHTML       string       `json:"contentHtml"`
	Summary           string       `json:"summary"`
	Variant           string       `json:"variant"`
	Priority          string       `json:"priority"`
	LinkType          string       `json:"linkType"`
	Link              string       `json:"link"`
	AudienceType      AudienceType `json:"audienceType"`
	TargetIDs         []int64      `json:"targetIds"`
	ScheduledAt       *time.Time   `json:"scheduledAt"`
	AudienceMaxUserID *int64       `json:"audienceMaxUserId"`
	SubmittedAt       *time.Time   `json:"submittedAt"`
	PublishedAt       *time.Time   `json:"publishedAt"`
	CompletedAt       *time.Time   `json:"completedAt"`
	CanceledAt        *time.Time   `json:"canceledAt"`
	FailedAt          *time.Time   `json:"failedAt"`
	FailureMessage    *string      `json:"failureMessage"`
	Status            Status       `json:"status"`
	GeneratedCount    int64        `json:"generatedCount"`
	CreatedBy         int64        `json:"createdBy"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
}

func taskDTO(v Task) taskResponse {
	return taskResponse{
		ID: v.ID, PlatformID: v.PlatformID, NotificationID: v.NotificationID, Title: v.Title,
		ContentHTML: v.ContentHTML, Summary: v.Summary, Variant: string(v.Variant), Priority: string(v.Priority),
		LinkType: string(v.LinkType), Link: v.Link, AudienceType: v.AudienceType, TargetIDs: append([]int64{}, v.TargetIDs...),
		ScheduledAt: v.ScheduledAt, AudienceMaxUserID: v.AudienceMaxUserID, SubmittedAt: v.SubmittedAt,
		PublishedAt: v.PublishedAt, CompletedAt: v.CompletedAt, CanceledAt: v.CanceledAt, FailedAt: v.FailedAt,
		FailureMessage: v.FailureMessage, Status: v.Status, GeneratedCount: v.GeneratedCount, CreatedBy: v.CreatedBy,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

type taskListItemResponse struct {
	ID             int64        `json:"id"`
	PlatformID     int64        `json:"platformId"`
	Title          string       `json:"title"`
	Variant        string       `json:"variant"`
	Priority       string       `json:"priority"`
	AudienceType   AudienceType `json:"audienceType"`
	ScheduledAt    *time.Time   `json:"scheduledAt"`
	SubmittedAt    *time.Time   `json:"submittedAt"`
	CompletedAt    *time.Time   `json:"completedAt"`
	Status         Status       `json:"status"`
	GeneratedCount int64        `json:"generatedCount"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

func taskListDTO(v Task) taskListItemResponse {
	return taskListItemResponse{
		ID: v.ID, PlatformID: v.PlatformID, Title: v.Title, Variant: string(v.Variant), Priority: string(v.Priority),
		AudienceType: v.AudienceType, ScheduledAt: v.ScheduledAt, SubmittedAt: v.SubmittedAt,
		CompletedAt: v.CompletedAt, Status: v.Status, GeneratedCount: v.GeneratedCount, UpdatedAt: v.UpdatedAt,
	}
}

type taskListResponse struct {
	List     []taskListItemResponse `json:"list"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}
type optionResponse struct {
	Items       []Option `json:"items"`
	NextAfterID *int64   `json:"nextAfterId"`
}
