package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBusinessModulesKeepCanonicalTableNames(t *testing.T) {
	for module, tables := range map[string][]string{
		"user/account": {"user_account"}, "user/profile": {"user_profile"},
		"user/session": {"user_session"}, "user/loginlog": {"user_login_log"},
		"permission/authplatform": {"permission_auth_platform"}, "permission/menu": {"permission_menu"},
		"permission/role": {"permission_role"}, "system/operationlog": {"system_operation_log"},
		"storage/cosconfig": {"storage_cos_config"}, "storage/uploadrule": {"storage_upload_rule", "storage_upload_rule_code"},
		"message/mail": {"message_mail_config", "message_mail_template", "message_mail_log", "message_mail_log_verification", "message_mail_rate_limit_policy", "message_mail_recipient_rule"},
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
