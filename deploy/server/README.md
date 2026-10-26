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

已有环境升级使用 `bash deploy.sh upgrade backend`（PDF 解析服务、API、Consumer），
或 `bash deploy.sh upgrade app`（用户端）。脚本先完成目标镜像构建再切换容器；
构建失败保留旧容器，切换失败只恢复原有服务且禁止重新构建。
回滚未通过启动或状态检查时保留回滚镜像。镜像回滚不恢复数据库数据或迁移。

## 默认访问地址

部署完成后默认会输出：

- 管理台：`http://SERVER_IP:18080`
- 前台：`http://SERVER_IP:3010`
- 后端：`http://SERVER_IP:8000`
- 文件访问：根目录 `.env` 中的 `OSS_DOWNLOAD_URL`

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
