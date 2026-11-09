-- AI 官方模型目录：官方美元价 + 系统人民币价，价格均以 1M tokens 为单位。
-- 金额使用 micros（1 单位 = 0.000001 美元/人民币）存储，避免浮点误差。
-- 本表由官方目录 JSON 同步维护，页面不新增/删除官方模型，只允许修改人民币价。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

CREATE TABLE IF NOT EXISTS ai_official_model (
    id bigserial PRIMARY KEY,
    vendor_code varchar(64) NOT NULL,
    vendor_name varchar(128) NOT NULL,
    model_key varchar(200) NOT NULL,
    display_name varchar(200) NOT NULL,
    input_usd_micros bigint NOT NULL CHECK (input_usd_micros >= 0),
    output_usd_micros bigint NOT NULL CHECK (output_usd_micros >= 0),
    input_cny_micros bigint NOT NULL CHECK (input_cny_micros >= 0),
    output_cny_micros bigint NOT NULL CHECK (output_cny_micros >= 0),
    catalog_version varchar(64) NOT NULL,
    source_url varchar(512) NOT NULL,
    verified_at timestamptz NOT NULL,
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ai_official_model_vendor_model_key UNIQUE (vendor_code, model_key)
);

-- 兼容已经建表的环境；新环境执行也不会重复加列。
ALTER TABLE ai_official_model ADD COLUMN IF NOT EXISTS sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0);

CREATE INDEX IF NOT EXISTS ai_official_model_vendor_idx ON ai_official_model (vendor_code, display_name);

COMMENT ON TABLE ai_official_model IS 'AI 官方模型价格目录；官方价为美元/1M tokens，系统价为人民币/1M tokens';
COMMENT ON COLUMN ai_official_model.input_usd_micros IS '官方输入价，美元 micros / 1M tokens';
COMMENT ON COLUMN ai_official_model.output_usd_micros IS '官方输出价，美元 micros / 1M tokens';
COMMENT ON COLUMN ai_official_model.input_cny_micros IS '系统输入价，人民币 micros / 1M tokens';
COMMENT ON COLUMN ai_official_model.output_cny_micros IS '系统输出价，人民币 micros / 1M tokens';
COMMENT ON COLUMN ai_official_model.sort_order IS '官方目录顺序；数字越小越靠前';
COMMENT ON COLUMN ai_official_model.version IS '乐观锁版本；编辑人民币价和目录同步都会递增';
