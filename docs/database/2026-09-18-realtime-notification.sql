BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.realtime_notification_schema_fingerprint()
RETURNS TEXT
LANGUAGE SQL
STABLE
AS $fingerprint$
WITH targets(table_name) AS (VALUES
  ('message_notification_task'),('message_notification_task_target'),('message_notification'),
  ('message_notification_recipient'),('message_notification_broadcast_state'),
  ('message_notification_mailbox_state'),('message_notification_dispatch_outbox'),
  ('realtime_event'),('realtime_event_outbox'),('realtime_retention_state')
), columns AS (
  SELECT string_agg(format('%s.%s:%s:%s:%s:%s:%s:%s',column_row.table_name,column_row.ordinal_position,column_row.column_name,column_row.udt_name,column_row.is_nullable,column_row.is_identity,COALESCE(column_row.character_maximum_length::text,''),COALESCE(column_row.column_default,'')),E'\n' ORDER BY column_row.table_name,column_row.ordinal_position) AS value
  FROM information_schema.columns column_row JOIN targets ON targets.table_name=column_row.table_name
  WHERE column_row.table_schema=current_schema()
), constraints AS (
  SELECT string_agg(format('%s:%s:%s:%s',relation.relname,constraint_row.conname,constraint_row.contype,pg_get_constraintdef(constraint_row.oid,true)),E'\n' ORDER BY relation.relname,constraint_row.conname) AS value
  FROM pg_constraint constraint_row
  JOIN pg_class relation ON relation.oid=constraint_row.conrelid
  JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
  JOIN targets ON targets.table_name=relation.relname
  WHERE namespace.nspname=current_schema()
), indexes AS (
  SELECT string_agg(format('%s:%s:%s',index_row.tablename,index_row.indexname,replace(index_row.indexdef,format('%I.',current_schema()),'')),E'\n' ORDER BY index_row.tablename,index_row.indexname) AS value
  FROM pg_indexes index_row JOIN targets ON targets.table_name=index_row.tablename
  WHERE index_row.schemaname=current_schema()
)
SELECT md5(concat_ws(E'\n--constraints--\n',COALESCE(columns.value,''),COALESCE(constraints.value,''),COALESCE(indexes.value,'')))
FROM columns,constraints,indexes
$fingerprint$;

DO $schema_preflight$
DECLARE
    target_count INTEGER;
    expected RECORD;
    actual_signature TEXT;
