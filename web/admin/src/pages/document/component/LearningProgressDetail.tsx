import { Modal } from '@/components/ui';
import { DomainNodeListItemResp } from '@/request/types';
import { Box, Chip, Stack, Typography } from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useState } from 'react';
import {
  formatLearningDuration,
  learningStageLabels,
  stageElapsedMs,
} from '../utils/learningProgress';

const timestamp = (value?: string) =>
  value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '尚无记录';

export default function LearningProgressDetail({
  doc,
  onClose,
}: {
  doc?: DomainNodeListItemResp;
  onClose: () => void;
}) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!doc) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [doc?.id]);
  const info = doc?.rag_info;
  const progress = info?.learning_progress;
  const extraction = progress?.extraction;
  const stages = progress?.stages || [];
  const current = stages.at(-1);
  const longest = stages.reduce<typeof current>(
    (result, stage) =>
      !result || stageElapsedMs(stage, now) > stageElapsedMs(result, now)
        ? stage
        : result,
    undefined,
  );
  const message = info?.task_message || info?.message;
  return (
    <Modal
      open={!!doc}
      title={`索引进度 · ${doc?.name || '未命名'}`}
      width={760}
      footer={null}
      onCancel={onClose}
    >
      <Stack gap={2} sx={{ maxHeight: '70vh', overflowY: 'auto' }}>
        {current ? (
          <Box sx={{ p: 2, bgcolor: 'action.hover', borderRadius: 1 }}>
            <Typography fontWeight={600}>
              {progress?.finished_at ? '结束前所在步骤' : '当前步骤'}：
              {learningStageLabels[current.key] || current.key}
              {' · '}
              {formatLearningDuration(stageElapsedMs(current, now))}
            </Typography>
            <Typography
              fontSize={13}
              sx={{ mt: 0.5, overflowWrap: 'anywhere' }}
            >
              {current.message}
            </Typography>
            {longest && (
              <Typography fontSize={12} color='text.secondary' sx={{ mt: 1 }}>
                耗时最长：{learningStageLabels[longest.key] || longest.key}（
                {formatLearningDuration(stageElapsedMs(longest, now))}）
              </Typography>
            )}
          </Box>
        ) : (
          <Typography color='text.secondary' fontSize={13}>
            此任务暂无分步骤记录。历史步骤无法补录，重新提交后会记录完整时间线。
          </Typography>
        )}
        {extraction && (
          <Box
            sx={{
              p: 2,
              bgcolor: 'action.hover',
              border: '1px solid',
              borderColor:
                extraction.scope === 'full' ? 'divider' : 'warning.main',
              borderRadius: 1,
            }}
          >
            <Typography fontWeight={600}>
              索引内容：
              {{
                full: '完整正文',
                partial: '部分正文（请核对）',
                metadata: '仅名称和文件元信息',
                unknown: '已提取正文，覆盖范围待核验',
              }[extraction.scope] || '待核验'}
            </Typography>
            {extraction.parser && (
              <Typography fontSize={13}>
                解析方式：{extraction.parser}
                {extraction.repaired ? ' · 已尝试结构修复' : ''}
              </Typography>
            )}
            {!!extraction.page_count && (
              <Typography fontSize={13}>
                正文覆盖 {extraction.extracted_pages || 0} /{' '}
                {extraction.page_count} 页 · OCR 识别{' '}
                {extraction.ocr_pages || 0} 页
              </Typography>
            )}
            <Typography fontSize={13} sx={{ overflowWrap: 'anywhere' }}>
              {extraction.message}
            </Typography>
            {extraction.scope === 'metadata' && (
              <Typography fontSize={13} color='warning.main'>
                正文未参与问答，可按文件名称、路径和类型检索。
              </Typography>
            )}
            {extraction.warnings?.map((warning, index) => (
              <Typography
                key={index}
                fontSize={12}
                color='text.secondary'
                sx={{ overflowWrap: 'anywhere' }}
              >
                {warning}
              </Typography>
            ))}
          </Box>
        )}
        {message && (
          <Typography fontSize={13} sx={{ overflowWrap: 'anywhere' }}>
            {message}
          </Typography>
        )}
        <Stack gap={0.5}>
          {stages.length > 0 && progress && (
            <Typography fontSize={12}>
              已记录总耗时：
              {formatLearningDuration(
                Math.max(
                  0,
                  (progress.finished_at
                    ? dayjs(progress.finished_at).valueOf()
                    : now) - dayjs(progress.started_at).valueOf(),
                ),
              )}
            </Typography>
          )}
          <Typography fontSize={12}>
            开始记录：{timestamp(stages[0]?.started_at)} · 结束：
            {timestamp(progress?.finished_at)}
          </Typography>
          <Typography fontSize={12}>
            最后进展变化：
            {timestamp(stages.length ? progress?.last_progress_at : undefined)}
          </Typography>
          <Typography fontSize={12}>
            最近状态检查：{timestamp(progress?.last_checked_at)}
            {progress?.source
              ? `（${({ poll: '定时查询', event: '后台状态通知', upload: '上传接口' } as Record<string, string>)[progress.source] || progress.source}）`
              : ''}
          </Typography>
          {progress?.rag_status && (
            <Typography fontSize={12}>
              RAGLite 返回状态：{progress.rag_status}
            </Typography>
          )}
          {progress?.check_error && (
            <Typography
              color='error.main'
              fontSize={13}
              sx={{ overflowWrap: 'anywhere' }}
            >
              {progress.check_error}
            </Typography>
          )}
        </Stack>
        {stages.map((stage, index) => (
          <Stack
            key={`${stage.started_at}-${index}`}
            direction='row'
            gap={1.5}
            sx={{
              borderLeft: '2px solid',
              borderColor: stage.finished_at ? 'divider' : 'primary.main',
              pl: 1.5,
            }}
          >
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography fontSize={13} fontWeight={600}>
                {index + 1}. {learningStageLabels[stage.key] || stage.key}
              </Typography>
              <Typography fontSize={12} color='text.secondary'>
                {timestamp(stage.started_at)} →{' '}
                {stage.finished_at ? timestamp(stage.finished_at) : '进行中'}
              </Typography>
              <Typography fontSize={12} sx={{ overflowWrap: 'anywhere' }}>
                {stage.message}
              </Typography>
            </Box>
            <Chip
              size='small'
              variant='outlined'
              label={formatLearningDuration(stageElapsedMs(stage, now))}
            />
          </Stack>
        ))}
        <Typography fontSize={12} color='text.secondary'>
          耗时是步骤状态已持续的时间，最后进展长期不变时需要检查后台任务，不能据此判断仍在执行。状态检查成功不代表索引有新进展。RAGLite
          内部的切分、模型请求、向量写入目前共用一个步骤，具体进展以其返回消息为准；mp4
          等文件跳过下载与正文解析，只提交名称等元信息。
        </Typography>
        <Box
          sx={{
            color: 'text.secondary',
            fontSize: 12,
            overflowWrap: 'anywhere',
          }}
        >
          任务 ID：{info?.task_id || '-'}
          <br />
          索引文档 ID：{progress?.doc_id || '-'}
          <br />
          数据集 ID：{progress?.dataset_id || '-'}
        </Box>
      </Stack>
    </Modal>
  );
}
