package template

import "testing"

func TestFixedCatalogHasStableTencentIDs(t *testing.T) {
	want := map[string]int{SceneLogin: 47941, SceneForget: 47942, SceneBindEmail: 47943, SceneChangePassword: 47944}
	values := FixedCatalog()
	if len(values) != 4 {
		t.Fatalf("templates=%d", len(values))
	}
	for _, value := range values {
		if want[value.Scene] != value.TencentTemplateID || len(value.Variables) != 2 {
			t.Fatalf("invalid template: %+v", value)
		}
	}
}
