-- stock-release-multi.lua: 多 SKU 原子释放
-- stock-release-multi.lua: Multi-SKU atomic stock release
--
-- KEYS = [stock:sku1, stock:sku2, ...]  -- 多个 SKU 的库存 key / Stock keys for multiple SKUs
-- ARGV = [qty1, qty2, ...]              -- 对应的释放数量 / Corresponding release quantities
--
-- 返回值 / Return values:
--  0  = 全部释放成功 / All releases successful
-- -i  = 第 i 个 SKU key 不存在 / The i-th SKU key doesn't exist

-- Phase 1: 检查所有 key 存在
-- Phase 1: Verify all keys exist
for i = 1, #KEYS do
    local stock = tonumber(redis.call('GET', KEYS[i]))
    if stock == nil then
        return -i
    end
end

-- Phase 2: 全部释放
-- Phase 2: Release all
for i = 1, #KEYS do
    redis.call('INCRBY', KEYS[i], ARGV[i])
end

return 0
