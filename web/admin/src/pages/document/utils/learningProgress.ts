import { DomainLearningStage } from '@/request/types';
import dayjs from 'dayjs';

export const learningStageLabels: Record<string, string> = {
  waiting: '等待调度',
  mq_waiting: '等待后台消费',
  preparing: '读取发布版本',
  file_info: '读取文件信息',
  download: '下载源文件',
  parse: '本地解析正文',
  pdf_waiting: '等待 PDF 解析名额',
  pdf_text: '提取 PDF 文字',
  pdf_repair: '修复 PDF 结构',
  pdf_ocr: '逐页 OCR 识别',
  pdf_complete: 'PDF 正文提取完成',
  pdf_failed: 'PDF 解析失败',
  crawler: '解析服务提取正文',
  crawler_export: '解析服务转换正文',
  crawler_result: '等待解析结果',
  metadata: '准备名称与元信息',
  save_content: '保存解析结果',
  submit: '提交文本至 RAGLite',
  bind_doc: '保存索引文档 ID',
  rag_pending: '等待 RAGLite 索引',
  rag_running: 'RAGLite 生成索引',
};

export const stageElapsedMs = (stage: DomainLearningStage, now = Date.now()) =>
  Math.max(
    0,
    (stage.finished_at ? dayjs(stage.finished_at).valueOf() : now) -
      dayjs(stage.started_at).valueOf(),
  );

export const formatLearningDuration = (milliseconds: number) => {
  if (!Number.isFinite(milliseconds)) return '-';
  const seconds = Math.max(0, Math.floor(milliseconds / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分 ${seconds % 60} 秒`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时 ${minutes % 60} 分`;
  return `${Math.floor(hours / 24)} 天 ${hours % 24} 小时`;
};
