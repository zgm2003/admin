package phone

import (
	"context"
	"strconv"
	"time"
)

type sendCodeResponse struct {
	ChallengeID        string    `json:"challengeId"`
	ExpiresAt          time.Time `json:"expiresAt"`
	ResendAfterSeconds int       `json:"resendAfterSeconds"`
}

type phoneResponse struct {
	Phone string `json:"phone"`
}

type phoneChangeLogResponse struct {
	ActionLabel string       `json:"actionLabel"`
	ID          int64        `json:"id"`
	Action      ChangeAction `json:"action"`
	OldPhone    *string      `json:"oldPhone"`
	NewPhone    string       `json:"newPhone"`
	Platform    string       `json:"platform"`
	CreatedAt   string       `json:"createdAt"`
}
type phoneChangeLogListResponse struct {
	List     []phoneChangeLogResponse `json:"list"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

func changeLogListResponse(ctx context.Context, value ChangeLogPage) phoneChangeLogListResponse {
	rows := make([]phoneChangeLogResponse, 0, len(value.List))
	for _, item := range value.List {
		rows = append(rows, phoneChangeLogResponse{ID: item.ID, Action: item.Action, ActionLabel: actionLabel(ctx, item.Action), OldPhone: item.OldPhone, NewPhone: item.NewPhone, Platform: item.Platform, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return phoneChangeLogListResponse{List: rows, Total: value.Total, Page: value.Page, PageSize: value.PageSize}
}

func actionLabel(ctx context.Context, action ChangeAction) string {
	for _, item := range adminOptions(ctx).Actions {
		if item.Value == action {
			return item.Label
		}
	}
	return strconv.Itoa(int(action))
}
