# PandaWiki 技术架构总览

PandaWiki 是一个 **AI 驱动的开源知识库系统**，由长亭科技开发，整体采用前后端分离 + 微服务协作的架构。

## 核心技术栈

| 层级 | 技术 |
|------|------|
| **后端** | Go 1.24 + Echo 框架 + GORM + Google Wire (DI) |
| **管理前端** | React 19 + Vite + Redux Toolkit |
| **用户前端** | Next.js 16 + React 19 |
| **前端共享** | pnpm monorepo，3 个内部包：icons / themes / ui |
| **数据库** | PostgreSQL 15 (主库) + Qdrant (向量库) |
| **缓存/会话** | Redis 7 |
| **消息队列** | NATS JetStream |
| **对象存储** | MinIO (S3 兼容) |
| **AI/RAG** | CloudWeGo Eino + 多 LLM (OpenAI/DeepSeek/Gemini/Ollama) + RAGLite |
| **反向代理** | Caddy |

---

## 后端进程

系统有 3 个后端进程：

1. **`panda-wiki-api`** — HTTP API 服务 (Echo)，处理所有前端请求
2. **`panda-wiki-consumer`** — NATS 消费者，处理异步任务（文档处理、RAG 索引等）
3. **`panda-wiki-migrate`** — 数据库迁移工具

## 后端分层架构

```
backend/
├── handler/       # HTTP 处理层 (Echo)，薄层，委托给 usecase
├── usecase/       # 业务逻辑层（核心）
├── repo/          # 数据访问 / 持久化层
├── domain/        # 领域模型、请求/响应结构体
├── api/           # API 定义
├── middleware/    # 认证、日志等中间件
├── mq/            # 消息队列 (NATS) 抽象
├── store/         # 存储层 (PostgreSQL 迁移在 store/pg/)
├── config/        # 配置解析 (Viper)
├── telemetry/     # OpenTelemetry 链路追踪 + 版本注入
├── apm/           # 应用性能监控
├── pkg/           # 共享工具
├── pro/           # 专业版/企业版功能
└── server/        # 服务初始化
```

### API 请求链路示例

以 `GET /api/v1/node/list?kb_id=xxx` 为例，追踪完整的三层调用：

```
HTTP 请求
  │
  ▼
Handler (handler/v1/node.go:140)
  ├── Bind 请求参数 → domain.GetNodeListReq
  ├── Validate 校验 (kb_id 必填)
  └── 调用 h.usecase.GetList(ctx, &req)
       │
       ▼
Usecase (usecase/node.go:90)
  ├── 调用 nodeRepo.GetList() 查数据库
  ├── 企业权限过滤（如果是管理员角色）
  ├── 补充发布者信息 (GetNodeReleasePublisherMap)
  └── 返回 []*domain.NodeListItemResp
       │
       ▼
Repo (repo/pg/node.go:117)
  ├── GORM 查询: SELECT ... FROM nodes
  ├── LEFT JOIN users (创建者 + 编辑者)
  ├── WHERE kb_id = ? AND 可选过滤
  └── JSONB 提取: meta->>'summary', meta->>'emoji'
       │
       ▼
PostgreSQL → 结果逐层返回 → Handler 包装为 PWResponse → JSON 响应
```

**统一响应格式** (`domain/response.go`)：

```json
{
  "success": true,
  "data": { ... },
  "message": "",
  "code": 0
}
```

---

## 整体数据流

```
          用户浏览器
         ┌─────────┐
         │  Caddy   │  (反向代理/SSL，注入 X-KB-ID)
         └────┬─────┘
        ┌─────┴──────┐
   ┌────┴───┐  ┌─────┴────┐
   │ Admin  │  │   App    │
   │(Vite)  │  │(Next.js) │
   └───┬────┘  └────┬─────┘
       └─────┬───────┘
        HTTP REST / SSE
        ┌────┴─────┐
        │  API 服务  │  (Echo, 业务逻辑在 usecase 层)
        └────┬─────┘
     ┌───────┼────────┐
  ┌──┴──┐ ┌──┴──┐ ┌──┴───┐
  │ PG  │ │Redis│ │MinIO │  (持久化/缓存/文件)
  └─────┘ └──┬──┘ └──────┘
            ┌──┴──┐
            │NATS │  (消息队列)
            └──┬──┘
        ┌──────┴──────┐
   ┌────┴────┐ ┌──────┴──────┐
   │Consumer │ │   RAGLite   │  (异步处理 / RAG)
   └─────────┘ └──────┬──────┘
                ┌──────┴──────┐
                │   Qdrant    │  (向量检索)
                └─────────────┘
```

### 关键流转

