package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var settingScope = cachegeneration.Scope{Namespace: "system.setting", ScopeKey: "global"}

func parseMode(args []string) (string, bool, error) {
	f := flag.NewFlagSet("mail-rule-xlsx-template-publish", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mode := f.String("mode", "inspect", "inspect|publish|verify")
	stopped := f.Bool("old-api-stopped", false, "API and Worker have been stopped")
	if f.Parse(args) != nil || f.NArg() != 0 || (*mode != "inspect" && *mode != "publish" && *mode != "verify") || (*mode == "publish") != *stopped {
		return "", false, errors.New("use -mode inspect|verify, or -mode publish -old-api-stopped after backup and shutdown")
	}
	return *mode, *stopped, nil
}
func main() {
	if err := runMain(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func runMain() error {
	mode, _, err := parseMode(os.Args[1:])
	if err != nil {
		return err
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return errors.New("server environment unavailable")
	}
	cfg, err := config.LoadAPI(os.LookupEnv)
	if err != nil {
		return errors.New("server configuration invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	connection, err := database.Open(ctx, cfg.PostgresDSN)
	if err != nil {
		return errors.New("PostgreSQL unavailable")
	}
	defer connection.Close()
	db := connection.GORM.Session(&gorm.Session{Logger: logger.Discard})
	redis, err := projectredis.Open(ctx, cfg.RedisURL)
	if err != nil {
		return errors.New("Redis unavailable")
	}
	defer redis.Close()
	keys, err := secretkey.New(cfg.AppSecret)
	if err != nil {
		return errors.New("storage key unavailable")
	}
	generations := cachegeneration.NewRepository(db)
	states := cachegeneration.NewStore(redis)
	settingRepo := systemsetting.NewRepository(db)
	settingRepo.SetGenerations(generations)
	settings := systemsetting.NewService(settingRepo)
	settingCache := systemsetting.NewCache(redis)
	settingCache.SetStateStore(states)
	settings.SetCache(settingCache)
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
	inspect := func(ctx context.Context) (facts, error) { return inspectFacts(ctx, db, settingRepo, uploads) }
	if mode == "inspect" {
		f, err := inspect(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(f)
	}
	root := os.Getenv("LOCALAPPDATA")
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("LOCALAPPDATA is required")
	}
	path := filepath.Join(root, "Admin", "maintenance", "mail-rule-xlsx-template.json")
	content, err := os.ReadFile("../docs/templates/" + templateName)
	if err != nil || len(content) == 0 || len(content) > recipientrule.XlsxMaxBytes {
		return errors.New("local XLSX template unavailable or oversized")
	}
	// The real importer verifies the blank template; no repository is reached for an empty sheet.
	preview, err := recipientrule.NewService(nil).PreviewXlsx(ctx, recipientrule.XlsxImportInput{FileName: templateName, ContentBase64: base64.StdEncoding.EncodeToString(content)})
	if err != nil || len(preview.Rows) != 0 || !slices.Equal(preview.Errors, []string{"empty"}) {
		return errors.New("local XLSX template is not a valid blank import template")
	}
	ready := func(ctx context.Context, generation int64) error {
		state, found, err := states.Read(ctx, settingScope)
		if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != generation {
			return errors.New("setting Redis generation is not ready")
		}
		return nil
	}
	ops := publicationIO{
		inspect: inspect, client: client,
		extend: func(ctx context.Context, rule uploadrule.RuleValue) error {
			in, _ := xlsxInput(rule)
			if uploads.Update(ctx, rule.ID, in) != nil {
				return errors.New("setting upload rule extension failed")
			}
			return nil
		},
		issue: func(ctx context.Context, f facts, size int64) (uploadrule.CredentialItem, error) {
			res, err := uploads.IssueCredentials(ctx, auth.Identity{PlatformID: f.Rule.PlatformID}, uploadrule.CredentialInput{RuleCode: "setting", Files: []uploadrule.FileInput{{FileName: templateName, ContentType: xlsxMIME, FileSizeBytes: size}}})
			if err != nil || len(res.Items) != 1 || res.Items[0].PublicURL == nil || res.Items[0].Method != http.MethodPut {
				return uploadrule.CredentialItem{}, errors.New("standard setting upload credential issuance failed")
			}
			return res.Items[0], nil
		},
		resolve: func(ctx context.Context, platform int64, key string) (uploadrule.ObjectURLResult, error) {
			return uploads.ObjectURL(ctx, auth.Identity{PlatformID: platform}, key)
		},
		resume: func(ctx context.Context, m manifest) (storagecos.PutResult, error) {
			return resumeSignature(ctx, m, cosService, keys, cosClient)
		},
		bind: func(ctx context.Context, m manifest) error {
			return settings.Update(ctx, sharedsetting.MailRecipientRuleImportTemplateObjectKey, systemsetting.UpdateInput{Value: m.NewObjectKey, ValueType: systemsetting.ValueTypeMedia, Description: m.NewDescription})
		},
		sync: func(ctx context.Context, generation int64) error {
			state, found, err := states.Read(ctx, settingScope)
			if err != nil || (found && (state.State != cachegeneration.StateReady || state.Generation > generation)) {
				return errors.New("setting Redis state unavailable or another mutation is active; retry after it finishes")
			}
			result, err := states.Reconcile(ctx, settingScope, generation)
			if err != nil || result == cachegeneration.PublishSkippedInvalidating {
				return errors.New("PostgreSQL committed but Redis publication incomplete; rerun with manifest")
			}
			if err := ready(ctx, generation); err != nil {
				return err
			}
			var ids []int64
			if err := db.WithContext(ctx).Raw("SELECT id FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ? AND generation = ? AND published_at IS NULL", settingScope.Namespace, settingScope.ScopeKey, generation).Scan(&ids).Error; err != nil {
				return errors.New("setting outbox inspection failed")
			}
			for _, id := range ids {
				ok, err := generations.MarkPublishedIfUnclaimed(ctx, id, time.Now().UTC())
				if err != nil || !ok {
					return errors.New("setting outbox publication incomplete")
				}
			}
			return nil
		}, verifyReady: ready,
	}
	if mode == "publish" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return errors.New("cannot create template manifest directory")
		}
		m, err := publishTemplate(ctx, content, path, ops)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	}
	m, exists, err := readManifest(path)
	if err != nil || !exists {
		return errors.New("verified template manifest is required")
	}
	f, err := inspect(ctx)
	if err != nil {
		return err
	}
	if err := validateManifest(m, content, f); err != nil {
		return err
	}
	if f.Setting.Value != m.NewObjectKey {
		return errors.New("template setting is not bound")
	}
	if err := ready(ctx, settingGeneration(f)); err != nil {
		return err
	}
	consumer := recipientrule.NewService(nil)
	consumer.SetSettings(settings)
	template, err := consumer.ImportTemplate(ctx)
	if err != nil || template.ObjectKey != m.NewObjectKey {
		return errors.New("template service does not return the published object key")
	}
	resolved, err := ops.resolve(ctx, m.PlatformID, m.NewObjectKey)
	if err != nil || resolved.ExpiresAt != nil {
		return errors.New("published template resolver failed")
	}
	if err := downloadMatches(ctx, client, resolved.URL, content); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(m)
}

func inspectFacts(ctx context.Context, db *gorm.DB, settings *systemsetting.Repository, uploads *uploadrule.Service) (facts, error) {
	var f facts
	var err error
	f.Setting, err = settings.Find(ctx, sharedsetting.MailRecipientRuleImportTemplateObjectKey)
	if err != nil {
		return f, errors.New("template setting not found")
	}
	var ids []int64
	query := `SELECT r.id FROM storage_upload_rule r JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.deleted_at IS NULL JOIN permission_auth_platform p ON p.id=r.platform_id WHERE c.code='setting' AND r.deleted_at IS NULL AND r.is_enabled=1 AND p.code='admin' AND p.is_enabled=1 AND p.deleted_at IS NULL`
	if err := db.WithContext(ctx).Raw(query).Scan(&ids).Error; err != nil || len(ids) != 1 {
		return f, errors.New("exactly one enabled Admin setting upload rule is required")
	}
	f.Rule, err = uploads.Get(ctx, ids[0])
	if err != nil {
		return f, errors.New("setting upload rule unavailable")
	}
	if err := db.WithContext(ctx).Raw("SELECT namespace,scope_key,generation FROM system_config_cache_generation ORDER BY namespace,scope_key").Scan(&f.Generations).Error; err != nil || settingGeneration(f) < 1 {
		return f, errors.New("configuration generations unavailable")
	}
	control := `SELECT md5(jsonb_build_object(
 'otherGenerations',(SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key),'[]'::jsonb) FROM system_config_cache_generation g WHERE NOT(namespace='system.setting' AND scope_key='global')),
 'platforms',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'menuVersion',menu_version) ORDER BY id),'[]'::jsonb) FROM permission_auth_platform),
 'mailRules',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id),'[]'::jsonb) FROM message_mail_recipient_rule r))::text)`
	if err := db.WithContext(ctx).Raw(control).Scan(&f.ControlHash).Error; err != nil {
		return f, errors.New("control audit facts unavailable")
	}
	return f, nil
}

func resumeSignature(ctx context.Context, m manifest, service *cosconfig.Service, keys *secretkey.KeyRing, client *storagecos.Client) (storagecos.PutResult, error) {
	runtime, err := service.Runtime(ctx, m.CosConfigID)
	if err != nil || runtime.Deleted || runtime.IsEnabled != yesno.Yes {
		return storagecos.PutResult{}, errors.New("COS runtime unavailable")
	}
	var version *cosconfig.RuntimeVersion
	for i := range runtime.Versions {
		if runtime.Versions[i].Version == m.PhysicalVersion {
			version = &runtime.Versions[i]
		}
	}
	if version == nil {
		return storagecos.PutResult{}, errors.New("manifest COS physical version unavailable")
	}
	id, secret, err := cosconfig.DecryptCredentials(keys.StorageEncryptionKey(), runtime.SecretIDCiphertext, runtime.SecretKeyCiphertext)
	if err != nil {
		return storagecos.PutResult{}, errors.New("COS credentials unavailable")
	}
	credentials := storagecos.Credentials{AppID: runtime.AppID, SecretID: id, SecretKey: secret, Bucket: version.Bucket, Region: version.Region}
	if version.Endpoint != nil {
		credentials.Endpoint = *version.Endpoint
	}
	signed, err := client.PresignPut(ctx, credentials, storagecos.PutRequest{ObjectKey: m.NewObjectKey, ContentType: xlsxMIME, ContentLength: m.SizeBytes, PublicRead: true})
	if err != nil {
		return storagecos.PutResult{}, errors.New("resume COS signing failed")
	}
	return signed, nil
}
