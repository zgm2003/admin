package template

import (
	"reflect"
	"testing"
)

// SMS owns its scene values; they are frozen as literals so a renamed or
// mistyped constant cannot silently keep the catalog test green.
func TestSceneValuesAreFrozenLiterals(t *testing.T) {
	got := []string{SceneLogin, SceneForget, SceneBindPhone, SceneChangePassword}
	want := []string{"login", "forget", "bind_phone", "change_password"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scene constants = %v, want %v", got, want)
	}
}

func TestFixedCatalogHasStableScenesAndParameterKeys(t *testing.T) {
	want := []struct {
		scene string
		name  string
	}{
		{scene: "login", name: "登录验证码"},
		{scene: "forget", name: "找回密码"},
		{scene: "bind_phone", name: "绑定/换绑手机"},
		{scene: "change_password", name: "修改密码"},
	}

	values := FixedCatalog()
	if len(values) != len(want) {
		t.Fatalf("catalog has %d entries, want %d", len(values), len(want))
	}
	for index, expected := range want {
		value := values[index]
		if value.Scene != expected.scene || value.Name != expected.name {
			t.Fatalf("catalog[%d] = %+v, want scene %q name %q", index, value, expected.scene, expected.name)
		}
		if !reflect.DeepEqual(value.ParameterKeys, []string{"code", "ttl_minutes"}) {
			t.Fatalf("catalog[%d] parameter keys = %v", index, value.ParameterKeys)
		}
	}
}

func TestFixedCatalogCopiesParameterKeys(t *testing.T) {
	mutated := FixedCatalog()
	mutated[0].ParameterKeys[0] = "mutated"
	mutated[1].ParameterKeys = append(mutated[1].ParameterKeys, "extra")

	fresh := FixedCatalog()
	if !reflect.DeepEqual(fresh[0].ParameterKeys, []string{"code", "ttl_minutes"}) {
		t.Fatalf("catalog parameter keys are shared: %v", fresh[0].ParameterKeys)
	}
	if len(fresh[1].ParameterKeys) != 2 {
		t.Fatalf("catalog parameter keys are shared: %v", fresh[1].ParameterKeys)
	}
}

func TestValidateSceneAcceptsExactlyFourSnakeCaseValues(t *testing.T) {
	for _, scene := range []string{"login", "forget", "bind_phone", "change_password"} {
		if err := ValidateScene(scene); err != nil {
			t.Fatalf("ValidateScene(%q) = %v", scene, err)
		}
	}

	for _, scene := range []string{
		"",
		"test",
		"forgetPassword",
		"setPassword",
		"bindPhone",
		"changePhone",
		"bind_email",
		"login ",
		"Login",
		"bind-phone",
	} {
		if err := ValidateScene(scene); err == nil {
			t.Fatalf("ValidateScene(%q) accepted an unsupported scene", scene)
		}
	}
}

func TestFindFixedReturnsOnlyKnownScenes(t *testing.T) {
	for _, scene := range []string{"login", "forget", "bind_phone", "change_password"} {
		value, found := FindFixed(scene)
		if !found || value.Scene != scene {
			t.Fatalf("FindFixed(%q) = %+v,%v", scene, value, found)
		}
	}
	if _, found := FindFixed("test"); found {
		t.Fatal("FindFixed accepted the admin test mode as a scene")
	}
}
