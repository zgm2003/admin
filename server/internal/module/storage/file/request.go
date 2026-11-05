package file

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

func parseListQuery(v url.Values) (ListQuery, error) {
	q := ListQuery{Page: 1, PageSize: 20}
	for key, values := range v {
		if len(values) != 1 {
			return q, errors.New("duplicate query")
		}
		switch key {
		case "page", "pageSize", "keyword", "category", "purpose":
		default:
			return q, errors.New("unknown query")
		}
	}
	for key, target := range map[string]*int{"page": &q.Page, "pageSize": &q.PageSize} {
		if raw := v.Get(key); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 {
				return q, errors.New("invalid page")
			}
			*target = n
		}
	}
	q.Keyword = strings.TrimSpace(v.Get("keyword"))
	q.Category = v.Get("category")
	q.Purpose = v.Get("purpose")
	if q.PageSize > 100 || len(q.Keyword) > 255 || q.Purpose != "" && q.Purpose != "avatar" {
		return q, errors.New("invalid file filter")
	}
	switch q.Category {
	case "", "image", "video", "audio", "document", "other":
	default:
		return q, errors.New("invalid category")
	}
	return q, nil
}
