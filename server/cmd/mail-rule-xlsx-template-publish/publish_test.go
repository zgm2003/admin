package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	uploadrule "admin/server/internal/module/storage/uploadRule"
	systemsetting "admin/server/internal/module/system/setting"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
)

func TestXLSXRuleExtensionPreservesOtherRestrictions(t *testing.T) {
	rule := uploadrule.RuleValue{ID: 1, PlatformID: 1, CosConfigID: 2, Codes: []string{"avatar", "file", "setting"}, Name: "Keep", MaxFileSizeBytes: 1024, AllowedExtensions: []string{"csv", "png"}, AllowedMimeTypes: []string{"text/csv", "image/png"}, AccessMode: "public", IsEnabled: yesno.Yes, Remark: "unchanged"}
	next, changed := xlsxInput(rule)
	if !changed || next.Name != rule.Name || next.MaxFileSizeBytes != rule.MaxFileSizeBytes || next.Remark != rule.Remark || !reflect.DeepEqual(next.Codes, rule.Codes) {
		t.Fatalf("unrelated fields changed: %+v", next)
	}
	if !reflect.DeepEqual(next.AllowedExtensions, []string{"csv", "png", "xlsx"}) || !reflect.DeepEqual(next.AllowedMimeTypes, []string{"text/csv", "image/png", xlsxMIME}) {
		t.Fatalf("allowlists: %+v", next)
	}
	if !reflect.DeepEqual(rule.AllowedExtensions, []string{"csv", "png"}) {
		t.Fatal("mutated source")
	}
	rule.AllowedExtensions, rule.AllowedMimeTypes = next.AllowedExtensions, next.AllowedMimeTypes
	if _, changed := xlsxInput(rule); changed {
		t.Fatal("rerun changes rule")
	}
	rule.AllowedMimeTypes = nil
	if next, changed := xlsxInput(rule); changed || len(next.AllowedMimeTypes) != 0 {
		t.Fatal("unrestricted MIME narrowed")
	}
	rule.AllowedExtensions = []string{"csv"}
	if next, changed := xlsxInput(rule); !changed || len(next.AllowedMimeTypes) != 0 {
		t.Fatal("extension narrowed MIME")
	}
}

const oldTestKey = "file/.admin-storage/v2/p1/r2/c3/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv"
const newTestKey = "setting/.admin-storage/v2/p1/r2/c3/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx"

