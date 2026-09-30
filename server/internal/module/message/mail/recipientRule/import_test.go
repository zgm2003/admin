package recipientrule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const importHeader = "类型,邮箱/域名,动作,名称,备注,启用状态\n"

func TestCSVParserBoundsAndRowValidation(t *testing.T) {
	for _, tc := range []struct {
		content, code string
		row           bool
	}{
		{"", "empty", false}, {importHeader, "empty", false},
		{importHeader + string([]byte{0xff}), "invalid_encoding", false},
		{"scope,pattern,action,name,remark,isEnabled\n", "invalid_header", false},
		{"类型\x00邮箱/域名,动作,名称,备注,启用状态\n", "invalid_header", false},
		{importHeader + "email,\"bad", "invalid_csv", false},
		{strings.Repeat("a", CSVMaxBytes+1), "too_large", false},
		{importHeader + strings.Repeat("email,a@example.com,deny,name,,1\n", CSVMaxRows+1), "too_many_rows", false},
		{importHeader + "email,a@example.com,deny,name,1\n", "invalid_columns", true},
		{importHeader + "prefix,qq.com,deny,name,,1\n", "invalid_scope", true},
		{importHeader + "domain,@qq.com,deny,name,,1\n", "invalid_pattern", true},
		{importHeader + "email,a@example.com,drop,name,,1\n", "invalid_action", true},
		{importHeader + "email,a@example.com,deny, ,,1\n", "invalid_name", true},
		{importHeader + "email,a@example.com,deny," + strings.Repeat("名", 129) + ",,1\n", "invalid_name", true},
		{importHeader + "email,a@example.com,deny,name," + strings.Repeat("注", 513) + ",1\n", "invalid_remark", true},
		{importHeader + "email,a@example.com,deny,name,,true\n", "invalid_status", true},
		{importHeader + "email,a@example.com,deny,name,\x00,1\n", "invalid_character", true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			p := parseCSV(tc.content)
			codes := p.Errors
			if tc.row {
				if len(p.Rows) != 1 {
					t.Fatalf("preview=%+v", p)
				}
				codes = p.Rows[0].Errors
			}
			found := false
			for _, code := range codes {
				found = found || code == tc.code
			}
			if !found {
				t.Fatalf("preview=%+v want=%s", p, tc.code)
			}
		})
	}
}

func TestCSVImportIsAtomicAndRechecksExistingRules(t *testing.T) {
	db, ctx := openServiceDatabase(t)
	s := NewService(configuredRepository(db))
	generation := int64(1)
	s.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		result, err := change(ctx, generation)
		if err == nil && result.Changed {
			generation = result.Generation
		}
		return err
	}))
	content := importHeader + "email,A@example.com,deny,one,,1\ndomain,EXAMPLE.org,allow,two,,0\n"
	p, err := s.PreviewCSV(ctx, content)
	if err != nil || len(p.Rows) != 2 || len(p.Rows[0].Errors) != 0 || p.Rows[0].Values[1] != "a@example.com" {
		t.Fatalf("preview=%+v err=%v", p, err)
	}
	result, err := s.ImportCSV(ctx, content)
	if err != nil || result.Imported != 2 || generation != 2 {
		t.Fatalf("result=%+v generation=%d err=%v", result, generation, err)
	}
	var outbox int64
	if err := db.Table("system_config_cache_outbox").Count(&outbox).Error; err != nil || outbox != 1 {
		t.Fatalf("outbox=%d err=%v", outbox, err)
	}
	_, err = s.ImportCSV(ctx, content+"email,b@example.com,deny,new,,1\n")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict {
		t.Fatalf("err=%v", err)
	}
	var count int64
	if err := db.Model(&Model{}).Count(&count).Error; err != nil || count != 2 || generation != 2 {
		t.Fatalf("count=%d generation=%d err=%v", count, generation, err)
	}
	if err := db.Exec("ALTER TABLE message_mail_recipient_rule ADD CONSTRAINT test_reject CHECK (name <> 'reject')").Error; err != nil {
		t.Fatal(err)
	}
	_, err = s.ImportCSV(ctx, importHeader+"email,c@example.com,deny,valid,,1\nemail,d@example.com,deny,reject,,1\n")
	if err == nil {
		t.Fatal("constraint failure accepted")
	}
	if err := db.Model(&Model{}).Count(&count).Error; err != nil || count != 2 || generation != 2 {
		t.Fatalf("count=%d generation=%d err=%v", count, generation, err)
	}
}

