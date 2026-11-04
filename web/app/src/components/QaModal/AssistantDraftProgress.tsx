'use client';

import { Box, LinearProgress, Stack, Typography } from '@mui/material';
import type { AssistantRun } from '@/request/Assistant';

export default function AssistantDraftProgress({
  generation,
  active,
  resumable = true,
}: {
  generation: NonNullable<AssistantRun['generation']>;
  active: boolean;
  resumable?: boolean;
}) {
  const completed = Math.min(generation.completed, generation.total);
  return (
    <Stack gap={1.5} sx={{ my: 1.5 }}>
      <Stack direction='row' justifyContent='space-between' gap={2}>
        <Typography
          variant='body2'
          fontWeight={600}
          role='status'
          aria-live='polite'
        >
          {!active
            ? `已保存 ${completed} ${generation.unit}内容`
            : completed === generation.total
              ? '内容已整理完成，正在排版和检查文件'
              : `正在整理：${generation.current_title || '文件内容'}`}
        </Typography>
        <Typography
          variant='caption'
          color='text.secondary'
          sx={{ flexShrink: 0 }}
        >
          {completed} / {generation.total} {generation.unit}
        </Typography>
      </Stack>
      <LinearProgress
        variant='determinate'
        value={generation.total ? (completed / generation.total) * 100 : 0}
        aria-label='已完成内容'
        sx={{ height: 5, borderRadius: 3 }}
      />
      <Box component='details' open={active}>
        <Typography
          component='summary'
          variant='body2'
          sx={{ cursor: 'pointer', color: 'text.secondary' }}
        >
          查看已整理内容
        </Typography>
        <Stack gap={1} mt={1}>
          {generation.parts.map((part, index) => (
            <Box
              key={part.id}
              sx={{
                border: '1px solid',
                borderColor: part.completed ? 'divider' : 'transparent',
                bgcolor: part.completed ? 'background.paper' : 'action.hover',
                borderRadius: 1.5,
                px: 1.5,
                py: 1.25,
                opacity: part.completed ? 1 : 0.65,
              }}
            >
              <Stack direction='row' gap={1} alignItems='baseline'>
                <Typography variant='caption' color='text.secondary'>
                  {String(part.position || index + 1).padStart(2, '0')}
                </Typography>
                <Typography
                  variant='body2'
                  fontWeight={part.completed ? 600 : 400}
                >
                  {part.title}
                </Typography>
                <Typography
                  variant='caption'
                  color='text.secondary'
                  sx={{ ml: 'auto', flexShrink: 0 }}
                >
                  {part.completed
                    ? '已整理'
                    : index === completed && active
                      ? '正在整理'
                      : '待整理'}
                </Typography>
              </Stack>
              {part.preview && (
                <Typography
                  variant='body2'
                  color='text.secondary'
                  sx={{ mt: 0.75, lineHeight: 1.7 }}
                >
                  {part.preview}
                </Typography>
              )}
            </Box>
          ))}
        </Stack>
      </Box>
      {!active && resumable && completed < generation.total && (
        <Typography variant='caption' color='text.secondary'>
          已完成的内容已保存，继续处理时会接着整理。
        </Typography>
      )}
    </Stack>
  );
}
