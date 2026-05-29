# TODO

- 排查并修复本地 Docker Desktop 环境下 `panda-wiki-caddy` admin endpoint 启动失败的问题。
  当前现象：
  - 官方 `panda-wiki-caddy:2.10-alpine` 容器已能启动，但 admin endpoint 初始化失败
  - 日志反复报错：`starting caddy administration endpoint: operation not supported`
  影响：
  - PandaWiki 后端无法把知识库访问配置同步到 Caddy
  - 创建知识库接口 `/api/v1/knowledge_base` 在开启 Caddy 同步时失败
  已尝试：
  - Unix socket admin 模式
  - TCP `2019` admin 端口模式
  后续方向：
  - 继续排查该官方镜像在 macOS + Docker Desktop 下的兼容性
  - 如仍不兼容，考虑改为本地开发专用的通用 Caddy / Nginx 反向代理方案
