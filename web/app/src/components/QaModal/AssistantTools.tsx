'use client';
import SecureFilePreview from '@panda-wiki/file-preview';
import { useEffect, useRef, useState } from 'react';
import {
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Menu,
  Select,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import AttachFileIcon from '@mui/icons-material/AttachFile';
import SlideshowIcon from '@mui/icons-material/Slideshow';
import {
  assistantRequest,
  assistantUploadAccept,
  AssistantOptions,
  AssistantFileFormat,
  AssistantTemplate,
  AssistantUpload,
  isAssistantUploadFile,
} from '@/request/Assistant';
import { useBasePath } from '@/hooks';
import { usePathname } from 'next/navigation';
import { useStore } from '@/provider';
import { reportAssistantIssue } from './assistantFeedback.mjs';

const formatNames = { docx: 'Word', pptx: 'PPT', xlsx: 'Excel' } as const;
type DocumentOption = { id: string; name: string; type: number };

export const emptyAssistantOptions = (): AssistantOptions => ({
  attachments: [],
  scope: { type: 'all' },
});
export default function AssistantTools({
  value,
  onChange,
  disabled,
  conversationId,
  onUploadingChange,
  outputFormat,
}: {
  value: AssistantOptions;
  onChange: (v: AssistantOptions) => void;
  disabled: boolean;
  conversationId: string;
  onUploadingChange: (busy: boolean) => void;
  outputFormat?: AssistantFileFormat;
}) {
  const base = useBasePath();
  const pathname = usePathname();
  const currentNode = pathname.match(/\/node\/([^/]+)/)?.[1];
  const { nodeList } = useStore();
  const [documentQuery, setDocumentQuery] = useState('');
  const [documentOptions, setDocumentOptions] = useState<DocumentOption[]>([]);
  const [rememberedDocuments, setRememberedDocuments] = useState<
    DocumentOption[]
  >([]);
  const [pickerError, setPickerError] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const [formatAnchor, setFormatAnchor] = useState<HTMLElement | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [templates, setTemplates] = useState<AssistantTemplate[]>([]);
  const [templateRetry, setTemplateRetry] = useState(0);
  const [templatesStatus, setTemplatesStatus] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('idle');
  const [error, setError] = useState('');
  const [templateDialog, setTemplateDialog] = useState<AssistantUpload | null>(
    null,
  );
  const preferenceGeneration = useRef(0);
  const valueRef = useRef(value);
  valueRef.current = value;
  const showPptTemplate = outputFormat === 'pptx';
  useEffect(() => {
    if (!showPptTemplate || templatesStatus === 'ready') return;
    const ac = new AbortController();
    setTemplatesStatus('loading');
    assistantRequest<AssistantTemplate[]>(
      '/templates',
      'GET',
      undefined,
      ac.signal,
    )
      .then(items => {
        if (ac.signal.aborted) return;
        setTemplates(items);
        setTemplatesStatus('ready');
      })
      .catch(() => {
        if (!ac.signal.aborted) setTemplatesStatus('error');
      });
    return () => ac.abort();
  }, [showPptTemplate, templateRetry]);
  useEffect(() => {
    if (!conversationId) return;
    const generation = preferenceGeneration.current;
    const ac = new AbortController();
    assistantRequest<{
      template_ref?: AssistantOptions['template'];
    }>(`/preferences/${conversationId}`, 'GET', undefined, ac.signal)
      .then(s => {
        if (ac.signal.aborted || generation !== preferenceGeneration.current)
          return;
        onChange({
          ...valueRef.current,
          template: s.template_ref,
        });
      })
      .catch(() => {});
    return () => ac.abort();
  }, [conversationId]);
  const change = (next: AssistantOptions) => {
    const templateChanged =
      JSON.stringify(next.template) !==
      JSON.stringify(valueRef.current.template);
    valueRef.current = next;
    onChange(next);
    if (!templateChanged) return;
    preferenceGeneration.current++;
    if (conversationId)
      void assistantRequest(`/preferences/${conversationId}`, 'PATCH', {
        template_ref: next.template ?? null,
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
    if (!files?.length) return;
    const selected = Array.from(files);
    setError('');
    if (!selected.every(isAssistantUploadFile)) {
      setError('仅支持上传 Excel（.xlsx）和 PPT（.pptx）文件');
      return;
    }
    if (value.attachments.length + selected.length > 10) {
      setError('每轮最多上传10份文件');
      return;
    }
    if (
      selected.some(f => f.size > 20 * 1024 * 1024) ||
      value.attachments.reduce((n, f) => n + f.size, 0) +
        selected.reduce((n, f) => n + f.size, 0) >
        100 * 1024 * 1024
    ) {
      setError('单文件最多20 MB，每轮总计100 MB');
      return;
    }
    setUploading(true);
    onUploadingChange(true);
    try {
      for (const file of selected) {
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
    }
  };
  useEffect(() => {
    if (!moreOpen || value.scope.type !== 'documents') return;
    const ac = new AbortController();
    const t = setTimeout(() => {
      assistantRequest<{ id: string; name: string; type: number }[]>(
        `/documents?q=${encodeURIComponent(documentQuery)}`,
        'GET',
        undefined,
        ac.signal,
      )
        .then(items => {
          if (!ac.signal.aborted) {
            setDocumentOptions(items);
            setPickerError(false);
          }
        })
        .catch(() => {
          if (!ac.signal.aborted) setPickerError(true);
        });
    }, 300);
    return () => {
      ac.abort();
      clearTimeout(t);
    };
  }, [documentQuery, value.scope.type, moreOpen]);
  const selectedTemplate =
    value.template?.kind === 'enterprise'
      ? `${value.template.id}:${value.template.version}`
      : value.template?.kind === 'upload'
        ? `upload:${value.template.id}`
        : '';
  const selectedDocuments = (value.scope.node_ids || []).map(
    id =>
      documentOptions.find(doc => doc.id === id) ||
      rememberedDocuments.find(doc => doc.id === id) || {
        id,
        name: nodeList?.find(doc => doc.id === id)?.name || '已选资料',
        type: 2,
      },
  );
  const personalTemplates = value.attachments.filter(
    file =>
      file.format === 'pptx' &&
      file.status === 'ready' &&
      file.report?.template_supported,
  );
  const templateAttachment = value.attachments.find(
    file => value.template?.kind === 'upload' && value.template.id === file.id,
  );
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
          accept={assistantUploadAccept}
          onChange={event => {
            void upload(event.currentTarget.files);
            event.currentTarget.value = '';
          }}
        />
        <Button
          size='small'
          startIcon={
            uploading ? <CircularProgress size={14} /> : <AttachFileIcon />
          }
          disabled={disabled || uploading}
          onClick={() => input.current?.click()}
        >
          {uploading ? '上传中…' : '上传资料'}
        </Button>
        <Button
          size='small'
          startIcon={<SlideshowIcon />}
          variant={outputFormat ? 'contained' : 'text'}
          aria-haspopup='menu'
          aria-expanded={formatAnchor ? true : undefined}
          disabled={disabled}
          onClick={event => setFormatAnchor(event.currentTarget)}
        >
          {outputFormat ? `生成 ${formatNames[outputFormat]}` : '生成文件'}
        </Button>
        <Menu
          anchorEl={formatAnchor}
          open={!!formatAnchor}
          onClose={() => setFormatAnchor(null)}
        >
          <MenuItem
            selected={!value.format}
            onClick={() => {
              change({ ...value, format: undefined });
              setFormatAnchor(null);
            }}
          >
            根据问题自动判断
          </MenuItem>
          {(['docx', 'pptx', 'xlsx'] as const).map(format => (
            <MenuItem
              key={format}
              selected={value.format === format}
              onClick={() => {
                change({ ...value, format });
                setFormatAnchor(null);
              }}
            >
              生成 {formatNames[format]}
            </MenuItem>
          ))}
        </Menu>
        <Button
          size='small'
          aria-expanded={moreOpen}
          disabled={disabled}
          onClick={() => setMoreOpen(v => !v)}
        >
          更多
        </Button>
      </Stack>
      {showPptTemplate && (
        <Stack direction='row' gap={1} alignItems='center' flexWrap='wrap'>
          <FormControl size='small' sx={{ minWidth: 175, maxWidth: '100%' }}>
            <InputLabel>PPT 模板</InputLabel>
            <Select
              label='PPT 模板'
              value={selectedTemplate}
              disabled={disabled}
              onChange={event => {
                const id = String(event.target.value);
                if (id.startsWith('upload:')) {
                  const file = personalTemplates.find(
                    item => `upload:${item.id}` === id,
                  );
                  if (file) setTemplateDialog(file);
                  return;
                }
                const selected = templates.find(
                  item => `${item.id}:${item.version}` === id,
                );
                change({
                  ...value,
                  includeTemplateContent: false,
                  template: selected
                    ? {
                        kind: 'enterprise',
                        id: selected.id,
                        version: selected.version,
                      }
                    : undefined,
                });
              }}
            >
              <MenuItem value=''>使用默认模板</MenuItem>
              {!!selectedTemplate &&
                !personalTemplates.some(
                  t => `upload:${t.id}` === selectedTemplate,
                ) &&
                !templates.some(
                  t => `${t.id}:${t.version}` === selectedTemplate,
                ) && (
                  <MenuItem value={selectedTemplate} disabled>
                    已选模板
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
              {personalTemplates.map(file => (
                <MenuItem key={file.id} value={`upload:${file.id}`}>
                  套用 {file.name} 的版式
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {templatesStatus === 'error' && (
            <Button
              size='small'
              disabled={disabled}
              onClick={() => setTemplateRetry(n => n + 1)}
            >
              重试加载企业模板
            </Button>
          )}
          {templateAttachment && (
            <FormControlLabel
              label='同时参考这份 PPT 的正文'
              control={
                <Checkbox
                  checked={!!value.includeTemplateContent}
                  onChange={(_, checked) =>
                    change({ ...value, includeTemplateContent: checked })
                  }
                  disabled={disabled}
                />
              }
            />
          )}
        </Stack>
      )}
      {value.scope.type === 'documents' && (
        <Chip
          size='small'
          sx={{ alignSelf: 'flex-start', maxWidth: '100%' }}
          label={
            selectedDocuments.length
              ? `知识库范围：${selectedDocuments.map(doc => doc.name).join('、')}`
              : '仅参考上传资料'
          }
          onClick={disabled ? undefined : () => setMoreOpen(true)}
          onDelete={
            disabled
              ? undefined
              : () => change({ ...value, scope: { type: 'all' } })
          }
        />
      )}
      {moreOpen && (
        <Stack gap={1}>
          <FormControl
            size='small'
            sx={{ minWidth: 155, alignSelf: 'flex-start' }}
          >
            <InputLabel>资料范围</InputLabel>
            <Select
              label='资料范围'
              value={value.scope.type}
              disabled={disabled}
              onChange={event =>
                change({
                  ...value,
                  scope:
                    event.target.value === 'documents'
                      ? { type: 'documents', node_ids: [] }
                      : { type: 'all' },
                })
              }
            >
              <MenuItem value='all'>全部授权知识</MenuItem>
              <MenuItem value='documents'>指定文档 / 目录</MenuItem>
            </Select>
          </FormControl>
          {value.scope.type === 'documents' && (
            <Stack direction='row' gap={1} alignItems='flex-start'>
              <Autocomplete
                multiple
                fullWidth
                size='small'
                disabled={disabled}
                options={documentOptions}
                filterOptions={items => items}
                getOptionLabel={doc =>
                  (doc.type === 1 ? '目录：' : '') + doc.name
                }
                isOptionEqualToValue={(a, b) => a.id === b.id}
                value={selectedDocuments}
                onInputChange={(_, query) => setDocumentQuery(query)}
                onChange={(_, items) => {
                  setRememberedDocuments(items);
                  change({
                    ...value,
                    scope: {
                      type: 'documents',
                      node_ids: items.map(doc => doc.id),
                    },
                  });
                }}
                renderInput={params => (
                  <TextField
                    {...params}
                    label='搜索授权文档或目录'
                    error={pickerError}
                    helperText={
                      pickerError
                        ? '资料列表读取失败，可重新搜索'
                        : '未选择文档或目录时，只使用上传资料。'
                    }
                  />
                )}
              />
              {currentNode && (
                <Button
                  size='small'
                  disabled={disabled}
                  sx={{ flexShrink: 0 }}
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
        </Stack>
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
            label={`${a.name} · ${a.status === 'ready' ? '可使用' : a.status === 'failed' ? '解析失败' : a.status === 'processing' ? '解析中' : '等待处理'}`}
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
          {showPptTemplate && a.report?.template_error && (
            <Typography variant='caption' color='text.secondary'>
              这份文件可作为参考资料；使用PPT模板时，请选择包含标题和正文占位符的16:9模板。
            </Typography>
          )}
        </Stack>
      ))}
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
                  includeTemplateContent: false,
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
