package setting

import (
	"net/url"
	"strings"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const settingImageKey = "setting/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.png"

func TestAdvancedListIncludesMediaSettings(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	now := time.Now().UTC()
	for _, key := range []string{BrandTitleZhCNKey, BrandTitleEnUSKey, BrandDefaultAvatarKey, sharedsetting.MailRecipientRuleImportTemplateObjectKey, LegalUserAgreementKey, LegalPrivacyPolicyKey, "auth.captcha.ttl_minutes"} {
		row := Model{Key: key, Value: "value", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(db)
	rows, total, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20})
	if err != nil || total != 5 || len(rows) != 5 {
		t.Fatalf("advanced rows=%+v total=%d err=%v", rows, total, err)
	}
	for _, row := range rows {
		if row.Key == LegalUserAgreementKey || row.Key == LegalPrivacyPolicyKey {
			t.Fatalf("legal setting exposed: %s", row.Key)
		}
	}
	if _, err := parseListQuery(url.Values{"group": {"media"}}); err == nil {
		t.Fatal("removed media group accepted")
	}

}

func TestBrandFieldsUseStrictScalarAndImageContracts(t *testing.T) {
	for _, key := range []string{BrandTitleZhCNKey, BrandTitleEnUSKey} {
		for _, value := range []string{"", strings.Repeat("字", 129)} {
			if err := validateInput(key, value, ValueTypeString, ""); err == nil {
				t.Fatalf("invalid title accepted: %s", key)
			}
		}
		if err := validateInput(key, "true", ValueTypeBool, ""); err == nil {
			t.Fatal("non-string title accepted")
		}
		if !isRequiredSetting(key) {
			t.Fatal("brand title can be disabled")
		}
	}
	for _, value := range []string{"https://example.com/avatar.png", "avatar/legacy.png", strings.TrimSuffix(settingImageKey, ".png") + ".csv"} {
		if err := validateInput(BrandDefaultAvatarKey, value, 5, ""); err == nil {
			t.Fatalf("invalid avatar accepted: %q", value)
		}
	}
	for _, value := range []string{"", settingImageKey, strings.Replace(settingImageKey, "setting/", "avatar/", 1)} {
		if err := validateInput(BrandDefaultAvatarKey, value, 5, ""); err != nil {
			t.Fatalf("valid avatar rejected: %v", err)
		}
	}
}

func TestMediaIsAValueTypeForAnySettingKey(t *testing.T) {
	for _, value := range []string{"", settingImageKey, strings.TrimSuffix(settingImageKey, ".png") + ".csv"} {
		if err := validateInput("app.assets.custom", value, 5, ""); err != nil {
			t.Fatalf("media rejected: %v", err)
		}
	}
	for _, value := range []string{"https://example.com/file.csv", "setting/legacy.csv", "not a key"} {
		if err := validateInput("app.assets.custom", value, 5, ""); err == nil {
			t.Fatalf("bad media accepted: %q", value)
		}
	}
	if err := validateInput(BrandDefaultAvatarKey, settingImageKey, ValueTypeString, ""); err == nil {
		t.Fatal("builtin avatar accepted old string type")
	}
}

func TestMediaTypeSurvivesCacheRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	row := Record{ID: 1, Key: "app.assets.custom", Value: settingImageKey, ValueType: 5, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	encoded, err := encodeRecordPayload(1, row)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRecordPayload(encoded)
	if err != nil || decoded.ValueType != 5 || decoded.Value != settingImageKey {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	if _, err := decodeRecordPayload(strings.Replace(encoded, settingImageKey, "https://example.com/file.png", 1)); err == nil {
		t.Fatal("corrupt media value accepted from cache")
	}
	row.Value = "https://example.com/file.png"
	if _, err := encodeRecordPayload(1, row); err == nil {
		t.Fatal("invalid media source encoded")
	}
}

func TestBrandColdReadRejectsInvalidMediaBeforeCaching(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		BrandTitleZhCNKey:     {Key: BrandTitleZhCNKey, Value: "Title", ValueType: ValueTypeString, IsEnabled: yesno.Yes},
		BrandTitleEnUSKey:     {Key: BrandTitleEnUSKey, Value: "Title", ValueType: ValueTypeString, IsEnabled: yesno.Yes},
		BrandDefaultAvatarKey: {Key: BrandDefaultAvatarKey, Value: "https://example.com/avatar.png", ValueType: 5, IsEnabled: yesno.Yes},
	}}
	if _, err := harness.service(fake).Brand(harness.ctx); err == nil {
		t.Fatal("bad brand media source returned as success")
	}
}
