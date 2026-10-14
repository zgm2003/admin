-- Maintainer-only forward migration. Run the whole file in one transaction.
-- Removes the redundant administrator-facing payment configuration code.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(20261014, 102);
DO $migration$
DECLARE
  current_generation bigint;
  changed boolean := false;
BEGIN
  IF to_regclass('payment_config') IS NULL OR to_regclass('payment_config_version') IS NULL THEN
    RAISE EXCEPTION 'payment configuration schema is missing; run the base payment migration first';
  END IF;

  IF EXISTS (
    SELECT 1 FROM pg_attribute
    WHERE attrelid='payment_config'::regclass AND attname='code' AND NOT attisdropped
  ) THEN
    IF to_regclass('ux_payment_config_code_active') IS NOT NULL THEN
      DROP INDEX ux_payment_config_code_active;
    END IF;
    IF EXISTS (
      SELECT 1 FROM pg_constraint
      WHERE conrelid='payment_config'::regclass AND conname='ck_payment_config_code'
    ) THEN
      ALTER TABLE payment_config DROP CONSTRAINT ck_payment_config_code;
    END IF;
    ALTER TABLE payment_config DROP COLUMN code;
    changed := true;
  END IF;

  IF (SELECT count(*) FROM pg_attribute WHERE attrelid='payment_config'::regclass AND attnum>0 AND NOT attisdropped) <> 12
     OR (SELECT count(*) FROM pg_attribute WHERE attrelid='payment_config_version'::regclass AND attnum>0 AND NOT attisdropped) <> 10 THEN
    RAISE EXCEPTION 'payment configuration column contract differs after code removal';
  END IF;

  EXECUTE $fn$CREATE OR REPLACE FUNCTION payment_config_guard() RETURNS trigger LANGUAGE plpgsql AS $body$
  BEGIN
    IF TG_OP='DELETE' OR TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'payment configuration facts cannot be physically removed'; END IF;
    IF ROW(NEW.id,NEW.provider,NEW.environment,NEW.app_id,NEW.seller_id,NEW.created_at)
       IS DISTINCT FROM ROW(OLD.id,OLD.provider,OLD.environment,OLD.app_id,OLD.seller_id,OLD.created_at)
       OR NEW.current_version < OLD.current_version THEN RAISE EXCEPTION 'payment configuration identity is immutable'; END IF;
    RETURN NEW;
  END $body$$fn$;

  IF NOT EXISTS (SELECT 1 FROM system_config_cache_state WHERE namespace='payment.config' AND scope_key='global') THEN
    INSERT INTO system_config_cache_state(namespace,scope_key,generation,created_at,updated_at)
      VALUES('payment.config','global',1,now(),now());
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,created_at,updated_at)
      VALUES('payment.config','global',1,now(),now(),now());
  ELSIF changed THEN
    SELECT generation INTO current_generation
      FROM system_config_cache_state
      WHERE namespace='payment.config' AND scope_key='global'
      FOR UPDATE;
    IF current_generation < 1 THEN RAISE EXCEPTION 'payment cache generation is invalid'; END IF;
    UPDATE system_config_cache_state
      SET generation=current_generation+1, updated_at=now()
      WHERE namespace='payment.config' AND scope_key='global';
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,created_at,updated_at)
      VALUES('payment.config','global',current_generation+1,now(),now(),now());
  END IF;
END $migration$;
