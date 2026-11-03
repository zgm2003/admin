package officialmodel

import (
	"context"
	"testing"
)

func TestOfficialModelRejectsUnknownOperationsWithoutRepositoryIO(t *testing.T) {
	if _, err := NewService(nil).Create(context.Background(), CreateRequest{VendorCode: "openai", ModelKey: "fixture", Name: "Fixture", Operations: []string{"unknown"}, SourceURLs: []byte(`[]`)}, 1); err == nil {
		t.Fatal("unknown operation accepted")
	}
}
