# 安全测试报告逐项处理清单

> 2026-10-26 补充修复已完成：两端 HttpOnly 会话、普通文档私有上传策略、服务指纹头过滤、Gotenberg 转换及 NATS 监控认证。详见[安全补充修复与部署说明](安全补充修复-20261026.md)。原报告本地已修复跟踪项现为 **33 项**；本文较早表格/说明保留历史口径，最新逐项状态见 JSON 与补充说明，生产仍未验证。

> 2026-10-26 最新汇总见[安全整改状态与协助清单](安全整改状态与协助清单-20261026.md)：区分已接受的本地修复、后续代码工作、运维与测试方待办，包含最新强制登录改动。本文保留原日期的详细记录。

2026-10-26 新进展：用户端已改为强制登录，目录、文档、下载、搜索、网页及嵌入式问答、历史会话均拒绝匿名请求，原文中 P0/P7 匿名可读是调整前记录。这里的原条目统计保留原核查口径；新增实现与 32 项本地 HTTP 验证见[用户端强制登录](用户端强制登录-20261026.md)。生产状态仍未验证，外部 OSS 公开直链仍需单独核实。

整理日期：2026-10-23。依据两份原始 PDF 的问题顺序、编号和页码，对照 2026-10-22 的本地代码核查、修复及验证记录整理。本次仅整理资料，没有重新测试漏洞或连接生产环境。

**“本地已修复”表示确认的代码/配置缺口已改且有本地证据，不代表生产已部署或测试机构已验收。生产状态对所有条目统一为“未验证”。** “部分处理”表示还有明确缺口或利用样本未复测；“尚未修复”表示相应措施未实施；“待核实”表示现有证据不足、疑似误报或当前条件不适用，不能当作已修复。

**2026-10-26 验收口径更新：你已接受下列 29 项已有本地修复结论，无需你重复验证。** 本清单保留原验收步骤作为参考，后续仅推进部分处理、尚未修复和待核实事项；部署状态及测试机构的正式销项状态分别记录，不因本次接受结论而变为“生产已修复”。

## 怎么使用这份清单

29 项本地已修复项无需你重复验证；其中原验收操作保留为参考。其余条目按“你需要做什么”继续推进；需要实际部署时，先完成“部署前共同事项”。部署版本及测试方销项结论单独记录。本文没有修改应用代码，也没有代你完成正式部署。

- 第一部分按管理端 PDF 列出 **22 个跟踪子项 M01–M22**；登录/新增/重置、密钥创建/列表、PDF 两个入口分别列出。
- 第二部分按用户端 PDF 列出 **19 个跟踪项 U01–U19**；五个上传入口分别列出。CORS 的两个接口和 Cookie 的五个脚本都在对应条目明确列出。
- 第三部分列出两份 PDF 共有的 **44 个主机条目 H01–H44**，每项保留两份报告各自的原编号和页码。编号顺序不同处已单独核对。
- M/U/H 是本清单跟踪编号，原报告位置另列，旧核查的 A01–A23 根因编号也保留以便查证。共 85 个跟踪项，不是 85 个已证实漏洞；报告中的重复 CVE、信息项、未成立项均不计作修复成果。

用户端 PDF 统计口径存在不一致：第 5 页 CORS 数量为 2、Cookie 数量为 5，但低危/参考合计分别按 3/2 汇总；本文按详情问题及上传入口组织 19 项，同时保留所有列出的 URL/脚本，不把统计差异虚构成新增漏洞。请测试方统一最终计数。

### 当前进度速览

| 范围 | 本地已修复 | 部分处理 | 尚未修复 | 待核实 / 不适用 / 疑似误报 / 重复 | 合计 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 管理端应用 | 12 | 4 | 3 | 3 | 22 |
| 用户端应用 | 15 | 1 | 1 | 2 | 19 |
| 两份报告共有主机条目 | 2 | 2 | 0 | 40 | 44 |

主机表“尚未修复”为 0 不表示所有主机漏洞已解决：40 项中包含大量尚未确认补丁状态的 CVE、条件不符和疑似误报。未确定真实漏洞前，不把它们计作“已修复”或确定的“未修复漏洞”。原报告同一 CVE 的重复项仍占一行。

明确尚未实施的措施为：**M01–M03 的实际入口 HTTPS、U19 的 HttpOnly 身份 Cookie 改造**。部分处理为：**M06–M07 的 PDF 原样本闭环、M08 的内部 IP 命中复核、M18 的指纹头完全移除、U03 的 Gotenberg 完整鉴权/转换验证、H31 的监控鉴权、H38 的主机各 HTTP 服务指纹**。其余待核实项的材料要求与操作见对应条目。

### 你优先需要处理的事项

1. **部署本地修复**：先备份，再整套升级；仅更新 API 不足以同步会话、前端和 Redis/NATS 凭据。
2. **完成实际入口 HTTPS**：M01–M03 尚未完成；管理端、用户端及敏感 API 都要检查。
3. **核对 OSS 与历史文件**：U02 的直链/ACL，以及 M06–M07 的原恶意 PDF、旧对象，需要你或测试方提供证据与复测。
4. **确认 Cookie 验收要求**：U19 的身份 Cookie 仍未改成 HttpOnly，需要继续开发；M22 先请测试方明确命中 Cookie 的名称与用途，避免与用户端实现混淆。
5. **请运维补齐主机证据**：Redis/Nginx 的确切构建、Ubuntu OpenSSH 完整包版本和运行配置；请测试方复核疑似误报、客户端漏洞与重复项。

## 部署前共同事项

这些是各条目“部署后复测”的前提，详细步骤见[修复与部署说明](修复与部署说明.md)。

1. 暂停写入，备份 PostgreSQL、持久化数据、配置及外部 OSS 对象版本/副本，并先确认能恢复。测试期间的新增/修改需随整体恢复回到测试前状态，不能只恢复数据库而保留被改动的 OSS 对象。
2. 在 deploy/server/.env 分别设置 REDIS_PASSWORD、NATS_PASSWORD，可分别用 `openssl rand -hex 24` 生成；不要贴出真实值。按实际域名设置 SECURITY_ALLOWED_HOSTS，需要独立跨源 API 时再设置 SECURITY_ALLOWED_ORIGINS。
3. 首次安全升级使用 `bash deploy/server/deploy.sh upgrade all`，同步基础设施、API/Consumer 及两端。项目脚本兼容当前 Docker Compose V1 使用方式，勿直接输出完整配置泄露凭据。
4. 数据库迁移 56 会把历史 MCP 密钥转为不可逆哈希；旧密钥仍能验证，旧账号 JWT 需重新登录，既有密码账号也会提示改密。跨迁移回退需恢复升级前数据库和配套版本/配置，不能只换回旧 API 镜像。
5. 运维在最终访问入口复测，而非只看容器内响应。新模板收紧端口后，仍需核对防火墙、宿主监听和其他反向代理是否存在旁路。

## 管理端报告：应用问题

### M01｜明文传输：登录

- **原报告位置**：第 6–7 页；2.2.1 高危，问题 1/3，登录接口。旧核查根因编号：A01。
- **涉及入口**：/api/v1/user/login。
- **处理状态：尚未修复。** 有条件存在。
- **核实结果 / 为什么未标已修复**：本地用 HTTP 访问 API/两端，部署模板监听 HTTP；没有测试真实密码传输，不能据此确认正式入口是否有 HTTPS 网关。
- **尚未完成 / 结论边界**：尚未为实际部署入口完成 HTTPS。密码强度等代码修复不能替代传输加密。
- **你需要做什么**：由运维为实际管理端和用户端入口配置有效 HTTPS，HTTP 统一跳转 HTTPS，并收紧可绕过网关的后端端口。分别在浏览器 Network 检查登录、新增、重置请求确实走 HTTPS；只给首页加 HTTPS 不算完成。证据应保留协议和跳转结果，不记录密码。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf:7)；[deploy/server/docker-compose.yml](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml:25)。

### M02｜明文传输：重置密码

- **原报告位置**：第 6–7 页；2.2.1 高危，问题 1/3，重置密码接口。旧核查根因编号：A01。
- **涉及入口**：/api/v1/user/reset_password。
- **处理状态：尚未修复。** 有条件存在。
- **核实结果 / 为什么未标已修复**：本地用 HTTP 访问 API/两端，部署模板监听 HTTP；没有测试真实密码传输，不能据此确认正式入口是否有 HTTPS 网关。
- **尚未完成 / 结论边界**：尚未为实际部署入口完成 HTTPS。密码强度等代码修复不能替代传输加密。
- **你需要做什么**：由运维为实际管理端和用户端入口配置有效 HTTPS，HTTP 统一跳转 HTTPS，并收紧可绕过网关的后端端口。分别在浏览器 Network 检查登录、新增、重置请求确实走 HTTPS；只给首页加 HTTPS 不算完成。证据应保留协议和跳转结果，不记录密码。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf:7)；[deploy/server/docker-compose.yml](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml:25)。

### M03｜明文传输：新增用户

- **原报告位置**：第 6–7 页；2.2.1 高危，问题 1/3，新增用户接口。旧核查根因编号：A01。
- **涉及入口**：/api/v1/user/create。
- **处理状态：尚未修复。** 有条件存在。
- **核实结果 / 为什么未标已修复**：本地用 HTTP 访问 API/两端，部署模板监听 HTTP；没有测试真实密码传输，不能据此确认正式入口是否有 HTTPS 网关。
- **尚未完成 / 结论边界**：尚未为实际部署入口完成 HTTPS。密码强度等代码修复不能替代传输加密。
- **你需要做什么**：由运维为实际管理端和用户端入口配置有效 HTTPS，HTTP 统一跳转 HTTPS，并收紧可绕过网关的后端端口。分别在浏览器 Network 检查登录、新增、重置请求确实走 HTTPS；只给首页加 HTTPS 不算完成。证据应保留协议和跳转结果，不记录密码。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf:7)；[deploy/server/docker-compose.yml](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml:25)。

### M04｜敏感信息泄露：创建 MCP 密钥

- **原报告位置**：第 8 页；2.2.1 高危，问题 2/3，创建接口。旧核查根因编号：A02。
- **涉及入口**：/api/pro/v1/token/create。
- **处理状态：本地已修复。** 已修复；本地接口/数据库验证通过。
- **已经怎么修复/处理**：创建 MCP 密钥：完整值只在创建成功时一次性展示，关闭后不再取回。服务端保存 SHA-256，禁止把哈希当可复用 Token 返回；历史明文由迁移 56 转换，原密钥仍可认证。
- **尚未完成 / 结论边界**：生产未部署；创建时一次返回完整密钥是取用凭据所需的预期行为，不应要求“永不返回”。
- **保留验收参考（无需你重复执行）**：部署后端和管理端并执行迁移 56。创建密钥后仅允许看到一次完整值，关闭后检查列表接口原始 JSON 只有掩码，不能只看页面打码；旧密钥仍能认证，删除后立即失效。测试记录不保存真实密钥。 本项允许创建时一次性取用，重点是不能在后续查询中再次取回。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/api_token.go](/Users/topazzz/code/PandaWiki/backend/usecase/api_token.go)；[backend/repo/pg/ap_token.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/ap_token.go)；[backend/repo/pg/account_security_test.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_security_test.go)。

### M05｜敏感信息泄露：查询 MCP 密钥列表

- **原报告位置**：第 8 页；2.2.1 高危，问题 2/3，列表接口。旧核查根因编号：A02。
- **涉及入口**：/api/pro/v1/token/list。
- **处理状态：本地已修复。** 已修复；本地接口/数据库验证通过。
- **已经怎么修复/处理**：列表接口只返回 token_hint 掩码，不再持续返回完整密钥；模型也不序列化哈希。删除密钥后立即失效，已移除旧认证缓存。
- **尚未完成 / 结论边界**：生产未部署；创建时一次返回完整密钥是取用凭据所需的预期行为，不应要求“永不返回”。
- **保留验收参考（无需你重复执行）**：部署后端和管理端并执行迁移 56。创建密钥后仅允许看到一次完整值，关闭后检查列表接口原始 JSON 只有掩码，不能只看页面打码；旧密钥仍能认证，删除后立即失效。测试记录不保存真实密钥。 本项须直接检查 token/list 原始响应，不以页面遮罩代替接口验收。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/api_token.go](/Users/topazzz/code/PandaWiki/backend/usecase/api_token.go)；[backend/repo/pg/ap_token.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/ap_token.go)；[backend/repo/pg/account_security_test.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_security_test.go)。