BEGIN
    SELECT count(*) INTO target_count
    FROM pg_class relation
    JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
    WHERE namespace.nspname=current_schema()
      AND relation.relkind='r'
      AND relation.relname IN (
        'message_notification_task','message_notification_task_target','message_notification',
        'message_notification_recipient','message_notification_broadcast_state',
        'message_notification_mailbox_state','message_notification_dispatch_outbox',
        'realtime_event','realtime_event_outbox','realtime_retention_state'
      );

    IF target_count NOT IN (0,10) THEN
        RAISE EXCEPTION 'realtime notification target schema must be entirely absent or complete; found % of 10 tables',target_count;
    END IF;

    IF target_count=10 THEN
		IF pg_temp.realtime_notification_schema_fingerprint() <> '2ebc4dc2c8f4746f21d676304fa120e6' THEN
			RAISE EXCEPTION 'realtime notification target schema fingerprint is incompatible';
		END IF;
        FOR expected IN SELECT * FROM (VALUES
          ('message_notification_task','id:int8:NO:YES,platform_id:int8:NO:NO,title:varchar:NO:NO,content_html:text:NO:NO,summary:varchar:NO:NO,variant:varchar:NO:NO,priority:varchar:NO:NO,link_type:varchar:NO:NO,link:varchar:NO:NO,audience_type:varchar:NO:NO,scheduled_at:timestamptz:YES:NO,audience_max_user_id:int8:YES:NO,submitted_at:timestamptz:YES:NO,status:varchar:NO:NO,next_user_id:int8:NO:NO,next_batch_no:int4:NO:NO,generated_count:int8:NO:NO,failure_message:varchar:YES:NO,published_at:timestamptz:YES:NO,completed_at:timestamptz:YES:NO,canceled_at:timestamptz:YES:NO,failed_at:timestamptz:YES:NO,created_by:int8:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO,deleted_at:timestamptz:YES:NO'),
          ('message_notification_task_target','id:int8:NO:YES,task_id:int8:NO:NO,target_type:varchar:NO:NO,target_id:int8:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO,deleted_at:timestamptz:YES:NO'),
          ('message_notification','id:int8:NO:YES,platform_id:int8:NO:NO,source_task_id:int8:YES:NO,source_type:varchar:NO:NO,source_key:varchar:NO:NO,audience_type:varchar:NO:NO,audience_max_user_id:int8:NO:NO,title:varchar:NO:NO,content_html:text:NO:NO,summary:varchar:NO:NO,variant:varchar:NO:NO,priority:varchar:NO:NO,link_type:varchar:NO:NO,link:varchar:NO:NO,published_at:timestamptz:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO'),
          ('message_notification_recipient','id:int8:NO:YES,notification_id:int8:NO:NO,platform_id:int8:NO:NO,user_id:int8:NO:NO,read_at:timestamptz:YES:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO,deleted_at:timestamptz:YES:NO'),
          ('message_notification_broadcast_state','id:int8:NO:YES,notification_id:int8:NO:NO,platform_id:int8:NO:NO,user_id:int8:NO:NO,read_at:timestamptz:YES:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO,deleted_at:timestamptz:YES:NO'),
          ('message_notification_mailbox_state','platform_id:int8:NO:NO,user_id:int8:NO:NO,read_through_notification_id:int8:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO'),
          ('realtime_event','sequence:int8:NO:YES,event_id:uuid:NO:NO,dedup_key:varchar:NO:NO,platform_id:int8:NO:NO,event_type:varchar:NO:NO,target_type:varchar:NO:NO,target_user_id:int8:YES:NO,audience_max_user_id:int8:YES:NO,payload:jsonb:NO:NO,occurred_at:timestamptz:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO'),
          ('realtime_event_outbox','id:int8:NO:YES,event_sequence:int8:NO:NO,attempts:int4:NO:NO,available_at:timestamptz:NO:NO,locked_until:timestamptz:YES:NO,lock_token:varchar:YES:NO,published_at:timestamptz:YES:NO,last_error:varchar:YES:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO'),
          ('realtime_retention_state','platform_id:int8:NO:NO,deleted_through_sequence:int8:NO:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO'),
          ('message_notification_dispatch_outbox','id:int8:NO:YES,task_id:int8:NO:NO,batch_no:int4:NO:NO,attempts:int4:NO:NO,available_at:timestamptz:NO:NO,locked_until:timestamptz:YES:NO,lock_token:varchar:YES:NO,published_at:timestamptz:YES:NO,last_error:varchar:YES:NO,created_at:timestamptz:NO:NO,updated_at:timestamptz:NO:NO')
        ) AS contracts(table_name,column_signature)
        LOOP
            SELECT string_agg(format('%s:%s:%s:%s',column_name,udt_name,is_nullable,is_identity),',' ORDER BY ordinal_position)
            INTO actual_signature
            FROM information_schema.columns
            WHERE table_schema=current_schema() AND table_name=expected.table_name;
            IF actual_signature IS DISTINCT FROM expected.column_signature THEN
                RAISE EXCEPTION 'realtime notification target schema is incompatible for table %',expected.table_name;
            END IF;
        END LOOP;

        IF EXISTS (
          SELECT 1 FROM (VALUES
            ('message_notification_task','message_notification_task_pkey'),('message_notification_task','fk_message_notification_task_platform'),('message_notification_task','fk_message_notification_task_creator'),('message_notification_task','ck_message_notification_task_title'),('message_notification_task','ck_message_notification_task_content'),('message_notification_task','ck_message_notification_task_summary'),('message_notification_task','ck_message_notification_task_variant'),('message_notification_task','ck_message_notification_task_priority'),('message_notification_task','ck_message_notification_task_link'),('message_notification_task','ck_message_notification_task_audience'),('message_notification_task','ck_message_notification_task_status'),('message_notification_task','ck_message_notification_task_counters'),('message_notification_task','ck_message_notification_task_state'),
            ('message_notification_task_target','message_notification_task_target_pkey'),('message_notification_task_target','fk_message_notification_task_target_task'),('message_notification_task_target','ck_message_notification_task_target_type'),('message_notification_task_target','ck_message_notification_task_target_id'),
            ('message_notification','message_notification_pkey'),('message_notification','fk_message_notification_platform'),('message_notification','fk_message_notification_source_task'),('message_notification','ck_message_notification_source'),('message_notification','ck_message_notification_audience'),('message_notification','ck_message_notification_content'),('message_notification','ck_message_notification_variant'),('message_notification','ck_message_notification_priority'),('message_notification','ck_message_notification_link'),
            ('message_notification_recipient','message_notification_recipient_pkey'),('message_notification_recipient','fk_message_notification_recipient_notification'),('message_notification_recipient','fk_message_notification_recipient_platform'),('message_notification_recipient','fk_message_notification_recipient_user'),
            ('message_notification_broadcast_state','message_notification_broadcast_state_pkey'),('message_notification_broadcast_state','fk_message_notification_broadcast_notification'),('message_notification_broadcast_state','fk_message_notification_broadcast_platform'),('message_notification_broadcast_state','fk_message_notification_broadcast_user'),
            ('message_notification_mailbox_state','message_notification_mailbox_state_pkey'),('message_notification_mailbox_state','fk_message_notification_mailbox_platform'),('message_notification_mailbox_state','fk_message_notification_mailbox_user'),('message_notification_mailbox_state','ck_message_notification_mailbox_watermark'),
            ('realtime_event','realtime_event_pkey'),('realtime_event','fk_realtime_event_platform'),('realtime_event','fk_realtime_event_user'),('realtime_event','ck_realtime_event_type'),('realtime_event','ck_realtime_event_target'),('realtime_event','ck_realtime_event_payload'),
            ('realtime_event_outbox','realtime_event_outbox_pkey'),('realtime_event_outbox','fk_realtime_event_outbox_event'),('realtime_event_outbox','ck_realtime_event_outbox_attempts'),('realtime_event_outbox','ck_realtime_event_outbox_lock'),('realtime_event_outbox','ck_realtime_event_outbox_published'),
            ('realtime_retention_state','realtime_retention_state_pkey'),('realtime_retention_state','fk_realtime_retention_platform'),('realtime_retention_state','ck_realtime_retention_watermark'),
            ('message_notification_dispatch_outbox','message_notification_dispatch_outbox_pkey'),('message_notification_dispatch_outbox','fk_message_notification_dispatch_task'),('message_notification_dispatch_outbox','ck_message_notification_dispatch_counters'),('message_notification_dispatch_outbox','ck_message_notification_dispatch_lock'),('message_notification_dispatch_outbox','ck_message_notification_dispatch_published')
          ) AS required(table_name,constraint_name)
          WHERE NOT EXISTS (
            SELECT 1 FROM pg_constraint constraint_row
            JOIN pg_class relation ON relation.oid=constraint_row.conrelid
            JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
            WHERE namespace.nspname=current_schema() AND relation.relname=required.table_name AND constraint_row.conname=required.constraint_name
          )
        ) THEN
            RAISE EXCEPTION 'realtime notification target schema has missing or incompatible constraints';
        END IF;

        IF EXISTS (
          SELECT 1 FROM (VALUES
            ('message_notification','ux_message_notification_source'),('message_notification','ux_message_notification_source_task'),
            ('message_notification_task_target','ux_message_notification_task_target_active'),
            ('message_notification_recipient','ux_message_notification_recipient'),('message_notification_broadcast_state','ux_message_notification_broadcast_state'),
            ('realtime_event','ux_realtime_event_event_id'),('realtime_event','ux_realtime_event_dedup_key'),
            ('realtime_event_outbox','ux_realtime_event_outbox_event'),('message_notification_dispatch_outbox','ux_message_notification_dispatch_batch'),
            ('message_notification_task','ix_message_notification_task_due'),('message_notification_task','ix_message_notification_task_platform_list'),('message_notification_task','ix_message_notification_task_creator_list'),
            ('message_notification_recipient','ix_message_notification_recipient_mailbox'),('message_notification_recipient','ix_message_notification_recipient_unread'),
            ('message_notification','ix_message_notification_platform_mailbox'),('realtime_event','ix_realtime_event_user_resume'),('realtime_event','ix_realtime_event_platform_resume'),('realtime_event','ix_realtime_event_retention'),
            ('realtime_event_outbox','ix_realtime_event_outbox_pending'),('message_notification_dispatch_outbox','ix_message_notification_dispatch_pending')
          ) AS required(table_name,index_name)
          WHERE NOT EXISTS (
            SELECT 1 FROM pg_indexes index_row
            WHERE index_row.schemaname=current_schema() AND index_row.tablename=required.table_name AND index_row.indexname=required.index_name
          )
        ) THEN
            RAISE EXCEPTION 'realtime notification target schema has missing or incompatible indexes';
        END IF;
    END IF;
