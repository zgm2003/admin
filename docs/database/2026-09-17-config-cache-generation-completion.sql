-- 配置缓存代际协议完成迁移。
-- 目标：建立 Dictionary/Mail/SMS 固定 generation scope，并删除 Mail/SMS 管理端乐观锁列。
-- 本文件只修改 PostgreSQL；Redis 旧协议 key 由同名 PowerShell runner 的固定清理命令处理。
-- 幂等：重复执行不推进 generation、不新增 outbox、不修改业务事实或时间戳。

BEGIN;

SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '60s';

LOCK TABLE permission_auth_platform IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE message_mail_rate_limit_policy, message_sms_rate_limit_policy IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE system_config_cache_generation, system_config_cache_outbox IN SHARE ROW EXCLUSIVE MODE;

-- 每个未删除认证平台必须恰好具有两条 Mail 固定策略，且 key/模式/维度完全符合最终契约。
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM permission_auth_platform AS platform
    LEFT JOIN message_mail_rate_limit_policy AS policy ON policy.platform_id = platform.id
    WHERE platform.deleted_at IS NULL
    GROUP BY platform.id
    HAVING count(policy.platform_id) <> 2
       OR count(*) FILTER (WHERE policy.policy_key = 'business_email_minute') <> 1
       OR count(*) FILTER (WHERE policy.policy_key = 'business_email_10m') <> 1
       OR count(*) FILTER (
            WHERE policy.platform_id IS NOT NULL
              AND (policy.mode <> 'business' OR policy.dimension <> 'platform_email')
          ) <> 0
  ) THEN
    RAISE EXCEPTION 'mail rate limit policies must contain exactly the two fixed business/platform_email rows for every active platform';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM message_mail_rate_limit_policy AS policy
    JOIN permission_auth_platform AS platform ON platform.id = policy.platform_id AND platform.deleted_at IS NULL
    WHERE policy.limit_count < 1 OR policy.limit_count > 100000
       OR policy.window_seconds < 1 OR policy.window_seconds > 86400
  ) THEN
    RAISE EXCEPTION 'mail rate limit policies contain invalid values';
  END IF;
END
$$;

-- 每个未删除认证平台必须恰好具有两条 SMS 固定策略，且 key/模式/维度完全符合最终契约。
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM permission_auth_platform AS platform
    LEFT JOIN message_sms_rate_limit_policy AS policy ON policy.platform_id = platform.id
    WHERE platform.deleted_at IS NULL
    GROUP BY platform.id
    HAVING count(policy.platform_id) <> 2
       OR count(*) FILTER (WHERE policy.policy_key = 'business_phone_minute') <> 1
       OR count(*) FILTER (WHERE policy.policy_key = 'business_phone_10m') <> 1
       OR count(*) FILTER (
            WHERE policy.platform_id IS NOT NULL
              AND (policy.mode <> 'business' OR policy.dimension <> 'platform_phone')
          ) <> 0
  ) THEN
    RAISE EXCEPTION 'sms rate limit policies must contain exactly the two fixed business/platform_phone rows for every active platform';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM message_sms_rate_limit_policy AS policy
    JOIN permission_auth_platform AS platform ON platform.id = policy.platform_id AND platform.deleted_at IS NULL
    WHERE policy.limit_count < 1 OR policy.limit_count > 100000
       OR policy.window_seconds < 1 OR policy.window_seconds > 86400
  ) THEN
    RAISE EXCEPTION 'sms rate limit policies contain invalid values';
  END IF;
END
$$;

INSERT INTO system_config_cache_generation (namespace, scope_key, generation)
VALUES
  ('system.dictionary', 'global', 1),
  ('message.mail', 'global', 1),
  ('message.sms', 'global', 1)
ON CONFLICT (namespace, scope_key) DO NOTHING;

INSERT INTO system_config_cache_outbox (namespace, scope_key, generation)
SELECT generation.namespace, generation.scope_key, generation.generation
FROM system_config_cache_generation AS generation
WHERE generation.scope_key = 'global'
  AND generation.namespace IN ('system.dictionary', 'message.mail', 'message.sms')
ON CONFLICT (namespace, scope_key, generation) DO NOTHING;

DROP INDEX IF EXISTS ix_message_mail_rate_limit_policy_platform_revision;
ALTER TABLE message_mail_rate_limit_policy
  DROP CONSTRAINT IF EXISTS ck_message_mail_rate_limit_policy_revision,
  DROP COLUMN IF EXISTS revision;

DROP INDEX IF EXISTS ix_message_sms_rate_limit_policy_platform_revision;
ALTER TABLE message_sms_rate_limit_policy
  DROP CONSTRAINT IF EXISTS ck_message_sms_rate_limit_policy_revision,
  DROP COLUMN IF EXISTS revision;

COMMIT;
