-- Maintainer-only forward migration. Run the whole file in one transaction.
-- Existing IDs, application identities and credential versions are preserved.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(20261013, 101);
DO $migration$
DECLARE
  has_code boolean;
  current_generation bigint;
  item record;
BEGIN
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('payment_config') AND relkind='r')
     OR NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('payment_config_version') AND relkind='r') THEN
    RAISE EXCEPTION 'payment configuration schema is missing; apply the base migration first';
  END IF;
  LOCK TABLE payment_config, payment_config_version IN ACCESS EXCLUSIVE MODE;
  SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='payment_config'::regclass AND attname='code' AND attnum>0 AND NOT attisdropped) INTO has_code;
    -- Reject column drift instead of treating IF NOT EXISTS as a schema validator.
    IF (SELECT count(*) FROM pg_attribute WHERE attrelid='payment_config'::regclass AND attnum>0 AND NOT attisdropped)<>(CASE WHEN has_code THEN 13 ELSE 12 END)
       OR (SELECT count(*) FROM pg_attribute WHERE attrelid='payment_config_version'::regclass AND attnum>0 AND NOT attisdropped)<>10 THEN
      RAISE EXCEPTION 'payment configuration column contract differs';
    END IF;
    -- Build expected CHECK definitions with this PostgreSQL parser, not textual guesses.
    CREATE TEMP TABLE payment_expected_check(provider smallint,environment smallint,is_enabled smallint,current_version bigint,pay_methods smallint[],code varchar(64),name varchar(128),app_id varchar(64),seller_id varchar(64),version bigint,private_key_ciphertext text,app_cert_pem text,alipay_cert_pem text,root_cert_pem text,notify_url varchar(2048),config_id bigint) ON COMMIT DROP;
    ALTER TABLE payment_expected_check
      ADD CONSTRAINT ck_payment_config_provider CHECK(provider IN(1,2)),
      ADD CONSTRAINT ck_payment_config_environment CHECK(environment IN(1,2)),
      ADD CONSTRAINT ck_payment_config_is_enabled CHECK(is_enabled IN(0,1)),
      ADD CONSTRAINT ck_payment_config_current_version CHECK(current_version>0),
      ADD CONSTRAINT ck_payment_config_name CHECK(btrim(name)<>''),
      ADD CONSTRAINT ck_payment_config_app_id CHECK(app_id ~ '^[0-9]{1,64}$'),
      ADD CONSTRAINT ck_payment_config_seller_id CHECK(seller_id ~ '^[0-9]{1,64}$'),
      ADD CONSTRAINT ck_payment_config_version_version CHECK(version>0),
      ADD CONSTRAINT ck_payment_config_version_key CHECK(octet_length(private_key_ciphertext) BETWEEN 1 AND 32768),
      ADD CONSTRAINT ck_payment_config_version_app_cert CHECK(octet_length(app_cert_pem) BETWEEN 1 AND 65536),
      ADD CONSTRAINT ck_payment_config_version_alipay_cert CHECK(octet_length(alipay_cert_pem) BETWEEN 1 AND 65536),
      ADD CONSTRAINT ck_payment_config_version_root_cert CHECK(octet_length(root_cert_pem) BETWEEN 1 AND 65536),
      ADD CONSTRAINT ck_payment_config_version_notify_url CHECK(notify_url LIKE 'https://%' AND notify_url LIKE '%/api/v1/payment/callback/alipay/' || config_id::text || '/' || version::text AND position('?' IN notify_url)=0 AND position('#' IN notify_url)=0),
      ADD CONSTRAINT ck_payment_config_version_pay_methods CHECK(pay_methods IN(ARRAY[1]::smallint[],ARRAY[2]::smallint[],ARRAY[1,2]::smallint[]));
    IF has_code THEN
      ALTER TABLE payment_expected_check ADD CONSTRAINT ck_payment_config_code CHECK(code ~ '^[a-z][a-z0-9_]{0,63}$');
    END IF;
    FOR item IN SELECT c.conname,pg_get_constraintdef(c.oid) AS definition FROM pg_constraint c WHERE c.conrelid='pg_temp.payment_expected_check'::regclass LOOP
      IF NOT EXISTS(SELECT 1 FROM pg_constraint c WHERE c.conname=item.conname AND c.conrelid=CASE WHEN item.conname LIKE 'ck_payment_config_version_%' THEN 'payment_config_version'::regclass ELSE 'payment_config'::regclass END AND c.contype='c' AND c.convalidated AND pg_get_constraintdef(c.oid)=item.definition) THEN RAISE EXCEPTION 'payment CHECK contract differs: %',item.conname; END IF;
    END LOOP;
    FOR item IN SELECT * FROM (VALUES
      ('payment_config','id','bigint',true),('payment_config','code','character varying(64)',true),
      ('payment_config','name','character varying(128)',true),('payment_config','provider','smallint',true),
      ('payment_config','environment','smallint',true),('payment_config','app_id','character varying(64)',true),
      ('payment_config','seller_id','character varying(64)',true),('payment_config','current_version','bigint',true),
      ('payment_config','sort_order','integer',true),('payment_config','is_enabled','smallint',true),
      ('payment_config','created_at','timestamp with time zone',true),('payment_config','updated_at','timestamp with time zone',true),('payment_config','deleted_at','timestamp with time zone',false),
      ('payment_config_version','config_id','bigint',true),('payment_config_version','version','bigint',true),
      ('payment_config_version','private_key_ciphertext','text',true),('payment_config_version','app_cert_pem','text',true),
      ('payment_config_version','alipay_cert_pem','text',true),('payment_config_version','root_cert_pem','text',true),
      ('payment_config_version','notify_url','character varying(2048)',true),('payment_config_version','pay_methods','smallint[]',true),
      ('payment_config_version','created_at','timestamp with time zone',true),('payment_config_version','updated_at','timestamp with time zone',true)
    ) AS expected(table_name,column_name,data_type,not_null) LOOP
      IF item.column_name='code' AND NOT has_code THEN CONTINUE; END IF;
      IF NOT EXISTS(SELECT 1 FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=item.table_name::regclass AND a.attname=item.column_name AND format_type(a.atttypid,a.atttypmod)=item.data_type AND a.attnotnull=item.not_null AND NOT a.attisdropped AND a.attgenerated='' AND a.attidentity=CASE WHEN item.table_name='payment_config' AND item.column_name='id' THEN 'd' ELSE '' END
        AND COALESCE(pg_get_expr(d.adbin,d.adrelid),'')=CASE WHEN item.column_name IN('created_at','updated_at') THEN 'CURRENT_TIMESTAMP' WHEN item.table_name='payment_config' AND item.column_name IN('sort_order','is_enabled') THEN '0' ELSE '' END) THEN RAISE EXCEPTION 'payment column contract differs: %.%',item.table_name,item.column_name; END IF;
    END LOOP;
    IF pg_get_serial_sequence('payment_config','id') IS NULL OR NOT EXISTS(SELECT 1 FROM pg_depend d JOIN pg_attribute a ON a.attrelid=d.refobjid AND a.attnum=d.refobjsubid JOIN pg_class seq ON seq.oid=d.objid WHERE d.classid='pg_class'::regclass AND d.refclassid='pg_class'::regclass AND d.refobjid='payment_config'::regclass AND a.attname='id' AND d.deptype='i' AND seq.relkind='S' AND seq.oid=pg_get_serial_sequence('payment_config','id')::regclass) THEN RAISE EXCEPTION 'payment identity sequence contract differs'; END IF;
    FOR item IN SELECT * FROM (VALUES
      ('payment_config','payment_config_pkey','p',ARRAY['id']::text[],NULL::text,NULL::text[],false),
      ('payment_config_version','pk_payment_config_version','p',ARRAY['config_id','version']::text[],NULL::text,NULL::text[],false),
      ('payment_config_version','fk_payment_config_version_config','f',ARRAY['config_id']::text[],'payment_config',ARRAY['id']::text[],false),
      ('payment_config','fk_payment_config_current_version','f',ARRAY['id','current_version']::text[],'payment_config_version',ARRAY['config_id','version']::text[],true)
    ) AS expected(table_name,constraint_name,kind,columns,ref_table,ref_columns,deferred) LOOP
      IF NOT EXISTS(SELECT 1 FROM pg_constraint c WHERE c.conrelid=item.table_name::regclass AND c.conname=item.constraint_name AND c.contype::text=item.kind AND c.convalidated AND c.condeferrable=item.deferred AND c.condeferred=item.deferred
        AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num ORDER BY k.ord)=item.columns
        AND (item.kind='p' OR (c.confrelid=item.ref_table::regclass AND c.confupdtype='a' AND c.confdeltype='r' AND c.confmatchtype='s'
          AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num ORDER BY k.ord)=item.ref_columns))) THEN RAISE EXCEPTION 'payment PK/FK contract differs: %',item.constraint_name; END IF;
    END LOOP;
    FOR item IN SELECT * FROM (VALUES('ux_payment_config_code_active',ARRAY['code']::text[]),('ux_payment_config_app_active',ARRAY['provider','environment','app_id']::text[])) AS expected(index_name,columns) LOOP
      IF item.index_name='ux_payment_config_code_active' AND NOT has_code THEN CONTINUE; END IF;
      IF NOT EXISTS(SELECT 1 FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid JOIN pg_am am ON am.oid=idx.relam WHERE i.indexrelid=to_regclass(item.index_name) AND i.indrelid='payment_config'::regclass AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive AND i.indimmediate AND NOT i.indnullsnotdistinct AND am.amname='btree' AND i.indexprs IS NULL AND i.indnkeyatts=i.indnatts AND pg_get_expr(i.indpred,i.indrelid)='(deleted_at IS NULL)'
        AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.num ORDER BY k.ord)=item.columns) THEN RAISE EXCEPTION 'payment unique index contract differs: %',item.index_name; END IF;
    END LOOP;
    FOR item IN SELECT * FROM (VALUES('payment_config','payment_config_identity_guard','payment_config_guard',27),('payment_config','payment_config_truncate_guard','payment_config_guard',34),('payment_config_version','payment_config_version_immutable','payment_config_version_guard',58)) AS expected(table_name,trigger_name,function_name,trigger_type) LOOP
      IF NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=item.table_name::regclass AND t.tgname=item.trigger_name AND NOT t.tgisinternal AND t.tgenabled='O' AND t.tgtype=item.trigger_type AND t.tgfoid=to_regprocedure(item.function_name || '()') AND t.tgqual IS NULL AND t.tgnargs=0 AND t.tgattr=''::int2vector) THEN RAISE EXCEPTION 'payment trigger contract differs: %',item.trigger_name; END IF;
    END LOOP;
    FOR item IN SELECT * FROM (VALUES
      ('payment_config_guard',$body$BEGIN
        IF TG_OP='DELETE' OR TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'payment configuration facts cannot be physically removed'; END IF;
        IF ROW(NEW.id,NEW.code,NEW.provider,NEW.environment,NEW.app_id,NEW.seller_id,NEW.created_at)
           IS DISTINCT FROM ROW(OLD.id,OLD.code,OLD.provider,OLD.environment,OLD.app_id,OLD.seller_id,OLD.created_at)
           OR NEW.current_version < OLD.current_version THEN RAISE EXCEPTION 'payment configuration identity is immutable'; END IF;
        RETURN NEW;
      END$body$),
      ('payment_config_version_guard',$body$BEGIN RAISE EXCEPTION 'payment credential versions are immutable'; END$body$)
    ) AS expected(function_name,body) LOOP
      IF NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang WHERE p.oid=to_regprocedure(item.function_name || '()') AND l.lanname='plpgsql' AND p.prorettype='trigger'::regtype AND NOT p.prosecdef AND p.proconfig IS NULL AND btrim(regexp_replace(p.prosrc,'\s+',' ','g'))=btrim(regexp_replace(CASE WHEN has_code THEN item.body ELSE replace(replace(item.body,'NEW.code,',''),'OLD.code,','') END,'\s+',' ','g'))) THEN RAISE EXCEPTION 'payment guard function contract differs: %',item.function_name; END IF;
    END LOOP;

  SELECT generation INTO STRICT current_generation FROM system_config_cache_state
    WHERE namespace='payment.config' AND scope_key='global' FOR UPDATE;
  IF current_generation < 1 OR (has_code AND current_generation = 9223372036854775807) THEN
    RAISE EXCEPTION 'payment cache generation is invalid or exhausted';
  END IF;
  IF has_code THEN
    IF current_setting('payment.config_verify_only',true)='on' THEN
      RAISE EXCEPTION 'payment code still exists; verification cannot upgrade the schema';
    END IF;
    DROP INDEX ux_payment_config_code_active;
    ALTER TABLE payment_config DROP CONSTRAINT ck_payment_config_code;
    ALTER TABLE payment_config DROP COLUMN code;
    EXECUTE $fn$CREATE OR REPLACE FUNCTION payment_config_guard() RETURNS trigger LANGUAGE plpgsql AS $body$
    BEGIN
      IF TG_OP='DELETE' OR TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'payment configuration facts cannot be physically removed'; END IF;
      IF ROW(NEW.id,NEW.provider,NEW.environment,NEW.app_id,NEW.seller_id,NEW.created_at)
         IS DISTINCT FROM ROW(OLD.id,OLD.provider,OLD.environment,OLD.app_id,OLD.seller_id,OLD.created_at)
         OR NEW.current_version < OLD.current_version THEN RAISE EXCEPTION 'payment configuration identity is immutable'; END IF;
      RETURN NEW;
    END $body$$fn$;

    UPDATE system_config_cache_state SET generation=current_generation+1,updated_at=now()
      WHERE namespace='payment.config' AND scope_key='global';
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,created_at,updated_at)
      VALUES('payment.config','global',current_generation+1,now(),now(),now());
  ELSIF to_regclass('ux_payment_config_code_active') IS NOT NULL
     OR EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='payment_config'::regclass AND conname='ck_payment_config_code') THEN
    RAISE EXCEPTION 'removed payment code protections still exist';
  END IF;
END $migration$;
