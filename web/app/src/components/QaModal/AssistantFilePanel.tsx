'use client';
import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Checkbox,
  FormControl,
  IconButton,
  InputLabel,
  ListItemText,
  LinearProgress,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import {
  assistantRequest,
  downloadAssistantContent,
  type AssistantArtifact,
} from '@/request/Assistant';
import AssistantArtifactPreview from './AssistantArtifactPreview';
import { reportAssistantIssue } from './assistantFeedback.mjs';

export default function AssistantFilePanel({
  file,
  files,
  onSelect,
  onClose,
  onRevise,
}: {
  file: AssistantArtifact;
  files: AssistantArtifact[];
  onSelect: (file: AssistantArtifact) => void;
  onClose: () => void;
  onRevise: (id: string, targets?: string[]) => void;
}) {
  const [error, setError] = useState(''),
    [allowed, setAllowed] = useState(false),
    [busy, setBusy] = useState(false);
  const [outline, setOutline] = useState<
    NonNullable<AssistantArtifact['outline']>
  >([]);
  const [targets, setTargets] = useState<string[]>([]);
  useEffect(() => {
    const ac = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    setAllowed(false);
    setError('');
    setOutline([]);
    setTargets([]);
    const check = async () => {
      try {
        const current = await assistantRequest<AssistantArtifact>(
          `/artifacts/${file.id}`,
          'GET',
          undefined,
          ac.signal,
        );
        if (!ac.signal.aborted) {
          setAllowed(true);
          setOutline(current.outline || []);
          setError('');
        }
      } catch (e) {
        if (!ac.signal.aborted) {
          setAllowed(false);
          setOutline([]);
          setError(reportAssistantIssue('preview', e));
        }
      } finally {
        if (!ac.signal.aborted)
          timer = setTimeout(() => {
            if (document.hidden) timer = setTimeout(check, 10000);
            else void check();
          }, 30000);
      }
    };
    void check();
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [file.id]);
  const download = async () => {
    setBusy(true);
    try {
      await downloadAssistantContent(
        `/artifacts/${file.id}/content`,
        file.name,
      );
    } catch (e) {
      setError(reportAssistantIssue('download', e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Stack
      aria-label='文件工作区'
      sx={{
        width: { xs: '100%', md: '57%' },
        minWidth: 0,
        minHeight: 0,
        height: '100%',
        borderLeft: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.paper',
        pl: { xs: 0, md: 2 },
      }}
    >
      <Stack direction='row' gap={1} alignItems='center' pb={1}>
        <Select
          size='small'
          value={file.id}
          onChange={e => {
            const item = files.find(f => f.id === e.target.value);
            if (item) onSelect(item);
          }}
          sx={{ flex: 1, minWidth: 0 }}
          aria-label='选择文件版本'
        >
          {files.map(f => (
            <MenuItem key={f.id} value={f.id}>
              {f.name} · v{f.revision}
            </MenuItem>
          ))}
        </Select>
        <IconButton aria-label='关闭文件预览，回到对话' onClick={onClose}>
          <CloseIcon />
        </IconButton>
      </Stack>
      <Stack direction='row' gap={1} pb={1} flexWrap='wrap'>
        <Button
          size='small'
          variant='contained'
          disabled={!allowed || busy}
          onClick={() => onRevise(file.id, targets)}
        >
          继续修改此版本
        </Button>
        <Button size='small' disabled={!allowed || busy} onClick={download}>
          下载 {file.format.toUpperCase()}
        </Button>
        <Typography
          variant='caption'
          color='text.secondary'
          sx={{ alignSelf: 'center' }}
        >
          旧版本会保留
        </Typography>
      </Stack>
      {allowed &&
        ['pptx', 'docx'].includes(file.format) &&
        outline.length > 0 && (
          <FormControl size='small' sx={{ mb: 1 }}>
            <InputLabel id='assistant-edit-range' shrink>
              修改范围
            </InputLabel>
            <Select
              multiple
              labelId='assistant-edit-range'
              label='修改范围'
              notched
              value={targets}
              onChange={e => {
                const values =
                  typeof e.target.value === 'string'
                    ? e.target.value.split(',')
                    : e.target.value;
                setTargets(values.includes('all') ? [] : values);
              }}
              renderValue={values =>
                values.length
                  ? `已选择 ${values.length} ${file.format === 'pptx' ? '页' : '节'}，其他内容保持原样`
                  : '整份文件（也可以在对话中指定）'
              }
              displayEmpty
            >
              <MenuItem value='all'>
                <Checkbox checked={targets.length === 0} />
                <ListItemText primary='整份文件' />
              </MenuItem>
              {outline.map(part => (
                <MenuItem key={part.id} value={part.id}>
                  <Checkbox checked={targets.includes(part.id)} />
                  <ListItemText
                    primary={`第${part.position}${part.unit} · ${part.title}`}
                  />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
      {error && (
        <Typography variant='body2' color='text.secondary' role='status'>
          {error}
        </Typography>
      )}
      {!allowed && !error && <LinearProgress aria-label='正在打开文件' />}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflow: 'hidden',
          bgcolor: 'action.hover',
          borderRadius: 1.5,
        }}
      >
        {allowed && <AssistantArtifactPreview file={file} />}
      </Box>
    </Stack>
  );
}
