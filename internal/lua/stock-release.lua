-- stock-release.lua: 单 SKU 库存释放（订单取消/超时时归还库存）
-- stock-release.lua: Single SKU stock release (return stock when order is cancelled/expired)
--
-- KEYS[1] = stock:{skuId}   -- Redis 中该 SKU 的库存 key / Stock key for this SKU in Redis
-- ARGV[1] = quantity         -- 要释放的数量 / Quantity to release
--
-- 返回值 / Return values:
--  1  = 释放成功 / Release successful
-- -1  = key 不存在 / Key doesn't exist

local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then return -1 end
redis.call('INCRBY', KEYS[1], ARGV[1])
return 1
