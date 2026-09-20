package scheduler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

const (
	PermissionView    = "system:scheduler:view"
	PermissionList    = "system:scheduler:list"
	PermissionDetail  = "system:scheduler:detail"
	PermissionCreate  = "system:scheduler:create"
	PermissionUpdate  = "system:scheduler:update"
	PermissionStatus  = "system:scheduler:status"
	PermissionDelete  = "system:scheduler:delete"
	PermissionExecute = "system:scheduler:execute"
	PermissionRetry   = "system:scheduler:retry"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) actor(c *gin.Context) (int64, bool) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok || identity.UserID <= 0 {
		return 0, false
	}
	return identity.UserID, true
}
func (h *Handler) ListSchedule(c *gin.Context) {
	limit := queryLimit(c)
	rows, err := h.service.ListSchedules(c.Request.Context(), ScheduleQuery{Limit: limit, AfterID: queryID(c, "afterId")})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, rowsToScheduleResponse(rows))
}
func (h *Handler) GetSchedule(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.GetSchedule(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, scheduleDTO(row))
}
func (h *Handler) Options(c *gin.Context) {
	items := h.service.Options()
	out := make([]taskOptionResponse, 0, len(items))
	for _, item := range items {
		out = append(out, taskOptionDTO(item))
	}
	response.OK(c, http.StatusOK, out)
}
func (h *Handler) CreateSchedule(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req scheduleRequest
	if err := validate.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := req.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.CreateSchedule(c.Request.Context(), input, actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, scheduleDTO(row))
}
func (h *Handler) UpdateSchedule(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req scheduleRequest
	if err = validate.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := req.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.UpdateSchedule(c.Request.Context(), id, input, actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, scheduleDTO(row))
}
func (h *Handler) Status(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var req statusRequest
	if err = validate.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.SetStatus(c.Request.Context(), id, req.IsEnabled, actor); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
func (h *Handler) Execute(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.Execute(c.Request.Context(), id, actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, jobDTO(row))
}
func (h *Handler) ListJobs(c *gin.Context) {
	var status *JobStatus
	if value := c.Query("status"); value != "" {
		parsed := JobStatus(value)
		switch parsed {
		case JobScheduled, JobQueued, JobRunning, JobCompleted, JobFailed, JobCanceled:
			status = &parsed
		default:
			response.Fail(c, apperror.InvalidRequest(errors.New("scheduler job status is invalid")))
			return
		}
	}
	rows, err := h.service.ListJobs(c.Request.Context(), JobQuery{Limit: queryLimit(c), AfterID: queryID(c, "afterId"), Status: status})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, rowsToJobResponse(rows))
}
func (h *Handler) GetJob(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.GetJob(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, jobDTO(row))
}
func (h *Handler) ListRuns(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	rows, err := h.service.ListRuns(c.Request.Context(), RunQuery{JobID: id, Limit: queryLimit(c), AfterID: queryID(c, "afterId")})
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]runResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, runDTO(row))
	}
	response.OK(c, http.StatusOK, out)
}
func (h *Handler) Retry(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	row, err := h.service.Retry(c.Request.Context(), id, actor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, jobDTO(row))
}
func parseID(c *gin.Context) (int64, error) {
	value, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || value <= 0 {
		return 0, apperror.InvalidRequest(errors.New("scheduler id is invalid"))
	}
	return value, nil
}
func queryID(c *gin.Context, key string) int64 {
	value, _ := strconv.ParseInt(c.Query(key), 10, 64)
	if value < 0 {
		return 0
	}
	return value
}
func queryLimit(c *gin.Context) int {
	value, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if value < 1 || value > 100 {
		return 50
	}
	return value
}
func rowsToScheduleResponse(rows []Schedule) []scheduleResponse {
	out := make([]scheduleResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleDTO(row))
	}
	return out
}
func rowsToJobResponse(rows []Job) []jobResponse {
	out := make([]jobResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobDTO(row))
	}
	return out
}

var _ = time.Now
