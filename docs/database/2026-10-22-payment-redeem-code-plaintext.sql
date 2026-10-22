-- 由同名 PowerShell runner 备份并调用一次性 Go 命令执行。
-- 非空旧表必须先在同一事务内填充 pg_temp.payment_redeem_code_plaintext；
-- Go 命令逐条验证旧密文、HMAC 和规范格式，不生成新码、不丢弃历史事实。
-- 必须在事务内执行；API/Worker 不得加载本文件。
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '120s';
SELECT pg_advisory_xact_lock(20261022, 1);
LOCK TABLE payment_redeem_code IN ACCESS EXCLUSIVE MODE;

DO $migration$
DECLARE
    legacy_columns integer;
BEGIN
    SELECT count(*) INTO legacy_columns FROM pg_attribute
    WHERE attrelid='payment_redeem_code'::regclass AND NOT attisdropped
      AND attname IN ('code_hash','code_ciphertext');
    IF legacy_columns NOT IN (0,2) THEN
        RAISE EXCEPTION 'incomplete legacy redeem code schema';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='payment_redeem_code'::regclass
      AND tgname='payment_redeem_code_guard' AND tgenabled='O')
      OR NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='payment_redeem_code'::regclass
      AND tgname='payment_redeem_code_truncate_guard' AND tgenabled='O') THEN
        RAISE EXCEPTION 'redeem code guards missing or disabled';
    END IF;
    IF legacy_columns=2 THEN
        IF EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='payment_redeem_code'::regclass
          AND attname='code' AND NOT attisdropped) THEN
            RAISE EXCEPTION 'mixed plaintext and encrypted schema';
        END IF;
        IF to_regclass('pg_temp.payment_redeem_code_plaintext') IS NULL THEN
            CREATE TEMP TABLE payment_redeem_code_plaintext(id bigint PRIMARY KEY,code text NOT NULL) ON COMMIT DROP;
        END IF;
        IF EXISTS (SELECT 1 FROM payment_redeem_code c FULL JOIN pg_temp.payment_redeem_code_plaintext p USING(id)
          WHERE c.id IS NULL OR p.id IS NULL OR p.code !~ '^[A-HJ-NP-Z2-9]{20}$') THEN
            RAISE EXCEPTION 'complete validated plaintext conversion required';
        END IF;
        ALTER TABLE payment_redeem_code ADD COLUMN code varchar(20);
        -- Backfill only; no timestamps, status, identity or transaction links are changed.
        ALTER TABLE payment_redeem_code DISABLE TRIGGER payment_redeem_code_guard;
        UPDATE payment_redeem_code c SET code=p.code FROM pg_temp.payment_redeem_code_plaintext p WHERE c.id=p.id;
        ALTER TABLE payment_redeem_code ENABLE TRIGGER payment_redeem_code_guard;
        ALTER TABLE payment_redeem_code ALTER COLUMN code SET NOT NULL;
        ALTER TABLE payment_redeem_code ADD CONSTRAINT ck_payment_redeem_code_format CHECK(code ~ '^[A-HJ-NP-Z2-9]{20}$');
        CREATE UNIQUE INDEX ux_payment_redeem_code_code ON payment_redeem_code(code);
        -- No CASCADE: unexpected dependent objects must abort the transaction.
        ALTER TABLE payment_redeem_code DROP COLUMN code_hash, DROP COLUMN code_ciphertext;
    END IF;
END $migration$;

CREATE OR REPLACE FUNCTION payment_redeem_code_guard() RETURNS trigger LANGUAGE plpgsql AS $guard$
BEGIN
    IF TG_OP IN ('DELETE','TRUNCATE') THEN
        RAISE EXCEPTION 'payment redeem codes cannot be physically removed';
    END IF;
    IF TG_OP='UPDATE' AND (
      ROW(NEW.id,NEW.batch_id,NEW.code,NEW.created_at)
        IS DISTINCT FROM ROW(OLD.id,OLD.batch_id,OLD.code,OLD.created_at)
      OR OLD.status<>1 OR NEW.status NOT IN (2,3)) THEN
        RAISE EXCEPTION 'payment redeem code identity or terminal state is immutable';
    END IF;
    RETURN NEW;
END $guard$;

DO $verify$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='payment_redeem_code'::regclass
      AND attname='code' AND NOT attisdropped AND attnotnull AND atttypid='varchar'::regtype AND atttypmod=24)
      OR NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_redeem_code'::regclass
      AND conname='ck_payment_redeem_code_format' AND contype='c' AND convalidated)
      OR NOT EXISTS (SELECT 1 FROM pg_index i JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=i.indkey[0]
      WHERE i.indexrelid=to_regclass('ux_payment_redeem_code_code') AND i.indrelid='payment_redeem_code'::regclass
      AND i.indisunique AND i.indisvalid AND i.indnkeyatts=1 AND i.indnatts=1 AND i.indpred IS NULL AND a.attname='code') THEN
        RAISE EXCEPTION 'plaintext redeem code schema validation failed';
    END IF;
END $verify$;
