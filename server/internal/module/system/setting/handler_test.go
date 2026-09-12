package setting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type detailHandlerService struct {
	record Record
}

func (f detailHandlerService) List(context.Context, ListQuery) (ListResult, error) {
	return ListResult{}, nil
}
func (f detailHandlerService) FindByKey(context.Context, string) (sharedsetting.Record, error) {
	return sharedsetting.Record{Key: f.record.Key, Value: f.record.Value, ValueType: f.record.ValueType, Description: f.record.Description, IsEnabled: f.record.IsEnabled, IsBuiltin: f.record.IsBuiltin}, nil
}
func (f detailHandlerService) Find(context.Context, string) (Record, error) { return f.record, nil }
func (f detailHandlerService) Create(context.Context, CreateInput) (int64, error) {
	return 0, nil
}
func (f detailHandlerService) Update(context.Context, string, UpdateInput) error       { return nil }
func (f detailHandlerService) UpdateStatus(context.Context, string, yesno.Value) error { return nil }
func (f detailHandlerService) Delete(context.Context, string) error                    { return nil }

func TestDetailReturnsPersistenceMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	createdAt := time.Date(2026, time.September, 12, 1, 2, 3, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	handler := NewHandler(detailHandlerService{record: Record{
		ID: 42, Key: "auth.captcha.ttl_minutes", Value: "2", ValueType: ValueTypeNumber,
		Description: "Captcha lifetime", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}})
	router := gin.New()
	router.GET("/system/setting/:key", handler.Detail)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/system/setting/auth.captcha.ttl_minutes", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data detailResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	got := envelope.Data.Setting
	if got.ID != 42 || got.Key != "auth.captcha.ttl_minutes" || got.CreatedAt != createdAt || got.UpdatedAt != updatedAt {
		t.Fatalf("detail = %+v, want persistence metadata", got)
	}
}
