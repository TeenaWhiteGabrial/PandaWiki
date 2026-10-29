# 取消尚未提交给 RAGLite 的任务

管理端文档管理页面的“后台任务”弹窗增加“取消未提交任务”。此操作仅超级管理员可用，范围是当前知识库全部未提交任务，不受弹窗筛选条件限制。

- 已登记的排队、解析任务：标记取消，提交前再次检查，不再发送给 RAGLite。
- 已提交、已开始提交的任务：保留并继续处理，包括 HTTP 上传请求正在等待响应的情况。
- 无任务 ID 的旧版全量重建：将当前 NATS 队列序号与消息时间边界保存到数据库。Consumer 恢复后确认并跳过边界内的旧消息，不清空整个队列。
- 取消后新提交的任务、其他知识库、摘要和删除消息不受旧队列取消标记影响。
- 不删除原文档、已有索引或 RAGLite 中正在处理的文档。取消不会恢复被旧版全量重建替换的旧数据集。

旧版队列缺少批次和完整任务记录，界面不会捏造其精确取消数量。提示中的数量仅为已登记任务；旧队列单独说明已保存取消标记。

## 本次生产升级顺序

先提交、推送并在服务器拉取代码。现有 Consumer 已停止时，保持停止，RAGLite 可以继续处理已接收的文档。

```bash
cd /opt/WikiCode/PandaWiki
KEEP_CONSUMER_STOPPED=1 bash deploy/server/deploy.sh upgrade backend
bash deploy/server/deploy.sh upgrade admin
```

该选项仅支持 `upgrade backend`，构建新 Consumer 容器但不启动；失败镜像回滚路径也不会启动 Consumer。不使用该选项的升级行为保持原样。

超级管理员打开“后台任务”，点击“取消未提交任务”，确认看到“取消标记已保存”后，再恢复 Consumer：

```bash
docker start pandawiki-consumer
```

必须先完成 API、Consumer、管理端三项升级。旧版 Consumer 不识别取消标记，不要在新旧代码混用时提前启动旧 Consumer。恢复后它会跳过已取消旧消息，并恢复处理完成通知与后续新任务；RAGLite 的已接收任务继续运行。

## 本地验证

`deploy/server/tests/upgrade-paused-consumer.sh` 使用桩函数验证普通升级、保持停止的升级和失败回滚路径，不调用 Docker。后端包含 PostgreSQL 回滚事务测试，以及需隔离 NATS 的真实消息边界测试。
