// User-facing task states describe what can happen next. Technical causes stay
// in the API/worker logs and browser diagnostics, never in task-card copy.
export function assistantTaskView(run) {
  if (run.state === 'canceled') return { label: '已结束', detail: '' };
  if (run.error && /权限|无权|访问范围|access|forbidden/i.test(run.error))
    return {
      label: '资料范围已更新',
      detail: '请重新选择当前可访问的资料后继续。',
    };
  if (run.state === 'failed')
    return {
      label: '已暂停',
      detail: '任务信息已保留，你可以继续处理或结束任务。',
    };
  if (run.state === 'completed')
    return {
      label: run.artifact_id ? '已完成' : '已回复',
      detail: run.error ? '任务信息已保留，可稍后查看。' : '',
    };
  const states = {
    queued: '等待处理',
    awaiting_input: '等待补充需求',
    awaiting_sources: '等待补充资料',
    awaiting_outline: '等待确认大纲',
  };
  if (states[run.state]) return { label: states[run.state], detail: '' };
  const stages = {
    检查附件与模板: '准备资料与模板',
    收集资料: '整理参考资料',
    联网搜索: '查找公开资料',
    整理内容: '组织内容',
    检查资料与规划大纲: '分析需求与规划内容',
    修正需求计划格式: '正在调整内容方案',
    需求计划格式校验失败: '正在整理任务状态',
    需求计划校验通过: '准备内容方案',
    开始生成内容: '开始制作你的文件',
    自动继续整理: '已保留进度，正在继续整理',
    正式内容校验通过: '内容已整理，准备生成文件',
    调整正式内容格式: '正在整理文件内容',
    读取知识库完整目录: '读取有权查阅的完整目录',
    核对文件大小: '核对原文件大小',
    生成完整目录册: '正在生成完整目录册',
    生成文件: '正在生成文件',
    检查文件: '检查文件完整性',
    保存文件: '保存生成结果',
    生成预览: '准备文件预览',
    保存修改版本: '正在保存新版本',
    整理文件说明: '正在解读文件',
  };
  return { label: stages[run.stage] || '正在处理', detail: '' };
}

export function assistantNotice(operation, problem) {
  const code = problem?.code || problem?.status;
  if (code === 401 || /未登录|登录.*过期/.test(problem?.message || ''))
    return '请登录后继续，会话中的任务信息会保留。';
  if (code === 403 || /无权|权限.*变|来源权限/.test(problem?.message || ''))
    return '资料访问范围有变化，请重新选择当前可访问的资料。';
  return (
    {
      sync: '正在同步任务进度，稍后会自动更新。',
      submit: '这次请求尚未确认提交，请再次发送；已选资料会保留。',
      continue: '本次操作尚未确认，请稍后继续；已填写的内容会保留。',
      upload: '这份资料还未添加，可再次上传或换一份资料。',
      attachment: '这份资料暂未准备好，可继续处理或换一份资料。',
      documents: '资料列表暂未更新，可稍后重新搜索或上传补充资料。',
      settings: '当前选择已保留在本页，可稍后继续设置。',
      preview: '文件暂未准备好预览，可稍后查看或下载文件。',
      download: '文件暂未准备好下载，请稍后再次下载。',
      chat: '这次回答已暂停，你可以稍后再次发送。',
      history: '会话暂未同步完成，请稍后重新打开。',
    }[operation] || '任务信息已保留，可稍后继续。'
  );
}

export function reportAssistantIssue(operation, problem) {
  console.warn(`[assistant:${operation}]`, problem);
  return assistantNotice(operation, problem);
}
