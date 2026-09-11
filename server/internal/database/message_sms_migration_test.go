package database_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type messageSMSFixture struct {
	adminID     int64
	canvasID    int64
	disabledID  int64
	messageDir  int64
	profilePage int64
	userID      int64
	roleID      int64
	grants      int64
}

func TestMessageSMSMigrationIsIdempotentAndSeedsFixedFacts(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms")
	fixture := createMessageSMSFixture(t, db, ctx)

	script := readMessageSMSMigration(t)
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(script).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	for _, table := range []string{
		"message_sms_config",
		"message_sms_template",
		"message_sms_recipient_rule",
		"message_sms_rate_limit_policy",
		"message_sms_log",
		"message_sms_log_verification",
		"user_phone_change_log",
		"user_email_change_log",
	} {
		assertTableExists(t, db, ctx, table)
	}

	var templates []string
	if err := db.WithContext(ctx).Raw(`SELECT scene || '|' || tencent_template_id || '|' || parameter_keys::text || '|' || is_enabled::text || '|' || example_variables::text
FROM message_sms_template ORDER BY scene`).Scan(&templates).Error; err != nil {
		t.Fatal(err)
	}
	wantTemplates := []string{
		`bind_phone||["code", "ttl_minutes"]|0|{"code": "123456", "ttl_minutes": "5"}`,
		`change_password||["code", "ttl_minutes"]|0|{"code": "123456", "ttl_minutes": "5"}`,
		`forget||["code", "ttl_minutes"]|0|{"code": "123456", "ttl_minutes": "5"}`,
		`login||["code", "ttl_minutes"]|0|{"code": "123456", "ttl_minutes": "5"}`,
	}
	if strings.Join(templates, "\n") != strings.Join(wantTemplates, "\n") {
		t.Fatalf("templates =\n%s\nwant\n%s", strings.Join(templates, "\n"), strings.Join(wantTemplates, "\n"))
	}

	var policies []string
	if err := db.WithContext(ctx).Raw(`SELECT platform_id || '|' || policy_key || '|' || mode || '|' || dimension || '|' || limit_count::text || '|' || window_seconds::text
FROM message_sms_rate_limit_policy ORDER BY platform_id, policy_key`).Scan(&policies).Error; err != nil {
		t.Fatal(err)
	}
	wantPolicies := []string{
		"1|business_phone_10m|business|platform_phone|5|600",
		"1|business_phone_minute|business|platform_phone|1|60",
		"2|business_phone_10m|business|platform_phone|5|600",
		"2|business_phone_minute|business|platform_phone|1|60",
	}
	if strings.Join(policies, "\n") != strings.Join(wantPolicies, "\n") {
		t.Fatalf("policies =\n%s\nwant\n%s", strings.Join(policies, "\n"), strings.Join(wantPolicies, "\n"))
	}

	var smsPage string
	if err := db.WithContext(ctx).Raw(`SELECT parent_id || '|' || coalesce(i18n_key,'-') || '|' || coalesce(path,'-') || '|' || coalesce(component_path,'-') || '|' || coalesce(icon,'-') || '|' || is_hidden::text
FROM permission_menu WHERE code='message:sms:view' AND platform_id=?`, fixture.adminID).Scan(&smsPage).Error; err != nil {
		t.Fatal(err)
	}
	wantSmsPage := strings.Join([]string{
		itoa(fixture.messageDir), "navigation.sms", "/message/sms", "message/sms", "lucide:message-square-more", "0",
	}, "|")
	if smsPage != wantSmsPage {
		t.Fatalf("sms page = %q, want %q", smsPage, wantSmsPage)
	}

	var actions []string
	if err := db.WithContext(ctx).Raw(`SELECT code FROM permission_menu
WHERE platform_id=? AND parent_id=(SELECT id FROM permission_menu WHERE code='message:sms:view' AND platform_id=?)
ORDER BY code`, fixture.adminID, fixture.adminID).Scan(&actions).Error; err != nil {
		t.Fatal(err)
	}
	wantActions := []string{
		"message:sms:config:delete",
		"message:sms:config:update",
		"message:sms:detail",
		"message:sms:list",
		"message:sms:rate-limit:update",
		"message:sms:rule:create",
		"message:sms:rule:delete",
		"message:sms:rule:status",
		"message:sms:rule:update",
		"message:sms:template:status",
		"message:sms:template:update",
		"message:sms:test",
	}
	if strings.Join(actions, ",") != strings.Join(wantActions, ",") {
		t.Fatalf("sms actions = %v", actions)
	}

	var actionShape string
	if err := db.WithContext(ctx).Raw(`SELECT coalesce(i18n_key,'null') || '|' || coalesce(path,'null') || '|' || coalesce(component_path,'null') || '|' || coalesce(icon,'null') || '|' || is_hidden::text
FROM permission_menu WHERE code='message:sms:config:update' AND platform_id=?`, fixture.adminID).Scan(&actionShape).Error; err != nil {
		t.Fatal(err)
	}
	if actionShape != "null|null|null|null|1" {
		t.Fatalf("sms action shape = %q", actionShape)
	}

	var phoneAction int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu
WHERE platform_id=? AND code='user:phone:update' AND parent_id=? AND is_hidden=1`, fixture.adminID, fixture.profilePage).Scan(&phoneAction).Error; err != nil {
		t.Fatal(err)
	}
	if phoneAction != 1 {
		t.Fatalf("user:phone:update rows = %d", phoneAction)
	}
	var emailAction int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu
WHERE platform_id=? AND code='user:email:update' AND parent_id=? AND is_hidden=1`, fixture.adminID, fixture.profilePage).Scan(&emailAction).Error; err != nil {
		t.Fatal(err)
	}
	if emailAction != 1 {
		t.Fatalf("user:email:update rows = %d", emailAction)
	}

	var canvasMenus int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=? AND code LIKE 'message:sms%'`, fixture.canvasID).Scan(&canvasMenus).Error; err != nil {
		t.Fatal(err)
	}
	if canvasMenus != 0 {
		t.Fatalf("canvas received %d sms menus without a message directory", canvasMenus)
	}

	var versions []string
	if err := db.WithContext(ctx).Raw(`SELECT id || '|' || menu_version FROM permission_auth_platform ORDER BY id`).Scan(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(versions, ",") != "1|6,2|1,3|1" {
		t.Fatalf("menu versions = %v", versions)
	}

	var grants int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_role_menu WHERE deleted_at IS NULL`).Scan(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != fixture.grants+2 {
		t.Fatalf("role grants = %d, want %d (original %d plus email/phone identity updates)", grants, fixture.grants+2, fixture.grants)
	}
	var grantedCodes []string
	if err := db.WithContext(ctx).Raw(`SELECT menu.code
FROM permission_role_menu AS grant_row
JOIN permission_role AS app_role ON app_role.id = grant_row.role_id
JOIN permission_menu AS menu ON menu.id = grant_row.menu_id
WHERE app_role.code = 'registered_user' AND grant_row.deleted_at IS NULL
ORDER BY menu.code`).Scan(&grantedCodes).Error; err != nil {
		t.Fatal(err)
	}
	wantGranted := []string{
		"storage:object:upload",
		"user:email:update",
		"user:password:update",
		"user:phone:update",
		"user:profile:detail",
		"user:profile:update",
		"user:profile:view",
	}
	if strings.Join(grantedCodes, ",") != strings.Join(wantGranted, ",") {
		t.Fatalf("registered_user grants = %v, want %v", grantedCodes, wantGranted)
	}
	var phoneGrant int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*)
