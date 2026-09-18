-- COS 配置版本化、上传规则互斥启用与 v2 对象 key 迁移（唯一入口见同名 .ps1）。
-- 幂等：结构变更全部带 IF NOT EXISTS / 约束存在性判断；数据回填与 seed 使用 ON CONFLICT DO NOTHING，
-- 重复执行不新增物理版本、不推进任何 generation、不重复清空对象引用。
-- 旧对象协议整体废弃：不创建 legacy route 表，旧 key 一律视为无效请求；已知数据库旧引用在本迁移中清空，
-- 但不调用 COS API 删除 Bucket 中已存在的对象字节。
-- 不修改菜单、角色授权或权限码，不递增 permission_auth_platform.menu_version。

BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- 迁移窗口内阻断并发写入：storage 三表、对象引用事实表与 generation/outbox 全部进入写排斥锁。
LOCK TABLE storage_cos_config, storage_upload_rule, storage_upload_rule_code,
           system_setting, user_profile, system_config_cache_generation, system_config_cache_outbox
  IN SHARE ROW EXCLUSIVE MODE;

-- 1. 预检：每平台至多一条活动规则。冲突不得自动停用，必须由维护者先人工收敛。
DO $$
DECLARE
  conflict RECORD;
BEGIN
  FOR conflict IN
    SELECT platform_id, count(*) AS enabled_count
    FROM storage_upload_rule
    WHERE is_enabled = 1 AND deleted_at IS NULL
    GROUP BY platform_id
    HAVING count(*) > 1
    ORDER BY platform_id
  LOOP
    RAISE EXCEPTION 'storage_upload_rule platform % has % enabled rules', conflict.platform_id, conflict.enabled_count;
  END LOOP;
END $$;

-- 2. 预检：全部历史编码行必须满足冻结的 code 契约，否则新 CHECK 无法建立。
DO $$
DECLARE
  conflict RECORD;
BEGIN
  FOR conflict IN
    SELECT rule_id, code
    FROM storage_upload_rule_code
    WHERE octet_length(code) NOT BETWEEN 1 AND 64
       OR code <> lower(btrim(code))
       OR code !~ '^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$'
       OR position('..' in code) > 0
       OR ('/' || code || '/') LIKE '%/.admin-storage/%'
    ORDER BY rule_id, code
  LOOP
    RAISE EXCEPTION 'storage_upload_rule_code rule % code % violates the frozen code contract', conflict.rule_id, conflict.code;
  END LOOP;
END $$;

-- 3. 预检：本协议不创建旧对象路由表；若它已存在说明库状态与预期不符。
DO $$
BEGIN
  IF to_regclass('storage_object_legacy_route') IS NOT NULL THEN
    RAISE EXCEPTION 'storage_object_legacy_route must not exist: the legacy object protocol is dropped';
  END IF;
END $$;

-- 4. 预检：品牌默认头像与 system.setting/global generation 必须各恰好一行（缺行时无法安全推进代际）。
DO $$
DECLARE
  brand_rows BIGINT;
  generation_rows BIGINT;
BEGIN
  SELECT count(*) INTO brand_rows FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL;
  IF brand_rows <> 1 THEN
    RAISE EXCEPTION 'app.brand.default_avatar must exist exactly once (found %)', brand_rows;
  END IF;
  SELECT count(*) INTO generation_rows FROM system_config_cache_generation WHERE namespace = 'system.setting' AND scope_key = 'global';
  IF generation_rows <> 1 THEN
    RAISE EXCEPTION 'system.setting/global generation row must exist exactly once (found %)', generation_rows;
  END IF;
END $$;

-- 5. 不可变物理位置版本表。
CREATE TABLE IF NOT EXISTS storage_cos_config_version (
  cos_config_id BIGINT NOT NULL,
  version BIGINT NOT NULL,
  bucket VARCHAR(128) NOT NULL,
  region VARCHAR(64) NOT NULL,
  endpoint VARCHAR(255) NULL,
  bucket_domain VARCHAR(255) NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT pk_storage_cos_config_version PRIMARY KEY (cos_config_id, version),
  CONSTRAINT fk_storage_cos_config_version_config
    FOREIGN KEY (cos_config_id) REFERENCES storage_cos_config(id) ON DELETE RESTRICT,
  CONSTRAINT ck_storage_cos_config_version_value CHECK (version >= 1)
);

-- 6. 在旧物理列仍存在时回填 revision 1（重跑不得静态引用已删除列）。
DO $outer$
DECLARE
  has_legacy_columns BOOLEAN;
BEGIN
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'storage_cos_config' AND column_name = 'bucket'
  ) INTO has_legacy_columns;
  IF has_legacy_columns THEN
    EXECUTE $backfill$
      INSERT INTO storage_cos_config_version (cos_config_id, version, bucket, region, endpoint, bucket_domain, created_at, updated_at)
      SELECT id, 1, bucket, region, endpoint, bucket_domain, created_at, updated_at
      FROM storage_cos_config
      ON CONFLICT (cos_config_id, version) DO NOTHING
    $backfill$;
  END IF;
END $outer$;

