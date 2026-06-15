# PandaWiki Web App 项目分析

## 技术栈

| 类别       | 技术                                                        |
| ---------- | ----------------------------------------------------------- |
| 框架       | Next.js 16 + React + TypeScript 5.9                         |
| UI         | MUI (CSS Variables 模式) + `@ctzhian/ui` + `@panda-wiki/ui` |
| 富文本     | `@ctzhian/tiptap`（文档渲染）+ react-markdown（AI 回复）    |
| 状态管理   | React Context（`StoreProvider` + `ThemeStoreProvider`）     |
| API 客户端 | Swagger 自动生成，基于原生 fetch 的 `HttpClient`            |
| 监控       | Sentry（仅生产环境）                                        |
| 部署       | Docker, Node 22 Alpine, standalone 输出, 端口 3010          |

## 核心页面

- **首页 `/home`** — 可配置的落地页引擎，支持 16+ 组件类型（Banner、文档列表、轮播、FAQ 等），动态导入实现代码分割
- **文档详情 `/node/[id]`** — 服务端组件获取数据，Tiptap 渲染，目录锚点导航，评论区，水印保护
- **AI 问答弹窗** — 双模式：智能问答 + 文档搜索，Tab 切换 UI
- **编辑器 `/editor/[[...id]]`** — Catch-all 路由
- **登录 `/auth/login`** — 账号密码登录，记住密码，毛玻璃设计
- **Widget `/widget`** — 嵌入式聊天组件，供外部集成
- **移动端聊天 `/h5-chat`** — 独立的移动端聊天体验

## 目录结构

```
web/app/
  .env / .env.local / .env.test    — 环境配置（TARGET, STATIC_FILE_TARGET, SWAGGER_BASE_URL）
  Dockerfile                        — Docker 生产构建（Node 22 Alpine, standalone 输出）
  Makefile                          — 构建 & Docker 镜像命令
  next.config.ts                    — Next.js 配置
  swagger.api.config.ts             — Swagger 代码生成配置
  tsconfig.json                     — TypeScript 配置
  eslint.config.mjs                 — ESLint flat config
  prettier.config.js                — Prettier 格式化配置
  api-templates/                    — Swagger 代码生成自定义模板
  public/                           — 静态资源（favicon、图片、widget JS/CSS、验证码文件）
  dist/                             — 构建输出目录
  src/
    app/                            — Next.js App Router 页面和布局
    assets/                         — 字体（Gilroy, AlibabaPuHuiTi）、图片、类型定义
    components/                     — 共享 UI 组件
    constant/                       — 常量（布局尺寸、场景枚举）
    hooks/                          — 自定义 React Hooks
    provider/                       — Context Provider（Store、Theme）
    request/                        — Swagger 生成的 API 客户端
    request/pro/                    — 企业版 API 客户端
    utils/                          — 工具函数
    views/                          — 页面级视图组件
    instrumentation.ts              — Sentry 服务端初始化
    instrumentation-client.ts       — Sentry 客户端初始化
    proxy.ts                        — Next.js 中间件（认证、路由、API 代理）
    theme.ts                        — MUI 主题配置
```

## 路由结构

应用使用 Next.js App Router，配合基于中间件的路由系统。

**中间件 `proxy.ts` 是路由核心：**

- 匹配路径：`/`, `/home`, `/share/*`, `/api/*`, `/chat/*`, `/widget`, `/welcome`, `/auth/login`, `/node/*`
- **动态 Base Path**：从知识库设置中剥离可配置的 `base_url` 前缀
- **认证守卫**：受保护路径（`/`, `/node`, `/home`, `/welcome`, `/chat`）无 token 时重定向到登录页
- **Widget 守卫**：未启用 widget 时重定向到 404
- **首页路由**：若知识库设置了自定义落地页则重写到 `/home`，否则重写到 `/node/{firstNodeId}`
- **API 代理**：`/api/*` 和 `/share/*` 请求代理到后端 `process.env.TARGET`
- **访问统计**：通过 `postShareV1StatPage` 上报页面访问
- **Session 管理**：创建 `x-pw-session-id` cookie（UUID，1 年有效期）

**路由分组：**

- `(pages)` — 主页面（共享 layout，注入自定义 head/body 代码）
  - `(doc)` — 文档相关页面（共享水印包装）
    - `home` — 落地页
    - `welcome` — 首页别名
    - `node` — 目录重定向
    - `node/[id]` — 文档详情（动态路由，`force-dynamic`）
    - `editor/[[...id]]` — 编辑器（Catch-all 路由）
  - `auth/login` — 登录页
- `feedback` — 反馈页
- `widget` — Widget 页
- `h5-chat` — 移动端聊天页

## 状态管理

采用 React Context + useState 模式，未使用 Redux/Zustand。

### StoreProvider

`src/provider/index.tsx` — 大型 React Context，持有整个应用状态：

- `kbDetail` — 知识库配置（由根 layout 服务端组件传入）
- `authInfo` — 用户认证状态（客户端通过 `getApiV1User` 获取）
- `widget` — Widget 配置
- `tree` — 文档树/目录结构
- `nodeList` — 扁平节点列表
- `navList` / `selectedNavId` / `navDataMap` — 导航栏（多栏目）状态
- `catalogShow` / `catalogWidth` — 目录侧边栏可见性和宽度
- `qaModalOpen` — 问答弹窗开关
- `themeMode` — 明暗主题
- `mobile` — 移动/桌面端检测

通过 `useStore()` Hook 访问，状态持久化使用 localStorage。

### ThemeStoreProvider

