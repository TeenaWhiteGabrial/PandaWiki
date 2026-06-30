# PandaWiki Admin 项目文档

## 技术栈

| 维度       | 技术                                          | 版本/细节                        |
| ---------- | --------------------------------------------- | -------------------------------- |
| 框架       | React + TypeScript                            | ES2020, react-jsx                |
| 构建       | Vite                                          | ^6.0.1                           |
| 路由       | react-router-dom                              | ^7.0.2 (懒加载)                  |
| UI         | MUI + antd + `@ctzhian/modelkit`              | 含本地兼容层 `src/components/ui` |
| 状态管理   | Redux Toolkit + react-redux                   | ^2.5.0                           |
| HTTP       | axios                                         | ^1.7.9                           |
| 编辑器     | `@ctzhian/tiptap` + ProseMirror               | 协同编辑用 yjs                   |
| 图表       | ECharts                                       | ^5.6.0                           |
| Markdown   | react-markdown + remark-gfm + rehype-katex 等 | 含 GFM、数学公式、安全过滤       |
| 协同编辑   | yjs + y-websocket                             | CRDT 实时协同                    |
| 拖拽排序   | @dnd-kit/core + @dnd-kit/sortable             | 文档树拖拽                       |
| 代码检查   | ESLint (flat config) + typescript-eslint      |                                  |
| 代码格式化 | Prettier                                      | 单引号, 2空格, 尾逗号            |
| 包管理     | pnpm                                          | ^10.12.1 (workspace)             |

## 目录结构

```
src/
├── main.tsx                # 应用入口 - 挂载 React, Provider, BrowserRouter
├── App.tsx                 # 根组件 - 路由渲染, 主题, 登录校验
├── router.tsx              # 路由配置 - lazy loading + Suspense
├── api/                    # [已废弃] 旧版 API 层, 保留兼容
├── request/                # Swagger 生成的 API 客户端 (当前使用)
│   ├── httpClient.ts       # 核心 HTTP 客户端 (axios 封装, 拦截器, Bearer token)
│   ├── types.ts            # 自动生成的类型定义
│   ├── nodeStream.ts       # SSE 流式封装 (节点摘要流)
│   ├── pro/                # Pro 版 API (独立生成)
│   └── ...                 # 各模块 API 文件
├── pages/                  # 页面组件
│   ├── document/           # 文档管理 (最复杂模块)
│   │   ├── layout/         # 文档列表布局
│   │   ├── editor/         # 文档编辑器 (Header, TOC, AI生成, 摘要)
│   │   └── component/      # 文档相关组件 (添加/删除/移动/属性/协同)
│   ├── setting/            # 系统设置 (28个子组件: 机器人/SEO/样式/Web等)
│   ├── release/            # 版本发布
│   ├── stat/               # 数据统计 (访客/热文/来源/地图)
│   ├── conversation/       # 对话管理
│   ├── login/              # 登录页
│   ├── 401/                # 无权限页
│   ├── contribution/       # 贡献 (待开发)
│   └── feedback/           # 反馈 (待开发)
├── components/             # 通用组件 (24个)
│   ├── Avatar / Card / Cascader / CustomImage
│   ├── CustomModal / Drag / Emoji / Form
│   ├── Header / Loading / MarkDown / Sidebar
│   ├── TreeDragSortable/   # 拖拽树排序 (10个文件)
│   ├── UploadFile / MapChart / PieTrend
│   └── ...
├── store/                  # Redux 状态管理
│   └── slices/
│       ├── config.ts       # 主配置 (用户, 知识库, 模型, 应用预览)
│       └── breadcrumb.ts   # 面包屑 (仅 pageName)
├── hooks/                  # 自定义 Hooks
│   ├── useBindCaptcha.ts
│   ├── useCommitPendingInput.tsx
│   ├── useDebounceAppPreviewData.tsx
│   └── useURLSearchParams.tsx
├── utils/                  # 工具函数
│   ├── index.ts            # 通用工具
│   ├── drag.ts             # 拖拽工具
│   ├── fetch.ts            # SSE 流式请求封装
│   ├── getBasename.ts      # basename 计算
│   ├── permission.ts       # 权限判断
│   └── tree.ts             # 树结构操作
├── constant/               # 常量定义
│   ├── enums.tsx           # 枚举定义 (20KB)
│   └── version.ts          # 版本信息
├── services/               # 服务层
│   └── modelService.ts     # 模型服务
├── themes/                 # MUI 主题定制 (浅色/深色)
├── layouts/                # 布局组件 (MainLayout, NoSidebarHeaderLayout)
└── assets/                 # 静态资源 (字体/图片/CSS/JSON)
```

## 构建与开发命令

| 命令                   | 说明                                                |
| ---------------------- | --------------------------------------------------- |
| `pnpm dev`             | 启动 Vite 开发服务器 (代理 `/api` → localhost:8000) |
| `pnpm build`           | 生产构建 (tsc 类型检查 + vite build)                |
| `pnpm build:dev`       | 开发环境构建 (无 tsc 检查)                          |
| `pnpm build:analyze`   | 构建产物可视化分析                                  |
| `pnpm api`             | 从后端 Swagger 重新生成 API 客户端                  |
| `pnpm generate-routes` | 手动执行路由生成脚本                                |