END
$schema_preflight$;

CREATE TABLE IF NOT EXISTS message_notification_task (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    platform_id BIGINT NOT NULL,
    title VARCHAR(128) NOT NULL,
    content_html TEXT NOT NULL,
    summary VARCHAR(256) NOT NULL,
    variant VARCHAR(16) NOT NULL,
    priority VARCHAR(16) NOT NULL,
    link_type VARCHAR(16) NOT NULL,
    link VARCHAR(2048) NOT NULL DEFAULT '',
    audience_type VARCHAR(16) NOT NULL,
    scheduled_at TIMESTAMPTZ NULL,
    audience_max_user_id BIGINT NULL,
    submitted_at TIMESTAMPTZ NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    next_user_id BIGINT NOT NULL DEFAULT 0,
    next_batch_no INTEGER NOT NULL DEFAULT 0,
    generated_count BIGINT NOT NULL DEFAULT 0,
    failure_message VARCHAR(512) NULL,
    published_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    canceled_at TIMESTAMPTZ NULL,
    failed_at TIMESTAMPTZ NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT fk_message_notification_task_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_message_notification_task_creator FOREIGN KEY (created_by) REFERENCES user_account(id) ON DELETE RESTRICT,
    CONSTRAINT ck_message_notification_task_title CHECK (btrim(title) <> '' AND char_length(title) <= 128),
    CONSTRAINT ck_message_notification_task_content CHECK (char_length(content_html) <= 16384),
    CONSTRAINT ck_message_notification_task_summary CHECK (char_length(summary) <= 256),
    CONSTRAINT ck_message_notification_task_variant CHECK (variant IN ('info','success','warning','error')),
    CONSTRAINT ck_message_notification_task_priority CHECK (priority IN ('normal','urgent')),
    CONSTRAINT ck_message_notification_task_link CHECK ((link_type='none' AND link='') OR (link_type IN ('internal','external') AND btrim(link)<>'')),
    CONSTRAINT ck_message_notification_task_audience CHECK (audience_type IN ('user','role','platform')),
    CONSTRAINT ck_message_notification_task_status CHECK (status IN ('draft','scheduled','queued','processing','completed','failed','canceled')),
    CONSTRAINT ck_message_notification_task_counters CHECK (next_user_id>=0 AND next_batch_no>=0 AND generated_count>=0),
    CONSTRAINT ck_message_notification_task_state CHECK (
        (status='draft' AND submitted_at IS NULL AND audience_max_user_id IS NULL AND published_at IS NULL AND completed_at IS NULL AND canceled_at IS NULL AND failed_at IS NULL AND failure_message IS NULL)
        OR
        (status<>'draft' AND submitted_at IS NOT NULL AND audience_max_user_id IS NOT NULL AND audience_max_user_id>=0
          AND (status<>'scheduled' OR scheduled_at IS NOT NULL)
          AND (status<>'completed' OR completed_at IS NOT NULL)
          AND (status<>'canceled' OR canceled_at IS NOT NULL)
          AND (status<>'failed' OR (failed_at IS NOT NULL AND failure_message IS NOT NULL AND btrim(failure_message)<>'')))
    )
);

