-- Maintainer-only P4 forward migration. Run in one transaction with API/Worker stopped.
-- This migration creates redeem-code financial facts and extends the existing
-- transaction source contract. It does not seed codes, money, menus, roles, or Redis.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';
SELECT pg_advisory_xact_lock(20261017, 401);

DO $prerequisites$
BEGIN
    IF to_regclass('payment_transaction') IS NULL OR to_regclass('payment_wallet') IS NULL
       OR to_regclass('user_account') IS NULL THEN
        RAISE EXCEPTION 'P2 payment transaction, wallet, and user tables are required first';
    END IF;
END $prerequisites$;

DO $existing_contract$
DECLARE item record; actual_names text[];
BEGIN
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_code_batch', ARRAY['id','batch_no','request_id','request_fingerprint','amount_cents','quantity','expires_at','remark','created_by','created_at','updated_at']::text[]),
        ('payment_redeem_code', ARRAY['id','batch_id','code_hash','code_ciphertext','status','used_by','used_at','created_at','updated_at']::text[])
    ) AS expected(table_name, columns) LOOP
        IF to_regclass(item.table_name) IS NOT NULL THEN
            SELECT array_agg(a.attname::text ORDER BY a.attnum) INTO actual_names
              FROM pg_attribute a WHERE a.attrelid=to_regclass(item.table_name)
                AND a.attnum>0 AND NOT a.attisdropped;
            IF actual_names IS DISTINCT FROM item.columns THEN
                RAISE EXCEPTION 'payment redeem table column contract differs: %', item.table_name;
            END IF;
        END IF;
    END LOOP;
END $existing_contract$;

DO $existing_objects$
DECLARE item record; names text[];
BEGIN
    IF (to_regclass('payment_redeem_code_batch') IS NULL) <> (to_regclass('payment_redeem_code') IS NULL) THEN
        RAISE EXCEPTION 'payment redeem schema is partial; inspect before continuing';
    END IF;
    IF to_regclass('payment_redeem_code_batch') IS NOT NULL THEN
        FOR item IN SELECT * FROM (VALUES
            ('payment_redeem_code_batch',ARRAY['payment_redeem_code_batch_pkey','fk_payment_redeem_batch_creator','ck_payment_redeem_batch_no','ck_payment_redeem_batch_fingerprint','ck_payment_redeem_batch_amount','ck_payment_redeem_batch_quantity','ck_payment_redeem_batch_remark']::text[]),
            ('payment_redeem_code',ARRAY['payment_redeem_code_pkey','fk_payment_redeem_code_batch','fk_payment_redeem_code_user','ck_payment_redeem_code_hash','ck_payment_redeem_code_ciphertext','ck_payment_redeem_code_status','ck_payment_redeem_code_used_state']::text[])
        ) AS expected(table_name,constraints) LOOP
            SELECT array_agg(conname::text ORDER BY conname) INTO names FROM pg_constraint WHERE conrelid=item.table_name::regclass AND contype <> 'n';
            IF coalesce(array_length(names,1),0) <> array_length(item.constraints,1)
               OR EXISTS (SELECT 1 FROM unnest(item.constraints) expected_name WHERE NOT EXISTS (SELECT 1 FROM unnest(COALESCE(names,'{}'::text[])) actual_name WHERE actual_name=expected_name)) THEN
                RAISE EXCEPTION 'payment redeem constraint set differs: %',item.table_name;
            END IF;
        END LOOP;
        FOR item IN SELECT * FROM (VALUES
            ('payment_redeem_code_batch','ux_payment_redeem_batch_no'),('payment_redeem_code_batch','ux_payment_redeem_batch_request'),
            ('payment_redeem_code','ux_payment_redeem_code_hash'),('payment_redeem_code','ix_payment_redeem_code_batch'),('payment_redeem_code','ix_payment_redeem_code_used_by'),
            ('payment_transaction','ux_payment_transaction_redeem_code')
        ) AS expected(table_name,index_name) LOOP
            IF NOT EXISTS (SELECT 1 FROM pg_index WHERE indexrelid=to_regclass(item.index_name) AND indrelid=item.table_name::regclass) THEN
                RAISE EXCEPTION 'payment redeem index is missing: %',item.index_name;
            END IF;
        END LOOP;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='fk_payment_transaction_redeem_code')
           OR NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_source_xor')
           OR EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_payment_source') THEN
            RAISE EXCEPTION 'payment transaction P4 source contract differs';
        END IF;
        FOR item IN SELECT * FROM (VALUES
            ('payment_redeem_code_batch','payment_redeem_batch_guard','payment_redeem_batch_guard'),
            ('payment_redeem_code_batch','payment_redeem_batch_truncate_guard','payment_redeem_batch_guard'),
            ('payment_redeem_code','payment_redeem_code_guard','payment_redeem_code_guard'),
            ('payment_redeem_code','payment_redeem_code_truncate_guard','payment_redeem_code_guard'),
            ('payment_transaction','payment_transaction_immutable','payment_transaction_immutable_guard')
        ) AS expected(table_name,trigger_name,function_name) LOOP
            IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid=item.table_name::regclass AND tgname=item.trigger_name)
               OR to_regprocedure(item.function_name||'()') IS NULL THEN
                RAISE EXCEPTION 'payment redeem guard missing: %',item.trigger_name;
            END IF;
        END LOOP;
    END IF;
