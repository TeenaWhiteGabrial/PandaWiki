# PandaWiki Backend 项目分析

> 生成时间：2026-06-12

## 1. 项目概述

PandaWiki 是一个智能知识库平台，后端使用 Go 语言开发，模块路径为 `github.com/chaitin/panda-wiki`，Go 版本 1.24.3。项目支持 AI 对话、多渠道 Bot 接入、多种知识源爬取、企业级认证等功能。

## 2. 项目结构

```
backend/
├── api/                    # 请求/响应类型定义（按领域划分）
│   ├── auth/               # 认证 API 类型
│   ├── conversation/       # 对话 API 类型
│   ├── crawler/            # 爬虫 API 类型（Notion/Siyuan/Confluence 等）
│   ├── kb/                 # 知识库 API 类型
│   ├── mcp/                # MCP API 类型
│   ├── nav/                # 导航 API 类型
│   ├── node/               # 节点 API 类型
│   ├── openapi/            # OpenAPI 类型
│   ├── share/              # 分享/公开 API 类型
│   ├── stat/               # 统计 API 类型
│   ├── token/              # API Token 类型
│   ├── user/               # 用户 API 类型
│   └── enterprise/         # 企业功能类型
├── cmd/                    # 可执行入口
│   ├── api/                # HTTP API 服务（main.go + wire.go + wire_gen.go）
│   ├── consumer/           # NATS 消息消费者
│   └── migrate/            # 数据迁移运行器
├── config/                 # 配置（Viper，YAML + 环境变量覆盖）
├── domain/                 # 核心领域模型（37+ 文件）
├── handler/                # HTTP 处理器
│   ├── base.go             # BaseHandler，通用响应辅助方法
│   ├── mq/                 # 消息队列处理器
│   ├── v1/                 # 管理/认证 API 处理器
│   └── share/              # 公开/分享访问 API 处理器
├── middleware/              # HTTP 中间件（认证、只读、会话、验证码）
├── migration/              # Go 数据迁移函数（5 个命名迁移）
├── mq/                     # 消息队列抽象
│   └── nats/               # NATS 实现
├── pkg/                    # 外部服务集成包
│   ├── bot/                # Bot 集成（钉钉/飞书/Discord/微信/企微）
│   ├── captcha/            # 验证码
│   ├── cas/                # CAS 认证
│   ├── dingtalk/           # 钉钉 SDK
│   ├── feishu/             # 飞书 SDK
│   ├── ldap/               # LDAP 认证
│   ├── oauth/              # OAuth2 认证
│   ├── ratelimit/          # 限流
│   └── wecom/              # 企微 SDK
├── repo/                   # 仓储层（持久化）
│   └── pg/                 # PostgreSQL 实现（15+ 仓储）
├── server/                 # 服务器设置
│   └── http/               # Echo HTTP 服务器配置
├── store/                  # 基础设施/存储层
│   ├── pg/                 # GORM 数据库 + SQL 迁移（42 个迁移文件）
│   ├── rag/                # RAG/向量搜索抽象
│   └── s3/                 # MinIO/S3 对象存储
├── telemetry/              # 使用遥测上报
├── usecase/                # 业务逻辑层（20+ 用例）
├── Dockerfile.api          # API + 迁移多阶段构建
├── Dockerfile.consumer     # Consumer 多阶段构建
├── Makefile                # 构建、生成、lint、迁移、Docker 目标
├── go.mod
└── go.sum
```

## 3. 三个可执行二进制

| 二进制 | 入口 | 职责 |
|--------|------|------|
| `cmd/api` | `main.go` + Wire | HTTP API 服务器，监听 8000 端口（默认），启动时自动执行 SQL 迁移 |
| `cmd/consumer` | `main.go` + Wire | NATS 消息消费者，处理 RAG 文档更新等异步任务 |
| `cmd/migrate` | `main.go` + Wire | 执行 Go 数据迁移（区别于 SQL schema 迁移） |

## 4. 分层架构

项目遵循严格的分层架构，依赖单向流动：

```
cmd/（入口 + Wire DI）
  │
  ▼
handler/（HTTP 逻辑：请求解析、响应格式化）
  │
  ▼
usecase/（业务逻辑，20+ 用例）
  │
  ▼
repo/（持久化接口实现，15+ 仓储）
  │
  ▼
store/（基础设施：数据库连接、外部服务客户端）
```

**关键规则：**
- 业务逻辑放在 `usecase/`，禁止放入 handler
- HTTP 相关逻辑留在 `handler/`
- 持久化和外部数据访问在 `repo/` 和 `store/`
- 共享请求/响应模型在 `domain/` 和 `api/`

## 5. 核心依赖

