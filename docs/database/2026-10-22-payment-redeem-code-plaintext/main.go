// Maintainer-only forward migration. Not imported by API or Worker.
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var codePattern = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{20}$`)
var schemaPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// These legacy purposes exist only in this offline conversion command.
func legacyCode(root, ciphertext string, hash []byte) (string, error) {
	invalid := errors.New("legacy code validation failed; no data committed")
	if len(root) < 64 || len(hash) != 32 || !strings.HasPrefix(ciphertext, "payment-redeem:v1:") {
		return "", invalid
	}
	key, err := hkdf.Key(sha256.New, []byte(root), nil, "admin:payment:redeem-code-encryption:v1", 32)
	if err != nil {
		return "", invalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", invalid
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, "payment-redeem:v1:"))
	if err != nil || len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return "", invalid
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil || !codePattern.Match(plain) {
		return "", invalid
	}
	hmacKey, err := hkdf.Key(sha256.New, []byte(root), nil, "admin:payment:redeem-code-hmac:v1", 32)
	if err != nil {
		return "", invalid
	}
	h := hmac.New(sha256.New, hmacKey)
	h.Write(plain)
	if !hmac.Equal(h.Sum(nil), hash) {
		return "", invalid
	}
	return string(plain), nil
}

func facts(ctx context.Context, tx pgx.Tx) (string, error) {
	var digest string
	err := tx.QueryRow(ctx, `SELECT md5(jsonb_build_object(
 'code',(SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5((to_jsonb(c)-'code_hash'-'code_ciphertext'-'code')::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_redeem_code c),
 'batch',(SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(b)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_redeem_code_batch b),
 'transaction',(SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(t)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_transaction t),
 'wallet',(SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(w)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_wallet w)
)::text)`).Scan(&digest)
	return digest, err
}

type result struct {
	Rows         int64  `json:"rows"`
	Converted    int64  `json:"converted"`
	FactsDigest  string `json:"factsDigest"`
	SchemaDigest string `json:"schemaDigest"`
}

func migrate(ctx context.Context, conn *pgx.Conn, schema, root, sql string) (result, error) {
	var out result
	if !schemaPattern.MatchString(schema) {
		return out, errors.New("invalid schema")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return out, errors.New("begin migration failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO `+pgx.Identifier{schema}.Sanitize()+`,pg_catalog; SET LOCAL lock_timeout='10s'; SET LOCAL statement_timeout='120s'; SELECT pg_advisory_xact_lock(20261022,1); LOCK TABLE payment_redeem_code IN ACCESS EXCLUSIVE MODE;`); err != nil {
		return out, errors.New("migration lock failed")
	}
	before, err := facts(ctx, tx)
	if err != nil {
		return out, errors.New("read financial facts failed")
	}
	var legacy bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='payment_redeem_code'::regclass AND attname='code_ciphertext' AND NOT attisdropped)`).Scan(&legacy); err != nil {
		return out, errors.New("read legacy schema failed")
	}
	if legacy {
		if _, err = tx.Exec(ctx, `CREATE TEMP TABLE payment_redeem_code_plaintext(id bigint PRIMARY KEY,code text NOT NULL) ON COMMIT DROP`); err != nil {
			return out, errors.New("create conversion map failed")
		}
		var lastID int64
		for {
			rows, e := tx.Query(ctx, `SELECT id,code_hash,code_ciphertext FROM payment_redeem_code WHERE id>$1 ORDER BY id LIMIT 250`, lastID)
			if e != nil {
				return out, errors.New("read legacy codes failed")
			}
			converted := make([][]any, 0, 250)
			for rows.Next() {
				var id int64
				var hash []byte
				var ciphertext string
				if e = rows.Scan(&id, &hash, &ciphertext); e != nil {
					rows.Close()
					return out, errors.New("read legacy code failed")
				}
				code, e := legacyCode(root, ciphertext, hash)
				if e != nil {
					rows.Close()
					return out, e
				}
				converted = append(converted, []any{id, code})
				lastID = id
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, errors.New("read legacy codes interrupted")
			}
			if len(converted) == 0 {
				break
			}
			if _, err = tx.CopyFrom(ctx, pgx.Identifier{"pg_temp", "payment_redeem_code_plaintext"}, []string{"id", "code"}, pgx.CopyFromRows(converted)); err != nil {
				return out, errors.New("write conversion map failed")
			}
			out.Converted += int64(len(converted))
		}
	}
	if _, err = tx.Exec(ctx, sql); err != nil {
		return out, errors.New("plaintext schema migration failed; transaction rolled back")
	}
	if legacy {
		var matched int64
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM payment_redeem_code c JOIN pg_temp.payment_redeem_code_plaintext p ON c.id=p.id AND c.code=p.code`).Scan(&matched); err != nil || matched != out.Converted {
			return out, errors.New("converted code preservation check failed")
		}
	}
	out.FactsDigest, err = facts(ctx, tx)
	if err != nil || before != out.FactsDigest {
		return out, errors.New("financial facts changed; transaction rolled back")
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM payment_redeem_code`).Scan(&out.Rows); err != nil {
		return out, errors.New("row count verification failed")
	}
	if err = tx.QueryRow(ctx, `SELECT md5(jsonb_build_object(
'columns',(SELECT jsonb_agg(jsonb_build_object('name',attname,'type',atttypid,'mod',atttypmod,'required',attnotnull) ORDER BY attnum) FROM pg_attribute WHERE attrelid='payment_redeem_code'::regclass AND attnum>0 AND NOT attisdropped),
'constraints',(SELECT jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname) FROM pg_constraint WHERE conrelid='payment_redeem_code'::regclass),
'indexes',(SELECT jsonb_agg(indexdef ORDER BY indexname) FROM pg_indexes WHERE schemaname=current_schema() AND tablename='payment_redeem_code'),
'guard',pg_get_functiondef('payment_redeem_code_guard()'::regprocedure)
)::text)`).Scan(&out.SchemaDigest); err != nil {
		return out, errors.New("schema verification failed")
	}
	if err = tx.Commit(ctx); err != nil {
		return out, errors.New("commit result unknown; keep services stopped and inspect database")
	}
	return out, nil
}

func main() {
	path := flag.String("sql", "../docs/database/2026-10-22-payment-redeem-code-plaintext.sql", "forward SQL path")
	schema := flag.String("schema", "public", "target schema")
	flag.Parse()
	_ = godotenv.Load(".env")
	sql, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration SQL unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("POSTGRES_DSN"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "PostgreSQL connection failed")
		os.Exit(1)
	}
	defer func() { _ = conn.Close(ctx) }()
	output, err := migrate(ctx, conn, *schema, os.Getenv("APP_SECRET"), string(sql))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, "committed; result output failed")
		os.Exit(1)
	}
}
