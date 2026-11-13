-- Expand the general upload rule with common text, office, and archive types.
-- Existing files and other upload rules (for example the restricted canvas rule)
-- are left untouched.
UPDATE storage_upload_rule AS r
SET allowed_extensions = ARRAY[
        'jpg', 'jpeg', 'png', 'gif', 'webp',
        'pdf', 'doc', 'docx', 'xls', 'xlsx', 'csv', 'ppt', 'pptx',
        'txt', 'md', 'json', 'xml', 'yaml', 'yml', 'log',
        'zip', 'rar', '7z'
    ]::text[],
    allowed_mime_types = ARRAY[
        'image/jpeg', 'image/png', 'image/gif', 'image/webp',
        'application/pdf', 'application/msword',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'application/vnd.ms-excel',
        'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        'text/csv', 'application/vnd.ms-powerpoint',
        'application/vnd.openxmlformats-officedocument.presentationml.presentation',
        'text/plain', 'text/markdown', 'application/json',
        'application/xml', 'text/xml', 'application/x-yaml', 'text/yaml',
        'application/zip', 'application/vnd.rar', 'application/x-rar-compressed',
        'application/x-7z-compressed'
    ]::text[],
    updated_at = CURRENT_TIMESTAMP
WHERE r.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM storage_upload_rule_code AS c
      WHERE c.rule_id = r.id
        AND c.code = 'file'
        AND c.deleted_at IS NULL
  );