func testFacts() facts {
	return facts{
		Setting:     systemsetting.Record{ID: 13, Key: sharedsetting.MailRecipientRuleImportTemplateObjectKey, Value: oldTestKey, ValueType: 5, Description: "CSV template", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		Rule:        uploadrule.RuleValue{ID: 2, PlatformID: 1, CosConfigID: 3, Codes: []string{"file", "setting"}, Name: "Keep", MaxFileSizeBytes: 104857600, AllowedExtensions: []string{"csv", "xlsx"}, AllowedMimeTypes: []string{"text/csv", xlsxMIME}, AccessMode: "public", IsEnabled: yesno.Yes},
		Generations: []generationFact{{Namespace: "system.setting", ScopeKey: "global", Generation: 17}, {Namespace: "message.mail", ScopeKey: "global", Generation: 4}},
	}
}

func testManifest(content []byte, f facts) manifest {
	return manifest{Version: 1, FileName: templateName, SHA256: digest(content), SizeBytes: int64(len(content)), SettingID: f.Setting.ID, OldObjectKey: f.Setting.Value, NewObjectKey: newTestKey, OldDescription: f.Setting.Description, NewDescription: templateDescription, PlatformID: f.Rule.PlatformID, RuleID: f.Rule.ID, CosConfigID: f.Rule.CosConfigID, PhysicalVersion: 1, BaseGeneration: 17, RuleSHA256: ruleDigest(f.Rule)}
}

func TestManifestStrictValidationAndImmutablePublication(t *testing.T) {
	content := []byte("workbook")
	f := testFacts()
	m := testManifest(content, f)
	if err := validateManifest(m, content, f); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*manifest){
		"wrong hash":        func(m *manifest) { m.SHA256 = strings.Repeat("0", 64) },
		"wrong size":        func(m *manifest) { m.SizeBytes++ },
		"wrong setting":     func(m *manifest) { m.SettingID++ },
		"wrong platform":    func(m *manifest) { m.PlatformID++ },
		"wrong rule":        func(m *manifest) { m.RuleID++ },
		"wrong config":      func(m *manifest) { m.CosConfigID++ },
		"wrong version":     func(m *manifest) { m.PhysicalVersion++ },
		"arbitrary URL":     func(m *manifest) { m.NewObjectKey = "https://example.invalid/t.xlsx" },
		"wrong upload code": func(m *manifest) { m.NewObjectKey = strings.Replace(newTestKey, "setting/", "file/", 1) },
		"wrong original":    func(m *manifest) { m.OldObjectKey = newTestKey },
		"generation drift":  func(m *manifest) { m.BaseGeneration++ },
		"rule drift":        func(m *manifest) { m.RuleSHA256 = strings.Repeat("0", 64) },
		"wrong description": func(m *manifest) { m.NewDescription = "wrong" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := m
			change(&bad)
			if validateManifest(bad, content, f) == nil {
				t.Fatal("accepted inconsistent manifest")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := writeManifest(path, m); err != nil {
		t.Fatal(err)
	}
	got, exists, err := readManifest(path)
	if err != nil || !exists || got != m {
		t.Fatalf("manifest=%+v exists=%v err=%v", got, exists, err)
	}
	if err := writeManifest(path, m); err == nil {
		t.Fatal("overwrote manifest")
	}
	raw, _ := json.Marshal(m)
	for _, bad := range [][]byte{nil, []byte("null"), []byte("{}"), append(raw, []byte("{}")...), []byte(`{"version":1,"version":1}`), []byte(`{"unexpected":1}`), bytes.Repeat([]byte(" "), 17000)} {
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readManifest(path); err == nil {
			t.Fatalf("accepted malformed manifest %q", string(bad[:min(len(bad), 100)]))
		}
	}
}

func TestPublicationDownloadFailureNeverBindsAndResumeDoesNotUploadAgain(t *testing.T) {
	for _, failMode := range []string{"HTTP", "different bytes"} {
		t.Run(failMode, func(t *testing.T) {
			content := []byte("verified workbook bytes")
			f := testFacts()
			stored := false
			failDownload := true
			puts, issued, binds, syncs := 0, 0, 0, 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					if r.Header.Get("x-cos-forbid-overwrite") != "true" || r.Header.Get("Content-Type") != xlsxMIME {
						t.Error("missing standard upload restrictions")
					}
					puts++
					stored = true
					w.WriteHeader(http.StatusOK)
					return
				}
				if !stored {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if failDownload {
					if failMode == "HTTP" {
						w.WriteHeader(http.StatusServiceUnavailable)
					} else {
						_, _ = w.Write([]byte("wrong bytes"))
					}
					return
				}
				_, _ = w.Write(content)
			}))
			defer server.Close()
			ops := publicationIO{
				inspect: func(context.Context) (facts, error) { return f, nil },
				extend:  func(context.Context, uploadrule.RuleValue) error { t.Fatal("already allowed"); return nil },
				issue: func(context.Context, facts, int64) (uploadrule.CredentialItem, error) {
					issued++
					u := server.URL
					return uploadrule.CredentialItem{ObjectKey: newTestKey, UploadURL: server.URL, PublicURL: &u, Method: http.MethodPut, Headers: map[string]string{"Content-Type": xlsxMIME}}, nil
				},
				resolve: func(context.Context, int64, string) (uploadrule.ObjectURLResult, error) {
					return uploadrule.ObjectURLResult{URL: server.URL}, nil
				},
				resume: func(context.Context, manifest) (storagecos.PutResult, error) {
					t.Fatal("existing object must not be uploaded again")
					return storagecos.PutResult{}, nil
				},
				bind: func(_ context.Context, m manifest) error {
					binds++
					f.Setting.Value = m.NewObjectKey
					f.Setting.Description = m.NewDescription
					f.Generations[0].Generation++
					return nil
				},
				sync:        func(context.Context, int64) error { syncs++; return nil },
				verifyReady: func(context.Context, int64) error { return nil },
				client:      server.Client(),
			}
			path := filepath.Join(t.TempDir(), "manifest.json")
			if _, err := publishTemplate(context.Background(), content, path, ops); err == nil {
				t.Fatal("download failure accepted")
			}
			if puts != 1 || issued != 1 || binds != 0 || syncs != 0 || f.Setting.Value != oldTestKey || f.Generations[0].Generation != 17 {
				t.Fatalf("partial state: puts=%d issued=%d binds=%d syncs=%d facts=%+v", puts, issued, binds, syncs, f)
			}
			failDownload = false
			if _, err := publishTemplate(context.Background(), content, path, ops); err != nil {
				t.Fatal(err)
			}
			if _, err := publishTemplate(context.Background(), content, path, ops); err != nil {
				t.Fatal(err)
			}
			if puts != 1 || issued != 1 || binds != 1 || f.Generations[0].Generation != 18 || f.Generations[1].Generation != 4 {
				t.Fatalf("not idempotent: puts=%d issued=%d binds=%d", puts, issued, binds)
			}
		})
	}
}

func TestMissingManifestForAlreadyBoundXLSXAndRuleDriftAreRejected(t *testing.T) {
	content := []byte("workbook")
	f := testFacts()
	f.Setting.Value = newTestKey
	ops := publicationIO{inspect: func(context.Context) (facts, error) { return f, nil }}
	if _, err := publishTemplate(context.Background(), content, filepath.Join(t.TempDir(), "absent.json"), ops); err == nil {
		t.Fatal("bound setting accepted without manifest")
	}
	f = testFacts()
	m := testManifest(content, f)
	f.Rule.MaxFileSizeBytes++
	if validateManifest(m, content, f) == nil {
		t.Fatal("rule drift accepted")
	}
}

func TestReadFailureAndMissingObjectAreDistinct(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		err := downloadMatches(context.Background(), server.Client(), server.URL, []byte("x"))
		server.Close()
		if err == nil || errors.Is(err, errObjectMissing) != (status == http.StatusNotFound) {
			t.Fatalf("status %d: %v", status, err)
		}
	}
}

func TestDigestIsSHA256(t *testing.T) {
	if got := digest([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || len(got) != sha256.Size*2 {
		t.Fatal(got)
	}
}

func TestPublicationModeRequiresExplicitStoppedFlag(t *testing.T) {
	for _, args := range [][]string{{"-mode", "publish"}, {"-mode", "bad"}, {"-mode", "inspect", "-old-api-stopped"}, {"unexpected"}} {
		if _, _, err := parseMode(args); err == nil {
			t.Fatalf("accepted unsafe flags %v", args)
		}
	}
	for _, args := range [][]string{nil, {"-mode", "verify"}, {"-mode", "publish", "-old-api-stopped"}} {
		if _, _, err := parseMode(args); err != nil {
			t.Fatalf("rejected flags %v: %v", args, err)
		}
	}
}
