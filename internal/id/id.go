package id

import (
	"fmt"
	"math/rand/v2"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// GenerateID 生成 21 字符 nanoid 作为主键。
// 为什么用 nanoid 而不是 UUID？nanoid 更短（21 vs 36 字符）、URL 安全、无需编码，
// 碰撞概率：每秒 1000 个 ID 需要约 149 亿年才可能碰撞一次。
func GenerateID() (string, error) {
	return nanoid.New()
}

// MustGenerateID 在出错时直接 panic。
// 只在程序启动阶段使用——如果启动时连 ID 都生成不了，继续运行没有意义。
// 请求处理中必须用 GenerateID() 并处理 error，绝不能 panic。
func MustGenerateID() string {
	return nanoid.Must()
}

// GenerateOrderNo 生成格式：时间戳(14位) + 随机数(8位) = 22 字符。
// 时间戳在前让订单号天然有序，方便按时间排序和分表；随机后缀防止同一秒内碰撞。
// Go 的时间格式用参考时间 "2006-01-02 15:04:05"（Mon Jan 2 15:04:05 2006），不是 YYYY-MM-DD。
func GenerateOrderNo() string {
	ts := time.Now().Format("20060102150405")
	suffix := fmt.Sprintf("%08d", rand.IntN(100000000))
	return ts + suffix
}
