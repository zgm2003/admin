package architecture_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMailOptionsDoNotEmbedLocalizedLabels(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "server", "internal", "module", "message", "mail", "options.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, literal := range []string{`"待发送"`, `"Pending"`, `"登录验证码"`, `"Business email sending`, `"认证平台 + 邮箱"`} {
		if strings.Contains(text, literal) {
			t.Fatalf("mail/options.go embeds localized label %q; use a shared i18n key", strings.Trim(literal, `"`))
		}
	}
}
