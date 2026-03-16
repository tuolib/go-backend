package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	// flag.String 定义命令行参数：参数名 "direction"，默认值 "up"，用法说明。
	// flag.String defines a CLI flag: name "direction", default "up", usage description.
	//
	// 返回 *string 指针，必须用 *direction 解引用才能拿到值。
	// Returns a *string pointer — must dereference with *direction to get the value.
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse() // 解析命令行参数 / Parse command-line arguments

	slog.Info("migrate tool", "direction", *direction)

	// Phase 0 占位：实际迁移逻辑在 Phase 1 添加。
	// Phase 0 placeholder — actual migration logic will be added in Phase 1.
	fmt.Fprintf(os.Stderr, "migrate %s: no migrations configured yet\n", *direction)
}
