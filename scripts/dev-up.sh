#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"${ROOT_DIR}/scripts/dev-deps.sh"

echo
echo "本地开发启动方式已简化为："
echo
echo "1. 基础依赖：Docker 后台运行"
echo "2. API：      ./scripts/dev-api.sh"
echo "3. Consumer： ./scripts/dev-consumer.sh"
echo "4. 前端：     ./scripts/dev-web.sh"
echo
echo "访问地址："
echo "  管理台: http://localhost:5173"
echo "  前台:   http://localhost:3010"
echo "  后端:   http://127.0.0.1:8000"
echo
echo "说明：API、consumer、前端都以前台命令运行，日志会直接显示在对应终端里。"