### M06｜PDF 存储型 XSS：附件上传与预览

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 3/3，附件入口。旧核查根因编号：A03。
- **涉及入口**：/api/v1/file/upload/attachment。
- **处理状态：部分处理。** 校验和预览缺口已加固；报告 XSS 仍待原样本复测。
- **已经怎么修复/处理**：上传 PDF 解析对象图，拒绝 JavaScript、可执行自动动作、启动程序、嵌入文件及不安全 URI；两端原文件 iframe 不赋予脚本能力。新增测试含转义 PDF 名称，两份正常报告 PDF 可通过。没有扫描、重写历史 OSS 对象；未取得报告原始恶意 PDF，不能宣称原 XSS 已端到端复现并关闭。 普通 PDF 的首页与缩放定位保留。
- **尚未完成 / 结论边界**：缺原始恶意 PDF、执行域证据和完整链路复测；旧 OSS 对象未清洗，不能按“原 XSS 已关闭”销项。
- **你需要做什么**：向测试方索取报告原始恶意 PDF，在隔离测试环境分别测试本条上传入口、预览页和文件直链；记录是否执行、执行域和可访问的上下文。另行排查历史 OSS 文件，现有上传规则不会清洗旧对象；两份正常报告 PDF 也要能正常上传预览。
- **本地依据**：[本地集成测试](本地集成测试结果.txt)；[上传专项/后端测试](后端测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file_validation_test.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation_test.go)；[web/admin/src/pages/document/editor/edit/FilePreview.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/pages/document/editor/edit/FilePreview.tsx)；[web/app/src/views/node/components/FilePreview.tsx](/Users/topazzz/code/PandaWiki/web/app/src/views/node/components/FilePreview.tsx)。

### M07｜PDF 存储型 XSS：普通文件上传与预览

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 3/3，普通文件入口。旧核查根因编号：A03。
- **涉及入口**：/api/v1/file/upload。
- **处理状态：部分处理。** 校验和预览缺口已加固；报告 XSS 仍待原样本复测。
- **已经怎么修复/处理**：上传 PDF 解析对象图，拒绝 JavaScript、可执行自动动作、启动程序、嵌入文件及不安全 URI；两端原文件 iframe 不赋予脚本能力。新增测试含转义 PDF 名称，两份正常报告 PDF 可通过。没有扫描、重写历史 OSS 对象；未取得报告原始恶意 PDF，不能宣称原 XSS 已端到端复现并关闭。 普通 PDF 的首页与缩放定位保留。
- **尚未完成 / 结论边界**：缺原始恶意 PDF、执行域证据和完整链路复测；旧 OSS 对象未清洗，不能按“原 XSS 已关闭”销项。
- **你需要做什么**：向测试方索取报告原始恶意 PDF，在隔离测试环境分别测试本条上传入口、预览页和文件直链；记录是否执行、执行域和可访问的上下文。另行排查历史 OSS 文件，现有上传规则不会清洗旧对象；两份正常报告 PDF 也要能正常上传预览。
- **本地依据**：[本地集成测试](本地集成测试结果.txt)；[上传专项/后端测试](后端测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file_validation_test.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation_test.go)；[web/admin/src/pages/document/editor/edit/FilePreview.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/pages/document/editor/edit/FilePreview.tsx)；[web/app/src/views/node/components/FilePreview.tsx](/Users/topazzz/code/PandaWiki/web/app/src/views/node/components/FilePreview.tsx)。

### M08｜发现内部 IP 泄露模式

- **原报告位置**：第 11 页；2.2.2 中危，问题 1/8。旧核查根因编号：A04。
- **涉及入口**：管理端打包 JS，报告指向 index-BiKqTpCd.js。
- **处理状态：部分处理。** 示例地址已移除；真实内部信息泄露仍未确认。
- **已经怎么修复/处理**：移除 Xinference/GPUStack 前端默认 172.17.0.1 示例；管理员填写实际模型服务地址。此项是加固，不证明报告已取得真实内部拓扑。
- **尚未完成 / 结论边界**：只移除了固定示例；尚不能判断报告是否另有真实内部服务信息。
- **你需要做什么**：部署管理端后，让测试方指出扫描命中的具体 JS 字符串和上下文，确认旧示例已消失；若还含实际内部地址、凭据或拓扑，再提供脱敏证据继续定位。一个示例 IP 不能直接认定真实内网信息泄露。
- **本地依据**：[管理端构建结果](管理端构建结果.txt)。代码/配置：[web/admin/src/constant/enums.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/constant/enums.tsx)。

### M09｜易受攻击的组件（Nginx）

- **原报告位置**：第 12 页；2.2.2 中危，问题 2/8。旧核查根因编号：A05。
- **涉及入口**：管理端入口，报告标记 Nginx 1.31.6。
- **处理状态：待核实。** 不确定。
- **核实结果 / 为什么未标已修复**：当前本地管理端是 Vite，用户端是 Next.js，没有运行报告中的 Nginx 实例；报告也未给 18080 的这一项明确 CVE。不能仅凭 Server 字段断定组件可利用。
- **尚未完成 / 结论边界**：没有针对报告 Nginx 实例确认具体漏洞，也未实施其补丁升级。
- **你需要做什么**：让运维核对报告对应的 Nginx 容器/宿主进程，提供 nginx -v、nginx -V、镜像摘要及包补丁记录；请测试方补充该项具体 CVE 和命中依据。按实际受影响模块与补丁处理后重扫；不要把其他业务的 18081/18082 自动归入本项目。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf:7)。

### M10｜明文密码本地保存

- **原报告位置**：第 13 页；2.2.2 中危，问题 3/8。旧核查根因编号：A06。
- **涉及入口**：管理端记住账号/密码功能。
- **处理状态：本地已修复。** 已修复；代码/类型与登录页面检查通过。
- **已经怎么修复/处理**：两端只记住账号。读取旧 panda_wiki_remember_login 后先删除旧键，再按 account/updatedAt 白名单迁移到新键，不迁移密码；切换账号不再回填密码。旧站点数据在进入该站点登录页时清理，未读取个人浏览器的密码值。
- **尚未完成 / 结论边界**：旧明文数据在访问升级后的登录页时清理；尚未访问该页面的浏览器不会被远程自动清空。
- **保留验收参考（无需你重复执行）**：部署对应前端后访问一次该站点登录页，触发旧记录清理；用专用测试账号检查 localStorage 不再含 password 字段，切换记住的账号不回填密码。旧键 panda_wiki_remember_login 应被删除，新键 panda_wiki_remember_accounts 只保留账号信息；勿把真实密码截图发出。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：两端 TypeScript 与登录页面检查（详见[验证说明](修复与部署说明.md)）。代码/配置：[web/admin/src/pages/login/index.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/pages/login/index.tsx)；[web/app/src/views/auth/login.tsx](/Users/topazzz/code/PandaWiki/web/app/src/views/auth/login.tsx)。

### M11｜密码规则简单

- **原报告位置**：第 14 页；2.2.2 中危，问题 4/8。旧核查根因编号：A07。
- **涉及入口**：/api/v1/user/reset_password（修复同时覆盖新增和本人改密）。
- **处理状态：本地已修复。** 已修复；密码/改密/重置测试通过。
- **已经怎么修复/处理**：服务端统一要求 12–72 字节、大小写字母、数字、特殊字符，不能包含账号；随机密码使用 crypto.getRandomValues。保留批量添加固定初始密码 YunzhouWiki@2026，但新增、重置及迁移前已有密码账号均须首次改密，改密前业务接口返回 40302。重置与本人改密撤销全部账号 JWT 会话。默认 admin 密码仅初始化时使用，重启不再覆盖已改密码。
- **尚未完成 / 结论边界**：固定初始密码保留，依靠首次强制改密约束；不是所有账号已经由本人修改过密码。
- **保留验收参考（无需你重复执行）**：部署后端、两端及迁移 56。用专用账号验证 11111111 和 12 位纯数字均被服务端拒绝；新增/重置账号登录后必须先改密，改密前不能访问业务；改密/重置后原 JWT 失效。既有密码账号升级后也须重新登录并改密。批量初始值仍为 YunzhouWiki@2026，不能作为长期共用密码。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/domain/password_policy.go](/Users/topazzz/code/PandaWiki/backend/domain/password_policy.go)；[backend/usecase/user.go](/Users/topazzz/code/PandaWiki/backend/usecase/user.go)；[backend/repo/pg/user.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/user.go)；[web/packages/ui/src/security/index.tsx](/Users/topazzz/code/PandaWiki/web/packages/ui/src/security/index.tsx)。

### M12｜点击劫持

- **原报告位置**：第 15 页；2.2.2 中危，问题 5/8。旧核查根因编号：A08。
- **涉及入口**：/login。
- **处理状态：本地已修复。** 已修复防嵌入缺口；本地响应验证通过。
- **已经怎么修复/处理**：管理页面、普通用户页面增加 X-Frame-Options 和 CSP frame-ancestors；API 不允许嵌入。明确保留公开 widget 的嵌入能力，不能把该业务例外等同于后台页面可被嵌入。未做诱导真实管理操作。
- **尚未完成 / 结论边界**：已确认缺少防护并补齐；没有执行诱导用户真实操作的攻击复现。
- **保留验收参考（无需你重复执行）**：部署对应前端/API 后，在最终入口核对 X-Frame-Options 和 CSP frame-ancestors，从不同源 iframe 打开普通页面应被浏览器阻止。公开 widget 是有意保留的嵌入功能，应单独验证；无需执行真实管理操作来测试点击。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/src/proxy.ts](/Users/topazzz/code/PandaWiki/web/app/src/proxy.ts)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)；[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)。

### M13｜会话重用

- **原报告位置**：第 16 页；2.2.2 中危，问题 6/8。旧核查根因编号：A09。
- **涉及入口**：管理端退出后的原账号 JWT。
- **处理状态：本地已修复。** 已修复；旧 JWT 退出重放被拒绝。
- **已经怎么修复/处理**：账号 JWT 带 jti 并对应数据库会话；两端主动退出调用撤销接口，随后原 JWT 在管理端/用户端本人信息接口均为 40101。管理员重置和本人改密也撤销会话。MCP 密钥仍是独立凭据，需通过密钥删除操作撤销。
- **尚未完成 / 结论边界**：只对新版本账号会话生效；升级前无数据库会话记录的 JWT 需重新登录。
- **保留验收参考（无需你重复执行）**：部署后端与两端，重新登录获取新会话；用专用测试账号退出后，携原 JWT 访问本人信息应返回登录失效码 40101。本人改密和管理员重置也应使旧 JWT 失效。MCP 密钥独立管理，需要删除密钥撤销，不能混作账号 JWT。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/repo/pg/account_session.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_session.go)；[backend/middleware/account_session.go](/Users/topazzz/code/PandaWiki/backend/middleware/account_session.go)；[backend/handler/v1/user.go](/Users/topazzz/code/PandaWiki/backend/handler/v1/user.go)；[web/admin/src/components/Header/index.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/components/Header/index.tsx)；[web/app/src/components/header/index.tsx](/Users/topazzz/code/PandaWiki/web/app/src/components/header/index.tsx)。

### M14｜会话失效时间过长

- **原报告位置**：第 17 页；2.2.2 中危，问题 7/8。旧核查根因编号：A10。
- **涉及入口**：管理端登录会话。
- **处理状态：本地已修复。** 已修复闲置控制缺口；本地时间边界验证通过。
- **已经怎么修复/处理**：默认绝对期限 8 小时、最近有效认证请求后 30 分钟闲置失效，可配置。数据库原子检查并更新会话活动时间；已过期/已撤销会话不会被新请求复活。普通无权限响应不触发用户端全局退出，仅明确会话码 40101 清理；流式请求同样识别此码。测试通过移动临时会话时间字段验证边界，没有实际等待 8 小时。
- **尚未完成 / 结论边界**：本地通过调整临时会话时间验证边界，没有实际等待 8 小时。未实现报告建议中的所有附加策略，例如单设备登录和 IP/UA 绑定。
- **保留验收参考（无需你重复执行）**：确认你们要求的超时政策；默认 AUTH_SESSION_MAX_AGE_SECONDS=28800、AUTH_SESSION_IDLE_SECONDS=1800。部署后用无后台轮询的专用会话，闲置超过阈值再请求，应被拒绝；普通文档无权限不能被误判为退出。这里的闲置是最近有效认证请求，后台请求可能续活动时间。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/config/config.go](/Users/topazzz/code/PandaWiki/backend/config/config.go)；[backend/repo/pg/account_session.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_session.go)；[web/app/src/utils/fetch.ts](/Users/topazzz/code/PandaWiki/web/app/src/utils/fetch.ts)。

