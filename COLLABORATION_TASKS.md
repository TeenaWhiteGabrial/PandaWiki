# 协同编辑长期任务列表

## 阶段 1：协同服务持久化升级

目标：让 Yjs 协同内容可恢复、可压缩、不会无限膨胀。

1. 梳理 `web/packages/collab-service/src/server.mjs` 当前房间生命周期。
2. 设计磁盘目录结构：`snapshot.bin`、`updates.log` 或 updates 分片、`meta.json`。
3. 实现房间启动恢复：先 apply snapshot，再 apply updates。
4. 实现 `doc.on('update')` 追加保存 incremental update。
5. 实现 compact：把当前 Y.Doc 编码成 snapshot，原子替换，清理已 compact updates。
6. 增加 compact 触发条件：update 数量、update 总字节数、定时器。
7. 增加异常保护：compact 失败不删除旧 updates，写入失败要日志告警。
8. 增加 room stats 日志或调试接口。
11. 增加 room 保护：空房间 TTL 卸载、最大加载 room 数、单用户单文档 block room 限制。
9. 编写或补充本地验证脚本或最小测试。
10. 手动验证：编辑、断开、重启服务、恢复、compact 后恢复。

## 阶段 2：富文本 ProseMirror 协同闭环

目标：让 `collab.mode=prose_mirror` 可以真正用于富文本协同。

1. 明确后端开启条件：只允许 HTML/富文本文档。
2. 前端打开富文本时优先用 `editor_content` 初始化。
3. Y.Doc 初始化时注入 ProseMirror 初始内容。
4. Tiptap Collaboration 只同步 ProseMirror 结构，不同步 HTML 字符串。
5. 保存时写 `editor_content`，并导出 HTML 到 `content`。
6. 验证双人编辑、刷新恢复、断线重连。
7. 验证 undo/redo 行为。
8. 关闭或删除旧 legacy 字符串协同入口。

## 阶段 3：Markdown 长期方案

目标：决定 Markdown 的长期协同路径。

1. 评估 Markdown 转 ProseMirror 的格式损失。
2. 选择策略：统一结构化编辑，或 CodeMirror + Yjs 源码协同。
3. 如果选结构化：实现 Markdown -> ProseMirror JSON。
4. 如果选源码：新增 CodeMirror 协同编辑路径。
5. 处理 Markdown/富文本互转、表格、代码块、图片、链接。
6. 设定 Markdown 大文档保护策略。
7. 验证 Markdown 打开、保存、导出、发布。

## 阶段 4：长文档块级协同

目标：解决 500KB/1MB 文档的长期性能问题。

1. 设计 block 数据模型：`block_id`、类型、顺序、内容。`editor_blocks` 已作为块索引字段落库，暂不替代 `editor_content`。
2. 实现文档切块和稳定 block_id。富文本保存时已根据 ProseMirror JSON 生成块索引，并记录 ProseMirror 节点 `path`。
3. 拆分文档顺序与 block 内容。后端 `node/collab` 已返回 `doc_room_id`、`block_room_prefix`、`block_window`，并对长文档返回 `mode=block`。
4. 当前视口附近 block 才启用协同。前端已能识别块级预备模式，并会按当前编辑位置移动 block room 调度窗口。
5. 单 block 编辑只同步该 block。block room 已能初始化对应块的结构化内容、本地当前块编辑写入 room，并能把 room 中的块内容合并回整篇 `editor_content` 保存链路。
6. 大纲、字数、摘要异步计算。
7. 增加长文档性能 guardrail。
9. 协同服务已增加 block room 连接保护和空房间清理。
8. 压测 100KB、500KB、1MB 文档。

## 阶段 5：业务链路适配

目标：确保结构化内容不破坏现有业务。

1. 发布时从 `editor_content` 导出 `content`。
2. 历史版本保存和恢复结构化内容。
3. 复制文档带上结构化字段。
4. 贡献审核支持结构化字段。
5. 搜索/RAG 使用导出文本，不直接读 ProseMirror JSON。
6. 导入/导出支持 Markdown/HTML/结构化内容。
7. Swagger 重新生成。
8. 前端 generated client 重新生成，移除手写类型补丁。

## 阶段 6：完整验证和清理

目标：进入可提交/可发布状态。

1. 后端跑受影响包测试。
2. 后端跑 `make lint`。
3. 前端跑 `pnpm exec tsc -b`。
4. 前端跑目标 ESLint。
5. 修复 admin build OOM 或用明确 Node heap 参数验证。
6. 清理临时性能日志或改成配置开关。
7. 删除不用的 legacy 协同代码。
8. 更新开发/部署文档。
