-- stock-deduct-multi.lua: 多 SKU 原子扣减（下单时一次性扣减所有商品的库存）
-- stock-deduct-multi.lua: Multi-SKU atomic deduction (deduct all items' stock at once when placing an order)
--
-- KEYS = [stock:sku1, stock:sku2, ...]  -- 多个 SKU 的库存 key / Stock keys for multiple SKUs
-- ARGV = [qty1, qty2, ...]              -- 对应的扣减数量 / Corresponding deduction quantities
--
-- 返回值 / Return values:
--  0  = 全部扣减成功 / All deductions successful
--  i  = 第 i 个 SKU 库存不足（1-based 索引）/ The i-th SKU has insufficient stock (1-based index)
-- -i  = 第 i 个 SKU key 不存在 / The i-th SKU key doesn't exist
--
-- 两阶段设计：先检查全部，再扣减全部。避免部分扣减成功后回滚。
-- Two-phase design: check all first, then deduct all. Avoids partial deduction requiring rollback.

-- Phase 1: 检查所有 SKU 的库存是否充足
-- Phase 1: Verify all SKUs have sufficient stock
for i = 1, #KEYS do
    local stock = tonumber(redis.call('GET', KEYS[i]))
    if stock == nil then
        return -i
    end
    if stock < tonumber(ARGV[i]) then
        return i
    end
end

-- Phase 2: 全部检查通过后，原子扣减所有库存
-- Phase 2: After all checks pass, atomically deduct all stock
for i = 1, #KEYS do
    redis.call('DECRBY', KEYS[i], ARGV[i])
end

return 0
