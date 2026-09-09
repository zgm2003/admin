--
-- PostgreSQL database dump
--

\restrict cydo0mfmzKuo5SeHWsLnY3rjI1tVL7VYLwLfqnRxj3QdKBoAxdWTeuz2nnGXnWQ

-- Dumped from database version 18.6
-- Dumped by pg_dump version 18.6

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: test_access_1787825924504187200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_access_1787825924504187200;


--
-- Name: test_authplatform_1787825907271387600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1787825907271387600;


--
-- Name: test_authplatform_1788311751053591400; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788311751053591400;


--
-- Name: test_authplatform_1788311752797777800; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788311752797777800;


--
-- Name: test_authplatform_1788311760786861300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788311760786861300;


--
-- Name: test_authplatform_1788349452674711900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788349452674711900;


--
-- Name: test_authplatform_1788411137839783200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788411137839783200;


--
-- Name: test_authplatform_1788437259710199000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_authplatform_1788437259710199000;


--
-- Name: test_menu_1787708489894412200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1787708489894412200;


--
-- Name: test_menu_1788254206452353300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254206452353300;


--
-- Name: test_menu_1788254327450341500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254327450341500;


--
-- Name: test_menu_1788254336756965800; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254336756965800;


--
-- Name: test_menu_1788254342079010600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254342079010600;


--
-- Name: test_menu_1788254343498451800; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254343498451800;


--
-- Name: test_menu_1788254894117835900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788254894117835900;


--
-- Name: test_menu_1788311852387565700; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788311852387565700;


--
-- Name: test_menu_1788349432618315400; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788349432618315400;


--
-- Name: test_menu_1788410261887402900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788410261887402900;


--
-- Name: test_menu_1788437228419182900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437228419182900;


--
-- Name: test_menu_1788437229697515700; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437229697515700;


--
-- Name: test_menu_1788437231914388800; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437231914388800;


--
-- Name: test_menu_1788437234335272500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437234335272500;


--
-- Name: test_menu_1788437247205480900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437247205480900;


--
-- Name: test_menu_1788437248324521600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437248324521600;


--
-- Name: test_menu_1788437250549432600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437250549432600;


--
-- Name: test_menu_1788437251787681300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437251787681300;


--
-- Name: test_menu_1788437253175426000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437253175426000;


--
-- Name: test_menu_1788437254379752200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788437254379752200;


--
-- Name: test_menu_1788441857738923100; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788441857738923100;


--
-- Name: test_menu_1788441871114670000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788441871114670000;


--
-- Name: test_menu_1788441909090336400; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788441909090336400;


--
-- Name: test_menu_1788442272188551500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788442272188551500;


--
-- Name: test_menu_1788442277382854500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_menu_1788442277382854500;


--
-- Name: test_role_1788254889619946000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788254889619946000;


--
-- Name: test_role_1788408902789809500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788408902789809500;


--
-- Name: test_role_1788410258012550900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788410258012550900;


--
-- Name: test_role_1788437228767960600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437228767960600;


--
-- Name: test_role_1788437239894279000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437239894279000;


--
-- Name: test_role_1788437240954624000; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437240954624000;


--
-- Name: test_role_1788437259429051200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437259429051200;


--
-- Name: test_role_1788437281889155700; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437281889155700;


--
-- Name: test_role_1788437326766827300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_role_1788437326766827300;


--
-- Name: test_user_1788254838354852700; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788254838354852700;


--
-- Name: test_user_1788254906426654300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788254906426654300;


--
-- Name: test_user_1788311761257629300; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788311761257629300;


--
-- Name: test_user_1788311762877383200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788311762877383200;


--
-- Name: test_user_1788311830826014900; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788311830826014900;


--
-- Name: test_user_1788311838850049400; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788311838850049400;


--
-- Name: test_user_1788408911878467100; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788408911878467100;


--
-- Name: test_user_1788408915434940200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788408915434940200;


--
-- Name: test_user_1788408928409858800; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788408928409858800;


--
-- Name: test_user_1788410263703131500; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788410263703131500;


--
-- Name: test_user_1788437284027131600; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788437284027131600;


--
-- Name: test_user_1788437289956774700; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788437289956774700;


--
-- Name: test_user_1788441842042685200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_1788441842042685200;


--
-- Name: test_user_session_1788437240066425200; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA test_user_session_1788437240066425200;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: system_operation_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_operation_log (
    id bigint CONSTRAINT audit_operation_log_id_not_null NOT NULL,
    request_id character varying(128) CONSTRAINT audit_operation_log_request_id_not_null NOT NULL,
    user_id bigint,
    session_id bigint,
    method character varying(10) CONSTRAINT audit_operation_log_method_not_null NOT NULL,
    route character varying(255) CONSTRAINT audit_operation_log_route_not_null NOT NULL,
    module character varying(64) CONSTRAINT audit_operation_log_module_not_null NOT NULL,
    action character varying(128) CONSTRAINT audit_operation_log_action_not_null NOT NULL,
    client_ip character varying(64) CONSTRAINT audit_operation_log_client_ip_not_null NOT NULL,
    user_agent character varying(512) CONSTRAINT audit_operation_log_user_agent_not_null NOT NULL,
    status_code integer CONSTRAINT audit_operation_log_status_code_not_null NOT NULL,
    is_success smallint CONSTRAINT audit_operation_log_is_success_not_null NOT NULL,
    latency_ms bigint CONSTRAINT audit_operation_log_latency_ms_not_null NOT NULL,
    request_data jsonb,
    response_data jsonb,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT audit_operation_log_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT audit_operation_log_updated_at_not_null NOT NULL,
    event_id character varying(64) CONSTRAINT audit_operation_log_event_id_not_null NOT NULL,
    platform_id bigint,
    CONSTRAINT ck_audit_operation_log_is_success CHECK ((is_success = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_audit_operation_log_latency_ms CHECK ((latency_ms >= 0))
);


--
-- Name: audit_operation_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_operation_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_operation_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_operation_log_id_seq OWNED BY public.system_operation_log.id;


--
-- Name: user_session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_session (
    id bigint CONSTRAINT auth_session_id_not_null NOT NULL,
    user_id bigint CONSTRAINT auth_session_user_id_not_null NOT NULL,
    refresh_token_hash character(64) CONSTRAINT auth_session_refresh_token_hash_not_null NOT NULL,
    version bigint DEFAULT 1 CONSTRAINT auth_session_version_not_null NOT NULL,
    client_ip character varying(64) CONSTRAINT auth_session_client_ip_not_null NOT NULL,
    user_agent character varying(512) CONSTRAINT auth_session_user_agent_not_null NOT NULL,
    refresh_expires_at timestamp with time zone CONSTRAINT auth_session_refresh_expires_at_not_null NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT auth_session_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT auth_session_updated_at_not_null NOT NULL,
    device_id character varying(36) CONSTRAINT auth_session_device_id_not_null NOT NULL,
    platform_id bigint NOT NULL,
    CONSTRAINT ck_auth_session_version CHECK ((version >= 1))
);


--
-- Name: auth_session_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.auth_session_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: auth_session_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.auth_session_id_seq OWNED BY public.user_session.id;


--
-- Name: message_mail_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_config (
    id bigint NOT NULL,
    secret_id_ciphertext text NOT NULL,
    secret_key_ciphertext text NOT NULL,
    secret_id_hint character varying(32) DEFAULT ''::character varying NOT NULL,
    secret_key_hint character varying(32) DEFAULT ''::character varying NOT NULL,
    region character varying(64) NOT NULL,
    endpoint character varying(255),
    from_email character varying(254) NOT NULL,
    from_name character varying(128) NOT NULL,
    reply_to character varying(254),
    ttl_minutes smallint NOT NULL,
    is_enabled smallint DEFAULT 0 NOT NULL,
    last_test_at timestamp with time zone,
    last_test_error character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT message_mail_config_is_enabled_check CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_mail_config_ttl_minutes_check CHECK (((ttl_minutes >= 1) AND (ttl_minutes <= 60)))
);


--
-- Name: message_mail_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_mail_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_mail_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_mail_config_id_seq OWNED BY public.message_mail_config.id;


--
-- Name: message_mail_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_log (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    challenge_id character varying(128),
    user_id bigint,
    scene character varying(32) NOT NULL,
    template_id integer NOT NULL,
    to_email character varying(254) NOT NULL,
    subject character varying(255) NOT NULL,
    status character varying(16) NOT NULL,
    request_id character varying(128) DEFAULT ''::character varying NOT NULL,
    message_id character varying(128) DEFAULT ''::character varying NOT NULL,
    error_code character varying(128) DEFAULT ''::character varying NOT NULL,
    error_summary character varying(512) DEFAULT ''::character varying NOT NULL,
    latency_ms bigint DEFAULT 0 NOT NULL,
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT message_mail_log_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'sent'::character varying, 'failed'::character varying])::text[])))
);