### Docker 构建

```bash
make build   # pnpm run build
make image   # 构建并加载 Docker 镜像 (nginx:alpine)
make save    # 导出 Docker 镜像为 tar 文件
```

## 核心架构设计

### 认证机制

- Token 存储在 `localStorage`，键名 `panda_wiki_token`
- 请求拦截器自动注入 `Authorization: Bearer ${token}`
- 响应拦截器：401 状态自动清除 token 并重定向到登录页
- `App.tsx` 检查 token，无 token 且非登录页则重定向

### API 层架构

项目存在新旧两套 API 层：

- **旧版** (`src/api/`)：已标记 `@deprecated`，保留兼容
- **新版** (`src/request/`)：Swagger 自动生成的客户端，包含基础版和 Pro 版两套

生成流程：`pnpm api` → `cx-swagger-api` 读取后端 Swagger 端点 → 生成到 `src/request/`

> **注意**: 不应手动编辑生成文件。如需非 JSON 行为（如 SSE），创建薄封装层（参考 `nodeStream.ts`）。

### 路由与布局

路由分为 3 组，均使用 `React.lazy()` 懒加载：

1. **主布局** (Sidebar + Header)：文档列表、设置、发布、统计、对话
2. **编辑器布局** (仅 Header，需认证)：文档编辑、历史版本、空间
3. **无布局** (无认证)：登录、401

### 状态管理

Redux Store 极简，仅两个 slice：

- `config`：用户信息、知识库 ID/列表、模型状态、应用预览数据
- `breadcrumb`：仅 `pageName` 字段

大部分状态保存在组件本地，全局状态非常精简。关闭了 `serializableCheck`（因 config 中存有函数引用）。

### 构建分块策略

Vite 手动分块为 6 组，优化加载性能：

| Chunk             | 内容                                             |
| ----------------- | ------------------------------------------------ |
| `vendor-react`    | React 核心 + Router + Redux                      |
| `vendor-mui`      | MUI                                              |
| `vendor-echarts`  | ECharts                                          |
| `vendor-editor`   | highlight.js, lowlight, katex, prosemirror-state |
| `vendor-markdown` | react-markdown + 生态                            |
| `vendor-yjs`      | yjs, y-websocket                                 |

## 关键特性

### 协同编辑

使用 `yjs` + `y-websocket` 实现 CRDT 实时协同编辑，`EditorCollaboration.tsx` 组件处理协同逻辑。

### SSE 流式请求

- `src/utils/fetch.ts` 封装 SSE 客户端
- `src/request/nodeStream.ts` 实现节点摘要流式生成
- 支持 `sse-json` 响应模式，事件类型：`data` / `done` / `error`

### 动态 Basename (子路径部署)

- `index.html` 内嵌 JS 通过正则匹配计算 `window.__BASENAME__`
- API 请求 `baseURL`、路由 `BrowserRouter` 的 `basename`、静态资源路径均依赖此值
- `wrapWindowOpen` 覆写 `window.open` 以注入 basename

### 路由自动生成

- `scripts/generate-routes.js` 在构建开始时自动执行
- Vite 插件 `generateRoutesPlugin` 在 `buildStart` 钩子触发
- 开发模式监听 `src/router.tsx` 变化自动重新生成路由列表

## pnpm Workspace 结构

```
web/
├── pnpm-workspace.yaml
├── admin/          # 本项目 (管理后台)
├── app/            # Next.js 用户端
└── packages/
    ├── icons/      # 图标包
    ├── themes/     # 共享主题 (@panda-wiki/themes)
    └── ui/         # 共享 UI 包（当前 admin 已主要使用本地兼容层/antd）
```

- `src/components/ui`：admin 侧本地兼容层，当前承接 `message`、`Modal`、`Table`、`ThemeProvider` 等通用 UI 能力
- `@panda-wiki/themes`：共享调色板 (`lightPalette`, `darkPalette`)
- `@ctzhian/tiptap`：自研富文本编辑器

## 开发代理配置

Vite 开发服务器配置了 3 个代理规则：

| 路径           | 目标         | 环境变量                                   |
| -------------- | ------------ | ------------------------------------------ |
| `/api`         | 后端 API     | `TARGET` (默认 localhost:8000)             |
| `/static-file` | 静态文件服务 | `STATIC_FILE_TARGET` (默认 localhost:9000) |
| `/share`       | 分享服务     | `SHARE_TARGET`                             |

## 注意事项

1. **不要编辑生成文件**：`src/request/` 下的文件由 Swagger 生成，修改需通过模板或封装层
2. **API 同步**：后端 Swagger 变更后，需执行 `cd web && pnpm api` 重新生成前端客户端
3. **路径别名**：`@` → `src/`，在 `vite.config.ts` 和 `tsconfig.app.json` 中同步配置
4. **esbuild keepNames**：保留函数/类名，避免第三方库依赖 `constructor.name` 出问题
5. **类型检查**：`noUnusedLocals` / `noUnusedParameters` 已关闭，`no-explicit-any` 仅为 warn
