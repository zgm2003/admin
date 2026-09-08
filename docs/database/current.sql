/*
 Navicat Premium Dump SQL

 Source Server         : localhost
 Source Server Type    : PostgreSQL
 Source Server Version : 180006 (180006)
 Source Host           : localhost:5432
 Source Catalog        : admin
 Source Schema         : public

 Target Server Type    : PostgreSQL
 Target Server Version : 180006 (180006)
 File Encoding         : 65001

 Date: 07/09/2026 18:49:15
*/


-- ----------------------------
-- Sequence structure for audit_operation_log_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."audit_operation_log_id_seq";
CREATE SEQUENCE "public"."audit_operation_log_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for auth_platform_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."auth_platform_id_seq";
CREATE SEQUENCE "public"."auth_platform_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for auth_session_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."auth_session_id_seq";
CREATE SEQUENCE "public"."auth_session_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for message_mail_config_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."message_mail_config_id_seq";
CREATE SEQUENCE "public"."message_mail_config_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for message_mail_log_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."message_mail_log_id_seq";
CREATE SEQUENCE "public"."message_mail_log_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for message_mail_log_verification_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."message_mail_log_verification_id_seq";
CREATE SEQUENCE "public"."message_mail_log_verification_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for message_mail_recipient_rule_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."message_mail_recipient_rule_id_seq";
CREATE SEQUENCE "public"."message_mail_recipient_rule_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for message_mail_template_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."message_mail_template_id_seq";
CREATE SEQUENCE "public"."message_mail_template_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for rbac_access_version_user_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."rbac_access_version_user_id_seq";
CREATE SEQUENCE "public"."rbac_access_version_user_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for rbac_menu_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."rbac_menu_id_seq";
CREATE SEQUENCE "public"."rbac_menu_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for rbac_role_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."rbac_role_id_seq";
CREATE SEQUENCE "public"."rbac_role_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for rbac_role_menu_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."rbac_role_menu_id_seq";
CREATE SEQUENCE "public"."rbac_role_menu_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for rbac_user_role_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."rbac_user_role_id_seq";
CREATE SEQUENCE "public"."rbac_user_role_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for storage_cos_config_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."storage_cos_config_id_seq";
CREATE SEQUENCE "public"."storage_cos_config_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for storage_upload_rule_code_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."storage_upload_rule_code_id_seq";
CREATE SEQUENCE "public"."storage_upload_rule_code_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for storage_upload_rule_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."storage_upload_rule_id_seq";
CREATE SEQUENCE "public"."storage_upload_rule_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for user_account_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."user_account_id_seq";
CREATE SEQUENCE "public"."user_account_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Sequence structure for user_login_log_id_seq
-- ----------------------------
DROP SEQUENCE IF EXISTS "public"."user_login_log_id_seq";
CREATE SEQUENCE "public"."user_login_log_id_seq" 
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1;

-- ----------------------------
-- Table structure for auth_platform
-- ----------------------------
DROP TABLE IF EXISTS "public"."auth_platform";
CREATE TABLE "public"."auth_platform" (
  "id" int8 NOT NULL DEFAULT nextval('auth_platform_id_seq'::regclass),
  "code" varchar(49) COLLATE "pg_catalog"."default" NOT NULL,
  "name" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "policy_version" int8 NOT NULL DEFAULT 1,
  "access_ttl_seconds" int4 NOT NULL,
  "refresh_ttl_seconds" int4 NOT NULL,
  "session_cache_ttl_seconds" int4 NOT NULL,
  "access_cache_ttl_seconds" int4 NOT NULL,
  "bind_device" int2 NOT NULL,
  "bind_ip" int2 NOT NULL,
  "max_sessions" int2 NOT NULL,
  "allow_register" int2 NOT NULL,
  "is_enabled" int2 NOT NULL,
  "is_builtin" int2 NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6),
  "login_types" jsonb NOT NULL DEFAULT '["email", "password"]'::jsonb
)
;

