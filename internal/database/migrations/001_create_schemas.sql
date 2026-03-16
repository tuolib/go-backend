-- +goose Up
-- 创建服务级 schema 隔离。每个服务使用独立 schema，共享同一个数据库但逻辑隔离。
-- Create service-level schema isolation. Each service gets its own schema — shared DB but logically isolated.
-- Cart 服务纯用 Redis，不需要 PG schema。
-- Cart service is Redis-only, no PG schema needed.
CREATE SCHEMA IF NOT EXISTS user_service;
CREATE SCHEMA IF NOT EXISTS product_service;
CREATE SCHEMA IF NOT EXISTS order_service;

-- +goose Down
DROP SCHEMA IF EXISTS order_service CASCADE;
DROP SCHEMA IF EXISTS product_service CASCADE;
DROP SCHEMA IF EXISTS user_service CASCADE;
