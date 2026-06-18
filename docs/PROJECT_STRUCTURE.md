# PandaWiki 项目结构文档

## 项目概述

PandaWiki 是一个由 AI 大模型驱动的开源知识库搭建系统。该项目采用前后端分离的架构，包含后端服务、前端管理界面、前端用户界面以及 SDK。

## 根目录结构

```
/workspace/
├── .github/              # GitHub 相关配置 (如 workflows, issue templates)
├── backend/              # 后端服务代码 (Go 语言)
├── images/               # 项目相关的图片资源 (如 README 中使用的图片)
├── sdk/                  # 软件开发工具包 (SDK)
├── web/                  # 前端代码 (Node.js/React)
├── .gitattributes        # Git 属性配置
├── .gitignore            # Git 忽略文件配置
├── .gitmodules           # Git 子模块配置
├── CODE_OF_CONDUCT.md    # 行为准则
├── CONTRIBUTING.md       # 贡献指南
├── LICENSE               # 许可证 (AGPL-3.0)
├── README.md             # 项目介绍和使用指南
└── SECURITY.md           # 安全策略
```

## 后端 (backend/) 结构

后端服务使用 Go 语言编写，主要负责 API 提供、业务逻辑处理、数据存储等。

```
/workspace/backend/
├── api/                  # API 定义和接口实现
├── apm/                  # 应用性能管理 (APM) 相关代码
├── cmd/                  # 应用程序入口点 (main 函数)
├── config/               # 配置文件解析和管理
├── consts/               # 常量定义
├── docs/                 # 项目内部文档
├── domain/               # 领域模型和核心业务逻辑
├── handler/              # HTTP 请求处理器
├── log/                  # 日志管理
├── middleware/           # 中间件 (如认证、日志记录)
├── migration/            # 数据库迁移脚本
├── mq/                   # 消息队列相关代码
├── pkg/                  # 公共包和工具库
├── pro/                  # 专业版功能相关代码
├── repo/                 # 数据访问层 (Repository)
├── server/               # 服务器初始化和启动逻辑
├── setup/                # 安装和初始化相关代码
├── store/                # 存储层抽象和实现
├── telemetry/            # 遥测和监控相关代码
├── usecase/              # 用例层 (业务逻辑的具体实现)
├── utils/                # 工具函数
├── .dockerignore         # Docker 构建忽略文件
├── .golangci.toml        # Go 语言 lint 工具配置
├── cSpell.json           # 拼写检查配置
├── Dockerfile.api        # API 服务的 Dockerfile
├── Dockerfile.api.pro    # 专业版 API 服务的 Dockerfile
├── Dockerfile.consumer   # 消费者服务的 Dockerfile
├── Dockerfile.consumer.pro # 专业版消费者服务的 Dockerfile
├── go.mod                # Go 模块依赖管理
├── go.sum                # Go 模块依赖校验
├── Makefile              # 构建脚本
├── pro_imports.go        # 专业版功能导入
└── project-words.txt     # 项目特定词汇列表 (用于拼写检查)
```

## 前端 (web/) 结构

前端使用 Node.js 和 React 构建，采用 monorepo 结构管理多个应用。

```
/workspace/web/
├── .husky/               # Git hooks 配置
├── admin/                # 管理后台前端代码
├── app/                  # 用户端 Wiki 网站前端代码
├── packages/             # 共享的组件库和工具包
├── .gitignore            # Git 忽略文件配置
├── .prettierignore       # Prettier 格式化忽略文件
├── package.json          # Node.js 项目配置
├── pnpm-lock.yaml        # pnpm 依赖锁定文件
├── pnpm-workspace.yaml   # pnpm 工作区配置
└── prettier.config.js    # Prettier 代码格式化配置
```

## SDK (sdk/) 结构

SDK 提供了与 PandaWiki 系统交互的工具包。

```
/workspace/sdk/
└── rag/                  # RAG (Retrieval-Augmented Generation) 相关 SDK
```