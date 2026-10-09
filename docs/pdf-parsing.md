# PDF 正文提取与 OCR

PDF 的实际处理由独立的 `pdf-parser` 容器完成，API/Consumer 不再在业务进程内执行 PDF 解析库。镜像包含中文 CMap 资源和 Noto CJK 替代字体，用于兼容部分依赖外部字符映射或未嵌入字体的中文 PDF。生产环境仅通过 Compose 内部网络访问，不映射服务端口。本地开发绑定 `127.0.0.1:18090`。

## 处理顺序

1. 从已有 OSS 下载到临时文件，上传给解析服务。
2. Poppler 获取页数并提取现有文字。
3. 结构读取或文字提取失败时，qpdf 生成修复副本，再提取文字。qpdf 返回 3 表示有警告，仍尝试读取输出；原始 OSS 文件不修改。
4. 对缺少有效文字或明显存在替换字符的页面，逐页渲染并由 Tesseract 使用 `chi_sim+eng` 识别；默认识别结果不足时尝试连续文本模式，两次识别共用每页 60 秒预算。保留已有文字的页面。
5. 返回正文及覆盖页数、OCR 页数、结构修复记录、警告。全部失败时，只有显式配置且可访问的 crawler 才作为备用；否则索引文件名称和元信息。

加密文件未提供密码时可能无法解析；空白页、纯图片页或过少文字会保守计为未取得有效正文。完整正文表示各页通过有效文字检查，不保证 OCR 字词、表格或多栏阅读顺序完全正确。人工核验应包含业务使用的真实样本。

MP4、音频、图片和归档文件继续只索引名称、路径、类型，不进入 PDF 服务。

## 进度与结果

进度包含等待解析名额、提取文字、结构修复、OCR 当前页/总页数、解析完成或失败。原有开始、结束、最长步骤时间仍保留。

节点 `rag_info.learning_progress.extraction` 和发布版本 `meta.extraction` 保存实际内容范围：

- `full`：各页均取得有效文字。
- `partial`：部分页面未提取到有效文字或正文被截断。
- `metadata`：只索引名称和文件元信息，正文不能用于问答。
- `unknown`：备用 crawler 已返回正文，但页覆盖范围未核验。

存储使用现有 JSONB 字段，不新增数据库迁移。旧 PDF 缓存正文首次执行新流程时重新提取；仅元信息和部分正文在后续重建时也会重新尝试。完整提取结果可复用。

## 限制与清理

默认每个容器 1 个工作线程、最多 2 个额外排队任务；服务忙时后端等待名额，不立即降级为元信息。单文件最大 200 MB、500 页、正文最大 20 MB。每个命令独立进程，限制虚拟内存 1 GB、输出文件 64 MB、文件描述符 256；容器内存上限 2 GB、进程上限 128。

文字提取最多 120 秒，结构修复 60 秒，每页渲染 45 秒、OCR 60 秒；每个任务总预算默认 900 秒（含服务内排队）。业务端 PDF 总预算 20 分钟（含下载、上传、等待服务名额）。预算耗尽时可保留已经成功提取的页面，并明确显示部分正文；取消任务时终止解析进程组。

Consumer 在执行期间每 10 秒向 JetStream 发送处理续约，成功后才确认消息；失败消息保留重试机会。每 2 秒检查任务是否已取消或失败，并取消对应的远端解析请求。原有知识库并发名额和队列顺序仍生效，扫描件仍需实际 OCR 计算时间。

后端取得结果后 DELETE 任务，清理临时 PDF、图片和正文。客户端丢失时，终态结果在总预算加 5 分钟后由后续请求清理，最多保留约 16 个结果；容器重建后任务不保留，索引任务需重试。服务不写入 OSS。

## 生产部署（Docker Compose 1.29.2）

将本次后端、服务目录、管理端及部署脚本更新到服务器。建议先暂停提交新索引任务，按既有方案备份。

如使用本次提供的 `pandawiki-pdf-parser-update.patch`，上传到服务器 `/tmp` 后先检查，再应用：

```bash
cd /opt/WikiCode/PandaWiki
git apply --check /tmp/pandawiki-pdf-parser-update.patch
git apply /tmp/pandawiki-pdf-parser-update.patch
```

