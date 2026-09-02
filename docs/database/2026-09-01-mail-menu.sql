BEGIN;
WITH p AS (SELECT id FROM auth_platform WHERE code='admin' AND deleted_at IS NULL), d AS (SELECT id,platform_id FROM permission_menu WHERE code='message' AND menu_type='directory' AND deleted_at IS NULL)
INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,sort_order,is_enabled,is_hidden)
SELECT p.id,NULL,'directory','消息服务','message','navigation.message',NULL,NULL,80,1,0 FROM p WHERE NOT EXISTS (SELECT 1 FROM d WHERE d.platform_id=p.id);
INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,sort_order,is_enabled,is_hidden)
SELECT p.id,d.id,'page','邮件服务','system:mail:view','navigation.mail','/message/mail','message/mail',10,1,0 FROM auth_platform p JOIN permission_menu d ON d.platform_id=p.id AND d.code='message' AND d.menu_type='directory' AND d.deleted_at IS NULL WHERE p.code='admin' AND p.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM permission_menu m WHERE m.platform_id=p.id AND m.code='system:mail:view' AND m.deleted_at IS NULL);
WITH a(code,name) AS (VALUES ('system:mail:list','读取邮件服务'),('system:mail:detail','读取邮件详情'),('system:mail:config:update','修改邮件配置'),('system:mail:config:delete','删除邮件配置'),('system:mail:test','发送测试邮件'),('system:mail:template:update','修改邮件模板'),('system:mail:template:status','启停邮件模板'),('system:mail:log:delete','删除邮件日志'),('system:mail:rule:create','新增收件人规则'),('system:mail:rule:update','修改收件人规则'),('system:mail:rule:status','启停收件人规则'),('system:mail:rule:delete','删除收件人规则'))
INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,sort_order,is_enabled,is_hidden)
SELECT p.id,m.id,'action',a.name,a.code,10,1,1 FROM auth_platform p JOIN permission_menu m ON m.platform_id=p.id AND m.code='system:mail:view' AND m.deleted_at IS NULL CROSS JOIN a WHERE p.code='admin' AND p.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM permission_menu x WHERE x.platform_id=p.id AND x.code=a.code AND x.deleted_at IS NULL);
COMMIT;
