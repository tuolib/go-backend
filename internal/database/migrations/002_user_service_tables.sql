-- +goose Up

-- 用户表：存储用户基本信息和认证数据。
-- Users table: stores user profile and authentication data.
CREATE TABLE user_service.users (
    id          VARCHAR(21)    PRIMARY KEY,                         -- nanoid 主键 / nanoid primary key
    email       VARCHAR(255)   NOT NULL UNIQUE,                    -- 邮箱唯一，用于登录 / Unique email for login
    password    VARCHAR(255)   NOT NULL,                           -- Argon2id 哈希后的密码 / Argon2id hashed password
    nickname    VARCHAR(50),                                       -- 昵称（可选）/ Nickname (optional)
    avatar_url  TEXT,                                              -- 头像 URL / Avatar URL
    phone       VARCHAR(20),                                       -- 手机号 / Phone number
    status      VARCHAR(20)    NOT NULL DEFAULT 'active',          -- active / suspended / deleted
    last_login  TIMESTAMPTZ,                                       -- 最近登录时间 / Last login timestamp
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ                                        -- 软删除标记，非 NULL 表示已删除 / Soft delete marker, non-NULL means deleted
);

-- 收货地址表：一个用户可以有多个地址。
-- User addresses table: a user can have multiple addresses.
CREATE TABLE user_service.user_addresses (
    id          VARCHAR(21)    PRIMARY KEY,
    user_id     VARCHAR(21)    NOT NULL REFERENCES user_service.users(id), -- 外键关联用户 / Foreign key to users
    label       VARCHAR(50),                                       -- 标签如"家"、"公司" / Label like "Home", "Office"
    recipient   VARCHAR(100)   NOT NULL,                           -- 收件人姓名 / Recipient name
    phone       VARCHAR(20)    NOT NULL,                           -- 收件人电话 / Recipient phone
    province    VARCHAR(50)    NOT NULL,                           -- 省份 / Province
    city        VARCHAR(50)    NOT NULL,                           -- 城市 / City
    district    VARCHAR(50)    NOT NULL,                           -- 区/县 / District
    address     TEXT           NOT NULL,                           -- 详细地址 / Detailed address
    postal_code VARCHAR(10),                                       -- 邮编 / Postal code
    is_default  BOOLEAN        NOT NULL DEFAULT false,             -- 是否默认地址 / Whether this is the default address
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Refresh token 表：存储 refresh token 的哈希值，支持吊销。
-- Refresh tokens table: stores SHA-256 hashed tokens, supports revocation.
-- 注意：存的是 token 的 SHA-256 哈希，不是原始 token（防数据库泄露）。
-- Note: stores SHA-256 hash of the token, not the raw token (defense against DB leaks).
CREATE TABLE user_service.refresh_tokens (
    id          VARCHAR(21)    PRIMARY KEY,
    user_id     VARCHAR(21)    NOT NULL REFERENCES user_service.users(id),
    token_hash  VARCHAR(255)   NOT NULL UNIQUE,                    -- SHA-256(refreshToken)
    expires_at  TIMESTAMPTZ    NOT NULL,                           -- Token 过期时间 / Token expiration
    revoked_at  TIMESTAMPTZ,                                       -- 吊销时间，非 NULL 表示已吊销 / Revocation time, non-NULL means revoked
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS user_service.refresh_tokens;
DROP TABLE IF EXISTS user_service.user_addresses;
DROP TABLE IF EXISTS user_service.users;
