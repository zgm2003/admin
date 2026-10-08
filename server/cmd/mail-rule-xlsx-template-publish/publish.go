// Explicit, resumable maintenance for this template only. No startup hooks.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	uploadrule "admin/server/internal/module/storage/uploadRule"
	systemsetting "admin/server/internal/module/system/setting"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"admin/server/internal/storage/objectKey"
)

const templateName = "mail-recipient-rule-import.xlsx"
const templateDescription = "邮件收件规则 Excel 模板（.xlsx），使用 setting 上传规则，仅保存 COS 对象键"
const xlsxMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

var errObjectMissing = errors.New("template object does not exist")

type generationFact struct {
	Namespace  string `json:"namespace"`
	ScopeKey   string `json:"scopeKey"`
	Generation int64  `json:"generation"`
}
type facts struct {
	Setting     systemsetting.Record `json:"setting"`
	Rule        uploadrule.RuleValue `json:"rule"`
	Generations []generationFact     `json:"generations"`
	ControlHash string               `json:"controlHash"`
}
type manifest struct {
	Version         int    `json:"version"`
	FileName        string `json:"fileName"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	SettingID       int64  `json:"settingId"`
	OldObjectKey    string `json:"oldObjectKey"`
	NewObjectKey    string `json:"newObjectKey"`
	OldDescription  string `json:"oldDescription"`
	NewDescription  string `json:"newDescription"`
	PlatformID      int64  `json:"platformId"`
	RuleID          int64  `json:"ruleId"`
	CosConfigID     int64  `json:"cosConfigId"`
	PhysicalVersion int64  `json:"physicalVersion"`
	BaseGeneration  int64  `json:"baseGeneration"`
	RuleSHA256      string `json:"ruleSha256"`
	ControlHash     string `json:"controlHash"`
}

// Function dependencies keep offline failure tests away from real COS and data.
// This is a single maintenance operation, not a runtime importer registry.
type publicationIO struct {
	inspect     func(context.Context) (facts, error)
	extend      func(context.Context, uploadrule.RuleValue) error
	issue       func(context.Context, facts, int64) (uploadrule.CredentialItem, error)
	resolve     func(context.Context, int64, string) (uploadrule.ObjectURLResult, error)
	resume      func(context.Context, manifest) (storagecos.PutResult, error)
	bind        func(context.Context, manifest) error
	sync        func(context.Context, int64) error
	verifyReady func(context.Context, int64) error
	client      *http.Client
}

func digest(value []byte) string { return fmt.Sprintf("%x", sha256.Sum256(value)) }
func ruleDigest(rule uploadrule.RuleValue) string {
	// Timestamp changes are excluded; all policy and routing facts are included.
	in := struct {
		ID, PlatformID, CosConfigID int64
		AccessMode                  string
		IsEnabled                   yesno.Value
		Policy                      uploadrule.UpdateInput
	}{
		rule.ID, rule.PlatformID, rule.CosConfigID, rule.AccessMode, rule.IsEnabled,
		uploadrule.UpdateInput{Codes: rule.Codes, Name: rule.Name, MaxFileSizeBytes: rule.MaxFileSizeBytes, AllowedExtensions: rule.AllowedExtensions, AllowedMimeTypes: rule.AllowedMimeTypes, Remark: rule.Remark},
	}
	encoded, _ := json.Marshal(in)
	return digest(encoded)
}
func settingGeneration(f facts) int64 {
	for _, g := range f.Generations {
		if g.Namespace == "system.setting" && g.ScopeKey == "global" {
			return g.Generation
		}
	}
	return 0
}
func xlsxInput(rule uploadrule.RuleValue) (uploadrule.UpdateInput, bool) {
	in := uploadrule.UpdateInput{Codes: slices.Clone(rule.Codes), Name: rule.Name, MaxFileSizeBytes: rule.MaxFileSizeBytes, AllowedExtensions: slices.Clone(rule.AllowedExtensions), AllowedMimeTypes: slices.Clone(rule.AllowedMimeTypes), Remark: rule.Remark}
	changed := false
	if !slices.Contains(in.AllowedExtensions, "xlsx") {
		in.AllowedExtensions = append(in.AllowedExtensions, "xlsx")
		changed = true
	}
	if len(in.AllowedMimeTypes) > 0 && !slices.Contains(in.AllowedMimeTypes, xlsxMIME) {
		in.AllowedMimeTypes = append(in.AllowedMimeTypes, xlsxMIME)
		changed = true
	}
	return in, changed
}
func validateManifest(m manifest, content []byte, f facts) error {
	c, err := objectkey.Parse(m.NewObjectKey)
	if err != nil || !strings.HasPrefix(m.NewObjectKey, "setting/") || !strings.HasSuffix(m.NewObjectKey, ".xlsx") || m.OldObjectKey == m.NewObjectKey || objectkey.Validate(m.OldObjectKey) != nil || !strings.HasSuffix(m.OldObjectKey, ".csv") {
		return errors.New("template manifest contains invalid object keys")
	}
	if m.Version != 1 || m.FileName != templateName || m.SHA256 != digest(content) || m.SizeBytes != int64(len(content)) || len(content) == 0 || m.SettingID != f.Setting.ID || m.NewDescription != templateDescription ||
		m.PlatformID != f.Rule.PlatformID || m.RuleID != f.Rule.ID || m.CosConfigID != f.Rule.CosConfigID || m.PhysicalVersion != c.Version || c.PlatformID != m.PlatformID || c.RuleID != m.RuleID || c.CosConfigID != m.CosConfigID || m.RuleSHA256 != ruleDigest(f.Rule) || m.ControlHash != f.ControlHash || m.BaseGeneration < 1 {
		return errors.New("template manifest does not match current template or policy facts")
	}
	if f.Setting.Key != sharedsetting.MailRecipientRuleImportTemplateObjectKey || f.Setting.ValueType != systemsetting.ValueTypeMedia || f.Setting.IsBuiltin != yesno.Yes || f.Setting.IsEnabled != yesno.Yes {
		return errors.New("template setting is unavailable or not media")
	}
	if f.Setting.Value == m.OldObjectKey && f.Setting.Description == m.OldDescription && settingGeneration(f) == m.BaseGeneration {
		return nil
	}
	if f.Setting.Value == m.NewObjectKey && f.Setting.Description == m.NewDescription && settingGeneration(f) == m.BaseGeneration+1 {
		return nil
	}
	return errors.New("template setting or generation changed; refusing to overwrite")
}

func readManifest(path string) (manifest, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return manifest{}, false, nil
	}
	if err != nil {
		return manifest{}, false, errors.New("manifest cannot be opened")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(raw) > 16384 {
		return manifest{}, true, errors.New("manifest cannot be read or is oversized")
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, true, errors.New("manifest JSON is invalid")
	}
	canonical, _ := json.Marshal(m)
	var expected map[string]json.RawMessage
	_ = json.Unmarshal(canonical, &expected)
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return m, true, errors.New("manifest must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] || expected[key] == nil {
			return m, true, errors.New("manifest fields are invalid")
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(value, []byte("null")) {
			return m, true, errors.New("manifest value is invalid")
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || len(seen) != len(expected) {
		return m, true, errors.New("manifest fields are missing")
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return m, true, errors.New("manifest has trailing JSON")
	}
	return m, true, nil
}
func writeManifest(path string, m manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".xlsx-template-*.json")
	if err != nil {
		return errors.New("cannot create manifest temporary file")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err != nil {
		return errors.New("cannot write manifest")
	}
	if err = f.Sync(); err != nil {
		return errors.New("cannot sync manifest")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close manifest")
	}
	if err = os.Link(f.Name(), path); err != nil {
		return errors.New("manifest publication failed or manifest already exists")
	}
	return nil
}

func downloadMatches(ctx context.Context, client *http.Client, address string, content []byte) error {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("template resolver must return an HTTPS URL without credentials")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return errors.New("template download request invalid")
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("template download failed")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errObjectMissing
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("template download HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, int64(len(content)+1)))
	if err != nil || !bytes.Equal(body, content) {
		return errors.New("template download byte verification failed")
	}
	return nil
}
func putTemplate(ctx context.Context, ops publicationIO, signed storagecos.PutResult, content []byte) error {
	u, err := url.Parse(signed.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("COS PUT URL invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed.URL, bytes.NewReader(content))
	if err != nil {
		return errors.New("COS PUT request invalid")
	}
	for key, value := range signed.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", xlsxMIME)
	req.Header.Set("x-cos-forbid-overwrite", "true")
	req.Header.Set("Content-Disposition", `attachment; filename="`+templateName+`"`)
	res, err := ops.client.Do(req)
	if err != nil {
		return errors.New("COS PUT failed; resume with saved manifest")
	}
	defer res.Body.Close()
	if (res.StatusCode < 200 || res.StatusCode >= 300) && res.StatusCode != http.StatusConflict {
		return fmt.Errorf("COS PUT HTTP %d", res.StatusCode)
	}
	return nil
}

func publishTemplate(ctx context.Context, content []byte, path string, ops publicationIO) (manifest, error) {
	f, err := ops.inspect(ctx)
	if err != nil {
		return manifest{}, err
	}
	m, exists, err := readManifest(path)
	if err != nil {
		return m, err
	}
	var signed storagecos.PutResult
	if !exists {
		if f.Setting.ValueType != 5 || f.Setting.IsBuiltin != yesno.Yes || f.Setting.IsEnabled != yesno.Yes || objectkey.Validate(f.Setting.Value) != nil || !strings.HasSuffix(f.Setting.Value, ".csv") {
			return m, errors.New("an original CSV media setting is required before first publication")
		}
		if f.Rule.IsEnabled != yesno.Yes || f.Rule.AccessMode != "public" || !slices.Contains(f.Rule.Codes, "setting") || f.Rule.MaxFileSizeBytes < int64(len(content)) {
			return m, errors.New("active public setting upload rule unavailable")
		}
		if _, changed := xlsxInput(f.Rule); changed {
			if err := ops.extend(ctx, f.Rule); err != nil {
				return m, err
			}
			f, err = ops.inspect(ctx)
			if err != nil {
				return m, err
			}
		}
		item, err := ops.issue(ctx, f, int64(len(content)))
		if err != nil {
			return m, err
		}
		coords, err := objectkey.Parse(item.ObjectKey)
		if err != nil {
			return m, errors.New("issued object key invalid")
		}
		m = manifest{Version: 1, FileName: templateName, SHA256: digest(content), SizeBytes: int64(len(content)), SettingID: f.Setting.ID, OldObjectKey: f.Setting.Value, NewObjectKey: item.ObjectKey, OldDescription: f.Setting.Description, NewDescription: templateDescription, PlatformID: f.Rule.PlatformID, RuleID: f.Rule.ID, CosConfigID: f.Rule.CosConfigID, PhysicalVersion: coords.Version, BaseGeneration: settingGeneration(f), RuleSHA256: ruleDigest(f.Rule), ControlHash: f.ControlHash}
		if err := validateManifest(m, content, f); err != nil {
			return m, err
		}
		if err := writeManifest(path, m); err != nil {
			return m, err
		}
		signed = storagecos.PutResult{URL: item.UploadURL, Headers: item.Headers}
	} else if err := validateManifest(m, content, f); err != nil {
		return m, err
	}
	resolved, err := ops.resolve(ctx, m.PlatformID, m.NewObjectKey)
	if err != nil {
		return m, errors.New("template object resolver failed")
	}
	if resolved.ExpiresAt != nil {
		return m, errors.New("public template object unexpectedly has expiry")
	}
	if !exists {
		if err := putTemplate(ctx, ops, signed, content); err != nil {
			return m, err
		}
	} else if err := downloadMatches(ctx, ops.client, resolved.URL, content); err != nil {
		if !errors.Is(err, errObjectMissing) || f.Setting.Value == m.NewObjectKey {
			return m, err
		}
		signed, err = ops.resume(ctx, m)
		if err != nil {
			return m, err
		}
		if err := putTemplate(ctx, ops, signed, content); err != nil {
			return m, err
		}
	}
	if err := downloadMatches(ctx, ops.client, resolved.URL, content); err != nil {
		return m, err
	}
	f, err = ops.inspect(ctx)
	if err != nil {
		return m, err
	}
	if err := validateManifest(m, content, f); err != nil {
		return m, err
	}
	if f.Setting.Value != m.NewObjectKey {
		if err := ops.bind(ctx, m); err != nil {
			return m, errors.New("template binding failed; inspect PostgreSQL and resume with manifest")
		}
	}
	f, err = ops.inspect(ctx)
	if err != nil {
		return m, err
	}
	if err := validateManifest(m, content, f); err != nil {
		return m, err
	}
	if f.Setting.Value != m.NewObjectKey {
		return m, errors.New("template binding not committed")
	}
	if err := ops.sync(ctx, settingGeneration(f)); err != nil {
		return m, err
	}
	if err := ops.verifyReady(ctx, settingGeneration(f)); err != nil {
		return m, err
	}
	return m, nil
}
