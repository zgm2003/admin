package run

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"context"
	"github.com/gin-gonic/gin"
	"strconv"
)

type MediaPreview struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}
type MediaAccess interface {
	Preview(context.Context, int64, int64, int64, int64, bool) (MediaPreview, error)
	Retry(context.Context, int64, int64, int64) error
}

func (h *Handler) SetQueries(q *QueryService)   { h.queries = q }
func (h *Handler) SetMediaAccess(m MediaAccess) { h.media = m }
func (h *Handler) queryActor(c *gin.Context) (QueryOwner, bool) {
	u, p, ok := runActor(c)
	if !ok || u < 1 || p < 1 {
		response.Fail(c, apperror.Unauthorized(nil))
		return QueryOwner{}, false
	}
	if h.queries == nil {
		response.Fail(c, apperror.DependencyUnavailable(nil))
		return QueryOwner{}, false
	}
	return QueryOwner{u, p}, true
}
func queryID(c *gin.Context, key string) (int64, bool) {
	v, e := validate.ParsePositiveInt64(c.Param(key), key)
	if e != nil {
		response.Fail(c, e)
		return 0, false
	}
	return v, true
}
func queryPage(c *gin.Context) (int64, int, bool) {
	var v struct {
		BeforeID int64 `form:"beforeId"`
		Limit    int   `form:"limit"`
	}
	if e := c.ShouldBindQuery(&v); e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return 0, 0, false
	}
	if v.Limit == 0 {
		v.Limit = 20
	}
	if v.BeforeID < 0 || v.Limit < 1 || v.Limit > 100 {
		response.Fail(c, apperror.InvalidRequest(nil))
		return 0, 0, false
	}
	return v.BeforeID, v.Limit, true
}
func (h *Handler) list(c *gin.Context, admin bool) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	var q RunQuery
	if e := c.ShouldBindQuery(&q); e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	rows, more, e := h.queries.List(c.Request.Context(), q, o, admin)
	if e != nil {
		response.Fail(c, e)
		return
	}
	next := ""
	if len(rows) > 0 {
		next = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	if admin {
		out := make([]AdminRunResponse, 0, len(rows))
		for _, m := range rows {
			out = append(out, AdminResponse(m))
		}
		response.OK(c, 200, CursorPage[AdminRunResponse]{List: out, HasMore: more, NextBeforeID: next})
		return
	}
	out := make([]Response, 0, len(rows))
	for _, m := range rows {
		out = append(out, PublicResponse(m))
	}
	response.OK(c, 200, CursorPage[Response]{List: out, HasMore: more, NextBeforeID: next})
}
func (h *Handler) AdminList(c *gin.Context) { h.list(c, true) }
func (h *Handler) OwnList(c *gin.Context)   { h.list(c, false) }
func (h *Handler) AdminDetail(c *gin.Context) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	id, ok := queryID(c, "id")
	if !ok {
		return
	}
	m, e := h.queries.Detail(c.Request.Context(), id, o, true)
	if e != nil {
		response.Fail(c, e)
		return
	}
	input, result, e := h.queries.Content(c.Request.Context(), m)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, AdminDetailResponse{AdminRunResponse: AdminResponse(m), Input: input, Result: result})
}
func (h *Handler) Summary(c *gin.Context) {
	if _, ok := h.queryActor(c); !ok {
		return
	}
	var q RunQuery
	if e := c.ShouldBindQuery(&q); e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	v, e := h.queries.Summary(c.Request.Context(), q)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) Events(c *gin.Context) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	id, ok := queryID(c, "id")
	if !ok {
		return
	}
	var q struct {
		After int64 `form:"afterSequence"`
		Limit int   `form:"limit"`
	}
	if e := c.ShouldBindQuery(&q); e != nil {
		response.Fail(c, apperror.InvalidRequest(e))
		return
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	v, e := h.queries.Events(c.Request.Context(), id, q.After, q.Limit, o)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) children(c *gin.Context, kind string, admin bool) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	id, ok := queryID(c, "id")
	if !ok {
		return
	}
	before, limit, ok := queryPage(c)
	if !ok {
		return
	}
	v, e := h.queries.Children(c.Request.Context(), id, before, limit, kind, o, admin)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) AdminRequests(c *gin.Context)    { h.children(c, "request", true) }
