'use client';

import { useEffect, useRef, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  assistantRequest,
  type AssistantOutlineItem,
  type AssistantRun,
  type AssistantUpload,
} from '@/request/Assistant';

import { reportAssistantIssue } from './assistantFeedback.mjs';

type DocumentOption = { id: string; name: string; type: number };

export default function AssistantPlanningCard({
  run,
  onUpdate,
}: {
  run: AssistantRun;
  onUpdate: (run: AssistantRun) => void;
}) {
  const plan = run.planning;
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [outline, setOutline] = useState<AssistantOutlineItem[]>(
    plan?.outline || [],
  );
  const [feedback, setFeedback] = useState('');
  const [documents, setDocuments] = useState<DocumentOption[]>([]);
  const [options, setOptions] = useState<DocumentOption[]>([]);
  const [query, setQuery] = useState('');
  const [uploads, setUploads] = useState<AssistantUpload[]>([]);
  const [busy, setBusy] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  const pendingRequest = useRef<
    { fingerprint: string; id: string } | undefined
  >(undefined);
  useEffect(() => {
    setAnswers({});
    setOutline(plan?.outline || []);
    setFeedback('');
    setDocuments([]);
    setUploads([]);
    setError('');
    pendingRequest.current = undefined;
  }, [plan?.revision, run.id]);
  useEffect(() => {
    if (run.state !== 'awaiting_sources') return;
    const ac = new AbortController();
    const timer = setTimeout(() => {
      assistantRequest<DocumentOption[]>(
        `/documents?q=${encodeURIComponent(query)}`,
        'GET',
        undefined,
        ac.signal,
      )
        .then(items => setOptions(items.filter(item => item.type === 2)))
        .catch(e => {
          if (!ac.signal.aborted)
            setError(reportAssistantIssue('documents', e));
        });
    }, 300);
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [query, run.state]);
  if (!plan) return null;

  const proceed = async (action: string, useDefaults = false) => {
    setBusy(true);
    setError('');
    const body = {
      revision: plan.revision,
      action,
      answers: action === 'answer' ? answers : {},
      use_defaults: useDefaults,
      message: feedback,
      outline: action === 'confirm_outline' ? outline : undefined,
      attachment_ids:
        action === 'supplement' ? uploads.map(item => item.id) : [],
      node_ids: action === 'supplement' ? documents.map(item => item.id) : [],
    };
    const fingerprint = JSON.stringify(body);
    if (pendingRequest.current?.fingerprint !== fingerprint)
      pendingRequest.current = { fingerprint, id: uuidv4() };
    try {
      const current = await assistantRequest<AssistantRun>(
        `/runs/${run.id}/continue`,
        'POST',
        { ...body, client_request_id: pendingRequest.current.id },
      );
      pendingRequest.current = undefined;
      onUpdate(current);
    } catch (e) {
      setError(reportAssistantIssue('continue', e));
    } finally {
      setBusy(false);
    }
  };
  const uploadFiles = async (files: FileList | null) => {
    if (!files?.length) return;
    if (uploads.length + files.length > 10) {
      setError('每轮最多补充10份附件');
      return;
    }
    setUploading(true);
    setError('');
    try {
      for (const file of Array.from(files)) {
        const body = new FormData();
        body.append('file', file);
        const result = await assistantRequest<AssistantUpload>(
          '/uploads',
          'POST',
          body,
        );
        setUploads(previous => [...previous, result]);
      }
    } catch (e) {
      setError(reportAssistantIssue('upload', e));
    } finally {
      setUploading(false);
    }
  };
  const disabled = busy || uploading;
  return (
    <Stack gap={1.5} sx={{ mt: 1.5 }}>
      <Typography fontWeight={600}>{plan.title}</Typography>
      <Typography variant='body2' color='text.secondary'>
        {plan.summary}
      </Typography>
      {plan.framework_only && (
        <Alert severity='info'>
          当前仅生成框架草稿；缺失事实会明确留空，不作为正式产品材料。
        </Alert>
      )}
      {run.state === 'awaiting_sources' &&
        !plan.sufficient &&
        !plan.framework_only && (
          <Alert severity='info'>
            现有资料不足以生成正式内容，请先补齐以下信息：
            <Box component='ul' sx={{ my: 0.5, pl: 2 }}>
              {plan.gaps?.map((gap, i) => (
                <li key={i}>{gap}</li>
              ))}
            </Box>
          </Alert>
        )}
      {run.state === 'awaiting_input' && (
        <>
          <Typography variant='body2'>
            确认关键点就可以继续，其他设置我会帮你安排。
          </Typography>
          {plan.questions.map(question => (
            <Autocomplete
              key={question.id}
              freeSolo
              options={question.options || []}
              inputValue={answers[question.id] ?? question.default ?? ''}
              onInputChange={(_, value) =>
                setAnswers(previous => ({ ...previous, [question.id]: value }))
              }
              disabled={disabled}
              renderInput={params => (
                <TextField
                  {...params}
                  label={question.question}
                  size='small'
                  helperText={`推荐：${question.default || '采用合理默认设置'}`}
                />
              )}
            />
          ))}
          <TextField
            label='其他要求（选填）'
            size='small'
            multiline
            minRows={2}
            value={feedback}
            onChange={event => setFeedback(event.target.value)}
            disabled={disabled}
          />
          <Stack direction='row' gap={1} flexWrap='wrap'>
            <Button
              variant='contained'
              disabled={disabled}
              onClick={() => proceed('answer', true)}
            >
              按这个要求开始制作
            </Button>
          </Stack>
        </>
      )}
      {run.state === 'awaiting_sources' && (
        <>
          <Autocomplete
            multiple
            options={options}
            value={documents}
            getOptionLabel={item => item.name}
            isOptionEqualToValue={(a, b) => a.id === b.id}
            onInputChange={(_, value) => setQuery(value)}
            onChange={(_, value) => setDocuments(value)}
            filterOptions={items => items}
            disabled={disabled}
            renderInput={params => (
              <TextField
                {...params}
                label='选择可访问的已发布文档'
                size='small'
                helperText='选择后使用这些文档及本轮附件直接取材，不依赖向量命中。'
              />
            )}
          />
          <Stack direction='row' gap={1} flexWrap='wrap'>
            <Button component='label' variant='outlined' disabled={disabled}>
              {uploading ? '正在上传…' : '上传补充资料'}
              <input
                hidden
                type='file'
                multiple
                accept='.pdf,.docx,.pptx,.xlsx,.txt,.csv'
                onChange={event => {
                  void uploadFiles(event.target.files);
                  event.target.value = '';
                }}
              />
            </Button>
            {uploads.map(item => (
              <Chip
                key={item.id}
                label={item.name}
                onDelete={
                  disabled
                    ? undefined
                    : () =>
                        setUploads(previous =>
                          previous.filter(upload => upload.id !== item.id),
                        )
                }
              />
            ))}
          </Stack>
          <TextField
            label='补充事实或说明（选填）'
            helperText='可以补充产品信息、数据出处等；用户提供的事实会标注为用户补充说明。'
            multiline
            minRows={3}
            value={feedback}
            onChange={event => setFeedback(event.target.value)}
            disabled={disabled}
          />
          <Stack direction='row' gap={1} flexWrap='wrap'>
            <Button
              variant='contained'
              disabled={
                disabled ||
                (!documents.length && !uploads.length && !feedback.trim())
              }
              onClick={() => proceed('supplement')}
            >
              补充后重新检查资料
            </Button>
            <Button
              variant='outlined'
              disabled={disabled}
              onClick={() => proceed('framework')}
            >
              仅生成框架草稿
            </Button>
          </Stack>
        </>
      )}
      {run.state === 'awaiting_outline' && (
        <>
          <Typography variant='body2'>
            这是你要求先看的大纲。可以直接开始制作，也可以调整标题和重点。
          </Typography>
          {Object.entries(plan.brief || {}).length > 0 && (
            <Typography variant='caption' color='text.secondary'>
              {Object.entries(plan.brief)
                .map(
                  ([key, value]) =>
                    `${({ audience: '受众', goal: '目标', length: '规模', style: '风格' } as Record<string, string>)[key] || key}：${value}`,
                )
                .join(' · ')}
            </Typography>
          )}
          {outline.map((item, index) => (
            <Stack
              key={item.id}
              gap={1}
              sx={{
                p: 1.5,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1.5,
              }}
            >
              <TextField
                label={`${plan.format === 'pptx' ? '第' + (index + 1) + '页' : '第' + (index + 1) + '部分'}标题`}
                size='small'
                value={item.title}
                disabled={disabled}
                inputProps={{ maxLength: 50 }}
                onChange={event =>
                  setOutline(previous =>
                    previous.map(part =>
                      part.id === item.id
                        ? { ...part, title: event.target.value }
                        : part,
                    ),
                  )
                }
              />
              <TextField
                label='核心观点 / 讲述重点'
                size='small'
                multiline
                minRows={2}
                value={item.purpose}
                disabled={disabled}
                inputProps={{ maxLength: 500 }}
                onChange={event =>
                  setOutline(previous =>
                    previous.map(part =>
                      part.id === item.id
                        ? { ...part, purpose: event.target.value }
                        : part,
                    ),
                  )
                }
              />
              <Typography variant='caption' color='text.secondary'>
                依据：
                {item.source_ids
                  ?.map(
                    id =>
                      plan.materials.find(source => source.id === id)?.name ||
                      '已授权资料',
                  )
                  .join('、') ||
                  (plan.framework_only
                    ? '框架占位，不填业务事实'
                    : '已有文件或用户需求')}
              </Typography>
            </Stack>
          ))}
          <TextField
            label='调整意见（需要重新规划时填写）'
            size='small'
            multiline
            minRows={2}
            value={feedback}
            onChange={event => setFeedback(event.target.value)}
            disabled={disabled}
          />
          <Stack direction='row' gap={1} flexWrap='wrap'>
            <Button
              variant='contained'
              disabled={disabled || !outline.length}
              onClick={() => proceed('confirm_outline')}
            >
              确认大纲，生成文件
            </Button>
            <Button
              variant='outlined'
              disabled={disabled || !feedback.trim()}
              onClick={() => proceed('revise')}
            >
              按意见调整大纲
            </Button>
          </Stack>
        </>
      )}
      {!!plan.materials?.length && (
        <Box component='details' sx={{ color: 'text.secondary', fontSize: 12 }}>
          <summary>查看资料覆盖情况（{plan.materials.length} 项）</summary>
          <Stack gap={0.5} mt={1}>
            {plan.materials.map(item => (
              <Typography key={item.id} variant='caption'>
                {item.name} · {item.status}
                {item.total_characters
                  ? ` · 本轮已读 ${item.characters} / ${item.total_characters} 字符（可用正文）`
                  : item.characters > 0
                    ? ` · 正文 ${item.characters} 字符`
                    : ''}
              </Typography>
            ))}
          </Stack>
        </Box>
      )}
      {disabled && (
        <Stack direction='row' gap={1} alignItems='center'>
          <CircularProgress size={16} />
          <Typography variant='caption'>
            {uploading ? '资料正在上传…' : '正在保存并继续处理…'}
          </Typography>
        </Stack>
      )}
      {error && (
        <Typography
          role='status'
          aria-live='polite'
          variant='body2'
          color='text.secondary'
        >
          {error}
        </Typography>
      )}
    </Stack>
  );
}