FROM permission_role_menu AS grant_row
JOIN permission_role AS app_role ON app_role.id = grant_row.role_id
JOIN permission_menu AS menu ON menu.id = grant_row.menu_id
WHERE app_role.code = 'registered_user'
  AND menu.code = 'user:phone:update'
  AND grant_row.deleted_at IS NULL`).Scan(&phoneGrant).Error; err != nil {
		t.Fatal(err)
	}
	if phoneGrant != 1 {
		t.Fatalf("user:phone:update grants for registered_user = %d, want 1", phoneGrant)
	}
	var emailGrant int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*)
FROM permission_role_menu AS grant_row
JOIN permission_role AS app_role ON app_role.id = grant_row.role_id
JOIN permission_menu AS menu ON menu.id = grant_row.menu_id
WHERE app_role.code = 'registered_user'
  AND menu.code = 'user:email:update'
  AND grant_row.deleted_at IS NULL`).Scan(&emailGrant).Error; err != nil {
		t.Fatal(err)
	}
	if emailGrant != 1 {
		t.Fatalf("user:email:update grants for registered_user = %d, want 1", emailGrant)
	}

	var phone string
	if err := db.WithContext(ctx).Raw(`SELECT phone FROM user_account WHERE id=?`, fixture.userID).Scan(&phone).Error; err != nil {
		t.Fatal(err)
	}
	if phone != "+8613800000000" {
		t.Fatalf("phone = %q", phone)
	}

	var phoneCheck int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_constraint
