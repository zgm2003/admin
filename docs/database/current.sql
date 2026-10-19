--
-- PostgreSQL database dump
--

\restrict E2OGrYmQHI6urhZlrUaUxOJJfOcHu9WMMehOyLWKik31RxIUSjIj1FHXjAC87D3

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
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


--
-- Name: payment_callback_event_identity_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_callback_event_identity_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            IF ROW(NEW.id,NEW.config_id,NEW.config_version,NEW.dedupe_key,NEW.payload,NEW.created_at)
                IS DISTINCT FROM ROW(OLD.id,OLD.config_id,OLD.config_version,OLD.dedupe_key,OLD.payload,OLD.created_at)
                OR (OLD.order_id IS NOT NULL AND NEW.order_id IS DISTINCT FROM OLD.order_id)
                OR (OLD.process_status=2 AND NEW.process_status<>2) THEN
                RAISE EXCEPTION 'payment callback evidence is immutable';
            END IF;
            RETURN NEW;
        END$$;


--
-- Name: payment_config_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_config_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      IF TG_OP='DELETE' OR TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'payment configuration facts cannot be physically removed'; END IF;
      IF ROW(NEW.id,NEW.provider,NEW.environment,NEW.app_id,NEW.seller_id,NEW.created_at)
         IS DISTINCT FROM ROW(OLD.id,OLD.provider,OLD.environment,OLD.app_id,OLD.seller_id,OLD.created_at)
         OR NEW.current_version < OLD.current_version THEN RAISE EXCEPTION 'payment configuration identity is immutable'; END IF;
      RETURN NEW;
    END $$;


--
-- Name: payment_config_version_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_config_version_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN RAISE EXCEPTION 'payment credential versions are immutable'; END $$;


--
-- Name: payment_fact_no_delete_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_fact_no_delete_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            RAISE EXCEPTION 'payment financial facts cannot be physically removed';
        END$$;


--
-- Name: payment_order_identity_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_order_identity_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            IF ROW(NEW.id,NEW.order_no,NEW.user_id,NEW.platform_id,NEW.package_id,NEW.package_name,NEW.amount_cents,NEW.config_id,NEW.config_version,NEW.pay_method,NEW.request_id,NEW.request_fingerprint,NEW.return_url,NEW.expires_at,NEW.created_at)
                IS DISTINCT FROM ROW(OLD.id,OLD.order_no,OLD.user_id,OLD.platform_id,OLD.package_id,OLD.package_name,OLD.amount_cents,OLD.config_id,OLD.config_version,OLD.pay_method,OLD.request_id,OLD.request_fingerprint,OLD.return_url,OLD.expires_at,OLD.created_at) THEN
                RAISE EXCEPTION 'payment order purchase facts are immutable';
            END IF;
            IF (OLD.status=2 AND ROW(NEW.status,NEW.provider_trade_no,NEW.paid_at,NEW.credited_at,NEW.closed_at) IS DISTINCT FROM ROW(OLD.status,OLD.provider_trade_no,OLD.paid_at,OLD.credited_at,OLD.closed_at))
                OR (OLD.status=3 AND NEW.status NOT IN (2,3))
                OR (OLD.closed_at IS NOT NULL AND NEW.closed_at IS DISTINCT FROM OLD.closed_at)
                OR (OLD.provider_trade_no IS NOT NULL AND NEW.provider_trade_no IS DISTINCT FROM OLD.provider_trade_no) THEN
                RAISE EXCEPTION 'payment order final facts cannot be rewritten';
            END IF;
            RETURN NEW;
        END$$;


--
-- Name: payment_recharge_package_identity_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_recharge_package_identity_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            IF ROW(NEW.id,NEW.code,NEW.created_at) IS DISTINCT FROM ROW(OLD.id,OLD.code,OLD.created_at) THEN
                RAISE EXCEPTION 'payment recharge package identity is immutable';
            END IF;
            RETURN NEW;
        END$$;


--
-- Name: payment_redeem_batch_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_redeem_batch_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'payment redeem batches are immutable';
END $$;


--
-- Name: payment_redeem_code_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_redeem_code_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP IN ('DELETE','TRUNCATE') THEN RAISE EXCEPTION 'payment redeem codes cannot be physically removed'; END IF;
    IF TG_OP='UPDATE' AND (ROW(NEW.id,NEW.batch_id,NEW.code_hash,NEW.code_ciphertext,NEW.created_at)
       IS DISTINCT FROM ROW(OLD.id,OLD.batch_id,OLD.code_hash,OLD.code_ciphertext,OLD.created_at)
       OR (OLD.status<>1) OR (NEW.status NOT IN (2,3))) THEN
        RAISE EXCEPTION 'payment redeem code identity or terminal state is immutable';
    END IF;
    RETURN NEW;
END $$;


--
-- Name: payment_transaction_immutable_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_transaction_immutable_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            RAISE EXCEPTION 'payment transactions are immutable';
        END$$;


--
-- Name: payment_wallet_identity_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.payment_wallet_identity_guard() RETURNS trigger
    LANGUAGE plpgsql
    AS $$BEGIN
            IF ROW(NEW.user_id,NEW.created_at) IS DISTINCT FROM ROW(OLD.user_id,OLD.created_at) THEN
                RAISE EXCEPTION 'payment wallet identity is immutable';
            END IF;
            RETURN NEW;
        END$$;


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
    CONSTRAINT ck_auth_session_version CHECK ((version >= 1)),
    CONSTRAINT ck_user_session_device_id_nonempty CHECK ((btrim((device_id)::text) <> ''::text))
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
    status smallint NOT NULL,
    request_id character varying(128) DEFAULT ''::character varying NOT NULL,
    message_id character varying(128) DEFAULT ''::character varying NOT NULL,
    error_code character varying(128) DEFAULT ''::character varying NOT NULL,
    error_summary character varying(512) DEFAULT ''::character varying NOT NULL,
    latency_ms bigint DEFAULT 0 NOT NULL,
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_message_mail_log_scene CHECK (((scene)::text = ANY ((ARRAY['login'::character varying, 'forget'::character varying, 'bind_email'::character varying, 'change_password'::character varying])::text[]))),
    CONSTRAINT message_mail_log_status_check CHECK ((status = ANY (ARRAY[1, 2, 3])))
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
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
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
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    platform_id bigint NOT NULL,
    CONSTRAINT ck_message_mail_rate_limit_policy_platform CHECK ((platform_id > 0)),
    CONSTRAINT ck_message_mail_rate_limit_policy_shape CHECK ((((policy_key)::text = ANY ((ARRAY['business_email_minute'::character varying, 'business_email_10m'::character varying])::text[])) AND ((mode)::text = 'business'::text) AND ((dimension)::text = 'platform_email'::text))),
    CONSTRAINT ck_message_mail_rate_limit_policy_values CHECK ((((limit_count >= 1) AND (limit_count <= 100000)) AND ((window_seconds >= 1) AND (window_seconds <= 86400))))
);


