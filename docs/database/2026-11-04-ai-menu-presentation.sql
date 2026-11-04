-- Correct AI navigation presentation in the Admin platform.
-- Run with the maintenance runner while API, Worker and permission writers are stopped.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(hashtextextended('menu.ai.presentation.2026-11-04', 0));

CREATE TEMP TABLE _ai_menu_presentation_expected (
    code varchar(128) PRIMARY KEY,
    menu_type varchar(16) NOT NULL,
    parent_code varchar(128),
    icon varchar(128) NOT NULL,
    sort_order integer NOT NULL,
    i18n_key varchar(128)
) ON COMMIT DROP;

INSERT INTO _ai_menu_presentation_expected(code, menu_type, parent_code, icon, sort_order, i18n_key) VALUES
    ('ai', 'directory', NULL, 'lucide:sparkles', 900, 'navigation.ai'),
    ('ai:officialModel:view', 'page', 'ai', 'lucide:brain-circuit', 10, 'navigation.aiOfficialModel'),
    ('ai:provider:view', 'page', 'ai', 'lucide:server', 20, 'navigation.aiProvider'),
    ('ai:agent:view', 'page', 'ai', 'lucide:bot', 30, 'navigation.aiAgent'),
    ('ai:chat:view', 'page', 'ai', 'lucide:message-square-more', 40, 'navigation.aiChat'),
    ('ai:run:view', 'page', 'ai', 'lucide:activity', 50, 'navigation.aiRun'),
    ('ai:tool:view', 'page', 'ai', 'lucide:settings-2', 60, 'navigation.aiTool');

DO $migration$
DECLARE
    v_platform_id bigint;
    v_initial_version bigint;
    changed_rows integer;
BEGIN
    SELECT id, menu_version INTO v_platform_id, v_initial_version
    FROM permission_auth_platform
    WHERE code = 'admin' AND status = 1 AND deleted_at IS NULL
    FOR UPDATE;

    IF v_platform_id IS NULL OR v_initial_version IS NULL OR v_initial_version < 1 THEN
        RAISE EXCEPTION 'active Admin platform is missing';
    END IF;

    IF (SELECT count(*) FROM permission_menu
        WHERE permission_menu.platform_id = v_platform_id AND deleted_at IS NULL
          AND (code = 'ai' OR code LIKE 'ai:%')) <> 45 THEN
        RAISE EXCEPTION 'AI menu must contain the complete 45-node catalog';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM _ai_menu_presentation_expected expected
        LEFT JOIN permission_menu actual
          ON actual.platform_id = v_platform_id
         AND actual.code = expected.code
         AND actual.deleted_at IS NULL
        LEFT JOIN permission_menu parent ON parent.id = actual.parent_id
        WHERE actual.id IS NULL
           OR actual.menu_type IS DISTINCT FROM expected.menu_type
           OR actual.i18n_key IS DISTINCT FROM expected.i18n_key
           OR actual.status <> 1
           OR (expected.parent_code IS NULL AND actual.parent_id IS NOT NULL)
           OR (expected.parent_code IS NOT NULL AND parent.code IS DISTINCT FROM expected.parent_code)
    ) THEN
        RAISE EXCEPTION 'AI menu identity or structure drifted';
    END IF;

    UPDATE permission_menu actual
       SET icon = expected.icon,
           sort_order = expected.sort_order,
           updated_at = CURRENT_TIMESTAMP
      FROM _ai_menu_presentation_expected expected
     WHERE actual.platform_id = v_platform_id
       AND actual.code = expected.code
       AND actual.deleted_at IS NULL
       AND (actual.icon IS DISTINCT FROM expected.icon
         OR actual.sort_order IS DISTINCT FROM expected.sort_order);
    GET DIAGNOSTICS changed_rows = ROW_COUNT;

    IF changed_rows > 0 THEN
        IF v_initial_version = 9223372036854775807 THEN
            RAISE EXCEPTION 'Admin menu version overflow';
        END IF;
        UPDATE permission_auth_platform
           SET menu_version = v_initial_version + 1,
               updated_at = CURRENT_TIMESTAMP
         WHERE id = v_platform_id AND menu_version = v_initial_version;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'Admin menu version changed during presentation migration';
        END IF;
    END IF;
END
$migration$;
