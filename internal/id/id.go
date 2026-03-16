package id

import (
	"fmt"
	"math/rand/v2"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// GenerateID 生成 21 字符 nanoid 作为主键。
// GenerateID generates a 21-character nanoid for use as a primary key.
//
// 为什么用 nanoid 而不是 UUID？nanoid 更短（21 vs 36 字符）、URL 安全、无需编码，
// 碰撞概率：每秒 1000 个 ID 需要约 149 亿年才可能碰撞一次。
// Why nanoid over UUID? Nanoid is shorter (21 vs 36 chars), URL-safe, and needs no encoding.
// Collision probability: at 1000 IDs/sec, it would take ~14.9 billion years for a collision.
func GenerateID() (string, error) {
	return nanoid.New()
}

// MustGenerateID 在出错时直接 panic。
// MustGenerateID panics on error.
//
// 只在程序启动阶段使用——如果启动时连 ID 都生成不了，继续运行没有意义。
// Only use during program startup — if we can't even generate IDs at startup, there's no point continuing.
//
// 请求处理中必须用 GenerateID() 并处理 error，绝不能 panic。
// During request handling, always use GenerateID() and handle the error — never panic.
func MustGenerateID() string {
	return nanoid.Must()
}

// GenerateOrderNo 生成订单号，格式：时间戳(14位) + 随机数(8位) = 22 字符。
// GenerateOrderNo generates an order number: timestamp (14 digits) + random (8 digits) = 22 chars.
//
// 时间戳在前让订单号天然有序，方便按时间排序和分表；随机后缀防止同一秒内碰撞。
// Timestamp-first makes order numbers naturally sortable for time-based queries and sharding; random suffix prevents same-second collisions.
//
// Go 的时间格式用参考时间 "2006-01-02 15:04:05"（Mon Jan 2 15:04:05 2006），不是 YYYY-MM-DD。
// Go's time format uses the reference time "2006-01-02 15:04:05" (Mon Jan 2 15:04:05 2006), NOT YYYY-MM-DD.
func GenerateOrderNo() string {
	ts := time.Now().Format("20060102150405")              // 格式化当前时间为 14 位字符串 / Format current time as 14-digit string
	suffix := fmt.Sprintf("%08d", rand.IntN(100000000))    // 生成 8 位随机数，%08d 保证补零到 8 位 / Generate 8-digit random number, %08d pads with leading zeros
	return ts + suffix
}