--
-- Name: message_mail_recipient_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_mail_recipient_rule (
    id bigint NOT NULL,
    scope smallint NOT NULL,
    pattern character varying(254) NOT NULL,
    action smallint NOT NULL,
    name character varying(128) NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT message_mail_recipient_rule_action_check CHECK ((action = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_mail_recipient_rule_is_enabled_check CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_mail_recipient_rule_scope_check CHECK ((scope = ANY (ARRAY[0, 1])))
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
    tencent_template_id integer,
    example_variables jsonb NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    content text NOT NULL,
    variable_keys jsonb NOT NULL,
    CONSTRAINT ck_message_mail_template_variable_keys CHECK (((jsonb_typeof(variable_keys) = 'array'::text) AND (jsonb_array_length(variable_keys) >= 2))),
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
-- Name: message_notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    source_task_id bigint,
    source_type character varying(64) NOT NULL,
    source_key character varying(128) NOT NULL,
    audience_type character varying(16) NOT NULL,
    audience_max_user_id bigint NOT NULL,
    title character varying(128) NOT NULL,
    content_html text NOT NULL,
    summary character varying(256) NOT NULL,
    variant character varying(16) NOT NULL,
    priority character varying(16) NOT NULL,
    link_type character varying(16) NOT NULL,
    link character varying(2048) DEFAULT ''::character varying NOT NULL,
    published_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_message_notification_audience CHECK ((((audience_type)::text = ANY ((ARRAY['targeted'::character varying, 'platform'::character varying])::text[])) AND (audience_max_user_id >= 0))),
    CONSTRAINT ck_message_notification_content CHECK (((btrim((title)::text) <> ''::text) AND (char_length((title)::text) <= 128) AND (char_length(content_html) <= 16384) AND (char_length((summary)::text) <= 256))),
    CONSTRAINT ck_message_notification_link CHECK (((((link_type)::text = 'none'::text) AND ((link)::text = ''::text)) OR (((link_type)::text = ANY ((ARRAY['internal'::character varying, 'external'::character varying])::text[])) AND (btrim((link)::text) <> ''::text)))),
    CONSTRAINT ck_message_notification_priority CHECK (((priority)::text = ANY ((ARRAY['normal'::character varying, 'urgent'::character varying])::text[]))),
    CONSTRAINT ck_message_notification_source CHECK ((((source_type)::text ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'::text) AND (btrim((source_key)::text) <> ''::text))),
    CONSTRAINT ck_message_notification_variant CHECK (((variant)::text = ANY ((ARRAY['info'::character varying, 'success'::character varying, 'warning'::character varying, 'error'::character varying])::text[])))
);


--
-- Name: message_notification_broadcast_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification_broadcast_state (
    id bigint NOT NULL,
    notification_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    user_id bigint NOT NULL,
    read_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: message_notification_broadcast_state_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.message_notification_broadcast_state ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.message_notification_broadcast_state_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: message_notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.message_notification ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.message_notification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: message_notification_mailbox_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification_mailbox_state (
    platform_id bigint NOT NULL,
    user_id bigint NOT NULL,
    read_through_notification_id bigint DEFAULT 0 CONSTRAINT message_notification_mailbo_read_through_notification__not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_message_notification_mailbox_watermark CHECK ((read_through_notification_id >= 0))
);


--
-- Name: message_notification_recipient; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification_recipient (
    id bigint NOT NULL,
    notification_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    user_id bigint NOT NULL,
    read_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: message_notification_recipient_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.message_notification_recipient ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.message_notification_recipient_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: message_notification_task; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification_task (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    title character varying(128) NOT NULL,
    content_html text NOT NULL,
    summary character varying(256) NOT NULL,
    variant character varying(16) NOT NULL,
    priority character varying(16) NOT NULL,
    link_type character varying(16) NOT NULL,
    link character varying(2048) DEFAULT ''::character varying NOT NULL,
    audience_type character varying(16) NOT NULL,
    scheduled_at timestamp with time zone,
    audience_max_user_id bigint,
    submitted_at timestamp with time zone,
    status smallint DEFAULT 1 NOT NULL,
    next_user_id bigint DEFAULT 0 NOT NULL,
    next_batch_no integer DEFAULT 0 NOT NULL,
    generated_count bigint DEFAULT 0 NOT NULL,
    failure_message character varying(512),
    published_at timestamp with time zone,
    completed_at timestamp with time zone,
    canceled_at timestamp with time zone,
    failed_at timestamp with time zone,
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_message_notification_task_audience CHECK (((audience_type)::text = ANY ((ARRAY['user'::character varying, 'role'::character varying, 'platform'::character varying])::text[]))),
    CONSTRAINT ck_message_notification_task_content CHECK ((char_length(content_html) <= 16384)),
    CONSTRAINT ck_message_notification_task_counters CHECK (((next_user_id >= 0) AND (next_batch_no >= 0) AND (generated_count >= 0))),
    CONSTRAINT ck_message_notification_task_link CHECK (((((link_type)::text = 'none'::text) AND ((link)::text = ''::text)) OR (((link_type)::text = ANY ((ARRAY['internal'::character varying, 'external'::character varying])::text[])) AND (btrim((link)::text) <> ''::text)))),
    CONSTRAINT ck_message_notification_task_priority CHECK (((priority)::text = ANY ((ARRAY['normal'::character varying, 'urgent'::character varying])::text[]))),
    CONSTRAINT ck_message_notification_task_state CHECK ((((status = 1) AND (submitted_at IS NULL) AND (audience_max_user_id IS NULL) AND (published_at IS NULL) AND (completed_at IS NULL) AND (canceled_at IS NULL) AND (failed_at IS NULL) AND (failure_message IS NULL)) OR ((status <> 1) AND (submitted_at IS NOT NULL) AND (audience_max_user_id IS NOT NULL) AND (audience_max_user_id >= 0) AND ((status <> 2) OR (scheduled_at IS NOT NULL)) AND ((status <> 5) OR (completed_at IS NOT NULL)) AND ((status <> 7) OR (canceled_at IS NOT NULL)) AND ((status <> 6) OR ((failed_at IS NOT NULL) AND (failure_message IS NOT NULL) AND (btrim((failure_message)::text) <> ''::text)))))),
    CONSTRAINT ck_message_notification_task_status CHECK (((status >= 1) AND (status <= 7))),
    CONSTRAINT ck_message_notification_task_summary CHECK ((char_length((summary)::text) <= 256)),
    CONSTRAINT ck_message_notification_task_title CHECK (((btrim((title)::text) <> ''::text) AND (char_length((title)::text) <= 128))),
    CONSTRAINT ck_message_notification_task_variant CHECK (((variant)::text = ANY ((ARRAY['info'::character varying, 'success'::character varying, 'warning'::character varying, 'error'::character varying])::text[])))
);


--
-- Name: message_notification_task_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.message_notification_task ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.message_notification_task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: message_notification_task_target; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_notification_task_target (
    id bigint NOT NULL,
    task_id bigint NOT NULL,
    target_type character varying(16) NOT NULL,
    target_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_message_notification_task_target_id CHECK ((target_id > 0)),
    CONSTRAINT ck_message_notification_task_target_type CHECK (((target_type)::text = ANY ((ARRAY['user'::character varying, 'role'::character varying])::text[])))
);


--
-- Name: message_notification_task_target_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.message_notification_task_target ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.message_notification_task_target_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: message_sms_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_config (
    id bigint NOT NULL,
    secret_id_ciphertext text NOT NULL,
    secret_key_ciphertext text NOT NULL,
    secret_id_hint character varying(32) DEFAULT ''::character varying NOT NULL,
    secret_key_hint character varying(32) DEFAULT ''::character varying NOT NULL,
    sms_sdk_app_id character varying(64) NOT NULL,
    sign_name character varying(128) NOT NULL,
    region character varying(64) NOT NULL,
    endpoint character varying(255),
    ttl_minutes smallint NOT NULL,
    is_enabled smallint DEFAULT 0 NOT NULL,
    last_test_at timestamp with time zone,
    last_test_error character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_message_sms_config_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_message_sms_config_ttl_minutes CHECK (((ttl_minutes >= 1) AND (ttl_minutes <= 60)))
);


--
-- Name: message_sms_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_sms_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_sms_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_sms_config_id_seq OWNED BY public.message_sms_config.id;


--
-- Name: message_sms_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_log (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    challenge_id character varying(128),
    user_id bigint,
    scene character varying(32) NOT NULL,
    template_id bigint NOT NULL,
    status smallint NOT NULL,
    request_id character varying(128) DEFAULT ''::character varying NOT NULL,
    serial_no character varying(128) DEFAULT ''::character varying NOT NULL,
    fee integer DEFAULT 0 NOT NULL,
    error_code character varying(128) DEFAULT ''::character varying NOT NULL,
    error_summary character varying(512) DEFAULT ''::character varying NOT NULL,
    latency_ms bigint DEFAULT 0 NOT NULL,
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    to_phone character varying(32) NOT NULL,
    CONSTRAINT ck_message_sms_log_fee CHECK ((fee >= 0)),
    CONSTRAINT ck_message_sms_log_scene CHECK (((scene)::text = ANY ((ARRAY['login'::character varying, 'forget'::character varying, 'bind_phone'::character varying, 'change_password'::character varying])::text[]))),
    CONSTRAINT ck_message_sms_log_status CHECK ((status = ANY (ARRAY[1, 2, 3]))),
    CONSTRAINT message_sms_log_to_phone_check CHECK (((to_phone)::text ~ '^[+]861[3-9][0-9]{9}$'::text))
);


--
-- Name: message_sms_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_sms_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_sms_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_sms_log_id_seq OWNED BY public.message_sms_log.id;


--
-- Name: message_sms_log_verification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_log_verification (
    id bigint NOT NULL,
    platform_id bigint NOT NULL,
    sms_log_id bigint NOT NULL,
    key_version character varying(16) NOT NULL,
    code_ciphertext text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: message_sms_log_verification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_sms_log_verification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_sms_log_verification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_sms_log_verification_id_seq OWNED BY public.message_sms_log_verification.id;


--
-- Name: message_sms_rate_limit_policy; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_rate_limit_policy (
    platform_id bigint NOT NULL,
    policy_key character varying(64) NOT NULL,
    mode character varying(16) NOT NULL,
    dimension character varying(64) NOT NULL,
    limit_count integer NOT NULL,
    window_seconds integer NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_message_sms_rate_limit_policy_platform CHECK ((platform_id > 0)),
    CONSTRAINT ck_message_sms_rate_limit_policy_shape CHECK ((((policy_key)::text = ANY ((ARRAY['business_phone_minute'::character varying, 'business_phone_10m'::character varying])::text[])) AND ((mode)::text = 'business'::text) AND ((dimension)::text = 'platform_phone'::text))),
    CONSTRAINT ck_message_sms_rate_limit_policy_values CHECK (((limit_count >= 1) AND (limit_count <= 100000) AND (window_seconds >= 1) AND (window_seconds <= 86400)))
);


--
-- Name: message_sms_recipient_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_recipient_rule (
    id bigint NOT NULL,
    scope smallint NOT NULL,
    action smallint NOT NULL,
    name character varying(128) NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    pattern character varying(32) NOT NULL,
    CONSTRAINT ck_message_sms_recipient_rule_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_sms_recipient_rule_action_check CHECK ((action = ANY (ARRAY[0, 1]))),
    CONSTRAINT message_sms_recipient_rule_pattern_check CHECK ((((scope = 0) AND ((pattern)::text ~ '^[+]861[3-9][0-9]{9}$'::text)) OR ((scope = 1) AND ((pattern)::text ~ '^[+]86[0-9]{3,10}$'::text)))),
    CONSTRAINT message_sms_recipient_rule_scope_check CHECK ((scope = ANY (ARRAY[0, 1])))
);


--
-- Name: message_sms_recipient_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_sms_recipient_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_sms_recipient_rule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_sms_recipient_rule_id_seq OWNED BY public.message_sms_recipient_rule.id;


--
-- Name: message_sms_template; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_sms_template (
    id bigint NOT NULL,
    scene character varying(32) NOT NULL,
    name character varying(128) NOT NULL,
    tencent_template_id character varying(64) DEFAULT ''::character varying NOT NULL,
    variable_keys jsonb CONSTRAINT message_sms_template_parameter_keys_not_null NOT NULL,
    example_variables jsonb NOT NULL,
    is_enabled smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    content text NOT NULL,
    CONSTRAINT ck_message_sms_template_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_message_sms_template_scene CHECK (((scene)::text = ANY ((ARRAY['login'::character varying, 'forget'::character varying, 'bind_phone'::character varying, 'change_password'::character varying])::text[]))),
    CONSTRAINT ck_message_sms_template_variable_keys CHECK (((jsonb_typeof(variable_keys) = 'array'::text) AND (jsonb_array_length(variable_keys) >= 2)))
);


--
-- Name: message_sms_template_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_sms_template_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_sms_template_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_sms_template_id_seq OWNED BY public.message_sms_template.id;


--
-- Name: payment_callback_event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_callback_event (
    id bigint NOT NULL,
    config_id bigint NOT NULL,
    config_version bigint NOT NULL,
    dedupe_key bytea NOT NULL,
    order_id bigint,
    payload jsonb NOT NULL,
    process_status smallint NOT NULL,
    error_code smallint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_callback_event_dedupe CHECK ((octet_length(dedupe_key) = 32)),
    CONSTRAINT ck_payment_callback_event_error CHECK (((error_code IS NULL) OR ((error_code >= 1) AND (error_code <= 9)))),
    CONSTRAINT ck_payment_callback_event_payload CHECK (((jsonb_typeof(payload) = 'object'::text) AND (payload ?& ARRAY['notifyId'::text, 'orderNo'::text, 'tradeNo'::text, 'tradeStatus'::text, 'appId'::text, 'sellerId'::text, 'totalAmount'::text, 'paidAt'::text]) AND ((payload - ARRAY['notifyId'::text, 'orderNo'::text, 'tradeNo'::text, 'tradeStatus'::text, 'appId'::text, 'sellerId'::text, 'totalAmount'::text, 'paidAt'::text]) = '{}'::jsonb) AND ((jsonb_typeof((payload -> 'notifyId'::text)) = 'null'::text) OR ((jsonb_typeof((payload -> 'notifyId'::text)) = 'string'::text) AND ((octet_length((payload ->> 'notifyId'::text)) >= 1) AND (octet_length((payload ->> 'notifyId'::text)) <= 128)) AND ((payload ->> 'notifyId'::text) ~ '^[!-~]+$'::text) AND ((payload ->> 'notifyId'::text) !~ '[&=]'::text))) AND (jsonb_typeof((payload -> 'orderNo'::text)) = 'string'::text) AND ((payload ->> 'orderNo'::text) ~ '^[A-Za-z0-9_-]{1,32}$'::text) AND ((jsonb_typeof((payload -> 'tradeNo'::text)) = 'null'::text) OR ((jsonb_typeof((payload -> 'tradeNo'::text)) = 'string'::text) AND ((octet_length((payload ->> 'tradeNo'::text)) >= 1) AND (octet_length((payload ->> 'tradeNo'::text)) <= 64)) AND ((payload ->> 'tradeNo'::text) ~ '^[!-~]+$'::text) AND ((payload ->> 'tradeNo'::text) !~ '[&=]'::text))) AND (jsonb_typeof((payload -> 'tradeStatus'::text)) = 'string'::text) AND ((octet_length((payload ->> 'tradeStatus'::text)) >= 1) AND (octet_length((payload ->> 'tradeStatus'::text)) <= 64)) AND ((payload ->> 'tradeStatus'::text) ~ '^[!-~]+$'::text) AND ((payload ->> 'tradeStatus'::text) !~ '[&=]'::text) AND (jsonb_typeof((payload -> 'appId'::text)) = 'string'::text) AND ((octet_length((payload ->> 'appId'::text)) >= 1) AND (octet_length((payload ->> 'appId'::text)) <= 64)) AND ((payload ->> 'appId'::text) ~ '^[!-~]+$'::text) AND ((payload ->> 'appId'::text) !~ '[&=]'::text) AND (jsonb_typeof((payload -> 'sellerId'::text)) = 'string'::text) AND ((octet_length((payload ->> 'sellerId'::text)) >= 1) AND (octet_length((payload ->> 'sellerId'::text)) <= 64)) AND ((payload ->> 'sellerId'::text) ~ '^[!-~]+$'::text) AND ((payload ->> 'sellerId'::text) !~ '[&=]'::text) AND (jsonb_typeof((payload -> 'totalAmount'::text)) = 'string'::text) AND ((payload ->> 'totalAmount'::text) ~ '^[0-9]+\.[0-9]{2}$'::text) AND ((((payload ->> 'totalAmount'::text))::numeric >= 0.01) AND (((payload ->> 'totalAmount'::text))::numeric <= 9223372036854.00)) AND ((jsonb_typeof((payload -> 'paidAt'::text)) = 'null'::text) OR ((jsonb_typeof((payload -> 'paidAt'::text)) = 'string'::text) AND ((octet_length((payload ->> 'paidAt'::text)) >= 1) AND (octet_length((payload ->> 'paidAt'::text)) <= 64)) AND ((payload ->> 'paidAt'::text) ~ '^[0-9T:.+Z-]+$'::text))))),
    CONSTRAINT ck_payment_callback_event_status CHECK ((process_status = ANY (ARRAY[1, 2, 3, 4])))
);


--
-- Name: payment_callback_event_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_callback_event ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_callback_event_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_config (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    provider smallint NOT NULL,
    environment smallint NOT NULL,
    app_id character varying(64) NOT NULL,
    seller_id character varying(64) NOT NULL,
    current_version bigint NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_payment_config_app_id CHECK (((app_id)::text ~ '^[0-9]{1,64}$'::text)),
    CONSTRAINT ck_payment_config_current_version CHECK ((current_version > 0)),
    CONSTRAINT ck_payment_config_environment CHECK ((environment = ANY (ARRAY[1, 2]))),
    CONSTRAINT ck_payment_config_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_payment_config_name CHECK ((btrim((name)::text) <> ''::text)),
    CONSTRAINT ck_payment_config_provider CHECK ((provider = ANY (ARRAY[1, 2]))),
    CONSTRAINT ck_payment_config_seller_id CHECK (((seller_id)::text ~ '^[0-9]{1,64}$'::text))
);


--
-- Name: payment_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_config ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_config_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_config_version (
    config_id bigint NOT NULL,
    version bigint NOT NULL,
    private_key_ciphertext text NOT NULL,
    app_cert_pem text NOT NULL,
    alipay_cert_pem text NOT NULL,
    root_cert_pem text NOT NULL,
    notify_url character varying(2048) NOT NULL,
    pay_methods smallint[] NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_config_version_alipay_cert CHECK (((octet_length(alipay_cert_pem) >= 1) AND (octet_length(alipay_cert_pem) <= 65536))),
    CONSTRAINT ck_payment_config_version_app_cert CHECK (((octet_length(app_cert_pem) >= 1) AND (octet_length(app_cert_pem) <= 65536))),
    CONSTRAINT ck_payment_config_version_key CHECK (((octet_length(private_key_ciphertext) >= 1) AND (octet_length(private_key_ciphertext) <= 32768))),
    CONSTRAINT ck_payment_config_version_notify_url CHECK (((notify_url)::text ~ (((('^https://[^/?#@]+/api/admin/v1/payment/callback/alipay/'::text || config_id) || '/'::text) || version) || '$'::text))),
    CONSTRAINT ck_payment_config_version_pay_methods CHECK ((((pay_methods = ARRAY[(1)::smallint]) OR (pay_methods = ARRAY[(2)::smallint])) OR (pay_methods = ARRAY[(1)::smallint, (2)::smallint]))),
    CONSTRAINT ck_payment_config_version_root_cert CHECK (((octet_length(root_cert_pem) >= 1) AND (octet_length(root_cert_pem) <= 65536))),
    CONSTRAINT ck_payment_config_version_version CHECK ((version > 0))
);


--
-- Name: payment_order; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_order (
    id bigint NOT NULL,
    order_no character varying(32) NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    package_id bigint NOT NULL,
    package_name character varying(128) NOT NULL,
    amount_cents bigint NOT NULL,
    config_id bigint NOT NULL,
    config_version bigint NOT NULL,
    pay_method smallint NOT NULL,
    request_id uuid NOT NULL,
    request_fingerprint bytea NOT NULL,
    return_url character varying(2048) NOT NULL,
    status smallint NOT NULL,
    provider_trade_no character varying(64),
    expires_at timestamp with time zone NOT NULL,
    paid_at timestamp with time zone,
    credited_at timestamp with time zone,
    closed_at timestamp with time zone,
    next_check_at timestamp with time zone,
    check_token uuid,
    check_lease_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_order_amount CHECK (((amount_cents >= 1) AND (amount_cents <= '9223372036854'::bigint))),
    CONSTRAINT ck_payment_order_order_no CHECK (((order_no)::text ~ '^[A-Za-z0-9_-]{1,32}$'::text)),
    CONSTRAINT ck_payment_order_package_name CHECK ((btrim((package_name)::text) <> ''::text)),
    CONSTRAINT ck_payment_order_pay_method CHECK ((pay_method = ANY (ARRAY[1, 2]))),
    CONSTRAINT ck_payment_order_request_fingerprint CHECK ((octet_length(request_fingerprint) = 32)),
    CONSTRAINT ck_payment_order_return_url CHECK ((((return_url)::text ~~ 'http://%'::text) OR ((return_url)::text ~~ 'https://%'::text))),
    CONSTRAINT ck_payment_order_state_times CHECK ((((status = 1) AND (paid_at IS NULL) AND (credited_at IS NULL) AND (closed_at IS NULL)) OR ((status = 2) AND (credited_at IS NOT NULL)) OR ((status = 3) AND (closed_at IS NOT NULL) AND (paid_at IS NULL) AND (credited_at IS NULL)))),
    CONSTRAINT ck_payment_order_status CHECK ((status = ANY (ARRAY[1, 2, 3])))
);


--
-- Name: payment_order_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_order ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_order_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_recharge_package; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_recharge_package (
    id bigint NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(128) NOT NULL,
    amount_cents bigint NOT NULL,
    badge character varying(32),
    sort_order integer DEFAULT 0 NOT NULL,
    is_enabled smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_payment_recharge_package_amount CHECK (((amount_cents >= 1) AND (amount_cents <= '9223372036854'::bigint))),
    CONSTRAINT ck_payment_recharge_package_code CHECK (((code)::text ~ '^[a-z][a-z0-9_]{0,63}$'::text)),
    CONSTRAINT ck_payment_recharge_package_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_payment_recharge_package_name CHECK ((btrim((name)::text) <> ''::text)),
    CONSTRAINT ck_payment_recharge_package_sort CHECK ((sort_order >= 0))
);


--
-- Name: payment_recharge_package_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_recharge_package ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_recharge_package_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_redeem_code; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_redeem_code (
    id bigint NOT NULL,
    batch_id bigint CONSTRAINT payment_redeem_code_batch_id_not_null1 NOT NULL,
    code_hash bytea NOT NULL,
    code_ciphertext text NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    used_by bigint,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_redeem_code_ciphertext CHECK (((octet_length(code_ciphertext) >= 1) AND (octet_length(code_ciphertext) <= 131072))),
    CONSTRAINT ck_payment_redeem_code_hash CHECK ((octet_length(code_hash) = 32)),
    CONSTRAINT ck_payment_redeem_code_status CHECK ((status = ANY (ARRAY[1, 2, 3]))),
    CONSTRAINT ck_payment_redeem_code_used_state CHECK ((((status = ANY (ARRAY[1, 3])) AND (used_by IS NULL) AND (used_at IS NULL)) OR ((status = 2) AND (used_by IS NOT NULL) AND (used_at IS NOT NULL))))
);


--
-- Name: payment_redeem_code_batch; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_redeem_code_batch (
    id bigint NOT NULL,
    batch_no character varying(32) NOT NULL,
    request_id uuid NOT NULL,
    request_fingerprint bytea NOT NULL,
    amount_cents bigint NOT NULL,
    quantity integer NOT NULL,
    expires_at timestamp with time zone,
    remark character varying(255),
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_redeem_batch_amount CHECK (((amount_cents >= 1) AND (amount_cents <= '9223372036854'::bigint))),
    CONSTRAINT ck_payment_redeem_batch_fingerprint CHECK ((octet_length(request_fingerprint) = 32)),
    CONSTRAINT ck_payment_redeem_batch_no CHECK (((batch_no)::text ~ '^[A-Za-z0-9_-]{1,32}$'::text)),
    CONSTRAINT ck_payment_redeem_batch_quantity CHECK (((quantity >= 1) AND (quantity <= 1000))),
    CONSTRAINT ck_payment_redeem_batch_remark CHECK (((remark IS NULL) OR (btrim((remark)::text) <> ''::text)))
);


--
-- Name: payment_redeem_code_batch_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_redeem_code_batch ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_redeem_code_batch_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_redeem_code_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_redeem_code ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_redeem_code_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_transaction; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_transaction (
    id bigint NOT NULL,
    transaction_no character varying(32) NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    amount_units bigint NOT NULL,
    balance_after_units bigint NOT NULL,
    order_id bigint,
    redeem_code_id bigint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_transaction_amount CHECK ((amount_units > 0)),
    CONSTRAINT ck_payment_transaction_balance CHECK (((balance_after_units)::numeric >= ((- '9223372036854775808'::numeric) + (amount_units)::numeric))),
    CONSTRAINT ck_payment_transaction_source_xor CHECK (((order_id IS NOT NULL) <> (redeem_code_id IS NOT NULL)))
);


--
-- Name: payment_transaction_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payment_transaction ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payment_transaction_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payment_wallet; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_wallet (
    user_id bigint NOT NULL,
    balance_units bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


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
-- Name: realtime_event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.realtime_event (
    sequence bigint NOT NULL,
    event_id uuid NOT NULL,
    dedup_key character varying(256) NOT NULL,
    platform_id bigint NOT NULL,
    event_type character varying(64) NOT NULL,
    target_type character varying(16) NOT NULL,
    target_user_id bigint,
    audience_max_user_id bigint,
    payload jsonb NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_realtime_event_payload CHECK ((jsonb_typeof(payload) = 'object'::text)),
    CONSTRAINT ck_realtime_event_target CHECK (((((target_type)::text = 'user'::text) AND (target_user_id IS NOT NULL) AND (target_user_id > 0) AND (audience_max_user_id IS NULL)) OR (((target_type)::text = 'platform'::text) AND (target_user_id IS NULL) AND (audience_max_user_id IS NOT NULL) AND (audience_max_user_id >= 0)))),
    CONSTRAINT ck_realtime_event_type CHECK ((btrim((event_type)::text) <> ''::text))
);


--
-- Name: realtime_event_outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.realtime_event_outbox (
    id bigint NOT NULL,
    event_sequence bigint NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    available_at timestamp with time zone NOT NULL,
    locked_until timestamp with time zone,
    lock_token character varying(64),
    published_at timestamp with time zone,
    last_error character varying(512),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_realtime_event_outbox_attempts CHECK ((attempts >= 0)),
    CONSTRAINT ck_realtime_event_outbox_lock CHECK (((locked_until IS NULL) = (lock_token IS NULL))),
    CONSTRAINT ck_realtime_event_outbox_published CHECK (((published_at IS NULL) OR ((locked_until IS NULL) AND (lock_token IS NULL))))
);


--
-- Name: realtime_event_outbox_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.realtime_event_outbox ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.realtime_event_outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: realtime_event_sequence_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.realtime_event ALTER COLUMN sequence ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.realtime_event_sequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: realtime_retention_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.realtime_retention_state (
    platform_id bigint NOT NULL,
    deleted_through_sequence bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_realtime_retention_watermark CHECK ((deleted_through_sequence >= 0))
);


--
-- Name: storage_cos_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_cos_config (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    app_id character varying(32) NOT NULL,
    secret_id_ciphertext text NOT NULL,
    secret_key_ciphertext text NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    current_version bigint NOT NULL,
    CONSTRAINT ck_storage_cos_config_current_version CHECK ((current_version >= 1)),
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
-- Name: storage_cos_config_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_cos_config_version (
    cos_config_id bigint NOT NULL,
    version bigint NOT NULL,
    bucket character varying(128) NOT NULL,
    region character varying(64) NOT NULL,
    endpoint character varying(255),
    bucket_domain character varying(255),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_storage_cos_config_version_value CHECK ((version >= 1))
);


--
-- Name: storage_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storage_file (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    kind smallint NOT NULL,
    cos_config_id bigint NOT NULL,
    config_version bigint NOT NULL,
    upload_rule_id bigint NOT NULL,
    rule_code character varying(128) NOT NULL,
    object_key character varying(1024) NOT NULL,
    upload_object_key character varying(1024),
    original_name character varying(255) NOT NULL,
    mime_type character varying(255) NOT NULL,
    size_bytes bigint NOT NULL,
    etag character varying(255) DEFAULT ''::character varying NOT NULL,
    status smallint NOT NULL,
    credential_expires_at timestamp with time zone,
    completed_at timestamp with time zone,
    request_key uuid,
    source_file_id bigint,
    operation_lease_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT storage_file_check CHECK (((upload_object_key IS NULL) OR ((btrim((upload_object_key)::text) <> ''::text) AND ((upload_object_key)::text <> (object_key)::text)))),
    CONSTRAINT storage_file_config_version_check CHECK ((config_version > 0)),
    CONSTRAINT storage_file_cos_config_id_check CHECK ((cos_config_id > 0)),
    CONSTRAINT storage_file_kind_check CHECK ((kind = ANY (ARRAY[1, 2]))),
    CONSTRAINT storage_file_mime_type_check CHECK ((btrim((mime_type)::text) <> ''::text)),
    CONSTRAINT storage_file_object_key_check CHECK ((btrim((object_key)::text) <> ''::text)),
    CONSTRAINT storage_file_original_name_check CHECK ((btrim((original_name)::text) <> ''::text)),
    CONSTRAINT storage_file_platform_id_check CHECK ((platform_id > 0)),
    CONSTRAINT storage_file_rule_code_check CHECK ((btrim((rule_code)::text) <> ''::text)),
    CONSTRAINT storage_file_size_bytes_check CHECK ((size_bytes > 0)),
    CONSTRAINT storage_file_source_file_id_check CHECK (((source_file_id IS NULL) OR (source_file_id > 0))),
    CONSTRAINT storage_file_status_check CHECK ((status = ANY (ARRAY[1, 2, 3]))),
    CONSTRAINT storage_file_upload_rule_id_check CHECK ((upload_rule_id > 0)),
    CONSTRAINT storage_file_user_id_check CHECK ((user_id > 0))
);


--
-- Name: storage_file_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.storage_file ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.storage_file_id_seq
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
    code character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT ck_storage_upload_rule_code_value CHECK ((((octet_length((code)::text) >= 1) AND (octet_length((code)::text) <= 64)) AND ((code)::text = lower(btrim((code)::text))) AND ((code)::text ~ '^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$'::text) AND (POSITION(('..'::text) IN (code)) = 0) AND ((('/'::text || (code)::text) || '/'::text) !~~ '%/.admin-storage/%'::text)))
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
-- Name: system_config_cache_outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_config_cache_outbox (
    id bigint NOT NULL,
    namespace character varying(128) NOT NULL,
    scope_key character varying(128) NOT NULL,
    generation bigint NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    available_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    locked_until timestamp with time zone,
    lock_token character varying(64),
    last_error character varying(512) DEFAULT ''::character varying NOT NULL,
    published_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_system_config_cache_outbox_attempts CHECK ((attempts >= 0)),
    CONSTRAINT ck_system_config_cache_outbox_generation_value CHECK ((generation >= 1)),
    CONSTRAINT ck_system_config_cache_outbox_lock CHECK ((((locked_until IS NULL) AND (lock_token IS NULL)) OR ((locked_until IS NOT NULL) AND (lock_token IS NOT NULL)))),
    CONSTRAINT ck_system_config_cache_outbox_namespace CHECK (((namespace)::text ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'::text)),
    CONSTRAINT ck_system_config_cache_outbox_published_lock CHECK (((published_at IS NULL) OR ((locked_until IS NULL) AND (lock_token IS NULL)))),
    CONSTRAINT ck_system_config_cache_outbox_scope_key CHECK (((scope_key)::text ~ '^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$'::text))
);


--
-- Name: system_config_cache_outbox_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.system_config_cache_outbox ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.system_config_cache_outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: system_config_cache_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_config_cache_state (
    namespace character varying(128) CONSTRAINT system_config_cache_generation_namespace_not_null NOT NULL,
    scope_key character varying(128) CONSTRAINT system_config_cache_generation_scope_key_not_null NOT NULL,
    generation bigint DEFAULT 1 CONSTRAINT system_config_cache_generation_generation_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT system_config_cache_generation_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP CONSTRAINT system_config_cache_generation_updated_at_not_null NOT NULL,
    CONSTRAINT ck_system_config_cache_state_namespace CHECK (((namespace)::text ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'::text)),
    CONSTRAINT ck_system_config_cache_state_scope_key CHECK (((scope_key)::text ~ '^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$'::text)),
    CONSTRAINT ck_system_config_cache_state_value CHECK ((generation >= 1))
);


--
-- Name: system_scheduler_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_scheduler_job (
    id bigint NOT NULL,
    schedule_id bigint,
    task_type character varying(128) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    trigger_source character varying(16) NOT NULL,
    source_key character varying(255),
    scheduled_at timestamp with time zone NOT NULL,
    available_at timestamp with time zone NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    publish_token character varying(64),
    publish_lease_until timestamp with time zone,
    run_token character varying(64),
    run_lease_until timestamp with time zone,
    retry_of_job_id bigint,
    triggered_by bigint,
    error_class character varying(64) DEFAULT ''::character varying NOT NULL,
    last_error character varying(1024) DEFAULT ''::character varying NOT NULL,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_system_scheduler_job_completed_at CHECK (((status = ANY (ARRAY[4, 5, 6])) = (completed_at IS NOT NULL))),
    CONSTRAINT ck_system_scheduler_job_counters CHECK (((attempt_count >= 0) AND (max_attempts > 0))),
    CONSTRAINT ck_system_scheduler_job_payload CHECK ((jsonb_typeof(payload) = 'object'::text)),
    CONSTRAINT ck_system_scheduler_job_publish_lease CHECK ((((status = 1) AND (((publish_lease_until IS NULL) AND (publish_token IS NULL)) OR ((publish_lease_until IS NOT NULL) AND (publish_token IS NOT NULL)))) OR ((status = 2) AND (publish_lease_until IS NOT NULL) AND (publish_token IS NOT NULL)) OR ((status <> ALL (ARRAY[1, 2])) AND (publish_lease_until IS NULL) AND (publish_token IS NULL)))),
    CONSTRAINT ck_system_scheduler_job_run_lease CHECK ((((run_lease_until IS NULL) AND (run_token IS NULL)) OR ((run_lease_until IS NOT NULL) AND (run_token IS NOT NULL)))),
    CONSTRAINT ck_system_scheduler_job_status CHECK (((status >= 1) AND (status <= 6))),
    CONSTRAINT ck_system_scheduler_job_task_type CHECK ((btrim((task_type)::text) <> ''::text)),
    CONSTRAINT ck_system_scheduler_job_trigger_source CHECK (((trigger_source)::text = ANY ((ARRAY['cron'::character varying, 'manual'::character varying, 'retry'::character varying, 'business'::character varying])::text[])))
);


--
-- Name: system_scheduler_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.system_scheduler_job ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.system_scheduler_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: system_scheduler_run; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_scheduler_run (
    id bigint NOT NULL,
    job_id bigint NOT NULL,
    attempt_no integer NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    worker_id character varying(128) NOT NULL,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone,
    duration_ms bigint,
    error_class character varying(64) DEFAULT ''::character varying NOT NULL,
    error_message character varying(1024) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_system_scheduler_run_attempt CHECK ((attempt_no > 0)),
    CONSTRAINT ck_system_scheduler_run_duration CHECK (((duration_ms IS NULL) OR (duration_ms >= 0))),
    CONSTRAINT ck_system_scheduler_run_finished CHECK ((((status = 1) AND (finished_at IS NULL)) OR ((status <> 1) AND (finished_at IS NOT NULL)))),
    CONSTRAINT ck_system_scheduler_run_status CHECK (((status >= 1) AND (status <= 3)))
);


--
-- Name: system_scheduler_run_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.system_scheduler_run ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.system_scheduler_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: system_scheduler_schedule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_scheduler_schedule (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    description character varying(512) DEFAULT ''::character varying NOT NULL,
    task_type character varying(128) NOT NULL,
    cron_expression character varying(128) NOT NULL,
    timezone character varying(64) DEFAULT 'Asia/Shanghai'::character varying NOT NULL,
    params jsonb DEFAULT '{}'::jsonb NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    next_run_at timestamp with time zone,
    builtin_key character varying(128),
    created_by bigint,
    updated_by bigint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_system_scheduler_schedule_builtin_key CHECK (((builtin_key IS NULL) OR (btrim((builtin_key)::text) <> ''::text))),
    CONSTRAINT ck_system_scheduler_schedule_cron CHECK ((btrim((cron_expression)::text) <> ''::text)),
    CONSTRAINT ck_system_scheduler_schedule_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_system_scheduler_schedule_name CHECK ((btrim((name)::text) <> ''::text)),
    CONSTRAINT ck_system_scheduler_schedule_params CHECK ((jsonb_typeof(params) = 'object'::text)),
    CONSTRAINT ck_system_scheduler_schedule_task_type CHECK ((btrim((task_type)::text) <> ''::text)),
    CONSTRAINT ck_system_scheduler_schedule_timezone CHECK ((btrim((timezone)::text) <> ''::text))
);


--
-- Name: system_scheduler_schedule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.system_scheduler_schedule ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.system_scheduler_schedule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: system_setting; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_setting (
    id bigint NOT NULL,
    setting_key character varying(128) NOT NULL,
    value text NOT NULL,
    value_type smallint NOT NULL,
    description character varying(512) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    is_builtin smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_system_setting_is_builtin CHECK ((is_builtin = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_system_setting_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_system_setting_key CHECK (((setting_key)::text ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'::text)),
    CONSTRAINT ck_system_setting_value_type CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4, 5, 6])))
);


--
-- Name: COLUMN system_setting.value_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.system_setting.value_type IS '1=字符串, 2=数字, 3=布尔, 4=JSON, 5=媒体-文件对象键, 6=媒体-图片/视频对象键';


--
-- Name: system_setting_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.system_setting_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: system_setting_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.system_setting_id_seq OWNED BY public.system_setting.id;


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
    CONSTRAINT ck_user_account_is_enabled CHECK ((is_enabled = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_user_account_phone_e164 CHECK (((phone IS NULL) OR ((phone)::text ~ '^\+861[3-9][0-9]{9}$'::text)))
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
-- Name: user_email_change_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_email_change_log (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    action smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    old_email character varying(254),
    new_email character varying(254) NOT NULL,
    CONSTRAINT ck_user_email_change_log_action CHECK ((action = ANY (ARRAY[1, 2]))),
    CONSTRAINT ck_user_email_change_log_email_shape CHECK (((btrim((new_email)::text) <> ''::text) AND (length((new_email)::text) <= 254) AND ((old_email IS NULL) OR ((btrim((old_email)::text) <> ''::text) AND (length((old_email)::text) <= 254))))),
    CONSTRAINT ck_user_email_change_log_old_email CHECK ((((action = 1) AND (old_email IS NOT NULL)) OR ((action = 2) AND (old_email IS NULL))))
);


--
-- Name: user_email_change_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_email_change_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_email_change_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_email_change_log_id_seq OWNED BY public.user_email_change_log.id;


--
-- Name: user_login_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_login_log (
    id bigint NOT NULL,
    user_id bigint,
    platform_id bigint NOT NULL,
    account character varying(254) CONSTRAINT user_login_log_login_account_not_null NOT NULL,
    event_type smallint NOT NULL,
    login_type smallint,
    is_success smallint NOT NULL,
    reason_code character varying(64) NOT NULL,
    client_ip character varying(64) NOT NULL,
    user_agent character varying(512) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_user_login_log_account CHECK ((btrim((account)::text) <> ''::text)),
    CONSTRAINT ck_user_login_log_event_type CHECK ((event_type = ANY (ARRAY[1, 2, 3]))),
    CONSTRAINT ck_user_login_log_is_success CHECK ((is_success = ANY (ARRAY[0, 1]))),
    CONSTRAINT ck_user_login_log_login_type CHECK ((((event_type = 1) AND (login_type = ANY (ARRAY[2, 3]))) OR ((event_type = 2) AND (login_type = ANY (ARRAY[1, 2, 3]))) OR ((event_type = 3) AND (login_type IS NULL))))
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
-- Name: user_phone_change_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_phone_change_log (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    platform_id bigint NOT NULL,
    action smallint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    old_phone character varying(32),
    new_phone character varying(32) NOT NULL,
    CONSTRAINT ck_user_phone_change_log_action CHECK ((action = ANY (ARRAY[1, 2]))),
    CONSTRAINT ck_user_phone_change_log_old_phone CHECK ((((action = 1) AND (old_phone IS NOT NULL)) OR ((action = 2) AND (old_phone IS NULL)))),
    CONSTRAINT ck_user_phone_change_log_phone_shape CHECK ((((new_phone)::text ~ '^[+]861[3-9][0-9]{9}$'::text) AND ((old_phone IS NULL) OR ((old_phone)::text ~ '^[+]861[3-9][0-9]{9}$'::text))))
);


--
-- Name: user_phone_change_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_phone_change_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_phone_change_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_phone_change_log_id_seq OWNED BY public.user_phone_change_log.id;


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
-- Name: message_sms_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_config ALTER COLUMN id SET DEFAULT nextval('public.message_sms_config_id_seq'::regclass);


--
-- Name: message_sms_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log ALTER COLUMN id SET DEFAULT nextval('public.message_sms_log_id_seq'::regclass);


--
-- Name: message_sms_log_verification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log_verification ALTER COLUMN id SET DEFAULT nextval('public.message_sms_log_verification_id_seq'::regclass);


--
-- Name: message_sms_recipient_rule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_recipient_rule ALTER COLUMN id SET DEFAULT nextval('public.message_sms_recipient_rule_id_seq'::regclass);


--
-- Name: message_sms_template id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_template ALTER COLUMN id SET DEFAULT nextval('public.message_sms_template_id_seq'::regclass);


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
-- Name: system_setting id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_setting ALTER COLUMN id SET DEFAULT nextval('public.system_setting_id_seq'::regclass);


--
-- Name: user_account id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_account ALTER COLUMN id SET DEFAULT nextval('public.user_account_id_seq'::regclass);


--
-- Name: user_email_change_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_email_change_log ALTER COLUMN id SET DEFAULT nextval('public.user_email_change_log_id_seq'::regclass);


--
-- Name: user_phone_change_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_phone_change_log ALTER COLUMN id SET DEFAULT nextval('public.user_phone_change_log_id_seq'::regclass);


--
-- Name: user_session id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_session ALTER COLUMN id SET DEFAULT nextval('public.auth_session_id_seq'::regclass);


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
-- Name: message_notification_broadcast_state message_notification_broadcast_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_broadcast_state
    ADD CONSTRAINT message_notification_broadcast_state_pkey PRIMARY KEY (id);


--
-- Name: message_notification_mailbox_state message_notification_mailbox_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_mailbox_state
    ADD CONSTRAINT message_notification_mailbox_state_pkey PRIMARY KEY (platform_id, user_id);


--
-- Name: message_notification message_notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification
    ADD CONSTRAINT message_notification_pkey PRIMARY KEY (id);


--
-- Name: message_notification_recipient message_notification_recipient_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_recipient
    ADD CONSTRAINT message_notification_recipient_pkey PRIMARY KEY (id);


--
-- Name: message_notification_task message_notification_task_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_task
    ADD CONSTRAINT message_notification_task_pkey PRIMARY KEY (id);


--
-- Name: message_notification_task_target message_notification_task_target_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_task_target
    ADD CONSTRAINT message_notification_task_target_pkey PRIMARY KEY (id);


--
-- Name: message_sms_config message_sms_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_config
    ADD CONSTRAINT message_sms_config_pkey PRIMARY KEY (id);


--
-- Name: message_sms_log message_sms_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log
    ADD CONSTRAINT message_sms_log_pkey PRIMARY KEY (id);


--
-- Name: message_sms_log_verification message_sms_log_verification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log_verification
    ADD CONSTRAINT message_sms_log_verification_pkey PRIMARY KEY (id);


--
-- Name: message_sms_rate_limit_policy message_sms_rate_limit_policy_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_rate_limit_policy
    ADD CONSTRAINT message_sms_rate_limit_policy_pkey PRIMARY KEY (platform_id, policy_key);


--
-- Name: message_sms_recipient_rule message_sms_recipient_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_recipient_rule
    ADD CONSTRAINT message_sms_recipient_rule_pkey PRIMARY KEY (id);


--
-- Name: message_sms_template message_sms_template_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_template
    ADD CONSTRAINT message_sms_template_pkey PRIMARY KEY (id);


--
-- Name: payment_callback_event payment_callback_event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_callback_event
    ADD CONSTRAINT payment_callback_event_pkey PRIMARY KEY (id);


--
-- Name: payment_config payment_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_config
    ADD CONSTRAINT payment_config_pkey PRIMARY KEY (id);


--
-- Name: payment_order payment_order_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT payment_order_pkey PRIMARY KEY (id);


--
-- Name: payment_recharge_package payment_recharge_package_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_recharge_package
    ADD CONSTRAINT payment_recharge_package_pkey PRIMARY KEY (id);


--
-- Name: payment_redeem_code_batch payment_redeem_code_batch_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_redeem_code_batch
    ADD CONSTRAINT payment_redeem_code_batch_pkey PRIMARY KEY (id);


--
-- Name: payment_redeem_code payment_redeem_code_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_redeem_code
    ADD CONSTRAINT payment_redeem_code_pkey PRIMARY KEY (id);


--
-- Name: payment_transaction payment_transaction_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transaction
    ADD CONSTRAINT payment_transaction_pkey PRIMARY KEY (id);


--
-- Name: payment_wallet payment_wallet_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_wallet
    ADD CONSTRAINT payment_wallet_pkey PRIMARY KEY (user_id);


--
-- Name: permission_auth_platform permission_auth_platform_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_auth_platform
    ADD CONSTRAINT permission_auth_platform_pkey PRIMARY KEY (id);


--
-- Name: payment_config_version pk_payment_config_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_config_version
    ADD CONSTRAINT pk_payment_config_version PRIMARY KEY (config_id, version);


--
-- Name: storage_cos_config_version pk_storage_cos_config_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_cos_config_version
    ADD CONSTRAINT pk_storage_cos_config_version PRIMARY KEY (cos_config_id, version);


--
-- Name: system_config_cache_state pk_system_config_cache_state; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_config_cache_state
    ADD CONSTRAINT pk_system_config_cache_state PRIMARY KEY (namespace, scope_key);


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
-- Name: realtime_event_outbox realtime_event_outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_event_outbox
    ADD CONSTRAINT realtime_event_outbox_pkey PRIMARY KEY (id);


--
-- Name: realtime_event realtime_event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_event
    ADD CONSTRAINT realtime_event_pkey PRIMARY KEY (sequence);


--
-- Name: realtime_retention_state realtime_retention_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_retention_state
    ADD CONSTRAINT realtime_retention_state_pkey PRIMARY KEY (platform_id);


--
-- Name: storage_cos_config storage_cos_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_cos_config
    ADD CONSTRAINT storage_cos_config_pkey PRIMARY KEY (id);


--
-- Name: storage_file storage_file_object_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_file
    ADD CONSTRAINT storage_file_object_key_key UNIQUE (object_key);


--
-- Name: storage_file storage_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_file
    ADD CONSTRAINT storage_file_pkey PRIMARY KEY (id);


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
-- Name: system_config_cache_outbox system_config_cache_outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_config_cache_outbox
    ADD CONSTRAINT system_config_cache_outbox_pkey PRIMARY KEY (id);


--
-- Name: system_scheduler_job system_scheduler_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_job
    ADD CONSTRAINT system_scheduler_job_pkey PRIMARY KEY (id);


--
-- Name: system_scheduler_run system_scheduler_run_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_run
    ADD CONSTRAINT system_scheduler_run_pkey PRIMARY KEY (id);


--
-- Name: system_scheduler_schedule system_scheduler_schedule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_schedule
    ADD CONSTRAINT system_scheduler_schedule_pkey PRIMARY KEY (id);


--
-- Name: system_setting system_setting_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_setting
    ADD CONSTRAINT system_setting_pkey PRIMARY KEY (id);


--
-- Name: message_sms_log uq_message_sms_log_id_platform; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log
    ADD CONSTRAINT uq_message_sms_log_id_platform UNIQUE (id, platform_id);


--
-- Name: permission_menu uq_rbac_menu_id_platform; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permission_menu
    ADD CONSTRAINT uq_rbac_menu_id_platform UNIQUE (id, platform_id);


--
-- Name: system_config_cache_outbox uq_system_config_cache_outbox_generation; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_config_cache_outbox
    ADD CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE (namespace, scope_key, generation);


--
-- Name: user_account user_account_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_account
    ADD CONSTRAINT user_account_pkey PRIMARY KEY (id);


--
-- Name: user_email_change_log user_email_change_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_email_change_log
    ADD CONSTRAINT user_email_change_log_pkey PRIMARY KEY (id);


--
-- Name: user_login_log user_login_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT user_login_log_pkey PRIMARY KEY (id);


--
-- Name: user_phone_change_log user_phone_change_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_phone_change_log
    ADD CONSTRAINT user_phone_change_log_pkey PRIMARY KEY (id);


--
-- Name: user_profile user_profile_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profile
    ADD CONSTRAINT user_profile_pkey PRIMARY KEY (user_id);


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
-- Name: ix_message_mail_log_created_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_log_created_id_desc ON public.message_mail_log USING btree (created_at DESC, id DESC);


--
-- Name: ix_message_mail_log_platform_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_log_platform_id_desc ON public.message_mail_log USING btree (platform_id, id DESC);


--
-- Name: ix_message_mail_log_scene_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_log_scene_id_desc ON public.message_mail_log USING btree (scene, id DESC);


--
-- Name: ix_message_mail_log_status_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_log_status_id_desc ON public.message_mail_log USING btree (status, id DESC);


--
-- Name: ix_message_mail_log_to_email_prefix; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_mail_log_to_email_prefix ON public.message_mail_log USING btree (to_email varchar_pattern_ops, id DESC);


--
-- Name: ix_message_notification_platform_mailbox; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_platform_mailbox ON public.message_notification USING btree (platform_id, id DESC) WHERE ((audience_type)::text = 'platform'::text);


--
-- Name: ix_message_notification_recipient_mailbox; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_recipient_mailbox ON public.message_notification_recipient USING btree (platform_id, user_id, notification_id DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_message_notification_recipient_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_recipient_unread ON public.message_notification_recipient USING btree (platform_id, user_id, notification_id DESC) WHERE ((deleted_at IS NULL) AND (read_at IS NULL));


--
-- Name: ix_message_notification_task_creator_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_task_creator_list ON public.message_notification_task USING btree (created_by, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_message_notification_task_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_task_due ON public.message_notification_task USING btree (status, scheduled_at, id) WHERE (deleted_at IS NULL);


--
-- Name: ix_message_notification_task_platform_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_notification_task_platform_list ON public.message_notification_task USING btree (platform_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_message_sms_log_created_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_sms_log_created_id_desc ON public.message_sms_log USING btree (created_at DESC, id DESC);


--
-- Name: ix_message_sms_log_platform_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_sms_log_platform_id_desc ON public.message_sms_log USING btree (platform_id, id DESC);


--
-- Name: ix_message_sms_log_scene_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_sms_log_scene_id_desc ON public.message_sms_log USING btree (scene, id DESC);


--
-- Name: ix_message_sms_log_status_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_sms_log_status_id_desc ON public.message_sms_log USING btree (status, id DESC);


--
-- Name: ix_payment_callback_event_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_callback_event_order ON public.payment_callback_event USING btree (order_id, created_at DESC, id DESC);


--
-- Name: ix_payment_callback_event_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_callback_event_pending ON public.payment_callback_event USING btree (created_at, id) WHERE (process_status = 1);


--
-- Name: ix_payment_order_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_order_created ON public.payment_order USING btree (created_at DESC, id DESC);


--
-- Name: ix_payment_order_reconcile_check; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_order_reconcile_check ON public.payment_order USING btree (next_check_at, id) WHERE ((status = 1) AND (next_check_at IS NOT NULL));


--
-- Name: ix_payment_order_reconcile_expiry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_order_reconcile_expiry ON public.payment_order USING btree (expires_at, id) WHERE (status = 1);


--
-- Name: ix_payment_order_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_order_user_created ON public.payment_order USING btree (user_id, created_at DESC, id DESC);


--
-- Name: ix_payment_redeem_code_batch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_redeem_code_batch ON public.payment_redeem_code USING btree (batch_id, id);


--
-- Name: ix_payment_redeem_code_used_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_redeem_code_used_by ON public.payment_redeem_code USING btree (used_by, used_at DESC, id DESC) WHERE (status = 2);


--
-- Name: ix_payment_transaction_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_transaction_created ON public.payment_transaction USING btree (created_at DESC, id DESC);


--
-- Name: ix_payment_transaction_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_transaction_user_created ON public.payment_transaction USING btree (user_id, created_at DESC, id DESC);


--
-- Name: ix_permission_auth_platform_code_history_prefix; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_permission_auth_platform_code_history_prefix ON public.permission_auth_platform USING btree (code varchar_pattern_ops, id);


--
-- Name: ix_permission_user_role_role_history_cursor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_permission_user_role_role_history_cursor ON public.permission_user_role USING btree (role_id, user_id, created_at, deleted_at);


--
-- Name: ix_rbac_menu_parent_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rbac_menu_parent_active ON public.permission_menu USING btree (platform_id, parent_id, sort_order, id) WHERE (deleted_at IS NULL);


--
-- Name: ix_rbac_menu_platform_parent_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rbac_menu_platform_parent_sort ON public.permission_menu USING btree (platform_id, parent_id, sort_order, id);


--
-- Name: ix_realtime_event_outbox_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_realtime_event_outbox_pending ON public.realtime_event_outbox USING btree (available_at, id) WHERE (published_at IS NULL);


--
-- Name: ix_realtime_event_platform_resume; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_realtime_event_platform_resume ON public.realtime_event USING btree (platform_id, sequence) WHERE ((target_type)::text = 'platform'::text);


--
-- Name: ix_realtime_event_retention; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_realtime_event_retention ON public.realtime_event USING btree (occurred_at, sequence);


--
-- Name: ix_realtime_event_user_resume; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_realtime_event_user_resume ON public.realtime_event USING btree (platform_id, target_user_id, sequence) WHERE ((target_type)::text = 'user'::text);


--
-- Name: ix_storage_cos_config_enabled_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_cos_config_enabled_created_at ON public.storage_cos_config USING btree (is_enabled, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_storage_file_cleanup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_file_cleanup ON public.storage_file USING btree (status, credential_expires_at, id) WHERE ((deleted_at IS NULL) AND (status = ANY (ARRAY[1, 3])));


--
-- Name: ix_storage_file_owner_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_file_owner_list ON public.storage_file USING btree (platform_id, user_id, created_at DESC, id DESC) WHERE ((kind = 1) AND (status = 2) AND (deleted_at IS NULL));


--
-- Name: ix_storage_file_temporary_cleanup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_file_temporary_cleanup ON public.storage_file USING btree (credential_expires_at, id) WHERE ((status = 2) AND (upload_object_key IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: ix_storage_upload_rule_code_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_upload_rule_code_rule ON public.storage_upload_rule_code USING btree (rule_id, id) WHERE (deleted_at IS NULL);


--
-- Name: ix_storage_upload_rule_config_enabled_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_storage_upload_rule_config_enabled_created_at ON public.storage_upload_rule USING btree (cos_config_id, is_enabled, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_system_config_cache_outbox_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_config_cache_outbox_pending ON public.system_config_cache_outbox USING btree (available_at, id) WHERE (published_at IS NULL);


--
-- Name: ix_system_scheduler_job_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_list ON public.system_scheduler_job USING btree (created_at DESC, id DESC);


--
-- Name: ix_system_scheduler_job_publish; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_publish ON public.system_scheduler_job USING btree (available_at, id) WHERE (status = 1);


--
-- Name: ix_system_scheduler_job_publish_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_publish_lease ON public.system_scheduler_job USING btree (publish_lease_until, id) WHERE ((status = 1) AND (publish_lease_until IS NOT NULL));


--
-- Name: ix_system_scheduler_job_queued_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_queued_lease ON public.system_scheduler_job USING btree (publish_lease_until, id) WHERE ((status = 2) AND (publish_lease_until IS NOT NULL));


--
-- Name: ix_system_scheduler_job_run_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_run_lease ON public.system_scheduler_job USING btree (run_lease_until, id) WHERE ((status = 3) AND (run_lease_until IS NOT NULL));


--
-- Name: ix_system_scheduler_job_schedule_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_schedule_list ON public.system_scheduler_job USING btree (schedule_id, created_at DESC, id DESC);


--
-- Name: ix_system_scheduler_job_terminal_cleanup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_job_terminal_cleanup ON public.system_scheduler_job USING btree (completed_at, id) WHERE (status = ANY (ARRAY[4, 5, 6]));


--
-- Name: ix_system_scheduler_schedule_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_schedule_due ON public.system_scheduler_schedule USING btree (next_run_at, id) WHERE ((deleted_at IS NULL) AND (is_enabled = 1));


--
-- Name: ix_system_scheduler_schedule_task_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_scheduler_schedule_task_type ON public.system_scheduler_schedule USING btree (task_type, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: ix_system_setting_enabled_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_system_setting_enabled_key ON public.system_setting USING btree (is_enabled, setting_key) WHERE (deleted_at IS NULL);


--
-- Name: ix_user_email_change_log_user_created_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_email_change_log_user_created_id_desc ON public.user_email_change_log USING btree (user_id, created_at DESC, id DESC);


--
-- Name: ix_user_email_change_log_user_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_email_change_log_user_id_desc ON public.user_email_change_log USING btree (user_id, id DESC);


--
-- Name: ix_user_login_log_account_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_login_log_account_created_at ON public.user_login_log USING btree (account, created_at DESC);


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
-- Name: ix_user_phone_change_log_user_created_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_phone_change_log_user_created_id_desc ON public.user_phone_change_log USING btree (user_id, created_at DESC, id DESC);


--
-- Name: ix_user_phone_change_log_user_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_phone_change_log_user_id_desc ON public.user_phone_change_log USING btree (user_id, id DESC);


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
-- Name: ux_message_mail_log_platform_challenge; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_log_platform_challenge ON public.message_mail_log USING btree (platform_id, challenge_id) WHERE (challenge_id IS NOT NULL);


--
-- Name: ux_message_mail_rule_scope_pattern_action_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_rule_scope_pattern_action_active ON public.message_mail_recipient_rule USING btree (scope, pattern, action) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_mail_template_scene_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_template_scene_active ON public.message_mail_template USING btree (scene) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_mail_verification_log; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_mail_verification_log ON public.message_mail_log_verification USING btree (mail_log_id);


--
-- Name: ux_message_notification_broadcast_state; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_notification_broadcast_state ON public.message_notification_broadcast_state USING btree (notification_id, user_id);


--
-- Name: ux_message_notification_recipient; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_notification_recipient ON public.message_notification_recipient USING btree (notification_id, user_id);


--
-- Name: ux_message_notification_source; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_notification_source ON public.message_notification USING btree (platform_id, source_type, source_key);


--
-- Name: ux_message_notification_source_task; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_notification_source_task ON public.message_notification USING btree (source_task_id) WHERE (source_task_id IS NOT NULL);


--
-- Name: ux_message_notification_task_target_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_notification_task_target_active ON public.message_notification_task_target USING btree (task_id, target_type, target_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_sms_config_active_singleton; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_sms_config_active_singleton ON public.message_sms_config USING btree ((true)) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_sms_log_platform_challenge; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_sms_log_platform_challenge ON public.message_sms_log USING btree (platform_id, challenge_id) WHERE (challenge_id IS NOT NULL);


--
-- Name: ux_message_sms_log_verification_log; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_sms_log_verification_log ON public.message_sms_log_verification USING btree (sms_log_id);


--
-- Name: ux_message_sms_recipient_rule_pattern_action_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_sms_recipient_rule_pattern_action_active ON public.message_sms_recipient_rule USING btree (scope, pattern, action) WHERE (deleted_at IS NULL);


--
-- Name: ux_message_sms_template_scene; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_message_sms_template_scene ON public.message_sms_template USING btree (scene);


--
-- Name: ux_payment_callback_event_dedupe; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_callback_event_dedupe ON public.payment_callback_event USING btree (config_id, config_version, dedupe_key);


--
-- Name: ux_payment_config_app_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_config_app_active ON public.payment_config USING btree (provider, environment, app_id) WHERE (deleted_at IS NULL);


--
-- Name: ux_payment_order_order_no; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_order_order_no ON public.payment_order USING btree (order_no);


--
-- Name: ux_payment_order_provider_trade; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_order_provider_trade ON public.payment_order USING btree (config_id, provider_trade_no) WHERE (provider_trade_no IS NOT NULL);


--
-- Name: ux_payment_order_request; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_order_request ON public.payment_order USING btree (user_id, platform_id, request_id);


--
-- Name: ux_payment_recharge_package_code_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_recharge_package_code_active ON public.payment_recharge_package USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: ux_payment_redeem_batch_no; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_redeem_batch_no ON public.payment_redeem_code_batch USING btree (batch_no);


--
-- Name: ux_payment_redeem_batch_request; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_redeem_batch_request ON public.payment_redeem_code_batch USING btree (created_by, request_id);


--
-- Name: ux_payment_redeem_code_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_redeem_code_hash ON public.payment_redeem_code USING btree (code_hash);


--
-- Name: ux_payment_transaction_no; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_transaction_no ON public.payment_transaction USING btree (transaction_no);


--
-- Name: ux_payment_transaction_order; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_transaction_order ON public.payment_transaction USING btree (order_id) WHERE (order_id IS NOT NULL);


--
-- Name: ux_payment_transaction_redeem_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_payment_transaction_redeem_code ON public.payment_transaction USING btree (redeem_code_id) WHERE (redeem_code_id IS NOT NULL);


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
-- Name: ux_realtime_event_dedup_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_realtime_event_dedup_key ON public.realtime_event USING btree (dedup_key);


--
-- Name: ux_realtime_event_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_realtime_event_event_id ON public.realtime_event USING btree (event_id);


--
-- Name: ux_realtime_event_outbox_event; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_realtime_event_outbox_event ON public.realtime_event_outbox USING btree (event_sequence);


--
-- Name: ux_storage_cos_config_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_cos_config_name_active ON public.storage_cos_config USING btree (lower((name)::text)) WHERE (deleted_at IS NULL);


--
-- Name: ux_storage_file_request; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_file_request ON public.storage_file USING btree (platform_id, user_id, request_key) WHERE (request_key IS NOT NULL);


--
-- Name: ux_storage_upload_rule_code_rule_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_upload_rule_code_rule_code ON public.storage_upload_rule_code USING btree (rule_id, code) WHERE (deleted_at IS NULL);


--
-- Name: ux_storage_upload_rule_platform_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_storage_upload_rule_platform_enabled ON public.storage_upload_rule USING btree (platform_id) WHERE ((is_enabled = 1) AND (deleted_at IS NULL));


--
-- Name: ux_system_scheduler_job_active_schedule; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_scheduler_job_active_schedule ON public.system_scheduler_job USING btree (schedule_id) WHERE ((schedule_id IS NOT NULL) AND (status = ANY (ARRAY[1, 2, 3])));


--
-- Name: ux_system_scheduler_job_source_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_scheduler_job_source_key ON public.system_scheduler_job USING btree (task_type, source_key) WHERE (source_key IS NOT NULL);


--
-- Name: ux_system_scheduler_run_job_attempt; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_scheduler_run_job_attempt ON public.system_scheduler_run USING btree (job_id, attempt_no);


--
-- Name: ux_system_scheduler_schedule_builtin_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_scheduler_schedule_builtin_active ON public.system_scheduler_schedule USING btree (builtin_key) WHERE ((deleted_at IS NULL) AND (builtin_key IS NOT NULL));


--
-- Name: ux_system_scheduler_schedule_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_scheduler_schedule_name_active ON public.system_scheduler_schedule USING btree (lower((name)::text)) WHERE (deleted_at IS NULL);


--
-- Name: ux_system_setting_key_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ux_system_setting_key_active ON public.system_setting USING btree (setting_key) WHERE (deleted_at IS NULL);


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
-- Name: payment_callback_event payment_callback_event_identity; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_callback_event_identity BEFORE UPDATE ON public.payment_callback_event FOR EACH ROW EXECUTE FUNCTION public.payment_callback_event_identity_guard();


--
-- Name: payment_callback_event payment_callback_event_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_callback_event_no_delete BEFORE DELETE OR TRUNCATE ON public.payment_callback_event FOR EACH STATEMENT EXECUTE FUNCTION public.payment_fact_no_delete_guard();


--
-- Name: payment_config payment_config_identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_config_identity_guard BEFORE DELETE OR UPDATE ON public.payment_config FOR EACH ROW EXECUTE FUNCTION public.payment_config_guard();


--
-- Name: payment_config payment_config_truncate_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_config_truncate_guard BEFORE TRUNCATE ON public.payment_config FOR EACH STATEMENT EXECUTE FUNCTION public.payment_config_guard();


--
-- Name: payment_config_version payment_config_version_immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_config_version_immutable BEFORE DELETE OR UPDATE OR TRUNCATE ON public.payment_config_version FOR EACH STATEMENT EXECUTE FUNCTION public.payment_config_version_guard();


--
-- Name: payment_order payment_order_identity; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_order_identity BEFORE UPDATE ON public.payment_order FOR EACH ROW EXECUTE FUNCTION public.payment_order_identity_guard();


--
-- Name: payment_order payment_order_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_order_no_delete BEFORE DELETE OR TRUNCATE ON public.payment_order FOR EACH STATEMENT EXECUTE FUNCTION public.payment_fact_no_delete_guard();


--
-- Name: payment_recharge_package payment_recharge_package_identity; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_recharge_package_identity BEFORE UPDATE ON public.payment_recharge_package FOR EACH ROW EXECUTE FUNCTION public.payment_recharge_package_identity_guard();


--
-- Name: payment_recharge_package payment_recharge_package_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_recharge_package_no_delete BEFORE DELETE OR TRUNCATE ON public.payment_recharge_package FOR EACH STATEMENT EXECUTE FUNCTION public.payment_fact_no_delete_guard();


--
-- Name: payment_redeem_code_batch payment_redeem_batch_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_redeem_batch_guard BEFORE DELETE OR UPDATE ON public.payment_redeem_code_batch FOR EACH ROW EXECUTE FUNCTION public.payment_redeem_batch_guard();


--
-- Name: payment_redeem_code_batch payment_redeem_batch_truncate_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_redeem_batch_truncate_guard BEFORE TRUNCATE ON public.payment_redeem_code_batch FOR EACH STATEMENT EXECUTE FUNCTION public.payment_redeem_batch_guard();


--
-- Name: payment_redeem_code payment_redeem_code_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_redeem_code_guard BEFORE DELETE OR UPDATE ON public.payment_redeem_code FOR EACH ROW EXECUTE FUNCTION public.payment_redeem_code_guard();


--
-- Name: payment_redeem_code payment_redeem_code_truncate_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_redeem_code_truncate_guard BEFORE TRUNCATE ON public.payment_redeem_code FOR EACH STATEMENT EXECUTE FUNCTION public.payment_redeem_code_guard();


--
-- Name: payment_transaction payment_transaction_immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_transaction_immutable BEFORE UPDATE ON public.payment_transaction FOR EACH STATEMENT EXECUTE FUNCTION public.payment_transaction_immutable_guard();


--
-- Name: payment_transaction payment_transaction_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_transaction_no_delete BEFORE DELETE OR TRUNCATE ON public.payment_transaction FOR EACH STATEMENT EXECUTE FUNCTION public.payment_fact_no_delete_guard();


--
-- Name: payment_wallet payment_wallet_identity; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_wallet_identity BEFORE UPDATE ON public.payment_wallet FOR EACH ROW EXECUTE FUNCTION public.payment_wallet_identity_guard();


--
-- Name: payment_wallet payment_wallet_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER payment_wallet_no_delete BEFORE DELETE OR TRUNCATE ON public.payment_wallet FOR EACH STATEMENT EXECUTE FUNCTION public.payment_fact_no_delete_guard();


--
-- Name: system_operation_log fk_audit_operation_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_operation_log
    ADD CONSTRAINT fk_audit_operation_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_broadcast_state fk_message_notification_broadcast_notification; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_broadcast_state
    ADD CONSTRAINT fk_message_notification_broadcast_notification FOREIGN KEY (notification_id) REFERENCES public.message_notification(id) ON DELETE CASCADE;


--
-- Name: message_notification_broadcast_state fk_message_notification_broadcast_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_broadcast_state
    ADD CONSTRAINT fk_message_notification_broadcast_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_broadcast_state fk_message_notification_broadcast_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_broadcast_state
    ADD CONSTRAINT fk_message_notification_broadcast_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: message_notification_mailbox_state fk_message_notification_mailbox_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_mailbox_state
    ADD CONSTRAINT fk_message_notification_mailbox_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_mailbox_state fk_message_notification_mailbox_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_mailbox_state
    ADD CONSTRAINT fk_message_notification_mailbox_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: message_notification fk_message_notification_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification
    ADD CONSTRAINT fk_message_notification_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_recipient fk_message_notification_recipient_notification; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_recipient
    ADD CONSTRAINT fk_message_notification_recipient_notification FOREIGN KEY (notification_id) REFERENCES public.message_notification(id) ON DELETE CASCADE;


--
-- Name: message_notification_recipient fk_message_notification_recipient_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_recipient
    ADD CONSTRAINT fk_message_notification_recipient_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_recipient fk_message_notification_recipient_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_recipient
    ADD CONSTRAINT fk_message_notification_recipient_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: message_notification fk_message_notification_source_task; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification
    ADD CONSTRAINT fk_message_notification_source_task FOREIGN KEY (source_task_id) REFERENCES public.message_notification_task(id) ON DELETE SET NULL;


--
-- Name: message_notification_task fk_message_notification_task_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_task
    ADD CONSTRAINT fk_message_notification_task_creator FOREIGN KEY (created_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: message_notification_task fk_message_notification_task_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_task
    ADD CONSTRAINT fk_message_notification_task_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: message_notification_task_target fk_message_notification_task_target_task; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_notification_task_target
    ADD CONSTRAINT fk_message_notification_task_target_task FOREIGN KEY (task_id) REFERENCES public.message_notification_task(id) ON DELETE CASCADE;


--
-- Name: message_sms_log fk_message_sms_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log
    ADD CONSTRAINT fk_message_sms_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: message_sms_log fk_message_sms_log_template; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log
    ADD CONSTRAINT fk_message_sms_log_template FOREIGN KEY (template_id) REFERENCES public.message_sms_template(id);


--
-- Name: message_sms_log_verification fk_message_sms_log_verification_log; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log_verification
    ADD CONSTRAINT fk_message_sms_log_verification_log FOREIGN KEY (sms_log_id, platform_id) REFERENCES public.message_sms_log(id, platform_id);


--
-- Name: message_sms_log_verification fk_message_sms_log_verification_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_log_verification
    ADD CONSTRAINT fk_message_sms_log_verification_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: message_sms_rate_limit_policy fk_message_sms_rate_limit_policy_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_sms_rate_limit_policy
    ADD CONSTRAINT fk_message_sms_rate_limit_policy_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: payment_callback_event fk_payment_callback_event_order; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_callback_event
    ADD CONSTRAINT fk_payment_callback_event_order FOREIGN KEY (order_id) REFERENCES public.payment_order(id) ON DELETE RESTRICT;


--
-- Name: payment_callback_event fk_payment_callback_event_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_callback_event
    ADD CONSTRAINT fk_payment_callback_event_version FOREIGN KEY (config_id, config_version) REFERENCES public.payment_config_version(config_id, version) ON DELETE RESTRICT;


--
-- Name: payment_config fk_payment_config_current_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_config
    ADD CONSTRAINT fk_payment_config_current_version FOREIGN KEY (id, current_version) REFERENCES public.payment_config_version(config_id, version) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;


--
-- Name: payment_config_version fk_payment_config_version_config; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_config_version
    ADD CONSTRAINT fk_payment_config_version_config FOREIGN KEY (config_id) REFERENCES public.payment_config(id) ON DELETE RESTRICT;


--
-- Name: payment_order fk_payment_order_config; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT fk_payment_order_config FOREIGN KEY (config_id) REFERENCES public.payment_config(id) ON DELETE RESTRICT;


--
-- Name: payment_order fk_payment_order_package; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT fk_payment_order_package FOREIGN KEY (package_id) REFERENCES public.payment_recharge_package(id) ON DELETE RESTRICT;


--
-- Name: payment_order fk_payment_order_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT fk_payment_order_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: payment_order fk_payment_order_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT fk_payment_order_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: payment_order fk_payment_order_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_order
    ADD CONSTRAINT fk_payment_order_version FOREIGN KEY (config_id, config_version) REFERENCES public.payment_config_version(config_id, version) ON DELETE RESTRICT;


--
-- Name: payment_redeem_code_batch fk_payment_redeem_batch_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_redeem_code_batch
    ADD CONSTRAINT fk_payment_redeem_batch_creator FOREIGN KEY (created_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: payment_redeem_code fk_payment_redeem_code_batch; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_redeem_code
    ADD CONSTRAINT fk_payment_redeem_code_batch FOREIGN KEY (batch_id) REFERENCES public.payment_redeem_code_batch(id) ON DELETE RESTRICT;


--
-- Name: payment_redeem_code fk_payment_redeem_code_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_redeem_code
    ADD CONSTRAINT fk_payment_redeem_code_user FOREIGN KEY (used_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: payment_transaction fk_payment_transaction_order; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transaction
    ADD CONSTRAINT fk_payment_transaction_order FOREIGN KEY (order_id) REFERENCES public.payment_order(id) ON DELETE RESTRICT;


--
-- Name: payment_transaction fk_payment_transaction_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transaction
    ADD CONSTRAINT fk_payment_transaction_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: payment_transaction fk_payment_transaction_redeem_code; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transaction
    ADD CONSTRAINT fk_payment_transaction_redeem_code FOREIGN KEY (redeem_code_id) REFERENCES public.payment_redeem_code(id) ON DELETE RESTRICT;


--
-- Name: payment_transaction fk_payment_transaction_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transaction
    ADD CONSTRAINT fk_payment_transaction_user FOREIGN KEY (user_id) REFERENCES public.payment_wallet(user_id) ON DELETE RESTRICT;


--
-- Name: payment_wallet fk_payment_wallet_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_wallet
    ADD CONSTRAINT fk_payment_wallet_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


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
-- Name: realtime_event_outbox fk_realtime_event_outbox_event; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_event_outbox
    ADD CONSTRAINT fk_realtime_event_outbox_event FOREIGN KEY (event_sequence) REFERENCES public.realtime_event(sequence) ON DELETE CASCADE;


--
-- Name: realtime_event fk_realtime_event_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_event
    ADD CONSTRAINT fk_realtime_event_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: realtime_event fk_realtime_event_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_event
    ADD CONSTRAINT fk_realtime_event_user FOREIGN KEY (target_user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: realtime_retention_state fk_realtime_retention_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_retention_state
    ADD CONSTRAINT fk_realtime_retention_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: storage_cos_config fk_storage_cos_config_current_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_cos_config
    ADD CONSTRAINT fk_storage_cos_config_current_version FOREIGN KEY (id, current_version) REFERENCES public.storage_cos_config_version(cos_config_id, version) DEFERRABLE INITIALLY DEFERRED;


--
-- Name: storage_cos_config_version fk_storage_cos_config_version_config; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storage_cos_config_version
    ADD CONSTRAINT fk_storage_cos_config_version_config FOREIGN KEY (cos_config_id) REFERENCES public.storage_cos_config(id) ON DELETE RESTRICT;


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
-- Name: system_config_cache_outbox fk_system_config_cache_outbox_state; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_config_cache_outbox
    ADD CONSTRAINT fk_system_config_cache_outbox_state FOREIGN KEY (namespace, scope_key) REFERENCES public.system_config_cache_state(namespace, scope_key);


--
-- Name: system_scheduler_job fk_system_scheduler_job_retry_of; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_job
    ADD CONSTRAINT fk_system_scheduler_job_retry_of FOREIGN KEY (retry_of_job_id) REFERENCES public.system_scheduler_job(id) ON DELETE SET NULL;


--
-- Name: system_scheduler_job fk_system_scheduler_job_schedule; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_job
    ADD CONSTRAINT fk_system_scheduler_job_schedule FOREIGN KEY (schedule_id) REFERENCES public.system_scheduler_schedule(id) ON DELETE SET NULL;


--
-- Name: system_scheduler_job fk_system_scheduler_job_triggered_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_job
    ADD CONSTRAINT fk_system_scheduler_job_triggered_by FOREIGN KEY (triggered_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: system_scheduler_run fk_system_scheduler_run_job; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_run
    ADD CONSTRAINT fk_system_scheduler_run_job FOREIGN KEY (job_id) REFERENCES public.system_scheduler_job(id) ON DELETE CASCADE;


--
-- Name: system_scheduler_schedule fk_system_scheduler_schedule_created_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_schedule
    ADD CONSTRAINT fk_system_scheduler_schedule_created_by FOREIGN KEY (created_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: system_scheduler_schedule fk_system_scheduler_schedule_updated_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_scheduler_schedule
    ADD CONSTRAINT fk_system_scheduler_schedule_updated_by FOREIGN KEY (updated_by) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: user_email_change_log fk_user_email_change_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_email_change_log
    ADD CONSTRAINT fk_user_email_change_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: user_email_change_log fk_user_email_change_log_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_email_change_log
    ADD CONSTRAINT fk_user_email_change_log_user FOREIGN KEY (user_id) REFERENCES public.user_account(id);


--
-- Name: user_login_log fk_user_login_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT fk_user_login_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id) ON DELETE RESTRICT;


--
-- Name: user_login_log fk_user_login_log_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_login_log
    ADD CONSTRAINT fk_user_login_log_user FOREIGN KEY (user_id) REFERENCES public.user_account(id) ON DELETE RESTRICT;


--
-- Name: user_phone_change_log fk_user_phone_change_log_platform; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_phone_change_log
    ADD CONSTRAINT fk_user_phone_change_log_platform FOREIGN KEY (platform_id) REFERENCES public.permission_auth_platform(id);


--
-- Name: user_phone_change_log fk_user_phone_change_log_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_phone_change_log
    ADD CONSTRAINT fk_user_phone_change_log_user FOREIGN KEY (user_id) REFERENCES public.user_account(id);


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
-- PostgreSQL database dump complete
--

\unrestrict E2OGrYmQHI6urhZlrUaUxOJJfOcHu9WMMehOyLWKik31RxIUSjIj1FHXjAC87D3

