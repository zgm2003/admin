package email

import "time"

type sendCodeResponse struct {
	ChallengeID        string    `json:"challengeId"`
	ExpiresAt          time.Time `json:"expiresAt"`
	ResendAfterSeconds int       `json:"resendAfterSeconds"`
}

type emailResponse struct {
	Email string `json:"email"`
}

type changeLogResponse struct {
	ID        int64        `json:"id"`
	Action    ChangeAction `json:"action"`
	OldEmail  *string      `json:"oldEmail"`
	NewEmail  string       `json:"newEmail"`
	Platform  string       `json:"platform"`
	CreatedAt string       `json:"createdAt"`
}

type emailChangeLogListResponse struct {
	List     []changeLogResponse `json:"list"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

func changeLogListResponse(value ChangeLogPage) emailChangeLogListResponse {
	rows := make([]changeLogResponse, 0, len(value.List))
	for _, item := range value.List {
		rows = append(rows, changeLogResponse{ID: item.ID, Action: item.Action, OldEmail: item.OldEmail, NewEmail: item.NewEmail, Platform: item.Platform, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return emailChangeLogListResponse{List: rows, Total: value.Total, Page: value.Page, PageSize: value.PageSize}
}