### M15｜主机头注入

- **原报告位置**：第 18 页；2.2.2 中危，问题 8/8。旧核查根因编号：A11。
- **涉及入口**：/panda-wiki-admin-assets，Host / Location。
- **处理状态：本地已修复。** 已加固；三处本地异常 Host 均被拒绝。
- **已经怎么修复/处理**：API、Vite、Next.js、管理端 Nginx 均校验 Host，audit.invalid 返回 421。可用 SECURITY_ALLOWED_HOSTS 指定精确域名；默认支持本地和 IP 访问及必要内部调用名。此修复针对未校验缺口，原报告的缓存/跳转污染利用链此前未确认。
- **尚未完成 / 结论边界**：本地拒绝异常 Host 已验证；报告描述的实际缓存污染/重置链接利用未完整复现。
- **保留验收参考（无需你重复执行）**：按实际域名配置 SECURITY_ALLOWED_HOSTS（精确主机名、无端口），并包含必要的内部 API 调用名。部署全部入口后，以测试域名修改 Host，应返回拒绝响应且不能产生指向该域的 Location；另复查前置代理、缓存与重定向。不要把正常返回 200 本身当作已成功利用。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/admin/docker-entrypoint.d/15-security-hosts.sh](/Users/topazzz/code/PandaWiki/web/admin/docker-entrypoint.d/15-security-hosts.sh)。

### M16｜Content-Security-Policy 头缺失

- **原报告位置**：第 19–20 页；2.2.3 低危，问题 1/5。旧核查根因编号：A12。
- **涉及入口**：管理端页面响应。
- **处理状态：本地已修复。** 已修复缺头；本地响应与页面检查通过。
- **已经怎么修复/处理**：API、Vite、Next.js、Nginx 增加 CSP；Next 页面按请求生成 nonce，生产不启用 unsafe-eval；管理端生产启动脚本改为同源独立文件。开发环境为 HMR 保留开发所需策略，样式保留 MUI 所需 inline。不是宣称 CSP 可以取代 XSS 修复。
- **尚未完成 / 结论边界**：修复的是缺头与执行策略缺口；不能代表所有 XSS 消除，也不能代表整个生产站点已回归。
- **保留验收参考（无需你重复执行）**：部署对应前端和 API，在最终入口检查 CSP 未被上层代理覆盖。回归登录、文档、预览和问答。生产不应照搬开发 HMR 的 unsafe-eval；MUI 的 inline 样式有业务兼容设置。用户端完整 Next 生产构建及全页面回归尚未完成，需补齐。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[管理端构建结果](管理端构建结果.txt)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/src/proxy.ts](/Users/topazzz/code/PandaWiki/web/app/src/proxy.ts)；[web/admin/public/bootstrap.js](/Users/topazzz/code/PandaWiki/web/admin/public/bootstrap.js)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)。

### M17｜X-Content-Type-Options 头缺失或不安全

- **原报告位置**：第 21 页；2.2.3 低危，问题 2/5。旧核查根因编号：A13。
- **涉及入口**：管理端页面响应。
- **处理状态：本地已修复。** 已修复应用缺头；外部 OSS 仍待核实。
- **已经怎么修复/处理**：本地两端/API 均返回 nosniff；Nginx 与 Next 的原始静态文件代理添加 nosniff、下载/沙箱头。没有访问外部 OSS，无法确认对象源站/网关是否补同样的头。
- **尚未完成 / 结论边界**：应用侧已补头；OSS 源站和正式代理响应仍未知。
- **保留验收参考（无需你重复执行）**：部署前端/API 后检查最终页面、错误响应、文件代理和预览均有 X-Content-Type-Options: nosniff。外部 OSS 原始对象/下载域需由你单独检查响应头；应用加头不会自动修改 OSS 源站。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/next.config.ts](/Users/topazzz/code/PandaWiki/web/app/next.config.ts)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)。

### M18｜不必要的 HTTP 响应头

- **原报告位置**：第 22 页；2.2.3 低危，问题 3/5。旧核查根因编号：A14。
- **涉及入口**：Server / X-Powered-By 等指纹。
- **处理状态：部分处理。** 已加固；本地 Next.js 指纹头已移除。
- **已经怎么修复/处理**：关闭 Next.js X-Powered-By，Nginx server_tokens off 隐藏版本。本地临时 Nginx 仍返回通用 Server: nginx，未声称删除了所有 Server 字段或修复任意组件 CVE。
- **尚未完成 / 结论边界**：通用 Server: nginx 仍存在；按“彻底移除所有指纹头”的验收口径尚未完成。
- **你需要做什么**：部署前端后检查最终入口响应头：Next.js 的 X-Powered-By 应消失，Nginx 不应带版本号。若验收要求连 Server: nginx 都不能出现，还需由网关或支持移除该头的组件处理并复测；目前没有完成这一点。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[web/app/next.config.ts](/Users/topazzz/code/PandaWiki/web/app/next.config.ts)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)。

### M19｜检测到隐藏目录

- **原报告位置**：第 23 页；2.2.3 低危，问题 4/5。旧核查根因编号：A15。
- **涉及入口**：/images/。
- **处理状态：待核实。** 不确定。
- **核实结果 / 为什么未标已修复**：本地未运行该 Nginx 目录站点，没有复现目录枚举/列目录/读取非公开文件。知道 images/ 名称本身不证明文件泄露。
- **尚未完成 / 结论边界**：未确认列目录或非公开文件泄露，因此没有盲目修改 /images/ 业务路径。
- **你需要做什么**：让测试方给出 /images/ 实际返回内容，并与随机不存在路径对比，区分 SPA 首页回退、目录名猜测、真实列目录与敏感文件可读。运维核对 autoindex 和静态资源路径；发现非公开文件可读才据此修复，不能只凭目录名删除业务资源。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf:7)。

### M20｜过度许可的 CORS 访问

- **原报告位置**：第 24 页；2.2.3 低危，问题 5/5。旧核查根因编号：A16。
- **涉及入口**：/share/v1/app/web/info。
- **处理状态：本地已修复。** 已收紧配置；本地恶意 Origin 被拒绝。
- **已经怎么修复/处理**：移除 share 处理器硬编码 *，统一允许同源及明确的 SECURITY_ALLOWED_ORIGINS；预检允许所需认证头，不返回通配凭据策略。恶意 Origin 本地 403；合法开发同源代理正常。公开 widget 通过 iframe 正常使用；独立跨域 API 集成需逐个允许来源。原敏感数据跨源窃取未被确认，不宣称已复现。
- **尚未完成 / 结论边界**：本地恶意 Origin 已拒绝；此前未证实敏感数据被跨源窃取，每个报告接口仍需部署后复测。
- **保留验收参考（无需你重复执行）**：部署 API 和前端后，对本条列出的每个接口分别测试恶意 Origin 与 OPTIONS 预检，不能只测首页；浏览器不得读取带凭据的非公开跨源响应。独立跨源集成才配置 SECURITY_ALLOWED_ORIGINS 精确来源，普通同源访问无需配置；同时验证评论、问答流和公开 widget 正常。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[backend/handler/share/common.go](/Users/topazzz/code/PandaWiki/backend/handler/share/common.go)；[backend/handler/share/comment.go](/Users/topazzz/code/PandaWiki/backend/handler/share/comment.go)。

### M21｜Referrer-Policy 头缺失

- **原报告位置**：第 25–26 页；2.2.4 参考，问题 1/2（报告写作 Referral Policy）。旧核查根因编号：A17。
- **涉及入口**：管理端页面响应。
- **处理状态：本地已修复。** 已修复应用缺头；本地响应验证通过。
- **已经怎么修复/处理**：两端/API 增加 strict-origin-when-cross-origin；原始文件代理采用 no-referrer。外部 OSS 响应策略仍未检查。
- **尚未完成 / 结论边界**：应用侧已补头；OSS 源站及其他下载域未核实。
- **保留验收参考（无需你重复执行）**：部署后检查最终页面/API 为 Referrer-Policy: strict-origin-when-cross-origin，原始文件代理为 no-referrer。OSS 源站/下载域由你另行核实；使用测试签名地址检查跨域跳转或资源加载没有泄露完整敏感 URL。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)；[web/app/next.config.ts](/Users/topazzz/code/PandaWiki/web/app/next.config.ts)。

### M22｜客户端 JavaScript 操作 Cookie

- **原报告位置**：第 27 页；2.2.4 参考，问题 2/2。旧核查根因编号：A18。
- **涉及入口**：管理端打包 JS，报告指向 index-B2ZihuWK.js。
- **处理状态：待核实。** 报告命中 Cookie 的安全用途未确认。
- **核实结果 / 为什么未标已修复**：管理端业务代码使用 localStorage 保存账号 JWT，当前源码未见直接操作 Cookie 的业务代码；报告只命中打包 JS，未给出具体 Cookie 名及其安全用途。依赖脚本的 Cookie 操作是否涉及身份、是否可形成利用仍不确定。JS 可读凭据也尚未迁移到服务端 HttpOnly 会话。
- **尚未完成 / 结论边界**：尚未取得报告命中 Cookie 的名称、用途及运行证据，不能把用户端 JS 写 JWT Cookie 的实现直接套到管理端，也不能将此项按漏洞已修复销项。
- **你需要做什么**：请测试方提供命中脚本的具体代码片段、实际 Cookie 名、产生时机及是否为登录凭据，先确认是否涉及身份安全。若验收要求身份凭据不能被 JS 读取，管理端当前 localStorage 登录态也需一并改造为服务端 HttpOnly 会话，并配套 Secure/SameSite 和 CSRF 防护；这是后续代码工作。不要仅凭存在 Cookie 操作代码认定提权。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[管理端登录凭据保存](/Users/topazzz/code/PandaWiki/web/admin/src/pages/login/index.tsx:136)；[管理端请求凭据读取](/Users/topazzz/code/PandaWiki/web/admin/src/utils/fetch.ts:56)。

## 用户端报告：应用问题

### U01｜知识库文档原文泄露

- **原报告位置**：第 6 页；2.2.1 高危，问题 1/5。旧核查根因编号：A19。
- **涉及入口**：/share/v1/node/detail。
- **处理状态：待核实。** 本地受限样本未复现；不能宣告全部修复。
- **核实结果 / 为什么未标已修复**：匿名访问 P2 返回 HTTP 200 但 success:false、code:40003、Permission Denied，没有正文和 file_key；P0 可匿名读取符合公开配置。P7 可匿名读取，但其栏目实际保存为公开，见 N01。代码具备节点、祖先、栏目权限检查。未穷举全部角色/节点。
- **尚未完成 / 结论边界**：只验证了本地部分权限样本，没有原报告文档的当前配置，不能确定报告问题已不存在。
- **你需要做什么**：用明确配置为受限的测试文档，分别以匿名、无权限、正常授权账号访问 detail；拒绝时检查 success/code/正文/file_key，不能只看 HTTP 200。另测父目录、栏目限制、发布旧版本和问答引用。P7 栏目在本地核查时为公开，需先由你确认并调整预期配置，再判断泄露；当前不宣称全组合已修复。
- **本地依据**：[修复前本地文档权限样本](本地服务与文档核查证据.json)。代码/配置：[backend/handler/share/node.go](/Users/topazzz/code/PandaWiki/backend/handler/share/node.go:79)；[backend/usecase/node.go](/Users/topazzz/code/PandaWiki/backend/usecase/node.go:275)；本地服务与文档核查证据.json。

### U02｜知识库任意文件下载

- **原报告位置**：第 7 页；2.2.1 高危，问题 2/5。旧核查根因编号：A20。
- **涉及入口**：/share/v1/node/detail 获取 file_key 后访问文件存储。
- **处理状态：待核实。** 接口检查存在；端到端不确定。
- **核实结果 / 为什么未标已修复**：签发接口要求 node_id、节点可读和 key 属于该发布节点；本地受限 P2 请求被拒绝，公开 P0 的伪造 key 也被拒绝。未请求外部 OSS，无法确认真实桶是否公共读或既有地址能否绕过。
- **尚未完成 / 结论边界**：接口权限检查存在且本地受限样本被拒绝；OSS ACL、历史公开直链及完整下载链路未核实。
- **你需要做什么**：由你检查 OSS 桶及对象是否公共读，建议受限文件使用私有读与短期签名；对匿名、无权限、授权账号分别测试获取 file_key 和签名，再测试未签名直链、跨节点换 key、旧地址。签发接口拒绝不代表公开桶不能被绕过，旧有效签名在自身期限内可用也须与新签发越权区分。
- **本地依据**：[修复前本地文档/签名拒绝样本](本地服务与文档核查证据.json)。代码/配置：[backend/handler/share/common.go](/Users/topazzz/code/PandaWiki/backend/handler/share/common.go:176)。

