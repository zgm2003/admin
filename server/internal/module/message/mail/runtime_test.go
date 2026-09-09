package mail

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestRuntimeSnapshotCarriesEncryptedMailCredentials(t *testing.T) {
	now := time.Now().UTC()
	snapshot := runtimeSnapshot{
		Generation: 3,
		Config: runtimeConfig{
			ID:                  1,
			SecretIDCiphertext:  "mail:v1:secret-id",
			SecretKeyCiphertext: "mail:v1:secret-key",
			Region:              "ap-guangzhou",
			FromEmail:           "sender@example.com",
			FromName:            "Sender",
			TTLMinutes:          5,
			IsEnabled:           yesno.Yes,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		Templates: []Template{{ID: 1, Scene: SceneLogin, TencentTemplateID: 47941, IsEnabled: yesno.Yes}},
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), snapshot.Config.SecretIDCiphertext) || !strings.Contains(string(raw), snapshot.Config.SecretKeyCiphertext) {
		t.Fatalf("runtime snapshot omitted encrypted credentials: %s", raw)
	}
	var decoded runtimeSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeSnapshot(decoded, SceneLogin); err != nil {
		t.Fatal(err)
	}
	if decoded.Config.SecretIDCiphertext != snapshot.Config.SecretIDCiphertext || decoded.Config.SecretKeyCiphertext != snapshot.Config.SecretKeyCiphertext {
		t.Fatalf("credentials did not round-trip: %+v", decoded.Config)
	}
}

func TestRuntimeSnapshotRejectsMissingCredentials(t *testing.T) {
	snapshot := runtimeSnapshot{
		Generation: 1,
		Config:     runtimeConfig{ID: 1, Region: "ap-guangzhou", FromEmail: "sender@example.com", FromName: "Sender", TTLMinutes: 5, IsEnabled: yesno.Yes, UpdatedAt: time.Now().UTC()},
		Templates:  []Template{{ID: 1, Scene: SceneLogin, TencentTemplateID: 47941, IsEnabled: yesno.Yes}},
	}
	if err := validateRuntimeSnapshot(snapshot, SceneLogin); err == nil {
		t.Fatal("runtime snapshot accepted missing encrypted credentials")
	}
}