`src/provider/themeStore.tsx` — 管理明暗主题模式，持久化到 cookie（`theme_mode`，10 年有效期），动态创建 MUI 主题。

## API 与数据获取

### 双 HTTP 客户端架构

**开源版 `src/request/`：**

- `httpClient.ts` — 自定义 `HttpClient` 类（Swagger 生成基类）
- 自动解包 `data.data`（`ExtractDataProp<T>` 类型）
- 401 自动重定向到登录页
- 服务端：使用 `getServerHeader()` 转发 cookies/auth headers
- 客户端：使用 `localStorage.getItem('panda_wiki_token')` 作为 `Authorization: Bearer` header
- 自动通过 `message.error()` 展示错误信息（可通过 `isAlert: false` 抑制）

**企业版 `src/request/pro/`：**

- 独立的 `httpClient.ts`，架构相同但 API 路径不同
- 服务端请求增加 `cache: "no-store"`

### API 模块（均为 Swagger 生成）

开源版：ShareApp, ShareAuth, ShareCaptcha, ShareChat, ShareChatSearch, ShareComment, ShareConversation, ShareFile, ShareNav, ShareNode, ShareOpenapi, ShareStat, User, Wechat, Widget

企业版：Auth, AuthGroup, AuthOrg, Block, Comment, Contribute, DocumentFeedback, License, Node, Prompt, ShareAuth, ShareContribute, ShareFile, ShareOpenapi, ApiToken 等

### 代码生成

`pnpm api` 命令（通过 `cx-swagger-api`）从两个 Swagger 端点生成所有请求客户端代码：

1. `${SWAGGER_BASE_URL}/swagger/doc.json` -> `src/request/`（过滤 `/share/v1` 路径）
2. `${SWAGGER_BASE_URL}/api/pro/swagger/doc.json` -> `src/request/pro/`

## 关键架构决策

### 1. Server-Client 数据桥接

根 layout 是服务端组件，直接获取知识库配置传给客户端 Provider，避免首屏 loading 闪烁。认证状态在客户端延迟水合。

### 2. 双环境 HTTP 客户端

`HttpClient` 同时工作在服务端和客户端：

- 服务端：通过 `next/headers` 转发请求头/cookies（`getServerHeader()`）
- 客户端：使用 `localStorage` token + `window._BASE_PATH_`
- 两端均有 401 自动处理

### 3. 动态 Base Path 系统

支持知识库自定义部署路径前缀（如 `/docs`），通过三种方式传播：

- `window._BASE_PATH_`（根 layout 通过 `<Script>` 设置）
- 服务端 `getServerBasePath()`（每次服务端请求时获取知识库信息）
- 中间件在路由决策前剥离 base path

### 4. 中间件驱动路由

`proxy.ts` 统一处理：认证守卫、API 代理、首页路由决策、访问统计、session 管理。

### 5. 多栏目/导航架构

`navList` / `navDataMap` / `selectedNavId` 系统支持单个知识库内的多导航栏目，每个栏目拥有独立的文档树。

### 6. 可配置落地页引擎

首页是全配置化的「组件引擎」——读取知识库设置中的 `web_app_landing_configs`，动态渲染 16+ 组件类型，每个配置块映射到 `@panda-wiki/ui` 的 UI 组件，动态导入实现代码分割。

### 7. 三层主题系统

1. 全局明暗主题（MUI CSS Variables 模式，cookie 持久化）
2. 落地页主题（每个知识库自定义色板，`THEME_TO_PALETTE` 映射）
3. Widget 主题（独立 CSS 变量前缀 `widget`）

### 8. Tiptap 文档渲染

文档使用 `@ctzhian/tiptap` 渲染（非原始 markdown-it 或 react-markdown），支持：

- 目录提取（`onTocUpdate`）
- Markdown 和 HTML 内容类型
- 字符计数
- Base URL 感知的链接处理
- 只读模式

### 9. PC/移动端响应式设计

- 服务端设备检测：`react-device-detect` `getSelectorsByUserAgent`
- 客户端媒体查询覆盖：MUI `useMediaQuery`
- 独立布局组件：`PCLayout` / `MobileLayout`
- 独立目录组件：`Catalog` / `CatalogH5`
- `useMounted()` 模式防止水合不匹配

### 10. Pro/企业版分层

Pro 功能（认证组、评论、文档反馈、许可证、Block、贡献、Prompt 等）在独立 `src/request/pro/` 目录，保持开源代码整洁。

### 11. 复制保护系统

三种模式，按知识库配置：

- `disabled` — 通过 `useCopy` Hook 禁止复制和右键菜单
- `append` — 允许复制但附加来源署名
- `none` — 不限制

### 12. 水印系统

三种模式：禁用 / 隐藏（opacity 0.01）/ 可见（opacity 0.1），内容包括用户账号 + 时间戳 + 自定义水印文字，通过 `WaterMarkProvider` 包装文档页面。

## 配置文件一览

| 文件                    | 用途                                                                        |
| ----------------------- | --------------------------------------------------------------------------- |
| `next.config.ts`        | standalone 输出、dist 目录、asset 前缀、Sentry 集成、开发代理、mermaid 转译 |
| `swagger.api.config.ts` | 两个 Swagger 源（开源 `/share/v1` 和企业版 `/api/pro`）                     |
| `.env`                  | 开发默认值：TARGET `100.120.3.46:8000`，静态文件 `:9000`                    |
| `.env.local`            | 本地开发：TARGET `localhost:8000`                                           |
| `Dockerfile`            | 生产：Node 22 Alpine, standalone 构建, 非 root 用户, 端口 3010              |