### U03｜Gotenberg 未授权访问

- **原报告位置**：第 8 页；2.2.1 高危，问题 3/5。旧核查根因编号：A21。
- **涉及入口**：报告 URL 仍写 node/detail，但描述指向 Gotenberg 转换服务，需测试方确认实际端口/路径。
- **处理状态：部分处理。** 部署模板已收紧；匿名转换/SSRF/RCE 仍不确定。
- **已经怎么修复/处理**：Gotenberg 3000 的宿主端口改为 127.0.0.1，仅 API 经容器内部网络调用。没有在当前本地部署该转换服务，也没有新增其 HTTP 身份认证；不能宣称已验证转换链路或报告利用样本。
- **尚未完成 / 结论边界**：仅调整了暴露边界；当前未运行对应转换实例，未新增 HTTP 鉴权，也未验证 SSRF/RCE。
- **你需要做什么**：部署最新 Compose 后确认 3000 仅映射 127.0.0.1，外部不能绕过 API 直达，API 容器内转换仍正常；检查 Docker 网络中的其他可达来源。请测试方补充真实转换端口/路径及利用证据。如要求本机也鉴权，需要适配实际 Gotenberg 版本的认证代理/认证配置，不能直接认定报告给出的环境变量可用。
- **本地依据**：部署模板检查，尚无对应服务完整运行证据。代码/配置：[deploy/server/docker-compose.yml](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml)。

### U04｜任意文件上传：普通文件

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 4/5，入口 1/5。旧核查根因编号：A22。
- **涉及入口**：/api/v1/file/upload。
- **处理状态：本地已修复。** 已修复确认的上传校验缺口；未证实 RCE。
- **已经怎么修复/处理**：普通文件上传改用明确类型白名单与真实内容检查；不信任请求 Content-Type，策略读取/解析异常时拒绝；新上传不再接受脚本、HTML、SVG 或可执行文件。
- **尚未完成 / 结论边界**：已修复确认的类型/内容校验缺口；没有执行原报告的完整上传利用，旧 OLE Office 只检查容器头，不是杀毒或完整消毒。
- **保留验收参考（无需你重复执行）**：部署后端与前端后，在隔离测试环境逐一复测本条上传入口：脚本、HTML、SVG、伪扩展名/伪 MIME 应拒绝，合法 PDF/Office/图片/MP4 等应正常。记录上传、访问、执行三个环节；尚未向外部 OSS 上传样本，不宣称服务器 RCE 已被复现或关闭。历史存储对象需单独检查，不能靠新上传校验自动清洗。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[后端测试](后端测试结果.txt)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file.go](/Users/topazzz/code/PandaWiki/backend/usecase/file.go)；[backend/store/s3/minio.go](/Users/topazzz/code/PandaWiki/backend/store/s3/minio.go)。

### U05｜任意文件上传：附件

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 4/5，入口 2/5。旧核查根因编号：A22。
- **涉及入口**：/api/v1/file/upload/attachment。
- **处理状态：本地已修复。** 已修复确认的上传校验缺口；未证实 RCE。
- **已经怎么修复/处理**：附件入口不再跳过扩展名/内容检查，与普通文件使用同一白名单、PDF/Office 内容规则。
- **尚未完成 / 结论边界**：已修复确认的类型/内容校验缺口；没有执行原报告的完整上传利用，旧 OLE Office 只检查容器头，不是杀毒或完整消毒。
- **保留验收参考（无需你重复执行）**：部署后端与前端后，在隔离测试环境逐一复测本条上传入口：脚本、HTML、SVG、伪扩展名/伪 MIME 应拒绝，合法 PDF/Office/图片/MP4 等应正常。记录上传、访问、执行三个环节；尚未向外部 OSS 上传样本，不宣称服务器 RCE 已被复现或关闭。历史存储对象需单独检查，不能靠新上传校验自动清洗。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[后端测试](后端测试结果.txt)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file.go](/Users/topazzz/code/PandaWiki/backend/usecase/file.go)；[backend/store/s3/minio.go](/Users/topazzz/code/PandaWiki/backend/store/s3/minio.go)。

### U06｜任意文件上传：图片

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 4/5，入口 3/5。旧核查根因编号：A22。
- **涉及入口**：/api/v1/file/upload/image。
- **处理状态：本地已修复。** 已修复确认的上传校验缺口；未证实 RCE。
- **已经怎么修复/处理**：图片必须与扩展名对应、可实际解码，最大 20 MB、4000 万像素；改名脚本和伪图片不通过。
- **尚未完成 / 结论边界**：已修复确认的类型/内容校验缺口；没有执行原报告的完整上传利用，旧 OLE Office 只检查容器头，不是杀毒或完整消毒。
- **保留验收参考（无需你重复执行）**：部署后端与前端后，在隔离测试环境逐一复测本条上传入口：脚本、HTML、SVG、伪扩展名/伪 MIME 应拒绝，合法 PDF/Office/图片/MP4 等应正常。记录上传、访问、执行三个环节；尚未向外部 OSS 上传样本，不宣称服务器 RCE 已被复现或关闭。历史存储对象需单独检查，不能靠新上传校验自动清洗。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[后端测试](后端测试结果.txt)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file.go](/Users/topazzz/code/PandaWiki/backend/usecase/file.go)；[backend/store/s3/minio.go](/Users/topazzz/code/PandaWiki/backend/store/s3/minio.go)。

### U07｜任意文件上传：URL 导入

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 4/5，入口 4/5。旧核查根因编号：A22。
- **涉及入口**：/api/v1/file/upload/url。
- **处理状态：本地已修复。** 已修复确认的上传校验缺口；未证实 RCE。
- **已经怎么修复/处理**：URL 导入保留 SSRF、重定向及大小限制，并接入统一类型/内容校验；远端给出的 MIME 不能替代真实内容检查。
- **尚未完成 / 结论边界**：已修复确认的类型/内容校验缺口；没有执行原报告的完整上传利用，旧 OLE Office 只检查容器头，不是杀毒或完整消毒。
- **保留验收参考（无需你重复执行）**：部署后端与前端后，在隔离测试环境逐一复测本条上传入口：脚本、HTML、SVG、伪扩展名/伪 MIME 应拒绝，合法 PDF/Office/图片/MP4 等应正常。记录上传、访问、执行三个环节；尚未向外部 OSS 上传样本，不宣称服务器 RCE 已被复现或关闭。历史存储对象需单独检查，不能靠新上传校验自动清洗。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[后端测试](后端测试结果.txt)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file.go](/Users/topazzz/code/PandaWiki/backend/usecase/file.go)；[backend/store/s3/minio.go](/Users/topazzz/code/PandaWiki/backend/store/s3/minio.go)。

### U08｜任意文件上传：用户端图片上传

- **原报告位置**：第 9–10 页；2.2.1 高危，问题 4/5，入口 5/5。旧核查根因编号：A22。
- **涉及入口**：/share/v1/common/file/upload。
- **处理状态：本地已修复。** 已修复确认的上传校验缺口；未证实 RCE。
- **已经怎么修复/处理**：用户端上传继续保留图片扩展名及验证码要求，并补充真实格式、实际解码、大小和像素检查；该共享入口不是任意文件上传通道。
- **尚未完成 / 结论边界**：已修复确认的类型/内容校验缺口；没有执行原报告的完整上传利用，旧 OLE Office 只检查容器头，不是杀毒或完整消毒。
- **保留验收参考（无需你重复执行）**：部署后端与前端后，在隔离测试环境逐一复测本条上传入口：脚本、HTML、SVG、伪扩展名/伪 MIME 应拒绝，合法 PDF/Office/图片/MP4 等应正常。记录上传、访问、执行三个环节；尚未向外部 OSS 上传样本，不宣称服务器 RCE 已被复现或关闭。历史存储对象需单独检查，不能靠新上传校验自动清洗。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[后端测试](后端测试结果.txt)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/usecase/file_validation.go](/Users/topazzz/code/PandaWiki/backend/usecase/file_validation.go)；[backend/usecase/file.go](/Users/topazzz/code/PandaWiki/backend/usecase/file.go)；[backend/store/s3/minio.go](/Users/topazzz/code/PandaWiki/backend/store/s3/minio.go)。

### U09｜匿名批量评论、伪造评论身份

- **原报告位置**：第 11 页；2.2.1 高危，问题 5/5。旧核查根因编号：A23。
- **涉及入口**：/share/v1/comment。
- **处理状态：本地已修复。** 已修复确认的评论身份/权限缺口；本地验证通过。
- **已经怎么修复/处理**：仅登录账号可写评论，姓名从真实用户资料取值；写入和读取均检查节点查阅权限及知识库归属。每账号每文档每分钟最多 5 次提交，Redis 原子计数，验证码和审核仍保留；无权限先拒绝。匿名写入、伪造姓名和跨知识库行为有本地测试；没有向业务文档写入测试评论。
- **尚未完成 / 结论边界**：本地身份、节点权限与限频已验证；验证码单次使用、全部审核场景未完整验证。
- **保留验收参考（无需你重复执行）**：部署后端和两端后，用测试文档检查匿名不能评论、无查阅权限不能读写评论、请求伪造姓名不生效；同一账号同一文档一分钟超过 5 次应被限频。验证码单次使用与审核流程还需单独验收。测试后清理专用评论，勿使用业务文档刷测。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/handler/share/comment.go](/Users/topazzz/code/PandaWiki/backend/handler/share/comment.go)；[backend/usecase/comment.go](/Users/topazzz/code/PandaWiki/backend/usecase/comment.go)；[backend/usecase/comment_security_test.go](/Users/topazzz/code/PandaWiki/backend/usecase/comment_security_test.go)。

### U10｜明文密码本地保存

- **原报告位置**：第 12 页；2.2.2 中危，问题 1/5。旧核查根因编号：A06。
- **涉及入口**：/auth/login。
- **处理状态：本地已修复。** 已修复；代码/类型与登录页面检查通过。
- **已经怎么修复/处理**：两端只记住账号。读取旧 panda_wiki_remember_login 后先删除旧键，再按 account/updatedAt 白名单迁移到新键，不迁移密码；切换账号不再回填密码。旧站点数据在进入该站点登录页时清理，未读取个人浏览器的密码值。
- **尚未完成 / 结论边界**：旧明文数据在访问升级后的登录页时清理；尚未访问该页面的浏览器不会被远程自动清空。
- **保留验收参考（无需你重复执行）**：部署对应前端后访问一次该站点登录页，触发旧记录清理；用专用测试账号检查 localStorage 不再含 password 字段，切换记住的账号不回填密码。旧键 panda_wiki_remember_login 应被删除，新键 panda_wiki_remember_accounts 只保留账号信息；勿把真实密码截图发出。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：两端 TypeScript 与登录页面检查（详见[验证说明](修复与部署说明.md)）。代码/配置：[web/admin/src/pages/login/index.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/pages/login/index.tsx)；[web/app/src/views/auth/login.tsx](/Users/topazzz/code/PandaWiki/web/app/src/views/auth/login.tsx)。

### U11｜点击劫持

- **原报告位置**：第 13 页；2.2.2 中危，问题 2/5。旧核查根因编号：A08。
- **涉及入口**：普通用户端页面 /。
- **处理状态：本地已修复。** 已修复防嵌入缺口；本地响应验证通过。
- **已经怎么修复/处理**：管理页面、普通用户页面增加 X-Frame-Options 和 CSP frame-ancestors；API 不允许嵌入。明确保留公开 widget 的嵌入能力，不能把该业务例外等同于后台页面可被嵌入。未做诱导真实管理操作。
- **尚未完成 / 结论边界**：已确认缺少防护并补齐；没有执行诱导用户真实操作的攻击复现。
- **保留验收参考（无需你重复执行）**：部署对应前端/API 后，在最终入口核对 X-Frame-Options 和 CSP frame-ancestors，从不同源 iframe 打开普通页面应被浏览器阻止。公开 widget 是有意保留的嵌入功能，应单独验证；无需执行真实管理操作来测试点击。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/src/proxy.ts](/Users/topazzz/code/PandaWiki/web/app/src/proxy.ts)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)；[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)。

### U12｜会话失效时间过长

