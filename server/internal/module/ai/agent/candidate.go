package agent

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

type Candidate struct {
	ID         int64           `json:"id,string"`
	Name       string          `json:"name"`
	Valid      bool            `json:"valid"`
	Operations StringArray     `json:"operations" gorm:"type:text[]"`
	Limits     json.RawMessage `json:"limits" gorm:"type:jsonb"`
}

func (r *Repository) Candidates(ctx context.Context, kind, keyword string, page, size int) ([]Candidate, int64, error) {
	var rows []Candidate
	var count int64
	table := "ai_provider_model"
	columns := "id,name,operations,limits,(status=1) AS valid"
	if kind == "tool" {
		table = "ai_tool"
		columns = "id,name,ARRAY[]::text[] AS operations,'{}'::jsonb AS limits,(status=1) AS valid"
	}
	q := r.db.WithContext(ctx).Table(table).Where("deleted_at IS NULL")
	if keyword != "" {
		q = q.Where("name ILIKE ?", "%"+keyword+"%")
	}
	if e := q.Count(&count).Error; e != nil {
		return nil, 0, e
	}
	e := q.Select(columns).Order("id DESC").Limit(size).Offset((page - 1) * size).Scan(&rows).Error
	return rows, count, e
}
func (s *Service) Candidates(ctx context.Context, agentID int64, kind, keyword string, page, size int) ([]Candidate, int64, error) {
	if kind != "providerModel" && kind != "tool" || len(keyword) > 100 || page < 1 || size < 1 || size > 100 {
		return nil, 0, apperror.InvalidRequest(errors.New("candidate query invalid"))
	}
	if agentID > 0 {
		if _, e := s.Find(ctx, agentID); e != nil {
			return nil, 0, e
		}
	}
	rows, total, e := s.repository.Candidates(ctx, kind, keyword, page, size)
	return rows, total, serviceErr(e)
}
func (h *Handler) Candidates(c *gin.Context) {
	var id int64
	var e error
	if c.Param("id") != "" {
		id, e = parseID(c)
		if e != nil {
			response.Fail(c, e)
			return
		}
	}
	for k, vs := range c.Request.URL.Query() {
		if len(vs) != 1 || (k != "kind" && k != "keyword" && k != "page" && k != "pageSize") {
			response.Fail(c, apperror.InvalidRequest(errors.New("candidate query invalid")))
			return
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, n, e := h.service.Candidates(c.Request.Context(), id, c.Query("kind"), strings.TrimSpace(c.Query("keyword")), page, size)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": n, "page": page, "pageSize": size})
}
