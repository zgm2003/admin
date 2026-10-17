-- Maintainer-only P4 RBAC forward migration. Run in one transaction with API/Worker stopped.
-- This migration seeds only Admin PostgreSQL menu/role facts. Redis ready-state
-- synchronization must be performed by the maintenance runner before services start.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
SELECT pg_advisory_xact_lock(20261017, 402);
LOCK TABLE permission_auth_platform, permission_menu, permission_role,
    permission_role_menu, permission_user_role, permission_access_version,
    user_account IN SHARE ROW EXCLUSIVE MODE;

DO $migration$
DECLARE
    admin_id bigint;
    directory_id bigint;
    redeem_page_id bigint;
    wallet_page_id bigint;
    node_id bigint;
    grant_id bigint;
    active_grants integer;
    registered_role_id bigint;
    menu_changed boolean := false;
    grant_changed boolean := false;
    item record;
BEGIN
    IF (SELECT count(*) FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
        RAISE EXCEPTION 'exactly one active Admin platform is required';
    END IF;
    SELECT id INTO admin_id FROM permission_auth_platform
     WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL;

    SELECT id INTO directory_id FROM permission_menu
     WHERE platform_id=admin_id AND code='payment' AND deleted_at IS NULL;
    IF (SELECT count(*) FROM permission_menu WHERE platform_id=admin_id AND code='payment' AND deleted_at IS NULL) <> 1 THEN
        IF directory_id IS NULL THEN
            RAISE EXCEPTION 'payment directory must be seeded by payment configuration migration first';
        END IF;
        RAISE EXCEPTION 'payment directory is duplicated';
    END IF;
    IF directory_id IS NULL THEN
        RAISE EXCEPTION 'payment directory must be seeded by payment configuration migration first';
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id=directory_id AND
        (menu_type<>'directory' OR parent_id IS NOT NULL OR i18n_key IS DISTINCT FROM 'navigation.payment'
         OR path IS NOT NULL OR component_path IS NOT NULL OR icon IS DISTINCT FROM 'lucide:circle-dollar-sign'
         OR is_enabled<>1 OR is_hidden<>0)) THEN
        RAISE EXCEPTION 'payment directory has incompatible shape';
    END IF;

    -- Admin redeem-code management page.
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_id AND deleted_at IS NULL
        AND (path='/payment/redeemCode' OR component_path='payment/redeemCode')
        AND code<>'payment:redeemCode:view') THEN
        RAISE EXCEPTION 'payment redeem-code route conflicts with an existing menu';
    END IF;
    IF (SELECT count(*) FROM permission_menu WHERE platform_id=admin_id AND code='payment:redeemCode:view' AND deleted_at IS NULL) > 1 THEN
        RAISE EXCEPTION 'payment redeem-code page is duplicated';
    END IF;
    SELECT id INTO redeem_page_id FROM permission_menu
     WHERE platform_id=admin_id AND code='payment:redeemCode:view' AND deleted_at IS NULL;
    IF redeem_page_id IS NULL THEN
        INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at)
        VALUES(admin_id,directory_id,'page','兑换码','payment:redeemCode:view','navigation.paymentRedeemCode',
               '/payment/redeemCode','payment/redeemCode','lucide:circle-dollar-sign',60,1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
        RETURNING id INTO redeem_page_id;
        menu_changed := true;
    ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=redeem_page_id AND
        (parent_id IS DISTINCT FROM directory_id OR menu_type<>'page' OR i18n_key IS DISTINCT FROM 'navigation.paymentRedeemCode'
         OR path IS DISTINCT FROM '/payment/redeemCode' OR component_path IS DISTINCT FROM 'payment/redeemCode'
         OR icon IS DISTINCT FROM 'lucide:circle-dollar-sign' OR is_enabled<>1 OR is_hidden<>0)) THEN
        RAISE EXCEPTION 'payment redeem-code page has incompatible shape';
    END IF;

    FOR item IN SELECT * FROM (VALUES
        ('payment:redeemCode:list','查询兑换码'),
        ('payment:redeemCode:detail','兑换码详情'),
        ('payment:redeemCode:generate','生成兑换码'),
        ('payment:redeemCode:void','作废兑换码'),
        ('payment:redeemCode:export','导出兑换码')
    ) AS actions(code,name) LOOP
        IF (SELECT count(*) FROM permission_menu WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL) > 1 THEN
            RAISE EXCEPTION 'payment redeem-code action is duplicated: %', item.code;
        END IF;
        SELECT id INTO node_id FROM permission_menu
         WHERE platform_id=admin_id AND code=item.code AND deleted_at IS NULL;
        IF node_id IS NULL THEN
            INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,is_hidden,is_enabled,created_at,updated_at)
            VALUES(admin_id,redeem_page_id,'action',item.name,item.code,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
            menu_changed := true;
        ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=node_id AND
            (parent_id IS DISTINCT FROM redeem_page_id OR menu_type<>'action' OR path IS NOT NULL
             OR component_path IS NOT NULL OR i18n_key IS NOT NULL OR icon IS NOT NULL OR is_hidden<>1 OR is_enabled<>1)) THEN
            RAISE EXCEPTION 'payment redeem-code action has incompatible shape: %', item.code;
        END IF;
    END LOOP;

    -- Personal wallet is an independent root page; add the redeem action to it.
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_id AND deleted_at IS NULL
        AND (path='/payment/myWallet' OR component_path='payment/myWallet') AND code<>'payment:myWallet:view') THEN
        RAISE EXCEPTION 'personal wallet route conflicts with an existing menu';
    END IF;
    IF (SELECT count(*) FROM permission_menu WHERE platform_id=admin_id AND code='payment:myWallet:view' AND deleted_at IS NULL) > 1 THEN
        RAISE EXCEPTION 'personal wallet page is duplicated';
    END IF;
    SELECT id INTO wallet_page_id FROM permission_menu
     WHERE platform_id=admin_id AND code='payment:myWallet:view' AND deleted_at IS NULL;
    IF wallet_page_id IS NULL THEN
        INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at)
        VALUES(admin_id,NULL,'page','我的钱包','payment:myWallet:view','navigation.paymentMyWallet',
               '/payment/myWallet','payment/myWallet','lucide:circle-dollar-sign',90,1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
        RETURNING id INTO wallet_page_id;
        menu_changed := true;
    ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=wallet_page_id AND
        (parent_id IS NOT NULL OR menu_type<>'page' OR i18n_key IS DISTINCT FROM 'navigation.paymentMyWallet'
         OR path IS DISTINCT FROM '/payment/myWallet' OR component_path IS DISTINCT FROM 'payment/myWallet'
         OR icon IS DISTINCT FROM 'lucide:circle-dollar-sign' OR is_enabled<>1 OR is_hidden<>0)) THEN
        RAISE EXCEPTION 'personal wallet page has incompatible shape';
    END IF;
    IF (SELECT count(*) FROM permission_menu WHERE platform_id=admin_id AND code='payment:myWallet:redeem' AND deleted_at IS NULL) > 1 THEN
        RAISE EXCEPTION 'personal wallet redeem action is duplicated';
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=admin_id AND code='payment:myWallet:redeem' AND deleted_at IS NULL) THEN
        SELECT id INTO node_id FROM permission_menu
         WHERE platform_id=admin_id AND code='payment:myWallet:redeem' AND deleted_at IS NULL;
        IF EXISTS (SELECT 1 FROM permission_menu WHERE id=node_id AND
            (parent_id IS DISTINCT FROM wallet_page_id OR menu_type<>'action' OR path IS NOT NULL
             OR component_path IS NOT NULL OR i18n_key IS NOT NULL OR icon IS NOT NULL OR is_hidden<>1 OR is_enabled<>1)) THEN
            RAISE EXCEPTION 'personal wallet redeem action has incompatible shape';
        END IF;
    ELSE
        INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,is_hidden,is_enabled,created_at,updated_at)
        VALUES(admin_id,wallet_page_id,'action','兑换码充值','payment:myWallet:redeem',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
        RETURNING id INTO node_id;
        menu_changed := true;
    END IF;

    IF menu_changed THEN
        UPDATE permission_auth_platform SET menu_version=menu_version+1,updated_at=CURRENT_TIMESTAMP WHERE id=admin_id;
    END IF;

    SELECT id INTO registered_role_id FROM permission_role
     WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL;
    IF registered_role_id IS NULL OR (SELECT count(*) FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
        RAISE EXCEPTION 'exactly one active registered_user role is required';
    END IF;
    -- The page and existing personal actions are P2 grants. This migration must
    -- not silently turn an action grant into page access.
    IF NOT EXISTS (SELECT 1 FROM permission_role_menu WHERE role_id=registered_role_id AND menu_id=wallet_page_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'registered_user personal wallet page grant is required before redeem grant';
    END IF;
    SELECT count(*) INTO active_grants FROM permission_role_menu
     WHERE role_id=registered_role_id AND menu_id=node_id AND deleted_at IS NULL;
    IF active_grants > 1 THEN RAISE EXCEPTION 'registered_user redeem grant is duplicated'; END IF;
    IF active_grants = 0 THEN
        SELECT id INTO grant_id FROM permission_role_menu
         WHERE role_id=registered_role_id AND menu_id=node_id ORDER BY id DESC LIMIT 1;
        IF grant_id IS NULL THEN
            INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at)
            VALUES(registered_role_id,node_id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
        ELSE
            UPDATE permission_role_menu SET deleted_at=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=grant_id;
        END IF;
        grant_changed := true;
    END IF;
    IF grant_changed THEN
        IF EXISTS (
            SELECT 1 FROM permission_user_role ur
            JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
            LEFT JOIN permission_access_version av ON av.user_id=u.id
            WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL AND av.user_id IS NULL
        ) THEN RAISE EXCEPTION 'an active registered_user lacks an access version'; END IF;
        UPDATE permission_access_version av SET version=av.version+1,updated_at=CURRENT_TIMESTAMP
         WHERE av.user_id IN (
             SELECT ur.user_id FROM permission_user_role ur
             JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
             WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL
         );
        UPDATE permission_role SET updated_at=CURRENT_TIMESTAMP WHERE id=registered_role_id;
    END IF;
END $migration$;
