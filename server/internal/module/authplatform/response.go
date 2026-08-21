package authplatform

import "time"

type emptyResponse struct{}

type idResponse struct {
	ID int64 `json:"id"`
}

type statusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}

type publicPolicyResponse struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	AllowRegister int16  `json:"allowRegister"`
}

func newPublicPolicyResponse(policy Policy) publicPolicyResponse {
	allowRegister := int16(0)
	if policy.AllowRegister {
		allowRegister = 1
	}
	return publicPolicyResponse{Code: policy.Code, Name: policy.Name, AllowRegister: allowRegister}
}

type listItemResponse struct {
	ID                     int64  `json:"id"`
	Code                   string `json:"code"`
	Name                   string `json:"name"`
	PolicyVersion          int64  `json:"policyVersion"`
	AccessTTLSeconds       int    `json:"accessTTLSeconds"`
	RefreshTTLSeconds      int    `json:"refreshTTLSeconds"`
	SessionCacheTTLSeconds int    `json:"sessionCacheTTLSeconds"`
	AccessCacheTTLSeconds  int    `json:"accessCacheTTLSeconds"`
	BindDevice             int16  `json:"bindDevice"`
	BindIP                 int16  `json:"bindIP"`
	MaxSessions            int16  `json:"maxSessions"`
	AllowRegister          int16  `json:"allowRegister"`
	IsEnabled              int16  `json:"isEnabled"`
	IsBuiltin              int16  `json:"isBuiltin"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

type listResponse struct {
	List     []listItemResponse `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

func newListResponse(items []ListItem, total int64, page, pageSize int) listResponse {
	list := make([]listItemResponse, 0, len(items))
	for _, item := range items {
		value := item.Platform
		list = append(list, listItemResponse{
			ID: value.ID, Code: value.Code, Name: value.Name, PolicyVersion: value.PolicyVersion,
			AccessTTLSeconds: value.AccessTTLSeconds, RefreshTTLSeconds: value.RefreshTTLSeconds,
			SessionCacheTTLSeconds: value.SessionCacheTTLSeconds, AccessCacheTTLSeconds: value.AccessCacheTTLSeconds,
			BindDevice: int16(value.BindDevice), BindIP: int16(value.BindIP), MaxSessions: value.MaxSessions,
			AllowRegister: int16(value.AllowRegister), IsEnabled: int16(value.IsEnabled), IsBuiltin: int16(value.IsBuiltin),
			CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return listResponse{List: list, Total: total, Page: page, PageSize: pageSize}
}

type deploymentResponse struct {
	CookieSecure      bool   `json:"cookieSecure"`
	CORSOrigin        string `json:"corsOrigin"`
	TrustedProxyMode  string `json:"trustedProxyMode"`
	TrustedProxyCount int    `json:"trustedProxyCount"`
	RedisStatus       string `json:"redisStatus"`
}
