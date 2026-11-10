# PandaWiki 单机部署

这套部署目录适合：

- 一台 Linux 服务器
- 直接在服务器上放当前仓库源码
- 使用 Docker Compose 启动 PandaWiki 及其依赖

## 资源建议

- 最低可试跑：`2C4G`
- 完整服务推荐：`8C16G`

如果服务器上还跑着别的服务，不建议把完整链路部署在 `2C4G` 机器上。

## 前置要求

- 已安装 Docker
- 已安装 Docker Compose 插件或 `docker-compose` v1.29.2
- 服务器能访问你所配置的镜像源

## 使用步骤

以下复制模板步骤仅用于首次部署。已有环境保留原 `.env`，不要用模板覆盖现有密码、数据库名或存储配置。

1. 复制环境模板：

```bash
cd deploy/server
cp .env.example .env
```

2. 修改 `.env` 中至少这些值：

- `SERVER_IP`
- `POSTGRES_PASSWORD`
- `RAGLITE_POSTGRES_DB`
- `JWT_SECRET`
- `ADMIN_PASSWORD`
- `QDRANT_API_KEY`
- `RAGLITE_API_KEY`

对象存储配置统一放在仓库根目录 `.env`，参考根目录 `.env.example`。
当前生产环境只启用 OSS，MinIO 不会启动或参与读取。
部署脚本会读取 `OSS_ENDPOINT`、`OSS_ACCESS_KEY`、`OSS_SECRET_KEY`、
`OSS_BUCKET_NAME`、`OSS_DOWNLOAD_URL` 和 `OSS_REGION`。

文件详情会检查对象是否存在于当前启用的 OSS。能找到时显示 OSS 类型和临时链接；找不到时
标记为“非 OSS”，不会尝试访问历史 MinIO。

PDF 解析基础镜像默认使用
`registry.cn-hangzhou.aliyuncs.com/misaka-images/python:3.12-slim-bookworm`。
私有仓库需在服务器执行 `docker login registry.cn-hangzhou.aliyuncs.com`。
已有 `deploy/server/.env` 中的 `PDF_PARSER_BASE_IMAGE` 会覆盖默认值，需同步更新。

说明：

- `POSTGRES_DB` 是 PandaWiki 主应用数据库
- `RAGLITE_POSTGRES_DB` 是 raglite 独立数据库
- 两者不要配置成同一个库名，否则会发生迁移表冲突

## 运行日志

部署会同时启动 Loki 与 Alloy：

- Alloy 只采集带有 `com.pandawiki.logs=true` 标签的本项目容器日志。
- Loki 保留 14 天日志，仅在 Compose 内网开放，不直接暴露公网端口。
- 管理端“系统设置 → 运行日志”仅系统管理员可访问，可按时间、服务、容器位置和内容筛选。
- “全部运行日志”显示各服务的完整 stdout/stderr；“业务事件日志”显示带问答、文档、发布、学习上下文的结构化事件。
- 密码、密钥、令牌等常见敏感字段会在写入集中日志前脱敏。

3. 执行一键部署：

```bash
cd deploy/server
bash deploy.sh
```

已有环境升级使用 `bash deploy.sh upgrade backend`（PDF 解析、文件生成服务、API、Consumer、助手 Worker），
或 `bash deploy.sh upgrade app`（用户端）。脚本先完成目标镜像构建再切换容器；
构建失败保留旧容器，切换失败只恢复原有服务且禁止重新构建。
回滚未通过启动或状态检查时保留回滚镜像。镜像回滚不恢复数据库数据或迁移。

## 默认访问地址

部署完成后默认会输出：

- 管理台：`http://SERVER_IP:18080`
- 前台：`http://SERVER_IP:3010`
- 后端：仅容器内网 `http://api:8000`，不开放宿主机端口
- 文件对象存储：根目录 `.env` 中的 `OSS_DOWNLOAD_URL`；业务文件仍通过原有鉴权、下载和预览流程访问

## 隐藏后端端口的升级

管理端继续使用 `ADMIN_PORT`（默认 18080），用户端继续使用 `APP_PORT`（默认 3010），登录方式和页面路径不变。管理端 Nginx、用户端 Next.js 均通过 Compose 内网访问 `api:8000`；API 不再配置宿主机 `ports` 映射。前端构建配置也使用内部服务地址，不依赖服务器开放 8000。其他基础设施当前仍按各自的回环地址配置运行，本改动不增加其对外映射。

已有 `deploy/server/.env` 中的 `API_PORT` 可以原样保留，但不再生效。无需修改密码、数据库名、Compose 项目名、数据卷名称、根目录 OSS 配置或门户网站地址。原先直接调用 `服务器IP:8000` 的外部程序，需要改为经现有前端代理访问相应接口，继续使用原认证与权限校验；接口路径仍会出现在浏览器 Network 中。

如配置了非空 `SECURITY_ALLOWED_HOSTS`，应保留实际对外 IP/域名，并包含内部主机名 `api`，供用户端服务端请求使用；不关闭 Host、Origin 或 CSRF 校验。外部部署的爬虫、机器人或其他集成若曾直接连接宿主机 API 端口，需要先确认它们的调用和回调地址；其他机器不能解析 Docker 内部的 `api` 主机名。

