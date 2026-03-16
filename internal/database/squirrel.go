package database

import (
	sq "github.com/Masterminds/squirrel"
)

// Psql 是 PostgreSQL 风格的 SQL 构建器，使用 $1, $2 占位符（不是 MySQL 的 ?）。
// Psql is a PostgreSQL-flavored SQL builder using $1, $2 placeholders (not MySQL's ?).
//
// 为什么需要 squirrel？sqlc 只能处理固定 SQL（编译时已知的查询），
// 但商品搜索需要根据用户输入动态组合 WHERE 条件（关键词、分类、价格范围等）。
// squirrel 在运行时构建 SQL，同时保证参数化防 SQL 注入。
// Why squirrel? sqlc only handles fixed SQL (queries known at compile time),
// but product search needs dynamically composed WHERE clauses (keyword, category, price range, etc.).
// Squirrel builds SQL at runtime while ensuring parameterized queries to prevent SQL injection.
var Psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
