-- 数据迁移：把历史上以权限码作为显示名的动作节点改成可读中文名。
-- 只更新 Admin 平台已存在的 action 行；不新增/删除权限、不授权、不动菜单版本。
-- 执行方式：psql -1 -f docs/database/2026-11-09-permission-action-labels.sql
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
LOCK TABLE permission_menu IN SHARE ROW EXCLUSIVE MODE;

DO $migration$
DECLARE
  admin_id BIGINT;
  label RECORD;
  affected INTEGER;
BEGIN
  SELECT id INTO admin_id
    FROM permission_auth_platform
   WHERE code = 'admin' AND status = 1 AND deleted_at IS NULL;

  IF admin_id IS NULL
     OR (SELECT count(*) FROM permission_auth_platform
          WHERE code = 'admin' AND status = 1 AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'exactly one active Admin platform is required';
  END IF;

  FOR label IN
    SELECT * FROM (VALUES
      ('message:sms:list',               '查看短信列表'),
      ('message:sms:detail',             '查看短信详情'),
      ('message:sms:config:update',      '修改短信配置'),
      ('message:sms:config:delete',      '删除短信配置'),
      ('message:sms:test',               '发送测试短信'),
      ('message:sms:template:update',    '修改短信模板'),
      ('message:sms:template:status',    '修改短信模板状态'),
      ('message:sms:rule:create',        '新增短信规则'),
      ('message:sms:rule:update',        '修改短信规则'),
      ('message:sms:rule:status',        '修改短信规则状态'),
      ('message:sms:rule:delete',        '删除短信规则'),
      ('message:sms:rate-limit:update',  '修改短信限流设置'),
      ('system:setting:list',            '查看设置列表'),
      ('system:setting:detail',          '查看设置详情'),
      ('system:setting:create',          '新增设置'),
      ('system:setting:update',          '修改设置'),
      ('system:setting:status',          '修改设置状态'),
      ('system:setting:delete',          '删除设置'),
      ('user:email:update',              '绑定或换绑邮箱'),
      ('user:phone:update',              '绑定或换绑手机')
    ) AS labels(code, name)
  LOOP
    UPDATE permission_menu
       SET name = label.name,
           remark = '控制“' || label.name || '”对应的具体操作。',
           updated_at = CURRENT_TIMESTAMP
     WHERE platform_id = admin_id
       AND menu_type = 'action'
       AND code = label.code
       AND deleted_at IS NULL;

    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
      RAISE EXCEPTION 'permission action % has % active rows, want 1', label.code, affected;
    END IF;
  END LOOP;
END $migration$;
