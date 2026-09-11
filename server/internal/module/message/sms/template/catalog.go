package template

import "fmt"

const (
	SceneLogin          = "login"
	SceneForget         = "forget"
	SceneBindPhone      = "bind_phone"
	SceneChangePassword = "change_password"
)

// Fixed describes one of the four SMS templates that always exist.
type Fixed struct {
	Scene         string   `json:"scene"`
	Name          string   `json:"name"`
	ParameterKeys []string `json:"parameterKeys"`
}

// FixedCatalog returns the four SMS scenes in stable order. SMS never imports
// the Mail scene constants even when a value happens to be identical.
func FixedCatalog() []Fixed {
	parameterKeys := []string{"code", "ttl_minutes"}
	return []Fixed{
		{Scene: SceneLogin, Name: "登录验证码", ParameterKeys: append([]string(nil), parameterKeys...)},
		{Scene: SceneForget, Name: "找回密码", ParameterKeys: append([]string(nil), parameterKeys...)},
		{Scene: SceneBindPhone, Name: "绑定/换绑手机", ParameterKeys: append([]string(nil), parameterKeys...)},
		{Scene: SceneChangePassword, Name: "修改密码", ParameterKeys: append([]string(nil), parameterKeys...)},
	}
}

func FindFixed(scene string) (Fixed, bool) {
	for _, value := range FixedCatalog() {
		if value.Scene == scene {
			return value, true
		}
	}
	return Fixed{}, false
}

func ValidateScene(scene string) error {
	if _, found := FindFixed(scene); !found {
		return fmt.Errorf("invalid sms scene")
	}
	return nil
}