- **同步操作**：前端请求 → API → PostgreSQL / Redis / MinIO
- **异步任务**：重任务（文档处理、RAG）→ API 投递 NATS 消息 → Consumer 异步消费 → 调用 RAGLite → Qdrant 向量检索
- **文档爬取**：Crawler (anydoc) 爬取外部文档 → MinIO 存储 → NATS 通知 → Consumer 处理

---

## 前端结构

```
web/
├── admin/          # 管理后台 (React + Vite + Redux Toolkit)
├── app/            # 用户端 (Next.js 16)
└── packages/
    ├── icons/      # 共享图标 (@panda-wiki/icons)
    ├── themes/     # 共享主题 (@panda-wiki/themes)
    └── ui/         # 共享 UI 组件 (@panda-wiki/ui)
```

### Admin vs App 对比

| 方面 | Admin (管理后台) | App (用户端) |
|------|------------------|--------------|
| **面向用户** | 知识库管理员 | 普通读者/用户 |
| **核心功能** | 管理节点、编辑文档、配置设置、查看统计 | 浏览文档、搜索、AI 问答 |
| **框架** | React + Vite (纯 SPA) | Next.js 16 App Router (SSR) |
| **路由** | react-router-dom (客户端) | 文件系统 App Router (服务端+客户端) |
| **状态管理** | Redux Toolkit (客户端) | Redux Toolkit (服务端初始化) |
| **API 代理** | `/api` + `/static-file` + `/share` | `/share/v1` + `/static-file` |
| **富文本** | @ctzhian/tiptap + Yjs 协同编辑 | markdown-it + react-markdown (只读) |
| **额外能力** | dnd-kit 拖拽、echarts 图表、协同编辑 | Mermaid 图表、换肤、嵌入式组件、Sentry |

### Admin 路由

```
/login, /401          ← 不需要登录
/                     ← 文档布局管理
/setting              ← 系统设置
/release              ← 版本管理
/stat                 ← 统计数据
/conversation         ← AI 对话管理
/doc/editor/:id       ← 文档编辑器（Yjs 协同）
```

### App 路由

```
/auth/login           ← 登录
/home                 ← 首页
/welcome              ← 欢迎页
/node/[id]            ← 查看文档节点
/editor/[[...id]]     ← 编辑文档
/h5-chat              ← 移动端聊天
/widget               ← 嵌入式组件
/feedback             ← 反馈
```

### 共享内部包

| 包 | 作用 | 主要使用者 |
|---|---|---|
| `@panda-wiki/icons` | SVG 图标组件 | 两个应用 |
| `@panda-wiki/themes` | MUI 主题定义（多色系） | App（用户端换肤） |
| `@panda-wiki/ui` | 通用 UI 组件（轮播、页头、FAQ 等） | App（站点搭建模块） |

---

## NATS 异步流程

### 三条消息通道

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| `apps.panda-wiki.vector.task` | API / Usecase | Consumer (RAGMQHandler) | 向量库同步（增/删/摘要/权限） |
| `raglite.events.doc.update` | RAGLite 外部服务 | Consumer (RagDocUpdateHandler) | RAG 处理结果回调 |
| `anydoc.persistence.doc.task.export` | Anydoc 爬虫服务 | Consumer (Anydoc handler) | 文档爬取完成通知 |

NATS JetStream 自动创建两个 Stream：
- **`task`** — 包含 `apps.panda-wiki.summary.task` 和 `apps.panda-wiki.vector.task`，7 天保留，1GB 上限
- **`scraper`** — 包含 `apps.panda-wiki.scraper.>`，同上

### VectorTask 的 4 种 Action

| Action | 触发时机 | 做什么 |
|--------|----------|--------|
| `upsert` | 发布/更新节点 | 把文档内容推到 RAG 向量库 |
| `delete` | 删除节点 | 从向量库移除文档 |
| `summary` | AI 摘要 | 调 LLM 生成摘要，写回 DB |
| `update_group_ids` | 修改权限 | 更新向量库中的访问组 ID |

### 完整链路：删除节点

```
1. POST /api/v1/node/action  {action: "delete", ids: [...]}

2. Usecase: NodeAction()
   ├── 从 DB 删除节点 (同步)
   └── 发布 NATS 消息: NodeReleaseVectorRequest{Action:"delete"}

3. Consumer: RAGMQHandler 收到消息
   ├── 查出 KB 的 DatasetID
   ├── 调用 rag.DeleteRecords() 从向量库删除
   └── ACK 确认

4. RAGLite 处理完 → 发 RagDocUpdate 消息
   → Consumer 更新 nodes.rag_info → 后续查询可见
```

