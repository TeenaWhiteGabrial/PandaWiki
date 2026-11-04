'use client';
import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/material';
import {
  assistantRequest,
  downloadAssistantContent,
  AssistantArtifact,
  AssistantRun,
} from '@/request/Assistant';
import MarkDown2 from '@/components/markdown2';
import AssistantPlanningCard from './AssistantPlanningCard';
import AssistantDraftProgress from './AssistantDraftProgress';
import AssistantArtifactPreview from './AssistantArtifactPreview';
import {
  assistantTaskView,
  reportAssistantIssue,
} from './assistantFeedback.mjs';

export default function AssistantRunCard({
  initial,
  onRevise,
  onUpdate,
  onPreview,
}: {
  initial: AssistantRun;
  onRevise: (id: string) => void;
  onUpdate?: (r: AssistantRun) => void;
  onPreview?: (file: AssistantArtifact) => void;
}) {
  const [run, setRun] = useState(initial);
  const [file, setFile] = useState<AssistantArtifact | null>(
    initial.artifact || null,
  );
  const [preview, setPreview] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [now, setNow] = useState(Date.now());
  useEffect(() => setRun(initial), [initial]);
  useEffect(() => {
    if (run.state !== 'running' && run.state !== 'queued') return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [run.state]);
  useEffect(() => {
    const ac = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      if (document.hidden) {
        timer = setTimeout(poll, 3000);
        return;
      }
      try {
        const current = await assistantRequest<AssistantRun>(
          `/runs/${initial.id}`,
          'GET',
          undefined,
          ac.signal,
        );
        if (ac.signal.aborted) return;
        setRun(current);
        onUpdate?.(current);
        setError('');
        if (current.state === 'queued' || current.state === 'running')
          timer = setTimeout(poll, 2000);
      } catch (e) {
        if (!ac.signal.aborted) {
          setError(reportAssistantIssue('sync', e));
          timer = setTimeout(poll, 10000);
        }
      }
    };
    void poll();
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [initial.id, run.state]);
  useEffect(() => {
    if (!run.artifact_id) {
      setFile(null);
      return;
    }
    if (run.artifact) {
      setFile(run.artifact);
      return;
    }
    const ac = new AbortController();
    assistantRequest<AssistantArtifact>(
      `/artifacts/${run.artifact_id}`,
      'GET',
      undefined,
      ac.signal,
    )
      .then(file => {
        setFile(file);
        onUpdate?.({ ...run, artifact: file });
      })
      .catch(e => {
        if (!ac.signal.aborted) setError(reportAssistantIssue('preview', e));
      });
    return () => ac.abort();
  }, [run.artifact_id]);
  const action = async (type: 'cancel' | 'retry') => {
    setBusy(true);
    try {
      await assistantRequest(`/runs/${run.id}/${type}`, 'POST', {});
      const current = await assistantRequest<AssistantRun>(`/runs/${run.id}`);
      setRun(current);
      onUpdate?.(current);
      setError('');
    } catch (e) {
      setError(reportAssistantIssue('continue', e));
    } finally {
      setBusy(false);
    }
  };
  const download = async (
    kind: 'uploads' | 'artifacts',
    id: string,
    name: string,
  ) => {
    setBusy(true);
    try {
      await downloadAssistantContent(`/${kind}/${id}/content`, name);
      setError('');
    } catch (e) {
      setError(reportAssistantIssue('download', e));
    } finally {
      setBusy(false);
    }
  };
  const view = assistantTaskView(run);
  const active = run.state === 'queued' || run.state === 'running';
  const waiting =
    run.state === 'awaiting_input' ||
    run.state === 'awaiting_sources' ||
    run.state === 'awaiting_outline';
  const seconds = Math.max(
    0,
    Math.floor((now - new Date(run.stage_started_at).getTime()) / 1000),
  );
  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2,
        my: 1,
      }}
    >
      <Stack direction='row' alignItems='center' gap={1} flexWrap='wrap'>
        <Typography fontWeight={600}>
          {file?.name ||
            run.planning?.title ||
            (run.state === 'completed' ? '答复' : '正在准备你的内容')}
        </Typography>
        <Chip size='small' label={view.label} />
        {active && (
          <Typography variant='caption' color='text.secondary'>
            {seconds >= 60
              ? `${Math.floor(seconds / 60)}分${seconds % 60}秒`
              : `${seconds}秒`}
          </Typography>
        )}
      </Stack>
      {active && !run.generation && <LinearProgress sx={{ my: 1 }} />}
      {run.generation && run.state !== 'completed' && (
        <AssistantDraftProgress
          generation={run.generation}
          active={active}
          resumable={run.state === 'failed'}
        />
      )}
      {waiting && (
        <AssistantPlanningCard
          run={run}
          onUpdate={current => {
            setRun(current);
            onUpdate?.(current);
          }}
        />
      )}
      {!!run.uploads?.length && (
        <Stack direction='row' flexWrap='wrap' gap={1} my={1}>
          {run.uploads.map(a => (
            <Button
              key={a.id}
              size='small'
              disabled={busy || a.status === 'unavailable'}
              onClick={() => void download('uploads', a.id, a.name)}
            >
              {a.name}
            </Button>
          ))}
        </Stack>
      )}
      {run.answer && <MarkDown2 content={run.answer} autoScroll={false} />}
      {(view.detail || error) && (
        <Typography
          role='status'
          aria-live='polite'
          variant='body2'
          color='text.secondary'
          sx={{ my: 1 }}
        >
          {error || view.detail}
        </Typography>
      )}
      {run.sources && run.sources.length > 0 && (
        <Box sx={{ fontSize: 12, color: 'text.secondary' }}>
          {run.sources.every(s => s.kind === 'catalog_node') ? (
            `目录范围：${run.sources.length} 份有权查阅的已发布文档，完整清单见 Excel。`
          ) : (
            <Box component='details'>
              <summary style={{ cursor: 'pointer' }}>
                参考了 {run.sources.length} 项资料
              </summary>
              <Box sx={{ mt: 0.5, maxHeight: 140, overflowY: 'auto' }}>
                {run.sources.map(s => s.name).join('、')}
              </Box>
            </Box>
          )}
        </Box>
      )}
      {file && (
        <Typography variant='body2' color='text.secondary' sx={{ my: 1 }}>
          版本 {file.revision} · {(file.size / 1024).toFixed(1)} KB · 保留至{' '}
          {new Date(file.expires_at).toLocaleDateString()}
          {file.preview_state === 'failed' ? ' · 可下载文件查看' : ''}
        </Typography>
      )}
      <Stack direction='row' gap={1} mt={1} flexWrap='wrap'>
        {(active || waiting || run.state === 'failed') && (
          <Button
            size='small'
            disabled={busy}
            onClick={() => void action('cancel')}
          >
            {run.state === 'failed' ? '结束任务' : '取消任务'}
          </Button>
        )}
        {run.state === 'failed' && (
          <Button
            size='small'
            disabled={busy}
            onClick={() => void action('retry')}
          >
            继续处理
          </Button>
        )}
        {file && (
          <>
            <Button
              size='small'
              variant='outlined'
              disabled={busy}
              onClick={() => void download('artifacts', file.id, file.name)}
            >
              下载文件
            </Button>
            <Button
              size='small'
              variant='contained'
              disabled={
                file.preview_state !== 'ready' && file.format !== 'xlsx'
              }
              onClick={() => {
                if (onPreview) onPreview(file);
                else setPreview(true);
              }}
            >
              预览文件
            </Button>
            <Button size='small' onClick={() => onRevise(file.id)}>
              继续修改
            </Button>
          </>
        )}
      </Stack>
      <Dialog
        open={preview}
        onClose={() => setPreview(false)}
        maxWidth='lg'
        fullWidth
      >
        <DialogTitle>
          {file?.name}
          <Button onClick={() => setPreview(false)} sx={{ float: 'right' }}>
            关闭
          </Button>
        </DialogTitle>
        <DialogContent sx={{ height: '70vh' }}>
          {file && <AssistantArtifactPreview file={file} />}
        </DialogContent>
      </Dialog>
    </Box>
  );
}
