-- +goose Up

-- 分类表：支持多级分类树（parent_id 自引用）。
-- Categories table: supports multi-level category tree (parent_id self-reference).
CREATE TABLE product_service.categories (
    id          VARCHAR(21)    PRIMARY KEY,
    parent_id   VARCHAR(21)    REFERENCES product_service.categories(id), -- 父分类，NULL 表示顶级 / Parent category, NULL means top-level
    name        VARCHAR(100)   NOT NULL,                           -- 分类名 / Category name
    slug        VARCHAR(100)   NOT NULL UNIQUE,                    -- URL 友好的唯一标识 / URL-friendly unique identifier
    icon_url    TEXT,                                               -- 图标 URL / Icon URL
    sort_order  INTEGER        NOT NULL DEFAULT 0,                 -- 排序权重，越小越靠前 / Sort weight, lower = higher priority
    is_active   BOOLEAN        NOT NULL DEFAULT true,              -- 是否启用 / Whether active
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 商品表：存储商品基本信息，价格范围是冗余字段（从 SKU 聚合），列表页用。
-- Products table: stores product info. min/max price are denormalized from SKUs for list page display.
CREATE TABLE product_service.products (
    id          VARCHAR(21)    PRIMARY KEY,
    title       VARCHAR(200)   NOT NULL,                           -- 商品标题 / Product title
    slug        VARCHAR(200)   NOT NULL UNIQUE,                    -- URL 友好标识 / URL-friendly slug
    description TEXT,                                               -- 商品描述 / Product description
    brand       VARCHAR(100),                                      -- 品牌 / Brand
    status      VARCHAR(20)    NOT NULL DEFAULT 'draft',           -- draft / active / archived
    attributes  JSONB,                                             -- 通用属性 JSON / Generic attributes JSON
    min_price   DECIMAL(12,2),                                     -- 最低 SKU 价格（冗余）/ Lowest SKU price (denormalized)
    max_price   DECIMAL(12,2),                                     -- 最高 SKU 价格（冗余）/ Highest SKU price (denormalized)
    total_sales INTEGER        NOT NULL DEFAULT 0,                 -- 累计销量 / Total sales count
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ                                        -- 软删除 / Soft delete
);

-- 商品-分类多对多关联表：一个商品可以属于多个分类。
-- Product-category junction table: a product can belong to multiple categories.
CREATE TABLE product_service.product_categories (
    product_id  VARCHAR(21)    NOT NULL REFERENCES product_service.products(id),
    category_id VARCHAR(21)    NOT NULL REFERENCES product_service.categories(id),
    PRIMARY KEY (product_id, category_id)                          -- 联合主键，自动唯一 / Composite PK, inherently unique
);

-- 商品图片表：一个商品可以有多张图片，is_primary 标记主图。
-- Product images table: a product can have multiple images, is_primary marks the main one.
CREATE TABLE product_service.product_images (
    id          VARCHAR(21)    PRIMARY KEY,
    product_id  VARCHAR(21)    NOT NULL REFERENCES product_service.products(id),
    url         TEXT           NOT NULL,                           -- 图片 URL / Image URL
    alt_text    VARCHAR(200),                                      -- 图片替代文本（SEO + 无障碍）/ Alt text (SEO + accessibility)
    is_primary  BOOLEAN        NOT NULL DEFAULT false,             -- 是否主图 / Whether this is the primary image
    sort_order  INTEGER        NOT NULL DEFAULT 0,                 -- 排序权重 / Sort weight
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- SKU 表：Stock Keeping Unit，每个 SKU 是一个可购买的具体规格组合（如红色+XL）。
-- SKUs table: each SKU is a purchasable variant (e.g. Red + XL).
CREATE TABLE product_service.skus (
    id            VARCHAR(21)    PRIMARY KEY,
    product_id    VARCHAR(21)    NOT NULL REFERENCES product_service.products(id),
    sku_code      VARCHAR(50)    NOT NULL UNIQUE,                  -- SKU 编码，全局唯一 / SKU code, globally unique
    price         DECIMAL(12,2)  NOT NULL,                         -- 售价 / Selling price
    compare_price DECIMAL(12,2),                                   -- 划线价（原价）/ Compare-at price (original price)
    cost_price    DECIMAL(12,2),                                   -- 成本价（内部用）/ Cost price (internal)
    stock         INTEGER        NOT NULL DEFAULT 0,               -- DB 真实库存（Redis 为预扣缓存）/ DB real stock (Redis is reservation cache)
    low_stock     INTEGER        NOT NULL DEFAULT 5,               -- 低库存预警阈值 / Low stock alert threshold
    weight        DECIMAL(8,2),                                    -- 重量（kg）/ Weight (kg)
    attributes    JSONB,                                           -- 规格属性如 {"color":"红","size":"XL"} / Variant attrs like {"color":"Red","size":"XL"}
    barcode       VARCHAR(50),                                     -- 条形码 / Barcode
    status        VARCHAR(20)    NOT NULL DEFAULT 'active',        -- active / inactive
    version       INTEGER        NOT NULL DEFAULT 0,               -- 乐观锁版本号 / Optimistic lock version
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 轮播图/Banner 表：首页展示用。
-- Banners table: for homepage display.
CREATE TABLE product_service.banners (
    id          VARCHAR(21)    PRIMARY KEY,
    title       VARCHAR(200)   NOT NULL,                           -- 标题 / Title
    subtitle    VARCHAR(200),                                      -- 副标题 / Subtitle
    image_url   TEXT           NOT NULL,                           -- 图片 URL / Image URL
    link_type   VARCHAR(20),                                       -- product / category
    link_value  VARCHAR(200),                                      -- 链接目标 ID / Link target ID
    sort_order  INTEGER        NOT NULL DEFAULT 0,                 -- 排序权重 / Sort weight
    is_active   BOOLEAN        NOT NULL DEFAULT true,              -- 是否启用 / Whether active
    start_at    TIMESTAMPTZ,                                       -- 展示开始时间 / Display start time
    end_at      TIMESTAMPTZ,                                       -- 展示结束时间 / Display end time
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS product_service.banners;
DROP TABLE IF EXISTS product_service.skus;
DROP TABLE IF EXISTS product_service.product_images;
DROP TABLE IF EXISTS product_service.product_categories;
DROP TABLE IF EXISTS product_service.products;
DROP TABLE IF EXISTS product_service.categories;
