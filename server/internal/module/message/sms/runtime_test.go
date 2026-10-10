package sms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/message/sms/config"
	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/shared/yesno"
)

type runtimeConfigSourceTest struct{}

func (runtimeConfigSourceTest) Credentials(context.Context) (config.Credentials, error) {
	return config.Credentials{
		SecretID: "plain-secret-id", SecretKey: "plain-secret-key",
		SDKAppID: "sdk-app", SignName: "sign", Region: "ap-guangzhou", TTLMinutes: 5,
	}, nil
}

func (runtimeConfigSourceTest) FindActive(context.Context) (config.Model, error) {
	return config.Model{
		ID: 1, SecretIDCiphertext: "sms:v1:cipher-id", SecretKeyCiphertext: "sms:v1:cipher-key",
		SDKAppID: "sdk-app", SignName: "sign", Region: "ap-guangzhou", TTLMinutes: 5, IsEnabled: yesno.Yes,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, nil
}

type runtimeTemplateSourceTest struct{}

func (runtimeTemplateSourceTest) List(context.Context) ([]template.Model, error) {
	rows := make([]template.Model, 0, len(template.FixedCatalog()))
	for index, fixed := range template.FixedCatalog() {
		rows = append(rows, template.Model{
			ID: int64(index + 1), Scene: fixed.Scene, Name: fixed.Name, TencentTemplateID: "123456",
			VariableKeys:     json.RawMessage(`["code","ttl_minutes"]`),
			ExampleVariables: json.RawMessage(`{"code":"123456","ttl_minutes":"5"}`),
			IsEnabled:        yesno.Yes, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		})
	}
	return rows, nil
}

type runtimeRuleSourceTest struct{}

func (runtimeRuleSourceTest) List(context.Context) ([]recipientRule.Model, error) { return nil, nil }

func validRuntimeSnapshotForTest() runtimeSnapshot {
	templates := make(map[string]templateRow, len(template.FixedCatalog()))
	for _, fixed := range template.FixedCatalog() {
		templates[fixed.Scene] = templateRow{
			Name:              fixed.Name,
			TencentTemplateID: "123456",
			VariableKeys:      append([]string(nil), fixed.VariableKeys...),
			ExampleVariables:  map[string]string{"code": "123456", "ttl_minutes": "5"},
			IsEnabled:         int16(yesno.Yes),
		}
	}
	return runtimeSnapshot{
		SchemaVersion:       runtimeCacheSchemaVersion,
		Generation:          7,
		Configured:          true,
		SecretIDCiphertext:  "ciphertext-id",
		SecretKeyCiphertext: "ciphertext-key",
		SDKAppID:            "sdk-app",
		SignName:            "sign",
		Region:              "ap-guangzhou",
		TTLMinutes:          5,
		IsEnabled:           int16(yesno.Yes),
		Templates:           templates,
		Rules: []ruleRow{{
			ID: 1, Scope: int16(recipientRule.ScopePhone), Action: int16(recipientRule.ActionDeny),
			Pattern: "+8615671628271", IsEnabled: int16(yesno.Yes),
		}},
	}
}

func TestDecodeRuntimeSnapshotRejectsInvalidSceneSet(t *testing.T) {
	snapshot := validRuntimeSnapshotForTest()
	delete(snapshot.Templates, template.SceneForget)
	snapshot.Templates["test"] = templateRow{
		Name: "test", TencentTemplateID: "123456",
		VariableKeys:     []string{"code", "ttl_minutes"},
		ExampleVariables: map[string]string{"code": "123456", "ttl_minutes": "5"},
		IsEnabled:        int16(yesno.Yes),
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRuntimeSnapshot(string(raw), 7); err == nil {
		t.Fatal("runtime snapshot accepted a missing fixed scene and an unknown test scene")
	}
}

func TestDecodeRuntimeSnapshotRejectsInvalidNestedFacts(t *testing.T) {
	for _, mutate := range []func(*runtimeSnapshot){
		func(value *runtimeSnapshot) { value.IsEnabled = 2 },
		func(value *runtimeSnapshot) { value.TTLMinutes = 0 },
		func(value *runtimeSnapshot) { value.Templates[template.SceneLogin] = templateRow{} },
		func(value *runtimeSnapshot) { value.Rules[0].Scope = 9 },
		func(value *runtimeSnapshot) { value.Rules[0].Pattern = "" },
	} {
		snapshot := validRuntimeSnapshotForTest()
		mutate(&snapshot)
		raw, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeRuntimeSnapshot(string(raw), 7); err == nil {
			t.Fatalf("runtime snapshot accepted invalid facts: %s", raw)
		}
	}
}

func TestRuntimeSnapshotCodecIsStrictAndPreservesPlaintextPattern(t *testing.T) {
	snapshot := validRuntimeSnapshotForTest()
	facts := factsOf(snapshot)
	raw, err := encodeRuntimeSnapshot(7, facts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, snapshot.SecretIDCiphertext) || !strings.Contains(raw, snapshot.SecretKeyCiphertext) ||
		strings.Contains(raw, "plain-secret") {
		t.Fatalf("runtime snapshot crossed the credential boundary: %s", raw)
	}
	decoded, err := decodeRuntimeSnapshot(raw, 7)
	if err != nil || decoded.Generation != 7 || len(decoded.Templates) != len(template.FixedCatalog()) || len(decoded.Rules) != 1 {
		t.Fatalf("decoded snapshot = %+v, err=%v", decoded, err)
	}

	for name, payload := range map[string]string{
		"unknown field":       strings.Replace(raw, `{`, `{"extra":true,`, 1),
		"duplicate field":     strings.Replace(raw, `"generation":7`, `"generation":7,"generation":8`, 1),
		"trailing value":      raw + `{}`,
		"wrong schema":        strings.Replace(raw, `"schemaVersion":2`, `"schemaVersion":1`, 1),
		"wrong generation":    strings.Replace(raw, `"generation":7`, `"generation":8`, 1),
		"missing fixed scene": strings.Replace(raw, `"forget":`, `"unknown":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRuntimeSnapshot(payload, 7); !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
				t.Fatalf("decode error = %v, want ErrRuntimeSnapshotCorrupt", err)
			}
		})
	}
}

func TestRuntimeLoaderCarriesCiphertextInsteadOfPlainCredentials(t *testing.T) {
	stores := Stores{Config: runtimeConfigSourceTest{}, Template: runtimeTemplateSourceTest{}, Rule: runtimeRuleSourceTest{}}
	facts, err := stores.runtimeLoader()(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "plain-secret-id") || strings.Contains(string(payload), "plain-secret-key") {
		t.Fatalf("runtime facts contain plaintext credentials: %s", payload)
	}
	if !strings.Contains(string(payload), "sms:v1:cipher-id") || !strings.Contains(string(payload), "sms:v1:cipher-key") {
		t.Fatalf("runtime facts omitted encrypted credentials: %s", payload)
	}
}

func TestRuntimePlaintextRulesRejectMissingNullAndLegacyProtocolValues(t *testing.T) {
	raw, err := encodeRuntimeSnapshot(7, factsOf(validRuntimeSnapshotForTest()))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "scope", "action", "pattern", "isEnabled"} {
		for _, mode := range []string{"missing", "null"} {
			t.Run(field+"-"+mode, func(t *testing.T) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal([]byte(raw), &fields); err != nil {
					t.Fatal(err)
				}
				var rules []map[string]json.RawMessage
				if err := json.Unmarshal(fields["rules"], &rules); err != nil {
					t.Fatal(err)
				}
				if mode == "missing" {
					delete(rules[0], field)
				} else {
					rules[0][field] = json.RawMessage("null")
				}
				fields["rules"], err = json.Marshal(rules)
				if err != nil {
					t.Fatal(err)
				}
				payload, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := decodeRuntimeSnapshot(string(payload), 7); !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
					t.Fatalf("missing/null %s accepted: %v", field, err)
				}
			})
		}
	}
	for name, payload := range map[string]string{
		"legacy string scope":     strings.Replace(raw, `"scope":0`, `"scope":"phone"`, 1),
		"legacy string action":    strings.Replace(raw, `"action":0`, `"action":"deny"`, 1),
		"legacy ciphertext field": strings.Replace(raw, `"pattern":"+8615671628271"`, `"pattern":"+8615671628271","patternCiphertext":"sms:v1:old"`, 1),
		"noncanonical phone":      strings.Replace(raw, `"pattern":"+8615671628271"`, `"pattern":"15671628271"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRuntimeSnapshot(payload, 7); !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
				t.Fatalf("bad rule accepted: %v", err)
			}
		})
	}
}

func TestRuntimeCacheValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewRuntimeCache(nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing SMS runtime dependencies")
	}
}
