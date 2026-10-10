package architecture_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDictionaryRetirementAndStaticOptionOwnership(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "server", "internal", "module", "system", "dictionary")); !os.IsNotExist(err) {
		t.Fatal("dictionary production module remains")
	}
	main, err := os.ReadFile(filepath.Join(root, "server", "cmd", "api", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(main), "dictionary") {
		t.Fatal("API still owns dictionary assembly/routes")
	}
	for _, module := range []string{"user/profile", "storage/cosConfig", "message/mail/config", "message/sms/config"} {
		data, err := os.ReadFile(filepath.Join(root, "server", "internal", "module", module, "options.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"gorm", "redis", "cacheGeneration", "repository.", "gin-gonic", "shared/response"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("%s static options has infrastructure/HTTP dependency %s", module, forbidden)
			}
		}
	}
}
