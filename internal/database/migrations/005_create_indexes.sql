-- +goose Up

-- ==================== User Service 索引 / User Service Indexes ====================

-- email 查找（登录）。UNIQUE 约束已自动创建索引，但显式创建可读性更好。
-- Email lookup (login). UNIQUE constraint auto-creates an index, but explicit creation improves readability.
-- 注意：users.email 已有 UNIQUE 约束，PG 自动创建了唯一索引，所以这里不需要额外创建。
-- Note: users.email already has a UNIQUE constraint, PG auto-creates a unique index — no extra index needed.

-- 按状态筛选活跃用户，排除已删除的（部分索引，更小更快）。
-- Filter active users by status, excluding deleted ones (partial index — smaller and faster).
CREATE INDEX idx_users_status ON user_service.users(status) WHERE deleted_at IS NULL;

-- 按用户查地址列表。
-- Lookup addresses by user.
CREATE INDEX idx_user_addresses_user ON user_service.user_addresses(user_id);

-- 按用户查未吊销的 refresh token。
-- Lookup non-revoked refresh tokens by user.
CREATE INDEX idx_refresh_tokens_user ON user_service.refresh_tokens(user_id) WHERE revoked_at IS NULL;

-- 按过期时间查找需要清理的 token（定时任务用）。
-- Find tokens to clean up by expiration time (for scheduled cleanup jobs).
CREATE INDEX idx_refresh_tokens_expires ON user_service.refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- ==================== Product Service 索引 / Product Service Indexes ====================

-- 按状态筛选商品（列表页）。
-- Filter products by status (list page).
CREATE INDEX idx_products_status ON product_service.products(status) WHERE deleted_at IS NULL;

-- 按 slug 查商品（URL 路由用）。
-- Lookup product by slug (URL routing).
CREATE INDEX idx_products_slug ON product_service.products(slug);

-- 按品牌筛选。
-- Filter by brand.
CREATE INDEX idx_products_brand ON product_service.products(brand) WHERE deleted_at IS NULL;

-- 按销量排序（热门商品列表）。
-- Sort by sales (popular products list).
CREATE INDEX idx_products_sales ON product_service.products(total_sales DESC) WHERE status = 'active' AND deleted_at IS NULL;

-- 商品属性 GIN 索引（JSONB 查询加速）。
-- Product attributes GIN index (speeds up JSONB queries).
CREATE INDEX idx_products_attrs ON product_service.products USING GIN(attributes);

-- SKU 按商品查找。
-- Lookup SKUs by product.
CREATE INDEX idx_skus_product ON product_service.skus(product_id);

-- SKU 按编码查找。
-- Lookup SKU by code.
CREATE INDEX idx_skus_code ON product_service.skus(sku_code);

-- 低库存预警（定时任务扫描用）。
-- Low stock alert (for scheduled scan jobs).
CREATE INDEX idx_skus_stock_low ON product_service.skus(product_id) WHERE stock <= low_stock AND status = 'active';

-- 分类树：按父分类查子分类。
-- Category tree: lookup children by parent.
CREATE INDEX idx_categories_parent ON product_service.categories(parent_id);

-- 分类 slug 查找。
-- Category slug lookup.
CREATE INDEX idx_categories_slug ON product_service.categories(slug);

-- ==================== Order Service 索引 / Order Service Indexes ====================

-- 按用户查订单列表。
-- Lookup orders by user.
CREATE INDEX idx_orders_user ON order_service.orders(user_id);

-- 按用户+状态筛选订单（如"我的待付款订单"）。
-- Filter orders by user + status (e.g. "my pending orders").
CREATE INDEX idx_orders_user_status ON order_service.orders(user_id, status);

-- 按状态筛选（后台管理用）。
-- Filter by status (admin dashboard).
CREATE INDEX idx_orders_status ON order_service.orders(status);

-- 按订单号查找（客服查询用）。
-- Lookup by order number (customer service queries).
CREATE INDEX idx_orders_no ON order_service.orders(order_no);

-- 超时未支付订单扫描（定时取消用），只索引 pending 状态。
-- Scan unpaid expired orders (for auto-cancellation), only index pending status.
CREATE INDEX idx_orders_expires ON order_service.orders(expires_at) WHERE status = 'pending';

-- 幂等键查找（创建订单时去重）。
-- Idempotency key lookup (dedup on order creation).
CREATE INDEX idx_orders_idempotency ON order_service.orders(idempotency_key);

-- 按订单查明细。
-- Lookup items by order.
CREATE INDEX idx_order_items_order ON order_service.order_items(order_id);

-- 按 SKU 查明细（统计 SKU 销量用）。
-- Lookup items by SKU (for SKU sales statistics).
CREATE INDEX idx_order_items_sku ON order_service.order_items(sku_id);

-- 按订单查支付记录。
-- Lookup payment records by order.
CREATE INDEX idx_payment_records_order ON order_service.payment_records(order_id);

-- 按 SKU 查库存操作记录。
-- Lookup stock operations by SKU.
CREATE INDEX idx_stock_ops_sku ON order_service.stock_operations(sku_id);

-- 按订单查库存操作记录。
-- Lookup stock operations by order.
CREATE INDEX idx_stock_ops_order ON order_service.stock_operations(order_id);

-- +goose Down
-- Order Service
DROP INDEX IF EXISTS order_service.idx_stock_ops_order;
DROP INDEX IF EXISTS order_service.idx_stock_ops_sku;
DROP INDEX IF EXISTS order_service.idx_payment_records_order;
DROP INDEX IF EXISTS order_service.idx_order_items_sku;
DROP INDEX IF EXISTS order_service.idx_order_items_order;
DROP INDEX IF EXISTS order_service.idx_orders_idempotency;
DROP INDEX IF EXISTS order_service.idx_orders_expires;
DROP INDEX IF EXISTS order_service.idx_orders_no;
DROP INDEX IF EXISTS order_service.idx_orders_status;
DROP INDEX IF EXISTS order_service.idx_orders_user_status;
DROP INDEX IF EXISTS order_service.idx_orders_user;

-- Product Service
DROP INDEX IF EXISTS product_service.idx_categories_slug;
DROP INDEX IF EXISTS product_service.idx_categories_parent;
DROP INDEX IF EXISTS product_service.idx_skus_stock_low;
DROP INDEX IF EXISTS product_service.idx_skus_code;
DROP INDEX IF EXISTS product_service.idx_skus_product;
DROP INDEX IF EXISTS product_service.idx_products_attrs;
DROP INDEX IF EXISTS product_service.idx_products_sales;
DROP INDEX IF EXISTS product_service.idx_products_brand;
DROP INDEX IF EXISTS product_service.idx_products_slug;
DROP INDEX IF EXISTS product_service.idx_products_status;

-- User Service
DROP INDEX IF EXISTS user_service.idx_refresh_tokens_expires;
DROP INDEX IF EXISTS user_service.idx_refresh_tokens_user;
DROP INDEX IF EXISTS user_service.idx_user_addresses_user;
DROP INDEX IF EXISTS user_service.idx_users_status;
