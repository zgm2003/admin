package template

import "fmt"

const (
	SceneLogin          = "login"
	SceneForget         = "forget"
	SceneBindEmail      = "bind_email"
	SceneChangePassword = "change_password"
)

type Fixed struct {
	Scene     string   `json:"scene"`
	Name      string   `json:"name"`
	Subject   string   `json:"subject"`
	Variables []string `json:"variables"`
}

func FixedCatalog() []Fixed {
	variables := []string{"code", "ttl_minutes"}
	return []Fixed{
		{Scene: SceneLogin, Name: "邮箱验证码登录", Subject: "登录验证码", Variables: append([]string(nil), variables...)},
		{Scene: SceneForget, Name: "找回密码", Subject: "找回密码验证码", Variables: append([]string(nil), variables...)},
		{Scene: SceneBindEmail, Name: "绑定/换绑邮箱", Subject: "绑定邮箱验证码", Variables: append([]string(nil), variables...)},
		{Scene: SceneChangePassword, Name: "验证码改密", Subject: "修改密码验证码", Variables: append([]string(nil), variables...)},
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
		return fmt.Errorf("invalid mail scene")
	}
	return nil
}

func IsVerificationScene(scene string) bool {
	return scene == SceneLogin || scene == SceneForget
}