func (h *Handler) OwnRequests(c *gin.Context)      { h.children(c, "request", false) }
func (h *Handler) AdminTools(c *gin.Context)       { h.children(c, "tool-call", true) }
func (h *Handler) AdminCharges(c *gin.Context)     { h.children(c, "usage-charge", true) }
func (h *Handler) OwnCharges(c *gin.Context)       { h.children(c, "usage-charge", false) }
func (h *Handler) AdminChargeItems(c *gin.Context) { h.children(c, "usage-charge-item", true) }
func (h *Handler) OwnChargeItems(c *gin.Context)   { h.children(c, "usage-charge-item", false) }
func (h *Handler) AdminMedia(c *gin.Context)       { h.children(c, "media", true) }
func (h *Handler) OwnMedia(c *gin.Context)         { h.children(c, "media", false) }
func (h *Handler) preview(c *gin.Context, admin bool) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	id, ok := queryID(c, "id")
	if !ok {
		return
	}
	mid, ok := queryID(c, "mediaId")
	if !ok {
		return
	}
	if h.media == nil {
		response.Fail(c, apperror.DependencyUnavailable(nil))
		return
	}
	v, e := h.media.Preview(c.Request.Context(), id, mid, o.UserID, o.PlatformID, admin)
	if e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, v)
}
func (h *Handler) AdminPreview(c *gin.Context) { h.preview(c, true) }
func (h *Handler) OwnPreview(c *gin.Context)   { h.preview(c, false) }
func (h *Handler) RetryMedia(c *gin.Context) {
	o, ok := h.queryActor(c)
	if !ok {
		return
	}
	id, ok := queryID(c, "id")
	if !ok {
		return
	}
	if h.media == nil {
		response.Fail(c, apperror.DependencyUnavailable(nil))
		return
	}
	if e := h.media.Retry(c.Request.Context(), id, o.UserID, o.PlatformID); e != nil {
		response.Fail(c, e)
		return
	}
	response.OK(c, 200, struct{}{})
}
func (h *Handler) Options(c *gin.Context) {
	response.OK(c, 200, StaticOptions(c.GetHeader("Accept-Language")))
}

type EnumOption struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

func StaticOptions(language string) map[string]any {
	zh := len(language) >= 2 && language[:2] == "zh"
	label := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	return map[string]any{"operations": []EnumOption{{"text.generate", label("Text", "文字")}, {"image.generate", label("Image generation", "图片生成")}, {"image.edit", label("Image editing", "图片编辑")}, {"audio.speech", label("Speech", "语音生成")}, {"audio.transcribe", label("Transcription", "音频转写")}, {"video.generate", label("Video", "视频")}}, "executionStatuses": []EnumOption{{0, label("Queued", "排队")}, {1, label("Running", "执行中")}, {2, label("Waiting upstream", "等待上游")}, {3, label("Succeeded", "成功")}, {4, label("Failed", "失败")}, {5, label("Canceled", "已取消")}}, "usageStatuses": []EnumOption{{0, label("Unknown", "未知")}, {1, label("Complete", "完整")}, {2, label("Incomplete", "不完整")}}, "billingStatuses": []EnumOption{{0, label("Pending usage", "等待用量")}, {1, label("Pending settlement", "等待结算")}, {2, label("Settled", "已结算")}, {3, label("Retry settlement", "扣款待重试")}, {4, label("Needs review", "待核对")}}, "transferStatuses": []EnumOption{{0, label("No media", "无媒体")}, {1, label("Pending transfer", "待保存")}, {2, label("Transferring", "保存中")}, {3, label("Ready", "全部就绪")}, {4, label("Partially failed", "部分失败")}, {5, label("Failed", "全部失败")}}, "mediaTransferStatuses": []EnumOption{{0, label("Queued", "等待保存")}, {1, label("Transferring", "保存中")}, {2, label("Ready", "已保存")}, {3, label("Retry", "待重试")}, {4, label("Failed", "保存失败")}}, "constraints": map[string]int{"defaultLimit": 20, "maxLimit": 100, "maxHistoryDays": 31, "maxSummaryHours": 24, "maxFiles": 4}}
}
