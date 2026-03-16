package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"go-backend/internal/database/migrations"
)

// migrationsFS 从嵌入的迁移文件中获取文件系统接口。
// migrationsFS gets the filesystem interface from embedded migration files.
var migrationsFS fs.FS = migrations.FS

func main() {
	// 命令行参数定义 / Command-line flag definitions
	direction := flag.String("direction", "up", "migration direction: up or down")
	dbURL := flag.String("db", "", "database URL (or set DATABASE_URL env)")
	flag.Parse()

	// 优先用命令行参数，其次用环境变量。
	// Prefer CLI flag, fall back to environment variable.
	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		fmt.Fprintln(os.Stderr, "error: database URL required (use -db flag or DATABASE_URL env)")
		os.Exit(1)
	}

	slog.Info("migrate tool starting", "direction", *direction)

	// pgx.ParseConfig 解析连接字符串为配置对象。
	// pgx.ParseConfig parses the connection string into a config object.
	connConfig, err := pgx.ParseConfig(connStr)
	if err != nil {
		slog.Error("parse database URL failed", "error", err)
		os.Exit(1)
	}

	// stdlib.RegisterConnConfig 将 pgx 配置注册到标准 database/sql 驱动中，返回一个 DSN。
	// stdlib.RegisterConnConfig registers the pgx config with the standard database/sql driver, returning a DSN.
	//
	// goose 要求标准 *sql.DB 接口，但底层仍然使用 pgx 驱动。
	// Goose requires *sql.DB, but the underlying driver is still pgx.
	dsn := stdlib.RegisterConnConfig(connConfig)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 测试连接 / Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	// 设置 goose 使用嵌入的迁移文件和 PostgreSQL 方言。
	// Configure goose to use the embedded migration files and PostgreSQL dialect.
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("set dialect failed", "error", err)
		os.Exit(1)
	}

	// 执行迁移 / Execute migration
	switch *direction {
	case "up":
		// Up 执行所有未应用的迁移。
		// Up applies all pending migrations.
		if err := goose.UpContext(ctx, db, "."); err != nil {
			slog.Error("migration up failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migration up completed")
	case "down":
		// Down 回滚最近一次迁移（每次只回滚一个，安全起见）。
		// Down rolls back the most recent migration (one at a time for safety).
		if err := goose.DownContext(ctx, db, "."); err != nil {
			slog.Error("migration down failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migration down completed")
	default:
		fmt.Fprintf(os.Stderr, "unknown direction: %s (use 'up' or 'down')\n", *direction)
		os.Exit(1)
	}
}
