BEGIN;

-- `code` is the canonical business permission code and COS object prefix.
-- Existing code values are preserved; path_prefix becomes obsolete.
DROP INDEX IF EXISTS ux_storage_upload_rule_platform_enabled;

ALTER TABLE storage_upload_rule
  DROP CONSTRAINT IF EXISTS ck_storage_upload_rule_max_file_count,
  DROP COLUMN IF EXISTS path_prefix,
  DROP COLUMN IF EXISTS max_file_count;

COMMIT;