CREATE TABLE IF NOT EXISTS message_notification_task_target (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    task_id BIGINT NOT NULL,
    target_type VARCHAR(16) NOT NULL,
    target_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT fk_message_notification_task_target_task FOREIGN KEY (task_id) REFERENCES message_notification_task(id) ON DELETE CASCADE,
    CONSTRAINT ck_message_notification_task_target_type CHECK (target_type IN ('user','role')),
    CONSTRAINT ck_message_notification_task_target_id CHECK (target_id>0)
);

CREATE TABLE IF NOT EXISTS message_notification (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    platform_id BIGINT NOT NULL,
    source_task_id BIGINT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_key VARCHAR(128) NOT NULL,
    audience_type VARCHAR(16) NOT NULL,
    audience_max_user_id BIGINT NOT NULL,
    title VARCHAR(128) NOT NULL,
    content_html TEXT NOT NULL,
    summary VARCHAR(256) NOT NULL,
    variant VARCHAR(16) NOT NULL,
    priority VARCHAR(16) NOT NULL,
    link_type VARCHAR(16) NOT NULL,
    link VARCHAR(2048) NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_message_notification_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_message_notification_source_task FOREIGN KEY (source_task_id) REFERENCES message_notification_task(id) ON DELETE SET NULL,
    CONSTRAINT ck_message_notification_source CHECK (source_type ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$' AND btrim(source_key)<>''),
    CONSTRAINT ck_message_notification_audience CHECK (audience_type IN ('targeted','platform') AND audience_max_user_id>=0),
    CONSTRAINT ck_message_notification_content CHECK (btrim(title)<>'' AND char_length(title)<=128 AND char_length(content_html)<=16384 AND char_length(summary)<=256),
    CONSTRAINT ck_message_notification_variant CHECK (variant IN ('info','success','warning','error')),
    CONSTRAINT ck_message_notification_priority CHECK (priority IN ('normal','urgent')),
    CONSTRAINT ck_message_notification_link CHECK ((link_type='none' AND link='') OR (link_type IN ('internal','external') AND btrim(link)<>''))
);

CREATE TABLE IF NOT EXISTS message_notification_recipient (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    notification_id BIGINT NOT NULL,
    platform_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT fk_message_notification_recipient_notification FOREIGN KEY (notification_id) REFERENCES message_notification(id) ON DELETE CASCADE,
    CONSTRAINT fk_message_notification_recipient_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_message_notification_recipient_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS message_notification_broadcast_state (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    notification_id BIGINT NOT NULL,
    platform_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT fk_message_notification_broadcast_notification FOREIGN KEY (notification_id) REFERENCES message_notification(id) ON DELETE CASCADE,
    CONSTRAINT fk_message_notification_broadcast_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_message_notification_broadcast_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS message_notification_mailbox_state (
    platform_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    read_through_notification_id BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (platform_id,user_id),
    CONSTRAINT fk_message_notification_mailbox_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_message_notification_mailbox_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT,
    CONSTRAINT ck_message_notification_mailbox_watermark CHECK (read_through_notification_id>=0)
);

CREATE TABLE IF NOT EXISTS realtime_event (
    sequence BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    event_id UUID NOT NULL,
    dedup_key VARCHAR(256) NOT NULL,
    platform_id BIGINT NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    target_type VARCHAR(16) NOT NULL,
    target_user_id BIGINT NULL,
    audience_max_user_id BIGINT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_realtime_event_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT fk_realtime_event_user FOREIGN KEY (target_user_id) REFERENCES user_account(id) ON DELETE RESTRICT,
    CONSTRAINT ck_realtime_event_type CHECK (btrim(event_type)<>''),
    CONSTRAINT ck_realtime_event_target CHECK ((target_type='user' AND target_user_id IS NOT NULL AND target_user_id>0 AND audience_max_user_id IS NULL) OR (target_type='platform' AND target_user_id IS NULL AND audience_max_user_id IS NOT NULL AND audience_max_user_id>=0)),
    CONSTRAINT ck_realtime_event_payload CHECK (jsonb_typeof(payload)='object')
);

CREATE TABLE IF NOT EXISTS realtime_event_outbox (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    event_sequence BIGINT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ NULL,
    lock_token VARCHAR(64) NULL,
    published_at TIMESTAMPTZ NULL,
    last_error VARCHAR(512) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_realtime_event_outbox_event FOREIGN KEY (event_sequence) REFERENCES realtime_event(sequence) ON DELETE CASCADE,
    CONSTRAINT ck_realtime_event_outbox_attempts CHECK (attempts>=0),
    CONSTRAINT ck_realtime_event_outbox_lock CHECK ((locked_until IS NULL)=(lock_token IS NULL)),
    CONSTRAINT ck_realtime_event_outbox_published CHECK (published_at IS NULL OR (locked_until IS NULL AND lock_token IS NULL))
);

CREATE TABLE IF NOT EXISTS realtime_retention_state (
    platform_id BIGINT PRIMARY KEY,
    deleted_through_sequence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_realtime_retention_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
    CONSTRAINT ck_realtime_retention_watermark CHECK (deleted_through_sequence>=0)
);

CREATE TABLE IF NOT EXISTS message_notification_dispatch_outbox (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    task_id BIGINT NOT NULL,
    batch_no INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ NULL,
    lock_token VARCHAR(64) NULL,
    published_at TIMESTAMPTZ NULL,
    last_error VARCHAR(512) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_message_notification_dispatch_task FOREIGN KEY (task_id) REFERENCES message_notification_task(id) ON DELETE CASCADE,
    CONSTRAINT ck_message_notification_dispatch_counters CHECK (batch_no>=0 AND attempts>=0),
    CONSTRAINT ck_message_notification_dispatch_lock CHECK ((locked_until IS NULL)=(lock_token IS NULL)),
    CONSTRAINT ck_message_notification_dispatch_published CHECK (published_at IS NULL OR (locked_until IS NULL AND lock_token IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_source ON message_notification(platform_id,source_type,source_key);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_source_task ON message_notification(source_task_id) WHERE source_task_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_task_target_active ON message_notification_task_target(task_id,target_type,target_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_recipient ON message_notification_recipient(notification_id,user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_broadcast_state ON message_notification_broadcast_state(notification_id,user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_realtime_event_event_id ON realtime_event(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_realtime_event_dedup_key ON realtime_event(dedup_key);
CREATE UNIQUE INDEX IF NOT EXISTS ux_realtime_event_outbox_event ON realtime_event_outbox(event_sequence);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_notification_dispatch_batch ON message_notification_dispatch_outbox(task_id,batch_no);
CREATE INDEX IF NOT EXISTS ix_message_notification_task_due ON message_notification_task(status,scheduled_at,id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_task_platform_list ON message_notification_task(platform_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_task_creator_list ON message_notification_task(created_by,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_recipient_mailbox ON message_notification_recipient(platform_id,user_id,notification_id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_recipient_unread ON message_notification_recipient(platform_id,user_id,notification_id DESC) WHERE deleted_at IS NULL AND read_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_platform_mailbox ON message_notification(platform_id,id DESC) WHERE audience_type='platform';
CREATE INDEX IF NOT EXISTS ix_realtime_event_user_resume ON realtime_event(platform_id,target_user_id,sequence) WHERE target_type='user';
CREATE INDEX IF NOT EXISTS ix_realtime_event_platform_resume ON realtime_event(platform_id,sequence) WHERE target_type='platform';
CREATE INDEX IF NOT EXISTS ix_realtime_event_retention ON realtime_event(occurred_at,sequence);
CREATE INDEX IF NOT EXISTS ix_realtime_event_outbox_pending ON realtime_event_outbox(available_at,id) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_message_notification_dispatch_pending ON message_notification_dispatch_outbox(available_at,id) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_permission_user_role_role_history_cursor ON permission_user_role(role_id,user_id,created_at,deleted_at);

DO $migration$
DECLARE
    admin_platform_id BIGINT;
    message_directory_id BIGINT;
    notification_page_id BIGINT;
    task_page_id BIGINT;
    admin_count INTEGER;
    directory_count INTEGER;
    changed_rows INTEGER;
    setting_changed BOOLEAN := FALSE;
    menu_changed BOOLEAN := FALSE;
    affected_role_ids BIGINT[] := ARRAY[]::BIGINT[];
    role_row RECORD;
    action_row RECORD;
    setting_row RECORD;
    next_generation BIGINT;
BEGIN
    SELECT count(*),min(id) INTO admin_count,admin_platform_id FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL;
    IF admin_count<>1 THEN RAISE EXCEPTION 'expected exactly one active Admin platform'; END IF;

    SELECT count(*),min(id) INTO directory_count,message_directory_id FROM permission_menu WHERE platform_id=admin_platform_id AND code='message' AND menu_type='directory' AND is_enabled=1 AND deleted_at IS NULL;
    IF directory_count<>1 THEN RAISE EXCEPTION 'expected exactly one active Admin message directory'; END IF;

    FOR setting_row IN SELECT * FROM (VALUES
        ('message.notification.retention_days',180,30,3650,'站内通知保留天数'),
        ('realtime.event.retention_days',7,1,30,'实时事件保留天数')
    ) AS settings(setting_key,default_value,min_value,max_value,description)
    LOOP
        IF EXISTS (SELECT 1 FROM system_setting WHERE setting_key=setting_row.setting_key AND deleted_at IS NULL) THEN
            IF EXISTS (SELECT 1 FROM system_setting WHERE setting_key=setting_row.setting_key AND deleted_at IS NULL AND (value !~ '^[0-9]+$' OR value::NUMERIC<>trunc(value::NUMERIC) OR value::NUMERIC<setting_row.min_value OR value::NUMERIC>setting_row.max_value)) THEN
                RAISE EXCEPTION 'invalid existing setting %',setting_row.setting_key;
            END IF;
            UPDATE system_setting SET value_type=2,is_enabled=1,is_builtin=1,description=setting_row.description,updated_at=CURRENT_TIMESTAMP
            WHERE setting_key=setting_row.setting_key AND deleted_at IS NULL
              AND (value_type<>2 OR is_enabled<>1 OR is_builtin<>1 OR description<>setting_row.description);
            GET DIAGNOSTICS changed_rows=ROW_COUNT;
            setting_changed := setting_changed OR changed_rows>0;
        ELSE
            INSERT INTO system_setting(setting_key,value,value_type,description,is_enabled,is_builtin,created_at,updated_at)
            VALUES(setting_row.setting_key,setting_row.default_value::TEXT,2,setting_row.description,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            setting_changed := TRUE;
        END IF;
    END LOOP;

    IF setting_changed THEN
        UPDATE system_config_cache_generation SET generation=generation+1,updated_at=CURRENT_TIMESTAMP
        WHERE namespace='system.setting' AND scope_key='global' RETURNING generation INTO next_generation;
        IF next_generation IS NULL THEN RAISE EXCEPTION 'system.setting/global generation is missing'; END IF;
        INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,last_error,created_at,updated_at)
        VALUES('system.setting','global',next_generation,0,CURRENT_TIMESTAMP,'',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
    END IF;

    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND deleted_at IS NULL AND path='/message/notification' AND code<>'message:notification:view')
       OR EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND deleted_at IS NULL AND component_path='message/notification' AND code<>'message:notification:view') THEN
        RAISE EXCEPTION 'notification page path is occupied';
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code='message:notification:view' AND deleted_at IS NULL AND NOT (menu_type='page' AND parent_id IS NULL AND name='通知中心' AND i18n_key='navigation.messageNotification' AND path='/message/notification' AND component_path='message/notification' AND icon='lucide:bell' AND sort_order=0 AND is_enabled=1 AND is_hidden=1)) THEN
        RAISE EXCEPTION 'incompatible menu node message:notification:view';
    END IF;
    SELECT id INTO notification_page_id FROM permission_menu WHERE platform_id=admin_platform_id AND code='message:notification:view' AND deleted_at IS NULL;
    IF notification_page_id IS NULL THEN
        INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at)
        VALUES(admin_platform_id,NULL,'page','通知中心','message:notification:view','navigation.messageNotification','/message/notification','message/notification','lucide:bell',0,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
        RETURNING id INTO notification_page_id;
        menu_changed := TRUE;
    END IF;

    FOR action_row IN SELECT * FROM (VALUES
        ('message:notification:list','查看个人通知',10),
        ('message:notification:read','标记个人通知已读',20),
        ('message:notification:delete','删除个人通知',30)
    ) AS actions(code,name,sort_order)
    LOOP
        IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code=action_row.code AND deleted_at IS NULL AND NOT (menu_type='action' AND parent_id=notification_page_id AND name=action_row.name AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND sort_order=action_row.sort_order AND is_enabled=1 AND is_hidden=1)) THEN
            RAISE EXCEPTION 'incompatible menu node %',action_row.code;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code=action_row.code AND deleted_at IS NULL) THEN
            INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,sort_order,is_enabled,is_hidden,created_at,updated_at)
            VALUES(admin_platform_id,notification_page_id,'action',action_row.name,action_row.code,action_row.sort_order,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            menu_changed := TRUE;
        END IF;
    END LOOP;

    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND deleted_at IS NULL AND path='/message/notificationTask' AND code<>'message:notificationTask:view')
       OR EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND deleted_at IS NULL AND component_path='message/notificationTask' AND code<>'message:notificationTask:view') THEN
        RAISE EXCEPTION 'notification task page path is occupied';
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code='message:notificationTask:view' AND deleted_at IS NULL AND NOT (menu_type='page' AND parent_id=message_directory_id AND name='通知任务' AND i18n_key='navigation.messageNotificationTask' AND path='/message/notificationTask' AND component_path='message/notificationTask' AND icon='lucide:bell' AND sort_order=30 AND is_enabled=1 AND is_hidden=0)) THEN
        RAISE EXCEPTION 'incompatible menu node message:notificationTask:view';
    END IF;
    SELECT id INTO task_page_id FROM permission_menu WHERE platform_id=admin_platform_id AND code='message:notificationTask:view' AND deleted_at IS NULL;
    IF task_page_id IS NULL THEN
        INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at)
        VALUES(admin_platform_id,message_directory_id,'page','通知任务','message:notificationTask:view','navigation.messageNotificationTask','/message/notificationTask','message/notificationTask','lucide:bell',30,1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
        RETURNING id INTO task_page_id;
        menu_changed := TRUE;
    END IF;

    FOR action_row IN SELECT * FROM (VALUES
        ('message:notificationTask:list','查看通知任务',10),('message:notificationTask:detail','查看通知任务详情',20),
        ('message:notificationTask:create','创建通知任务',30),('message:notificationTask:update','编辑通知任务',40),
        ('message:notificationTask:delete','删除通知任务',50),('message:notificationTask:submit','提交通知任务',60),
        ('message:notificationTask:cancel','取消通知任务',70),('message:notificationTask:copy','复制通知任务',80)
    ) AS actions(code,name,sort_order)
    LOOP
        IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code=action_row.code AND deleted_at IS NULL AND NOT (menu_type='action' AND parent_id=task_page_id AND name=action_row.name AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND sort_order=action_row.sort_order AND is_enabled=1 AND is_hidden=1)) THEN
            RAISE EXCEPTION 'incompatible menu node %',action_row.code;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_platform_id AND code=action_row.code AND deleted_at IS NULL) THEN
            INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,sort_order,is_enabled,is_hidden,created_at,updated_at)
            VALUES(admin_platform_id,task_page_id,'action',action_row.name,action_row.code,action_row.sort_order,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            menu_changed := TRUE;
        END IF;
    END LOOP;

    FOR role_row IN SELECT id FROM permission_role WHERE is_enabled=1 AND deleted_at IS NULL ORDER BY id
    LOOP
        changed_rows := 0;
        INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at)
        SELECT role_row.id,m.id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
        FROM permission_menu m
        WHERE m.platform_id=admin_platform_id AND m.deleted_at IS NULL
          AND m.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete')
          AND NOT EXISTS (SELECT 1 FROM permission_role_menu rm WHERE rm.role_id=role_row.id AND rm.menu_id=m.id AND rm.deleted_at IS NULL);
        GET DIAGNOSTICS changed_rows=ROW_COUNT;
        IF changed_rows>0 THEN affected_role_ids:=array_append(affected_role_ids,role_row.id); END IF;
    END LOOP;

    IF cardinality(affected_role_ids)>0 THEN
        UPDATE permission_access_version av SET version=av.version+1,updated_at=CURRENT_TIMESTAMP
        FROM user_account u
        WHERE u.id=av.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
          AND EXISTS (SELECT 1 FROM permission_user_role ur WHERE ur.user_id=av.user_id AND ur.role_id=ANY(affected_role_ids) AND ur.deleted_at IS NULL);
    END IF;

    IF menu_changed THEN
        UPDATE permission_auth_platform SET menu_version=menu_version+1,updated_at=CURRENT_TIMESTAMP WHERE id=admin_platform_id;
    END IF;
END
$migration$;

DO $schema_postflight$
BEGIN
    IF pg_temp.realtime_notification_schema_fingerprint() <> '2ebc4dc2c8f4746f21d676304fa120e6' THEN
        RAISE EXCEPTION 'realtime notification target schema fingerprint is incompatible after migration';
    END IF;
END
$schema_postflight$;

COMMIT;
