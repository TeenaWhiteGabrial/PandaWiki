# PandaWiki 生产安全测试：测试前完整备份与测试后整体恢复

适用环境：本仓库 `deploy/server/docker-compose.yml` 的单机 Docker Compose 部署，文件位于外部 S3 兼容 OSS，允许暂停写入。本文是操作方案和命令模板，没有对生产服务器执行备份、停机或恢复。

恢复目标：以测试开始前的冻结时间点 **T0** 为基线，撤销整个测试期间的新增、修改、删除，包括业务写入和测试写入。恢复后的系统从 T0 的业务数据继续运行，不合并测试期间的数据。测试证据另行保留。

## 1. 备份范围与恢复边界

| 对象               | 当前项目位置/用途                                                                                  | 备份与恢复要求                                                                                                                     |
| ------------------ | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| PandaWiki 主数据库 | `POSTGRES_DB`，默认 `panda-wiki`                                                                   | 全库：文档、栏目、发布版本、用户、部门、等级、权限、模型配置、系统配置、会话记录、上传任务等；不能只备份 nodes 表                  |
| RAGLite 数据库     | `RAGLITE_POSTGRES_DB`，默认 `panda-wiki-raglite`                                                   | 与主数据库、向量库属于同一基线；实际数据库名以运行容器配置和数据库目录为准                                                         |
| PostgreSQL 集群    | `pandawiki_postgres` 逻辑卷                                                                        | 停机物理备份，包括 WAL 和全局角色；另做两库逻辑备份和 globals 作为第二条恢复路径                                                   |
| Qdrant             | `pandawiki_qdrant`，容器 `/qdrant/storage`                                                         | 完整冷备卷，包含集合、向量、payload、别名和相关状态；恢复同版本，避免重新学习全部文档                                              |
| NATS JetStream     | `pandawiki_nats`，容器 `/data`                                                                     | 流、持久消费者、消息及消费状态一起回退，避免测试消息继续执行                                                                       |
| RAGLite 工作目录   | `pandawiki_raglite`，容器 `/data`                                                                  | 与数据库、Qdrant、NATS 一起回退                                                                                                    |
| Redis              | `pandawiki_redis`，容器 `/data`                                                                    | 保存 RDB，并冷备完整目录，包括启用 AOF 时的全部分段和 manifest；本项目还用 Redis 存登录 Session/SessionKey，不能一概视为可丢弃缓存 |
| 外部 OSS           | 根目录 `.env` 中的 `OSS_BUCKET_NAME`                                                               | 备份所有项目对象；上传原件、附件、ZIP、图片、解析资源、预览和 RAG 资源均需覆盖。不能只根据文件扩展名选取                           |
| 可选独立图片 OSS   | `IMAGE_OSS_*` 配置                                                                                 | 若实际启用，另外备份对应桶及真实对象前缀；不能假设与主 OSS 相同                                                                    |
| 部署与源码         | 根目录 `.env`、`deploy/server/.env`、Compose、部署脚本、配置、SSL/反向代理配置、源码、前端构建输出 | 全量归档，包括未提交修改；同时保存实际运行配置和镜像，不能仅记录 Git commit                                                        |
| 镜像与容器状态     | API、Consumer、Admin、App 及依赖                                                                   | 按运行镜像 ID 归档、固定版本；记录 mounts、网络、端口、启动命令；检查容器可写层的必要文件                                          |
| 服务器与外部配置   | 系统盘、全部数据盘、安全组、WAF/LB、DNS、证书、OSS/IAM/KMS 配置                                    | 停机后做整机/全盘快照；外部资源配置单独导出，不在磁盘快照内                                                                        |
| 运行日志           | Loki、Alloy 数据卷和宿主日志                                                                       | T0 日志随基线备份；测试过程日志/报告在恢复前另存到测试账号不可删除的位置                                                           |

当前 Compose 的 `pandawiki_*` 是逻辑卷名，实际名称一般带 Compose 项目前缀。恢复命令必须依据运行容器的 Mounts，不能猜卷名。pnpm 缓存不是业务数据，但完整快照可以一起保留。

完整备份采用三层：**整机/全盘快照 + 冷备卷/逻辑数据库/镜像配置 + 独立 OSS 备份**。三层属于同一个 T0，备份至少有一份离开生产主机，并且测试身份无权修改或删除。

