package scheduler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestJobHistoryIsScopedNewestFirstWithPagination(t *testing.T) {
	db, ctx := openSchedulerRepositoryDatabase(t)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE system_scheduler_job(id BIGINT PRIMARY KEY, schedule_id BIGINT, task_type TEXT, status SMALLINT, created_at TIMESTAMPTZ NOT NULL);
 INSERT INTO system_scheduler_job VALUES (1,42,'cleanup',4,'2026-10-12 12:00:00+00'), (2,42,'cleanup',5,'2026-10-12 10:00:00+00'), (3,99,'other',4,'2026-10-12 13:00:00+00'), (4,42,'cleanup',4,'2026-10-12 12:00:00+00');`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	scheduleID := int64(42)
	rows, total, err := repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Page: 1, PageSize: 2})
	if err != nil || total != 3 || len(rows) != 2 || rows[0].ID != 4 || rows[1].ID != 1 {
		t.Fatalf("first page: %+v, %v", rows, err)
	}
	rows, total, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Page: 2, PageSize: 2})
	if err != nil || total != 3 || len(rows) != 1 || rows[0].ID != 2 {
		t.Fatalf("older page: %+v, %v", rows, err)
	}
	r := gin.New()
	r.GET("/job", NewHandler(NewService(repo, TaskCatalog{})).ListJobs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/job?scheduleId=42&status=5&pageSize=20&page=1", nil))
	var result struct {
		Data struct {
			List []struct {
				ID int64 `json:"id"`
			} `json:"list"`
			Total    int64 `json:"total"`
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.Data.Total != 1 || result.Data.Page != 1 || result.Data.PageSize != 20 || len(result.Data.List) != 1 || result.Data.List[0].ID != 2 {
		t.Fatalf("handler response: %d %s %v", w.Code, w.Body.String(), err)
	}
	rows, total, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Page: 3, PageSize: 2})
	if err != nil || total != 3 || len(rows) != 0 {
		t.Fatalf("out-of-range page: %+v, %d, %v", rows, total, err)
	}
	missingID := int64(1234)
	rows, total, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &missingID, Page: 1, PageSize: 2})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("empty plan: %+v, %d, %v", rows, total, err)
	}
	failed := JobFailed
	rows, total, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Status: &failed, Page: 1, PageSize: 2})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != 2 {
		t.Fatalf("status filter: %+v, %v", rows, err)
	}
}
func TestHistoryRejectsInvalidFiltersBeforeDatabaseAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil, TaskCatalog{}))
	for _, query := range []string{"scheduleId=", "page=", "pageSize=", "scheduleId=0", "scheduleId=-1", "scheduleId=nope", "scheduleId=9223372036854775808", "page=-1", "page=abc", "pageSize=0", "pageSize=101", "page=9223372036854775807", "page=184467440737095516160", "page=1&page=2", "status=0", "status=nope"} {
		t.Run(query, func(t *testing.T) {
			r := gin.New()
			r.GET("/job", h.ListJobs)
			w := httptest.NewRecorder()
			values, err := url.ParseQuery(query)
			if err != nil {
				t.Fatal(err)
			}
			if !values.Has("page") {
				values.Set("page", "1")
			}
			if !values.Has("pageSize") {
				values.Set("pageSize", "20")
			}
			req := httptest.NewRequest("GET", "/job?"+values.Encode(), nil)
			r.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPlansAreNewestFirstWithStableCursor(t *testing.T) {
	db, ctx := openSchedulerRepositoryDatabase(t)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE system_scheduler_schedule(id BIGINT PRIMARY KEY, task_type TEXT, status SMALLINT, params JSONB, created_at TIMESTAMPTZ NOT NULL, deleted_at TIMESTAMPTZ);
 INSERT INTO system_scheduler_schedule VALUES (1,'cleanup',1,'{}','2026-10-12 12:00:00+00',NULL),(2,'cleanup',1,'{}','2026-10-12 10:00:00+00',NULL),(3,'cleanup',1,'{}','2026-10-12 13:00:00+00','2026-10-12 14:00:00+00'),(4,'cleanup',1,'{}','2026-10-12 12:00:00+00',NULL);`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	rows, err := repo.ListSchedules(ctx, ScheduleQuery{Limit: 2})
	if err != nil || len(rows) != 2 || rows[0].ID != 4 || rows[1].ID != 1 {
		t.Fatalf("plans: %+v %v", rows, err)
	}
	rows, err = repo.ListSchedules(ctx, ScheduleQuery{Limit: 2, BeforeID: 1})
	if err != nil || len(rows) != 1 || rows[0].ID != 2 {
		t.Fatalf("older plans: %+v %v", rows, err)
	}
}
