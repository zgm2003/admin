package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestMessageSMSHandlersAndServicesStayWithinLayerBoundaries(t *testing.T) {
	root := filepath.Join("..", "module", "message", "sms")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			name := strings.Trim(imported.Path.Value, "\"")
			base := filepath.Base(path)
			if strings.Contains(base, "handler") && (strings.Contains(name, "gorm.io") || strings.Contains(name, "/redis") || strings.Contains(name, "tencentcloud")) {
				t.Errorf("%s handler imports forbidden I/O dependency %s", path, name)
			}
			if strings.Contains(base, "service") && strings.Contains(name, "github.com/gin-gonic/gin") {
				t.Errorf("%s service imports Gin", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