END $existing_objects$;

CREATE TABLE IF NOT EXISTS payment_redeem_code_batch (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    batch_no varchar(32) NOT NULL,
    request_id uuid NOT NULL,
    request_fingerprint bytea NOT NULL,
    amount_cents bigint NOT NULL,
    quantity integer NOT NULL,
    expires_at timestamptz,
    remark varchar(255),
    created_by bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_payment_redeem_batch_creator FOREIGN KEY (created_by) REFERENCES user_account(id) ON DELETE RESTRICT,
    CONSTRAINT ck_payment_redeem_batch_no CHECK (batch_no ~ '^[A-Za-z0-9_-]{1,32}$'),
    CONSTRAINT ck_payment_redeem_batch_fingerprint CHECK (octet_length(request_fingerprint)=32),
    CONSTRAINT ck_payment_redeem_batch_amount CHECK (amount_cents BETWEEN 1 AND 9223372036854),
    CONSTRAINT ck_payment_redeem_batch_quantity CHECK (quantity BETWEEN 1 AND 1000),
    CONSTRAINT ck_payment_redeem_batch_remark CHECK (remark IS NULL OR btrim(remark) <> '')
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_redeem_batch_no ON payment_redeem_code_batch(batch_no);
CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_redeem_batch_request ON payment_redeem_code_batch(created_by,request_id);

CREATE TABLE IF NOT EXISTS payment_redeem_code (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    batch_id bigint NOT NULL,
    code_hash bytea NOT NULL,
    code_ciphertext text NOT NULL,
    status smallint NOT NULL DEFAULT 1,
    used_by bigint,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_payment_redeem_code_batch FOREIGN KEY (batch_id) REFERENCES payment_redeem_code_batch(id) ON DELETE RESTRICT,
    CONSTRAINT fk_payment_redeem_code_user FOREIGN KEY (used_by) REFERENCES user_account(id) ON DELETE RESTRICT,
    CONSTRAINT ck_payment_redeem_code_hash CHECK (octet_length(code_hash)=32),
    CONSTRAINT ck_payment_redeem_code_ciphertext CHECK (octet_length(code_ciphertext) BETWEEN 1 AND 131072),
    CONSTRAINT ck_payment_redeem_code_status CHECK (status IN (1,2,3)),
    CONSTRAINT ck_payment_redeem_code_used_state CHECK (
        (status IN (1,3) AND used_by IS NULL AND used_at IS NULL)
        OR (status=2 AND used_by IS NOT NULL AND used_at IS NOT NULL)
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_redeem_code_hash ON payment_redeem_code(code_hash);
CREATE INDEX IF NOT EXISTS ix_payment_redeem_code_batch ON payment_redeem_code(batch_id,id);
CREATE INDEX IF NOT EXISTS ix_payment_redeem_code_used_by ON payment_redeem_code(used_by,used_at DESC,id DESC) WHERE status=2;

DO $validate_columns$
DECLARE item record;
BEGIN
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_code_batch','id','bigint',true,''),
        ('payment_redeem_code_batch','batch_no','character varying(32)',true,''),
        ('payment_redeem_code_batch','request_id','uuid',true,''),
        ('payment_redeem_code_batch','request_fingerprint','bytea',true,''),
        ('payment_redeem_code_batch','amount_cents','bigint',true,''),
        ('payment_redeem_code_batch','quantity','integer',true,''),
        ('payment_redeem_code_batch','expires_at','timestamp with time zone',false,''),
        ('payment_redeem_code_batch','remark','character varying(255)',false,''),
        ('payment_redeem_code_batch','created_by','bigint',true,''),
        ('payment_redeem_code_batch','created_at','timestamp with time zone',true,'CURRENT_TIMESTAMP'),
        ('payment_redeem_code_batch','updated_at','timestamp with time zone',true,'CURRENT_TIMESTAMP'),
        ('payment_redeem_code','id','bigint',true,''),
        ('payment_redeem_code','batch_id','bigint',true,''),
        ('payment_redeem_code','code_hash','bytea',true,''),
        ('payment_redeem_code','code_ciphertext','text',true,''),
        ('payment_redeem_code','status','smallint',true,'1'),
        ('payment_redeem_code','used_by','bigint',false,''),
        ('payment_redeem_code','used_at','timestamp with time zone',false,''),
        ('payment_redeem_code','created_at','timestamp with time zone',true,'CURRENT_TIMESTAMP'),
        ('payment_redeem_code','updated_at','timestamp with time zone',true,'CURRENT_TIMESTAMP')
    ) AS expected(table_name,column_name,data_type,not_null,default_value) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=item.table_name::regclass AND a.attname=item.column_name AND format_type(a.atttypid,a.atttypmod)=item.data_type AND a.attnotnull=item.not_null AND NOT a.attisdropped AND a.attgenerated='' AND a.attidentity=CASE WHEN item.column_name='id' THEN 'd' ELSE '' END AND COALESCE(pg_get_expr(d.adbin,d.adrelid),'')=item.default_value) THEN
            RAISE EXCEPTION 'payment redeem column contract differs: %.%',item.table_name,item.column_name;
        END IF;
    END LOOP;
END $validate_columns$;

-- Compare parser-normalized CHECK definitions rather than accepting same names.
CREATE TEMP TABLE payment_redeem_expected_check (
    batch_no varchar(32),request_fingerprint bytea,amount_cents bigint,quantity integer,remark varchar(255),
    code_hash bytea,code_ciphertext text,status smallint,used_by bigint,used_at timestamptz,order_id bigint,redeem_code_id bigint
) ON COMMIT DROP;
ALTER TABLE payment_redeem_expected_check
    ADD CONSTRAINT ck_payment_redeem_batch_no CHECK (batch_no ~ '^[A-Za-z0-9_-]{1,32}$'),
    ADD CONSTRAINT ck_payment_redeem_batch_fingerprint CHECK (octet_length(request_fingerprint)=32),
    ADD CONSTRAINT ck_payment_redeem_batch_amount CHECK (amount_cents BETWEEN 1 AND 9223372036854),
    ADD CONSTRAINT ck_payment_redeem_batch_quantity CHECK (quantity BETWEEN 1 AND 1000),
    ADD CONSTRAINT ck_payment_redeem_batch_remark CHECK (remark IS NULL OR btrim(remark) <> ''),
    ADD CONSTRAINT ck_payment_redeem_code_hash CHECK (octet_length(code_hash)=32),
    ADD CONSTRAINT ck_payment_redeem_code_ciphertext CHECK (octet_length(code_ciphertext) BETWEEN 1 AND 131072),
    ADD CONSTRAINT ck_payment_redeem_code_status CHECK (status IN (1,2,3)),
    ADD CONSTRAINT ck_payment_redeem_code_used_state CHECK ((status IN (1,3) AND used_by IS NULL AND used_at IS NULL) OR (status=2 AND used_by IS NOT NULL AND used_at IS NOT NULL)),
    ADD CONSTRAINT ck_payment_transaction_payment_source CHECK (order_id IS NOT NULL AND redeem_code_id IS NULL),
    ADD CONSTRAINT ck_payment_transaction_source_xor CHECK ((order_id IS NOT NULL) <> (redeem_code_id IS NOT NULL));
DO $validate_checks$
DECLARE item record; target regclass;
BEGIN
    FOR item IN SELECT conname,pg_get_constraintdef(oid) definition FROM pg_constraint WHERE conrelid='pg_temp.payment_redeem_expected_check'::regclass LOOP
        target:=CASE WHEN item.conname LIKE 'ck_payment_redeem_batch_%' THEN 'payment_redeem_code_batch'::regclass WHEN item.conname LIKE 'ck_payment_redeem_code_%' THEN 'payment_redeem_code'::regclass ELSE 'payment_transaction'::regclass END;
        IF item.conname LIKE 'ck_payment_transaction_%' AND NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=target AND conname=item.conname) THEN CONTINUE; END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=target AND conname=item.conname AND contype='c' AND convalidated AND pg_get_constraintdef(oid)=item.definition) THEN
            RAISE EXCEPTION 'payment redeem CHECK contract differs: %',item.conname;
        END IF;
    END LOOP;
