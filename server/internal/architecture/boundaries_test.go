package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This is a source-boundary gate, not a substitute for business or I/O tests.
func violations(path string, file *ast.File) []string {
	var result []string
	base := filepath.Base(path)
	isHandler := strings.Contains(base, "handler")
	isService := strings.Contains(base, "service")
	isRepository := strings.Contains(base, "repository")
	imports := make(map[string]string)
	for _, item := range file.Imports {
		name, _ := strconv.Unquote(item.Path.Value)
		alias := filepath.Base(name)
		if item.Name != nil {
			alias = item.Name.Name
		}
		imports[alias] = name
		business := strings.Contains(name, "/internal/module/")
		gin := name == "github.com/gin-gonic/gin"
		redis := strings.Contains(name, "redis")
		queue := strings.HasSuffix(name, "/internal/queue") || strings.Contains(name, "asynq")
		orm := strings.HasPrefix(name, "gorm.io/")
		storage := strings.Contains(name, "/internal/module/storage/")
		provider := strings.Contains(name, "tencentcloud")
		if (strings.Contains(path, "/shared/") || strings.Contains(path, "/queue/")) && business {
			result = append(result, "shared/queue imports a business module: "+name)
		}
		if isHandler && (orm || redis || queue || provider || (storage && !strings.HasSuffix(name, "/protocol"))) {
			result = append(result, "handler imports I/O dependency: "+name)
		}
		if isService && gin {
			result = append(result, "service imports Gin")
		}
		if isRepository && (gin || redis || queue || provider) {
			result = append(result, "repository imports non-PostgreSQL I/O: "+name)
		}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		receiver := ""
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			expr := fn.Recv.List[0].Type
			if ptr, ok := expr.(*ast.StarExpr); ok {
				expr = ptr.X
			}
			if id, ok := expr.(*ast.Ident); ok {
				receiver = id.Name
			}
		}
		if receiver == "Handler" && !isHandler {
			result = append(result, "Handler method outside handler file: "+fn.Name.Name)
		}
		if receiver == "Repository" && isService {
			result = append(result, "Repository method in service file: "+fn.Name.Name)
		}
		if receiver == "Service" {
			ast.Inspect(fn, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if id, ok := selector.X.(*ast.Ident); ok && imports[id.Name] == "github.com/gin-gonic/gin" {
					result = append(result, "Service depends on Gin: "+fn.Name.Name)
				}
				if selector.Sel.Name == "db" {
					result = append(result, "Service accesses database field: "+fn.Name.Name)
				}
				return true
			})
		}
		if strings.Contains(path, "/cmd/") {
			ast.Inspect(fn, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case "AutoMigrate", "EnsureSchema", "PrepareSchema", "EnsureFoundation", "EnsureSystemRoles":
					result = append(result, "runtime entry calls schema/seed mutation: "+selector.Sel.Name)
				}
				return true
			})
		}
	}
	return result
}

func TestBackendSourceBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, issue := range violations(filepath.ToSlash(path), file) {
			t.Errorf("%s: %s", path, issue)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBoundaryGateRejectsViolations(t *testing.T) {
	for _, test := range []struct{ path, source string }{
		{"/internal/shared/x.go", `package x; import "admin/server/internal/module/user/account"`},
		{"/module/x/handler.go", `package x; import "gorm.io/gorm"`},
		{"/module/x/service.go", `package x; func(r *Repository) List(){}`},
		{"/module/x/password.go", `package x; import transport "github.com/gin-gonic/gin"; func(s *Service) Run(c *transport.Context){}`},
		{"/module/x/repository.go", `package x; import "github.com/redis/go-redis/v9"`},
		{"/cmd/api/main.go", `package main; func main(){ db.AutoMigrate() }`},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), test.path, test.source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations(test.path, file)) == 0 {
			t.Errorf("accepted %s", test.path)
		}
	}
}
