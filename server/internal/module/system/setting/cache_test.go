package setting

import "testing"

func TestDecodeRecordRejectsUnknownAndTrailingFields(t *testing.T) {
	for name, payload := range map[string]string{
		"unknown field":  `{"key":"auth.captcha.ttl_minutes","value":"2","valueType":2,"description":"","isEnabled":1,"isBuiltin":1,"extra":true}`,
		"trailing value": `{"key":"auth.captcha.ttl_minutes","value":"2","valueType":2,"description":"","isEnabled":1,"isBuiltin":1} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRecord(payload); err == nil {
				t.Fatal("decodeRecord accepted invalid cache payload")
			}
		})
	}
}

func TestDecodeRecordUsesStableLowerCamelFields(t *testing.T) {
	record, err := decodeRecord(`{"id":42,"key":"auth.captcha.ttl_minutes","value":"2","valueType":2,"description":"","isEnabled":1,"isBuiltin":1,"createdAt":"2026-09-12T00:00:00Z","updatedAt":"2026-09-12T01:00:00Z"}`)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != 42 || record.Key != "auth.captcha.ttl_minutes" || record.ValueType != ValueTypeNumber || record.IsEnabled != 1 || record.IsBuiltin != 1 {
		t.Fatalf("decoded record = %+v", record)
	}
}
