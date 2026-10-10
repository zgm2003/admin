package cachegeneration

import (
	"context"
	"time"
)

const (
	PermissionView = "system:cacheGeneration:view"
	PermissionList = "system:cacheGeneration:list"
)

// publishState 是 PostgreSQL 侧的发布状态；Redis state 由 Service 合成。
const (
	PublishStateReady    = "ready"
	PublishStatePending  = "pending"
	PublishStateRetrying = "retrying"
)

const (
	StatusReady        = "ready"
	StatusPending      = "pending"
	StatusRetrying     = "retrying"
	StatusInvalidating = "invalidating"
	StatusMissing      = "missing"
	StatusCorrupt      = "corrupt"
	StatusUnavailable  = "unavailable"
)

const (
	maxKeywordRunes   = 128
	maxLastErrorRunes = 512
)

type ListQuery struct {
	Page         int
	PageSize     int
	Keyword      string
	PublishState string
}

type Item struct {
	Namespace                 string
	ScopeKey                  string
	Generation                int64
	Status                    string
	PendingCount              int64
	OldestPendingAt           *time.Time
	LatestAttempts            int
	LastError                 string
	LatestPublishedGeneration *int64
	LatestPublishedAt         *time.Time
	UpdatedAt                 time.Time
}

type ListResult struct {
	Items    []Item
	Total    int64
	Page     int
	PageSize int
}

type listItem struct {
	NamespaceLabel            string     `json:"namespaceLabel"`
	ScopeLabel                string     `json:"scopeLabel"`
	StatusLabel               string     `json:"statusLabel"`
	StatusTone                string     `json:"statusTone"`
	StatusHint                string     `json:"statusHint"`
	PublishedVersion          *int64     `json:"publishedVersion"`
	Namespace                 string     `json:"namespace"`
	ScopeKey                  string     `json:"scopeKey"`
	Generation                int64      `json:"generation"`
	Status                    string     `json:"status"`
	PendingCount              int64      `json:"pendingCount"`
	OldestPendingAt           *time.Time `json:"oldestPendingAt"`
	LatestAttempts            int        `json:"latestAttempts"`
	LastError                 string     `json:"lastError"`
	LatestPublishedGeneration *int64     `json:"latestPublishedGeneration"`
	LatestPublishedAt         *time.Time `json:"latestPublishedAt"`
	UpdatedAt                 time.Time  `json:"updatedAt"`
}

type listResponse struct {
	List     []listItem `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

func IsPublishState(value string) bool {
	switch value {
	case PublishStateReady, PublishStatePending, PublishStateRetrying:
		return true
	default:
		return false
	}
}

func listItemResponse(ctx context.Context, item Item) listItem {
	label, tone, hint := statusDisplay(ctx, item.Status)
	var published *int64
	if item.Status == StatusReady {
		published = item.LatestPublishedGeneration
	}
	return listItem{
		NamespaceLabel: namespaceLabel(ctx, item.Namespace), ScopeLabel: scopeLabel(ctx, item.Namespace, item.ScopeKey),
		StatusLabel: label, StatusTone: tone, StatusHint: hint, PublishedVersion: published,
		Namespace:                 item.Namespace,
		ScopeKey:                  item.ScopeKey,
		Generation:                item.Generation,
		Status:                    item.Status,
		PendingCount:              item.PendingCount,
		OldestPendingAt:           item.OldestPendingAt,
		LatestAttempts:            item.LatestAttempts,
		LastError:                 item.LastError,
		LatestPublishedGeneration: item.LatestPublishedGeneration,
		LatestPublishedAt:         item.LatestPublishedAt,
		UpdatedAt:                 item.UpdatedAt,
	}
}

func listResultResponse(ctx context.Context, result ListResult) listResponse {
	items := make([]listItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, listItemResponse(ctx, item))
	}
	return listResponse{List: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}