END $validate_checks$;

DO $transaction_contract$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='fk_payment_transaction_redeem_code') THEN
        ALTER TABLE payment_transaction ADD CONSTRAINT fk_payment_transaction_redeem_code
            FOREIGN KEY (redeem_code_id) REFERENCES payment_redeem_code(id) ON DELETE RESTRICT;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='fk_payment_transaction_redeem_code'
               AND (confrelid <> 'payment_redeem_code'::regclass OR confdeltype <> 'r')) THEN
        RAISE EXCEPTION 'payment transaction redeem FK contract differs';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_payment_source') THEN
        ALTER TABLE payment_transaction DROP CONSTRAINT ck_payment_transaction_payment_source;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_source_xor') THEN
        ALTER TABLE payment_transaction ADD CONSTRAINT ck_payment_transaction_source_xor
            CHECK ((order_id IS NOT NULL) <> (redeem_code_id IS NOT NULL));
    END IF;
END $transaction_contract$;
CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_transaction_redeem_code ON payment_transaction(redeem_code_id) WHERE redeem_code_id IS NOT NULL;

DO $validate_keys$
DECLARE item record;
BEGIN
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_code_batch','payment_redeem_code_batch_pkey','p',ARRAY['id']::text[],NULL::text,NULL::text[]),
        ('payment_redeem_code','payment_redeem_code_pkey','p',ARRAY['id']::text[],NULL::text,NULL::text[]),
        ('payment_redeem_code_batch','fk_payment_redeem_batch_creator','f',ARRAY['created_by']::text[],'user_account',ARRAY['id']::text[]),
        ('payment_redeem_code','fk_payment_redeem_code_batch','f',ARRAY['batch_id']::text[],'payment_redeem_code_batch',ARRAY['id']::text[]),
        ('payment_redeem_code','fk_payment_redeem_code_user','f',ARRAY['used_by']::text[],'user_account',ARRAY['id']::text[]),
        ('payment_transaction','fk_payment_transaction_redeem_code','f',ARRAY['redeem_code_id']::text[],'payment_redeem_code',ARRAY['id']::text[])
    ) AS expected(table_name,constraint_name,kind,columns,ref_table,ref_columns) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_constraint c WHERE c.conrelid=item.table_name::regclass AND c.conname=item.constraint_name AND c.contype::text=item.kind AND c.convalidated AND NOT c.condeferrable AND NOT c.condeferred
            AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num ORDER BY k.ord)=item.columns
            AND (item.kind='p' OR (c.confrelid=item.ref_table::regclass AND c.confupdtype='a' AND c.confdeltype='r' AND c.confmatchtype='s'
                AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num ORDER BY k.ord)=item.ref_columns))) THEN
            RAISE EXCEPTION 'payment redeem PK/FK contract differs: %',item.constraint_name;
        END IF;
    END LOOP;
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_code_batch','ux_payment_redeem_batch_no',true,ARRAY['batch_no']::text[],ARRAY[0]::int2[],NULL::text),
        ('payment_redeem_code_batch','ux_payment_redeem_batch_request',true,ARRAY['created_by','request_id']::text[],ARRAY[0,0]::int2[],NULL::text),
        ('payment_redeem_code','ux_payment_redeem_code_hash',true,ARRAY['code_hash']::text[],ARRAY[0]::int2[],NULL::text),
        ('payment_redeem_code','ix_payment_redeem_code_batch',false,ARRAY['batch_id','id']::text[],ARRAY[0,0]::int2[],NULL::text),
        ('payment_redeem_code','ix_payment_redeem_code_used_by',false,ARRAY['used_by','used_at','id']::text[],ARRAY[0,3,3]::int2[],'(status = 2)'),
        ('payment_transaction','ux_payment_transaction_redeem_code',true,ARRAY['redeem_code_id']::text[],ARRAY[0]::int2[],'(redeem_code_id IS NOT NULL)')
    ) AS expected(table_name,index_name,is_unique,columns,options,predicate) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid JOIN pg_am am ON am.oid=idx.relam WHERE i.indexrelid=to_regclass(item.index_name) AND i.indrelid=item.table_name::regclass AND i.indisunique=item.is_unique AND i.indisvalid AND i.indisready AND i.indislive AND i.indimmediate AND NOT i.indnullsnotdistinct AND am.amname='btree' AND i.indexprs IS NULL AND i.indnkeyatts=i.indnatts AND pg_get_expr(i.indpred,i.indrelid) IS NOT DISTINCT FROM item.predicate
            AND ARRAY(SELECT o FROM unnest(i.indoption) o)=item.options
            AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.num ORDER BY k.ord)=item.columns) THEN
            RAISE EXCEPTION 'payment redeem index contract differs: %',item.index_name;
        END IF;
    END LOOP;
