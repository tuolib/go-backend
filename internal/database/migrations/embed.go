package migrations

import "embed"

// FS 包含所有 SQL 迁移文件，通过 go:embed 嵌入到二进制中。
// FS contains all SQL migration files, embedded into the binary via go:embed.
//
// 这样部署时只需要一个二进制文件，不需要额外携带 .sql 文件。
// This means only a single binary is needed for deployment — no extra .sql files required.

//go:embed *.sql
var FS embed.FS
