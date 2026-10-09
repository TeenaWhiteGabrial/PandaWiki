# 智能问答会话延续

## 使用行为

- 网页问答和嵌入窗口顶部提供“历史记录”和“新会话”入口。历史列表按最近对话时间排序，支持标题搜索和分页；选择记录后加载历史问答，直接在下方继续提问。
- 网页问答框默认最大宽度 1120 px、最大高度 900 px，并随窗口自适应。右上角支持全屏/退出全屏；全屏时 Esc 先返回普通窗口，再按 Esc 关闭。消息区独立滚动，输入区保持在底部，切换尺寸保留当前会话和未发送内容；手机端直接铺满可用屏幕。
- 已登录用户读取当前知识库中属于自己账号的会话，换浏览器登录后也可以查看。匿名用户在当前浏览器保留最近 100 个会话凭证，关闭后重新打开仍可继续；清除浏览器数据后无法自动找回。列表不会列出其他用户的会话，匿名列表也不会显示已登录账号的历史。
- 在同一会话中追问时，历史问题和回答同时用于检索和回答。
- 刷新页面后，地址栏的 `cid` 保留，会重新加载历史内容；历史加载完成后可以继续追问。
- 已登录用户打开自己的历史会话，可直接接回原会话。
- 匿名用户通过保存在当前浏览器的会话凭证恢复原会话；会话凭证不进入分享地址。
- 打开分享的历史对话后，首次追问会复制历史到独立会话，再发送新问题；不会修改分享者的原记录。旧匿名历史没有本地凭证时同样采用副本续聊。
- 点击“新对话”会清空当前会话身份和内容；旧历史仍然可以重新打开。
- “上述第二条”“把刚才的回答整理成表格”等请求，保留原始问题，同时使用结合历史生成的检索问题。纯粹整理前文时允许使用已有回答；需要新事实时仍依据可访问文档。
- 以上行为适用于网页问答和嵌入式问答窗口。

## 兼容接口

`POST /share/v1/chat/completions` 使用调用方 `messages` 中按顺序排列的用户和助手消息，并保留 system/developer 文本要求。最后一条必须是非空的用户问题。

兼容接口按请求传递历史，调用方每次应发送完整问答消息。新规则处理文本会话；不支持的 tool 等角色会明确返回错误，不再悄悄丢弃消息。此改动不增加跨会话的个人长期记忆。

## 历史列表接口

`POST /share/v1/conversation/history` 使用当前请求的知识库和已验证身份，不接受客户端指定用户。请求包括 `page`、`per_page`（最多 50）、`query`（标题关键字），以及匿名浏览器提供的 `sessions: [{id, nonce}]`（最多 100）。登录用户只查询账号自己的记录；匿名查询同时校验会话 ID、凭证及知识库，只返回匿名会话。结果为 `{data: [{id, subject, created_at, updated_at}], total}`，不返回会话凭证或用户资料。

选择记录后通过现有详情接口读取内容，首次追问时调用 `/share/v1/conversation/resume` 获得原会话的有效身份，再将 `conversation_id` 和 `nonce` 传给问答接口。切换会话会终止旧请求并使旧回调失效，历史加载失败时允许重试或新建会话。

## 部署

更新后端 **api** 和前台 **app** 服务（包括嵌入窗口页面）。不涉及数据库结构迁移，不需要因本次改动更新 consumer、admin 或基础设施服务。

## 验证

前端会话状态测试，在仓库根目录运行（Node.js 22）：

```bash
node --experimental-strip-types --test web/app/src/utils/chatSession.test.mjs
```

前端类型检查，在 `web/app` 运行：

```bash
../node_modules/.bin/tsc --noEmit --pretty false --incremental false
```

后端检查，在 `backend` 运行：

```bash
GOCACHE=/tmp/pandawiki-go-build go test ./domain ./usecase ./handler/share ./repo/pg ./middleware ./cmd/api
```

数据库回归使用独立测试数据，整个事务结束后自动回滚。设置 `PANDA_WIKI_TEST_PG_DSN` 为本地测试数据库连接串，再运行：

```bash
go test ./usecase -run 'TestConversationHistoryIsolationAndResumePostgres|TestResumeConversationPostgres|TestFollowupContextReachesRetrievalAndModelPostgres' -count=1 -v
```

可选真实模型验证只发送合成的三个步骤示例，不使用业务文档，数据库记录自动回滚。需要本地配置的模型连接及密钥有效，并设置上述数据库环境变量：

```bash
PANDA_WIKI_TEST_LIVE_LLM=1 go test ./usecase -run TestLiveFollowupAnswer -count=1 -v
```

本次检查：会话状态 8 项测试、数据库会话及上下文回归、历史列表权限隔离/排序/分页/搜索回归、后端包测试、前端类型检查通过。浏览器已验证历史列表、标题搜索、打开记录、切换记录、刷新恢复和新会话；测试使用独立合成记录并在结束后清理。真实模型回答效果仍需有效模型凭证验证；此前本地模型调用返回 401（API Key 无效），此轮未再次调用真实模型。
