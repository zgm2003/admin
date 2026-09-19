package notification

import "time"

type itemResponse struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	ContentHTML string    `json:"contentHtml"`
	Summary     string    `json:"summary"`
	Variant     Variant   `json:"variant"`
	Priority    Priority  `json:"priority"`
	LinkType    LinkType  `json:"linkType"`
	Link        string    `json:"link"`
	PublishedAt time.Time `json:"publishedAt"`
	IsRead      bool      `json:"isRead"`
}

type listResponse struct {
	Items        []itemResponse `json:"items"`
	NextBeforeID *int64         `json:"nextBeforeId"`
}
type recentResponse struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Variant     Variant   `json:"variant"`
	Priority    Priority  `json:"priority"`
	LinkType    LinkType  `json:"linkType"`
	Link        string    `json:"link"`
	PublishedAt time.Time `json:"publishedAt"`
	IsRead      bool      `json:"isRead"`
}
type summaryResponse struct {
	UnreadCount int64            `json:"unreadCount"`
	Recent      []recentResponse `json:"recent"`
}

func toListResponse(page MailboxPage) listResponse {
	result := listResponse{Items: make([]itemResponse, 0, len(page.Items)), NextBeforeID: page.NextBeforeID}
	for _, v := range page.Items {
		result.Items = append(result.Items, itemResponse{v.ID, v.Title, v.ContentHTML, v.Summary, v.Variant, v.Priority, v.LinkType, v.Link, v.PublishedAt, v.IsRead})
	}
	return result
}
func toSummaryResponse(summary MailboxSummary) summaryResponse {
	result := summaryResponse{UnreadCount: summary.UnreadCount, Recent: make([]recentResponse, 0, len(summary.Recent))}
	for _, v := range summary.Recent {
		result.Recent = append(result.Recent, recentResponse{v.ID, v.Title, v.Summary, v.Variant, v.Priority, v.LinkType, v.Link, v.PublishedAt, v.IsRead})
	}
	return result
}