WHERE conrelid='user_account'::regclass AND conname='ck_user_account_phone_e164'`).Scan(&phoneCheck).Error; err != nil {
		t.Fatal(err)
	}
	if phoneCheck != 1 {
		t.Fatal("ck_user_account_phone_e164 is missing")
	}

	var regions []string
	if err := db.WithContext(ctx).Raw(`SELECT i.value || '|' || i.label_zh || '|' || i.label_en || '|' || i.is_builtin::text || '|' || i.is_enabled::text
FROM system_dictionary_item i JOIN system_dictionary d ON d.id=i.dictionary_id
WHERE d.code='message.sms.region' AND i.deleted_at IS NULL ORDER BY i.sort`).Scan(&regions).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(regions, ",") != "ap-guangzhou|广州|Guangzhou|1|1" {
		t.Fatalf("sms regions = %v", regions)
	}
}

func TestMessageSMSMigrationRejectsInvalidLegacyPhoneAndRollsBack(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_invalid_phone")
	fixture := createMessageSMSFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`UPDATE user_account SET phone='12345' WHERE id=?`, fixture.userID).Error; err != nil {
		t.Fatal(err)
	}

	message := assertMigrationRejects(t, db, ctx, readMessageSMSMigration(t))

	var phone string
	if err := db.WithContext(ctx).Raw(`SELECT phone FROM user_account WHERE id=?`, fixture.userID).Scan(&phone).Error; err != nil {
		t.Fatal(err)
	}
	if phone != "12345" {
		t.Fatalf("phone mutated by a failed migration: %q", phone)
	}
	var tables int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'message_sms%'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("failed migration left %d sms tables", tables)
	}
	var version int64
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=?`, fixture.adminID).Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("menu_version changed by a failed migration: %d", version)
	}
	if !strings.Contains(strings.ToLower(message), "phone") {
		t.Fatalf("rejection message = %q", message)
	}
}

func TestMessageSMSMigrationRejectsNormalizedPhoneCollisionAndRollsBack(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_phone_collision")
	fixture := createMessageSMSFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account(username,email,phone,password_hash,is_enabled)
VALUES ('collision','collision@example.com','+8613800000000','',1)`).Error; err != nil {
		t.Fatal(err)
	}

	message := assertMigrationRejects(t, db, ctx, readMessageSMSMigration(t))

	var tables int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'message_sms%'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("failed migration left %d sms tables", tables)
	}
	var phones []string
	if err := db.WithContext(ctx).Raw(`SELECT phone FROM user_account ORDER BY id`).Scan(&phones).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(phones, ",") != "138 0000 0000,+8613800000000" {
		t.Fatalf("phones mutated by a failed migration: %v", phones)
	}
	if !strings.Contains(strings.ToLower(message), "phone") {
		t.Fatalf("rejection message = %q", message)
	}
	_ = fixture
}

func TestMessageSMSMigrationKeepsSoftDeletedPlatformLogsQueryable(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_soft_deleted_platform")
	fixture := createMessageSMSFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readMessageSMSMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status)
VALUES (?,'login',(SELECT id FROM message_sms_template WHERE scene='login'),'cipher','156****8271','hmac','sent')`, fixture.disabledID).Error; err != nil {
		t.Fatalf("insert log for a soft deleted platform: %v", err)
	}

	var row string
	if err := db.WithContext(ctx).Raw(`SELECT l.to_phone_hint || '|' || coalesce(p.code,'')
FROM message_sms_log l
LEFT JOIN permission_auth_platform p ON p.id = l.platform_id
WHERE l.platform_id = ?`, fixture.disabledID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row != "156****8271|retired" {
		t.Fatalf("soft deleted platform log row = %q", row)
	}
}