END $validate_keys$;

DO $guard_functions$
DECLARE item record;
BEGIN
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_batch_guard', $guard$
BEGIN
    RAISE EXCEPTION 'payment redeem batches are immutable';
END $guard$),
        ('payment_redeem_code_guard', $guard$
BEGIN
    IF TG_OP IN ('DELETE','TRUNCATE') THEN RAISE EXCEPTION 'payment redeem codes cannot be physically removed'; END IF;
    IF TG_OP='UPDATE' AND (ROW(NEW.id,NEW.batch_id,NEW.code_hash,NEW.code_ciphertext,NEW.created_at)
       IS DISTINCT FROM ROW(OLD.id,OLD.batch_id,OLD.code_hash,OLD.code_ciphertext,OLD.created_at)
       OR (OLD.status<>1) OR (NEW.status NOT IN (2,3))) THEN
        RAISE EXCEPTION 'payment redeem code identity or terminal state is immutable';
    END IF;
    RETURN NEW;
END $guard$),
        ('payment_transaction_immutable_guard', $guard$
BEGIN
    RAISE EXCEPTION 'payment transactions are immutable';
END $guard$)
    ) AS expected(name,body) LOOP
        IF to_regprocedure(item.name||'()') IS NULL THEN
            EXECUTE format('CREATE FUNCTION %I() RETURNS trigger LANGUAGE plpgsql AS %L',item.name,item.body);
        ELSIF NOT EXISTS (SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang WHERE p.oid=to_regprocedure(item.name||'()') AND l.lanname='plpgsql' AND p.prorettype='trigger'::regtype AND NOT p.prosecdef AND p.proconfig IS NULL AND btrim(regexp_replace(p.prosrc,'\s+',' ','g'))=btrim(regexp_replace(item.body,'\s+',' ','g'))) THEN
            RAISE EXCEPTION 'payment redeem guard function contract differs: %',item.name;
        END IF;
    END LOOP;
