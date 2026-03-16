-- +goose Up

-- PostgreSQL 全文搜索 GIN 索引：对商品标题+描述+品牌做全文索引。
-- PostgreSQL full-text search GIN index: indexes product title + description + brand.
--
-- to_tsvector('simple', ...) 用 'simple' 配置（不做词干提取），适合中英文混合搜索。
-- to_tsvector('simple', ...) uses the 'simple' config (no stemming), suitable for mixed Chinese/English search.
--
-- GIN (Generalized Inverted Index) 是全文搜索的标准索引类型，支持快速匹配。
-- GIN (Generalized Inverted Index) is the standard index type for full-text search, enabling fast matching.
--
-- coalesce(description, '') 处理 NULL 值：NULL || 'text' 在 PG 中结果是 NULL，会导致整个表达式为 NULL。
-- coalesce(description, '') handles NULLs: NULL || 'text' in PG returns NULL, which would make the entire expression NULL.
CREATE INDEX idx_products_fulltext ON product_service.products
    USING GIN(to_tsvector('simple', title || ' ' || coalesce(description, '') || ' ' || coalesce(brand, '')));

-- +goose Down
DROP INDEX IF EXISTS product_service.idx_products_fulltext;