- **原报告位置**：第 14 页；2.2.2 中危，问题 3/5。旧核查根因编号：A10。
- **涉及入口**：用户端登录会话。
- **处理状态：本地已修复。** 已修复闲置控制缺口；本地时间边界验证通过。
- **已经怎么修复/处理**：默认绝对期限 8 小时、最近有效认证请求后 30 分钟闲置失效，可配置。数据库原子检查并更新会话活动时间；已过期/已撤销会话不会被新请求复活。普通无权限响应不触发用户端全局退出，仅明确会话码 40101 清理；流式请求同样识别此码。测试通过移动临时会话时间字段验证边界，没有实际等待 8 小时。
- **尚未完成 / 结论边界**：本地通过调整临时会话时间验证边界，没有实际等待 8 小时。未实现报告建议中的所有附加策略，例如单设备登录和 IP/UA 绑定。
- **保留验收参考（无需你重复执行）**：确认你们要求的超时政策；默认 AUTH_SESSION_MAX_AGE_SECONDS=28800、AUTH_SESSION_IDLE_SECONDS=1800。部署后用无后台轮询的专用会话，闲置超过阈值再请求，应被拒绝；普通文档无权限不能被误判为退出。这里的闲置是最近有效认证请求，后台请求可能续活动时间。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/config/config.go](/Users/topazzz/code/PandaWiki/backend/config/config.go)；[backend/repo/pg/account_session.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_session.go)；[web/app/src/utils/fetch.ts](/Users/topazzz/code/PandaWiki/web/app/src/utils/fetch.ts)。

### U13｜会话重用

- **原报告位置**：第 15 页；2.2.2 中危，问题 4/5。旧核查根因编号：A09。
- **涉及入口**：用户端退出后的原账号 JWT。
- **处理状态：本地已修复。** 已修复；旧 JWT 退出重放被拒绝。
- **已经怎么修复/处理**：账号 JWT 带 jti 并对应数据库会话；两端主动退出调用撤销接口，随后原 JWT 在管理端/用户端本人信息接口均为 40101。管理员重置和本人改密也撤销会话。MCP 密钥仍是独立凭据，需通过密钥删除操作撤销。
- **尚未完成 / 结论边界**：只对新版本账号会话生效；升级前无数据库会话记录的 JWT 需重新登录。
- **保留验收参考（无需你重复执行）**：部署后端与两端，重新登录获取新会话；用专用测试账号退出后，携原 JWT 访问本人信息应返回登录失效码 40101。本人改密和管理员重置也应使旧 JWT 失效。MCP 密钥独立管理，需要删除密钥撤销，不能混作账号 JWT。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[23 项会话/密钥/评论 HTTP 验证](修复后会话HTTP验证.json)；[本地集成测试](本地集成测试结果.txt)。代码/配置：[backend/repo/pg/account_session.go](/Users/topazzz/code/PandaWiki/backend/repo/pg/account_session.go)；[backend/middleware/account_session.go](/Users/topazzz/code/PandaWiki/backend/middleware/account_session.go)；[backend/handler/v1/user.go](/Users/topazzz/code/PandaWiki/backend/handler/v1/user.go)；[web/admin/src/components/Header/index.tsx](/Users/topazzz/code/PandaWiki/web/admin/src/components/Header/index.tsx)；[web/app/src/components/header/index.tsx](/Users/topazzz/code/PandaWiki/web/app/src/components/header/index.tsx)。

### U14｜主机头未校验

- **原报告位置**：第 16 页；2.2.2 中危，问题 5/5。旧核查根因编号：A11。
- **涉及入口**：用户端入口 /。
- **处理状态：本地已修复。** 已加固；三处本地异常 Host 均被拒绝。
- **已经怎么修复/处理**：API、Vite、Next.js、管理端 Nginx 均校验 Host，audit.invalid 返回 421。可用 SECURITY_ALLOWED_HOSTS 指定精确域名；默认支持本地和 IP 访问及必要内部调用名。此修复针对未校验缺口，原报告的缓存/跳转污染利用链此前未确认。
- **尚未完成 / 结论边界**：本地拒绝异常 Host 已验证；报告描述的实际缓存污染/重置链接利用未完整复现。
- **保留验收参考（无需你重复执行）**：按实际域名配置 SECURITY_ALLOWED_HOSTS（精确主机名、无端口），并包含必要的内部 API 调用名。部署全部入口后，以测试域名修改 Host，应返回拒绝响应且不能产生指向该域的 Location；另复查前置代理、缓存与重定向。不要把正常返回 200 本身当作已成功利用。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/admin/docker-entrypoint.d/15-security-hosts.sh](/Users/topazzz/code/PandaWiki/web/admin/docker-entrypoint.d/15-security-hosts.sh)。

### U15｜Content-Security-Policy 头缺失

- **原报告位置**：第 17–18 页；2.2.3 低危，问题 1/3。旧核查根因编号：A12。
- **涉及入口**：用户端页面响应。
- **处理状态：本地已修复。** 已修复缺头；本地响应与页面检查通过。
- **已经怎么修复/处理**：API、Vite、Next.js、Nginx 增加 CSP；Next 页面按请求生成 nonce，生产不启用 unsafe-eval；管理端生产启动脚本改为同源独立文件。开发环境为 HMR 保留开发所需策略，样式保留 MUI 所需 inline。不是宣称 CSP 可以取代 XSS 修复。
- **尚未完成 / 结论边界**：修复的是缺头与执行策略缺口；不能代表所有 XSS 消除，也不能代表整个生产站点已回归。
- **保留验收参考（无需你重复执行）**：部署对应前端和 API，在最终入口检查 CSP 未被上层代理覆盖。回归登录、文档、预览和问答。生产不应照搬开发 HMR 的 unsafe-eval；MUI 的 inline 样式有业务兼容设置。用户端完整 Next 生产构建及全页面回归尚未完成，需补齐。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[管理端构建结果](管理端构建结果.txt)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/src/proxy.ts](/Users/topazzz/code/PandaWiki/web/app/src/proxy.ts)；[web/admin/public/bootstrap.js](/Users/topazzz/code/PandaWiki/web/admin/public/bootstrap.js)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)。

### U16｜X-Content-Type-Options 头缺失或不安全

- **原报告位置**：第 19 页；2.2.3 低危，问题 2/3。旧核查根因编号：A13。
- **涉及入口**：用户端页面响应。
- **处理状态：本地已修复。** 已修复应用缺头；外部 OSS 仍待核实。
- **已经怎么修复/处理**：本地两端/API 均返回 nosniff；Nginx 与 Next 的原始静态文件代理添加 nosniff、下载/沙箱头。没有访问外部 OSS，无法确认对象源站/网关是否补同样的头。
- **尚未完成 / 结论边界**：应用侧已补头；OSS 源站和正式代理响应仍未知。
- **保留验收参考（无需你重复执行）**：部署前端/API 后检查最终页面、错误响应、文件代理和预览均有 X-Content-Type-Options: nosniff。外部 OSS 原始对象/下载域需由你单独检查响应头；应用加头不会自动修改 OSS 源站。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[web/app/next.config.ts](/Users/topazzz/code/PandaWiki/web/app/next.config.ts)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)。

### U17｜过度许可的 CORS 访问（两个接口）

- **原报告位置**：第 20 页；2.2.3 低危，问题 3/3。旧核查根因编号：A16。
- **涉及入口**：/share/v1/comment/list；/share/v1/chat/message。
- **处理状态：本地已修复。** 已收紧配置；本地恶意 Origin 被拒绝。
- **已经怎么修复/处理**：移除 share 处理器硬编码 *，统一允许同源及明确的 SECURITY_ALLOWED_ORIGINS；预检允许所需认证头，不返回通配凭据策略。恶意 Origin 本地 403；合法开发同源代理正常。公开 widget 通过 iframe 正常使用；独立跨域 API 集成需逐个允许来源。原敏感数据跨源窃取未被确认，不宣称已复现。
- **尚未完成 / 结论边界**：本地恶意 Origin 已拒绝；此前未证实敏感数据被跨源窃取，每个报告接口仍需部署后复测。
- **保留验收参考（无需你重复执行）**：部署 API 和前端后，对本条列出的每个接口分别测试恶意 Origin 与 OPTIONS 预检，不能只测首页；浏览器不得读取带凭据的非公开跨源响应。独立跨源集成才配置 SECURITY_ALLOWED_ORIGINS 精确来源，普通同源访问无需配置；同时验证评论、问答流和公开 widget 正常。 本条两个接口必须各测一次，尤其要覆盖问答 SSE 流，不能用评论接口通过代替问答通过。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[backend/handler/share/common.go](/Users/topazzz/code/PandaWiki/backend/handler/share/common.go)；[backend/handler/share/comment.go](/Users/topazzz/code/PandaWiki/backend/handler/share/comment.go)。

### U18｜Referrer-Policy 头缺失

- **原报告位置**：第 21–22 页；2.2.4 参考，问题 1/2（报告写作 Referral Policy）。旧核查根因编号：A17。
- **涉及入口**：用户端页面响应。
- **处理状态：本地已修复。** 已修复应用缺头；本地响应验证通过。
- **已经怎么修复/处理**：两端/API 增加 strict-origin-when-cross-origin；原始文件代理采用 no-referrer。外部 OSS 响应策略仍未检查。
- **尚未完成 / 结论边界**：应用侧已补头；OSS 源站及其他下载域未核实。
- **保留验收参考（无需你重复执行）**：部署后检查最终页面/API 为 Referrer-Policy: strict-origin-when-cross-origin，原始文件代理为 no-referrer。OSS 源站/下载域由你另行核实；使用测试签名地址检查跨域跳转或资源加载没有泄露完整敏感 URL。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)。代码/配置：[web/security.mjs](/Users/topazzz/code/PandaWiki/web/security.mjs)；[backend/middleware/http_security.go](/Users/topazzz/code/PandaWiki/backend/middleware/http_security.go)；[web/admin/server.conf](/Users/topazzz/code/PandaWiki/web/admin/server.conf)；[web/app/next.config.ts](/Users/topazzz/code/PandaWiki/web/app/next.config.ts)。

### U19｜客户端 JavaScript 操作 Cookie（五个脚本实例）

- **原报告位置**：第 23–24 页；2.2.4 参考，问题 2/2。旧核查根因编号：A18。
- **涉及入口**：报告列出 5 个 JS chunk：bf855e95eb8420e4、74c83bfb133ec37e、ed1b04c3cc14c9b9、897729c1e8075c8b、ef3c852976ce61e5。
- **处理状态：尚未修复。** 实现存在；不能单独判定漏洞。
- **核实结果 / 为什么未标已修复**：本次补充了会话撤销、有效期与 JWT 校验相关防护，但没有改为服务端 HttpOnly 身份 Cookie；不能把其他会话修复写成这一项已经完成。报告列出的 JS 操作 Cookie 仍需区分实际用途。
- **尚未完成 / 结论边界**：JS 可读身份 JWT 的实现仍在。签名校验、注销撤销和缩短有效期均不等于 HttpOnly。仅发现 JS 操作 Cookie 也不足以证明提权。
- **你需要做什么**：此项尚未改成服务端 HttpOnly 登录 Cookie。若验收要求身份 Cookie 不可被 JS 读取，仍需继续改造服务端 Set-Cookie、Secure/SameSite、请求携带方式及 CSRF 防护；这是后续代码工作，不能仅靠部署本次改动完成。请测试方指出命中的具体 Cookie 名和值的用途，并用专用账号确认篡改 JWT 被拒绝，区分身份 Cookie 与普通统计 Cookie。 五个旧 chunk 文件名会随构建改变，应对应当前构建的 Cookie 操作点复查，不能只检查旧哈希文件是否还存在。
- **本地依据**：尚无该报告问题的修复后完整验证证据。代码/配置：[web/app/src/views/auth/login.tsx](/Users/topazzz/code/PandaWiki/web/app/src/views/auth/login.tsx:166)；[backend/middleware/jwt.go](/Users/topazzz/code/PandaWiki/backend/middleware/jwt.go:33)。

## 两份报告共有：主机问题

按管理端 PDF 顺序列出，用户端原编号与页码逐项标明。特别是管理端 3.2.1.5/3.2.1.6 对应用户端 3.2.1.6/3.2.1.5，不能只按数字直接对齐。

主机 CVE 的适用性说明沿用此前取得的[官方记录摘录](CVE官方记录摘录.json)与[核实说明](问题核实清单.md)，本次没有重新联网查询。报告服务器与本地实例不同；本地版本、未装模块、未监听端口都不能代替实际主机结论。

<a id="h01"></a>

### H01｜Redis NACK 双重释放 RCE 漏洞(CVE-2026-66373)

