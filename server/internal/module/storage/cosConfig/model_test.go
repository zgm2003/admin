package cosconfig

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestModelAndSafeValueContracts(t *testing.T) {
	if (Model{}).TableName() != "storage_cos_config" {
		t.Fatalf("table name = %q", (Model{}).TableName())
	}
	if (Version{}).TableName() != "storage_cos_config_version" {
		t.Fatalf("version table name = %q", (Version{}).TableName())
	}
	endpoint := "https://cos.example.com"
	domain := "https://cdn.example.com"
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)
	row := Current{
		Model:        Model{ID: 7, Name: "Main", AppID: "1250000000", SecretIDCiphertext: "v1:secret-id", SecretKeyCiphertext: "v1:secret-key", CurrentVersion: 3, IsEnabled: yesno.Yes, Remark: "primary", CreatedAt: now, UpdatedAt: now},
		Bucket:       "assets",
		Region:       "ap-guangzhou",
		Endpoint:     &endpoint,
		BucketDomain: &domain,
	}
	value := safeValue(row)
	*value.Endpoint = "changed"
	*value.BucketDomain = "changed"
	if *row.Endpoint != endpoint || *row.BucketDomain != domain {
		t.Fatal("safe value shares mutable pointer fields with database row")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"secretId", "secretKey", "ciphertext", "v1:secret-id", "v1:secret-key", "deletedAt", "currentVersion", "generation", "version"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("safe response leaks %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"hasCredentials":true`) {
		t.Fatalf("safe response = %s", text)
	}
}