| 类别 | 依赖 | 说明 |
|------|------|------|
| Web 框架 | `labstack/echo/v4` | HTTP 框架 |
| ORM | `gorm.io/gorm` + `postgres` driver | 数据库 ORM |
| Schema 迁移 | `golang-migrate/migrate/v4` | SQL 迁移 |
| 依赖注入 | `google/wire` | 编译时 DI |
| 消息队列 | `nats-io/nats.go` | NATS 客户端 |
| 对象存储 | `minio/minio-go/v7` | MinIO/S3 |
| 缓存/会话 | `redis/go-redis/v9` + `boj/redistore` | Redis |
| AI/LLM | `cloudwego/eino` + 扩展 | LLM 编排框架，支持 OpenAI/DeepSeek/Ollama/Gemini |
| JWT | `golang-jwt/jwt/v5` | JWT 认证 |
| MCP | `mark3labs/mcp-go` | Model Context Protocol |
| 可观测性 | `sentry-go` + `opentelemetry` | 错误追踪 + APM |
| Bot 集成 | 钉钉/飞书/Discord/微信/企微 SDK | 多渠道 Bot |
| 企业认证 | `go-ldap/ldap/v3` + `oauth2` | LDAP/CAS/OAuth2 |
| 配置 | `spf13/viper` | YAML + 环境变量 |
| API 文档 | `swaggo/swag` + `echo-swagger` | Swagger 生成 |

## 6. API 路由

### 6.1 管理 API（`/api/v1/`，需认证）

| 模块 | 路径前缀 | 主要功能 |
|------|----------|----------|
| 用户 | `/api/v1/user` | 登录 |
| 知识库 | `/api/v1/knowledge_base` | CRUD、用户管理、发布 |
| 节点 | `/api/v1/node` | 列表/详情/创建/移动/摘要(SSE)/批量操作 |
| 应用 | `/api/v1/app` | 详情/更新/删除 |
| 对话 | `/api/v1/conversation` | 列表/详情/消息 |
| MCP | `/api/v1/mcp` | 工具管理 |
| API Token | `/api/v1/token` | Token CRUD |
| 文件 | `/api/v1/file` | 上传/下载 |
| 模型 | `/api/v1/model` | 模型配置 |
| 爬虫 | `/api/v1/crawler` | Notion/Siyuan/Confluence/Yuque 等爬取管理 |
| 统计 | `/api/v1/stat` | 统计数据 |
| 评论 | `/api/v1/comment` | 评论管理 |
| 认证 | `/api/v1/auth` | 认证管理 |
| 导航 | `/api/v1/nav` | 导航管理 |
| 企业 | `/api/v1/enterprise` | 认证组/LDAP/CAS/OAuth/权限 |

### 6.2 分享/公开 API（`share/v1/`，需 ShareAuth）

| 模块 | 路径前缀 |
|------|----------|
| 节点 | `share/v1/node` |
| 导航 | `share/v1/nav` |
| 应用 | `share/v1/app` |
| 对话 | `share/v1/chat`、`share/v1/conversation` |
| Sitemap | `share/v1/sitemap` |
| 统计 | `share/v1/stat` |
| 评论 | `share/v1/comment` |
| 认证 | `share/v1/auth` |
| 微信 | `share/v1/wechat` |
| 验证码 | `share/v1/captcha` |
| OpenAPI | `share/v1/openapi` |

## 7. 数据库与存储

### 7.1 PostgreSQL + GORM
- 连接配置在 `store/pg/pg.go`
- 自定义 GORM 日志：warn 级别，200ms 慢查询阈值，忽略 record-not-found
- 自动创建 `raglite` 数据库

### 7.2 Schema 迁移（golang-migrate）
- 42 个 SQL 迁移文件（`store/pg/migration/000001` ~ `000042`）
- API 启动时自动执行
- 通过 `make migrate_sql` 创建新迁移

### 7.3 Go 数据迁移
- 5 个命名迁移函数（`migration/fns/`）
- 通过独立的 `cmd/migrate` 二进制执行
- 用于数据转换，区别于 schema 迁移

### 7.4 MinIO/S3 对象存储
- 启动时自动创建 bucket
- `static-file/*` 前缀设为公开读
- `SignURL()` 生成私有对象的预签名 URL

### 7.5 Redis
- 通用缓存（`go-redis/v9`）
- 会话存储（`redistore`，30 天有效期，SameSite=Lax，HttpOnly）

## 8. 外部服务集成

### 8.1 AI/LLM 服务
- OpenAI、DeepSeek、Ollama、Gemini（通过 eino 框架统一接入）
- 长亭自研 RAG 服务（向量搜索、嵌入）

### 8.2 Bot 平台
- 钉钉 Bot、飞书/Lark Bot、微信公众号/服务号、企微 Bot、Discord Bot

### 8.3 知识源爬虫
- Notion、思源、Confluence、语雀、Mindoc、Wiki.js、EPUB、飞书文档

### 8.4 企业认证
- LDAP、CAS、OAuth2

### 8.5 基础设施
- NATS（异步消息队列）、PostgreSQL、Redis、MinIO/S3、Caddy（反向代理）

## 9. 认证体系