-- 7. 逻辑配置只保留 current_version 指针，不再保存物理字段。
ALTER TABLE storage_cos_config ADD COLUMN IF NOT EXISTS current_version BIGINT;
UPDATE storage_cos_config SET current_version = 1 WHERE current_version IS NULL;
ALTER TABLE storage_cos_config ALTER COLUMN current_version SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ck_storage_cos_config_current_version' AND conrelid = 'storage_cos_config'::regclass
  ) THEN
    ALTER TABLE storage_cos_config ADD CONSTRAINT ck_storage_cos_config_current_version CHECK (current_version >= 1);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_storage_cos_config_current_version' AND conrelid = 'storage_cos_config'::regclass
  ) THEN
    ALTER TABLE storage_cos_config ADD CONSTRAINT fk_storage_cos_config_current_version
      FOREIGN KEY (id, current_version) REFERENCES storage_cos_config_version(cos_config_id, version)
      DEFERRABLE INITIALLY DEFERRED;
  END IF;
END $$;

-- 8. 上传规则：每平台至多一条活动规则；管理 CRUD 为 last-write-wins，不引入 revision。
CREATE UNIQUE INDEX IF NOT EXISTS ux_storage_upload_rule_platform_enabled
  ON storage_upload_rule (platform_id)
  WHERE is_enabled = 1 AND deleted_at IS NULL;

-- 9. 编码子表：平台维度退化为规则维度，跨规则/跨平台允许重复。
ALTER TABLE storage_upload_rule_code ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
UPDATE storage_upload_rule_code SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE storage_upload_rule_code ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE storage_upload_rule_code DROP CONSTRAINT IF EXISTS fk_storage_upload_rule_code_platform;
DROP INDEX IF EXISTS ux_storage_upload_rule_code_platform_code;
ALTER TABLE storage_upload_rule_code DROP COLUMN IF EXISTS platform_id;

CREATE UNIQUE INDEX IF NOT EXISTS ux_storage_upload_rule_code_rule_code
  ON storage_upload_rule_code (rule_id, code)
  WHERE deleted_at IS NULL;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_storage_upload_rule_code_value' AND conrelid = 'storage_upload_rule_code'::regclass
      AND pg_get_constraintdef(oid) NOT LIKE '%octet_length%'
  ) THEN
    ALTER TABLE storage_upload_rule_code DROP CONSTRAINT ck_storage_upload_rule_code_value;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ck_storage_upload_rule_code_value' AND conrelid = 'storage_upload_rule_code'::regclass
  ) THEN
    ALTER TABLE storage_upload_rule_code ADD CONSTRAINT ck_storage_upload_rule_code_value CHECK (
      octet_length(code) BETWEEN 1 AND 64
      AND code = lower(btrim(code))
      AND code ~ '^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$'
      AND position('..' in code) = 0
      AND ('/' || code || '/') NOT LIKE '%/.admin-storage/%'
    );
  END IF;
END $$;

-- 10. 每个 COS 配置（含已软删行）seed 初始 generation 与同代 outbox。
INSERT INTO system_config_cache_generation (namespace, scope_key, generation)
SELECT 'storage.cosconfig', config.id::text, 1
FROM storage_cos_config AS config
ON CONFLICT (namespace, scope_key) DO NOTHING;

INSERT INTO system_config_cache_outbox (namespace, scope_key, generation)
SELECT generation.namespace, generation.scope_key, generation.generation
FROM system_config_cache_generation AS generation
WHERE generation.namespace = 'storage.cosconfig'
ON CONFLICT (namespace, scope_key, generation) DO NOTHING;

-- 11. 清空品牌默认头像旧引用；仅在实际清空时推进 system.setting generation 一次并写同代 outbox。
DO $$
DECLARE
  current_value TEXT;
  next_generation BIGINT;
BEGIN
  SELECT value INTO current_value
  FROM system_setting
  WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL
  FOR UPDATE;

  IF btrim(coalesce(current_value, '')) = '' THEN
    RETURN;
  END IF;

  UPDATE system_setting
  SET value = ''
  WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL;

  UPDATE system_config_cache_generation
  SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP
  WHERE namespace = 'system.setting' AND scope_key = 'global'
  RETURNING generation INTO next_generation;

  INSERT INTO system_config_cache_outbox (namespace, scope_key, generation)
  VALUES ('system.setting', 'global', next_generation)
  ON CONFLICT (namespace, scope_key, generation) DO NOTHING;
END $$;

-- 12. 清空用户头像旧引用（v2 key 之前的对象一律废弃，不调用 COS 删除）。
UPDATE user_profile SET avatar = '' WHERE avatar <> '';

-- 13. 最后删除旧物理列，使旧二进制的新写入只能明确失败。
ALTER TABLE storage_cos_config
  DROP COLUMN IF EXISTS bucket,
  DROP COLUMN IF EXISTS region,
  DROP COLUMN IF EXISTS endpoint,
  DROP COLUMN IF EXISTS bucket_domain;

-- 14. 收尾断言：每个配置都有当前物理版本、规则 revision 合法、且不存在旧对象路由表。
DO $$
DECLARE
  missing_versions BIGINT;
BEGIN
  SELECT count(*) INTO missing_versions
  FROM storage_cos_config AS config
  WHERE NOT EXISTS (
    SELECT 1 FROM storage_cos_config_version AS version
    WHERE version.cos_config_id = config.id AND version.version = config.current_version
  );
  IF missing_versions <> 0 THEN
    RAISE EXCEPTION 'storage_cos_config rows without current version: %', missing_versions;
  END IF;

  IF to_regclass('storage_object_legacy_route') IS NOT NULL THEN
    RAISE EXCEPTION 'storage_object_legacy_route must not exist after migration';
  END IF;
END $$;

COMMIT;
