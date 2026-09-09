package template

import "testing"

func TestFixedCatalogHasStableScenesAndVariables(t *testing.T) {
	want := map[string]bool{SceneLogin: true, SceneForget: true, SceneBindEmail: true, SceneChangePassword: true}
	values := FixedCatalog()
	if len(values) != 4 {
		t.Fatalf("templates=%d", len(values))
	}
	for _, value := range values {
		if !want[value.Scene] || len(value.Variables) != 2 {
			t.Fatalf("invalid template: %+v", value)
		}
	}
}
