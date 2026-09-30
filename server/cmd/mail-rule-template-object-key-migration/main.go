// Explicit maintenance only: prepare the CSV through existing upload services,
// then let the forward SQL rename the setting. No startup migration or Redis deletion.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	auth "admin/server/internal/module/auth/login"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	cosconfig "admin/server/internal/module/storage/cosConfig"
	uploadrule "admin/server/internal/module/storage/uploadRule"
	systemsetting "admin/server/internal/module/system/setting"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"admin/server/internal/storage/objectKey"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const oldKey = "message.mail.recipient_rule.import_template_url"

type manifest struct {
	SourceURL  string `json:"sourceUrl"`
	ObjectKey  string `json:"objectKey"`
	PublicURL  string `json:"publicUrl"`
	SHA256     string `json:"sha256"`
	PlatformID int64  `json:"platformId"`
	RuleID     int64  `json:"ruleId"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validateManifest(m manifest, hash string, platformID int64, resolved uploadrule.ObjectURLResult) error {
	coords, err := objectkey.Parse(m.ObjectKey)
	if err != nil || !strings.HasSuffix(m.ObjectKey, ".csv") || m.SHA256 != hash ||
		m.PlatformID != platformID || coords.PlatformID != platformID || coords.RuleID != m.RuleID ||
		resolved.ExpiresAt != nil || resolved.URL != m.PublicURL || !strings.HasPrefix(resolved.URL, "https://") {
		return fmt.Errorf("manifest metadata or standard object URL does not match")
	}
	return nil
}

func writeManifest(path string, value manifest) error {
	if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("manifest already exists or cannot be inspected")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest encoding failed")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mail-template-*.json")
	if err != nil {
		return fmt.Errorf("manifest temporary file creation failed")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("manifest write failed")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("manifest sync failed")
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("manifest close failed")
	}
	// Atomically publish a complete same-volume file, refusing even a concurrent overwrite.
	if err := os.Link(file.Name(), path); err != nil {
		return fmt.Errorf("manifest atomic publication failed")
	}
	return nil
}

func csvInput(rule uploadrule.RuleValue) (uploadrule.UpdateInput, bool) {
	in := uploadrule.UpdateInput{Codes: slices.Clone(rule.Codes), Name: rule.Name, MaxFileSizeBytes: rule.MaxFileSizeBytes,
		AllowedExtensions: slices.Clone(rule.AllowedExtensions), AllowedMimeTypes: slices.Clone(rule.AllowedMimeTypes), Remark: rule.Remark}
	changed := false
	if !slices.Contains(in.AllowedExtensions, "csv") {
		in.AllowedExtensions = append(in.AllowedExtensions, "csv")
		changed = true
	}
	// Empty MIME restrictions already allow CSV; never narrow them as a side effect.
	if len(in.AllowedMimeTypes) > 0 && !slices.Contains(in.AllowedMimeTypes, "text/csv") {
		in.AllowedMimeTypes = append(in.AllowedMimeTypes, "text/csv")
		changed = true
	}
	return in, changed
}

func downloadMatches(ctx context.Context, client *http.Client, url string, content []byte) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("template must resolve to HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download URL invalid")
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("template download failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("template download HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, int64(len(content)+1)))
	if err != nil || !bytes.Equal(body, content) {
		return fmt.Errorf("template download byte verification failed")
	}
	return nil
}

func run() error {
	mode := flag.String("mode", "verify", "prepare|verify")
	path := flag.String("manifest", "", "absolute audit manifest path")
	stopped := flag.Bool("old-api-stopped", false, "API and Worker are stopped")
	flag.Parse()
	if flag.NArg() != 0 || (*mode != "prepare" && *mode != "verify") || !filepath.IsAbs(*path) || (*mode == "prepare" && !*stopped) {
		return fmt.Errorf("use -mode prepare|verify -manifest ABSOLUTE_PATH; prepare requires -old-api-stopped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	content, err := os.ReadFile("../docs/templates/mail-recipient-rule-import.csv")
	if err != nil || !bytes.Equal(content, []byte("\ufeff类型,邮箱/域名,动作,名称,备注,启用状态\r\n")) {
		return fmt.Errorf("fixed CSV template missing or changed")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("project environment unavailable")
	}
	cfg, err := config.LoadAPI(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("project configuration invalid")
	}
	databaseConnection, err := database.Open(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("PostgreSQL unavailable")
	}
	defer databaseConnection.Close()
	db := databaseConnection.GORM.Session(&gorm.Session{Logger: logger.Discard})
	redis, err := projectredis.Open(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("Redis unavailable")
	}
	defer redis.Close()
	keys, err := secretkey.New(cfg.AppSecret)
	if err != nil {
		return fmt.Errorf("storage key unavailable")
	}
	generations := cachegeneration.NewRepository(db)
	states := cachegeneration.NewStore(redis)
	settingRepo := systemsetting.NewRepository(db)
	settingRepo.SetGenerations(generations)
	settings := systemsetting.NewService(settingRepo)
	cache := systemsetting.NewCache(redis)
	cache.SetStateStore(states)
	settings.SetCache(cache)
	settings.SetGenerations(generations, states)
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	cosClient := storagecos.NewClient(client)
	cosRepo := cosconfig.NewRepository(db)
	cosRepo.SetGenerations(generations)
	cosService := cosconfig.NewService(cosRepo, keys, cosClient)
	cosCache := cosconfig.NewCache(redis)
	cosCache.SetStateStore(states)
	cosService.SetCache(cosCache)
	cosService.SetGenerations(generations, states)
	uploads := uploadrule.NewService(uploadrule.NewRepository(db), keys, cosClient, cosService, uploadrule.NewRouteCache(redis))
	var m manifest
	data, err := os.ReadFile(*path)
	manifestExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("manifest read failed")
	}
	if manifestExists {
		if json.Unmarshal(data, &m) != nil || m.SHA256 != hash || objectkey.Validate(m.ObjectKey) != nil {
			return fmt.Errorf("manifest invalid or template changed")
		}
	}
	page, err := uploads.PageInit(ctx)
	if err != nil {
		return fmt.Errorf("upload page facts unavailable")
	}
	var platformID int64
	for _, platform := range page.Platforms {
		if platform.Code == "admin" && platform.IsEnabled == yesno.Yes {
			platformID = platform.ID
		}
	}
	if platformID < 1 {
		return fmt.Errorf("enabled Admin platform unavailable")
	}
	identity := auth.Identity{PlatformID: platformID}
	if manifestExists {
		resolved, err := uploads.ObjectURL(ctx, identity, m.ObjectKey)
		if err != nil {
			return fmt.Errorf("manifest object route unavailable")
		}
		if err := validateManifest(m, hash, platformID, resolved); err != nil {
			return err
		}
	}
	if *mode == "verify" {
		if !manifestExists || m.PlatformID != platformID {
			return fmt.Errorf("verified manifest required")
		}
		consumer := recipientrule.NewService(nil)
		consumer.SetSettings(settings)
		configured, err := consumer.ImportTemplate(ctx)
		if err != nil || configured.ObjectKey != m.ObjectKey {
			return fmt.Errorf("template object key consumer verification failed")
		}
		resolved, err := uploads.ObjectURL(ctx, identity, m.ObjectKey)
		if err != nil || resolved.ExpiresAt != nil || resolved.URL != m.PublicURL {
			return fmt.Errorf("storage object URL verification failed")
		}
		if err := downloadMatches(ctx, client, resolved.URL, content); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	}
	// An already migrated setting is never replaced by preparing again.
	if current, err := settingRepo.Find(ctx, sharedsetting.MailRecipientRuleImportTemplateObjectKey); err == nil {
		if !manifestExists || current.Value != m.ObjectKey {
			return fmt.Errorf("existing target differs from manifest; refusing overwrite")
		}
		if err := downloadMatches(ctx, client, m.PublicURL, content); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	} else if !errors.Is(err, systemsetting.ErrNotFound) {
		return fmt.Errorf("target setting inspection failed")
	}
	source, err := settingRepo.Find(ctx, oldKey)
	if err != nil || source.ValueType != 1 || source.IsBuiltin != yesno.Yes || source.IsEnabled != yesno.Yes {
		return fmt.Errorf("source setting unavailable")
	}
	if source.Value != "" {
		if err := downloadMatches(ctx, client, source.Value, content); err != nil {
			return err
		}
	}
	if manifestExists && (m.SourceURL != source.Value || m.PlatformID != platformID) {
		return fmt.Errorf("source setting differs from manifest")
	}
	enabled := yesno.Yes
	rules, err := uploads.List(ctx, uploadrule.ListQuery{Page: 1, PageSize: 100, PlatformID: &platformID, IsEnabled: &enabled})
	if err != nil || rules.Total != 1 || len(rules.List) != 1 {
		return fmt.Errorf("exactly one enabled Admin upload rule is required")
	}
	rule := rules.List[0]
	if rule.AccessMode != "public" || !slices.Contains(rule.Codes, "file") {
		return fmt.Errorf("current Admin public file rule unavailable")
	}
	if manifestExists && m.RuleID != rule.ID {
		return fmt.Errorf("upload rule changed since preparation")
	}
	update, changed := csvInput(rule)
	if changed {
		if err := uploads.Update(ctx, rule.ID, update); err != nil {
			return fmt.Errorf("CSV upload rule extension failed")
		}
	}
	var signed storagecos.PutResult
	if !manifestExists {
		credentials, err := uploads.IssueCredentials(ctx, identity, uploadrule.CredentialInput{RuleCode: "file", Files: []uploadrule.FileInput{{FileName: "mail-recipient-rule-import.csv", ContentType: "text/csv", FileSizeBytes: int64(len(content))}}})
		if err != nil || len(credentials.Items) != 1 || credentials.Items[0].PublicURL == nil {
			return fmt.Errorf("standard CSV upload credential issuance failed")
		}
		item := credentials.Items[0]
		m = manifest{SourceURL: source.Value, ObjectKey: item.ObjectKey, PublicURL: *item.PublicURL, SHA256: hash, PlatformID: platformID, RuleID: rule.ID}
		resolved, err := uploads.ObjectURL(ctx, identity, m.ObjectKey)
		if err != nil {
			return fmt.Errorf("new template object route unavailable")
		}
		if err := validateManifest(m, hash, platformID, resolved); err != nil {
			return err
		}
		if err := writeManifest(*path, m); err != nil {
			return fmt.Errorf("manifest write failed; no COS PUT attempted")
		}
		signed = storagecos.PutResult{URL: item.UploadURL, Headers: item.Headers}
	} else {
		if err := downloadMatches(ctx, client, m.PublicURL, content); err == nil {
			return json.NewEncoder(os.Stdout).Encode(m)
		}
		coords, err := objectkey.Parse(m.ObjectKey)
		if err != nil || coords.PlatformID != platformID || coords.RuleID != rule.ID || coords.CosConfigID != rule.CosConfigID {
			return fmt.Errorf("manifest coordinates do not match upload rule")
		}
		runtime, err := cosService.Runtime(ctx, coords.CosConfigID)
		if err != nil || runtime.Deleted || runtime.IsEnabled != yesno.Yes {
			return fmt.Errorf("COS runtime unavailable")
		}
		var version *cosconfig.RuntimeVersion
		for index := range runtime.Versions {
			if runtime.Versions[index].Version == coords.Version {
				version = &runtime.Versions[index]
			}
		}
		if version == nil {
			return fmt.Errorf("manifest COS version unavailable")
		}
		id, secret, err := cosconfig.DecryptCredentials(keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
		if err != nil {
			return fmt.Errorf("COS credentials unavailable")
		}
		credentials := storagecos.Credentials{AppID: runtime.AppID, SecretID: id, SecretKey: secret, Bucket: version.Bucket, Region: version.Region}
		if version.Endpoint != nil {
			credentials.Endpoint = *version.Endpoint
		}
		signed, err = cosClient.PresignPut(ctx, credentials, storagecos.PutRequest{ObjectKey: m.ObjectKey, ContentType: "text/csv", ContentLength: int64(len(content)), PublicRead: true})
		if err != nil {
			return fmt.Errorf("resume upload signing failed")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed.URL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("COS PUT request invalid")
	}
	for name, value := range signed.Headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("x-cos-forbid-overwrite", "true")
	req.Header.Set("Content-Disposition", "attachment; filename=\"mail-recipient-rule-import.csv\"")
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("COS PUT failed; rerun with the saved manifest")
	}
	status := res.StatusCode
	res.Body.Close()
	if (status < 200 || status >= 300) && status != http.StatusConflict {
		return fmt.Errorf("COS PUT HTTP %d; rerun with the saved manifest", status)
	}
	resolved, err := uploads.ObjectURL(ctx, identity, m.ObjectKey)
	if err != nil || resolved.ExpiresAt != nil || resolved.URL != m.PublicURL {
		return fmt.Errorf("standard object resolver verification failed")
	}
	if err := downloadMatches(ctx, client, m.PublicURL, content); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(m)
}