func TestMessageSMSMigrationRejectsDoubleCountryCodeAndRollsBack(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_double_country_code")
	fixture := createMessageSMSFixture(t, db, ctx)
	// +86 followed by 86<valid national number>: only ONE optional country code
	// may be stripped, exactly like shared/phone.Normalize.
	if err := db.WithContext(ctx).Exec(`UPDATE user_account SET phone='+868615671628271' WHERE id=?`, fixture.userID).Error; err != nil {
		t.Fatal(err)
	}

	message := assertMigrationRejects(t, db, ctx, readMessageSMSMigration(t))

	var tables int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'message_sms%'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("failed migration left %d sms tables", tables)
	}
	var phone string
	if err := db.WithContext(ctx).Raw(`SELECT phone FROM user_account WHERE id=?`, fixture.userID).Scan(&phone).Error; err != nil {
		t.Fatal(err)
	}
	if phone != "+868615671628271" {
		t.Fatalf("phone mutated by a failed migration: %q", phone)
	}
	if !strings.Contains(strings.ToLower(message), "phone") {
		t.Fatalf("rejection message = %q", message)
	}
}

func TestMessageSMSMigrationEnforcesSchemaFacts(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_schema_facts")
	fixture := createMessageSMSFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readMessageSMSMigration(t)).Error; err != nil {
		t.Fatal(err)
	}

	admin := fixture.adminID
	canvas := fixture.canvasID
	user := fixture.userID
	loginTemplate := `(SELECT id FROM message_sms_template WHERE scene='login')`
	configColumns := "(secret_id_ciphertext,secret_key_ciphertext,sms_sdk_app_id,sign_name,region,ttl_minutes)"

	for _, test := range []struct {
		name           string
		statement      string
		code           string
		constraintName string
	}{
		{"config rejects ttl below one", `INSERT INTO message_sms_config` + configColumns + ` VALUES ('a','b','app','sign','ap-guangzhou',0)`, "23514", "ck_message_sms_config_ttl_minutes"},
		{"config rejects ttl above sixty", `INSERT INTO message_sms_config` + configColumns + ` VALUES ('a','b','app','sign','ap-guangzhou',61)`, "23514", "ck_message_sms_config_ttl_minutes"},
		{"config rejects a non yes/no flag", `INSERT INTO message_sms_config(secret_id_ciphertext,secret_key_ciphertext,sms_sdk_app_id,sign_name,region,ttl_minutes,is_enabled) VALUES ('a','b','app','sign','ap-guangzhou',5,2)`, "23514", "ck_message_sms_config_is_enabled"},
		{"template rejects an unknown scene", `INSERT INTO message_sms_template(scene,name,parameter_keys,example_variables) VALUES ('test','x','["code","ttl_minutes"]'::jsonb,'{}'::jsonb)`, "23514", "ck_message_sms_template_scene"},
		{"template rejects reordered parameter keys", `UPDATE message_sms_template SET parameter_keys='["ttl_minutes","code"]'::jsonb WHERE scene='login'`, "23514", "ck_message_sms_template_parameter_keys"},
		{"template rejects a duplicate scene", `INSERT INTO message_sms_template(scene,name,parameter_keys,example_variables) VALUES ('forget','x','["code","ttl_minutes"]'::jsonb,'{}'::jsonb)`, "23505", "ux_message_sms_template_scene"},
		{"template rejects a non yes/no flag", `UPDATE message_sms_template SET is_enabled=2 WHERE scene='login'`, "23514", "ck_message_sms_template_is_enabled"},
		{"rule rejects an unknown scope", `INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name) VALUES ('email','c','h','hmac-scope','allow','n')`, "23514", "ck_message_sms_recipient_rule_scope"},
		{"rule rejects an unknown action", `INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name) VALUES ('phone','c','h','hmac-action','block','n')`, "23514", "ck_message_sms_recipient_rule_action"},
		{"rule rejects a non yes/no flag", `INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name,is_enabled) VALUES ('phone','c','h','hmac-flag','allow','n',2)`, "23514", "ck_message_sms_recipient_rule_is_enabled"},
		{"policy rejects an unknown platform", `INSERT INTO message_sms_rate_limit_policy(platform_id,policy_key,mode,dimension,limit_count,window_seconds) VALUES (999999,'business_phone_minute','business','platform_phone',1,60)`, "23503", "fk_message_sms_rate_limit_policy_platform"},
		{"policy rejects an unknown policy key", fmt.Sprintf(`INSERT INTO message_sms_rate_limit_policy(platform_id,policy_key,mode,dimension,limit_count,window_seconds) VALUES (%d,'business_phone_ip','business','platform_phone',1,60)`, admin), "23514", "ck_message_sms_rate_limit_policy_shape"},
		{"policy rejects a wrong dimension", fmt.Sprintf(`UPDATE message_sms_rate_limit_policy SET dimension='platform' WHERE platform_id=%d AND policy_key='business_phone_minute'`, admin), "23514", "ck_message_sms_rate_limit_policy_shape"},
		{"policy rejects a zero limit", fmt.Sprintf(`UPDATE message_sms_rate_limit_policy SET limit_count=0 WHERE platform_id=%d AND policy_key='business_phone_minute'`, admin), "23514", "ck_message_sms_rate_limit_policy_values"},
		{"policy rejects a window above one day", fmt.Sprintf(`UPDATE message_sms_rate_limit_policy SET window_seconds=86401 WHERE platform_id=%d AND policy_key='business_phone_minute'`, admin), "23514", "ck_message_sms_rate_limit_policy_values"},
		{"policy rejects a zero revision", fmt.Sprintf(`UPDATE message_sms_rate_limit_policy SET revision=0 WHERE platform_id=%d AND policy_key='business_phone_minute'`, admin), "23514", "ck_message_sms_rate_limit_policy_revision"},
		{"log rejects an unknown scene", fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status) VALUES (%d,'test',%s,'c','156****8271','hmac-scene','sent')`, admin, loginTemplate), "23514", "ck_message_sms_log_scene"},
		{"log rejects an unknown status", fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status) VALUES (%d,'login',%s,'c','156****8271','hmac-status','queued')`, admin, loginTemplate), "23514", "ck_message_sms_log_status"},
		{"log rejects a negative fee", fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status,fee) VALUES (%d,'login',%s,'c','156****8271','hmac-fee','sent',-1)`, admin, loginTemplate), "23514", "ck_message_sms_log_fee"},
		{"log rejects an unknown template", fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status) VALUES (%d,'login',999999,'c','156****8271','hmac-template','sent')`, admin), "23503", "fk_message_sms_log_template"},
		{"log rejects an unknown platform", fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status) VALUES (999999,'login',%s,'c','156****8271','hmac-platform','sent')`, loginTemplate), "23503", "fk_message_sms_log_platform"},
		{"phone change log rejects an unknown action", fmt.Sprintf(`INSERT INTO user_phone_change_log(user_id,platform_id,action) VALUES (%d,%d,'reset')`, user, admin), "23514", "ck_user_phone_change_log_action"},
		{"phone change log rejects an unknown user", fmt.Sprintf(`INSERT INTO user_phone_change_log(user_id,platform_id,action) VALUES (999999,%d,'bind')`, admin), "23503", "fk_user_phone_change_log_user"},
		{"phone change log rejects an unknown platform", fmt.Sprintf(`INSERT INTO user_phone_change_log(user_id,platform_id,action) VALUES (%d,999999,'bind')`, user), "23503", "fk_user_phone_change_log_platform"},
	} {
		assertRejectedByConstraint(t, db, ctx, test.name, test.statement, test.code, test.constraintName)
	}

	execSQL(t, db, ctx, "config accepts the first active row",
		`INSERT INTO message_sms_config`+configColumns+` VALUES ('a','b','app','sign','ap-guangzhou',5)`)
	assertRejectedByConstraint(t, db, ctx, "config rejects a second active row",
		`INSERT INTO message_sms_config`+configColumns+` VALUES ('c','d','app','sign','ap-guangzhou',5)`,
		"23505", "ux_message_sms_config_active_singleton")

	execSQL(t, db, ctx, "rule accepts the first active pattern",
		`INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name) VALUES ('phone','c','156****8271','hmac-active','deny','n')`)
	assertRejectedByConstraint(t, db, ctx, "rule rejects a duplicate active pattern",
		`INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name) VALUES ('phone','c','156****8271','hmac-active','deny','n')`,
		"23505", "ux_message_sms_recipient_rule_pattern_action_active")

	logID := insertReturningID(t, db, ctx, "log accepts a pending row",
		fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,challenge_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status)
VALUES (%d,'challenge-1','login',%s,'c','156****8271','hmac-log','pending') RETURNING id`, admin, loginTemplate))
	assertRejectedByConstraint(t, db, ctx, "log rejects a duplicate platform challenge",
		fmt.Sprintf(`INSERT INTO message_sms_log(platform_id,challenge_id,scene,template_id,to_phone_ciphertext,to_phone_hint,to_phone_hmac,status)
	VALUES (%d,'challenge-1','login',%s,'c','156****8271','hmac-log-duplicate','pending')`, admin, loginTemplate),
		"23505", "ux_message_sms_log_platform_challenge")

	assertRejectedByConstraint(t, db, ctx, "verification rejects a mismatched platform",
		fmt.Sprintf(`INSERT INTO message_sms_log_verification(platform_id,sms_log_id,key_version,code_ciphertext,expires_at)
	VALUES (%d,%d,'v1','cipher',CURRENT_TIMESTAMP)`, canvas, logID),
		"23503", "fk_message_sms_log_verification_log")
	execSQL(t, db, ctx, "verification accepts the matching platform",
		fmt.Sprintf(`INSERT INTO message_sms_log_verification(platform_id,sms_log_id,key_version,code_ciphertext,expires_at)
	VALUES (%d,%d,'v1','cipher',CURRENT_TIMESTAMP)`, admin, logID))
	assertRejectedByConstraint(t, db, ctx, "verification rejects a duplicate log row",
		fmt.Sprintf(`INSERT INTO message_sms_log_verification(platform_id,sms_log_id,key_version,code_ciphertext,expires_at)
	VALUES (%d,%d,'v1','cipher',CURRENT_TIMESTAMP)`, admin, logID),
		"23505", "ux_message_sms_log_verification_log")

	var indexes []string
	if err := db.WithContext(ctx).Raw(`SELECT indexname FROM pg_indexes
WHERE schemaname = current_schema()
  AND tablename IN ('message_sms_config','message_sms_template','message_sms_recipient_rule',
                    'message_sms_rate_limit_policy','message_sms_log','message_sms_log_verification',
                    'user_phone_change_log','user_email_change_log')
ORDER BY indexname`).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool, len(indexes))
	for _, name := range indexes {
		present[name] = true
	}
	for _, want := range []string{
		"ux_message_sms_config_active_singleton",
		"ux_message_sms_template_scene",
		"ux_message_sms_recipient_rule_pattern_action_active",
		"ux_message_sms_log_platform_challenge",
		"ux_message_sms_log_verification_log",
		"ix_message_sms_log_created_id_desc",
		"ix_message_sms_log_platform_id_desc",
		"ix_message_sms_log_scene_id_desc",
		"ix_message_sms_log_status_id_desc",
		"ix_message_sms_log_to_phone_hmac_id_desc",
		"ix_message_sms_rate_limit_policy_platform_revision",
		"ix_user_phone_change_log_user_id_desc",
		"ix_user_email_change_log_user_id_desc",
	} {
		if !present[want] {
			t.Fatalf("index %q is missing, have %v", want, indexes)
		}
	}
}

