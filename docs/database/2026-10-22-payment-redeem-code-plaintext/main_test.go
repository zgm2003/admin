package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"runtime"
	"time"
)

func encryptedFixture(t *testing.T, root, code string) (string, []byte) {
	t.Helper()
	key, err := hkdf.Key(sha256.New, []byte(root), nil, "admin:payment:redeem-code-encryption:v1", 32)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	raw := gcm.Seal(nonce, nonce, []byte(code), nil)
	key, err = hkdf.Key(sha256.New, []byte(root), nil, "admin:payment:redeem-code-hmac:v1", 32)
	if err != nil {
		t.Fatal(err)
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(code))
	return "payment-redeem:v1:" + base64.RawURLEncoding.EncodeToString(raw), h.Sum(nil)
}

func TestLegacyCodeConversionValidatesEveryFact(t *testing.T) {
	root := strings.Repeat("r", 64)
	code := "ABCDEFGHJKLMNPQRSTUV"
	ct, hash := encryptedFixture(t, root, code)
	got, err := legacyCode(root, ct, hash)
	if err != nil || got != code {
		t.Fatalf("historical code changed: %v", err)
	}
	badCode, badCodeHash := encryptedFixture(t, root, strings.ToLower(code))
	for _, test := range []struct {
		root, ct string
		hash     []byte
	}{
		{"", ct, hash}, {strings.Repeat("s", 64), ct, hash}, {root, "bad", hash},
		{root, ct, nil}, {root, ct, make([]byte, 32)}, {root, badCode, badCodeHash},
		{root, "payment-redeem:v1:AA", hash},
	} {
		if _, err := legacyCode(test.root, test.ct, test.hash); err == nil {
			t.Fatal("invalid encrypted fact accepted")
		}
	}
}

func TestMigrationConvertsHistoricalCodeAtomicallyAndReplays(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_, source, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	_ = godotenv.Load(filepath.Join(repo, "server", ".env"))
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN unavailable")
	}
	sql, err := os.ReadFile(filepath.Join(repo, "docs", "database", "2026-10-22-payment-redeem-code-plaintext.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"success", "wrong-key", "wrong-hash", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			conn, err := pgx.Connect(ctx, dsn)
			if err != nil {
				t.Fatal("connect test database")
			}
			defer func() { _ = conn.Close(ctx) }()
			schema := fmt.Sprintf("test_redeem_convert_%d", time.Now().UnixNano())
			quoted := pgx.Identifier{schema}.Sanitize()
			if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quoted+"; SET search_path TO "+quoted); err != nil {
				t.Fatal(err)
			}
			defer func() { _, _ = conn.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") }()

			if _, err := conn.Exec(ctx, `
CREATE TABLE payment_redeem_code_batch(id bigint PRIMARY KEY);
CREATE TABLE payment_transaction(id bigint PRIMARY KEY);
CREATE TABLE payment_wallet(user_id bigint PRIMARY KEY);
CREATE TABLE payment_redeem_code(id bigint PRIMARY KEY,batch_id bigint,code_hash bytea NOT NULL,code_ciphertext text NOT NULL,status smallint,created_at timestamptz,updated_at timestamptz);
CREATE FUNCTION payment_redeem_code_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable'; END $$;
CREATE TRIGGER payment_redeem_code_guard BEFORE UPDATE OR DELETE ON payment_redeem_code FOR EACH ROW EXECUTE FUNCTION payment_redeem_code_guard();
CREATE TRIGGER payment_redeem_code_truncate_guard BEFORE TRUNCATE ON payment_redeem_code FOR EACH STATEMENT EXECUTE FUNCTION payment_redeem_code_guard();
`); err != nil {
				t.Fatal(err)
			}
			root := strings.Repeat("r", 64)
			code := "ABCDEFGHJKLMNPQRSTUV"
			ct, hash := encryptedFixture(t, root, code)
			if scenario == "wrong-hash" {
				hash = make([]byte, 32)
			}
			if scenario != "empty" {
				if _, err := conn.Exec(ctx, `INSERT INTO payment_redeem_code VALUES(1,1,$1,$2,1,now(),now())`, hash, ct); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "wrong-key" {
				root = strings.Repeat("w", 64)
			}
			first, err := migrate(ctx, conn, schema, root, string(sql))
			if scenario == "wrong-key" || scenario == "wrong-hash" {
				if err == nil {
					t.Fatal("corrupt data migrated")
				}
				var oldColumn int
				if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_attribute WHERE attrelid='payment_redeem_code'::regclass AND attname='code_ciphertext' AND NOT attisdropped`).Scan(&oldColumn); err != nil || oldColumn != 1 {
					t.Fatal("failed migration changed schema")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "empty" {
				var stored string
				if err := conn.QueryRow(ctx, `SELECT code FROM payment_redeem_code WHERE id=1`).Scan(&stored); err != nil || stored != code || first.Converted != 1 {
					t.Fatal("code not preserved")
				}
			}
			second, err := migrate(ctx, conn, schema, "", string(sql))
			if err != nil || second.Converted != 0 || first.FactsDigest != second.FactsDigest || first.SchemaDigest != second.SchemaDigest {
				t.Fatalf("replay changed facts: %v", err)
			}
		})
	}
}
