package taskdemo

import (
	"reflect"
	"strings"
	"testing"
)

func TestTaskDeclaresExplicitPostgresTimeTypes(t *testing.T) {
	taskType := reflect.TypeOf(Task{})
	for _, fieldName := range []string{"CreatedAt", "UpdatedAt"} {
		field, ok := taskType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("field %s is missing", fieldName)
		}
		gormTag := field.Tag.Get("gorm")
		if !strings.Contains(gormTag, "type:timestamptz") {
			t.Errorf("%s gorm tag = %q, want type:timestamptz", fieldName, gormTag)
		}
		if !strings.Contains(gormTag, "not null") {
			t.Errorf("%s gorm tag = %q, want not null", fieldName, gormTag)
		}
		if !strings.Contains(gormTag, "default:CURRENT_TIMESTAMP") {
			t.Errorf("%s gorm tag = %q, want default:CURRENT_TIMESTAMP", fieldName, gormTag)
		}
	}
}
