-- Maintainer-only P2 payment menu forward migration.
-- Run in one transaction with API/Worker stopped. This seeds the pages and
-- actions implemented by the P2 API, plus the two personal entry points.
-- It deliberately does not create refund, AI debit, freeze, or admin top-up
-- capabilities.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
SELECT pg_advisory_xact_lock(20261017, 401);
LOCK TABLE permission_auth_platform, permission_menu, permission_role,
    permission_role_menu, permission_user_role, permission_access_version,
    user_account IN SHARE ROW EXCLUSIVE MODE;

DO $migration$
DECLARE
    admin_id bigint;
    directory_id bigint;
    page_id bigint;
    node_id bigint;
    grant_id bigint;
    active_count integer;
    registered_role_id bigint;
    menu_changed boolean := false;
    grant_changed boolean := false;
    item record;
BEGIN
    IF (SELECT count(*) FROM permission_auth_platform
        WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
        RAISE EXCEPTION 'exactly one active Admin platform is required';
    END IF;
    SELECT id INTO admin_id FROM permission_auth_platform
     WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL;

    SELECT id INTO directory_id FROM permission_menu
     WHERE platform_id=admin_id AND code='payment' AND deleted_at IS NULL;
    IF (SELECT count(*) FROM permission_menu
        WHERE platform_id=admin_id AND code='payment' AND deleted_at IS NULL) <> 1 THEN
        RAISE EXCEPTION 'exactly one active payment directory is required';
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id=directory_id AND
        (menu_type<>'directory' OR parent_id IS NOT NULL OR path IS NOT NULL
         OR component_path IS NOT NULL OR i18n_key IS DISTINCT FROM 'navigation.payment'
         OR icon IS DISTINCT FROM 'lucide:circle-dollar-sign'
         OR is_enabled<>1 OR is_hidden<>0)) THEN
        RAISE EXCEPTION 'payment directory has incompatible shape';
    END IF;

    -- Page routes are rejected if another active menu already owns their path.
    IF EXISTS (
        SELECT 1 FROM permission_menu
        WHERE platform_id=admin_id AND deleted_at IS NULL
          AND (path IN ('/payment/rechargePackage','/payment/order','/payment/wallet',
                        '/payment/transaction','/payment/recharge')
               OR component_path IN ('payment/rechargePackage','payment/order','payment/wallet',
                                     'payment/transaction','payment/recharge'))
          AND code NOT IN ('payment:rechargePackage:view','payment:order:view',
                           'payment:wallet:view','payment:transaction:view','payment:recharge:view')
    ) THEN
        RAISE EXCEPTION 'payment P2 route conflicts with an existing menu';
    END IF;

    FOR item IN SELECT * FROM (VALUES
        ('payment:rechargePackage:view','充值套餐','navigation.paymentRechargePackage',
         '/payment/rechargePackage','payment/rechargePackage',10,true),
        ('payment:order:view','支付订单','navigation.paymentOrder',
         '/payment/order','payment/order',20,true),
        ('payment:wallet:view','用户钱包','navigation.paymentWallet',
         '/payment/wallet','payment/wallet',30,true),
        ('payment:transaction:view','资金流水','navigation.paymentTransaction',
         '/payment/transaction','payment/transaction',40,true),
        ('payment:recharge:view','充值中心','navigation.paymentRecharge',
         '/payment/recharge','payment/recharge',80,false),
        ('payment:myWallet:view','我的钱包','navigation.paymentMyWallet',
         '/payment/myWallet','payment/myWallet',90,false)
    ) AS pages(code,name,i18n_key,path,component_path,sort_order,under_directory) LOOP
        IF (SELECT count(*) FROM permission_menu
            WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL) > 1 THEN
            RAISE EXCEPTION 'payment page is duplicated: %', item.code;
        END IF;
        SELECT id INTO page_id FROM permission_menu
         WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL;
        IF page_id IS NULL THEN
            INSERT INTO permission_menu(
                platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,
                icon,sort_order,is_enabled,is_hidden,created_at,updated_at
            )
            VALUES(
                admin_id,CASE WHEN item.under_directory THEN directory_id ELSE NULL END,
                'page',item.name,item.code,item.i18n_key,item.path,item.component_path,
                'lucide:circle-dollar-sign',item.sort_order,1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
            ) RETURNING id INTO page_id;
            menu_changed := true;
        ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=page_id AND
            (parent_id IS DISTINCT FROM CASE WHEN item.under_directory THEN directory_id ELSE NULL END
             OR menu_type<>'page' OR name<>item.name OR i18n_key IS DISTINCT FROM item.i18n_key
             OR path IS DISTINCT FROM item.path OR component_path IS DISTINCT FROM item.component_path
             OR icon IS DISTINCT FROM 'lucide:circle-dollar-sign' OR sort_order<>item.sort_order
             OR is_enabled<>1 OR is_hidden<>0)) THEN
            RAISE EXCEPTION 'payment page has incompatible shape: %', item.code;
        END IF;
    END LOOP;

    FOR item IN SELECT * FROM (VALUES
        ('payment:rechargePackage:view','payment:rechargePackage:list','查询充值套餐'),
        ('payment:rechargePackage:view','payment:rechargePackage:detail','充值套餐详情'),
        ('payment:rechargePackage:view','payment:rechargePackage:create','创建充值套餐'),
        ('payment:rechargePackage:view','payment:rechargePackage:update','更新充值套餐'),
        ('payment:rechargePackage:view','payment:rechargePackage:status','启停充值套餐'),
        ('payment:rechargePackage:view','payment:rechargePackage:delete','删除充值套餐'),
        ('payment:order:view','payment:order:list','查询支付订单'),
        ('payment:order:view','payment:order:detail','支付订单详情'),
        ('payment:order:view','payment:callbackEvent:list','查询回调事件'),
        ('payment:order:view','payment:callbackEvent:detail','回调事件详情'),
        ('payment:wallet:view','payment:wallet:list','查询用户钱包'),
        ('payment:wallet:view','payment:wallet:detail','用户钱包详情'),
        ('payment:transaction:view','payment:transaction:list','查询资金流水'),
        ('payment:transaction:view','payment:transaction:detail','资金流水详情'),
        ('payment:myWallet:view','payment:myWallet:detail','我的钱包详情'),
        ('payment:myWallet:view','payment:myWallet:list','我的钱包流水'),
        ('payment:recharge:view','payment:recharge:list','查询我的订单'),
        ('payment:recharge:view','payment:recharge:detail','我的订单详情'),
        ('payment:recharge:view','payment:recharge:create','创建充值订单'),
        ('payment:recharge:view','payment:recharge:pay','发起充值支付')
    ) AS actions(page_code,code,name) LOOP
        SELECT id INTO page_id FROM permission_menu
         WHERE platform_id=admin_id AND code=item.page_code AND deleted_at IS NULL;
        IF page_id IS NULL THEN
            RAISE EXCEPTION 'parent page missing for payment action: %', item.code;
        END IF;
        IF (SELECT count(*) FROM permission_menu
            WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL) > 1 THEN
            RAISE EXCEPTION 'payment action is duplicated: %', item.code;
        END IF;
        SELECT id INTO node_id FROM permission_menu
         WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL;
        IF node_id IS NULL THEN
            INSERT INTO permission_menu(
                platform_id,parent_id,menu_type,name,code,is_hidden,is_enabled,created_at,updated_at
            ) VALUES(admin_id,page_id,'action',item.name,item.code,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            menu_changed := true;
        ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=node_id AND
            (parent_id IS DISTINCT FROM page_id OR menu_type<>'action' OR name<>item.name
             OR path IS NOT NULL OR component_path IS NOT NULL OR i18n_key IS NOT NULL
             OR icon IS NOT NULL OR is_hidden<>1 OR is_enabled<>1)) THEN
            RAISE EXCEPTION 'payment action has incompatible shape: %', item.code;
        END IF;
    END LOOP;

    IF menu_changed THEN
        UPDATE permission_auth_platform
           SET menu_version=menu_version+1,updated_at=CURRENT_TIMESTAMP
         WHERE id=admin_id;
    END IF;

    SELECT id INTO registered_role_id FROM permission_role
     WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL;
    IF registered_role_id IS NULL OR (SELECT count(*) FROM permission_role
        WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
        RAISE EXCEPTION 'exactly one active registered_user role is required';
    END IF;

    -- Personal pages and actions are explicit grants; management pages remain
    -- available only through the existing role authorization workflow.
    FOR item IN SELECT code FROM (VALUES
        ('payment:myWallet:view'),('payment:myWallet:detail'),('payment:myWallet:list'),
        ('payment:recharge:view'),('payment:recharge:list'),('payment:recharge:detail'),
        ('payment:recharge:create'),('payment:recharge:pay')
    ) AS grants(code) LOOP
        SELECT id INTO node_id FROM permission_menu
         WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL;
        IF node_id IS NULL THEN
            RAISE EXCEPTION 'personal payment grant menu missing: %', item.code;
        END IF;
        SELECT count(*) INTO active_count FROM permission_role_menu
         WHERE role_id=registered_role_id AND menu_id=node_id AND deleted_at IS NULL;
        IF active_count > 1 THEN
            RAISE EXCEPTION 'personal payment grant is duplicated: %', item.code;
        ELSIF active_count = 0 THEN
            SELECT id INTO grant_id FROM permission_role_menu
             WHERE role_id=registered_role_id AND menu_id=node_id
             ORDER BY id DESC LIMIT 1;
            IF grant_id IS NULL THEN
                INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at)
                VALUES(registered_role_id,node_id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            ELSE
                UPDATE permission_role_menu
                   SET deleted_at=NULL,updated_at=CURRENT_TIMESTAMP
                 WHERE id=grant_id;
            END IF;
            grant_changed := true;
        END IF;
    END LOOP;

    IF grant_changed THEN
        IF EXISTS (
            SELECT 1 FROM permission_user_role ur
            JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
            LEFT JOIN permission_access_version av ON av.user_id=u.id
            WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL AND av.user_id IS NULL
        ) THEN
            RAISE EXCEPTION 'an active registered_user lacks an access version';
        END IF;
        UPDATE permission_access_version av
           SET version=av.version+1,updated_at=CURRENT_TIMESTAMP
         WHERE av.user_id IN (
             SELECT ur.user_id FROM permission_user_role ur
             JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
             WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL
         );
        UPDATE permission_role SET updated_at=CURRENT_TIMESTAMP WHERE id=registered_role_id;
    END IF;
END $migration$;