func assertRejectedByConstraint(
	t *testing.T,
	db *gorm.DB,
	ctx context.Context,
	name string,
	statement string,
	wantCode string,
	wantConstraint string,
) {
	t.Helper()
	err := db.WithContext(ctx).Exec(statement).Error
	if err == nil {
		t.Fatalf("%s: PostgreSQL accepted %s", name, statement)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: error type = %T, want *pgconn.PgError: %v", name, err, err)
	}
	if pgErr.Code != wantCode || pgErr.ConstraintName != wantConstraint {
		t.Fatalf("%s: SQLSTATE/constraint = %s/%q, want %s/%q: %v",
			name, pgErr.Code, pgErr.ConstraintName, wantCode, wantConstraint, err)
	}
}

func execSQL(t *testing.T, db *gorm.DB, ctx context.Context, name, statement string) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func insertReturningID(t *testing.T, db *gorm.DB, ctx context.Context, name, statement string) int64 {
	t.Helper()
	var id int64
	if err := db.WithContext(ctx).Raw(statement).Scan(&id).Error; err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if id == 0 {
		t.Fatalf("%s: no identifier returned", name)
	}
	return id
}

func assertMigrationRejects(t *testing.T, db *gorm.DB, ctx context.Context, script string) string {
	t.Helper()
	var message string
	if err := db.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		_, migrationErr := connection.Statement.ConnPool.ExecContext(ctx, script)
		if _, rollbackErr := connection.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
			return rollbackErr
		}
		if migrationErr != nil {
			message = migrationErr.Error()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if message == "" {
		t.Fatal("migration accepted an invalid fixture")
	}
	return message
}

