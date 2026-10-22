package redeemcode

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminProjectionPreservesBatchRemark(t *testing.T) {
	remark := "  活动赠送\n客户补偿  "
	row := Model{ID: 1, BatchID: 2, Code: "ABCDEFGHJKLMNPQRSTUV", Status: StatusUnused, AmountCents: 100}
	field := reflect.ValueOf(&row).Elem().FieldByName("Remark")
	if !field.IsValid() || field.Type() != reflect.TypeOf((*string)(nil)) {
		t.Fatal("joined batch remark must be a nullable read-only field")
	}
	field.Set(reflect.ValueOf(&remark))
	service := NewService(&fakeRepository{rows: []Model{row}}, nil)
	value, err := service.Get(context.Background(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		Remark *string `json:"remark"`
	}
	if err := json.Unmarshal(data, &actual); err != nil || actual.Remark == nil || *actual.Remark != remark {
		t.Fatal("remark must preserve the stored text", err)
	}
}

func TestAdminReadsAlwaysIncludeNullableBatchRemark(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepository{rows: []Model{{ID: 1, BatchID: 2, Code: "ABCDEFGHJKLMNPQRSTUV", Status: StatusUnused, AmountCents: 100}}}
	handler := NewHandler(NewService(repo, nil))
	router := gin.New()
	router.GET("/", handler.List)
	router.GET("/:id", handler.Get)
	router.POST("/lookup", handler.Lookup)
	router.POST("/export", handler.Export)
	remark := "活动赠送\n客户补偿"
	for _, expected := range []*string{&remark, nil} {
		repo.rows[0].Remark = expected
		for _, request := range []struct{ method, path, body string }{
			{http.MethodGet, "/?page=1&pageSize=20", ""},
			{http.MethodGet, "/1", ""},
			{http.MethodPost, "/lookup", `{"code":"ABCDEFGHJKLMNPQRSTUV"}`},
			{http.MethodPost, "/export", `{"ids":[1]}`},
		} {
			t.Run(request.path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				req := httptest.NewRequest(request.method, request.path, bytes.NewBufferString(request.body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, req)
				if recorder.Code != http.StatusOK {
					t.Fatalf("HTTP status: %d", recorder.Code)
				}
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				data := envelope.Data
				if request.path == "/?page=1&pageSize=20" {
					var page struct {
						List []json.RawMessage `json:"list"`
					}
					if err := json.Unmarshal(data, &page); err != nil || len(page.List) != 1 {
						t.Fatal("missing list row", err)
					}
					data = page.List[0]
				} else if request.path == "/export" {
					var rows []json.RawMessage
					if err := json.Unmarshal(data, &rows); err != nil || len(rows) != 1 {
						t.Fatal("missing export row", err)
					}
					data = rows[0]
				}
				var row map[string]json.RawMessage
				if err := json.Unmarshal(data, &row); err != nil {
					t.Fatal(err)
				}
				want, err := json.Marshal(expected)
				if err != nil || string(row["remark"]) != string(want) {
					t.Fatalf("remark must preserve text or explicit null: got %s want %s error %v", row["remark"], want, err)
				}
			})
		}
	}
}

func TestPersonalRedemptionDoesNotIncludeBatchRemark(t *testing.T) {
	data, err := json.Marshal(Redemption{})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if _, found := value["remark"]; found {
		t.Fatal("batch remark leaked into personal redemption")
	}
}
