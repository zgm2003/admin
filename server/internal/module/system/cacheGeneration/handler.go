package cachegeneration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type service interface {
	List(context.Context, ListQuery) (ListResult, error)
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	query, err := parseListQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, listResultResponse(result))
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "publishState") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("invalid query parameter")
		}
	}
	page, pageSize := 1, 20
	var err error
	if raw := values.Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil {
			return ListQuery{}, fmt.Errorf("page is invalid")
		}
	}
	if raw := values.Get("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil {
			return ListQuery{}, fmt.Errorf("pageSize is invalid")
		}
	}
	if page < 1 {
		return ListQuery{}, fmt.Errorf("page is invalid")
	}
	if pageSize < 1 || pageSize > 100 {
		return ListQuery{}, fmt.Errorf("pageSize is invalid")
	}
	keyword := strings.TrimSpace(values.Get("keyword"))
	if utf8.RuneCountInString(keyword) > maxKeywordRunes {
		return ListQuery{}, fmt.Errorf("keyword is invalid")
	}
	publishState := strings.TrimSpace(values.Get("publishState"))
	if publishState != "" && !IsPublishState(publishState) {
		return ListQuery{}, fmt.Errorf("publishState is invalid")
	}
	return ListQuery{
		Page:         page,
		PageSize:     pageSize,
		Keyword:      keyword,
		PublishState: publishState,
	}, nil
}