END $guard_functions$;

DO $guard_triggers$
DECLARE item record;
BEGIN
    FOR item IN SELECT * FROM (VALUES
        ('payment_redeem_code_batch','payment_redeem_batch_guard','payment_redeem_batch_guard',27,'UPDATE OR DELETE','ROW'),
        ('payment_redeem_code_batch','payment_redeem_batch_truncate_guard','payment_redeem_batch_guard',34,'TRUNCATE','STATEMENT'),
        ('payment_redeem_code','payment_redeem_code_guard','payment_redeem_code_guard',27,'UPDATE OR DELETE','ROW'),
        ('payment_redeem_code','payment_redeem_code_truncate_guard','payment_redeem_code_guard',34,'TRUNCATE','STATEMENT'),
        ('payment_transaction','payment_transaction_immutable','payment_transaction_immutable_guard',18,'UPDATE','STATEMENT')
    ) AS expected(table_name,trigger_name,function_name,trigger_type,events,granularity) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid=item.table_name::regclass AND tgname=item.trigger_name) THEN
            EXECUTE format('CREATE TRIGGER %I BEFORE %s ON %I FOR EACH %s EXECUTE FUNCTION %I()',item.trigger_name,item.events,item.table_name,item.granularity,item.function_name);
        ELSIF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid=item.table_name::regclass AND tgname=item.trigger_name AND tgtype=item.trigger_type AND tgenabled='O' AND NOT tgisinternal AND tgfoid=to_regprocedure(item.function_name||'()') AND tgqual IS NULL AND tgnargs=0 AND tgattr=''::int2vector) THEN
            RAISE EXCEPTION 'payment redeem guard trigger contract differs: %',item.trigger_name;
        END IF;
    END LOOP;
END $guard_triggers$;

DO $verify$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_source_xor') THEN
        RAISE EXCEPTION 'payment transaction source XOR constraint missing';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname='ux_payment_transaction_redeem_code') THEN
        RAISE EXCEPTION 'payment transaction redeem uniqueness missing';
    END IF;
END $verify$;
