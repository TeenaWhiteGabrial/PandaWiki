'use client';
import SecureFilePreview from '@panda-wiki/file-preview';
import { useEffect, useRef, useState } from 'react';
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import AttachFileIcon from '@mui/icons-material/AttachFile';
import PublicIcon from '@mui/icons-material/Public';
import SlideshowIcon from '@mui/icons-material/Slideshow';
import {
  assistantRequest,
  AssistantOptions,
  AssistantTemplate,
  AssistantUpload,
} from '@/request/Assistant';
import { useBasePath } from '@/hooks';
import { usePathname } from 'next/navigation';
import { reportAssistantIssue } from './assistantFeedback.mjs';

export const emptyAssistantOptions = (): AssistantOptions => ({
  attachments: [],
  web: false,
  publicQuery: '',
  scope: { type: 'all' },
});
export default function AssistantTools({
  value,
  onChange,
  disabled,
  conversationId,
  onUploadingChange,
}: {
  value: AssistantOptions;
  onChange: (v: AssistantOptions) => void;
  disabled: boolean;
  conversationId: string;
  onUploadingChange: (busy: boolean) => void;
}) {
  const base = useBasePath();
  const pathname = usePathname();
  const currentNode = pathname.match(/\/node\/([^/]+)/)?.[1];
  const [documentQuery, setDocumentQuery] = useState('');
  const [documentOptions, setDocumentOptions] = useState<
    { id: string; name: string; type: number }[]
  >([]);
  const input = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [templates, setTemplates] = useState<AssistantTemplate[]>([]);
  const [searchAvailable, setSearchAvailable] = useState(false);
  const [error, setError] = useState('');
  const [templateDialog, setTemplateDialog] = useState<AssistantUpload | null>(
    null,
  );
  const preferenceGeneration = useRef(0);
  const valueRef = useRef(value);
  valueRef.current = value;
  useEffect(() => {
    const ac = new AbortController();
    Promise.all([
      assistantRequest<{ search_available: boolean }>(
        '/capabilities',
        'GET',
        undefined,
        ac.signal,
      ),
      assistantRequest<AssistantTemplate[]>(
        '/templates',
        'GET',
        undefined,
        ac.signal,
      ),
    ])
      .then(([c, t]) => {
        setSearchAvailable(c.search_available);
        setTemplates(t);
      })
      .catch(e => {
        if (!ac.signal.aborted) setError(reportAssistantIssue('settings', e));
      });
    return () => ac.abort();
  }, []);
  useEffect(() => {
    if (!conversationId) return;
    const generation = preferenceGeneration.current;
    const ac = new AbortController();
    assistantRequest<{
      web_search_enabled?: boolean;
      template_ref?: AssistantOptions['template'];
    }>(`/preferences/${conversationId}`, 'GET', undefined, ac.signal)
      .then(s => {
        if (generation !== preferenceGeneration.current) return;
        onChange({
          ...valueRef.current,
          web: !!s.web_search_enabled,
          template: s.template_ref,
        });
      })
      .catch(() => {});
    return () => ac.abort();
  }, [conversationId]);
  const change = (next: AssistantOptions) => {
    preferenceGeneration.current++;
    onChange(next);
    if (conversationId)
      void assistantRequest(`/preferences/${conversationId}`, 'PATCH', {
        web_search_enabled: next.web,
        template_ref: next.template,
      }).catch(e => setError(reportAssistantIssue('settings', e)));
  };
  useEffect(() => {
    const pending = value.attachments.filter(
      a => a.status === 'queued' || a.status === 'processing',
    );
    if (!pending.length) return;
    const ac = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      if (document.hidden) {
        timer = setTimeout(poll, 2000);
        return;
      }
      try {
        const latest = await Promise.all(
          pending.map(a =>
            assistantRequest<AssistantUpload>(
              `/uploads/${a.id}`,
              'GET',
              undefined,
              ac.signal,
            ),
          ),
        );
        if (!ac.signal.aborted)
          onChange({
            ...valueRef.current,
            attachments: valueRef.current.attachments.map(
              a => latest.find(v => v.id === a.id) || a,
            ),
          });
      } catch (e) {
        if (!ac.signal.aborted) setError(reportAssistantIssue('sync', e));
      }
      if (!ac.signal.aborted) timer = setTimeout(poll, 2000);
    };
    timer = setTimeout(poll, 1000);
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [value.attachments.map(a => `${a.id}:${a.status}`).join(',')]);
  const upload = async (files: FileList | null) => {
    if (!files) return;
    setError('');
    if (value.attachments.length + files.length > 10) {
      setError('每轮最多上传10份文件');
      return;
    }
    if (
      [...files].some(f => f.size > 20 * 1024 * 1024) ||
      value.attachments.reduce((n, f) => n + f.size, 0) +
        [...files].reduce((n, f) => n + f.size, 0) >
        100 * 1024 * 1024
    ) {
      setError('单文件最多20 MB，每轮总计100 MB');
      return;
    }
    setUploading(true);
    onUploadingChange(true);
    try {
      for (const file of files) {
        const form = new FormData();
        form.append('file', file);
        const a = await assistantRequest<AssistantUpload>(
          '/uploads',
          'POST',
          form,
        );
        const current = valueRef.current;
        const next = { ...current, attachments: [...current.attachments, a] };
        valueRef.current = next;
        onChange(next);
      }
    } catch (e) {
      setError(reportAssistantIssue('upload', e));
    } finally {
      setUploading(false);
      onUploadingChange(false);
      if (input.current) input.current.value = '';
    }
  };
  useEffect(() => {
    if (value.scope.type !== 'documents') return;
    const ac = new AbortController();
    const t = setTimeout(() => {
      assistantRequest<{ id: string; name: string; type: number }[]>(
        `/documents?q=${encodeURIComponent(documentQuery)}`,
        'GET',
        undefined,
        ac.signal,
      )
        .then(setDocumentOptions)
        .catch(() => {});
    }, 300);
    return () => {
      ac.abort();
      clearTimeout(t);
    };
  }, [documentQuery, value.scope.type]);
  const selected =
    value.template?.kind === 'enterprise'
      ? `${value.template.id}:${value.template.version}`
      : value.template?.kind === 'upload'
        ? `upload:${value.template.id}`
        : '';
  return (
    <Stack gap={1} sx={{ width: '100%' }}>
      {error && (
        <Typography
          role='status'
          aria-live='polite'
          variant='caption'
          color='text.secondary'
        >
          {error}
        </Typography>
      )}
      <Stack direction='row' gap={1} flexWrap='wrap' alignItems='center'>
        <input
          ref={input}
          type='file'
          multiple
          hidden
          accept='.pdf,.docx,.pptx,.xlsx,.txt,.csv,.png,.jpg,.jpeg'
          onChange={e => void upload(e.target.files)}
        />
        <Button
          size='small'
          startIcon={
            uploading ? <CircularProgress size={14} /> : <AttachFileIcon />
          }
          disabled={disabled || uploading}
          onClick={() => input.current?.click()}
        >
          {uploading ? '上传中…' : '上传文件'}
        </Button>
        <Tooltip
          title={
            searchAvailable
              ? '仅发送你确认可公开的检索词，不自动外发内部资料'
              : '尚未配置搜索服务'
          }
        >
          <span>
            <Button
              size='small'
              startIcon={<PublicIcon />}
              disabled={disabled || !searchAvailable}
              variant={value.web ? 'contained' : 'text'}
              onClick={() => change({ ...value, web: !value.web })}
            >
              {value.web ? '联网搜索 · 已开启' : '联网搜索'}
            </Button>
          </span>
        </Tooltip>
        <FormControl size='small' sx={{ minWidth: 155 }}>
          <InputLabel>PPT 模板</InputLabel>
          <Select
            label='PPT 模板'
            value={selected}
            disabled={disabled}
            onChange={e => {
              if (String(e.target.value).startsWith('upload:')) return;
              const t = templates.find(
                t => `${t.id}:${t.version}` === e.target.value,
              );
              change({
                ...value,
                template: t
                  ? { kind: 'enterprise', id: t.id, version: t.version }
                  : undefined,
              });
            }}
          >
            <MenuItem value=''>使用默认模板</MenuItem>
            {value.template?.kind === 'upload' && (
              <MenuItem value={`upload:${value.template.id}`}>
                个人上传模板
              </MenuItem>
            )}
            {templates.map(t => (
              <MenuItem
                key={`${t.id}:${t.version}`}
                value={`${t.id}:${t.version}`}
              >
                {t.name} · v{t.version}
                {t.is_default ? '（默认）' : ''}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 135 }}>
          <InputLabel>资料范围</InputLabel>
          <Select
            label='资料范围'
            value={value.scope.type}
            disabled={disabled}
            onChange={e =>
              change({
                ...value,
                scope:
                  e.target.value === 'documents'
                    ? { type: 'documents', node_ids: [] }
                    : { type: 'all' },
              })
            }
          >
            <MenuItem value='all'>全部授权知识</MenuItem>
            <MenuItem value='documents'>指定文档 / 目录</MenuItem>
          </Select>
        </FormControl>
      </Stack>
      {value.scope.type === 'documents' && (
        <Stack direction='row' gap={1}>
          <Autocomplete
            multiple
            fullWidth
            size='small'
            options={documentOptions}
            filterOptions={x => x}
            getOptionLabel={o => (o.type === 1 ? '目录：' : '') + o.name}
            isOptionEqualToValue={(a, b) => a.id === b.id}
            value={(value.scope.node_ids || []).map(
              id =>
                documentOptions.find(o => o.id === id) || {
                  id,
                  name: '已选资料',
                  type: 2,
                },
            )}
            onInputChange={(_, q) => setDocumentQuery(q)}
            onChange={(_, items) =>
              change({
                ...value,
                scope: { type: 'documents', node_ids: items.map(i => i.id) },
              })
            }
            renderInput={p => (
              <TextField
                {...p}
                label='搜索并选择授权文档或目录'
                helperText='不选择时只使用上传资料；目录范围会固定到本轮可访问清单。'
              />
            )}
          />
          {currentNode && (
            <Button
              size='small'
              onClick={() =>
                change({
                  ...value,
                  scope: { type: 'documents', node_ids: [currentNode] },
                })
              }
            >
              当前文档
            </Button>
          )}
        </Stack>
      )}
      {value.web && (
        <TextField
          size='small'
          label='本轮确认可公开的检索词'
          value={value.publicQuery}
          inputProps={{ maxLength: 200 }}
          onChange={e => change({ ...value, publicQuery: e.target.value })}
          helperText='只向搜索服务发送这里填写的词，不会自动附带会话或上传资料。'
        />
      )}
      {value.attachments.map(a => (
        <Stack
          key={a.id}
          direction='row'
          gap={1}
          alignItems='center'
          flexWrap='wrap'
        >
          <Chip
            size='small'
            label={`${a.name} · ${a.status === 'ready' ? '可使用' : a.status === 'failed' ? '待继续处理' : a.status === 'processing' ? '解析中' : '等待处理'}`}
            color='default'
            onDelete={
              disabled
                ? undefined
                : () =>
                    change({
                      ...value,
                      attachments: value.attachments.filter(x => x.id !== a.id),
                      template:
                        value.template?.id === a.id
                          ? undefined
                          : value.template,
                    })
            }
          />
          {(a.status === 'queued' || a.status === 'processing') && (
            <CircularProgress size={14} />
          )}
          {a.status === 'failed' && (
            <Button
              size='small'
              disabled={disabled}
              onClick={() =>
                void assistantRequest(`/uploads/${a.id}/retry`, 'POST', {})
                  .then(() =>
                    onChange({
                      ...valueRef.current,
                      attachments: valueRef.current.attachments.map(v =>
                        v.id === a.id
                          ? { ...v, status: 'queued', error: undefined }
                          : v,
                      ),
                    }),
                  )
                  .catch(e => setError(reportAssistantIssue('attachment', e)))
              }
            >
              继续处理
            </Button>
          )}
          {a.error && (
            <Typography color='text.secondary' variant='caption'>
              这份资料尚未准备好，可继续处理或换一份资料。
            </Typography>
          )}
          {a.format === 'pptx' && a.status === 'ready' && (
            <Button
              size='small'
              startIcon={<SlideshowIcon />}
              disabled={disabled || !a.report?.template_supported}
              onClick={() => setTemplateDialog(a)}
            >
              {value.template?.id === a.id ? '本次 PPT 模板' : '用作 PPT 模板'}
            </Button>
          )}
          {a.report?.template_error && (
            <Typography variant='caption' color='text.secondary'>
              这份文件可作为参考资料；使用PPT模板时，请选择包含标题和正文占位符的16:9模板。
            </Typography>
          )}
        </Stack>
      ))}
      {value.template?.kind === 'upload' && (
        <Chip
          size='small'
          color='primary'
          label={`个人模板：${value.attachments.find(a => a.id === value.template?.id)?.name || '已选模板'}`}
          onDelete={() => change({ ...value, template: undefined })}
        />
      )}
      {!!value.attachments.length && (
        <Typography variant='caption' color='text.secondary'>
          资料默认仅本人可用，不自动加入知识库；会按系统配置交给模型处理，保留期限见文件卡片。
        </Typography>
      )}
      <Dialog
        open={!!templateDialog}
        onClose={() => setTemplateDialog(null)}
        maxWidth='md'
        fullWidth
      >
        <DialogTitle>确认个人 PPT 模板</DialogTitle>
        <DialogContent>
          <Typography gutterBottom>
            {templateDialog?.report?.template_notice}
          </Typography>
          <Typography variant='body2'>
            支持版式：
            {templateDialog?.report?.layouts?.map(l => l.name).join('、')}
          </Typography>
          {templateDialog && (
            <Box sx={{ height: 420 }}>
              <SecureFilePreview
                unavailableMessage='预览暂未准备好，可稍后查看或下载文件。'
                title='模板预览'
                kind='pdf'
                assetBase={base}
                url={`${base}/share/v1/assistant/uploads/${templateDialog.id}/content?preview=true`}
              />
            </Box>
          )}
          <Typography variant='caption'>
            预览不可用时可先下载检查；此模板不会发布到企业模板库。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setTemplateDialog(null)}>取消</Button>
          <Button
            variant='contained'
            onClick={() => {
              if (templateDialog)
                change({
                  ...value,
                  template: { kind: 'upload', id: templateDialog.id },
                });
              setTemplateDialog(null);
            }}
          >
            确认使用
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
