package agent

import (
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/url"
	"strconv"
	"strings"
)

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   *int16
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	q := ListQuery{Page: 1, PageSize: 20}
	values, e := url.ParseQuery(c.Request.URL.RawQuery)
	if e != nil {
		return q, apperror.InvalidRequest(e)
	}
	for k, v := range values {
		if len(v) != 1 {
			return q, apperror.InvalidRequest(errors.New("repeated query"))
		}
		switch k {
		case "page":
			q.Page, e = strconv.Atoi(v[0])
		case "pageSize":
			q.PageSize, e = strconv.Atoi(v[0])
		case "keyword":
			q.Keyword = strings.TrimSpace(v[0])
		case "status":
			var n int64
			n, e = strconv.ParseInt(v[0], 10, 16)
			status := int16(n)
			q.Status = &status
		default:
			return q, apperror.InvalidRequest(errors.New("unknown query"))
		}
		if e != nil {
			return q, apperror.InvalidRequest(e)
		}
	}
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || len(q.Keyword) > 100 || (q.Status != nil && *q.Status != 0 && *q.Status != 1) {
		return q, apperror.InvalidRequest(errors.New("invalid query"))
	}
	return q, nil
}
func (r *Repository) ListFiltered(ctx context.Context, q ListQuery) ([]Model, int64, error) {
	rows := make([]Model, 0)
	var n int64
	db := r.db.WithContext(ctx).Model(&Model{})
	if q.Keyword != "" {
		db = db.Where("name ILIKE ? OR code ILIKE ?", "%"+q.Keyword+"%", "%"+q.Keyword+"%")
	}
	if q.Status != nil {
		db = db.Where("status=?", *q.Status)
	}
	if e := db.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := db.Order("updated_at DESC,id DESC").Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Find(&rows).Error
	return rows, n, e
}
func (s *Service) ListFiltered(ctx context.Context, q ListQuery) ([]Model, int64, error) {
	rows, n, e := s.repository.ListFiltered(ctx, q)
	return rows, n, serviceErr(e)
}