### 三种结果回传方式

| 方式 | 用在哪 | 机制 |
|------|--------|------|
| **写 DB** | 向量同步、RAG 回调 | Consumer 处理完直接写 PostgreSQL，API 后续查询读到 |
| **Go channel** | Anydoc 爬取 | `TaskWaitForCompletion()` 阻塞等待，爬完通知 |
| **SSE 直推** | AI 对话、流式摘要 | 和 NATS 无关，LLM 流式响应直接推给前端 |

### 关键认知

- API 只生产 `VectorTaskTopic` 消息，其余两个 topic 由外部服务生产
- Consumer 是被动消费者：收到消息 → 处理 → 写 DB → 完事，没有回复消息
- SSE 是独立路径，不走 NATS
- 最终一致性：API 先同步写 DB，再异步让 Consumer 更新向量库

---

## AI/RAG 链路

### 全流程概览

```
用户提问 "PandaWiki 怎么部署？"
  │
  ▼
Handler: ShareChatHandler.ChatMessage()
  ├── 设置 SSE 响应头 (text/event-stream)
  └── 调用 ChatUsecase.Chat() → 返回 <-chan SSEEvent
       │
       ▼
ChatUsecase.Chat() [goroutine]
  ├── 1. 验证应用 & 获取聊天模型
  ├── 2. 创建/验证对话（nonce 机制）
  ├── 3. 保存用户消息到 DB
  ├── 4. 敏感词检查（DFA 算法）
  ├── 5. ★ RAG 检索上下文
  ├── 6. 发送引用来源（chunk_result 事件）
  ├── 7. ★ 调 LLM 流式生成
  ├── 8. 敏感词过滤输出
  ├── 9. 保存助手回复到 DB
  └── 10. 发送 done 事件
       │
       ▼
Handler 逐事件写入 SSE: "data: {json}\n\n" → Flush
```

### RAG 检索细节

`LLMUsecase.BuildConversationMessageWithRAG()` 是核心：

```
用户问题 + 对话历史
  │
  ▼
1. 从 DB 加载对话历史 → Eino Message 格式
  │
  ▼
2. 拼系统提示词（自定义 or 默认模板）
  │
  ▼
3. 向量检索: GetRankNodes()
  ├── CTRAG.QueryRecords() → raglite SDK → 外部 RAG 服务
  │   └── Qdrant 向量搜索 Top-10 相似片段（阈值 0.2，支持权限过滤）
  ├── chunk 映射回节点（标题、摘要、路径）
  └── 企业权限过滤
  │
  ▼
4. 格式化文档为 XML 块:
   <document>
   ID: xxx
   标题: 部署指南
   URL: /node/xxx
   内容: 文档片段...
   </document>
  │
  ▼
5. 组装消息: [系统提示] + [对话历史] + [用户问题+检索文档]
```

### LLM 调用链路

```
domain.Model (Provider/APIKey/BaseURL)
  │ ToModelkitModel()
  ▼
ModelKit 统一格式
  │ modelkit.GetChatModel()
  ▼
Eino ChatModel 接口
  │ chatModel.Stream(ctx, messages)
  ▼
逐 token 返回 → 敏感词过滤 → SSE 推送
```

### SSE 事件流

前端按顺序收到以下事件：

| 事件类型 | 内容 | 时机 |
|----------|------|------|
| `conversation_id` | 对话 ID | 新对话创建 |
| `nonce` | 一次性令牌 | 每轮对话 |
| `message_id` | 消息 ID | 助手回复前 |
| `chunk_result` | 引用的文档节点 | RAG 检索完成后 |
| `data` | LLM token 片段 | 流式生成中 |
| `error` | 错误信息 | 出错 |
| `done` | 空 | 流结束 |

### 文档索引（离线，与 NATS 流程串联）

```
节点发布/更新 → NATS (VectorTask, action=upsert)
  → Consumer 取出内容 → HTML 转 Markdown → 上传 RAG 服务
  → RAG 服务自动分块、embedding、存入 Qdrant
  → 完成后发 RagDocUpdate → 更新 nodes.rag_info
```

写入和查询是两条独立路径：写入走 NATS 异步，查询走 HTTP 同步 + SSE。

### 聊天入口

| 路由 | 用途 |
|------|------|
| `POST /share/v1/chat/message` | Web 聊天 |
| `POST /share/v1/chat/widget` | 嵌入式组件聊天 |
| `POST /share/v1/chat/completions` | OpenAI 兼容 API（支持流式/非流式） |
| `POST /share/v1/chat/search` | 仅搜索（不调 LLM） |

---

## 基础设施

### 6 个中间件的职责

