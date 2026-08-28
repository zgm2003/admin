-- One-time database change for the Admin personal center.
-- Run this explicitly against an initialized database. The API never executes it at startup.

CREATE TABLE user_profile (
    user_id BIGINT PRIMARY KEY,
    birthday DATE NULL,
    gender SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_profile_account
        FOREIGN KEY (user_id) REFERENCES user_account (id) ON DELETE RESTRICT,
    CONSTRAINT ck_user_profile_gender
        CHECK (gender IN (0, 1, 2))
);

CREATE UNIQUE INDEX ux_user_account_phone_active
    ON user_account (phone)
    WHERE phone IS NOT NULL AND deleted_at IS NULL;