func createMessageSMSFixture(t *testing.T, db *gorm.DB, ctx context.Context) messageSMSFixture {
	t.Helper()
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE user_account(
 id BIGSERIAL PRIMARY KEY,
 username VARCHAR(64) NOT NULL,
 email VARCHAR(254) NOT NULL,
 phone VARCHAR(32),
 password_hash VARCHAR(255) NOT NULL DEFAULT '',
 is_enabled SMALLINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_user_account_phone_active ON user_account(phone) WHERE (phone IS NOT NULL AND deleted_at IS NULL);
CREATE TABLE permission_auth_platform(
 id BIGSERIAL PRIMARY KEY,
 code VARCHAR(64) NOT NULL,
 policy_version BIGINT NOT NULL DEFAULT 1,
 menu_version BIGINT NOT NULL DEFAULT 1,
 is_enabled SMALLINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE TABLE permission_menu(
 id BIGSERIAL PRIMARY KEY,
 parent_id BIGINT,
 platform_id BIGINT NOT NULL,
 menu_type VARCHAR(16) NOT NULL,
 code VARCHAR(128) NOT NULL,
 i18n_key VARCHAR(128),
 path VARCHAR(255),
 component_path VARCHAR(255),
 icon VARCHAR(64),
 sort_order INTEGER NOT NULL DEFAULT 0,
 is_enabled SMALLINT NOT NULL DEFAULT 1,
 is_hidden SMALLINT NOT NULL DEFAULT 0,
 name VARCHAR(128) NOT NULL,
 remark VARCHAR(255),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ,
 CONSTRAINT ck_fixture_menu_shape CHECK (
   btrim(name) <> '' AND (
     (menu_type='directory' AND i18n_key IS NOT NULL AND path IS NULL AND component_path IS NULL) OR
     (menu_type='page' AND i18n_key IS NOT NULL AND path IS NOT NULL AND btrim(path) <> '' AND component_path IS NOT NULL AND btrim(component_path) <> '') OR
     (menu_type='action' AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden=1)
   )),
 UNIQUE (id, platform_id),
 CONSTRAINT fk_fixture_menu_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id),
 CONSTRAINT fk_fixture_menu_parent FOREIGN KEY (parent_id, platform_id) REFERENCES permission_menu(id, platform_id)
);
CREATE UNIQUE INDEX ux_fixture_menu_platform_code_active ON permission_menu(platform_id, code) WHERE deleted_at IS NULL;
CREATE TABLE permission_role(
 id BIGSERIAL PRIMARY KEY,
 code VARCHAR(64) NOT NULL,
 name VARCHAR(128) NOT NULL,
 is_default SMALLINT NOT NULL DEFAULT 0,
 is_enabled SMALLINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ,
 CONSTRAINT ck_fixture_role_is_default CHECK (is_default IN (0,1)),
 CONSTRAINT ck_fixture_role_is_enabled CHECK (is_enabled IN (0,1))
);
CREATE UNIQUE INDEX ux_fixture_role_code_active ON permission_role(code) WHERE deleted_at IS NULL;
CREATE TABLE permission_role_menu(
 id BIGSERIAL PRIMARY KEY,
 role_id BIGINT NOT NULL REFERENCES permission_role(id) ON DELETE RESTRICT,
 menu_id BIGINT NOT NULL REFERENCES permission_menu(id) ON DELETE RESTRICT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_fixture_role_menu_active ON permission_role_menu(role_id, menu_id) WHERE deleted_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
	createDictionarySeedFixture(t, db, ctx)

	var fixture messageSMSFixture
	var disabled int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_auth_platform(code,policy_version,menu_version,is_enabled,deleted_at)
VALUES ('admin',3,5,1,NULL) RETURNING id`).Scan(&fixture.adminID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_auth_platform(code,policy_version,menu_version,is_enabled,deleted_at)
VALUES ('canvas',1,1,1,NULL) RETURNING id`).Scan(&fixture.canvasID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_auth_platform(code,policy_version,menu_version,is_enabled,deleted_at)
VALUES ('retired',1,1,1,CURRENT_TIMESTAMP) RETURNING id`).Scan(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	fixture.disabledID = disabled

	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,code,i18n_key,path,component_path,icon,sort_order,is_hidden,name)
VALUES (?::bigint,NULL,'directory','message','navigation.message',NULL,NULL,'lucide:mail',80,0,'消息') RETURNING id`, fixture.adminID).Scan(&fixture.messageDir).Error; err != nil {
		t.Fatal(err)
	}
	var mailPage int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,code,i18n_key,path,component_path,icon,sort_order,is_hidden,name)
VALUES (?::bigint,?::bigint,'page','message:mail:view','navigation.mail','/message/mail','message/mail','lucide:mail',10,0,'邮件服务') RETURNING id`, fixture.adminID, fixture.messageDir).Scan(&mailPage).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,code,i18n_key,path,component_path,icon,sort_order,is_hidden,name,remark)
VALUES (?::bigint,NULL,'page','user:profile:view','layout.user.profile','/user/profile','user/profile','lucide:user-circle',90,1,'个人中心','hidden profile page') RETURNING id`, fixture.adminID).Scan(&fixture.profilePage).Error; err != nil {
		t.Fatal(err)
	}
	var canvasPage int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,code,i18n_key,path,component_path,icon,sort_order,is_hidden,name)
