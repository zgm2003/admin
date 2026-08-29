package session

import "time"

type sessionAdminItemResponse struct {
	ID               int64         `json:"id"`
	UserID           int64         `json:"userId"`
	Username         string        `json:"username"`
	Platform         string        `json:"platform"`
	DeviceID         string        `json:"deviceId"`
	ClientIP         string        `json:"clientIp"`
	UserAgent        string        `json:"userAgent"`
	CreatedAt        string        `json:"createdAt"`
	UpdatedAt        string        `json:"updatedAt"`
	RefreshExpiresAt string        `json:"refreshExpiresAt"`
	RevokedAt        *string       `json:"revokedAt"`
	Status           SessionStatus `json:"status"`
	IsCurrent        bool          `json:"isCurrent"`
}

type sessionAdminListResponse struct {
	List     []sessionAdminItemResponse `json:"list"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"pageSize"`
}

type sessionAdminStatsResponse struct {
	ActiveTotal int64            `json:"activeTotal"`
	Platforms   map[string]int64 `json:"platforms"`
}

type sessionAdminRevokeResponse struct {
	Revoked        int `json:"revoked"`
	SkippedCurrent int `json:"skippedCurrent"`
	SkippedRevoked int `json:"skippedRevoked"`
}

func newSessionAdminListResponse(rows []AdminSession, total int64, query AdminSessionQuery, currentSessionID int64) sessionAdminListResponse {
	items := make([]sessionAdminItemResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, sessionAdminItemResponse{
			ID: row.ID, UserID: row.UserID, Username: row.Username, Platform: row.Platform, DeviceID: row.DeviceID,
			ClientIP: row.ClientIP, UserAgent: row.UserAgent, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano), RefreshExpiresAt: row.RefreshExpiresAt.UTC().Format(time.RFC3339Nano),
			RevokedAt: formatOptionalTime(row.RevokedAt), Status: row.Status, IsCurrent: row.ID == currentSessionID,
		})
	}
	return sessionAdminListResponse{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}