- **原报告位置**：管理端 `3.2.1.1`，第 29 页；用户端 `3.2.1.1`，第 26 页。
- **处理状态：待核实。** 有条件成立，未复现 RCE。
- **核实结果 / 已处理内容**：初次核查 Redis 7.4.8 的版本及 RESTORE 权限提示可能满足前提，未执行恶意载荷、未确认 RCE。当前匿名访问已在 H32 修复，但尚无该运行构建已包含 CVE-2026-66373 补丁的证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维核对 Redis 运行版本、镜像摘要和对应官方补丁，确认 NACK/RESTORE 相关修复是否已进入该构建，再升级受影响实例；同时限制 RESTORE 等高风险命令。不要运行恶意序列化载荷。认证加固不能替代该 CVE 补丁。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h02"></a>

### H02｜Redis 资源管理错误漏洞(CVE-2026-23631)

- **原报告位置**：管理端 `3.2.1.2`，第 29 页；用户端 `3.2.1.2`，第 26 页。
- **处理状态：待核实。** 当前不满足副本场景；仍不确定。
- **核实结果 / 已处理内容**：公告场景涉及可写副本同步/Lua，本地核查为 role:master；当前匿名访问另已加固。没有报告服务器的副本/配置与补丁证据，不能确认该漏洞或宣称补丁完成。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际 Redis 是否为副本、是否允许副本写入，以及 Lua/同步相关条件；若满足，核对对应厂商补丁并升级。保留角色与补丁证据；本地主节点结果不能替代生产副本检查。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h03"></a>

### H03｜Redis 安全漏洞(CVE-2026-25243)

- **原报告位置**：管理端 `3.2.1.3`，第 29 页；用户端 `3.2.1.3`，第 26 页。
- **处理状态：待核实。** 有条件成立，未复现 RCE。
- **核实结果 / 已处理内容**：初次核查的 Redis 版本及 RESTORE 权限提示可能满足前提，但未核实二进制补丁或发送恶意载荷。认证已加固不代表 CVE-2026-25243 已修复。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 Redis 二进制/镜像中 CVE-2026-25243 的补丁状态，以及认证用户可用的 RESTORE 权限；受影响时按厂商维护版本更新。无需通过恶意 RESTORE 或崩溃证明问题。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h04"></a>

### H04｜Redis 资源管理错误漏洞(CVE-2026-23479)

- **原报告位置**：管理端 `3.2.1.4`，第 30 页；用户端 `3.2.1.4`，第 27 页。
- **处理状态：待核实。** 有条件成立，未复现 RCE。
- **核实结果 / 已处理内容**：初次核查版本提示可能处于受影响范围，还需阻塞客户端/驱逐等触发条件；未复现 RCE，也未确认当前构建包含该 CVE 补丁。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 Redis 对 CVE-2026-23479 的修复记录及阻塞客户端、驱逐相关使用场景；受影响时更新维护版本。此项未实施专门 CVE 补丁，不能按“加了密码”销项。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h05"></a>

### H05｜OpenSSH 安全漏洞(CVE-2023-51767)

- **原报告位置**：管理端 `3.2.1.5`，第 30 页；用户端 `3.2.1.6`，第 27 页。
- **处理状态：待核实。** 疑似误报（报告定性）。
- **核实结果 / 已处理内容**：CVE-2023-51767 是需攻击者同机权限的 Rowhammer，Ubuntu 明确列 jammy Not affected；本地不是报告 Ubuntu 主机。不能按开放 SSH 端口认定远程高危。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：让运维提供 Ubuntu 版本、openssh-server/client 的完整包版本与来源；请测试方对照发行版 Not affected 状态及同机 Rowhammer 前提复核定级。拿到对应实例证据前记“疑似误报”，不是“代码已修复”。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h06"></a>

### H06｜NATS 未授权访问

- **原报告位置**：管理端 `3.2.1.6`，第 30 页；用户端 `3.2.1.5`，第 27 页。
- **处理状态：本地已修复。** 已修复本地匿名连接；部署模板已更新。
- **核实结果 / 已处理内容**：NATS 开启认证；本地无凭据 CONNECT 返回 Authorization Violation。API、Consumer、RAGLite、Crawler 统一凭据，容器间正常启动/订阅。4222 仅映射回环地址。未验证生产。
- **尚未完成 / 结论边界**：已完成本地认证与端口边界改动；正式环境尚未部署验证，不能据此宣称其他相关 CVE 已打补丁。
- **保留验收参考（无需你重复执行）**：在 deploy/server/.env 设置独立随机 NATS_PASSWORD，部署 NATS 及 API、Consumer、RAGLite、Crawler 等全部客户端。验证匿名 CONNECT 被拒绝，合法客户端能订阅，4222 不再对外部接口暴露。不要在记录中贴密码。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据 / 参考**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[部署模板](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml)。

<a id="h07"></a>

### H07｜OpenSSH 竞争条件问题漏洞(CVE-2024-6387)

- **原报告位置**：管理端 `3.2.1.7`，第 31 页；用户端 `3.2.1.7`，第 28 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：Ubuntu 22.04 修复于 1:8.9p1-3ubuntu0.10；报告显示 3ubuntu0.17，若为原厂包应含补丁。本地未运行报告实例。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际 openssh-server 包完整版本和官方 Ubuntu 来源，确认补丁已生效且运行进程已重启；若确为报告的 3ubuntu0.17，请测试方按发行版回补补丁重判并复扫。不要只比较上游 8.9p1 主版本。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h08"></a>

### H08｜OpenSSH 资源管理错误漏洞(CVE-2026-60002)

- **原报告位置**：管理端 `3.2.1.8`，第 31 页；用户端 `3.2.1.8`，第 28 页。
- **处理状态：待核实。** 范围不符，真实利用不确定。
- **核实结果 / 已处理内容**：官方 CVE-2026-60002 明确仅发生在 SSH 客户端重协商；扫描 sshd 监听版本不能证明服务端存在同样攻击面。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：请测试方说明是否实际测试 SSH 客户端重协商场景，并提供客户端二进制和版本依据；由运维核对对应客户端补丁。仅扫描 sshd 端口不足以确认该客户端漏洞。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h09"></a>

### H09｜F5 NGINX Open Source 资源管理错误漏洞(CVE-2026-42530)

- **原报告位置**：管理端 `3.2.1.9`，第 31 页；用户端 `3.2.1.9`，第 28 页。
- **处理状态：待核实。** 不确定（本地无对应 Nginx）。
- **核实结果 / 已处理内容**：需 HTTP/3 QUIC 模块/会话及受影响版本。仓库管理端模板只监听普通 HTTP；没有检查报告 18081/18082 实例。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对命中的是哪个 Nginx 实例，提供 nginx -V 与 HTTP/3/QUIC 的启用证据，再核对受影响版本和补丁。未启用相应能力应作为不适用证据提交测试方；其他业务实例交其负责人处理。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h10"></a>

### H10｜F5 NGINX Open Source 缓冲区错误漏洞(CVE-2026-42055)

- **原报告位置**：管理端 `3.2.1.10`，第 32 页；用户端 `3.2.1.10`，第 29 页。
- **处理状态：待核实。** 不确定（本地无对应 Nginx）。
- **核实结果 / 已处理内容**：官方条件包括 HTTP/2 上游或 grpc、ignore_invalid_headers off、大于 2MB 的 large_client_header_buffers 等；单看版本不够。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：针对报告 Nginx 实例核对 HTTP/2 上游或 grpc、ignore_invalid_headers、large_client_header_buffers 等必要配置及补丁；配置材料先脱敏。仅凭版本不能定为已可利用。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h11"></a>

### H11｜OpenSSH 资源管理错误漏洞(CVE-2026-60000)

- **原报告位置**：管理端 `3.2.1.11`，第 32 页；用户端 `3.2.1.11`，第 29 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：CVE-2026-60000 需 GSSAPIAuthentication 的认证重试场景；没有本地报告 sshd 配置或复现证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际 sshd 是否启用 GSSAPIAuthentication、对应认证重试条件和补丁；未满足则提供不适用证据，满足且未修复则由运维更新官方包并复测。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h12"></a>

### H12｜Redis handleClientsBlockedOnKey 远程代码执行漏洞

- **原报告位置**：管理端 `3.2.1.12`，第 32 页；用户端 `3.2.1.12`，第 29 页。
- **处理状态：待核实。** 不确定，可能与已有 Redis 条目重叠。
- **核实结果 / 已处理内容**：报告没有独立 CVE，称为 CVE-2026-23479 未完全修复变种；需测试方给独立公告/修复提交，不能多算一次已确认 RCE。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：请测试方提供此变种独立公告、CVE 或修复提交及与 CVE-2026-23479 的差异；先确认是否重复，拿到依据后再核对 Redis 补丁。当前不能凭无独立依据的描述确认又一个 RCE。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h13"></a>

### H13｜OpenSSH 安全漏洞(CVE-2026-35414)

- **原报告位置**：管理端 `3.2.1.13`，第 32 页；用户端 `3.2.1.13`，第 29 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：官方记录为 CA/authorized_keys principals 列表逗号等特殊证书场景；仅版本条件不足，本地无该 sshd 配置证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 SSH 证书 CA、authorized_keys principals 配置是否存在公告所述特殊列表场景，确认对应包补丁；配置不匹配时请测试方重判，勿按“开了 SSH 端口”直接定漏洞。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h14"></a>

### H14｜Redis RedisBloom TDigest 堆越界写漏洞

- **原报告位置**：管理端 `3.2.1.14`，第 33 页；用户端 `3.2.1.14`，第 30 页。
- **处理状态：待核实。** 本地不适用。
- **核实结果 / 已处理内容**：本地 Redis MODULE LIST 为空，没有 RedisBloom，报告要求的 TDigest 模块条件不存在。不能推广为其他环境也安全。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：在实际 Redis 执行只读 MODULE LIST，记录是否装有 RedisBloom 及版本；没有该模块则提交不适用证据，有则核对 TDigest 对应修复并升级模块。勿运行破坏性 TDigest 载荷。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h15"></a>

### H15｜OpenSSH/Dropbear 安全漏洞(CVE-2026-35385)

- **原报告位置**：管理端 `3.2.1.15`，第 33 页；用户端 `3.2.1.15`，第 30 页。
- **处理状态：待核实。** 范围不符，真实利用不确定。
- **核实结果 / 已处理内容**：官方条件是 scp 客户端以 root 使用 -O 且不带 -p 下载；不是仅远程 sshd 开放即可触发。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对是否存在 root 使用 scp -O 且不带 -p 从远程下载的客户端操作；确认实际 OpenSSH/Dropbear 产品和补丁。请测试方说明为何将客户端使用场景计为远端 sshd 高危。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h16"></a>

### H16｜Redis 资源管理错误漏洞(CVE-2026-66373)

- **原报告位置**：管理端 `3.2.1.16`，第 33 页；用户端 `3.2.1.16`，第 30 页。
- **处理状态：重复项，随 H01 处理。** 重复项。
- **核实结果 / 已处理内容**：与 H01 同为 CVE-2026-66373，原报告重复列出；未独立实施第二套修复，补丁状态随 H01 核实。
- **尚未完成 / 结论边界**：重复项不单独计修复成果；H01 尚未确认补丁。
- **你需要做什么**：与 H01 作为同一 CVE 一并核对 Redis 补丁与 RESTORE 权限，请测试方合并重复条目；原 PDF 编号保留用于追踪，不重复计算修复成果。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h17"></a>

### H17｜OpenSSH 信任管理问题漏洞(CVE-2026-59999)

- **原报告位置**：管理端 `3.2.1.17`，第 33 页；用户端 `3.2.1.17`，第 30 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：需核实 DisableForwarding=yes 与 PermitTunnel=yes 的实际组合，以及对应系统补丁。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 sshd 实际 DisableForwarding 与 PermitTunnel 组合，以及对应官方包补丁；满足受影响条件则修补，否则提供配置证据重新判定。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h18"></a>

### H18｜OpenSSH 安全漏洞(CVE-2023-28531)

- **原报告位置**：管理端 `3.2.1.18`，第 34 页；用户端 `3.2.1.18`，第 31 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：主要涉及 ssh-add/ssh-agent 智能卡目标约束；Ubuntu jammy 修复于 3ubuntu0.5，报告显示 0.17。仅 sshd 指纹不足。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 ssh-add/ssh-agent 包来源、智能卡目标约束场景及 Ubuntu 补丁。报告 3ubuntu0.17 若为原厂包应包含较早回补，需由测试方按完整包版本复核，不能只看 sshd banner。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h19"></a>