只有检查成功才执行应用；若已应用之前的 PDF 防崩溃补丁或服务器代码有其他差异，检查可能冲突，应先核对差异，不能强制覆盖。补丁不包含真实 `.env`、密钥或智能问答等其他改动。

在 `deploy/server/.env` 可配置以下非必填项：

```dotenv
PDF_PARSER_BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/misaka-images/python:3.12-slim-bookworm
PDF_PARSER_APT_MIRROR=https://deb.debian.org
PDF_PARSER_TOKEN=
PDF_PARSER_WORKERS=1
PDF_PARSER_MAX_PAGES=500
PDF_PARSER_JOB_TIMEOUT_SECONDS=900
ANYDOC_CRAWLER_BASE_URL=
```

建议用独立随机字符串设置 `PDF_PARSER_TOKEN`；部署会向 API、Consumer 和解析服务传入相同值。未设置时仅依赖不发布端口的内部网络隔离。已有 crawler 可填其内部地址；未部署则留空。基础镜像默认使用已同步的阿里云 `misaka-images/python:3.12-slim-bookworm`；私有仓库需先在构建服务器上执行 `docker login registry.cn-hangzhou.aliyuncs.com`。已有 `.env` 若仍设置为 Docker Hub 地址，需要手动更新，修改模板不会覆盖已有配置。首次构建还需要访问 Debian 软件包源，可通过 `PDF_PARSER_APT_MIRROR` 指定组织可访问且提供 Debian 和 Debian Security 的镜像站根地址。

```bash
cd /opt/WikiCode/PandaWiki/deploy/server
bash deploy.sh upgrade backend
bash deploy.sh upgrade admin
```

`upgrade backend` 现在包含 pdf-parser、API、Consumer，管理端需单独升级才能显示新阶段和覆盖范围。部署脚本继续读取项目根 `.env` 的 OSS 配置。不要执行 fresh-db 或删除数据卷。

升级会先保留旧容器运行并构建全部目标镜像；构建失败时不停止旧服务。切换失败时仅恢复升级前已存在的服务，使用旧镜像且禁止隐式构建；回滚启动或健康检查失败时保留回滚镜像标签供人工恢复。镜像回滚不会撤销数据库迁移或业务写入，不代替数据备份。

```bash
docker inspect --format '{{.State.Health.Status}}' pandawiki-pdf-parser
docker logs --since 10m pandawiki-pdf-parser
docker logs --since 10m pandawiki-consumer
```

确认 PDF 服务健康、消费者重启次数不再增长后，先选择正常文字 PDF、此前失败的 PDF、扫描件各一份重建，检查正文覆盖和内容质量，再批量重建。此前已经失败/超时的任务需要从页面重新发起。

## 本地开发与验证

```bash
./scripts/dev-deps.sh
./scripts/dev-api.sh
./scripts/dev-consumer.sh
./scripts/dev-web.sh
```

如设置了解析服务 token，本机 API/Consumer 脚本与 Docker Compose 必须从同一环境取得该变量。

```bash
cd backend
go test ./mq/nats ./usecase ./domain ./handler/mq ./repo/pg ./cmd/api ./cmd/consumer
```

纯逻辑测试：

```bash
cd services/pdf-parser
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest -v test_server.py
```

容器真实工具测试（挂载测试代码，不处理生产文档）：

```bash
docker run --rm --entrypoint python \
  -e PDF_PARSER_REAL_TOOLS=1 -e PYTHONDONTWRITEBYTECODE=1 \
  -v "$PWD/services/pdf-parser:/tests:ro" -w /tests \
  pandawiki-pdf-parser -m unittest -v test_server.py
```

上例从项目根目录执行。覆盖正常 PDF、对象索引错位、qpdf 修复、中文文字提取、中英文图片型 PDF 的实际 OCR，以及语言包检查。Go 测试覆盖接口认证、服务限流重试、进度传播、取消后清理、解析失败与缓存重新提取。

设置 `PANDA_WIKI_TEST_PDF_PARSER_URL=http://127.0.0.1:18090` 可验证 Go 到真实解析容器；设置 `PANDA_WIKI_TEST_NATS_URL=nats://127.0.0.1:4222` 可验证消息续约和失败重试；设置 `PANDA_WIKI_TEST_PG_DSN` 可验证数据库取消状态联动与覆盖范围持久化。集成测试使用合成文档、独立临时消息流和回滚事务，不应指向生产数据库或消息服务。
