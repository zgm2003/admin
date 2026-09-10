package dictionary

import (
	"reflect"
	"strings"
	"testing"
)

func TestMaintainedModelsExposeExplicitTimestampsAndSoftDelete(t *testing.T) {
	for _, model := range []any{Dictionary{}, Item{}} {
		typeOf := reflect.TypeOf(model)
		for _, name := range []string{"CreatedAt", "UpdatedAt", "DeletedAt"} {
			field, ok := typeOf.FieldByName(name)
			if !ok {
				t.Fatalf("%s.%s is missing", typeOf.Name(), name)
			}
			if name != "DeletedAt" && !strings.Contains(field.Tag.Get("gorm"), "type:timestamptz") {
				t.Fatalf("%s.%s tag=%q", typeOf.Name(), name, field.Tag.Get("gorm"))
			}
		}
	}
}

func TestDictionaryTablesUseSystemDomain(t *testing.T) {
	if (Dictionary{}).TableName() != "system_dictionary" || (Item{}).TableName() != "system_dictionary_item" {
		t.Fatal("dictionary table names are invalid")
	}
}
