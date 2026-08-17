package pagination_test

import (
	"encoding/json"
	"testing"

	"admin/server/internal/shared/pagination"
	"github.com/go-playground/validator/v10"
)

func TestRequestRequiresExplicitValidValues(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")

	for _, request := range []pagination.Request{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
	} {
		if err := validate.Struct(request); err == nil {
			t.Fatalf("expected %+v to be invalid", request)
		}
	}

	if err := validate.Struct(pagination.Request{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestResultUsesTheOnlyPaginationShape(t *testing.T) {
	encoded, err := json.Marshal(pagination.Result[string]{
		List:     []string{"one"},
		Total:    1,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	want := `{"list":["one"],"total":1,"page":1,"pageSize":20}`
	if string(encoded) != want {
		t.Fatalf("result JSON = %s, want %s", encoded, want)
	}
}
