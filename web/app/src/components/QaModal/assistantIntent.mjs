// Keep explicit file requests conversational: users need not use a fixed verb.
// Instructions about how to use the feature remain ordinary questions.
export function assistantFileIntent(message) {
  const text = message.trim().toLowerCase();
  if (/^(请问[，, ]*)?(如何|怎么|怎样|为什么|什么是|教我)/.test(text))
    return '';
  if (
    !/(生成|制作|创建|导出|整理成|做成|转成|转为|转换成|写|做|准备|输出|给我.{0,6}[份个])/.test(
      text,
    )
  )
    return '';
  const target =
    text.match(
      /(?:导出|转成|转为|转换成|做成|输出|整理成)[为成\s]*(pptx?|powerpoint|word|docx|excel|xlsx|幻灯片|演示文稿|表格|报告|文档|方案|材料|手册)/,
    )?.[1] || text;
  if (/\b(pptx?|powerpoint)\b|幻灯片|演示文稿/.test(target)) return 'pptx';
  if (/\b(word|docx)\b/.test(target)) return 'docx';
  if (/\b(excel|xlsx)\b/.test(target)) return 'xlsx';
  if (/表格/.test(target)) return 'xlsx';
  if (/报告|文档|方案|材料|手册/.test(target)) return 'docx';
  return '';
}

export function assistantTemplateIntent(message) {
  const text = message.trim();
  if (/^(请问[，, ]*)?(如何|怎么|怎样|为什么|什么是|教我)/.test(text))
    return false;
  return /(生成|制作|创建|套用|做一|做个|做份)/.test(text);
}
