package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCompoundModuleDirectoriesUseLowerCamelCase(t *testing.T) {
	for _, module := range []struct{ parent, expected, legacy string }{
		{"permission", "authPlatform", "authplatform"}, {"permission", "permissionVersion", "permissionversion"},
		{"permission", "roleMenu", "rolemenu"}, {"permission", "userRole", "userrole"},
		{"user", "loginLog", "loginlog"}, {"system", "operationLog", "operationlog"},
		{"storage", "cosConfig", "cosconfig"}, {"storage", "uploadRule", "uploadrule"},
		{"message/mail", "logVerification", "logverification"},
		{"message/mail", "rateLimitPolicy", "ratelimitpolicy"},
		{"message/mail", "recipientRule", "recipientrule"},
		{"../shared", "cacheFill", "cachefill"},
	} {
		entries, err := os.ReadDir(filepath.Join("..", "module", module.parent))
		if err != nil {
			t.Fatal(err)
		}
		var expectedFound, legacyFound bool
		for _, entry := range entries {
			expectedFound = expectedFound || entry.IsDir() && entry.Name() == module.expected
			legacyFound = legacyFound || entry.IsDir() && entry.Name() == module.legacy
		}
		if !expectedFound || legacyFound {
			t.Fatalf("module directory parent=%s expected=%s legacy=%s entries=%v", module.parent, module.expected, module.legacy, entries)
		}
	}
	for _, legacyRootFile := range []string{"model.go", "repository.go", "schema.go"} {
		if _, err := os.Stat(filepath.Join("..", "module", "message", "mail", legacyRootFile)); !os.IsNotExist(err) {
			t.Fatalf("root Mail still owns resource persistence file %s", legacyRootFile)
		}
	}
}

func TestBusinessModulesKeepCanonicalTableNames(t *testing.T) {
	for module, tables := range map[string][]string{
		"user/account": {"user_account"}, "user/profile": {"user_profile"},
		"user/session": {"user_session"}, "user/loginLog": {"user_login_log"},
		"permission/authPlatform": {"permission_auth_platform"}, "permission/menu": {"permission_menu"},
		"permission/role": {"permission_role"}, "system/operationLog": {"system_operation_log"},
		"storage/cosConfig": {"storage_cos_config"}, "storage/uploadRule": {"storage_upload_rule", "storage_upload_rule_code"},
		"message/mail/config": {"message_mail_config"}, "message/mail/template": {"message_mail_template"},
		"message/mail/log": {"message_mail_log"}, "message/mail/logVerification": {"message_mail_log_verification"},
		"message/mail/rateLimitPolicy": {"message_mail_rate_limit_policy"}, "message/mail/recipientRule": {"message_mail_recipient_rule"},
	} {
		t.Run(module, func(t *testing.T) {
			path := filepath.Join("..", "module", module, "model.go")
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			found := make(map[string]bool)
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "TableName" || fn.Recv == nil {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					ret, ok := node.(*ast.ReturnStmt)
					if !ok || len(ret.Results) != 1 {
						return true
					}
					literal, ok := ret.Results[0].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						return true
					}
					value, err := strconv.Unquote(literal.Value)
					if err == nil {
						found[value] = true
					}
					return true
				})
			}
			for _, table := range tables {
				if !found[table] {
					t.Errorf("missing canonical table %s", table)
				}
			}
			if len(found) != len(tables) {
				t.Fatalf("unexpected table mapping: %v", found)
			}
		})
	}
}
