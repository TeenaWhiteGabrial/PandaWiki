# PandaWiki 单机部署

这套部署目录适合：

- 一台 Linux 服务器
- 直接在服务器上放当前仓库源码
- 使用 Docker Compose 启动 PandaWiki 及其依赖

## 资源建议

- 最低可试跑：`2C4G`
- 更推荐：`4C8G`

如果服务器上还跑着别的服务，不建议把完整链路部署在 `2C4G` 机器上。

## 前置要求

- 已安装 Docker
- 已安装 Docker Compose 插件
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
- `MINIO_ROOT_PASSWORD`
- `JWT_SECRET`
- `ADMIN_PASSWORD`
- `QDRANT_API_KEY`
- `RAGLITE_API_KEY`

说明：

- `POSTGRES_DB` 是 PandaWiki 主应用数据库
- `RAGLITE_POSTGRES_DB` 是 raglite 独立数据库
- 两者不要配置成同一个库名，否则会发生迁移表冲突

3. 执行一键部署：

```bash
cd deploy/server
bash deploy.sh
```

## 默认访问地址

部署完成后默认会输出：

- 管理台：`http://SERVER_IP:18080`
- 前台：`http://SERVER_IP:3010`
- 后端：`http://SERVER_IP:8000`
- MinIO Console：`http://SERVER_IP:9001`

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