-- ----------------------------
-- Table structure for message_mail_config
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_config";
CREATE TABLE "public"."message_mail_config" (
  "id" int8 NOT NULL DEFAULT nextval('message_mail_config_id_seq'::regclass),
  "platform_id" int8 NOT NULL,
  "secret_id_ciphertext" text COLLATE "pg_catalog"."default" NOT NULL,
  "secret_key_ciphertext" text COLLATE "pg_catalog"."default" NOT NULL,
  "secret_id_hint" varchar(32) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "secret_key_hint" varchar(32) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "region" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "endpoint" varchar(255) COLLATE "pg_catalog"."default",
  "from_email" varchar(254) COLLATE "pg_catalog"."default" NOT NULL,
  "from_name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "reply_to" varchar(254) COLLATE "pg_catalog"."default",
  "ttl_minutes" int2 NOT NULL,
  "is_enabled" int2 NOT NULL DEFAULT 0,
  "last_test_at" timestamptz(6),
  "last_test_error" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for message_mail_log
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_log";
CREATE TABLE "public"."message_mail_log" (
  "id" int8 NOT NULL DEFAULT nextval('message_mail_log_id_seq'::regclass),
  "platform_id" int8 NOT NULL,
  "challenge_id" varchar(128) COLLATE "pg_catalog"."default",
  "user_id" int8,
  "scene" varchar(32) COLLATE "pg_catalog"."default" NOT NULL,
  "template_id" int4 NOT NULL,
  "to_email" varchar(254) COLLATE "pg_catalog"."default" NOT NULL,
  "subject" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  "status" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "request_id" varchar(128) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "message_id" varchar(128) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "error_code" varchar(128) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "error_summary" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "latency_ms" int8 NOT NULL DEFAULT 0,
  "sent_at" timestamptz(6),
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for message_mail_log_verification
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_log_verification";
CREATE TABLE "public"."message_mail_log_verification" (
  "id" int8 NOT NULL DEFAULT nextval('message_mail_log_verification_id_seq'::regclass),
  "platform_id" int8 NOT NULL,
  "mail_log_id" int8 NOT NULL,
  "key_version" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "code_ciphertext" text COLLATE "pg_catalog"."default" NOT NULL,
  "expires_at" timestamptz(6) NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for message_mail_rate_limit_policy
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_rate_limit_policy";
CREATE TABLE "public"."message_mail_rate_limit_policy" (
  "policy_key" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "mode" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "dimension" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "limit_count" int4 NOT NULL,
  "window_seconds" int4 NOT NULL,
  "revision" int8 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP
)
;

-- ----------------------------
-- Table structure for message_mail_recipient_rule
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_recipient_rule";
CREATE TABLE "public"."message_mail_recipient_rule" (
  "id" int8 NOT NULL DEFAULT nextval('message_mail_recipient_rule_id_seq'::regclass),
  "platform_id" int8 NOT NULL,
  "scope" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "pattern" varchar(254) COLLATE "pg_catalog"."default" NOT NULL,
  "action" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "remark" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for message_mail_template
-- ----------------------------
DROP TABLE IF EXISTS "public"."message_mail_template";
CREATE TABLE "public"."message_mail_template" (
  "id" int8 NOT NULL DEFAULT nextval('message_mail_template_id_seq'::regclass),
  "platform_id" int8 NOT NULL,
  "scene" varchar(32) COLLATE "pg_catalog"."default" NOT NULL,
  "name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "subject" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  "tencent_template_id" int4 NOT NULL,
  "variables" jsonb NOT NULL,
  "example_variables" jsonb NOT NULL,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for permission_access_version
-- ----------------------------
DROP TABLE IF EXISTS "public"."permission_access_version";
CREATE TABLE "public"."permission_access_version" (
  "user_id" int8 NOT NULL DEFAULT nextval('rbac_access_version_user_id_seq'::regclass),
  "version" int8 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP
)
;

-- ----------------------------
-- Table structure for permission_menu
-- ----------------------------
DROP TABLE IF EXISTS "public"."permission_menu";
CREATE TABLE "public"."permission_menu" (
  "id" int8 NOT NULL DEFAULT nextval('rbac_menu_id_seq'::regclass),
  "parent_id" int8,
  "menu_type" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "code" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "i18n_key" varchar(128) COLLATE "pg_catalog"."default",
  "path" varchar(255) COLLATE "pg_catalog"."default",
  "icon" varchar(128) COLLATE "pg_catalog"."default",
  "sort_order" int4 NOT NULL DEFAULT 0,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6),
  "component_path" varchar(255) COLLATE "pg_catalog"."default",
  "is_hidden" int2 NOT NULL DEFAULT 0,
  "name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "platform_id" int8 NOT NULL,
  "remark" varchar(512) COLLATE "pg_catalog"."default"
)
;

-- ----------------------------
-- Table structure for permission_role
-- ----------------------------
DROP TABLE IF EXISTS "public"."permission_role";
CREATE TABLE "public"."permission_role" (
  "id" int8 NOT NULL DEFAULT nextval('rbac_role_id_seq'::regclass),
  "code" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "name" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "is_default" int2 NOT NULL DEFAULT 0,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for permission_role_menu
-- ----------------------------
DROP TABLE IF EXISTS "public"."permission_role_menu";
CREATE TABLE "public"."permission_role_menu" (
  "id" int8 NOT NULL DEFAULT nextval('rbac_role_menu_id_seq'::regclass),
  "role_id" int8 NOT NULL,
  "menu_id" int8 NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for permission_user_role
-- ----------------------------
DROP TABLE IF EXISTS "public"."permission_user_role";
CREATE TABLE "public"."permission_user_role" (
  "id" int8 NOT NULL DEFAULT nextval('rbac_user_role_id_seq'::regclass),
  "user_id" int8 NOT NULL,
  "role_id" int8 NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for storage_cos_config
-- ----------------------------
DROP TABLE IF EXISTS "public"."storage_cos_config";
CREATE TABLE "public"."storage_cos_config" (
  "id" int8 NOT NULL GENERATED BY DEFAULT AS IDENTITY (
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1
),
  "name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "app_id" varchar(32) COLLATE "pg_catalog"."default" NOT NULL,
  "secret_id_ciphertext" text COLLATE "pg_catalog"."default" NOT NULL,
  "secret_key_ciphertext" text COLLATE "pg_catalog"."default" NOT NULL,
  "bucket" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "region" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "endpoint" varchar(255) COLLATE "pg_catalog"."default",
  "bucket_domain" varchar(255) COLLATE "pg_catalog"."default",
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "remark" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for storage_upload_rule
-- ----------------------------
DROP TABLE IF EXISTS "public"."storage_upload_rule";
CREATE TABLE "public"."storage_upload_rule" (
  "id" int8 NOT NULL GENERATED BY DEFAULT AS IDENTITY (
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1
),
  "platform_id" int8 NOT NULL,
  "name" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "cos_config_id" int8 NOT NULL,
  "max_file_size_bytes" int8 NOT NULL,
  "allowed_extensions" text[] COLLATE "pg_catalog"."default" NOT NULL,
  "allowed_mime_types" text[] COLLATE "pg_catalog"."default" NOT NULL,
  "access_mode" varchar(16) COLLATE "pg_catalog"."default" NOT NULL DEFAULT 'private'::character varying,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "remark" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for storage_upload_rule_code
-- ----------------------------
DROP TABLE IF EXISTS "public"."storage_upload_rule_code";
CREATE TABLE "public"."storage_upload_rule_code" (
  "id" int8 NOT NULL GENERATED BY DEFAULT AS IDENTITY (
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1
),
  "rule_id" int8 NOT NULL,
  "platform_id" int8 NOT NULL,
  "code" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6)
)
;

-- ----------------------------
-- Table structure for system_operation_log
-- ----------------------------
DROP TABLE IF EXISTS "public"."system_operation_log";
CREATE TABLE "public"."system_operation_log" (
  "id" int8 NOT NULL DEFAULT nextval('audit_operation_log_id_seq'::regclass),
  "request_id" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "user_id" int8,
  "session_id" int8,
  "method" varchar(10) COLLATE "pg_catalog"."default" NOT NULL,
  "route" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  "module" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "action" varchar(128) COLLATE "pg_catalog"."default" NOT NULL,
  "client_ip" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "user_agent" varchar(512) COLLATE "pg_catalog"."default" NOT NULL,
  "status_code" int4 NOT NULL,
  "is_success" int2 NOT NULL,
  "latency_ms" int8 NOT NULL,
  "request_data" jsonb,
  "response_data" jsonb,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "event_id" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "platform_id" int8
)
;

-- ----------------------------
-- Table structure for user_account
-- ----------------------------
DROP TABLE IF EXISTS "public"."user_account";
CREATE TABLE "public"."user_account" (
  "id" int8 NOT NULL DEFAULT nextval('user_account_id_seq'::regclass),
  "username" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "email" varchar(254) COLLATE "pg_catalog"."default" NOT NULL,
  "password_hash" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  "is_enabled" int2 NOT NULL DEFAULT 1,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz(6),
  "phone" varchar(32) COLLATE "pg_catalog"."default"
)
;

-- ----------------------------
-- Table structure for user_login_log
-- ----------------------------
DROP TABLE IF EXISTS "public"."user_login_log";
CREATE TABLE "public"."user_login_log" (
  "id" int8 NOT NULL GENERATED BY DEFAULT AS IDENTITY (
INCREMENT 1
MINVALUE  1
MAXVALUE 9223372036854775807
START 1
CACHE 1
),
  "user_id" int8,
  "session_id" int8,
  "platform_id" int8 NOT NULL,
  "login_account" varchar(254) COLLATE "pg_catalog"."default" NOT NULL,
  "event_type" varchar(16) COLLATE "pg_catalog"."default" NOT NULL,
  "login_type" varchar(32) COLLATE "pg_catalog"."default",
  "is_success" int2 NOT NULL,
  "reason_code" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "client_ip" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "user_agent" varchar(512) COLLATE "pg_catalog"."default" NOT NULL,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP
)
;

-- ----------------------------
-- Table structure for user_profile
-- ----------------------------
DROP TABLE IF EXISTS "public"."user_profile";
CREATE TABLE "public"."user_profile" (
  "user_id" int8 NOT NULL,
  "birthday" date,
  "gender" int2 NOT NULL DEFAULT 0,
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "avatar" varchar(512) COLLATE "pg_catalog"."default" NOT NULL DEFAULT ''::character varying
)
;

-- ----------------------------
-- Table structure for user_session
-- ----------------------------
DROP TABLE IF EXISTS "public"."user_session";
CREATE TABLE "public"."user_session" (
  "id" int8 NOT NULL DEFAULT nextval('auth_session_id_seq'::regclass),
  "user_id" int8 NOT NULL,
  "refresh_token_hash" char(64) COLLATE "pg_catalog"."default" NOT NULL,
  "version" int8 NOT NULL DEFAULT 1,
  "client_ip" varchar(64) COLLATE "pg_catalog"."default" NOT NULL,
  "user_agent" varchar(512) COLLATE "pg_catalog"."default" NOT NULL,
  "refresh_expires_at" timestamptz(6) NOT NULL,
  "revoked_at" timestamptz(6),
  "created_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "device_id" varchar(36) COLLATE "pg_catalog"."default" NOT NULL,
  "platform_id" int8 NOT NULL
)
;

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."audit_operation_log_id_seq"
OWNED BY "public"."system_operation_log"."id";
SELECT setval('"public"."audit_operation_log_id_seq"', 208, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."auth_platform_id_seq"
OWNED BY "public"."auth_platform"."id";
SELECT setval('"public"."auth_platform_id_seq"', 2, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."auth_session_id_seq"
OWNED BY "public"."user_session"."id";
SELECT setval('"public"."auth_session_id_seq"', 181, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."message_mail_config_id_seq"
OWNED BY "public"."message_mail_config"."id";
SELECT setval('"public"."message_mail_config_id_seq"', 1, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."message_mail_log_id_seq"
OWNED BY "public"."message_mail_log"."id";
SELECT setval('"public"."message_mail_log_id_seq"', 4, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."message_mail_log_verification_id_seq"
OWNED BY "public"."message_mail_log_verification"."id";
SELECT setval('"public"."message_mail_log_verification_id_seq"', 4, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."message_mail_recipient_rule_id_seq"
OWNED BY "public"."message_mail_recipient_rule"."id";
SELECT setval('"public"."message_mail_recipient_rule_id_seq"', 1, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."message_mail_template_id_seq"
OWNED BY "public"."message_mail_template"."id";
SELECT setval('"public"."message_mail_template_id_seq"', 4, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."rbac_access_version_user_id_seq"
OWNED BY "public"."permission_access_version"."user_id";
SELECT setval('"public"."rbac_access_version_user_id_seq"', 1, false);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."rbac_menu_id_seq"
OWNED BY "public"."permission_menu"."id";
SELECT setval('"public"."rbac_menu_id_seq"', 4224, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."rbac_role_id_seq"
OWNED BY "public"."permission_role"."id";
SELECT setval('"public"."rbac_role_id_seq"', 926, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."rbac_role_menu_id_seq"
OWNED BY "public"."permission_role_menu"."id";
SELECT setval('"public"."rbac_role_menu_id_seq"', 584, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."rbac_user_role_id_seq"
OWNED BY "public"."permission_user_role"."id";
SELECT setval('"public"."rbac_user_role_id_seq"', 804, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."storage_cos_config_id_seq"
OWNED BY "public"."storage_cos_config"."id";
SELECT setval('"public"."storage_cos_config_id_seq"', 2, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."storage_upload_rule_code_id_seq"
OWNED BY "public"."storage_upload_rule_code"."id";
SELECT setval('"public"."storage_upload_rule_code_id_seq"', 1, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."storage_upload_rule_id_seq"
OWNED BY "public"."storage_upload_rule"."id";
SELECT setval('"public"."storage_upload_rule_id_seq"', 1, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."user_account_id_seq"
OWNED BY "public"."user_account"."id";
SELECT setval('"public"."user_account_id_seq"', 717, true);

-- ----------------------------
-- Alter sequences owned by
-- ----------------------------
ALTER SEQUENCE "public"."user_login_log_id_seq"
OWNED BY "public"."user_login_log"."id";
SELECT setval('"public"."user_login_log_id_seq"', 30, true);

-- ----------------------------
-- Indexes structure for table auth_platform
-- ----------------------------
CREATE UNIQUE INDEX "ux_auth_platform_code_active" ON "public"."auth_platform" USING btree (
  "code" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table auth_platform
-- ----------------------------
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_access_cache_ttl_seconds" CHECK (access_cache_ttl_seconds >= 60 AND access_cache_ttl_seconds <= 86400);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_access_ttl_seconds" CHECK (access_ttl_seconds >= 60 AND access_ttl_seconds <= 2592000);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_allow_register" CHECK (allow_register = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_bind_device" CHECK (bind_device = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_bind_ip" CHECK (bind_ip = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_code" CHECK (code::text ~ '^[a-z][a-z0-9_]{1,48}$'::text);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_is_builtin" CHECK (is_builtin = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_login_types" CHECK (jsonb_typeof(login_types) = 'array'::text AND jsonb_array_length(login_types) >= 1 AND jsonb_array_length(login_types) <= 3 AND login_types <@ '["email", "phone", "password"]'::jsonb AND jsonb_array_length(login_types) = (
CASE
    WHEN login_types @> '["email"]'::jsonb THEN 1
    ELSE 0
END +
CASE
    WHEN login_types @> '["phone"]'::jsonb THEN 1
    ELSE 0
END +
CASE
    WHEN login_types @> '["password"]'::jsonb THEN 1
    ELSE 0
END));
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_max_sessions" CHECK (max_sessions >= 0 AND max_sessions <= 100);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_policy_version" CHECK (policy_version >= 1);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_refresh_ttl_seconds" CHECK (refresh_ttl_seconds >= 60 AND refresh_ttl_seconds <= 31536000);
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "ck_auth_platform_session_cache_ttl_seconds" CHECK (session_cache_ttl_seconds >= 60 AND session_cache_ttl_seconds <= 86400);

-- ----------------------------
-- Primary Key structure for table auth_platform
-- ----------------------------
ALTER TABLE "public"."auth_platform" ADD CONSTRAINT "auth_platform_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table message_mail_config
-- ----------------------------
CREATE UNIQUE INDEX "ux_message_mail_config_platform_active" ON "public"."message_mail_config" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table message_mail_config
-- ----------------------------
ALTER TABLE "public"."message_mail_config" ADD CONSTRAINT "message_mail_config_is_enabled_check" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."message_mail_config" ADD CONSTRAINT "message_mail_config_ttl_minutes_check" CHECK (ttl_minutes >= 1 AND ttl_minutes <= 60);

-- ----------------------------
-- Primary Key structure for table message_mail_config
-- ----------------------------
ALTER TABLE "public"."message_mail_config" ADD CONSTRAINT "message_mail_config_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table message_mail_log
-- ----------------------------
CREATE UNIQUE INDEX "ux_message_mail_log_platform_challenge_active" ON "public"."message_mail_log" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "challenge_id" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL AND challenge_id IS NOT NULL;

-- ----------------------------
-- Uniques structure for table message_mail_log
-- ----------------------------
ALTER TABLE "public"."message_mail_log" ADD CONSTRAINT "message_mail_log_id_platform_id_key" UNIQUE ("id", "platform_id");

-- ----------------------------
-- Checks structure for table message_mail_log
-- ----------------------------
ALTER TABLE "public"."message_mail_log" ADD CONSTRAINT "message_mail_log_status_check" CHECK (status::text = ANY (ARRAY['pending'::character varying, 'sent'::character varying, 'failed'::character varying]::text[]));

-- ----------------------------
-- Primary Key structure for table message_mail_log
-- ----------------------------
ALTER TABLE "public"."message_mail_log" ADD CONSTRAINT "message_mail_log_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table message_mail_log_verification
-- ----------------------------
CREATE UNIQUE INDEX "ux_message_mail_verification_log_active" ON "public"."message_mail_log_verification" USING btree (
  "mail_log_id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Primary Key structure for table message_mail_log_verification
-- ----------------------------
ALTER TABLE "public"."message_mail_log_verification" ADD CONSTRAINT "message_mail_log_verification_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Checks structure for table message_mail_rate_limit_policy
-- ----------------------------
ALTER TABLE "public"."message_mail_rate_limit_policy" ADD CONSTRAINT "ck_message_mail_rate_limit_policy_shape" CHECK (policy_key::text = 'business_email_minute'::text AND mode::text = 'business'::text AND dimension::text = 'platform_scene_email'::text OR policy_key::text = 'business_email_10m'::text AND mode::text = 'business'::text AND dimension::text = 'platform_scene_email'::text OR policy_key::text = 'business_ip_minute'::text AND mode::text = 'business'::text AND dimension::text = 'platform_ip'::text OR policy_key::text = 'business_scene_minute'::text AND mode::text = 'business'::text AND dimension::text = 'platform_scene'::text OR policy_key::text = 'admin_test_user_10m'::text AND mode::text = 'admin_test'::text AND dimension::text = 'admin_user'::text OR policy_key::text = 'admin_test_ip_minute'::text AND mode::text = 'admin_test'::text AND dimension::text = 'ip'::text OR policy_key::text = 'admin_test_email_10m'::text AND mode::text = 'admin_test'::text AND dimension::text = 'email'::text);
ALTER TABLE "public"."message_mail_rate_limit_policy" ADD CONSTRAINT "ck_message_mail_rate_limit_policy_values" CHECK (limit_count >= 1 AND limit_count <= 100000 AND window_seconds >= 1 AND window_seconds <= 86400);
ALTER TABLE "public"."message_mail_rate_limit_policy" ADD CONSTRAINT "ck_message_mail_rate_limit_policy_revision" CHECK (revision >= 1);

-- ----------------------------
-- Primary Key structure for table message_mail_rate_limit_policy
-- ----------------------------
ALTER TABLE "public"."message_mail_rate_limit_policy" ADD CONSTRAINT "message_mail_rate_limit_policy_pkey" PRIMARY KEY ("policy_key");

-- ----------------------------
-- Indexes structure for table message_mail_recipient_rule
-- ----------------------------
CREATE UNIQUE INDEX "ux_message_mail_rule_platform_scope_pattern_action_active" ON "public"."message_mail_recipient_rule" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "scope" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST,
  "pattern" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST,
  "action" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table message_mail_recipient_rule
-- ----------------------------
ALTER TABLE "public"."message_mail_recipient_rule" ADD CONSTRAINT "message_mail_recipient_rule_is_enabled_check" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."message_mail_recipient_rule" ADD CONSTRAINT "message_mail_recipient_rule_scope_check" CHECK (scope::text = ANY (ARRAY['email'::character varying, 'domain'::character varying]::text[]));
ALTER TABLE "public"."message_mail_recipient_rule" ADD CONSTRAINT "message_mail_recipient_rule_action_check" CHECK (action::text = ANY (ARRAY['allow'::character varying, 'deny'::character varying]::text[]));

-- ----------------------------
-- Primary Key structure for table message_mail_recipient_rule
-- ----------------------------
ALTER TABLE "public"."message_mail_recipient_rule" ADD CONSTRAINT "message_mail_recipient_rule_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table message_mail_template
-- ----------------------------
CREATE UNIQUE INDEX "ux_message_mail_template_platform_scene_active" ON "public"."message_mail_template" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "scene" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table message_mail_template
-- ----------------------------
ALTER TABLE "public"."message_mail_template" ADD CONSTRAINT "message_mail_template_is_enabled_check" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."message_mail_template" ADD CONSTRAINT "message_mail_template_scene_check" CHECK (scene::text = ANY (ARRAY['login'::character varying, 'forget'::character varying, 'bind_email'::character varying, 'change_password'::character varying]::text[]));

-- ----------------------------
-- Primary Key structure for table message_mail_template
-- ----------------------------
ALTER TABLE "public"."message_mail_template" ADD CONSTRAINT "message_mail_template_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Checks structure for table permission_access_version
-- ----------------------------
ALTER TABLE "public"."permission_access_version" ADD CONSTRAINT "ck_rbac_access_version_version" CHECK (version >= 1);

-- ----------------------------
-- Primary Key structure for table permission_access_version
-- ----------------------------
ALTER TABLE "public"."permission_access_version" ADD CONSTRAINT "rbac_access_version_pkey" PRIMARY KEY ("user_id");

-- ----------------------------
-- Indexes structure for table permission_menu
-- ----------------------------
CREATE INDEX "ix_rbac_menu_parent_active" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "parent_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "sort_order" "pg_catalog"."int4_ops" ASC NULLS LAST,
  "id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;
CREATE INDEX "ix_rbac_menu_platform_parent_sort" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "parent_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "sort_order" "pg_catalog"."int4_ops" ASC NULLS LAST,
  "id" "pg_catalog"."int8_ops" ASC NULLS LAST
);
CREATE UNIQUE INDEX "ux_rbac_menu_code_active" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "code" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_rbac_menu_page_path_active" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "path" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL AND menu_type::text = 'page'::text;
CREATE UNIQUE INDEX "ux_rbac_menu_platform_code_active" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "code" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_rbac_menu_platform_path_active" ON "public"."permission_menu" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "path" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE path IS NOT NULL AND deleted_at IS NULL;

-- ----------------------------
-- Uniques structure for table permission_menu
-- ----------------------------
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "uq_rbac_menu_id_platform" UNIQUE ("id", "platform_id");

-- ----------------------------
-- Checks structure for table permission_menu
-- ----------------------------
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "ck_rbac_menu_shape" CHECK (btrim(name::text) <> ''::text AND (menu_type::text = 'directory'::text AND i18n_key IS NOT NULL AND path IS NULL AND component_path IS NULL OR menu_type::text = 'page'::text AND i18n_key IS NOT NULL AND path IS NOT NULL AND btrim(path::text) <> ''::text AND component_path IS NOT NULL AND btrim(component_path::text) <> ''::text OR menu_type::text = 'action'::text AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1));
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "ck_rbac_menu_sort_order" CHECK (sort_order >= 0);
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "ck_rbac_menu_type" CHECK (menu_type::text = ANY (ARRAY['directory'::character varying, 'page'::character varying, 'action'::character varying]::text[]));
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "ck_rbac_menu_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "ck_rbac_menu_is_hidden" CHECK (is_hidden = ANY (ARRAY[0, 1]));

-- ----------------------------
-- Primary Key structure for table permission_menu
-- ----------------------------
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "rbac_menu_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table permission_role
-- ----------------------------
CREATE UNIQUE INDEX "ux_rbac_role_code_active" ON "public"."permission_role" USING btree (
  "code" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_rbac_role_default_active" ON "public"."permission_role" USING btree (
  "is_default" "pg_catalog"."int2_ops" ASC NULLS LAST
) WHERE is_default = 1 AND deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_rbac_role_name_active" ON "public"."permission_role" USING btree (
  "name" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table permission_role
-- ----------------------------
ALTER TABLE "public"."permission_role" ADD CONSTRAINT "ck_rbac_role_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."permission_role" ADD CONSTRAINT "ck_rbac_role_is_default" CHECK (is_default = ANY (ARRAY[0, 1]));

-- ----------------------------
-- Primary Key structure for table permission_role
-- ----------------------------
ALTER TABLE "public"."permission_role" ADD CONSTRAINT "rbac_role_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table permission_role_menu
-- ----------------------------
CREATE UNIQUE INDEX "ux_rbac_role_menu_active" ON "public"."permission_role_menu" USING btree (
  "role_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "menu_id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Primary Key structure for table permission_role_menu
-- ----------------------------
ALTER TABLE "public"."permission_role_menu" ADD CONSTRAINT "rbac_role_menu_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table permission_user_role
-- ----------------------------
CREATE UNIQUE INDEX "ux_rbac_user_role_active" ON "public"."permission_user_role" USING btree (
  "user_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "role_id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Primary Key structure for table permission_user_role
-- ----------------------------
ALTER TABLE "public"."permission_user_role" ADD CONSTRAINT "rbac_user_role_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table storage_cos_config
-- ----------------------------
CREATE INDEX "ix_storage_cos_config_enabled_created_at" ON "public"."storage_cos_config" USING btree (
  "is_enabled" "pg_catalog"."int2_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_storage_cos_config_name_active" ON "public"."storage_cos_config" USING btree (
  lower(name::text) COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table storage_cos_config
-- ----------------------------
ALTER TABLE "public"."storage_cos_config" ADD CONSTRAINT "ck_storage_cos_config_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));

-- ----------------------------
-- Primary Key structure for table storage_cos_config
-- ----------------------------
ALTER TABLE "public"."storage_cos_config" ADD CONSTRAINT "storage_cos_config_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table storage_upload_rule
-- ----------------------------
CREATE INDEX "ix_storage_upload_rule_config_enabled_created_at" ON "public"."storage_upload_rule" USING btree (
  "cos_config_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "is_enabled" "pg_catalog"."int2_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table storage_upload_rule
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "ck_storage_upload_rule_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "ck_storage_upload_rule_max_file_size" CHECK (max_file_size_bytes > 0);
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "ck_storage_upload_rule_access_mode" CHECK (access_mode::text = ANY (ARRAY['private'::character varying, 'public'::character varying]::text[]));

-- ----------------------------
-- Primary Key structure for table storage_upload_rule
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "storage_upload_rule_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table storage_upload_rule_code
-- ----------------------------
CREATE INDEX "ix_storage_upload_rule_code_rule" ON "public"."storage_upload_rule_code" USING btree (
  "rule_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "id" "pg_catalog"."int8_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_storage_upload_rule_code_platform_code" ON "public"."storage_upload_rule_code" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "code" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table storage_upload_rule_code
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule_code" ADD CONSTRAINT "ck_storage_upload_rule_code_value" CHECK (length(btrim(code::text)) > 0);

-- ----------------------------
-- Primary Key structure for table storage_upload_rule_code
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule_code" ADD CONSTRAINT "storage_upload_rule_code_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table system_operation_log
-- ----------------------------
CREATE INDEX "ix_audit_operation_log_action_created_at" ON "public"."system_operation_log" USING btree (
  "action" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_audit_operation_log_created_at" ON "public"."system_operation_log" USING btree (
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_audit_operation_log_request_id" ON "public"."system_operation_log" USING btree (
  "request_id" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
);
CREATE INDEX "ix_audit_operation_log_user_created_at" ON "public"."system_operation_log" USING btree (
  "user_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE UNIQUE INDEX "ux_audit_operation_log_event_id" ON "public"."system_operation_log" USING btree (
  "event_id" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
);

-- ----------------------------
-- Checks structure for table system_operation_log
-- ----------------------------
ALTER TABLE "public"."system_operation_log" ADD CONSTRAINT "ck_audit_operation_log_is_success" CHECK (is_success = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."system_operation_log" ADD CONSTRAINT "ck_audit_operation_log_latency_ms" CHECK (latency_ms >= 0);

-- ----------------------------
-- Primary Key structure for table system_operation_log
-- ----------------------------
ALTER TABLE "public"."system_operation_log" ADD CONSTRAINT "audit_operation_log_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table user_account
-- ----------------------------
CREATE UNIQUE INDEX "ux_user_account_email_active" ON "public"."user_account" USING btree (
  lower(email::text) COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE email::text <> ''::text AND deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_user_account_phone_active" ON "public"."user_account" USING btree (
  "phone" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE phone IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX "ux_user_account_username_active" ON "public"."user_account" USING btree (
  lower(username::text) COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST
) WHERE deleted_at IS NULL;

-- ----------------------------
-- Checks structure for table user_account
-- ----------------------------
ALTER TABLE "public"."user_account" ADD CONSTRAINT "ck_user_account_is_enabled" CHECK (is_enabled = ANY (ARRAY[0, 1]));

-- ----------------------------
-- Primary Key structure for table user_account
-- ----------------------------
ALTER TABLE "public"."user_account" ADD CONSTRAINT "user_account_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Indexes structure for table user_login_log
-- ----------------------------
CREATE INDEX "ix_user_login_log_account_created_at" ON "public"."user_login_log" USING btree (
  "login_account" COLLATE "pg_catalog"."default" "pg_catalog"."text_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_user_login_log_created_at" ON "public"."user_login_log" USING btree (
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_user_login_log_platform_created_at" ON "public"."user_login_log" USING btree (
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_user_login_log_user_created_at" ON "public"."user_login_log" USING btree (
  "user_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);

-- ----------------------------
-- Checks structure for table user_login_log
-- ----------------------------
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "ck_user_login_log_is_success" CHECK (is_success = ANY (ARRAY[0, 1]));
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "ck_user_login_log_login_type" CHECK (event_type::text = 'login'::text AND login_type IS NOT NULL OR event_type::text = 'logout'::text AND login_type IS NULL);
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "ck_user_login_log_event_type" CHECK (event_type::text = ANY (ARRAY['login'::character varying, 'logout'::character varying]::text[]));

-- ----------------------------
-- Primary Key structure for table user_login_log
-- ----------------------------
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "user_login_log_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Checks structure for table user_profile
-- ----------------------------
ALTER TABLE "public"."user_profile" ADD CONSTRAINT "ck_user_profile_gender" CHECK (gender = ANY (ARRAY[0, 1, 2]));

-- ----------------------------
-- Primary Key structure for table user_profile
-- ----------------------------
ALTER TABLE "public"."user_profile" ADD CONSTRAINT "user_profile_pkey" PRIMARY KEY ("user_id");

-- ----------------------------
-- Indexes structure for table user_session
-- ----------------------------
CREATE INDEX "ix_user_session_user_created_at" ON "public"."user_session" USING btree (
  "user_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST
);
CREATE INDEX "ix_user_session_user_platform_created_at" ON "public"."user_session" USING btree (
  "user_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "platform_id" "pg_catalog"."int8_ops" ASC NULLS LAST,
  "created_at" "pg_catalog"."timestamptz_ops" DESC NULLS FIRST,
  "id" "pg_catalog"."int8_ops" DESC NULLS FIRST
) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX "ux_user_session_refresh_token_hash" ON "public"."user_session" USING btree (
  "refresh_token_hash" COLLATE "pg_catalog"."default" "pg_catalog"."bpchar_ops" ASC NULLS LAST
);

-- ----------------------------
-- Checks structure for table user_session
-- ----------------------------
ALTER TABLE "public"."user_session" ADD CONSTRAINT "ck_auth_session_version" CHECK (version >= 1);

-- ----------------------------
-- Primary Key structure for table user_session
-- ----------------------------
ALTER TABLE "public"."user_session" ADD CONSTRAINT "auth_session_pkey" PRIMARY KEY ("id");

-- ----------------------------
-- Foreign Keys structure for table message_mail_config
-- ----------------------------
ALTER TABLE "public"."message_mail_config" ADD CONSTRAINT "message_mail_config_platform_id_fkey" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE NO ACTION ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table message_mail_log
-- ----------------------------
ALTER TABLE "public"."message_mail_log" ADD CONSTRAINT "message_mail_log_platform_id_fkey" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE NO ACTION ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table message_mail_log_verification
-- ----------------------------
ALTER TABLE "public"."message_mail_log_verification" ADD CONSTRAINT "message_mail_log_verification_mail_log_id_platform_id_fkey" FOREIGN KEY ("mail_log_id", "platform_id") REFERENCES "public"."message_mail_log" ("id", "platform_id") ON DELETE NO ACTION ON UPDATE NO ACTION;
ALTER TABLE "public"."message_mail_log_verification" ADD CONSTRAINT "message_mail_log_verification_platform_id_fkey" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE NO ACTION ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table message_mail_recipient_rule
-- ----------------------------
ALTER TABLE "public"."message_mail_recipient_rule" ADD CONSTRAINT "message_mail_recipient_rule_platform_id_fkey" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE NO ACTION ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table message_mail_template
-- ----------------------------
ALTER TABLE "public"."message_mail_template" ADD CONSTRAINT "message_mail_template_platform_id_fkey" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE NO ACTION ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table permission_access_version
-- ----------------------------
ALTER TABLE "public"."permission_access_version" ADD CONSTRAINT "fk_rbac_access_version_user" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table permission_menu
-- ----------------------------
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "fk_rbac_menu_parent_platform" FOREIGN KEY ("parent_id", "platform_id") REFERENCES "public"."permission_menu" ("id", "platform_id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."permission_menu" ADD CONSTRAINT "fk_rbac_menu_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table permission_role_menu
-- ----------------------------
ALTER TABLE "public"."permission_role_menu" ADD CONSTRAINT "fk_rbac_role_menu_menu" FOREIGN KEY ("menu_id") REFERENCES "public"."permission_menu" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."permission_role_menu" ADD CONSTRAINT "fk_rbac_role_menu_role" FOREIGN KEY ("role_id") REFERENCES "public"."permission_role" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table permission_user_role
-- ----------------------------
ALTER TABLE "public"."permission_user_role" ADD CONSTRAINT "fk_rbac_user_role_role" FOREIGN KEY ("role_id") REFERENCES "public"."permission_role" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."permission_user_role" ADD CONSTRAINT "fk_rbac_user_role_user" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table storage_upload_rule
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "fk_storage_upload_rule_cos_config" FOREIGN KEY ("cos_config_id") REFERENCES "public"."storage_cos_config" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."storage_upload_rule" ADD CONSTRAINT "fk_storage_upload_rule_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table storage_upload_rule_code
-- ----------------------------
ALTER TABLE "public"."storage_upload_rule_code" ADD CONSTRAINT "fk_storage_upload_rule_code_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."storage_upload_rule_code" ADD CONSTRAINT "fk_storage_upload_rule_code_rule" FOREIGN KEY ("rule_id") REFERENCES "public"."storage_upload_rule" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table system_operation_log
-- ----------------------------
ALTER TABLE "public"."system_operation_log" ADD CONSTRAINT "fk_audit_operation_log_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table user_login_log
-- ----------------------------
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "fk_user_login_log_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "fk_user_login_log_session" FOREIGN KEY ("session_id") REFERENCES "public"."user_session" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."user_login_log" ADD CONSTRAINT "fk_user_login_log_user" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table user_profile
-- ----------------------------
ALTER TABLE "public"."user_profile" ADD CONSTRAINT "fk_user_profile_account" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;

-- ----------------------------
-- Foreign Keys structure for table user_session
-- ----------------------------
ALTER TABLE "public"."user_session" ADD CONSTRAINT "fk_auth_session_user" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."user_session" ADD CONSTRAINT "fk_user_session_platform" FOREIGN KEY ("platform_id") REFERENCES "public"."auth_platform" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
ALTER TABLE "public"."user_session" ADD CONSTRAINT "fk_user_session_user" FOREIGN KEY ("user_id") REFERENCES "public"."user_account" ("id") ON DELETE RESTRICT ON UPDATE NO ACTION;
