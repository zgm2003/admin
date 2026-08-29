package operationlog

import "time"

type Item struct {
	ID           int64     `json:"id"`
	RequestID    string    `json:"requestId"`
	UserID       *int64    `json:"userId"`
	SessionID    *int64    `json:"sessionId"`
	Platform     string    `json:"platform"`
	Method       string    `json:"method"`
	Route        string    `json:"route"`
	Module       string    `json:"module"`
	Action       string    `json:"action"`
	ClientIP     string    `json:"clientIp"`
	UserAgent    string    `json:"userAgent"`
	StatusCode   int32     `json:"statusCode"`
	IsSuccess    int16     `json:"isSuccess"`
	LatencyMs    int64     `json:"latencyMs"`
	RequestData  JSON      `json:"requestData"`
	ResponseData JSON      `json:"responseData"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ListResult struct {
	List     []Item `json:"list"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}