func TestCSVImportRefusesFileDuplicatesInvalidRowsAndRuntimeFailure(t *testing.T) {
	db, ctx := openServiceDatabase(t)
	s := NewService(configuredRepository(db))
	s.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(context.Context, func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		return errors.New("redis failure")
	}))
	for _, content := range []string{importHeader + "email,a@example.com,deny,valid,,1\n", importHeader + "domain,@qq.com,deny,invalid,,1\n", importHeader + "email,A@example.com,deny,one,,1\nemail,a@example.com,deny,two,,1\n"} {
		if _, err := s.ImportCSV(ctx, content); err == nil {
			t.Fatal("import unexpectedly succeeded")
		}
	}
	var count int64
	if err := db.Model(&Model{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestCSVFormulaEscapingRoundTripsText(t *testing.T) {
	for _, value := range []string{"=1+1", "  @formula", "+note", "-note", "'literal", "''quoted", "\tformula", "\rformula", "\nformula", "备注,\"quote\"\nline", "safe@example.com"} {
		if got := unescapeCSVCell(escapeCSVCell(value)); got != value {
			t.Fatalf("got=%q want=%q", got, value)
		}
	}
}

func TestCSVImportRollsBackEarlierBatchesWhenALaterBatchFails(t *testing.T) {
	db, ctx := openServiceDatabase(t)
	if err := db.Exec("ALTER TABLE message_mail_recipient_rule ADD CONSTRAINT test_reject_late CHECK (name <> 'reject')").Error; err != nil {
		t.Fatal(err)
	}
	s := NewService(configuredRepository(db))
	s.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		_, err := change(ctx, 1)
		return err
	}))
	var csv strings.Builder
	csv.WriteString(importHeader)
	for index := 0; index < 201; index++ {
		name := "valid"
		if index == 200 {
			name = "reject"
		}
		fmt.Fprintf(&csv, "email,u%d@example.com,deny,%s,,1\n", index, name)
	}
	if _, err := s.ImportCSV(ctx, csv.String()); err == nil {
		t.Fatal("late constraint failure accepted")
	}
	var valid bool
	if err := db.Raw(`SELECT (SELECT count(*)=0 FROM message_mail_recipient_rule) AND (SELECT count(*)=0 FROM system_config_cache_outbox) AND (SELECT generation=1 FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global')`).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("rollback valid=%v err=%v", valid, err)
	}
}

func TestCSVImportMapsConcurrentDuplicateAndDoesNotWriteOtherRows(t *testing.T) {
	db, ctx := openServiceDatabase(t)
	s := NewService(configuredRepository(db))
	s.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		// Another writer wins after preview/recheck, before our transaction.
		now := time.Now().UTC()
		row := Model{Scope: ScopeEmail, Pattern: "race@example.com", Action: ActionDeny, Name: "competitor", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		_, err := change(ctx, 1)
		return err
	}))
	_, err := s.ImportCSV(ctx, importHeader+"email,new@example.com,deny,new,,1\nemail,race@example.com,deny,race,,1\n")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict {
		t.Fatalf("err=%v", err)
	}
	var count int64
	if err := db.Model(&Model{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestCSVBatchFailedGenerationRestoresIDsBeforeCoordinatorRetry(t *testing.T) {
	db, ctx := openServiceDatabase(t)
	repository := configuredRepository(db)
	now := time.Now().UTC()
	rows := []Model{{Scope: ScopeEmail, Pattern: "retry@example.com", Action: ActionDeny, Name: "retry", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}
	if _, err := repository.CreateBatch(ctx, rows, 2, now); err == nil {
		t.Fatal("stale expected generation accepted")
	}
	if rows[0].ID != 0 {
		t.Fatalf("rolled back generated ID=%d will be reused on retry", rows[0].ID)
	}
	if result, err := repository.CreateBatch(ctx, rows, 1, now); err != nil || !result.Changed || result.Generation != 2 {
		t.Fatalf("retry result=%+v err=%v", result, err)
	}
}

type templateReaderFunc func(context.Context, string) (sharedsetting.Record, error)

func (f templateReaderFunc) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	return f(ctx, key)
}

func TestCSVTemplateReadsObjectKeyAndFailsClosed(t *testing.T) {
	s := NewService(nil)
	const objectKey = "file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv"
	for _, tc := range []struct {
		value     string
		enabled   yesno.Value
		valueType int
		wantError bool
	}{
		{"", yesno.Yes, 1, false}, {objectKey, yesno.Yes, 1, false},
		{"https://example.com/template.csv", yesno.Yes, 1, true},
		{"file/template.csv", yesno.Yes, 1, true},
		{"javascript:alert(1)", yesno.Yes, 1, true}, {"https://user:password@example.com/t.csv", yesno.Yes, 1, true},
		{"https://example.com/t.csv", yesno.No, 1, true}, {"https://example.com/t.csv", yesno.Yes, 2, true},
	} {
		s.SetSettings(templateReaderFunc(func(ctx context.Context, key string) (sharedsetting.Record, error) {
			if key != "message.mail.recipient_rule.import_template_object_key" {
				t.Fatalf("key=%s", key)
			}
			return sharedsetting.Record{Value: tc.value, ValueType: tc.valueType, IsEnabled: tc.enabled}, nil
		}))
		value, err := s.ImportTemplate(context.Background())
		encoded, _ := json.Marshal(value)
		want, _ := json.Marshal(map[string]string{"objectKey": tc.value})
		if (err != nil) != tc.wantError || (!tc.wantError && string(encoded) != string(want)) {
			t.Fatalf("value=%+v err=%v", value, err)
		}
	}
	s.SetSettings(templateReaderFunc(func(context.Context, string) (sharedsetting.Record, error) {
		return sharedsetting.Record{}, errors.New("setting unavailable")
	}))
	if _, err := s.ImportTemplate(context.Background()); err == nil {
		t.Fatal("reader failure swallowed")
	}
}
