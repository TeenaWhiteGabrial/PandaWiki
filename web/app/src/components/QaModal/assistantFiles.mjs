export function assistantConversationFiles(runs) {
  const files = new Map();
  for (const run of runs) {
    if (run.state !== 'completed' || !run.artifact_id || run.error) continue;
    const file = run.artifact || {
      id: run.artifact_id,
      family_id: run.artifact_id,
      revision: 1,
      name: run.question || '生成文件',
      format: run.format || '',
      created_at: run.created_at,
    };
    const family = file.family_id || file.id;
    const old = files.get(family);
    if (!old || file.revision > old.revision) files.set(family, file);
  }
  return [...files.values()].sort((a, b) =>
    (a.created_at || '').localeCompare(b.created_at || ''),
  );
}

export function assistantRevisionTarget(message, runs) {
  if (/(新建|另外|另一份|新做|全新|(?:生成|制作|做).{0,4}新的)/.test(message))
    return { editing: false, fresh: true };
  let files = assistantConversationFiles(runs);
  const reading =
    /(解释|讲解|总结|概括|分析|比较|什么意思|说了什么)/.test(message) &&
    (/(刚才|上一份|这份|这个文件|第.{1,8}(页|章|节|表))/.test(message) ||
      files.some(
        f => f.name && message.includes(f.name.replace(/\.[^.]+$/, '')),
      ));
  const editing =
    /(修改|调整|改成|改为|精简|润色|压缩|删掉|删除|第.{1,4}(页|章|节)|封面|这个文件|这份文件)|(把这个|把这份|把刚才).{0,12}(导出|转成|转为)/i.test(
      message,
    );
  if (!editing && !reading) return { editing: false };
  const kind = /ppt|幻灯片|演示/i.test(message)
    ? 'pptx'
    : /word|章节/i.test(message)
      ? 'docx'
      : /excel|工作表/i.test(message)
        ? 'xlsx'
        : '';
  // Conversion wording names the destination; do not filter away its source.
  if (kind && !/(导出|转成|转为|改成|改为)/.test(message))
    files = files.filter(f => f.format === kind || !f.format);
  if (files.length === 1) return { editing: true, id: files[0].id };
  if (files.length > 1 && /(刚才|上一份|最新)/.test(message))
    return { editing: true, id: files.at(-1).id };
  const named = files.filter(
    f => f.name && message.includes(f.name.replace(/\.[^.]+$/, '')),
  );
  if (named.length === 1) return { editing: true, id: named[0].id };
  return { editing: true, ambiguous: files.length > 1, files };
}
