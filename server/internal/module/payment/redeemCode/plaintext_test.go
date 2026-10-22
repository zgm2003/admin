package redeemcode

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestModelStoresOnlyPlaintextCode(t *testing.T) {
	model := reflect.TypeOf(Model{})
	field, ok := model.FieldByName("Code")
	if !ok || field.Type.Kind() != reflect.String || field.Tag.Get("gorm") != "column:code" {
		t.Fatal("redeem code must be stored directly in the code column")
	}
	for _, obsolete := range []string{"CodeHash", "CodeCiphertext"} {
		if _, ok := model.FieldByName(obsolete); ok {
			t.Fatalf("obsolete encrypted field remains: %s", obsolete)
		}
	}
}

func TestGenerateAndReplayReturnStoredPlaintextWithoutKeys(t *testing.T) {
	code := "ABCDEFGHJKLMNPQRSTUV"
	repo := &fakeRepository{batch: Batch{ID: 9, BatchNo: "RB-existing"}, rows: []Model{{Code: code}}}
	service := NewService(repo, nil)
	input := GenerateInput{RequestID: "550e8400-e29b-41d4-a716-446655440000", AmountCents: 100, Quantity: 1}
	result, err := service.Generate(context.Background(), 1, input)
	if err != nil || result.BatchID != 9 || len(result.Codes) != 1 || result.Codes[0] != code {
		t.Fatalf("replay lost code: %v", err)
	}
	result, err = NewService(&fakeRepository{}, nil).Generate(context.Background(), 1, input)
	if err != nil || len(result.Codes) != 1 {
		t.Fatalf("generate failed: %v", err)
	}
	if _, err = storedCode(result.Codes[0]); err != nil {
		t.Fatal("generated noncanonical code")
	}
}

func TestAdminListAndDetailReturnFullCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	code := "ABCDEFGHJKLMNPQRSTUV"
	repo := &fakeRepository{rows: []Model{{ID: 1, BatchID: 2, Code: code, Status: StatusUnused, AmountCents: 100}}}
	handler := NewHandler(NewService(repo, nil))
	router := gin.New()
	router.GET("/", handler.List)
	router.GET("/:id", handler.Get)
	for _, path := range []string{"/?page=1&pageSize=20", "/1"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("unexpected HTTP response")
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		var value map[string]json.RawMessage
		if path[1] == '?' {
			var page struct {
				List []map[string]json.RawMessage `json:"list"`
			}
			if err := json.Unmarshal(envelope.Data, &page); err != nil || len(page.List) != 1 {
				t.Fatal("missing list")
			}
			value = page.List[0]
		} else if err := json.Unmarshal(envelope.Data, &value); err != nil {
			t.Fatal(err)
		}
		var actual string
		if err := json.Unmarshal(value["code"], &actual); err != nil || actual != code {
			t.Fatal("full code missing")
		}
		for _, obsolete := range []string{"codeMask", "codeHash", "codeCiphertext"} {
			if _, ok := value[obsolete]; ok {
				t.Fatalf("obsolete property %s", obsolete)
			}
		}
	}
}
