package scheduler
import (
 "testing"
 "net/http/httptest"
 "github.com/gin-gonic/gin"
)
func TestJobHistoryIsScopedNewestFirstWithStableCursor(t *testing.T) {
 db, ctx := openSchedulerRepositoryDatabase(t)
 if err := db.WithContext(ctx).Exec(`CREATE TABLE system_scheduler_job(id BIGINT PRIMARY KEY, schedule_id BIGINT, task_type TEXT, status SMALLINT, created_at TIMESTAMPTZ NOT NULL);
 INSERT INTO system_scheduler_job VALUES (1,42,'cleanup',4,'2026-10-12 12:00:00+00'), (2,42,'cleanup',5,'2026-10-12 10:00:00+00'), (3,99,'other',4,'2026-10-12 13:00:00+00'), (4,42,'cleanup',4,'2026-10-12 12:00:00+00');`).Error; err != nil { t.Fatal(err) }
 repo := NewRepository(db); scheduleID := int64(42)
 rows, err := repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Limit: 2})
 if err != nil || len(rows)!=2 || rows[0].ID!=4 || rows[1].ID!=1 { t.Fatalf("first page: %+v, %v",rows,err) }
 rows, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, BeforeID:1, Limit:2})
 if err != nil || len(rows)!=1 || rows[0].ID!=2 { t.Fatalf("older page: %+v, %v",rows,err) }
 failed := JobFailed; rows, err = repo.ListJobs(ctx, JobQuery{ScheduleID: &scheduleID, Status:&failed, Limit:2})
 if err != nil || len(rows)!=1 || rows[0].ID!=2 { t.Fatalf("status filter: %+v, %v",rows,err) }
}
func TestHistoryRejectsInvalidFiltersBeforeDatabaseAccess(t *testing.T) {
 gin.SetMode(gin.TestMode); h := NewHandler(NewService(nil, TaskCatalog{}))
 for _, query := range []string{"scheduleId=0","scheduleId=-1","scheduleId=nope","scheduleId=9223372036854775808","beforeId=-1","beforeId=abc","limit=0","limit=101","status=0","status=nope"} {
  t.Run(query,func(t *testing.T) { r := gin.New(); r.GET("/job",h.ListJobs); w := httptest.NewRecorder(); req := httptest.NewRequest("GET","/job?"+query,nil); r.ServeHTTP(w,req); if w.Code!=400 { t.Fatalf("status=%d body=%s",w.Code,w.Body.String()) } })
 }
}
