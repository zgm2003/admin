package option

import (
	"admin/server/internal/shared/i18n"
	"context"
	"encoding/json"
	"testing"
)

func TestLocalizedOptionPreservesNumericAndStringValues(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.EnUS)
	data, err := json.Marshal(New(ctx, 0, "未知", "Unknown"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"value":0,"label":"Unknown"}` {
		t.Fatalf("numeric option=%s", data)
	}
	zh := i18n.WithLocale(context.Background(), i18n.ZhCN)
	value := New(zh, "openai", "OpenAI", "OpenAI")
	if value.Value != "openai" || value.Label != "OpenAI" {
		t.Fatalf("string option=%+v", value)
	}
	if New(zh, 1, "男", "Male").Label != "男" {
		t.Fatal("Chinese label lost")
	}
}
