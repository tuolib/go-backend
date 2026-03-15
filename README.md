# Go Backend — 企业级高并发电商平台

Go 重写版，支持**单体模式**和**微服务模式**双模式运行。

## 技术栈

Go 1.22+ / chi v5 / pgx v5 / sqlc + squirrel / go-redis v9 / golang-jwt v5 / Argon2id / Docker + Caddy

## 前置依赖

| 工具 | 最低版本 | 安装 |
|------|----------|------|
| Go | 1.22+ | `brew install go` |
| Docker & Compose | Docker 24+ | [Docker Desktop](https://www.docker.com/products/docker-desktop/) |
| golangci-lint | 1.55+ | `brew install golangci-lint`（可选，用于 lint） |

## 快速开始

### 1. 克隆 & 初始化

```bash
git clone <repo-url> && cd go-backend
cp .env.example .env     # 创建本地环境变量（已被 .gitignore 忽略）
go mod tidy              # 下载依赖
```

### 2. 启动基础设施（PostgreSQL + Redis）

```bash
docker compose up -d postgres redis
```

等待健康检查通过：

```bash
docker compose ps   # STATUS 列应显示 (healthy)
```

> Caddy 及应用容器在本地开发时不需要启动，monolith 模式直接运行即可。

### 3. 单体模式启动（推荐）

```bash
make dev
# 等同于: go run ./cmd/monolith
# 默认监听 :3000
```

验证服务正常：

```bash
curl -s -X POST http://localhost:3000/health | jq .
curl -s -X POST http://localhost:3000/api/v1/user/health | jq .
curl -s -X POST http://localhost:3000/api/v1/product/health | jq .
curl -s -X POST http://localhost:3000/api/v1/cart/health | jq .
curl -s -X POST http://localhost:3000/api/v1/order/health | jq .
```

### 4. 微服务模式启动（可选）

分别启动各服务，每个占独立终端：

```bash
make run-gateway   # :3000
make run-user      # :3001 (新终端)
make run-product   # :3002 (新终端)
make run-cart      # :3003 (新终端)
make run-order     # :3004 (新终端)
```

## 常用命令

### 构建

```bash
make build           # go build ./... 编译检查
make build-all       # 构建全部 7 个二进制到 bin/
make build-monolith  # 只构建单体
```

### 测试

```bash
make test              # 全量测试
make test-short        # 快速测试（跳过集成测试）
make test-race         # 竞态检测
make test-integration  # 集成测试（需要 Docker 中的 PG/Redis）
make test-coverage     # 生成覆盖率报告 → coverage.html
make bench             # 基准测试
```

### 代码质量

```bash
make lint       # golangci-lint 检查
make lint-fix   # 自动修复
make vet        # go vet
```

### 数据库迁移

```bash
make migrate-up       # 执行迁移
make migrate-down     # 回滚迁移
make migrate-create   # 创建新迁移文件（交互式输入名称）
```

### 代码生成

```bash
make sqlc       # 从 SQL 查询文件生成 Go 代码
make generate   # 运行所有代码生成
```

### Docker 全栈

```bash
make docker-up     # 构建并启动所有容器（微服务模式）
make docker-down   # 停止所有容器
make docker-logs   # 查看实时日志
make docker-build  # 仅构建镜像
```

### 运维 & 调试

```bash
make health     # 检查网关健康状态
make smoke      # 冒烟测试
make clean      # 清理构建产物
```

## 环境变量

所有变量定义在 `.env.example` 中，本地开发使用默认值即可。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `API_GATEWAY_PORT` | 3000 | 网关/单体端口 |
| `USER_SERVICE_PORT` | 3001 | 用户服务端口 |
| `PRODUCT_SERVICE_PORT` | 3002 | 商品服务端口 |
| `CART_SERVICE_PORT` | 3003 | 购物车服务端口 |
| `ORDER_SERVICE_PORT` | 3004 | 订单服务端口 |
| `DATABASE_URL` | `postgresql://postgres:postgres@localhost:5432/ecommerce?sslmode=disable` | PG 连接串 |
| `REDIS_URL` | `redis://localhost:6379` | Redis 连接串 |
| `APP_ENV` | development | 环境标识 |
| `LOG_LEVEL` | debug | 日志级别 |

## 项目结构

```
cmd/
├── monolith/    单体入口（make dev）
├── gateway/     API 网关
├── user/        用户服务
├── product/     商品服务
├── cart/        购物车服务
├── order/       订单服务
└── migrate/     迁移工具

internal/
├── apperr/      统一错误类型
├── handler/     AppHandler + Wrap 包装器
├── response/    JSON 响应工具
├── config/      koanf 配置加载
├── middleware/   共享中间件
├── auth/        JWT + Argon2
├── database/    PG/Redis 连接 + 迁移 + sqlc
├── user/        用户域（handler/service/repository/dto）
├── product/     商品域
├── cart/        购物车域
├── order/       订单域
└── gateway/     网关路由/代理
```

## 开发工作流

1. 启动基础设施：`docker compose up -d postgres redis`
2. 启动应用：`make dev`
3. 编写代码 → 保存 → 重新 `make dev`（或使用 `air` 等热重载工具）
4. 运行测试：`make test`
5. 检查代码：`make lint`
6. 构建验证：`make build`

## 文档

- [架构设计](docs/architecture.md) — 系统全景、分阶段路线图
- [API 参考](docs/api-reference.md) — 全部接口定义
