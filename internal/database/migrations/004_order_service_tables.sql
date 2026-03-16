-- +goose Up

-- 订单主表：记录订单的全生命周期。
-- Orders table: records the full lifecycle of an order.
CREATE TABLE order_service.orders (
    id              VARCHAR(21)    PRIMARY KEY,
    order_no        VARCHAR(32)    NOT NULL UNIQUE,                -- 业务订单号（时间戳+随机数）/ Business order number (timestamp + random)
    user_id         VARCHAR(21)    NOT NULL,                       -- 下单用户（不 FK，跨 schema）/ Ordering user (no FK, cross-schema)
    status          VARCHAR(20)    NOT NULL DEFAULT 'pending',     -- pending/paid/shipped/delivered/completed/cancelled/refunded
    total_amount    DECIMAL(12,2)  NOT NULL,                       -- 商品总金额 / Total product amount
    discount_amount DECIMAL(12,2)  NOT NULL DEFAULT 0,             -- 优惠金额 / Discount amount
    pay_amount      DECIMAL(12,2)  NOT NULL,                       -- 实付金额 = total - discount / Actual payment = total - discount
    payment_method  VARCHAR(20),                                   -- 支付方式 / Payment method
    payment_no      VARCHAR(100),                                  -- 第三方支付交易号 / Third-party payment transaction ID
    paid_at         TIMESTAMPTZ,                                   -- 支付时间 / Payment timestamp
    shipped_at      TIMESTAMPTZ,                                   -- 发货时间 / Shipping timestamp
    delivered_at    TIMESTAMPTZ,                                   -- 送达时间 / Delivery timestamp
    completed_at    TIMESTAMPTZ,                                   -- 完成时间 / Completion timestamp
    cancelled_at    TIMESTAMPTZ,                                   -- 取消时间 / Cancellation timestamp
    cancel_reason   TEXT,                                          -- 取消原因 / Cancellation reason
    remark          TEXT,                                          -- 订单备注 / Order remark
    idempotency_key VARCHAR(64)    UNIQUE,                         -- 幂等键：防止重复提交创建多个订单 / Idempotency key: prevents duplicate order creation
    expires_at      TIMESTAMPTZ    NOT NULL,                       -- 支付截止时间，过期自动取消 / Payment deadline, auto-cancel on expiry
    version         INTEGER        NOT NULL DEFAULT 0,             -- 乐观锁版本号 / Optimistic lock version
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 订单明细表：每个订单包含多个商品项，价格/标题等是下单时的快照。
-- Order items table: each order has multiple product items. Prices/titles are snapshots at order time.
-- 为什么要快照？商品信息随时可能变，但订单记录应反映下单那一刻的真实情况。
-- Why snapshots? Product info can change anytime, but the order record should reflect the exact state at order time.
CREATE TABLE order_service.order_items (
    id              VARCHAR(21)    PRIMARY KEY,
    order_id        VARCHAR(21)    NOT NULL REFERENCES order_service.orders(id),
    product_id      VARCHAR(21)    NOT NULL,                       -- 商品 ID（快照引用）/ Product ID (snapshot reference)
    sku_id          VARCHAR(21)    NOT NULL,                       -- SKU ID（快照引用）/ SKU ID (snapshot reference)
    product_title   VARCHAR(200)   NOT NULL,                       -- 商品标题快照 / Product title snapshot
    sku_attrs       JSONB          NOT NULL DEFAULT '{}',          -- SKU 规格快照 / SKU attributes snapshot
    image_url       TEXT,                                          -- 商品图片快照 / Product image snapshot
    unit_price      DECIMAL(12,2)  NOT NULL,                       -- 下单时单价 / Unit price at order time
    quantity        INTEGER        NOT NULL,                       -- 购买数量 / Purchase quantity
    subtotal        DECIMAL(12,2)  NOT NULL,                       -- 小计 = unit_price × quantity / Subtotal = unit_price × quantity
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 订单地址表：下单时快照收货地址，和 user_addresses 解耦。
-- Order addresses table: snapshots the delivery address at order time, decoupled from user_addresses.
-- 一个订单只有一个地址（UNIQUE 约束）。
-- One order has exactly one address (UNIQUE constraint).
CREATE TABLE order_service.order_addresses (
    id              VARCHAR(21)    PRIMARY KEY,
    order_id        VARCHAR(21)    NOT NULL UNIQUE REFERENCES order_service.orders(id), -- 一单一地址 / One address per order
    recipient       VARCHAR(100)   NOT NULL,
    phone           VARCHAR(20)    NOT NULL,
    province        VARCHAR(50)    NOT NULL,
    city            VARCHAR(50)    NOT NULL,
    district        VARCHAR(50)    NOT NULL,
    address         TEXT           NOT NULL,
    postal_code     VARCHAR(10),
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 支付记录表：记录每次支付尝试。
-- Payment records table: records each payment attempt.
CREATE TABLE order_service.payment_records (
    id              VARCHAR(21)    PRIMARY KEY,
    order_id        VARCHAR(21)    NOT NULL REFERENCES order_service.orders(id),
    payment_method  VARCHAR(20)    NOT NULL,                       -- stripe / alipay / wechat / mock
    amount          DECIMAL(12,2)  NOT NULL,                       -- 支付金额 / Payment amount
    status          VARCHAR(20)    NOT NULL DEFAULT 'pending',     -- pending / success / failed / refunded
    transaction_id  VARCHAR(100),                                  -- 第三方交易号 / Third-party transaction ID
    raw_notify      JSONB,                                         -- 原始回调报文（用于对账）/ Raw callback payload (for reconciliation)
    idempotency_key VARCHAR(64)    UNIQUE,                         -- 幂等键 / Idempotency key
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 库存操作记录表：记录每次库存变动（预扣/确认/释放/调整），用于审计和对账。
-- Stock operations table: records every stock change (reserve/confirm/release/adjust) for auditing and reconciliation.
CREATE TABLE order_service.stock_operations (
    id          VARCHAR(21)    PRIMARY KEY,
    sku_id      VARCHAR(21)    NOT NULL,                           -- 关联的 SKU / Associated SKU
    order_id    VARCHAR(21),                                       -- 关联的订单（adjust 时可为空）/ Associated order (may be NULL for adjust)
    type        VARCHAR(20)    NOT NULL,                           -- reserve / confirm / release / adjust
    quantity    INTEGER        NOT NULL,                           -- 操作数量（正数）/ Operation quantity (positive)
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS order_service.stock_operations;
DROP TABLE IF EXISTS order_service.payment_records;
DROP TABLE IF EXISTS order_service.order_addresses;
DROP TABLE IF EXISTS order_service.order_items;
DROP TABLE IF EXISTS order_service.orders;
