package main

import "testing"

func TestParseModeGuards(t *testing.T) {
	if mode, stopped, err := parseMode(nil); err != nil || mode != "inspect" || stopped {
		t.Fatalf("inspect defaults: %q %v %v", mode, stopped, err)
	}
	if _, _, err := parseMode([]string{"-mode=sync"}); err == nil {
		t.Fatal("sync without stop flag accepted")
	}
	if mode, stopped, err := parseMode([]string{"-mode=sync", "-old-api-stopped"}); err != nil || mode != "sync" || !stopped {
		t.Fatalf("sync parse: %q %v %v", mode, stopped, err)
	}
	for _, args := range [][]string{{"-mode=wat"}, {"extra"}, {"-old-api-stopped"}} {
		if _, _, err := parseMode(args); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
