package main

import (
	"strings"
	"testing"
)

func TestCleanupPatternsOnlyTargetRemovedDictionary(t *testing.T) {
	if len(cleanupPatterns) != 6 {
		t.Fatalf("patterns=%v", cleanupPatterns)
	}
	for _, pattern := range cleanupPatterns {
		if !strings.Contains(pattern, "dictionary") || strings.Contains(pattern, "authz:") || strings.Contains(pattern, "message.") {
			t.Fatalf("unsafe pattern %q", pattern)
		}
	}
}
func TestManifestRejectsForeignMigration(t *testing.T) {
	if err := validateManifest(manifest{Migration: "foreign", Database: "same"}, "same"); err == nil {
		t.Fatal("foreign migration accepted")
	}
	if err := validateManifest(manifest{Migration: migrationID, Database: "foreign"}, "same"); err == nil {
		t.Fatal("foreign database accepted")
	}
}
