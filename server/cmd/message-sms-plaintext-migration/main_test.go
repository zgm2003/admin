package main

import "testing"

func TestParseModeRequiresExplicitShutdownForSync(t *testing.T) {
	if mode, stopped, err := parseMode([]string{"-mode", "inspect"}); err != nil || mode != "inspect" || stopped {
		t.Fatalf("inspect=%q/%v err=%v", mode, stopped, err)
	}
	if _, _, err := parseMode([]string{"-mode", "sync"}); err == nil {
		t.Fatal("sync without shutdown was accepted")
	}
	if mode, stopped, err := parseMode([]string{"-mode", "sync", "-old-api-stopped"}); err != nil || mode != "sync" || !stopped {
		t.Fatalf("sync=%q/%v err=%v", mode, stopped, err)
	}
}
