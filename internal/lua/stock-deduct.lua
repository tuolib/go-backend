-- stock-deduct.lua: 单 SKU 库存扣减
-- stock-deduct.lua: Single SKU stock deduction
--
-- KEYS[1] = stock:{skuId}   -- Redis 中该 SKU 的库存 key / Stock key for this SKU in Redis
-- ARGV[1] = quantity         -- 要扣减的数量 / Quantity to deduct
--
-- 返回值 / Return values:
--  1  = 扣减成功 / Deduction successful
--  0  = 库存不足 / Insufficient stock
-- -1  = key 不存在（SKU 未初始化）/ Key doesn't exist (SKU not initialized)

local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then return -1 end
if stock < tonumber(ARGV[1]) then return 0 end
redis.call('DECRBY', KEYS[1], ARGV[1])
return 1
