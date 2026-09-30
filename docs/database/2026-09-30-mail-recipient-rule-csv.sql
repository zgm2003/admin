-- Offline forward seed. Run the same-name PowerShell runner with API/Worker stopped.
-- Adds two independent Admin actions and an unset, global HTTPS template setting.
-- Does not grant roles, touch Canvas, modify recipient rules or reset send quotas.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
LOCK TABLE permission_auth_platform, permission_menu, system_setting,
  system_config_cache_generation, system_config_cache_outbox IN SHARE ROW EXCLUSIVE MODE;

DO $$
DECLARE
  admin_id bigint;
  page_id bigint;
  action_row record;
  existing_row record;
  menu_changed boolean := false;
  setting_generation bigint;
BEGIN
  SELECT id INTO STRICT admin_id FROM permission_auth_platform
    WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL;
  SELECT id INTO STRICT page_id FROM permission_menu
    WHERE platform_id=admin_id AND code='message:mail:view' AND deleted_at IS NULL;
  IF EXISTS (SELECT 1 FROM permission_menu WHERE id=page_id AND (
    menu_type<>'page' OR path IS DISTINCT FROM '/message/mail' OR
    component_path IS DISTINCT FROM 'message/mail' OR
    i18n_key IS DISTINCT FROM 'navigation.mail' OR is_enabled<>1
  )) THEN
    RAISE EXCEPTION 'message:mail:view page shape conflict';
  END IF;

  FOR action_row IN SELECT * FROM (VALUES
    ('message:mail:rule:import', '导入收件规则 CSV', 110),
    ('message:mail:rule:export', '导出收件规则 CSV', 120)
  ) AS action(code,name,sort_order) LOOP
    SELECT * INTO existing_row FROM permission_menu
      WHERE platform_id=admin_id AND code=action_row.code AND deleted_at IS NULL;
    IF FOUND THEN
      IF existing_row.menu_type<>'action' OR existing_row.parent_id IS DISTINCT FROM page_id OR
        existing_row.path IS NOT NULL OR existing_row.component_path IS NOT NULL OR
        existing_row.icon IS NOT NULL OR existing_row.i18n_key IS NOT NULL OR
        existing_row.is_hidden<>1 THEN
        RAISE EXCEPTION 'recipient-rule CSV action shape conflict: %', action_row.code;
      END IF;
      -- Preserve any explicit administrator status/name/sort and all grants on reruns.
    ELSE
      INSERT INTO permission_menu (platform_id,parent_id,menu_type,name,code,
        sort_order,is_enabled,is_hidden,created_at,updated_at)
      VALUES (admin_id,page_id,'action',action_row.name,action_row.code,
        action_row.sort_order,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
      menu_changed := true;
    END IF;
  END LOOP;
  IF menu_changed THEN
    UPDATE permission_auth_platform SET menu_version=menu_version+1,
      updated_at=CURRENT_TIMESTAMP WHERE id=admin_id;
  END IF;

  SELECT * INTO existing_row FROM system_setting
    WHERE setting_key='message.mail.recipient_rule.import_template_url' AND deleted_at IS NULL;
  IF FOUND THEN
    IF existing_row.value_type<>1 OR existing_row.is_enabled<>1 OR existing_row.is_builtin<>1 THEN
      RAISE EXCEPTION 'recipient-rule CSV template setting shape conflict';
    END IF;
    -- Never overwrite a previously configured COS URL or manufacture one.
  ELSE
    INSERT INTO system_setting (setting_key,value,value_type,description,
      is_enabled,is_builtin,created_at,updated_at)
    VALUES ('message.mail.recipient_rule.import_template_url','',1,
      '收件规则 CSV 模板下载链接；上传模板后填写 HTTPS 地址，空值表示未配置',
      1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
    UPDATE system_config_cache_generation SET generation=generation+1,
      updated_at=CURRENT_TIMESTAMP WHERE namespace='system.setting' AND scope_key='global'
      RETURNING generation INTO setting_generation;
    IF setting_generation IS NULL THEN
      RAISE EXCEPTION 'system.setting/global generation is required';
    END IF;
    INSERT INTO system_config_cache_outbox (namespace,scope_key,generation,attempts,
      available_at,created_at,updated_at)
    VALUES ('system.setting','global',setting_generation,0,
      CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
  END IF;
END $$;
COMMIT;
