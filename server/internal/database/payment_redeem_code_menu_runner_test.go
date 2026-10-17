package database_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaymentRedeemCodeMenuRunnerHelperCompiles(t *testing.T) {
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "docs", "database", "2026-10-17-payment-redeem-code-menu.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	_, helper, found := strings.Cut(normalized, "$helper = @'\n")
	if !found {
		t.Fatal("embedded maintenance helper missing")
	}
	helper, _, found = strings.Cut(helper, "\n'@")
	if !found || !strings.HasPrefix(helper, "package main\n") {
		t.Fatal("embedded maintenance helper is malformed")
	}
	temp, err := os.MkdirTemp(filepath.Join(root, "server", "tmp"), "paymentRedeemMenuRunner-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temp) })
	if err := os.WriteFile(filepath.Join(temp, "main.go"), []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-run", "^$", "-count=1", "./tmp/"+filepath.Base(temp))
	command.Dir = filepath.Join(root, "server")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("maintenance helper does not compile: %v\n%s", err, output)
	}
}