升级前必须核对当前运行容器的实际 Compose 项目标签、配置文件路径、挂载卷和数据库名，不能按当前目录或 `PROJECT_NAME` 猜测卷名。保存现有配置、记录栏目和文档数量，确认升级前数据库备份可读取、可恢复；如本次发布包含其他数据库迁移，还需保存必要的原文件/索引备份。数据库备份不等于 OSS 原文件或向量索引备份。发现原卷不存在、挂载不同或业务库意外为空时，停止排查，不执行初始化或重置。

在仓库根目录执行（本步骤假定服务器已完成前文历史安全升级）：

```bash
bash deploy/server/deploy.sh upgrade backend
```

必须重建 API 容器才能移除旧端口映射，单独执行 `restart api` 不会生效。部署脚本兼容 Docker Compose V1；升级不删除数据卷。脚本会一并升级后端组件，切换期间 API 会短暂中断，不应在生产任务执行过程中随意重启。管理端与用户端的现有运行时代理已经指向内网，本改动不要求强制重建两个前端；如果另有旧版前端仍直接调用 `IP:8000`，则需分别执行 `upgrade admin`、`upgrade app`。旧版安全基线尚未更新的环境仍应按下方对应说明整套升级。

如果 Consumer 原本因旧学习队列而有意停止，使用 `KEEP_CONSUMER_STOPPED=1 bash deploy/server/deploy.sh upgrade backend`，避免本次升级恢复旧任务消费。

API 重建后 Docker 内部 IP 可能改变，现有管理端 Nginx 的 `upstream` 在加载配置时解析服务名。对仍在运行的管理端执行一次配置检查和重载，使它重新解析 API 地址，避免沿用旧 IP 导致 502（请使用实际管理端容器名）：

```bash
docker exec pandawiki-admin sh -c 'nginx -t && nginx -s reload'
```

这只重载 Nginx，不删除容器或数据卷；若管理端本次也已重建，则它已重新解析地址，无需重复操作。

升级后用实际容器名确认（默认名称如下，若项目配置不同请替换）：

```bash
docker inspect pandawiki-api --format '{{json .HostConfig.PortBindings}}'
docker ps --format 'table {{.Names}}\t{{.Ports}}'
```

API 的 `PortBindings` 应为空对象 `{}` 或 `null`，端口列表中不应再有 `0.0.0.0:8000->8000/tcp` 或 `[::]:8000->8000/tcp`。单独显示 `8000/tcp` 只是镜像的内部端口声明，不代表对外发布。若旧配置曾发布其他宿主机端口，同样必须确认该映射已消失。还需从外部客户端验证旧 API 地址不可达，排除服务器上其他代理或监听程序仍提供该入口；不要为此扩大防火墙放行范围。

分别经管理端和用户端验证登录、文档树、问答流式返回、附件下载与预览，并对比原挂载卷、数据库及栏目/文档数量。本次网络调整不需要重新学习文档或重建索引。接口代理继续执行原有权限和来源校验，不以“隐藏端口”代替鉴权。回滚时需注意旧 Compose 配置可能重新发布 API 端口；仅回滚镜像不能替代配置核对或数据库恢复。

## 常用命令

查看状态：

```bash
docker compose --env-file .env -f docker-compose.yml ps
```

查看后端日志：

```bash
docker compose --env-file .env -f docker-compose.yml logs -f api
```

重启服务：

```bash
docker compose --env-file .env -f docker-compose.yml restart api consumer admin app
```

停止服务：

```bash
docker compose --env-file .env -f docker-compose.yml down
```

## 2026-10-22 安全升级

本次涉及会话、密钥不可逆哈希迁移及 Redis/NATS 认证，首次需 `upgrade all`。部署前配置独立的 `REDIS_PASSWORD`、`NATS_PASSWORD` 并备份数据库；不能只回滚旧镜像。[安全问题核实、修复、验证与部署合并说明](../../docs/security-review-20261022/安全问题核实与修复部署合并说明.md)。


### 2026-10-26 安全升级：请整套发布

两端登录改为服务端 HttpOnly Cookie，原有 localStorage 登录令牌不再使用；升级后需重新登录。账号登录接口不再在 JSON 返回 JWT，程序化账号接入应使用 Cookie Jar 和 `X-PW-CSRF: 1`。显式 API 密钥接入保留。

本次必须使用 `bash deploy/server/deploy.sh upgrade all`，同步 API、Consumer、两端、NATS/监控代理及 Gotenberg；不要只更新一个前端或后端。脚本自动生成并保存缺失的 `PREVIEW_PASSWORD`、`NATS_MONITOR_PASSWORD`，不需要发送密码。正式 HTTPS 入口需覆盖转发协议头，可设置 `AUTH_COOKIE_SECURE=true` 强制 Secure。

新增 `nats-monitor` 仅代理认证后的监控请求，原始监控只在 NATS 网络命名空间回环监听；预览转换使用原生 Basic Auth。管理端容器内使用 Go 过滤代理和回环 Nginx，宿主访问端口不变。

普通文档上传显式设置 private ACL；外部 OSS 实际 ACL 支持、上传账号权限、桶策略和历史对象需运维核对。设置私有 ACL 失败时不会退回公共上传。MinIO 公共读桶会拒绝普通文档写入，不会由升级脚本自动修改历史桶策略。

完整实现及本地验证见 [安全补充修复说明](../../docs/security-review-20261022/安全补充修复-20261026.md)。