| 中间件 | 存什么 | 为什么需要 |
|--------|--------|------------|
| **PostgreSQL** | 用户、节点、知识库、对话、模型配置、统计…所有结构化数据 | 关系型主数据库，Single Source of Truth |
| **Redis** | Session、KB 缓存、登录限速、分布式锁、企微对话映射 | 需要 TTL 和低延迟的临时状态 |
| **MinIO** | 图片、文档、EPUB 等上传文件 (bucket: `static-file`) | 二进制文件不适合存 PG，需要 S3 接口 + 公开 URL |
| **NATS** | 向量同步任务、爬虫任务、RAG 回调事件 | 解耦异步任务，JetStream 保证持久化 |
| **Qdrant** | 文档片段的向量 embedding | 高效向量相似度搜索，PG 做不了 |
| **Caddy** | 不存数据，动态反向代理 | 多租户：每个知识库独立域名/端口/SSL |

### Redis 的 5 种用途

```
Redis
├── Session 存储    ← 管理后台登录态（30 天有效）
├── KB 缓存         ← 按 KB ID 缓存知识库配置，减少 PG 查询
├── 登录限速        ← 失败 5/10/15 次锁定 1分/15分/1小时
├── 分布式锁        ← SetNX 10 秒 TTL，防并发操作
└── 企微对话映射    ← 消息 ID → 对话 ID，15 分钟 TTL
```

### Caddy 多租户路由

每个知识库可绑定不同域名/端口，Caddy 由 API 动态配置路由：

```
用户访问 kb1.example.com:443
  │
  ▼
Caddy 读取路由规则（API 动态配置）
  ├── 设置 X-KB-ID: "kb1"  ← 注入知识库 ID
  ├── /share/v1/chat/*  → API 服务（SSE，10 分钟超时）
  ├── /static-file/*    → MinIO（非 PDF 加 Content-Disposition）
  └── 其他路径           → App 前端（Next.js）
```

API 拿到 `X-KB-ID` 即知请求属于哪个知识库，无需解析域名。

### 服务启动顺序

```
第一层（无依赖）:  PostgreSQL, Redis, NATS, MinIO, Qdrant
       │
第二层（依赖第一层）:  Crawler → NATS + PG + MinIO
                      RAGLite  → PG + Qdrant + MinIO + NATS
       │
第三层（依赖前两层）:  API → PG + Redis + NATS + MinIO + RAGLite + Crawler
       │
第四层（依赖 API）:  Consumer → API
       │
第五层（构建前端）:  Admin-Build, App-Build
       │
第六层（前端服务）:  Admin (Nginx), App (Next.js)
```

### 服务依赖矩阵

| 服务 | PG | Redis | NATS | MinIO | Qdrant | Caddy |
|------|:--:|:-----:|:----:|:-----:|:------:|:-----:|
| API | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| Consumer | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| RAGLite | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ |
| Crawler | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ |

> 后端 Go 代码不直接访问 Qdrant，全部通过 RAGLite 间接使用。

### 基础设施版本与端口

| 服务 | 版本 | 端口 | 用途 |
|------|------|------|------|
| Caddy | 2.10-alpine | 2019 | 反向代理 |
| PostgreSQL | 15.17 | 15432 | 主数据库 |
| Redis | 7-alpine | 16379 | 缓存 + 会话 |
| NATS | 2.14.1 | 4222/8222 | 消息队列 |
| MinIO | 2025-07-23 | 9000/9001 | 对象存储 |
| Qdrant | v1.14.1 | 6333 | 向量数据库 |
| anydoc | v0.9.11 | 8080 | 文档爬虫 |
| RAGLite | v2.17.0 | 5050 | RAG 处理引擎 |

---

## 本地开发启动

```bash
# 方式一：一键脚本（启动基础设施 + 后端 + 前端）
./scripts/dev-up.sh

# 方式二：手动启动
docker compose -f docker-compose.local.yml up -d   # 基础设施
cd backend && go run cmd/api/main.go                # API 服务
cd backend && go run cmd/consumer/main.go           # 消费者
cd web && pnpm install && pnpm dev                  # 前端
```

## 生产部署

通过 `deploy/server/docker-compose.yml` 一键拉起完整栈，前端在构建阶段编译后由 Nginx (Admin) 和 Next.js SSR (App) 分别提供服务。

## AI 集成

后端通过 CloudWeGo Eino 框架和长亭自研 ModelKit 集成多个 LLM 提供商，支持功能：

- AI 驱动的问答
- 知识提取
- 文档理解
- RAG (检索增强生成)

## IM 集成

支持多个即时通讯平台的通知和交互：

- 钉钉、企业微信、微信、飞书、Discord