--
-- Name: message_mail_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_mail_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_mail_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_mail_log_id_seq OWNED BY public.message_mail_log.id;


--
-- Name: message_mail_log_verification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_log_verification (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    mail_log_id bigint NOT NULL,
    key_version character varying(16) NOT NULL,
    code_ciphertext text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: message_mail_log_verification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_mail_log_verification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_mail_log_verification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_mail_log_verification_id_seq OWNED BY public.message_mail_log_verification.id;


--
-- Name: message_mail_rate_limit_policy; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_rate_limit_policy (
    policy_key character varying(64) NOT NULL,
    mode character varying(16) NOT NULL,
    dimension character varying(64) NOT NULL,
    limit_count integer NOT NULL,
    window_seconds integer NOT NULL,
    revision bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    platform_id bigint NOT NULL,
    CONSTRAINT ck_message_mail_rate_limit_policy_platform CHECK ((platform_id > 0)),
    CONSTRAINT ck_message_mail_rate_limit_policy_revision CHECK ((revision >= 1)),
    CONSTRAINT ck_message_mail_rate_limit_policy_shape CHECK ((((policy_key)::text = ANY ((ARRAY['business_email_minute'::character varying, 'business_email_10m'::character varying])::text[])) AND ((mode)::text = 'business'::text) AND ((dimension)::text = 'platform_email'::text))),
    CONSTRAINT ck_message_mail_rate_limit_policy_values CHECK ((((limit_count >= 1) AND (limit_count <= 100000)) AND ((window_seconds >= 1) AND (window_seconds <= 86400))))
);


--
-- Name: message_mail_recipient_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_recipient_rule (
    id bigint NOT NULL,
    scope character varying(16) NOT NULL,
    pattern character varying(254) NOT NULL,
    action character varying(16) NOT NULL,
    name character varying(128) NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT message_mail_recipient_rule_action_check CHECK (((action)::text = ANY ((ARRAY['allow'::character varying, 'deny'::character varying])::text[]))),
    CONSTRAINT message_mail_recipient_rule_is_enabled_check CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_mail_recipient_rule_scope_check CHECK (((scope)::text = ANY ((ARRAY['email'::character varying, 'domain'::character varying])::text[])))
);


--
-- Name: message_mail_recipient_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_mail_recipient_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_mail_recipient_rule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_mail_recipient_rule_id_seq OWNED BY public.message_mail_recipient_rule.id;


--
-- Name: message_mail_template; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_template (
    id bigint NOT NULL,
    scene character varying(32) NOT NULL,
    name character varying(128) NOT NULL,
    subject character varying(255) NOT NULL,
    tencent_template_id integer NOT NULL,
    variables jsonb NOT NULL,
    example_variables jsonb NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT message_mail_template_is_enabled_check CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_mail_template_scene_check CHECK (((scene)::text = ANY ((ARRAY['login'::character varying, 'forget'::character varying, 'bind_email'::character varying, 'change_password'::character varying])::text[])))
);


--
-- Name: message_mail_template_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_mail_template_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_mail_template_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_mail_template_id_seq OWNED BY public.message_mail_template.id;


--
-- Name: permission_access_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_access_version (
    user_id bigint CONSTRAINT rbac_access_version_user_id_not_null NOT NULL,
    version bigint DEFAULT 1 CONSTRAINT rbac_access_version_version_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_access_version_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_access_version_updated_at_not_null NOT NULL,
    CONSTRAINT ck_rbac_access_version_version CHECK ((version >= 1))
);


--
-- Name: permission_auth_platform; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_auth_platform (
    id bigint NOT NULL,
    code character varying(49) NOT NULL,
    name character varying(64) NOT NULL,
    policy_version bigint DEFAULT 1 NOT NULL,
    access_ttl_seconds integer NOT NULL,
    refresh_ttl_seconds integer NOT NULL,
    session_cache_ttl_seconds integer NOT NULL,
    access_cache_ttl_seconds integer NOT NULL,
    bind_device smallint NOT NULL,
    bind_ip smallint NOT NULL,
    max_sessions smallint NOT NULL,
    allow_register smallint NOT NULL,
    is_enabled smallint NOT NULL,
    is_builtin smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    login_types jsonb DEFAULT '["email", "password"]'::jsonb NOT NULL,
    menu_version bigint DEFAULT 1 NOT NULL,
    CONSTRAINT ck_permission_auth_platform_access_cache_ttl_seconds CHECK (((access_cache_ttl_seconds >= 60) AND (access_cache_ttl_seconds <= 86400))),
    CONSTRAINT ck_permission_auth_platform_access_ttl_seconds CHECK (((access_ttl_seconds >= 60) AND (access_ttl_seconds <= 2592000))),
    CONSTRAINT ck_permission_auth_platform_allow_register CHECK ((allow_register = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_auth_platform_bind_device CHECK ((bind_device = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_auth_platform_bind_ip CHECK ((bind_ip = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_auth_platform_code CHECK (((code)::text ~ '^[a-z][a-z0-9_]{1,48}$'::text)),
    CONSTRAINT ck_permission_auth_platform_is_builtin CHECK ((is_builtin = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_auth_platform_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_auth_platform_login_types CHECK (((jsonb_typeof(login_types) = 'array'::text) AND ((jsonb_array_length(login_types) >= 1) AND (jsonb_array_length(login_types) <= 3)) AND (login_types <@ '["email", "phone", "password"]'::jsonb) AND (jsonb_array_length(login_types) = ((
CASE
    WHEN (login_types @> '["email"]'::jsonb) THEN 1
    ELSE 0
END +
CASE
    WHEN (login_types @> '["phone"]'::jsonb) THEN 1
    ELSE 0
END) +
CASE
    WHEN (login_types @> '["password"]'::jsonb) THEN 1
    ELSE 0
END)))),
    CONSTRAINT ck_permission_auth_platform_max_sessions CHECK (((max_sessions >= 0) AND (max_sessions <= 100))),
    CONSTRAINT ck_permission_auth_platform_menu_version CHECK ((menu_version >= 1)),
    CONSTRAINT ck_permission_auth_platform_policy_version CHECK ((policy_version >= 1)),
    CONSTRAINT ck_permission_auth_platform_refresh_ttl_seconds CHECK (((refresh_ttl_seconds >= 60) AND (refresh_ttl_seconds <= 31536000))),
    CONSTRAINT ck_permission_auth_platform_session_cache_ttl_seconds CHECK (((session_cache_ttl_seconds >= 60) AND (session_cache_ttl_seconds <= 86400)))
);


--
-- Name: permission_auth_platform_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.permission_auth_platform_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_auth_platform_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.permission_auth_platform_id_seq OWNED BY public.permission_auth_platform.id;


--
-- Name: permission_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_menu (
    id bigint CONSTRAINT rbac_menu_id_not_null NOT NULL,
    parent_id bigint,
    menu_type character varying(16) CONSTRAINT rbac_menu_menu_type_not_null NOT NULL,
    code character varying(128) CONSTRAINT rbac_menu_code_not_null NOT NULL,
    i18n_key character varying(128),
    path character varying(255),
    icon character varying(128),
    sort_order integer DEFAULT 0 CONSTRAINT rbac_menu_sort_order_not_null NOT NULL,
    is_enabled smallint DEFAULT 1 CONSTRAINT rbac_menu_is_enabled_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_menu_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_menu_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    component_path character varying(255),
    is_hidden smallint DEFAULT 0 CONSTRAINT rbac_menu_is_hidden_not_null NOT NULL,
    name character varying(128) CONSTRAINT rbac_menu_name_not_null NOT NULL,
    platform_id bigint CONSTRAINT rbac_menu_platform_id_not_null NOT NULL,
    remark character varying(512),
    CONSTRAINT ck_rbac_menu_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_rbac_menu_is_hidden CHECK ((is_hidden = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_rbac_menu_shape CHECK (((btrim((name)::text) <> ''::text) AND ((((menu_type)::text = 'directory'::text) AND (i18n_key IS NOT NULL) AND (path IS NULL) AND (component_path IS NULL)) OR (((menu_type)::text = 'page'::text) AND (i18n_key IS NOT NULL) AND (path IS NOT NULL) AND (btrim((path)::text) <> ''::text) AND (component_path IS NOT NULL) AND (btrim((component_path)::text) <> ''::text)) OR (((menu_type)::text = 'action'::text) AND (i18n_key IS NULL) AND (path IS NULL) AND (component_path IS NULL) AND (icon IS NULL) AND (is_hidden = 1))))),
    CONSTRAINT ck_rbac_menu_sort_order CHECK ((sort_order >= 0)),
    CONSTRAINT ck_rbac_menu_type CHECK (((menu_type)::text = ANY ((ARRAY['directory'::character varying, 'page'::character varying, 'action'::character varying])::text[])))
);


--
-- Name: permission_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_role (
    id bigint CONSTRAINT rbac_role_id_not_null NOT NULL,
    code character varying(64) CONSTRAINT rbac_role_code_not_null NOT NULL,
    name character varying(64) CONSTRAINT rbac_role_name_not_null NOT NULL,
    is_default smallint DEFAULT 0 CONSTRAINT rbac_role_is_default_not_null NOT NULL,
    is_enabled smallint DEFAULT 1 CONSTRAINT rbac_role_is_enabled_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_role_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_role_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_rbac_role_is_default CHECK ((is_default = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_rbac_role_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1])))
);


--
-- Name: permission_role_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_role_menu (
    id bigint CONSTRAINT rbac_role_menu_id_not_null NOT NULL,
    role_id bigint CONSTRAINT rbac_role_menu_role_id_not_null NOT NULL,
    menu_id bigint CONSTRAINT rbac_role_menu_menu_id_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_role_menu_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_role_menu_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_user_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permission_user_role (
    id bigint CONSTRAINT rbac_user_role_id_not_null NOT NULL,
    user_id bigint CONSTRAINT rbac_user_role_user_id_not_null NOT NULL,
    role_id bigint CONSTRAINT rbac_user_role_role_id_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_user_role_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT rbac_user_role_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: rbac_access_version_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rbac_access_version_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rbac_access_version_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rbac_access_version_user_id_seq OWNED BY public.permission_access_version.user_id;


--
-- Name: rbac_menu_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rbac_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rbac_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rbac_menu_id_seq OWNED BY public.permission_menu.id;


--
-- Name: rbac_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rbac_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rbac_role_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rbac_role_id_seq OWNED BY public.permission_role.id;


--
-- Name: rbac_role_menu_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rbac_role_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rbac_role_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rbac_role_menu_id_seq OWNED BY public.permission_role_menu.id;


--
-- Name: rbac_user_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rbac_user_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rbac_user_role_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rbac_user_role_id_seq OWNED BY public.permission_user_role.id;


--
-- Name: storage_cos_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_cos_config (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    app_id character varying(32) NOT NULL,
    secret_id_ciphertext text NOT NULL,
    secret_key_ciphertext text NOT NULL,
    bucket character varying(128) NOT NULL,
    region character varying(64) NOT NULL,
    endpoint character varying(255),
    bucket_domain character varying(255),
    is_enabled smallint DEFAULT 1 NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_storage_cos_config_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1])))
);


--
-- Name: storage_cos_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.storage_cos_config ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.storage_cos_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: storage_upload_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_upload_rule (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    name character varying(128) NOT NULL,
    cos_config_id bigint NOT NULL,
    max_file_size_bytes bigint NOT NULL,
    allowed_extensions text[] NOT NULL,
    allowed_mime_types text[] NOT NULL,
    access_mode character varying(16) DEFAULT 'private'::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_storage_upload_rule_access_mode CHECK (((access_mode)::text = ANY ((ARRAY['private'::character varying, 'public'::character varying])::text[]))),
    CONSTRAINT ck_storage_upload_rule_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_storage_upload_rule_max_file_size CHECK ((max_file_size_bytes > 0))
);


--
-- Name: storage_upload_rule_code; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_upload_rule_code (
    id bigint NOT NULL,
    rule_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    code character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_storage_upload_rule_code_value CHECK ((length(btrim((code)::text)) > 0))
);


--
-- Name: storage_upload_rule_code_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.storage_upload_rule_code ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.storage_upload_rule_code_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: storage_upload_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.storage_upload_rule ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.storage_upload_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_account; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_account (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(254) NOT NULL,
    password_hash character varying(255) NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    phone character varying(32),
    CONSTRAINT ck_user_account_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1])))
);


--
-- Name: user_account_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_account_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_account_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_account_id_seq OWNED BY public.user_account.id;


--
-- Name: user_login_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_login_log (
    id bigint NOT NULL,
    user_id bigint,
    session_id bigint,
    platform_id bigint NOT NULL,
    login_account character varying(254) NOT NULL,
    event_type character varying(16) NOT NULL,
    login_type character varying(32),
    is_success smallint NOT NULL,
    reason_code character varying(64) NOT NULL,
    client_ip character varying(64) NOT NULL,
    user_agent character varying(512) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_user_login_log_event_type CHECK (((event_type)::text = ANY ((ARRAY['login'::character varying, 'logout'::character varying])::text[]))),
    CONSTRAINT ck_user_login_log_is_success CHECK ((is_success = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_user_login_log_login_type CHECK (((((event_type)::text = 'login'::text) AND (login_type IS NOT NULL)) OR (((event_type)::text = 'logout'::text) AND (login_type IS NULL))))
);


--
-- Name: user_login_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_login_log ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.user_login_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_profile; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profile (
    user_id bigint NOT NULL,
    birthday date,
    gender smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    avatar character varying(512) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT ck_user_profile_gender CHECK ((gender = ANY (ARRAY[0, 1, 2])))
);


--
-- Name: auth_platform; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.auth_platform (
    id bigint NOT NULL,
    code character varying(49) NOT NULL,
    name character varying(64) NOT NULL,
    policy_version bigint DEFAULT 1 NOT NULL,
    access_ttl_seconds integer NOT NULL,
    refresh_ttl_seconds integer NOT NULL,
    session_cache_ttl_seconds integer NOT NULL,
    access_cache_ttl_seconds integer NOT NULL,
    bind_device smallint NOT NULL,
    bind_ip smallint NOT NULL,
    max_sessions smallint NOT NULL,
    allow_register smallint NOT NULL,
    is_enabled smallint NOT NULL,
    is_builtin smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_auth_platform_access_cache_ttl_seconds CHECK (((access_cache_ttl_seconds >= 60) AND (access_cache_ttl_seconds <= 86400))),
    CONSTRAINT ck_auth_platform_access_ttl_seconds CHECK (((access_ttl_seconds >= 60) AND (access_ttl_seconds <= 2592000))),
    CONSTRAINT ck_auth_platform_allow_register CHECK ((allow_register = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_device CHECK ((bind_device = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_ip CHECK ((bind_ip = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_code CHECK (((code)::text ~ '^[a-z][a-z0-9_]{1,48}$'::text)),
    CONSTRAINT ck_auth_platform_is_builtin CHECK ((is_builtin = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_max_sessions CHECK (((max_sessions >= 0) AND (max_sessions <= 100))),
    CONSTRAINT ck_auth_platform_policy_version CHECK ((policy_version >= 1)),
    CONSTRAINT ck_auth_platform_refresh_ttl_seconds CHECK (((refresh_ttl_seconds >= 60) AND (refresh_ttl_seconds <= 31536000))),
    CONSTRAINT ck_auth_platform_session_cache_ttl_seconds CHECK (((session_cache_ttl_seconds >= 60) AND (session_cache_ttl_seconds <= 86400)))
);


--
-- Name: auth_platform_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.auth_platform_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: auth_platform_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.auth_platform_id_seq OWNED BY test_menu_1788437234335272500.auth_platform.id;


--
-- Name: permission_access_version; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.permission_access_version (
    user_id bigint NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.permission_access_version_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.permission_access_version_user_id_seq OWNED BY test_menu_1788437234335272500.permission_access_version.user_id;


--
-- Name: permission_menu; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.permission_menu (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    parent_id bigint,
    menu_type character varying(16) NOT NULL,
    name character varying(128) NOT NULL,
    code character varying(128) NOT NULL,
    i18n_key character varying(128),
    path character varying(255),
    component_path character varying(255),
    icon character varying(128),
    remark character varying(512),
    sort_order integer DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    is_hidden smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_permission_menu_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_menu_is_hidden CHECK ((is_hidden = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_menu_shape CHECK (((btrim((name)::text) <> ''::text) AND ((((menu_type)::text = 'directory'::text) AND (i18n_key IS NOT NULL) AND (path IS NULL) AND (component_path IS NULL)) OR (((menu_type)::text = 'page'::text) AND (i18n_key IS NOT NULL) AND (path IS NOT NULL) AND (btrim((path)::text) <> ''::text) AND (component_path IS NOT NULL) AND (btrim((component_path)::text) <> ''::text)) OR (((menu_type)::text = 'action'::text) AND (i18n_key IS NULL) AND (path IS NULL) AND (component_path IS NULL) AND (icon IS NULL) AND (is_hidden = 1))))),
    CONSTRAINT ck_permission_menu_sort_order CHECK ((sort_order >= 0)),
    CONSTRAINT ck_permission_menu_type CHECK (((menu_type)::text = ANY ((ARRAY['directory'::character varying, 'page'::character varying, 'action'::character varying])::text[])))
);


--
-- Name: permission_menu_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.permission_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.permission_menu_id_seq OWNED BY test_menu_1788437234335272500.permission_menu.id;


--
-- Name: permission_role; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.permission_role (
    id bigint NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    is_default smallint DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_role_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.permission_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_role_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.permission_role_id_seq OWNED BY test_menu_1788437234335272500.permission_role.id;


--
-- Name: permission_role_menu; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.permission_role_menu (
    id bigint NOT NULL,
    role_id bigint NOT NULL,
    menu_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_role_menu_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.permission_role_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_role_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.permission_role_menu_id_seq OWNED BY test_menu_1788437234335272500.permission_role_menu.id;


--
-- Name: user_account; Type: TABLE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE TABLE test_menu_1788437234335272500.user_account (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(254) NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: user_account_id_seq; Type: SEQUENCE; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE SEQUENCE test_menu_1788437234335272500.user_account_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_account_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER SEQUENCE test_menu_1788437234335272500.user_account_id_seq OWNED BY test_menu_1788437234335272500.user_account.id;


--
-- Name: auth_platform; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.auth_platform (
    id bigint NOT NULL,
    code character varying(49) NOT NULL,
    name character varying(64) NOT NULL,
    policy_version bigint DEFAULT 1 NOT NULL,
    access_ttl_seconds integer NOT NULL,
    refresh_ttl_seconds integer NOT NULL,
    session_cache_ttl_seconds integer NOT NULL,
    access_cache_ttl_seconds integer NOT NULL,
    bind_device smallint NOT NULL,
    bind_ip smallint NOT NULL,
    max_sessions smallint NOT NULL,
    allow_register smallint NOT NULL,
    is_enabled smallint NOT NULL,
    is_builtin smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_auth_platform_access_cache_ttl_seconds CHECK (((access_cache_ttl_seconds >= 60) AND (access_cache_ttl_seconds <= 86400))),
    CONSTRAINT ck_auth_platform_access_ttl_seconds CHECK (((access_ttl_seconds >= 60) AND (access_ttl_seconds <= 2592000))),
    CONSTRAINT ck_auth_platform_allow_register CHECK ((allow_register = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_device CHECK ((bind_device = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_ip CHECK ((bind_ip = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_code CHECK (((code)::text ~ '^[a-z][a-z0-9_]{1,48}$'::text)),
    CONSTRAINT ck_auth_platform_is_builtin CHECK ((is_builtin = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_max_sessions CHECK (((max_sessions >= 0) AND (max_sessions <= 100))),
    CONSTRAINT ck_auth_platform_policy_version CHECK ((policy_version >= 1)),
    CONSTRAINT ck_auth_platform_refresh_ttl_seconds CHECK (((refresh_ttl_seconds >= 60) AND (refresh_ttl_seconds <= 31536000))),
    CONSTRAINT ck_auth_platform_session_cache_ttl_seconds CHECK (((session_cache_ttl_seconds >= 60) AND (session_cache_ttl_seconds <= 86400)))
);


--
-- Name: auth_platform_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.auth_platform_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: auth_platform_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.auth_platform_id_seq OWNED BY test_menu_1788437254379752200.auth_platform.id;


--
-- Name: permission_access_version; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.permission_access_version (
    user_id bigint NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.permission_access_version_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.permission_access_version_user_id_seq OWNED BY test_menu_1788437254379752200.permission_access_version.user_id;


--
-- Name: permission_menu; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.permission_menu (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    parent_id bigint,
    menu_type character varying(16) NOT NULL,
    name character varying(128) NOT NULL,
    code character varying(128) NOT NULL,
    i18n_key character varying(128),
    path character varying(255),
    component_path character varying(255),
    icon character varying(128),
    remark character varying(512),
    sort_order integer DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    is_hidden smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_permission_menu_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_menu_is_hidden CHECK ((is_hidden = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_menu_shape CHECK (((btrim((name)::text) <> ''::text) AND ((((menu_type)::text = 'directory'::text) AND (i18n_key IS NOT NULL) AND (path IS NULL) AND (component_path IS NULL)) OR (((menu_type)::text = 'page'::text) AND (i18n_key IS NOT NULL) AND (path IS NOT NULL) AND (btrim((path)::text) <> ''::text) AND (component_path IS NOT NULL) AND (btrim((component_path)::text) <> ''::text)) OR (((menu_type)::text = 'action'::text) AND (i18n_key IS NULL) AND (path IS NULL) AND (component_path IS NULL) AND (icon IS NULL) AND (is_hidden = 1))))),
    CONSTRAINT ck_permission_menu_sort_order CHECK ((sort_order >= 0)),
    CONSTRAINT ck_permission_menu_type CHECK (((menu_type)::text = ANY ((ARRAY['directory'::character varying, 'page'::character varying, 'action'::character varying])::text[])))
);


--
-- Name: permission_menu_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.permission_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.permission_menu_id_seq OWNED BY test_menu_1788437254379752200.permission_menu.id;


--
-- Name: permission_role; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.permission_role (
    id bigint NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    is_default smallint DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_role_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.permission_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_role_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.permission_role_id_seq OWNED BY test_menu_1788437254379752200.permission_role.id;


--
-- Name: permission_role_menu; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.permission_role_menu (
    id bigint NOT NULL,
    role_id bigint NOT NULL,
    menu_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_role_menu_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.permission_role_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_role_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.permission_role_menu_id_seq OWNED BY test_menu_1788437254379752200.permission_role_menu.id;


--
-- Name: user_account; Type: TABLE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE TABLE test_menu_1788437254379752200.user_account (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(254) NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: user_account_id_seq; Type: SEQUENCE; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE SEQUENCE test_menu_1788437254379752200.user_account_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_account_id_seq; Type: SEQUENCE OWNED BY; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER SEQUENCE test_menu_1788437254379752200.user_account_id_seq OWNED BY test_menu_1788437254379752200.user_account.id;


--
-- Name: auth_platform; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.auth_platform (
    id bigint NOT NULL,
    code character varying(49) NOT NULL,
    name character varying(64) NOT NULL,
    policy_version bigint DEFAULT 1 NOT NULL,
    access_ttl_seconds integer NOT NULL,
    refresh_ttl_seconds integer NOT NULL,
    session_cache_ttl_seconds integer NOT NULL,
    access_cache_ttl_seconds integer NOT NULL,
    bind_device smallint NOT NULL,
    bind_ip smallint NOT NULL,
    max_sessions smallint NOT NULL,
    allow_register smallint NOT NULL,
    is_enabled smallint NOT NULL,
    is_builtin smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_auth_platform_access_cache_ttl_seconds CHECK (((access_cache_ttl_seconds >= 60) AND (access_cache_ttl_seconds <= 86400))),
    CONSTRAINT ck_auth_platform_access_ttl_seconds CHECK (((access_ttl_seconds >= 60) AND (access_ttl_seconds <= 2592000))),
    CONSTRAINT ck_auth_platform_allow_register CHECK ((allow_register = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_device CHECK ((bind_device = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_bind_ip CHECK ((bind_ip = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_code CHECK (((code)::text ~ '^[a-z][a-z0-9_]{1,48}$'::text)),
    CONSTRAINT ck_auth_platform_is_builtin CHECK ((is_builtin = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_auth_platform_max_sessions CHECK (((max_sessions >= 0) AND (max_sessions <= 100))),
    CONSTRAINT ck_auth_platform_policy_version CHECK ((policy_version >= 1)),
    CONSTRAINT ck_auth_platform_refresh_ttl_seconds CHECK (((refresh_ttl_seconds >= 60) AND (refresh_ttl_seconds <= 31536000))),
    CONSTRAINT ck_auth_platform_session_cache_ttl_seconds CHECK (((session_cache_ttl_seconds >= 60) AND (session_cache_ttl_seconds <= 86400)))
);


--
-- Name: auth_platform_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.auth_platform_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: auth_platform_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.auth_platform_id_seq OWNED BY test_user_session_1788437240066425200.auth_platform.id;


--
-- Name: permission_access_version; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.permission_access_version (
    user_id bigint NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.permission_access_version_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_access_version_user_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.permission_access_version_user_id_seq OWNED BY test_user_session_1788437240066425200.permission_access_version.user_id;


--
-- Name: permission_role; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.permission_role (
    id bigint NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    is_default smallint DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_permission_role_is_default CHECK ((is_default = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_permission_role_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1])))
);


--
-- Name: permission_role_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.permission_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_role_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.permission_role_id_seq OWNED BY test_user_session_1788437240066425200.permission_role.id;


--
-- Name: permission_user_role; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.permission_user_role (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: permission_user_role_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.permission_user_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permission_user_role_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.permission_user_role_id_seq OWNED BY test_user_session_1788437240066425200.permission_user_role.id;


--
-- Name: user_account; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.user_account (
    id bigint NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(254) NOT NULL,
    phone character varying(32),
    password_hash character varying(255) NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_user_account_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1])))
);


--
-- Name: user_account_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.user_account_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_account_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.user_account_id_seq OWNED BY test_user_session_1788437240066425200.user_account.id;


--
-- Name: user_session; Type: TABLE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE TABLE test_user_session_1788437240066425200.user_session (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    device_id character varying(36) NOT NULL,
    refresh_token_hash character(64) NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    client_ip character varying(64) NOT NULL,
    user_agent character varying(512) NOT NULL,
    refresh_expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_user_session_version CHECK ((version >= 1))
);


--
-- Name: user_session_id_seq; Type: SEQUENCE; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE SEQUENCE test_user_session_1788437240066425200.user_session_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_session_id_seq; Type: SEQUENCE OWNED BY; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER SEQUENCE test_user_session_1788437240066425200.user_session_id_seq OWNED BY test_user_session_1788437240066425200.user_session.id;


--
-- Name: message_mail_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_config ALTER COLUMN id SET DEFAULT nextval('public.message_mail_config_id_seq'::regclass);


--
-- Name: message_mail_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log ALTER COLUMN id SET DEFAULT nextval('public.message_mail_log_id_seq'::regclass);


--
-- Name: message_mail_log_verification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log_verification ALTER COLUMN id SET DEFAULT nextval('public.message_mail_log_verification_id_seq'::regclass);


--
-- Name: message_mail_recipient_rule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_recipient_rule ALTER COLUMN id SET DEFAULT nextval('public.message_mail_recipient_rule_id_seq'::regclass);


--
-- Name: message_mail_template id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_template ALTER COLUMN id SET DEFAULT nextval('public.message_mail_template_id_seq'::regclass);


--
-- Name: permission_access_version user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_access_version ALTER COLUMN user_id SET DEFAULT nextval('public.rbac_access_version_user_id_seq'::regclass);


--
-- Name: permission_auth_platform id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_auth_platform ALTER COLUMN id SET DEFAULT nextval('public.permission_auth_platform_id_seq'::regclass);


--
-- Name: permission_menu id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu ALTER COLUMN id SET DEFAULT nextval('public.rbac_menu_id_seq'::regclass);


--
-- Name: permission_role id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role ALTER COLUMN id SET DEFAULT nextval('public.rbac_role_id_seq'::regclass);


--
-- Name: permission_role_menu id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role_menu ALTER COLUMN id SET DEFAULT nextval('public.rbac_role_menu_id_seq'::regclass);


--
-- Name: permission_user_role id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_user_role ALTER COLUMN id SET DEFAULT nextval('public.rbac_user_role_id_seq'::regclass);


--
-- Name: system_operation_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_operation_log ALTER COLUMN id SET DEFAULT nextval('public.audit_operation_log_id_seq'::regclass);


--
-- Name: user_account id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_account ALTER COLUMN id SET DEFAULT nextval('public.user_account_id_seq'::regclass);


--
-- Name: user_session id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session ALTER COLUMN id SET DEFAULT nextval('public.auth_session_id_seq'::regclass);


--
-- Name: auth_platform id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.auth_platform ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437234335272500.auth_platform_id_seq'::regclass);


--
-- Name: permission_access_version user_id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_access_version ALTER COLUMN user_id SET DEFAULT nextval('test_menu_1788437234335272500.permission_access_version_user_id_seq'::regclass);


--
-- Name: permission_menu id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_menu ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437234335272500.permission_menu_id_seq'::regclass);


--
-- Name: permission_role id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437234335272500.permission_role_id_seq'::regclass);


--
-- Name: permission_role_menu id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role_menu ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437234335272500.permission_role_menu_id_seq'::regclass);


--
-- Name: user_account id; Type: DEFAULT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.user_account ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437234335272500.user_account_id_seq'::regclass);


--
-- Name: auth_platform id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.auth_platform ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437254379752200.auth_platform_id_seq'::regclass);


--
-- Name: permission_access_version user_id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_access_version ALTER COLUMN user_id SET DEFAULT nextval('test_menu_1788437254379752200.permission_access_version_user_id_seq'::regclass);


--
-- Name: permission_menu id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_menu ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437254379752200.permission_menu_id_seq'::regclass);


--
-- Name: permission_role id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437254379752200.permission_role_id_seq'::regclass);


--
-- Name: permission_role_menu id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role_menu ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437254379752200.permission_role_menu_id_seq'::regclass);


--
-- Name: user_account id; Type: DEFAULT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.user_account ALTER COLUMN id SET DEFAULT nextval('test_menu_1788437254379752200.user_account_id_seq'::regclass);


--
-- Name: auth_platform id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.auth_platform ALTER COLUMN id SET DEFAULT nextval('test_user_session_1788437240066425200.auth_platform_id_seq'::regclass);


--
-- Name: permission_access_version user_id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_access_version ALTER COLUMN user_id SET DEFAULT nextval('test_user_session_1788437240066425200.permission_access_version_user_id_seq'::regclass);


--
-- Name: permission_role id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_role ALTER COLUMN id SET DEFAULT nextval('test_user_session_1788437240066425200.permission_role_id_seq'::regclass);


--
-- Name: permission_user_role id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_user_role ALTER COLUMN id SET DEFAULT nextval('test_user_session_1788437240066425200.permission_user_role_id_seq'::regclass);


--
-- Name: user_account id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.user_account ALTER COLUMN id SET DEFAULT nextval('test_user_session_1788437240066425200.user_account_id_seq'::regclass);


--
-- Name: user_session id; Type: DEFAULT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.user_session ALTER COLUMN id SET DEFAULT nextval('test_user_session_1788437240066425200.user_session_id_seq'::regclass);


--
-- Name: system_operation_log audit_operation_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_operation_log
    ADD CONSTRAINT audit_operation_log_pkey PRIMARY KEY (id);


--
-- Name: user_session auth_session_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session
    ADD CONSTRAINT auth_session_pkey PRIMARY KEY (id);


--
-- Name: message_mail_config message_mail_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_config
    ADD CONSTRAINT message_mail_config_pkey PRIMARY KEY (id);


--
-- Name: message_mail_log message_mail_log_id_platform_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log
    ADD CONSTRAINT message_mail_log_id_platform_id_key UNIQUE (id, platform_id);


--
-- Name: message_mail_log message_mail_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log
    ADD CONSTRAINT message_mail_log_pkey PRIMARY KEY (id);


--
-- Name: message_mail_log_verification message_mail_log_verification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log_verification
    ADD CONSTRAINT message_mail_log_verification_pkey PRIMARY KEY (id);


--
-- Name: message_mail_rate_limit_policy message_mail_rate_limit_policy_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_rate_limit_policy
    ADD CONSTRAINT message_mail_rate_limit_policy_pkey PRIMARY KEY (platform_id, policy_key);


--
-- Name: message_mail_recipient_rule message_mail_recipient_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_recipient_rule
    ADD CONSTRAINT message_mail_recipient_rule_pkey PRIMARY KEY (id);


--
-- Name: message_mail_template message_mail_template_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_template
    ADD CONSTRAINT message_mail_template_pkey PRIMARY KEY (id);


--
-- Name: permission_auth_platform permission_auth_platform_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_auth_platform
    ADD CONSTRAINT permission_auth_platform_pkey PRIMARY KEY (id);


--
-- Name: permission_access_version rbac_access_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_access_version
    ADD CONSTRAINT rbac_access_version_pkey PRIMARY KEY (user_id);


--
-- Name: permission_menu rbac_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu
    ADD CONSTRAINT rbac_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role_menu rbac_role_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role_menu
    ADD CONSTRAINT rbac_role_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role rbac_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role
    ADD CONSTRAINT rbac_role_pkey PRIMARY KEY (id);


--
-- Name: permission_user_role rbac_user_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_user_role
    ADD CONSTRAINT rbac_user_role_pkey PRIMARY KEY (id);


--
-- Name: storage_cos_config storage_cos_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_cos_config
    ADD CONSTRAINT storage_cos_config_pkey PRIMARY KEY (id);


--
-- Name: storage_upload_rule_code storage_upload_rule_code_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule_code
    ADD CONSTRAINT storage_upload_rule_code_pkey PRIMARY KEY (id);


--
-- Name: storage_upload_rule storage_upload_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule
    ADD CONSTRAINT storage_upload_rule_pkey PRIMARY KEY (id);


--
-- Name: permission_menu uq_rbac_menu_id_platform; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu
    ADD CONSTRAINT uq_rbac_menu_id_platform UNIQUE (id, platform_id);


--
-- Name: user_account user_account_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_account
    ADD CONSTRAINT user_account_pkey PRIMARY KEY (id);


--
-- Name: user_login_log user_login_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT user_login_log_pkey PRIMARY KEY (id);


--
-- Name: user_profile user_profile_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profile
    ADD CONSTRAINT user_profile_pkey PRIMARY KEY (user_id);


--
-- Name: auth_platform auth_platform_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.auth_platform
    ADD CONSTRAINT auth_platform_pkey PRIMARY KEY (id);


--
-- Name: permission_access_version permission_access_version_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_access_version
    ADD CONSTRAINT permission_access_version_pkey PRIMARY KEY (user_id);


--
-- Name: permission_menu permission_menu_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_menu
    ADD CONSTRAINT permission_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role_menu permission_role_menu_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role_menu
    ADD CONSTRAINT permission_role_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role permission_role_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role
    ADD CONSTRAINT permission_role_pkey PRIMARY KEY (id);


--
-- Name: permission_menu uq_permission_menu_id_platform; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_menu
    ADD CONSTRAINT uq_permission_menu_id_platform UNIQUE (id, platform_id);


--
-- Name: user_account user_account_pkey; Type: CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.user_account
    ADD CONSTRAINT user_account_pkey PRIMARY KEY (id);


--
-- Name: auth_platform auth_platform_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.auth_platform
    ADD CONSTRAINT auth_platform_pkey PRIMARY KEY (id);


--
-- Name: permission_access_version permission_access_version_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_access_version
    ADD CONSTRAINT permission_access_version_pkey PRIMARY KEY (user_id);


--
-- Name: permission_menu permission_menu_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_menu
    ADD CONSTRAINT permission_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role_menu permission_role_menu_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role_menu
    ADD CONSTRAINT permission_role_menu_pkey PRIMARY KEY (id);


--
-- Name: permission_role permission_role_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role
    ADD CONSTRAINT permission_role_pkey PRIMARY KEY (id);


--
-- Name: permission_menu uq_permission_menu_id_platform; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_menu
    ADD CONSTRAINT uq_permission_menu_id_platform UNIQUE (id, platform_id);


--
-- Name: user_account user_account_pkey; Type: CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.user_account
    ADD CONSTRAINT user_account_pkey PRIMARY KEY (id);


--
-- Name: auth_platform auth_platform_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.auth_platform
    ADD CONSTRAINT auth_platform_pkey PRIMARY KEY (id);


--
-- Name: permission_access_version permission_access_version_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_access_version
    ADD CONSTRAINT permission_access_version_pkey PRIMARY KEY (user_id);


--
-- Name: permission_role permission_role_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_role
    ADD CONSTRAINT permission_role_pkey PRIMARY KEY (id);


--
-- Name: permission_user_role permission_user_role_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_user_role
    ADD CONSTRAINT permission_user_role_pkey PRIMARY KEY (id);


--
-- Name: user_account user_account_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.user_account
    ADD CONSTRAINT user_account_pkey PRIMARY KEY (id);


--
-- Name: user_session user_session_pkey; Type: CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.user_session
    ADD CONSTRAINT user_session_pkey PRIMARY KEY (id);


--
-- Name: ix_audit_operation_log_action_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audit_operation_log_action_created_at ON public.system_operation_log USING btree (action, created_at DESC);


--
-- Name: ix_audit_operation_log_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audit_operation_log_created_at ON public.system_operation_log USING btree (created_at DESC);


--
-- Name: ix_audit_operation_log_request_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audit_operation_log_request_id ON public.system_operation_log USING btree (request_id);


--
-- Name: ix_audit_operation_log_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audit_operation_log_user_created_at ON public.system_operation_log USING btree (user_id, created_at DESC);


--
-- Name: ix_message_mail_rate_limit_policy_platform_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_rate_limit_policy_platform_revision ON public.message_mail_rate_limit_policy USING btree (platform_id, revision);


--
-- Name: ix_rbac_menu_parent_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rbac_menu_parent_active ON public.permission_menu USING btree (platform_id, parent_id, sort_order, id) WHERE (deleted_at IS NULL);


--
-- Name: ix_rbac_menu_platform_parent_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rbac_menu_platform_parent_sort ON public.permission_menu USING btree (platform_id, parent_id, sort_order, id);


--
-- Name: ix_storage_cos_config_enabled_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_cos_config_enabled_created_at ON public.storage_cos_config USING btree (is_enabled, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_storage_upload_rule_code_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_upload_rule_code_rule ON public.storage_upload_rule_code USING btree (rule_id, id) WHERE (deleted_at IS NULL);


--
-- Name: ix_storage_upload_rule_config_enabled_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_upload_rule_config_enabled_created_at ON public.storage_upload_rule USING btree (cos_config_id, is_enabled, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_user_login_log_account_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_login_log_account_created_at ON public.user_login_log USING btree (login_account, created_at DESC);


--
-- Name: ix_user_login_log_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_login_log_created_at ON public.user_login_log USING btree (created_at DESC);


--
-- Name: ix_user_login_log_platform_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_login_log_platform_created_at ON public.user_login_log USING btree (platform_id, created_at DESC);


--
-- Name: ix_user_login_log_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_login_log_user_created_at ON public.user_login_log USING btree (user_id, created_at DESC);


--
-- Name: ix_user_session_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_session_user_created_at ON public.user_session USING btree (user_id, created_at DESC);


--
-- Name: ix_user_session_user_platform_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_session_user_platform_created_at ON public.user_session USING btree (user_id, platform_id, created_at DESC, id DESC) WHERE (revoked_at IS NULL);


--
-- Name: ux_audit_operation_log_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_audit_operation_log_event_id ON public.system_operation_log USING btree (event_id);


--
-- Name: ux_message_mail_config_active_singleton; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_config_active_singleton ON public.message_mail_config USING btree ((true)) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_mail_log_platform_challenge_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_log_platform_challenge_active ON public.message_mail_log USING btree (platform_id, challenge_id) WHERE ((deleted_at IS NULL) AND (challenge_id IS NOT NULL));


--
-- Name: ux_message_mail_rule_scope_pattern_action_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_rule_scope_pattern_action_active ON public.message_mail_recipient_rule USING btree (scope, pattern, action) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_mail_template_scene_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_template_scene_active ON public.message_mail_template USING btree (scene) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_mail_verification_log_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_verification_log_active ON public.message_mail_log_verification USING btree (mail_log_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_auth_platform_code_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_permission_auth_platform_code_active ON public.permission_auth_platform USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_menu_code_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_menu_code_active ON public.permission_menu USING btree (platform_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_menu_page_path_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_menu_page_path_active ON public.permission_menu USING btree (platform_id, path) WHERE ((deleted_at IS NULL) AND ((menu_type)::text = 'page'::text));


--
-- Name: ux_rbac_menu_platform_code_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_menu_platform_code_active ON public.permission_menu USING btree (platform_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_menu_platform_path_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_menu_platform_path_active ON public.permission_menu USING btree (platform_id, path) WHERE ((path IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: ux_rbac_role_code_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_role_code_active ON public.permission_role USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_role_default_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_role_default_active ON public.permission_role USING btree (is_default) WHERE ((is_default = 1) AND (deleted_at IS NULL));


--
-- Name: ux_rbac_role_menu_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_role_menu_active ON public.permission_role_menu USING btree (role_id, menu_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_role_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_role_name_active ON public.permission_role USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: ux_rbac_user_role_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_rbac_user_role_active ON public.permission_user_role USING btree (user_id, role_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_storage_cos_config_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_cos_config_name_active ON public.storage_cos_config USING btree (lower((name)::text)) WHERE (deleted_at IS NULL);


--
-- Name: ux_storage_upload_rule_code_platform_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_upload_rule_code_platform_code ON public.storage_upload_rule_code USING btree (platform_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_user_account_email_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_user_account_email_active ON public.user_account USING btree (lower((email)::text)) WHERE (((email)::text <> ''::text) AND (deleted_at IS NULL));


--
-- Name: ux_user_account_phone_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_user_account_phone_active ON public.user_account USING btree (phone) WHERE ((phone IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: ux_user_account_username_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_user_account_username_active ON public.user_account USING btree (lower((username)::text)) WHERE (deleted_at IS NULL);


--
-- Name: ux_user_session_refresh_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_user_session_refresh_token_hash ON public.user_session USING btree (refresh_token_hash);


--
-- Name: ix_permission_menu_parent_active; Type: INDEX; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE INDEX ix_permission_menu_parent_active ON test_menu_1788437234335272500.permission_menu USING btree (platform_id, parent_id, sort_order, id) WHERE (deleted_at IS NULL);


--
-- Name: ux_auth_platform_code_active; Type: INDEX; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE UNIQUE INDEX ux_auth_platform_code_active ON test_menu_1788437234335272500.auth_platform USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_menu_code_active; Type: INDEX; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE UNIQUE INDEX ux_permission_menu_code_active ON test_menu_1788437234335272500.permission_menu USING btree (platform_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_menu_page_path_active; Type: INDEX; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE UNIQUE INDEX ux_permission_menu_page_path_active ON test_menu_1788437234335272500.permission_menu USING btree (platform_id, path) WHERE ((deleted_at IS NULL) AND ((menu_type)::text = 'page'::text));


--
-- Name: ux_permission_role_menu_active; Type: INDEX; Schema: test_menu_1788437234335272500; Owner: -
--

CREATE UNIQUE INDEX ux_permission_role_menu_active ON test_menu_1788437234335272500.permission_role_menu USING btree (role_id, menu_id) WHERE (deleted_at IS NULL);


--
-- Name: ix_permission_menu_parent_active; Type: INDEX; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE INDEX ix_permission_menu_parent_active ON test_menu_1788437254379752200.permission_menu USING btree (platform_id, parent_id, sort_order, id) WHERE (deleted_at IS NULL);


--
-- Name: ux_auth_platform_code_active; Type: INDEX; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE UNIQUE INDEX ux_auth_platform_code_active ON test_menu_1788437254379752200.auth_platform USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_menu_code_active; Type: INDEX; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_menu_code_active ON test_menu_1788437254379752200.permission_menu USING btree (platform_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_menu_page_path_active; Type: INDEX; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_menu_page_path_active ON test_menu_1788437254379752200.permission_menu USING btree (platform_id, path) WHERE ((deleted_at IS NULL) AND ((menu_type)::text = 'page'::text));


--
-- Name: ux_permission_role_menu_active; Type: INDEX; Schema: test_menu_1788437254379752200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_role_menu_active ON test_menu_1788437254379752200.permission_role_menu USING btree (role_id, menu_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_auth_platform_code_active; Type: INDEX; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE UNIQUE INDEX ux_auth_platform_code_active ON test_user_session_1788437240066425200.auth_platform USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_role_code_active; Type: INDEX; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_role_code_active ON test_user_session_1788437240066425200.permission_role USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_role_default_active; Type: INDEX; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_role_default_active ON test_user_session_1788437240066425200.permission_role USING btree (is_default) WHERE ((is_default = 1) AND (deleted_at IS NULL));


--
-- Name: ux_permission_role_name_active; Type: INDEX; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_role_name_active ON test_user_session_1788437240066425200.permission_role USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: ux_permission_user_role_active; Type: INDEX; Schema: test_user_session_1788437240066425200; Owner: -
--

CREATE UNIQUE INDEX ux_permission_user_role_active ON test_user_session_1788437240066425200.permission_user_role USING btree (user_id, role_id) WHERE (deleted_at IS NULL);


--
-- Name: system_operation_log fk_audit_operation_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_operation_log
    ADD CONSTRAINT fk_audit_operation_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: user_session fk_auth_session_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session
    ADD CONSTRAINT fk_auth_session_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: permission_access_version fk_rbac_access_version_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_access_version
    ADD CONSTRAINT fk_rbac_access_version_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: permission_menu fk_rbac_menu_parent_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu
    ADD CONSTRAINT fk_rbac_menu_parent_platform FOREIGN KEY (parent_id, platform_id) REFERENCES public.permission_menu(id, platform_id) ON DELETE RESTRICT;


--
-- Name: permission_menu fk_rbac_menu_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu
    ADD CONSTRAINT fk_rbac_menu_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_rbac_role_menu_menu; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role_menu
    ADD CONSTRAINT fk_rbac_role_menu_menu FOREIGN KEY (menu_id) REFERENCES public.permission_menu(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_rbac_role_menu_role; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_role_menu
    ADD CONSTRAINT fk_rbac_role_menu_role FOREIGN KEY (role_id) REFERENCES public.permission_role(id) ON DELETE RESTRICT;


--
-- Name: permission_user_role fk_rbac_user_role_role; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_user_role
    ADD CONSTRAINT fk_rbac_user_role_role FOREIGN KEY (role_id) REFERENCES public.permission_role(id) ON DELETE RESTRICT;


--
-- Name: permission_user_role fk_rbac_user_role_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_user_role
    ADD CONSTRAINT fk_rbac_user_role_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: storage_upload_rule_code fk_storage_upload_rule_code_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule_code
    ADD CONSTRAINT fk_storage_upload_rule_code_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: storage_upload_rule_code fk_storage_upload_rule_code_rule; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule_code
    ADD CONSTRAINT fk_storage_upload_rule_code_rule FOREIGN KEY (rule_id) REFERENCES public.storage_upload_rule(id) ON DELETE CASCADE;


--
-- Name: storage_upload_rule fk_storage_upload_rule_cos_config; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule
    ADD CONSTRAINT fk_storage_upload_rule_cos_config FOREIGN KEY (cos_config_id) REFERENCES public.storage_cos_config(id) ON DELETE RESTRICT;


--
-- Name: storage_upload_rule fk_storage_upload_rule_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_upload_rule
    ADD CONSTRAINT fk_storage_upload_rule_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: user_login_log fk_user_login_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT fk_user_login_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: user_login_log fk_user_login_log_session; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT fk_user_login_log_session FOREIGN KEY (session_id) REFERENCES public.user_session(id) ON DELETE RESTRICT;


--
-- Name: user_login_log fk_user_login_log_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT fk_user_login_log_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: user_profile fk_user_profile_account; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profile
    ADD CONSTRAINT fk_user_profile_account FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: user_session fk_user_session_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session
    ADD CONSTRAINT fk_user_session_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: user_session fk_user_session_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session
    ADD CONSTRAINT fk_user_session_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: message_mail_log message_mail_log_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log
    ADD CONSTRAINT message_mail_log_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: message_mail_log_verification message_mail_log_verification_mail_log_id_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log_verification
    ADD CONSTRAINT message_mail_log_verification_mail_log_id_platform_id_fkey FOREIGN KEY (mail_log_id, platform_id) REFERENCES public.message_mail_log(id, platform_id);


--
-- Name: message_mail_log_verification message_mail_log_verification_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_log_verification
    ADD CONSTRAINT message_mail_log_verification_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: message_mail_rate_limit_policy message_mail_rate_limit_policy_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_mail_rate_limit_policy
    ADD CONSTRAINT message_mail_rate_limit_policy_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: permission_menu fk_permission_menu_parent_platform; Type: FK CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_menu
    ADD CONSTRAINT fk_permission_menu_parent_platform FOREIGN KEY (parent_id, platform_id) REFERENCES test_menu_1788437234335272500.permission_menu(id, platform_id) ON DELETE RESTRICT;


--
-- Name: permission_menu fk_permission_menu_platform; Type: FK CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_menu
    ADD CONSTRAINT fk_permission_menu_platform FOREIGN KEY (platform_id) REFERENCES test_menu_1788437234335272500.auth_platform(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_permission_role_menu_menu; Type: FK CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role_menu
    ADD CONSTRAINT fk_permission_role_menu_menu FOREIGN KEY (menu_id) REFERENCES test_menu_1788437234335272500.permission_menu(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_permission_role_menu_role; Type: FK CONSTRAINT; Schema: test_menu_1788437234335272500; Owner: -
--

ALTER TABLE ONLY test_menu_1788437234335272500.permission_role_menu
    ADD CONSTRAINT fk_permission_role_menu_role FOREIGN KEY (role_id) REFERENCES test_menu_1788437234335272500.permission_role(id) ON DELETE RESTRICT;


--
-- Name: permission_menu fk_permission_menu_parent_platform; Type: FK CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_menu
    ADD CONSTRAINT fk_permission_menu_parent_platform FOREIGN KEY (parent_id, platform_id) REFERENCES test_menu_1788437254379752200.permission_menu(id, platform_id) ON DELETE RESTRICT;


--
-- Name: permission_menu fk_permission_menu_platform; Type: FK CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_menu
    ADD CONSTRAINT fk_permission_menu_platform FOREIGN KEY (platform_id) REFERENCES test_menu_1788437254379752200.auth_platform(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_permission_role_menu_menu; Type: FK CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role_menu
    ADD CONSTRAINT fk_permission_role_menu_menu FOREIGN KEY (menu_id) REFERENCES test_menu_1788437254379752200.permission_menu(id) ON DELETE RESTRICT;


--
-- Name: permission_role_menu fk_permission_role_menu_role; Type: FK CONSTRAINT; Schema: test_menu_1788437254379752200; Owner: -
--

ALTER TABLE ONLY test_menu_1788437254379752200.permission_role_menu
    ADD CONSTRAINT fk_permission_role_menu_role FOREIGN KEY (role_id) REFERENCES test_menu_1788437254379752200.permission_role(id) ON DELETE RESTRICT;


--
-- Name: permission_user_role fk_permission_user_role_role; Type: FK CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_user_role
    ADD CONSTRAINT fk_permission_user_role_role FOREIGN KEY (role_id) REFERENCES test_user_session_1788437240066425200.permission_role(id) ON DELETE RESTRICT;


--
-- Name: permission_user_role fk_permission_user_role_user; Type: FK CONSTRAINT; Schema: test_user_session_1788437240066425200; Owner: -
--

ALTER TABLE ONLY test_user_session_1788437240066425200.permission_user_role
    ADD CONSTRAINT fk_permission_user_role_user FOREIGN KEY (user_id) REFERENCES test_user_session_1788437240066425200.user_account(id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--

\unrestrict cydo0mfmzKuo5SeHWsLnY3rjI1tVL7VYLwLfqnRxj3QdKBoAxdWTeuz2nnGXnWQ