### 9.1 双重认证（管理 API）
- **JWT Bearer Token**（含 `.` 的 token）—— 标准 JWT 认证
- **API Token**（不含 `.` 的 token）—— 编程式访问
- `JWTMiddleware.Authorize` 先尝试 JWT，失败则回退到 API Token

### 9.2 分享认证
- `ShareAuthMiddleware` 从 `X-KB-ID` Header 解析知识库身份
- 多层检查：知识库可访问性 → 企业认证 → 简单认证 → 会话认证

### 9.3 权限系统
- 知识库级权限：`FullControl`、`DocManage`、`DataOperate`、`UserKBPermissionNotNull`
- `ValidateKBUserPerm` 中间件按路由执行权限级别校验

## 10. 中间件链（顺序）

1. **Sentry 中间件** —— 错误追踪上下文
2. **OpenTelemetry 中间件** —— 分布式追踪
3. **请求日志** —— 结构化 slog
4. **ReadOnly 中间件** —— 当 `READONLY` 环境变量设置时，阻止非 GET 请求到 `/api/v1/*`
5. **Session 中间件** —— Redis 会话

## 11. Wire 依赖注入

三个二进制均使用 Google Wire 编译时 DI：

**API 服务器（`cmd/api/wire.go`）：**
```
Config → Logger → Telemetry → HTTPServer → V1Handlers(16) + ShareHandlers(13)
  └── 依赖链：middleware → usecase(20+) → repo(15+) → store(pg/mq/rag/s3/ipdb)
```

**Consumer（`cmd/consumer/wire.go`）：**
```
Config → Logger → MQConsumer → MQHandlers(RAG/DocUpdate/StatCron)
  └── 依赖：pg + rag + mq + s3 + LLM/Stat/Node/Model usecases
```

**Migrate（`cmd/migrate/wire.go`）：**
```
Config → Logger → MigrationManager
```

## 12. 关键设计模式

### 12.1 响应格式
```json
{
  "message": "...",
  "success": true/false,
  "data": {...},
  "code": 0
}
```
错误码：`0`=成功，`40003`=权限拒绝，`40004`=未找到，`50001`=内部错误

### 12.2 错误处理
- `BaseHandler.NewResponseWithError()` —— 日志记录含 trace ID，返回用户友好消息
- Domain 哨兵错误：`ErrModelNotConfigured`、`ErrPermissionDenied` 等
- 错误包装使用 `fmt.Errorf("...: %w", err)`，哨兵检查使用 `errors.Is`
- 用户面向中文错误消息

### 12.3 SSE 流式推送
- 用于实时 AI 响应（节点摘要、对话）
- Content-Type: `text/event-stream`，分块传输编码
- 自定义 `writeSSEEvent` 辅助函数

### 12.4 多应用类型
Web、Widget、钉钉 Bot、飞书 Bot、微信 Bot、Discord Bot、企微 AI Bot、Lark Bot、OpenAI API、微信公众号、微信服务号

### 12.5 遥测
- AES 加密使用数据上报到长亭云
- 事件类型：安装（首次运行）、心跳（每小时）、日常数据报告（23:30-23:58）
- 上报内容：版本、机器 ID、知识库数量、模型配置、文档数量、对话统计等聚合指标

## 13. 配置

配置使用 Viper，YAML 文件为基础，环境变量覆盖特定字段：

| 环境变量 | 说明 |
|----------|------|
| `POSTGRES_PASSWORD` | PostgreSQL 密码 |
| `NATS_PASSWORD` | NATS 密码 |
| `REDIS_PASSWORD` | Redis 密码 |
| `JWT_SECRET` | JWT 密钥 |
| `S3_SECRET_KEY` | S3 密钥 |
| `ADMIN_PASSWORD` | 初始管理员密码 |
| `SUBNET_PREFIX` | 网络子网前缀 |
| `PG_DSN` | PostgreSQL DSN |
| `MQ_NATS_SERVER` | NATS 服务器地址 |
| `RAG_CT_RAG_BASE_URL` | RAG 服务地址 |
| `REDIS_ADDR` | Redis 地址 |
| `S3_ENDPOINT` | S3 端点 |
| `SENTRY_ENABLED` | Sentry 开关 |
| `SENTRY_DSN` | Sentry DSN |
| `CADDY_API` | Caddy 反向代理 API |
| `LOG_LEVEL` | 日志级别 |
| `READONLY` | 只读模式 |

## 14. 构建与开发命令

| 操作 | 命令 |
|------|------|
| 安装依赖 | `cd backend && go mod tidy` |
| 构建全部 | `cd backend && go build ./...` |
| 运行全部测试 | `cd backend && go test ./...` |
| 生成 Swagger/Wire | `cd backend && make generate` |
| Lint（提交前必做） | `cd backend && make lint` |
| 创建 SQL 迁移 | `cd backend && make migrate_sql` |
| Docker 构建 | `cd backend && make image` |