逻辑备份可以在线生成，但独立生成的数据库、文件和队列备份不自动具备跨系统一致性；本次直接停写以固定共同基线。PostgreSQL 原始目录不能在服务运行时直接 tar，需完整停机或使用经过验证的一致快照机制。[PostgreSQL 文件系统备份文档](https://www.postgresql.org/docs/15/backup-file.html)

## 2. 测试前准备

1. 安排维护窗口，明确 T0 后所有写入均会被丢弃；暂停正常用户使用、定时导入、机器人/集成回调和外部调用。
2. 在 LB/WAF/反向代理以及直连 API 入口切维护模式。停止前端不等于 API、数据库、消息队列或 OSS 停写。
3. 提前创建本次专用备份目录和独立备份桶/前缀，预置备份工具和恢复镜像，检查磁盘容量。保存原镜像架构、Docker/Compose 和内核版本。
4. 清点主 OSS 与图片 OSS 的真实范围。若桶与其他系统共用，只操作经清点的项目对象前缀；不能恢复整个共享桶。
5. 尽量完成正在上传的 ZIP 和索引任务。记录不能完成的任务 ID、状态及队列积压。OSS 对象复制不包含未完成 multipart 上传的 UploadID/分片，恢复到新桶不能继续旧上传；因此优先让这些任务完成或在 T0 前明确结束。
6. 已发出的 OSS 预签名地址仍可直接写 OSS，项目分片上传地址有效期可达 30 分钟。用存储侧访问策略冻结该项目写入，或等待预签名失效并确认无在途上传；不要只停止 API。
7. 安排恢复演练服务器或隔离的恢复实例。演练时使用独立 PostgreSQL/Redis/NATS/Qdrant、独立 OSS；阻断生产机器人、回调和真实消息通知，避免克隆系统回连生产。

不要把备份保存到项目目录、前端静态目录或测试账号可读写的目录。`.env`、数据库 globals、完整 Compose 配置和容器 inspect 文件包含敏感配置，按加密备份管理，禁止发到工单正文或聊天。

## 3. 初始化命令与清点

下列命令在生产 Linux 的 **Bash** 中执行。先替换项目路径、容器前缀和备份盘路径；需要 Docker 管理权限。使用 Docker Compose V2；旧版 Compose 需要先适配多 env-file 参数。命令中的路径和参数都是示例，不能直接套用到未核对的部署。

```bash
set -euo pipefail
umask 077

PW_ROOT=/opt/PandaWiki
PW_DEPLOY="$PW_ROOT/deploy/server"
PW_CONTAINER_PREFIX=pandawiki
PW_BACKUP_ROOT=/srv/pandawiki-backups
PW_STAMP=$(date -u +%Y%m%dT%H%M%SZ)
PW_BACKUP="$PW_BACKUP_ROOT/$PW_STAMP"
mkdir -p "$PW_BACKUP"/{meta,db,volumes,images,source,oss,runtime}

# 以运行容器的 Compose 标签为准；PROJECT_NAME 只控制容器名，
# 它未必就是 Compose 项目名。
PW_PROJECT=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' \
  "$PW_CONTAINER_PREFIX-postgres")
test -n "$PW_PROJECT"

# 还原 deploy.sh 派生的 OSS 环境变量，不执行 .env 文件内容。
# 本模板与当前部署脚本相同，约定 OSS_ENDPOINT 使用未加引号的 KEY=value。
PW_OSS_ENDPOINT=$(sed -n 's/^OSS_ENDPOINT=//p' "$PW_ROOT/.env" | tail -n 1)
test -n "$PW_OSS_ENDPOINT"
case "$PW_OSS_ENDPOINT" in
  https://*) export OSS_S3_ENDPOINT="${PW_OSS_ENDPOINT#https://}" OSS_USE_SSL=true ;;
  http://*) export OSS_S3_ENDPOINT="${PW_OSS_ENDPOINT#http://}" OSS_USE_SSL=false ;;
  *) export OSS_S3_ENDPOINT="$PW_OSS_ENDPOINT" OSS_USE_SSL=false ;;
esac
export OSS_S3_ENDPOINT="${OSS_S3_ENDPOINT%/}"

PW_COMPOSE=(docker compose --project-directory "$PW_DEPLOY" \
  --env-file "$PW_ROOT/.env" --env-file "$PW_DEPLOY/.env" \
  -p "$PW_PROJECT" -f "$PW_DEPLOY/docker-compose.yml")

"${PW_COMPOSE[@]}" ps -a > "$PW_BACKUP/meta/compose-ps.txt"
"${PW_COMPOSE[@]}" config > "$PW_BACKUP/meta/compose.resolved.yml"
"${PW_COMPOSE[@]}" config --format json > "$PW_BACKUP/meta/compose.resolved.json"
mapfile -t PW_CONTAINER_IDS < <("${PW_COMPOSE[@]}" ps -a -q)
test "${#PW_CONTAINER_IDS[@]}" -gt 0
docker inspect "${PW_CONTAINER_IDS[@]}" > "$PW_BACKUP/meta/containers.json"
docker version > "$PW_BACKUP/meta/docker-version.txt"
docker compose version > "$PW_BACKUP/meta/compose-version.txt"
uname -a > "$PW_BACKUP/meta/uname.txt"
date -u +%FT%TZ > "$PW_BACKUP/meta/backup-start-utc.txt"
printf '%s\n' "$PW_PROJECT" > "$PW_BACKUP/meta/compose-project-name.txt"
printf '%s\n' "$PW_ROOT" > "$PW_BACKUP/meta/project-path.txt"
printf '%s\n' "$PW_CONTAINER_PREFIX" > "$PW_BACKUP/meta/container-prefix.txt"
git -C "$PW_ROOT" rev-parse HEAD > "$PW_BACKUP/meta/git-head.txt"
git -C "$PW_ROOT" status --short > "$PW_BACKUP/meta/git-status.txt"
```

`compose.resolved.yml` 必须核对与 `containers.json` 的运行配置一致：若运行实例由其他 override 文件、临时环境变量或不同 Compose 启动，本方案中的 PW_COMPOSE 必须纳入那些配置。默认环境模板不代表当前生产实际配置。

从 Mounts 生成卷和镜像清单，并给运行镜像加独立基线标签。这里只处理本项目容器，构建任务不属于运行服务：

```bash
export PW_BACKUP PW_STAMP
python3 - <<'PY'
import json, os, pathlib
root = pathlib.Path(os.environ['PW_BACKUP'])
containers = json.loads((root / 'meta/containers.json').read_text())
volumes, images = set(), {}
for c in containers:
    service = c['Config']['Labels']['com.docker.compose.service']
    if service.endswith('-build'):
        continue
    images[service] = {
        'id': c['Image'],
        'tag': f'pw-baseline/{service}:{os.environ["PW_STAMP"].lower()}',
    }
    for m in c.get('Mounts', []):
        if m['Type'] == 'volume':
            volumes.add(m['Name'])
(root / 'meta/volume-names.txt').write_text(''.join(v + '\n' for v in sorted(volumes)))
(root / 'meta/images.json').write_text(json.dumps(images, indent=2))
(root / 'meta/images.tsv').write_text(''.join(f'{s}\t{x["id"]}\t{x["tag"]}\n' for s, x in sorted(images.items())))
(root / 'meta/images.override.yml').write_text('services:\n' + ''.join(f'  {s}:\n    image: {json.dumps(x["tag"])}\n' for s, x in sorted(images.items())))
PY

PW_IMAGE_REFS=()
while IFS=$'\t' read -r PW_SERVICE PW_IMAGE_ID PW_IMAGE_REF; do
  docker tag "$PW_IMAGE_ID" "$PW_IMAGE_REF"
  PW_IMAGE_REFS+=("$PW_IMAGE_REF")
done < "$PW_BACKUP/meta/images.tsv"
docker image save "${PW_IMAGE_REFS[@]}" | gzip > "$PW_BACKUP/images/baseline-images.tar.gz"

# 使用已经存在的 PostgreSQL 镜像作 tar 工具镜像；必须确认其中是 GNU tar。
# 若实际 PG 镜像为 Alpine/不含 GNU tar，请预置经批准的 GNU tar 工具镜像。
PW_HELPER_IMAGE=$(docker inspect -f '{{.Image}}' "$PW_CONTAINER_PREFIX-postgres")
docker run --rm --network none --entrypoint tar "$PW_HELPER_IMAGE" --version
printf '%s\n' "$PW_HELPER_IMAGE" > "$PW_BACKUP/meta/helper-image.txt"
```

检查每个运行容器的 `docker diff 容器名`，记录可写层上的配置、证书、许可证或身份文件。默认 API 会使用 `/data/.machine_id`，但当前生产 Compose 没有为 API 挂载 `/data`；如果生产中存在该目录，停机后使用 `docker cp` 单独保存。镜像归档不会包含运行后新增的可写层文件。整机快照是这些文件的另一层保障。

## 4. 建立冻结点并备份数据库

维护入口已关闭后，停止 API 和前端，避免继续产生新请求；让已发出的任务在限定时间内结束，再停止所有消费者/索引服务。下面的 stop 带超时时间，但仍要检查进程是否被强制杀死；若发生强制终止，先查日志并在隔离环境验证恢复，不能宣称获得干净冷备。

```bash
"${PW_COMPOSE[@]}" stop -t 120 admin app api
# 观察正在执行的任务是否结束；外部 AnyDoc、爬虫、回调和其他写入源也必须停写。
"${PW_COMPOSE[@]}" stop -t 300 consumer raglite
```

确认所有写入者都停止、OSS 无在途写入之后记录 T0：

```bash
date -u +%FT%TZ > "$PW_BACKUP/meta/T0-utc.txt"

# 逻辑备份时 PostgreSQL 仍运行，其他应用写入者已停止。
# exec 不加 -t，避免二进制 dump 被伪终端破坏。
"${PW_COMPOSE[@]}" exec -T postgres sh -ec \
  'pg_dumpall -U "$POSTGRES_USER" --globals-only' \
  > "$PW_BACKUP/db/globals.sql"
"${PW_COMPOSE[@]}" exec -T postgres sh -ec \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "$PW_BACKUP/db/pandawiki.dump"

PW_RAG_DB=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' \
  "$PW_CONTAINER_PREFIX-raglite" | sed -n 's/^DATABASE_POSTGRESQL_DATABASE=//p' | tail -n 1)
test -n "$PW_RAG_DB"
printf '%s\n' "$PW_RAG_DB" > "$PW_BACKUP/meta/raglite-db-name.txt"
"${PW_COMPOSE[@]}" exec -T postgres sh -ec \
  'pg_dump -U "$POSTGRES_USER" -d "$1" -Fc' sh "$PW_RAG_DB" \
  > "$PW_BACKUP/db/raglite.dump"

# 保存数据库目录，发现其他实际数据库时也要备份，不能遗漏自定义库。
"${PW_COMPOSE[@]}" exec -T postgres sh -ec \
  'psql -X -U "$POSTGRES_USER" -d postgres -At -c "SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname"' \
  > "$PW_BACKUP/meta/database-names.txt"

# 当前 Compose 的 Redis 无密码；有密码/ACL 的生产部署须使用其实际认证配置。
"${PW_COMPOSE[@]}" exec -T redis redis-cli SAVE
"${PW_COMPOSE[@]}" exec -T redis redis-cli INFO persistence \
  > "$PW_BACKUP/meta/redis-persistence.txt"
```

数据库备份失败、数据库名不明确、Redis SAVE 报错时，停止流程排查，不能继续把半份备份作为基线。本方案保存 globals 是因为 pg_dump 不包含全局角色等对象。[PostgreSQL pg_dumpall 文档](https://www.postgresql.org/docs/15/app-pg-dumpall.html)

记录基线业务表行数及内容指纹。下例每行先取内容摘要，再排序计算整体摘要，因此能检测“数量相同但内容被改过”；对大型表可改用 DBA 的分批校验。不要只检查文档总数量。

```bash
for PW_TABLE in users departments user_departments permission_levels \
  knowledge_bases navs nodes node_releases system_settings api_tokens; do
  "${PW_COMPOSE[@]}" exec -T postgres sh -ec \
    'psql -X -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"' sh \
    "COPY (SELECT md5(to_jsonb(t)::text) AS h FROM public.$PW_TABLE t ORDER BY h) TO STDOUT" \
    | sha256sum > "$PW_BACKUP/meta/table-$PW_TABLE.sha256"
done
```

表名以当前迁移实际存在为准；业务要求覆盖全库时，把发布关系、管理授权关联表、模型、聊天、上传任务等剩余业务表都纳入同样校验。物理备份仍保存全部数据库，不会因上例只列部分表而少备数据。

## 5. 全部服务停机，保存卷与整机快照

```bash
"${PW_COMPOSE[@]}" stop -t 300
"${PW_COMPOSE[@]}" ps -a > "$PW_BACKUP/meta/compose-stopped.txt"

# 再确认清点出的所有容器都已停止；有 running 容器就停止流程。
for PW_ID in "${PW_CONTAINER_IDS[@]}"; do
  test "$(docker inspect -f '{{.State.Running}}' "$PW_ID")" = false
done

# 保存未挂卷的必要运行文件。先确认路径存在，再按实际情况执行：
# docker cp "$PW_CONTAINER_PREFIX-api:/data" "$PW_BACKUP/runtime/api-data"

while IFS= read -r PW_VOLUME; do
  test -n "$PW_VOLUME"
  docker volume inspect "$PW_VOLUME" > "$PW_BACKUP/meta/volume-$PW_VOLUME.json"
  docker run --rm --network none --entrypoint tar \
    --mount "type=volume,source=$PW_VOLUME,target=/data,readonly" \
    --mount "type=bind,source=$PW_BACKUP/volumes,target=/backup" \
    "$PW_HELPER_IMAGE" --numeric-owner --xattrs --acls --sparse \
    -czpf "/backup/$PW_VOLUME.tar.gz" -C /data .
done < "$PW_BACKUP/meta/volume-names.txt"

# 必须保证备份盘位于项目目录之外。包含 .git、.env、未提交文件、配置和 dist。
tar --numeric-owner --xattrs --acls --sparse \
  --exclude='./.cache' --exclude='./.logs' --exclude='node_modules' \
  -czpf "$PW_BACKUP/source/project.tar.gz" -C "$PW_ROOT" .
```

容器存在额外 bind mount 时，除了当前 Compose 指向项目内的文件，还需备份项目目录外的挂载源。宿主机 Nginx/Caddy、systemd、证书、定时任务及部署时追加的配置也单独归档。Docker 的卷备份/恢复用临时容器挂卷实现；恢复文件所属 UID/GID、权限和符号链接。[Docker 卷备份文档](https://docs.docker.com/engine/storage/volumes/)

此时申请 **系统盘 + 全部数据盘** 的同一组停机快照，保存快照 ID、卷 ID、区域、镜像/操作系统信息及创建完成状态。备份目录可放独立备份盘，快照回滚时不要把唯一备份也回滚或删除。快照创建完成后才能把它算作可用备份。

## 6. 外部 OSS 的固定基线备份

### 6.1 选择恢复粒度

优先使用 OSS 厂商提供的、已经演练过的桶快照/备份与时间点恢复功能，确认其覆盖对象内容、删除、元数据及桶配置。若没有原生能力，使用独立备份桶保存 T0 的所有当前对象，并导出对象与桶属性清单。

开启版本控制可以保护覆盖/删除，但不能自动把整个桶回到 T0，也不能保证测试者无权永久删除历史版本。S3 版本控制保留多个版本；删除标记与对象版本需要正确处理。[S3 版本控制文档](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Versioning.html)

备份桶/前缀必须与生产范围分离、初始为空，并且生产/测试凭证不能写备份。不要把备份放在被测试的生产前缀下；否则镜像复制可能递归，测试或恢复还可能删除唯一副本。

### 6.2 S3 兼容服务的命令模板

下面采用 **AWS CLI V2**，凭证使用预先配置好的备份 profile，并设置实际 region；本项目 S3 SDK 使用 path-style 寻址，兼容 OSS 的 profile 也按厂商要求设置 `s3.addressing_style=path`。不要把密钥直接写进命令。示例假设生产桶与备份桶在同一个兼容端点，且已经验证 CopyObject、multipart copy 和属性复制；跨厂商/跨端点复制要使用厂商备份或经验证的双端 SDK 工具。厂商不支持的 S3 API 不能当成“已经备份”。

```bash
PW_OSS_API=https://oss.example.com
PW_OSS_PROFILE=pandawiki-backup
PW_PROD_BUCKET=your-production-bucket
PW_OSS_PREFIX=static-file/
PW_ARCHIVE_BUCKET=your-backup-bucket
PW_ARCHIVE_PREFIX="pandawiki-baseline/$PW_STAMP/main/"
PW_S3=(aws --no-cli-pager --profile "$PW_OSS_PROFILE" --endpoint-url "$PW_OSS_API")

# 原生分页保持开启；大于 1000 个对象时也必须列全。
"${PW_S3[@]}" s3api list-objects-v2 \
  --bucket "$PW_PROD_BUCKET" --prefix "$PW_OSS_PREFIX" \
  --output json > "$PW_BACKUP/oss/main-T0-objects.json"

# 先确认目标前缀为空，再执行全量复制。
PW_ARCHIVE_COUNT=$("${PW_S3[@]}" s3api list-objects-v2 \
  --bucket "$PW_ARCHIVE_BUCKET" --prefix "$PW_ARCHIVE_PREFIX" \
  --query 'length(Contents || `[]`)' --output text)
test "$PW_ARCHIVE_COUNT" -eq 0

"${PW_S3[@]}" s3 cp "s3://$PW_PROD_BUCKET/$PW_OSS_PREFIX" \
  "s3://$PW_ARCHIVE_BUCKET/$PW_ARCHIVE_PREFIX" \
  --recursive --copy-props default --only-show-errors

"${PW_S3[@]}" s3api list-objects-v2 \
  --bucket "$PW_ARCHIVE_BUCKET" --prefix "$PW_ARCHIVE_PREFIX" \
  --output json > "$PW_BACKUP/oss/main-backup-objects.json"
```

`static-file/` 是本项目主文件存储的常见前缀，**不能据此认定它覆盖全部对象**：先清点 RAGLite 的真实对象路径、独立图片前缀、解析资源及历史路径。专用桶可令 `PW_OSS_PREFIX=''` 备份全桶；共享桶必须逐个备份项目所属前缀，不能默认全桶回退。独立图片 OSS 按相同流程再备份一份。

对象层必须导出/保存：Key、Size、内容校验和、VersionId（已启用版本控制时）、Content-Type、Content-Disposition、Content-Encoding、Cache-Control、自定义 Metadata、Tagging、对象 ACL（服务使用 ACL 时）、加密信息及所需 KMS 密钥引用。存储侧备份凭证只能按已批准范围读取原件、写备份；验证时逐项比较前缀去除后的 Key、大小、内容和上述属性。

AWS CLI 的 `--copy-props default` 可复制常见属性和标签，但不等于复制所有桶策略或对象 ACL。**仅复制文件字节不构成这里定义的完整 OSS 备份**。使用厂商控制台/API 导出 bucket policy、CORS、ACL/Ownership、版本控制、生命周期、加密、通知、域名/CDN、IAM 权限；这些配置的恢复须独立验收。[AWS CLI 属性复制说明](https://docs.aws.amazon.com/cli/latest/userguide/cli-services-s3-commands.html)

如果厂商不能自动导出某些属性，按其控制台/API 保存配置并在演练中验证；未验证的部分在清单中明确标为“未覆盖”，不能宣布全量备份完成。ETag 在分片/加密场景不一定是内容 MD5；用服务提供的可靠 checksum，或下载计算 SHA-256。数据量很大时可全量清单+关键文件校验，但要说明这不是全量内容校验。

## 7. 验证、加密、异机保存和试恢复

```bash
# 验证归档可以读取；这只能证明格式可读，不能替代试恢复。
find "$PW_BACKUP/volumes" -name '*.tar.gz' -exec gzip -t {} \;
gzip -t "$PW_BACKUP/images/baseline-images.tar.gz"
tar -tzf "$PW_BACKUP/source/project.tar.gz" > /dev/null
```

使用 **无生产挂载的工具容器** 验证两份 dump，避免自动启动数据库或接触原卷：

```bash
for PW_DUMP in pandawiki raglite; do
  docker run --rm --network none --entrypoint pg_restore \
    --mount "type=bind,source=$PW_BACKUP/db,target=/backup,readonly" \
    "$PW_HELPER_IMAGE" --list "/backup/$PW_DUMP.dump" \
    > "$PW_BACKUP/meta/$PW_DUMP-dump-list.txt"
done

(
  cd "$PW_BACKUP"
  find . -type f ! -name SHA256SUMS -print0 | sort -z \
    | xargs -0 sha256sum > SHA256SUMS
  sha256sum -c SHA256SUMS > /dev/null
)
```

按组织备份规范使用加密文件系统或加密归档；例如已配置 age 时，将备份包用备份管理员公钥加密，私钥离机保存：

```bash
# 将示例公钥替换为实际备份接收方公钥，不是生产业务密钥。
tar -cf - -C "$PW_BACKUP_ROOT" "$PW_STAMP" \
  | age -r 'age1REPLACE_WITH_BACKUP_RECIPIENT' \
    -o "$PW_BACKUP_ROOT/$PW_STAMP.tar.age"
sha256sum "$PW_BACKUP_ROOT/$PW_STAMP.tar.age" \
  > "$PW_BACKUP_ROOT/$PW_STAMP.tar.age.sha256"
```

复制加密包、校验文件、快照标识与 OSS 备份标识到离机备份位置，并测试解密密钥可用。不得仅在生产系统上留唯一一份。

在隔离环境做一次完整恢复，记录：恢复用时、数据库启动、两库内容校验、OSS 对象/元数据、Qdrant 集合/点数/别名、NATS 流/消费者、登录和权限、预览/下载/问答结果。只有演练通过，才开始安全测试。RTO 以实测时间为准，不能在不了解数据大小时承诺“几分钟恢复”。

基线确认完成后，按第 9 节启动顺序恢复运行以供测试；撤销临时 OSS 停写策略，仅开放已约定的测试入口。正常用户和非测试写入源仍暂停。不要覆盖或重新生成 T0 备份。

## 8. 测试结束：再次冻结并保留证据

1. 关闭测试入口，停止测试客户端、扫描器、回调、爬虫及所有外部写入。
2. 同样处理仍有效的 OSS 预签名上传，并冻结项目 OSS 写入。
3. 停止前后端、Consumer、RAGLite 和基础服务。
4. **先**导出测试报告、发现的漏洞、测试期间全部日志、恶意文件样本清单及必要现场快照，保存在基线之外的受保护位置。
5. 不把测试结束时的数据作为“恢复备份”覆盖 T0。可保存一份清楚标注的“测试结束现场”，但恢复源必须仍是 T0。

正常用户在 T0 后误产生的业务写入也会被撤销，这是本次明确的恢复要求。无法通过数据库回滚撤销已发送的邮件/机器人消息、外部接口调用、模型调用计费或对其他系统的修改；测试期间需隔离这些外部副作用。

## 9. 整体恢复的执行顺序

### 9.1 选择恢复路径

**优先：将 T0 的整机/全盘快照恢复到干净主机或替换原主机，然后独立恢复 OSS。** 恢复主机启动前隔离入口，禁止容器随 Docker 自动启动；使用云平台离线挂载/救援模式屏蔽自动服务，或保证网络与外部存储写入已被隔离。`restart: unless-stopped` 容器、外部消息回调都要纳入检查。

如果测试发现主机入侵/容器逃逸，不要仅导回数据库后继续使用可能受污染的系统。用可信操作系统和 T0 归档重建，并修复验证过的漏洞。失窃的凭证不会因为“恢复旧配置”而重新保密，需要另行轮换。

**备选：在同一可信主机上重建 T0 配置和镜像，恢复到新的空卷，再切换 Compose。** 保留测试卷直到证据保存、恢复验收和保留期结束，避免不可逆覆盖。

### 9.2 校验恢复源，恢复源码与镜像

先解密备份到隔离的 root-only 目录，对 `SHA256SUMS` 全量校验。不要向被测目录直接覆盖解压：把当前项目目录改名留证，再在原路径建立空目录，解压 `source/project.tar.gz`，保留 UID/GID/权限。绝对路径 bind mount 要恢复到原路径或逐项调整。

```bash
# 进入已解密的 T0 备份目录，重新设置上述 PW_* 变量，不能沿用测试修改后的 .env。
PW_BACKUP=/srv/pandawiki-backups/REPLACE_WITH_T0_STAMP
PW_BACKUP_ROOT=/srv/pandawiki-backups
PW_PROJECT=$(cat "$PW_BACKUP/meta/compose-project-name.txt")
PW_ROOT=$(cat "$PW_BACKUP/meta/project-path.txt")
PW_CONTAINER_PREFIX=$(cat "$PW_BACKUP/meta/container-prefix.txt")
(
  cd "$PW_BACKUP"
  sha256sum -c SHA256SUMS > /dev/null
)
gunzip -c "$PW_BACKUP/images/baseline-images.tar.gz" | docker image load

# 当前项目已停止，且源代码与配置备份已校验；保留测试结束现场目录。
PW_TEST_END_PROJECT="${PW_ROOT}.test-end-$(date -u +%Y%m%dT%H%M%SZ)"
test -d "$PW_ROOT"
test ! -e "$PW_TEST_END_PROJECT"
mv "$PW_ROOT" "$PW_TEST_END_PROJECT"
install -d -m 700 "$PW_ROOT"
tar --numeric-owner --xattrs --acls \
  -xzpf "$PW_BACKUP/source/project.tar.gz" -C "$PW_ROOT"
```

恢复优先使用冻结的 resolved Compose 与 `images.override.yml`，强制使用基线镜像。不要在恢复过程中升级软件、重新构建前端或运行 `deploy.sh fresh-db`；API 镜像启动会先执行迁移，必须与 T0 的数据库版本匹配。

### 9.3 恢复到新空卷

逐个原卷创建唯一的新卷，例如 `pw-recovery-20261008T120000Z-0`，保存 old → new 对应表。所有原数据卷的生产服务都必须保持停止。以下循环不清空原卷：

```bash
PW_RECOVERY_STAMP=$(date -u +%Y%m%dT%H%M%SZ)
PW_HELPER_IMAGE=$(cat "$PW_BACKUP/meta/helper-image.txt")
PW_INDEX=0
: > "$PW_BACKUP_ROOT/recovery-$PW_RECOVERY_STAMP-volumes.tsv"
while IFS= read -r PW_OLD_VOLUME; do
  test -n "$PW_OLD_VOLUME"
  PW_NEW_VOLUME="pw-recovery-${PW_RECOVERY_STAMP,,}-$PW_INDEX"
  # 目标必须不存在，避免把旧目录当作空卷覆盖。
  if docker volume inspect "$PW_NEW_VOLUME" >/dev/null 2>&1; then
    echo "恢复目标卷已存在，停止：$PW_NEW_VOLUME" >&2
    exit 1
  fi
  docker volume create "$PW_NEW_VOLUME" > /dev/null
  docker run --rm --network none --entrypoint tar \
    --mount "type=volume,source=$PW_NEW_VOLUME,target=/data" \
    --mount "type=bind,source=$PW_BACKUP/volumes,target=/backup,readonly" \
    "$PW_HELPER_IMAGE" --numeric-owner --xattrs --acls \
    -xzpf "/backup/$PW_OLD_VOLUME.tar.gz" -C /data
  printf '%s\t%s\n' "$PW_OLD_VOLUME" "$PW_NEW_VOLUME" \
    >> "$PW_BACKUP_ROOT/recovery-$PW_RECOVERY_STAMP-volumes.tsv"
  PW_INDEX=$((PW_INDEX + 1))
done < "$PW_BACKUP/meta/volume-names.txt"
```

修改恢复 Compose 的顶层卷定义，把逻辑卷映射为新卷：

```yaml
volumes:
  pandawiki_postgres:
    external: true
    name: REPLACE_WITH_NEW_POSTGRES_VOLUME
  pandawiki_redis:
    external: true
    name: REPLACE_WITH_NEW_REDIS_VOLUME
  pandawiki_nats:
    external: true
    name: REPLACE_WITH_NEW_NATS_VOLUME
  pandawiki_qdrant:
    external: true
    name: REPLACE_WITH_NEW_QDRANT_VOLUME
  pandawiki_raglite:
    external: true
    name: REPLACE_WITH_NEW_RAGLITE_VOLUME
  pandawiki_loki:
    external: true
    name: REPLACE_WITH_NEW_LOKI_VOLUME
  pandawiki_alloy:
    external: true
    name: REPLACE_WITH_NEW_ALLOY_VOLUME
```

根据实际 resolved Compose 补齐其他卷，包括必要时的 pnpm 缓存、额外服务及挂载。若某逻辑卷没有容器挂载记录，先依据实际 volume inspect/Compose 定义清点后再恢复。

项目还有一个恢复时必须处理的启动行为：`ADMIN_PASSWORD` 非空时，启动 API 会重设 admin 密码。为保留 T0 数据库中真实的管理员密码，恢复配置要把 API 的 `ADMIN_PASSWORD` 设为空，不重新初始化账号：

```yaml
services:
  api:
    environment:
      ADMIN_PASSWORD: ""
```

这项 override 应保留到后续启动配置中；否则以后重启仍可能覆盖恢复的密码。如果确认需要轮换管理员密码，则作为恢复后的独立安全操作执行，不与基线校验混在一起。检查其他环境秘密、OSS/IAM 和模型配置是否也被测试修改，按基线恢复；泄漏过的秘密再按安全流程轮换。

### 9.4 OSS 恢复：必须处理三类变化

| 测试造成的变化   | 恢复动作                                                             |
| ---------------- | -------------------------------------------------------------------- |
| 测试删除了原文件 | 从 T0 备份恢复原 Key 的内容与属性                                    |
| 测试覆盖了原文件 | 强制用 T0 内容覆盖，不能因文件大小相同而跳过                         |
| 测试新增了文件   | 清除生产恢复范围内 T0 清单没有的 Key；数据库回滚不会自动删除这些对象 |

推荐 OSS 原生时间点恢复；否则对同一目标桶/前缀执行**全量强制复制 + 额外对象清理 + 属性/策略恢复**。文件 URL 可能嵌在文档内容或品牌配置中，保持桶名、Key、下载域名/路径；改新桶时必须验证地址映射，不能只改环境变量后认为完成。

以下模板只适用于已经核对过的专属恢复范围。共享桶不能在根目录使用 `--delete`：

```bash
# 先核对备份源及生产目标，确认方向；--recursive cp 强制恢复每个基线对象。
"${PW_S3[@]}" s3 cp "s3://$PW_ARCHIVE_BUCKET/$PW_ARCHIVE_PREFIX" \
  "s3://$PW_PROD_BUCKET/$PW_OSS_PREFIX" \
  --recursive --copy-props default --only-show-errors

# 先预览，再根据逐 Key 清单审核其删除范围；不得把预览当成已经删除。
"${PW_S3[@]}" s3 sync "s3://$PW_ARCHIVE_BUCKET/$PW_ARCHIVE_PREFIX" \
  "s3://$PW_PROD_BUCKET/$PW_OSS_PREFIX" \
  --delete --dryrun > "$PW_BACKUP_ROOT/recovery-$PW_RECOVERY_STAMP-oss-dryrun.txt"

# 确认新增对象清单仅属于本次项目恢复范围后，才执行清理。
"${PW_S3[@]}" s3 sync "s3://$PW_ARCHIVE_BUCKET/$PW_ARCHIVE_PREFIX" \
  "s3://$PW_PROD_BUCKET/$PW_OSS_PREFIX" \
  --delete --copy-props default --only-show-errors
```

全量 cp 之后，再根据备份清单恢复对象 ACL 和独立导出的桶策略/CORS/生命周期/加密等配置。图片与原文档的权限可能不同，不能统一改为公开。厂商兼容性不满足时使用其原生恢复工具替代命令，不要省略属性恢复。

版本化桶里的 `--delete` 通常只是创建删除标记，**测试新增的历史版本/覆盖版本仍可能存在**。若要求测试数据在存储层也不存在，必须在独立保存证据后，用已审核的 T0 VersionId/删除标记清单仅删除测试产生的版本；不能删除 T0 的历史版本。对象复制会产生新的 LastModified/VersionId，不是物理还原这些属性；需要精确恢复版本历史时只能选支持该能力的原生备份/恢复或厂商方案。

无版本控制时，基线强制 cp 与范围内额外 Key 清理可以恢复 T0 当前对象集合。`sync` 默认按大小和时间判断，不能单独用它证明同大小被覆盖文件已恢复；因此先强制 cp。AWS CLI 对 `--delete` 的含义是删除目标侧源不存在的对象。[AWS CLI sync 文档](https://docs.aws.amazon.com/cli/latest/reference/s3/sync.html)

旧 multipart 上传会话不属于普通对象备份。恢复前审核并终止测试创建的上传会话，必要时重新上传 T0 前未完成的业务上传；不能声称仅恢复数据库就会恢复旧 UploadID。也要清理/失效 CDN 和浏览器缓存，避免仍显示测试时的图片或文件。

### 9.5 启动顺序与检查点

把恢复后的卷定义和 ADMIN_PASSWORD override 放入一个实际的 `recovery.override.yml`，使用明确指定的 Compose 文件：

```bash
PW_RECOVERY_OVERRIDE=/secure/recovery/recovery.override.yml
PW_RECOVER=(docker compose -p "$PW_PROJECT" \
  -f "$PW_BACKUP/meta/compose.resolved.yml" \
  -f "$PW_BACKUP/meta/images.override.yml" \
  -f "$PW_RECOVERY_OVERRIDE")

# resolved 文件可能含明文秘密，保留在 root-only 目录。
"${PW_RECOVER[@]}" config > "$PW_BACKUP_ROOT/recovery-$PW_RECOVERY_STAMP-compose.yml"

# 不拉新镜像、不重新构建、不自动运行依赖构建任务。
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never postgres redis nats qdrant
```

检查 PostgreSQL 两库可读、内容指纹与 T0 一致，Redis 持久化可加载，Qdrant 集合/别名/点数正确，NATS 没有测试期间的流/消息/消费者状态。**在 API/Consumer/RAGLite 启动前做持久业务数据校验**，避免启动后正常日志、访问时间或任务处理影响比对。

```bash
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never loki alloy preview
# OSS 已恢复、基线校验已通过后，才允许索引后台连接恢复的队列。
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never raglite
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never api
```

如果保存了 API `/data` 等非挂卷文件，先使用 `create --no-deps --no-build --pull never api` 创建停机容器，`docker cp` 将文件恢复到原路径、确认权限，再 `start api`，替代上面直接启动 API 的命令。其他可写层必要文件同样处理。

API 可启动后，先核对主库与队列的基线任务。若 T0 存在积压/超时任务，恢复后仍是这些任务，不能混入测试消息；经过当前时间后某些任务可能触发超时，按记录处理。不要同时恢复队列又重复手动提交全量学习任务。

```bash
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never consumer
"${PW_RECOVER[@]}" up -d --no-deps --no-build --pull never admin app
"${PW_RECOVER[@]}" ps
"${PW_RECOVER[@]}" logs --tail 100 api consumer raglite
```

整个启动期间入口保持关闭，仅允许运维验收。OSS 已恢复并通过校验后，先允许可信恢复服务所需的内部 OSS 写入，继续阻断测试客户端和其他写入源，避免 RAGLite 启动后因全局写入禁令处理任务失败。验收全部通过，再开放用户入口、恢复外部集成和定时任务。已经恢复完整 RAGLite/Qdrant 状态时，先验证已有检索，不默认重建索引。Qdrant 快照包含集合数据和配置，全集快照支持别名，但只适用单节点；当前方案使用同版本完整冷备卷。NATS 恢复还必须覆盖消费状态，不能只备一份应用数据库。[Qdrant 快照文档](https://qdrant.tech/documentation/operations/snapshots/)，[NATS 备份与恢复文档](https://docs.nats.io/learn/backup-recovery/)

若选择逻辑数据库备份作为恢复路径，由 DBA 在空集群恢复 globals、创建对应数据库、执行 `pg_restore --exit-on-error` 并检查所有错误。启动初始化角色可能与 globals 中角色重名，需要提前设计/演练 bootstrap 角色；不能盲目忽略“角色已存在”或只追加导入旧数据。不要在原测试库上直接 restore 混合数据。物理卷恢复与逻辑恢复二选一，不能在恢复的物理集群上再叠加导入 dump。

## 10. 恢复验收与放行标准

恢复检查至少包括：

- [ ] T0、整机快照、所有卷归档、逻辑数据库、镜像、配置和 OSS 基线标识一致，全部校验通过。
- [ ] 两个 PostgreSQL 数据库恢复；全部关键业务表数量/内容指纹匹配；测试用户、文档、栏目、权限、模型设置和恶意 SQL 修改均不残留。
- [ ] 测试前已有文档的正文、附件、图片及被测试删除的文件恢复；测试新增的文件/恶意上传已从业务访问范围清除；要求彻底删除时额外清理测试版本和 multipart。
- [ ] OSS 对象内容、属性、ACL、桶策略、CORS、下载路径与基线一致；不存在文件误公开或不同系统共享对象被删除。
- [ ] Qdrant 集合、点数、别名及 RAGLite 文档/任务关联恢复；抽查问答及引用，不依赖重新学习掩盖恢复遗漏。
- [ ] NATS 没有测试期间产生的待执行消息；基线遗留任务已按记录审核，后台启动没有重复导入或批量删除。
- [ ] 管理员/超级管理员可登录 admin 端，普通用户不可登录后台；普通用户在用户端按等级、部门、指定用户规则查阅，直接 URL 和问答引用同样不越权。
- [ ] 一级栏目、文件夹、文档的查阅/管理配置恢复；测试授予的权限和 API Token 不残留。
- [ ] 模型调用、文件预览、原件下载、编辑和发布正常；不为验收执行全量重建索引。
- [ ] 镜像版本、配置、环境、反向代理、证书、端口和外部 IAM 已核对；漏洞修复作为独立、验证后的变更应用。
- [ ] 测试报告与审计证据已离机保留，T0 备份仍受保护且未覆盖。
- [ ] 用户入口开放和外部集成恢复经过运维与业务验收；恢复过程时间、负责人和问题有记录。

恢复持久业务数据并不能倒转现实时间：JWT/签名 URL/证书/Redis TTL 会随时间到期，用户可能需要重新登录；版本化 OSS 的版本历史和已发出的外部消息有独立恢复边界。对这类边界提前记录，不把正常过期误判为测试数据残留。[Redis 持久化文档](https://redis.io/docs/latest/management/persistence/)

## 11. 禁止操作与保留策略

- 不执行 `docker compose down -v`、`docker volume prune` 或 `deploy.sh fresh-db` 来“准备恢复”；这些会删除恢复所需数据或证据。
- 不在服务运行时 tar PostgreSQL/Qdrant/NATS 的原始数据目录，且不把不同时间点的备份拼在一起。
- 不把“复制 PostgreSQL 数据”“只导出 nodes”“只保存 Git commit”当成全项目备份。
- 不仅恢复数据库而保留测试的 OSS 对象、向量库或 NATS 队列。
- 不在共享桶根目录镜像删除，不用测试凭证保管或更新基线备份。
- 不把测试期间的日志与报告随业务整体回退一起抹掉。

建议 T0 基线至少保留到测试验收和漏洞修复复测完成，之后按组织保留策略处理；日常另保留定期数据库/OSS/索引备份。此次 T0 基线不可滚动覆盖。整机快照不是离机独立备份的替代品，复制/副本也不能替代能回到固定时间点的备份。

交付给运维的清单应包含：T0 时间、项目/容器实际名称、两个数据库名、实际卷映射、镜像标签/摘要、配置归档、整机快照 ID、OSS 原/备份桶与前缀、对象/属性/版本清单、SHA256SUMS、加密密钥保管人、已验证恢复步骤、实测恢复时间、验收结果以及外部副作用/过期状态等已知边界。