VALUES (?::bigint,NULL,'page','canvas:test:view','navigation.test','/test','test',NULL,10,0,'测试页') RETURNING id`, fixture.canvasID).Scan(&canvasPage).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_role(code,name,is_default,is_enabled)
VALUES ('registered_user','Registered User',0,1) RETURNING id`).Scan(&fixture.roleID).Error; err != nil {
		t.Fatal(err)
	}
	// The fixture keeps the same five direct grants the real registered_user seed holds.
	for _, code := range []string{"user:profile:detail", "user:profile:update", "user:password:update", "storage:object:upload"} {
		var actionID int64
		if err := db.WithContext(ctx).Raw(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,code,sort_order,is_hidden,name)
VALUES (?::bigint,?::bigint,'action',?::varchar,0,1,?::varchar) RETURNING id`, fixture.adminID, fixture.profilePage, code, code).Scan(&actionID).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.WithContext(ctx).Exec(`INSERT INTO permission_role_menu(role_id,menu_id) VALUES (?::bigint,?::bigint)`, fixture.roleID, actionID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_role_menu(role_id,menu_id) VALUES (?::bigint,?::bigint)`, fixture.roleID, fixture.profilePage).Error; err != nil {
		t.Fatal(err)
	}
	var grants int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_role_menu WHERE deleted_at IS NULL`).Scan(&grants).Error; err != nil {
		t.Fatal(err)
	}
	fixture.grants = grants

	if err := db.WithContext(ctx).Raw(`INSERT INTO user_account(username,email,phone,password_hash,is_enabled)
VALUES ('legacy','legacy@example.com','138 0000 0000','',1) RETURNING id`).Scan(&fixture.userID).Error; err != nil {
		t.Fatal(err)
	}
	return fixture
}

func readMessageSMSMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-10-message-sms.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
