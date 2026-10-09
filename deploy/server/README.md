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
部署脚本会读取 `OSS_ENDPOINT`、`OSS_ACCESS_KEY`、`OSS_SECRET_KEY`、
`OSS_BUCKET_NAME`、`OSS_DOWNLOAD_URL` 和 `OSS_REGION`。生产环境不再启动 MinIO。

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