### H19｜Redis TLS Pending-List UAF 漏洞

- **原报告位置**：管理端 `3.2.1.19`，第 34 页；用户端 `3.2.1.19`，第 31 页。
- **处理状态：待核实。** 本地当前不适用；版本受影响状态不确定。
- **核实结果 / 已处理内容**：本地 CONFIG GET tls-port 为 0，当前没有 TLS 监听；官方提交证实 TLS pending-list UAF，但不能把崩溃修复直接等同所有配置均能 RCE。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际 Redis tls-port 和 TLS 使用情况，再核对 TLS pending-list 修复是否进入当前二进制。TLS 未启用可以说明当前场景不适用，但不能据此认定版本已打补丁。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h20"></a>

### H20｜检测到远端 RPCBIND/PORTMAP 正在运行中(CVE-1999-0632)

- **原报告位置**：管理端 `3.2.1.20`，第 34 页；用户端 `3.2.1.20`，第 31 页。
- **处理状态：待核实。** 本地未复现；报告环境不确定。
- **核实结果 / 已处理内容**：本地未发现 111/TCP/UDP 监听，不能据此推断报告服务器。此 CVE 记录本身是 RPC portmapper 运行信息。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维核实 111/TCP/UDP 的实际监听进程和 RPC/NFS 用途；不用则停用，需要则限制管理网访问并复测。该服务可能不属于本项目，先确认负责人，勿直接停用共享 NFS。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h21"></a>

### H21｜OpenSSH 安全漏洞(CVE-2023-38408)

- **原报告位置**：管理端 `3.2.1.21`，第 34 页；用户端 `3.2.1.21`，第 31 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：主要是 ssh-agent 转发/PKCS11 场景；Ubuntu jammy 修复于 3ubuntu0.3，报告显示 0.17。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 ssh-agent 转发、PKCS11 场景与完整官方包补丁；若确为报告 3ubuntu0.17，请测试方核对发行版回补后重判。仅远端 sshd 版本不足以证明 agent 利用链存在。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h22"></a>

### H22｜OpenSSH 路径遍历漏洞(CVE-2026-59996)

- **原报告位置**：管理端 `3.2.2.1`，第 35 页；用户端 `3.2.2.1`，第 32 页。
- **处理状态：待核实。** 范围不符，真实利用不确定。
- **核实结果 / 已处理内容**：scp 在两个远程目标间复制的客户端路径处理问题；没有对应使用场景或样本。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对是否使用 scp 在两个远程目标间复制、实际客户端版本及补丁；让测试方提供路径处理样本或场景依据。没有该客户端场景时不能靠 sshd 扫描确认。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h23"></a>

### H23｜OpenSSH 路径遍历漏洞(CVE-2026-59995)

- **原报告位置**：管理端 `3.2.2.2`，第 35 页；用户端 `3.2.2.2`，第 32 页。
- **处理状态：待核实。** 范围不符，真实利用不确定。
- **核实结果 / 已处理内容**：sftp 客户端连接恶意服务端并下载的路径问题；不能按普通 sshd 端口扫描确认。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 sftp 客户端是否连接不可信服务端下载、客户端补丁与路径处理证据；让测试方明确受影响端。普通 sshd 开放不等于该客户端漏洞存在。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h24"></a>

### H24｜OpenSSH 处理逻辑错误漏洞(CVE-2026-59998)

- **原报告位置**：管理端 `3.2.2.3`，第 35 页；用户端 `3.2.2.3`，第 32 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：需核实服务器处于 Windows AD/GSSAPI 场景和实际配置，当前没有证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际主机是否使用 Windows AD/GSSAPI 对应认证场景和公告所述条件，记录相关补丁；条件不符时提交不适用证据，条件匹配则由运维修补。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h25"></a>

### H25｜OpenSSH 输入验证错误漏洞(CVE-2026-59997)

- **原报告位置**：管理端 `3.2.2.4`，第 35 页；用户端 `3.2.2.4`，第 32 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：需实际使用 internal-sftp 且安全选项位于第 10 个及以后参数等条件；未取得运行配置。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：检查实际 internal-sftp 参数顺序与安全选项是否位于第 10 个及以后参数，核对对应官方包补丁；不要在没有运行配置的情况下直接认定漏洞。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h26"></a>

### H26｜OpenSSH 安全漏洞(CVE-2023-48795)

- **原报告位置**：管理端 `3.2.2.5`，第 36 页；用户端 `3.2.2.5`，第 33 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：Ubuntu openssh jammy 对 Terrapin 的修复是 3ubuntu0.5，报告为 0.17；应查包补丁与协商配置，不是统一强升主版本。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 Ubuntu OpenSSH 完整包版本、Terrapin 回补状态、实际协商配置和运行进程；若为报告原厂 3ubuntu0.17，请测试方按已回补版本重判，不必无依据地强升主版本。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h27"></a>

### H27｜OpenSSH 安全漏洞(CVE-2023-51385)

- **原报告位置**：管理端 `3.2.2.6`，第 36 页；用户端 `3.2.2.6`，第 33 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：SSH 客户端不可信用户名/主机名扩展场景；Ubuntu jammy 修复于 3ubuntu0.6，报告显示 0.17。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对实际 SSH 客户端是否处理不可信用户名/主机名扩展，以及 Ubuntu 包补丁；报告 3ubuntu0.17 若为原厂包，应让测试方核对较早的回补修复。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h28"></a>

### H28｜F5 Nginx Plus 缓冲区错误漏洞(CVE-2026-48142)

- **原报告位置**：管理端 `3.2.2.7`，第 36 页；用户端 `3.2.2.7`，第 33 页。
- **处理状态：待核实。** 不确定（本地无对应 Nginx）。
- **核实结果 / 已处理内容**：官方要求 location 中同时配置 source_charset utf-8 和 charset；仅看到版本号不能确认。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：确认 Nginx 产品、实际实例和 location 中是否同时配置 source_charset utf-8 与 charset，再查对应补丁；未满足条件就提交不适用材料，避免误改其他业务服务。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h29"></a>

### H29｜OpenSSH 资源管理错误漏洞(CVE-2026-60001)

- **原报告位置**：管理端 `3.2.2.8`，第 37 页；用户端 `3.2.2.8`，第 34 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：需实际 sshd 最小认证延迟行为和对应修复包，不做爆破或时序压力试验。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维核对 sshd 最小认证延迟相关实现、配置和官方补丁；向测试方索取该项的有效检测依据。不要使用爆破、耗尽资源或大并发时序测试。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h30"></a>

### H30｜OpenSSH 安全漏洞(CVE-2026-35387)

- **原报告位置**：管理端 `3.2.2.9`，第 37 页；用户端 `3.2.2.9`，第 34 页。
- **处理状态：待核实。** 不确定。
- **核实结果 / 已处理内容**：需检查 PubkeyAcceptedAlgorithms/HostbasedAcceptedAlgorithms 的 ECDSA 配置与实际协商，版本不是完整证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 PubkeyAcceptedAlgorithms、HostbasedAcceptedAlgorithms 的实际 ECDSA 配置与协商结果，确认对应包补丁；未有算法配置证据前暂记不确定。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h31"></a>

### H31｜NATS 监控端口未授权访问

- **原报告位置**：管理端 `3.2.2.10`，第 37 页；用户端 `3.2.2.10`，第 34 页。
- **处理状态：部分处理。** 已收紧监听边界；监控本身仍无认证。
- **核实结果 / 已处理内容**：8222 从所有网卡改为仅映射 127.0.0.1。宿主本机 /varz 仍可匿名 200，这是明确保留的本地管理边界；客户端 NATS 认证不保护监控 HTTP。若要求本机也鉴权，需前置认证代理。
- **尚未完成 / 结论边界**：本机监控 HTTP 仍允许匿名读取；本次只隔离对外端口，没有新增监控登录认证。
- **你需要做什么**：部署后确认 8222 仅映射回环，且没有上层代理或防火墙规则重新对外开放；从获授权外部测试位置验证不可达。若安全要求本机访问也必须鉴权，仍需配置认证反向代理；NATS_PASSWORD 不保护该 HTTP 监控接口。
- **本地依据 / 参考**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[部署模板](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml)。

<a id="h32"></a>

### H32｜Redis 未授权访问漏洞(CNNVD-201511-230)【原理扫描】

- **原报告位置**：管理端 `3.2.2.11`，第 38 页；用户端 `3.2.2.11`，第 35 页。
- **处理状态：本地已修复。** 已修复本地匿名访问；部署模板已更新。
- **核实结果 / 已处理内容**：Redis 开启 requirepass；无凭据原始 TCP PING 返回 NOAUTH，合法 API/Consumer 使用同一配置运行。16379 仅回环映射。此认证修复不等于修复任意 Redis CVE。
- **尚未完成 / 结论边界**：已完成本地认证与端口边界改动；正式环境尚未部署验证，不能据此宣称其他相关 CVE 已打补丁。
- **保留验收参考（无需你重复执行）**：设置独立随机 REDIS_PASSWORD，统一部署 Redis 与所有客户端。验证无凭据 PING 返回 NOAUTH、合法凭据 PING 返回 PONG，并确认正式环境 Redis 宿主映射仅回环且无外部旁路。此项只修未授权，不代表其他 Redis CVE 修复。
- **用户处理（2026-10-26）**：接受已有本地修复结论，无需用户重复验证；正式部署另行处理，生产状态仍未验证。
- **本地依据 / 参考**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[部署模板](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml)。

<a id="h33"></a>

### H33｜OpenSSH 安全漏洞(CVE-2023-51384)

- **原报告位置**：管理端 `3.2.2.12`，第 38 页；用户端 `3.2.2.12`，第 35 页。
- **处理状态：待核实。** 不确定；服务端扫描依据不足。
- **核实结果 / 已处理内容**：CVE-2023-51384 涉及 ssh-agent PKCS11 多密钥约束；本轮 Ubuntu 页面未成功读取，不能臆测包补丁结论。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 ssh-agent/PKCS11 多密钥约束使用场景，以及实际 openssh-client 包对应公告/补丁；请测试方补充服务端扫描与该客户端问题的关联。当前缺补丁核对证据，不能直接写误报。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h34"></a>

### H34｜OpenSSH 安全漏洞(CVE-2025-26465 )

- **原报告位置**：管理端 `3.2.2.13`，第 38 页；用户端 `3.2.2.13`，第 35 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：需 SSH 客户端启用 VerifyHostKeyDNS 等；Ubuntu jammy 修复于 3ubuntu0.11，报告显示 0.17。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 SSH 客户端 VerifyHostKeyDNS 是否启用、Ubuntu 包来源和 CVE 回补；报告 3ubuntu0.17 若为原厂包应包含 0.11 的修复，让测试方据此复核。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h35"></a>

### H35｜PostgreSQL Server 版本获取漏洞

- **原报告位置**：管理端 `3.2.3.1`，第 39 页；用户端 `3.2.3.1`，第 35 页。
- **处理状态：待核实。** 信息项；未证实匿名越权。
- **核实结果 / 已处理内容**：本地 Docker 镜像是 PostgreSQL 15.17，能用获授权的容器 psql 查询版本不等于外部匿名可读数据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：确认数据库只对必要网络开放、未授权连接不能执行查询，再核对版本信息是否通过应用错误意外暴露。最新模板已收紧宿主端口，但未改变 PostgreSQL 协议本身；获授权查询版本不是匿名越权。
- **本地依据 / 参考**：[初始本地服务证据](本地服务与文档核查证据.json)（历史记录，不代表当前匿名认证状态）；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h36"></a>

### H36｜OpenSSH 安全漏洞(CVE-2025-61985)

- **原报告位置**：管理端 `3.2.3.2`，第 39 页；用户端 `3.2.3.2`，第 35 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：SSH 客户端 ProxyCommand 与特制 ssh:// URI 场景；Ubuntu jammy 修复于 3ubuntu0.14，报告显示 0.17。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 SSH 客户端 ProxyCommand 与特制 ssh:// URI 场景、完整 Ubuntu 包版本和补丁。若确为报告原厂 3ubuntu0.17，请测试方按 0.14 回补重判；不把 sshd 扫描直接当客户端利用证据。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h37"></a>

### H37｜OpenSSH（OpenBSD Secure Shell） 安全漏洞(CVE-2025-32728)

