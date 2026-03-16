package lua

import (
	"context"
	_ "embed" // embed 包需要 blank import 才能使用 //go:embed 指令 / embed package requires blank import to use //go:embed directive

	"github.com/redis/go-redis/v9"
)

// go:embed 将 Lua 脚本文件内容在编译时嵌入到 Go 二进制中。
// go:embed embeds Lua script file contents into the Go binary at compile time.
//
// 好处：部署时只需一个二进制文件，不需要额外携带 .lua 文件。
// Benefit: only a single binary is needed for deployment — no extra .lua files required.

//go:embed stock-deduct.lua
var stockDeductScript string

//go:embed stock-deduct-multi.lua
var stockDeductMultiScript string

//go:embed stock-release.lua
var stockReleaseScript string

//go:embed stock-release-multi.lua
var stockReleaseMultiScript string

// StockScripts 封装所有库存相关的 Lua 脚本，服务启动时加载到 Redis（获取 SHA），
// 后续调用使用 EVALSHA 而不是 EVAL，减少网络带宽（只传 SHA 不传脚本内容）。
// StockScripts wraps all stock-related Lua scripts. Scripts are loaded into Redis at startup (getting their SHA),
// and subsequent calls use EVALSHA instead of EVAL — reduces network bandwidth (sends SHA, not script content).
type StockScripts struct {
	deductSHA       string // 单 SKU 扣减脚本的 SHA / SHA for single-SKU deduction script
	deductMultiSHA  string // 多 SKU 扣减脚本的 SHA / SHA for multi-SKU deduction script
	releaseSHA      string // 单 SKU 释放脚本的 SHA / SHA for single-SKU release script
	releaseMultiSHA string // 多 SKU 释放脚本的 SHA / SHA for multi-SKU release script
}

// LoadStockScripts 将 4 个 Lua 脚本加载到 Redis 中，返回包含 SHA 的 StockScripts。
// LoadStockScripts loads all 4 Lua scripts into Redis and returns a StockScripts with their SHAs.
//
// SCRIPT LOAD 命令：Redis 缓存脚本并返回 SHA1 哈希值，之后用 EVALSHA 调用。
// SCRIPT LOAD command: Redis caches the script and returns its SHA1 hash for later EVALSHA calls.
func LoadStockScripts(ctx context.Context, rdb *redis.Client) (*StockScripts, error) {
	scripts := &StockScripts{}
	var err error

	scripts.deductSHA, err = rdb.ScriptLoad(ctx, stockDeductScript).Result()
	if err != nil {
		return nil, err
	}

	scripts.deductMultiSHA, err = rdb.ScriptLoad(ctx, stockDeductMultiScript).Result()
	if err != nil {
		return nil, err
	}

	scripts.releaseSHA, err = rdb.ScriptLoad(ctx, stockReleaseScript).Result()
	if err != nil {
		return nil, err
	}

	scripts.releaseMultiSHA, err = rdb.ScriptLoad(ctx, stockReleaseMultiScript).Result()
	if err != nil {
		return nil, err
	}

	return scripts, nil
}

// StockDeductResult 库存扣减结果常量。
// StockDeductResult stock deduction result constants.
const (
	StockDeductSuccess      = 1  // 扣减成功 / Deduction successful
	StockDeductInsufficient = 0  // 库存不足 / Insufficient stock
	StockDeductKeyMissing   = -1 // Key 不存在 / Key doesn't exist
)

// Deduct 扣减单个 SKU 的库存。
// Deduct deducts stock for a single SKU.
func (s *StockScripts) Deduct(ctx context.Context, rdb *redis.Client, skuID string, quantity int) (int, error) {
	// EVALSHA 用 SHA 调用已缓存的脚本，比 EVAL 省带宽。
	// EVALSHA calls a cached script by SHA — saves bandwidth compared to EVAL.
	result, err := rdb.EvalSha(ctx, s.deductSHA, []string{"stock:" + skuID}, quantity).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// DeductMulti 原子扣减多个 SKU 的库存（下单时使用）。
// DeductMulti atomically deducts stock for multiple SKUs (used when placing an order).
//
// items 格式：[]StockItem{{SkuID: "xxx", Quantity: 2}, ...}
// items format: []StockItem{{SkuID: "xxx", Quantity: 2}, ...}
//
// 返回 0 表示全部成功，正数 i 表示第 i 个 SKU 库存不足，负数 -i 表示第 i 个 key 不存在。
// Returns 0 for all success, positive i means i-th SKU insufficient, negative -i means i-th key missing.
func (s *StockScripts) DeductMulti(ctx context.Context, rdb *redis.Client, items []StockItem) (int, error) {
	keys := make([]string, len(items))
	args := make([]any, len(items))
	for i, item := range items {
		keys[i] = "stock:" + item.SkuID
		args[i] = item.Quantity
	}

	result, err := rdb.EvalSha(ctx, s.deductMultiSHA, keys, args...).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Release 释放单个 SKU 的库存（取消订单时使用）。
// Release releases stock for a single SKU (used when cancelling an order).
func (s *StockScripts) Release(ctx context.Context, rdb *redis.Client, skuID string, quantity int) (int, error) {
	result, err := rdb.EvalSha(ctx, s.releaseSHA, []string{"stock:" + skuID}, quantity).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// ReleaseMulti 原子释放多个 SKU 的库存。
// ReleaseMulti atomically releases stock for multiple SKUs.
func (s *StockScripts) ReleaseMulti(ctx context.Context, rdb *redis.Client, items []StockItem) (int, error) {
	keys := make([]string, len(items))
	args := make([]any, len(items))
	for i, item := range items {
		keys[i] = "stock:" + item.SkuID
		args[i] = item.Quantity
	}

	result, err := rdb.EvalSha(ctx, s.releaseMultiSHA, keys, args...).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// StockItem 库存操作项，包含 SKU ID 和数量。
// StockItem represents a stock operation item with SKU ID and quantity.
type StockItem struct {
	SkuID    string // SKU 唯一标识 / SKU unique identifier
	Quantity int    // 操作数量 / Operation quantity
}
