package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"admin/server/internal/module/storage/uploadRule"
)

func TestManifestRejectsMismatchedMetadataAndResolvedURL(t *testing.T) {
	m := manifest{ObjectKey: "file/.admin-storage/v2/p1/r2/c3/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv", PlatformID: 1, RuleID: 2, SHA256: "expected", PublicURL: "https://example.com/template.csv"}
	resolved := uploadrule.ObjectURLResult{URL: m.PublicURL}
	if err := validateManifest(m, "expected", 1, resolved); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*manifest){
		func(m *manifest) { m.RuleID = 3 }, func(m *manifest) { m.PlatformID = 2 }, func(m *manifest) { m.SHA256 = "wrong" }, func(m *manifest) { m.PublicURL = "https://example.com/different.csv" },
	} {
		bad := m
		change(&bad)
		if err := validateManifest(bad, "expected", 1, resolved); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}

func TestManifestIsPublishedCompletelyAndNeverOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	want := manifest{ObjectKey: "object", SHA256: "expected"}
	if err := writeManifest(path, want); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		t.Fatalf("manifest=%+v err=%v", got, err)
	}
	if err := writeManifest(path, manifest{SHA256: "replacement"}); err == nil {
		t.Fatal("existing manifest overwritten")
	}
	if err := writeManifest(filepath.Join(dir, "missing", "manifest.json"), want); err == nil {
		t.Fatal("missing directory accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left: %v err=%v", entries, err)
	}
}

func TestCSVRuleExtensionPreservesOtherFieldsAndIsIdempotent(t *testing.T) {
	rule := uploadrule.RuleValue{ID: 1, PlatformID: 1, CosConfigID: 1, Codes: []string{"avatar", "file"}, Name: "Files", MaxFileSizeBytes: 1024, AllowedExtensions: []string{"png", "zip"}, AllowedMimeTypes: []string{"image/png", "application/zip"}, AccessMode: "public", Remark: "keep"}
	next, changed := csvInput(rule)
	if !changed || next.Name != rule.Name || next.MaxFileSizeBytes != rule.MaxFileSizeBytes || next.Remark != rule.Remark || !reflect.DeepEqual(next.Codes, rule.Codes) {
		t.Fatalf("changed fields: %+v", next)
	}
	if !reflect.DeepEqual(next.AllowedExtensions, []string{"png", "zip", "csv"}) || !reflect.DeepEqual(next.AllowedMimeTypes, []string{"image/png", "application/zip", "text/csv"}) {
		t.Fatalf("allowlists=%+v", next)
	}
	if !reflect.DeepEqual(rule.AllowedExtensions, []string{"png", "zip"}) {
		t.Fatal("mutated original rule")
	}
	rule.AllowedExtensions = next.AllowedExtensions
	rule.AllowedMimeTypes = next.AllowedMimeTypes
	if _, changed := csvInput(rule); changed {
		t.Fatal("rerun changed rule")
	}
	rule.AllowedMimeTypes = nil
	if next, changed := csvInput(rule); changed || len(next.AllowedMimeTypes) != 0 {
		t.Fatal("unrestricted MIME types were narrowed")
	}
}

func TestDownloadVerificationRejectsErrorsAndDifferentBytes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"matching", 200, "template", false}, {"wrong bytes", 200, "different", true}, {"too long", 200, "template extra", true}, {"failure", 403, "template", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := downloadMatches(context.Background(), server.Client(), server.URL, []byte("template"))
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if err := downloadMatches(context.Background(), http.DefaultClient, "http://example.com/template.csv", []byte("template")); err == nil {
		t.Fatal("insecure URL accepted")
	}
}