- **原报告位置**：管理端 `3.2.3.3`，第 39 页；用户端 `3.2.3.3`，第 36 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：DisableForwarding 约束问题在 Ubuntu jammy 修复于 3ubuntu0.13，报告显示 0.17；仍需确认包来源及实际启用设置。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 sshd DisableForwarding 实际配置、Ubuntu 包补丁及重启状态；若为报告原厂 3ubuntu0.17，请测试方核对 0.13 回补后重判。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h38"></a>

### H38｜可通过 HTTP(S)获取远端 WWW 服务信息

- **原报告位置**：管理端 `3.2.3.4`，第 39 页；用户端 `3.2.3.4`，第 36 页。
- **处理状态：部分处理。** 应用指纹已加固；其余服务信息仍按原边界核实。
- **核实结果 / 已处理内容**：本地 Next.js 不再返回 X-Powered-By，临时 Nginx 仅返回 Server: nginx，不带版本。未修改或核查其他报告端口的服务。
- **尚未完成 / 结论边界**：Server: nginx 通用头仍在，报告其他端口未核实；不是所有服务信息都已隐藏。
- **你需要做什么**：部署前端后核对最终入口 X-Powered-By 和带版本的 Server 是否消失；通用 Server: nginx 仍存在。报告其余 HTTP 端口由对应服务负责人逐个核对，不以本项目响应代替全部主机响应。
- **本地依据 / 参考**：[11 项响应头/基础设施边界验证](修复后边界HTTP验证.json)；[部署模板](/Users/topazzz/code/PandaWiki/deploy/server/docker-compose.yml)。

<a id="h39"></a>

### H39｜OpenSSH 安全漏洞(CVE-2025-61984)

- **原报告位置**：管理端 `3.2.3.5`，第 40 页；用户端 `3.2.3.5`，第 36 页。
- **处理状态：待核实。** 疑似误报（按报告包版本）。
- **核实结果 / 已处理内容**：SSH 客户端使用 ProxyCommand 加不可信用户名；Ubuntu jammy 修复于 3ubuntu0.14，报告显示 0.17。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 SSH 客户端 ProxyCommand 加不可信用户名的使用场景与官方 Ubuntu 包补丁；报告原厂 3ubuntu0.17 应按较早 0.14 回补让测试方复核，不能只看上游版本。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h40"></a>

### H40｜SMTP 服务器版本信息可被获取

- **原报告位置**：管理端 `3.2.3.6`，第 40 页；用户端 `3.2.3.6`，第 37 页。
- **处理状态：待核实。** 本地未复现；报告环境不确定。
- **核实结果 / 已处理内容**：本地未发现 25/TCP SMTP 监听，项目没有 SMTP 服务容器；知道服务版本不是弱口令或入侵证据。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维确认 25/TCP SMTP 服务归属和业务用途，检查 banner 是否暴露多余版本；无须对业务网开放则限制网络。项目没有该容器，本轮未修它；不要直接停用共享邮件服务。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h41"></a>

### H41｜目标主机 rpcinfo -p 信息泄露

- **原报告位置**：管理端 `3.2.3.7`，第 40 页；用户端 `3.2.3.7`，第 37 页。
- **处理状态：待核实。** 本地未复现；报告环境不确定。
- **核实结果 / 已处理内容**：本地未发现 RPC 111 监听，没有运行 rpcinfo 请求去报告服务器。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维核对 RPC 111/TCP/UDP 是否允许非管理网查询 rpcinfo；按业务需要收紧或停用，再由测试方复测。与 H20 同源但原编号保留，不能把本地无监听写为生产已修。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h42"></a>

### H42｜OpenSSH 安全漏洞(CVE-2026-35388)

- **原报告位置**：管理端 `3.2.3.8`，第 40 页；用户端 `3.2.3.8`，第 37 页。
- **处理状态：待核实。** 范围不符，真实利用不确定。
- **核实结果 / 已处理内容**：SSH 客户端 proxy-mode 复用会话确认，不是仅 sshd 端口开放即可证明。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：核对 SSH 客户端 proxy-mode 多路复用及连接确认场景、实际客户端版本和补丁；要求测试方给出客户端场景证据，仅 sshd banner 不能确认。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h43"></a>

### H43｜SSH 版本信息可被获取

- **原报告位置**：管理端 `3.2.3.9`，第 41 页；用户端 `3.2.3.9`，第 37 页。
- **处理状态：待核实。** 本地未复现；属信息项。
- **核实结果 / 已处理内容**：本地未发现报告 22022 监听；SSH banner 本身是协议识别信息，不能单独当作认证绕过。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：由运维核对实际 22022 SSH 服务及允许来源，限制为管理网/必要运维来源；评估是否隐藏发行版附加信息。不要破坏 SSH 协议所需标识，版本可见本身不等于认证绕过。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

<a id="h44"></a>

### H44｜探测到 SSH 服务器支持的算法【原理扫描】

- **原报告位置**：管理端 `3.2.3.10`，第 41 页；用户端 `3.2.3.10`，第 37 页。
- **处理状态：待核实。** 信息项；没有弱算法证据。
- **核实结果 / 已处理内容**：报告只说获取算法列表，未列出被实际接受的弱算法及协商证据，因此不能确认独立可利用漏洞。
- **尚未完成 / 结论边界**：没有为报告对应实例实施或证实该项专门修复。需按下述前提与补丁核对，确认适用后再修；不满足条件或已含官方补丁时提交证据给测试方重判。
- **你需要做什么**：请测试方列出真正被服务端接受并成功协商的弱算法及依据，运维再核对有效 sshd 配置。只禁用确认的弱算法并回归运维连接；仅“能获取算法列表”不足以证明漏洞。
- **本地依据 / 参考**：[原逐项核实依据与官方补丁对照](问题核实清单.md)；[官方记录摘录](CVE官方记录摘录.json)。

## 报告其他章节：弱口令结果与端口清单

这两节不是新增漏洞编号，但涉及你部署后的验收，不能漏掉。

| 原报告位置 | 报告内容与当前判断 | 你需要做什么 |
| --- | --- | --- |
| 管理端第 41 页第 4 节；用户端第 38 页第 4 节 | 报告弱口令统计未检出弱口令；管理端数据库一栏为“--”。“密码规则简单”则是允许未来设置弱密码的策略问题，两者不是同一结论。 | 复测 M11 的服务端密码策略和强制首次改密；统计未检出不能代表全部账号密码均安全，也不能推断管理端数据库已检查。 |
| 管理端第 42 页第 5 节；用户端第 39 页第 5 节 | 报告枚举主机开放端口，不能把全部端口自动归为项目。新模板已收紧部分基础设施宿主映射，但实际主机仍未核对。 | 运维逐个确认进程、容器、业务用途、监听地址与防火墙；重点检查 3000、4222、5050、5432、6333、6379、8000、8222 是否应对外开放。邮件、RPC、其他 Nginx 由其所属负责人处理。 |

报告端口表把 4222 识别成 mongodb/MongoDB 14.1，与本项目 NATS 及 H06 描述不一致，应让测试方确认实际协议和实例。对这类指纹误识别先核实服务归属，不据此执行 MongoDB 升级。

## 用户端主机条目的原顺序索引

若你从用户端 PDF 逐条对照，按此顺序跳到上面的 H 项；表中只做定位，不重复统计。

| 用户端原编号 | 用户端页码 | 跟踪项 | 管理端原编号 |
| --- | ---: | --- | --- |
| 3.2.1.1 | 26 | [H01](#h01) | 3.2.1.1 |
| 3.2.1.2 | 26 | [H02](#h02) | 3.2.1.2 |
| 3.2.1.3 | 26 | [H03](#h03) | 3.2.1.3 |
| 3.2.1.4 | 27 | [H04](#h04) | 3.2.1.4 |
| 3.2.1.5 | 27 | [H06](#h06) | 3.2.1.6 |
| 3.2.1.6 | 27 | [H05](#h05) | 3.2.1.5 |
| 3.2.1.7 | 28 | [H07](#h07) | 3.2.1.7 |
| 3.2.1.8 | 28 | [H08](#h08) | 3.2.1.8 |
| 3.2.1.9 | 28 | [H09](#h09) | 3.2.1.9 |
| 3.2.1.10 | 29 | [H10](#h10) | 3.2.1.10 |
| 3.2.1.11 | 29 | [H11](#h11) | 3.2.1.11 |
| 3.2.1.12 | 29 | [H12](#h12) | 3.2.1.12 |
| 3.2.1.13 | 29 | [H13](#h13) | 3.2.1.13 |
| 3.2.1.14 | 30 | [H14](#h14) | 3.2.1.14 |
| 3.2.1.15 | 30 | [H15](#h15) | 3.2.1.15 |
| 3.2.1.16 | 30 | [H16](#h16) | 3.2.1.16 |
| 3.2.1.17 | 30 | [H17](#h17) | 3.2.1.17 |
| 3.2.1.18 | 31 | [H18](#h18) | 3.2.1.18 |
| 3.2.1.19 | 31 | [H19](#h19) | 3.2.1.19 |
| 3.2.1.20 | 31 | [H20](#h20) | 3.2.1.20 |
| 3.2.1.21 | 31 | [H21](#h21) | 3.2.1.21 |
| 3.2.2.1 | 32 | [H22](#h22) | 3.2.2.1 |
| 3.2.2.2 | 32 | [H23](#h23) | 3.2.2.2 |
| 3.2.2.3 | 32 | [H24](#h24) | 3.2.2.3 |
| 3.2.2.4 | 32 | [H25](#h25) | 3.2.2.4 |
| 3.2.2.5 | 33 | [H26](#h26) | 3.2.2.5 |
| 3.2.2.6 | 33 | [H27](#h27) | 3.2.2.6 |
| 3.2.2.7 | 33 | [H28](#h28) | 3.2.2.7 |
| 3.2.2.8 | 34 | [H29](#h29) | 3.2.2.8 |
| 3.2.2.9 | 34 | [H30](#h30) | 3.2.2.9 |
| 3.2.2.10 | 34 | [H31](#h31) | 3.2.2.10 |
| 3.2.2.11 | 35 | [H32](#h32) | 3.2.2.11 |
| 3.2.2.12 | 35 | [H33](#h33) | 3.2.2.12 |
| 3.2.2.13 | 35 | [H34](#h34) | 3.2.2.13 |
| 3.2.3.1 | 35 | [H35](#h35) | 3.2.3.1 |
| 3.2.3.2 | 35 | [H36](#h36) | 3.2.3.2 |
| 3.2.3.3 | 36 | [H37](#h37) | 3.2.3.3 |
| 3.2.3.4 | 36 | [H38](#h38) | 3.2.3.4 |
| 3.2.3.5 | 36 | [H39](#h39) | 3.2.3.5 |
| 3.2.3.6 | 37 | [H40](#h40) | 3.2.3.6 |
| 3.2.3.7 | 37 | [H41](#h41) | 3.2.3.7 |
| 3.2.3.8 | 37 | [H42](#h42) | 3.2.3.8 |
| 3.2.3.9 | 37 | [H43](#h43) | 3.2.3.9 |
| 3.2.3.10 | 37 | [H44](#h44) | 3.2.3.10 |

## 验收记录怎么填写

每项可在工单中直接记录：`跟踪编号 / 部署版本 / 复测日期 / 复测账号或环境 / 脱敏结果 / 测试方结论`。需要测试方提供原样本、CVE、具体配置或命中请求的条目，先记“待材料”；没有证据不要填“已修复”。

已完成验证的实际范围：23 项会话/密钥/评论 HTTP 断言、11 项响应头与基础设施边界检查、后端与本地数据库事务测试、两端 TypeScript、管理端生产构建及登录页检查。未完成用户端完整 Next 生产构建；未访问外部 OSS、未清洗历史对象、未完成原恶意 PDF 复测；未穷举权限组合，也未部署或复测生产。

详细测试命令、结果和回滚限制见[修复与部署说明](修复与部署说明.md)；历史发现与代码位置见[原问题核实清单](问题核实清单.md)；全量合并参考见[核实与修复部署合并说明](安全问题核实与修复部署合并说明.md)。本清单的当前结论优先区分修复前证据与修复后结果，避免把旧 nopass/匿名连接记录误当作已加固后的现状。

## 原始 PDF 来源

- [管理端安全测试报告（2026-10-13，42 页）](/Users/topazzz/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/wxid_nsu08ytjxhxu22_5027/msg/file/2026-10/202610-知识库管理系统管理端安全测试报告-第一轮.pdf)。
- [用户端安全测试报告（2026-10-14，39 页）](/Users/topazzz/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/wxid_nsu08ytjxhxu22_5027/temp/drag/202610-知识库管理系统用户端安全测试报告-第一轮(1).pdf)。
