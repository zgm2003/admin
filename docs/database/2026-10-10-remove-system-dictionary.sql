-- Forward-only removal. Execute only with API and Worker stopped, via the same-name runner.
-- PostgreSQL commit and Redis cleanup are separate; the runner records partial failure.
-- No CASCADE: an unexpected database dependency aborts the entire transaction.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(20261010, 1001);
LOCK TABLE permission_auth_platform, permission_menu, permission_role_menu IN SHARE ROW EXCLUSIVE MODE;
CREATE TEMP TABLE remove_dictionary_menu ON COMMIT DROP AS
 SELECT id, platform_id FROM permission_menu
 WHERE code LIKE 'system:dictionary:%'
    OR path = '/system/dictionary' OR component_path = 'system/dictionary';
DO $check$
BEGIN
 IF EXISTS (SELECT 1 FROM permission_menu child JOIN remove_dictionary_menu parent ON child.parent_id=parent.id
            WHERE NOT EXISTS (SELECT 1 FROM remove_dictionary_menu target WHERE target.id=child.id)) THEN
  RAISE EXCEPTION 'dictionary menu has unrelated descendant; removal aborted';
 END IF;
 IF EXISTS (SELECT 1 FROM remove_dictionary_menu target LEFT JOIN permission_auth_platform p ON p.id=target.platform_id
            WHERE p.id IS NULL OR p.menu_version < 1) THEN
  RAISE EXCEPTION 'dictionary platform menu version is invalid';
 END IF;
END;
$check$;
-- Both active and previously soft-deleted relations disappear with their resource.
DELETE FROM permission_role_menu WHERE menu_id IN (SELECT id FROM remove_dictionary_menu);
-- Delete leaves before parents, preserving action shape checks and RESTRICT self-FKs.
DO $remove$
DECLARE removed_count bigint;
BEGIN
 LOOP
  DELETE FROM permission_menu target WHERE target.id IN (SELECT id FROM remove_dictionary_menu)
   AND NOT EXISTS (SELECT 1 FROM permission_menu child WHERE child.parent_id=target.id);
  GET DIAGNOSTICS removed_count = ROW_COUNT;
  EXIT WHEN NOT EXISTS (SELECT 1 FROM permission_menu WHERE id IN (SELECT id FROM remove_dictionary_menu));
  IF removed_count=0 THEN RAISE EXCEPTION 'dictionary menu cycle or unrelated descendant; removal aborted'; END IF;
 END LOOP;
END;
$remove$;
UPDATE permission_auth_platform SET menu_version=menu_version+1, updated_at=CURRENT_TIMESTAMP
 WHERE id IN (SELECT DISTINCT platform_id FROM remove_dictionary_menu);
-- Do not remove any other namespace or scope, including dictionary test/non-global scopes.
DELETE FROM system_config_cache_outbox WHERE namespace='system.dictionary' AND scope_key='global';
DELETE FROM system_config_cache_generation WHERE namespace='system.dictionary' AND scope_key='global';
DROP TABLE IF EXISTS system_dictionary_item;
DROP TABLE IF EXISTS system_dictionary;
DO $verify$
BEGIN
 IF EXISTS (SELECT 1 FROM permission_menu WHERE code LIKE 'system:dictionary:%' OR path='/system/dictionary' OR component_path='system/dictionary') THEN
  RAISE EXCEPTION 'dictionary menus remain';
 END IF;
 IF EXISTS (SELECT 1 FROM system_config_cache_generation WHERE namespace='system.dictionary' AND scope_key='global')
    OR EXISTS (SELECT 1 FROM system_config_cache_outbox WHERE namespace='system.dictionary' AND scope_key='global') THEN
  RAISE EXCEPTION 'dictionary generation facts remain';
 END IF;
 RAISE NOTICE 'dictionary removal: platforms=%, menus=%',
  (SELECT count(DISTINCT platform_id) FROM remove_dictionary_menu),(SELECT count(*) FROM remove_dictionary_menu);
END;
$verify$;
COMMIT;
