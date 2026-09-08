package mail

import "fmt"

type FixedTemplate struct {
	Scene             string   `json:"scene"`
	Name              string   `json:"name"`
	Subject           string   `json:"subject"`
	TencentTemplateID int      `json:"tencentTemplateId"`
	Variables         []string `json:"variables"`
}

func FixedTemplates() []FixedTemplate {
	vars := []string{"code", "ttl_minutes"}
	return []FixedTemplate{
		{Scene: SceneLogin, Name: "邮箱验证码登录", Subject: "登录验证码", TencentTemplateID: 47941, Variables: append([]string(nil), vars...)},
		{Scene: SceneForget, Name: "找回密码", Subject: "找回密码验证码", TencentTemplateID: 47942, Variables: append([]string(nil), vars...)},
		{Scene: SceneBindEmail, Name: "绑定/换绑邮箱", Subject: "绑定邮箱验证码", TencentTemplateID: 47943, Variables: append([]string(nil), vars...)},
		{Scene: SceneChangePassword, Name: "验证码改密", Subject: "修改密码验证码", TencentTemplateID: 47944, Variables: append([]string(nil), vars...)},
	}
}

func ValidateScene(value string) error {
	for _, item := range FixedTemplates() {
		if value == item.Scene {
			return nil
		}
	}
	return fmt.Errorf("invalid mail scene")
}

// isVerifyCodeScene reports whether the scene participates in the auth
// verification-code contract: login serves code login, forget serves password
// recovery. Other scenes (bind_email, change_password) stay closed until
// their own flows land.
func isVerifyCodeScene(scene string) bool {
	return scene == SceneLogin || scene == SceneForget
}

func ValidateStatus(value string) error {
	if value == StatusPending || value == StatusSent || value == StatusFailed {
		return nil
	}
	return fmt.Errorf("invalid mail status")
}
