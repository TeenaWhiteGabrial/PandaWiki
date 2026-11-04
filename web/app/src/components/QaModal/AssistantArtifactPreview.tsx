'use client';
import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tabs,
  Tab,
  Typography,
} from '@mui/material';
import SecureFilePreview from '@panda-wiki/file-preview';
import { assistantRequest, type AssistantArtifact } from '@/request/Assistant';
import { useBasePath } from '@/hooks';
import { reportAssistantIssue } from './assistantFeedback.mjs';

export default function AssistantArtifactPreview({
  file,
}: {
  file: AssistantArtifact;
}) {
  const base = useBasePath();
  const [sheets, setSheets] = useState<
    { name: string; columns: string[]; rows: unknown[][] }[]
  >([]);
  const [sheet, setSheet] = useState(0),
    [page, setPage] = useState(0);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [retry, setRetry] = useState(0);
  useEffect(() => {
    if (file.format !== 'xlsx') return;
    const ac = new AbortController();
    setBusy(true);
    setError('');
    setSheets([]);
    setSheet(0);
    setPage(0);
    assistantRequest<{ sheets: typeof sheets }>(
      `/artifacts/${file.id}/sheets`,
      'GET',
      undefined,
      ac.signal,
    )
      .then(data => {
        if (!ac.signal.aborted) setSheets(data.sheets);
      })
      .catch(e => {
        if (!ac.signal.aborted) setError(reportAssistantIssue('preview', e));
      })
      .finally(() => {
        if (!ac.signal.aborted) setBusy(false);
      });
    return () => ac.abort();
  }, [file.id, file.format, retry]);
  if (file.format !== 'xlsx')
    return file.preview_state === 'ready' ? (
      <SecureFilePreview
        key={file.id}
        unavailableMessage='预览暂未准备好，可下载文件查看。'
        title={file.name}
        kind='pdf'
        assetBase={base}
        url={`${base}/share/v1/assistant/artifacts/${file.id}/content?preview=true`}
      />
    ) : (
      <Stack alignItems='center' justifyContent='center' height='100%' gap={1}>
        <Typography color='text.secondary'>
          文件已生成，可下载后查看。
        </Typography>
      </Stack>
    );
  if (busy)
    return (
      <Stack alignItems='center' justifyContent='center' height='100%' gap={1}>
        <CircularProgress size={24} />
        <Typography variant='body2'>正在打开工作簿…</Typography>
      </Stack>
    );
  if (error)
    return (
      <Stack alignItems='center' gap={1}>
        <Typography>{error}</Typography>
        <Button onClick={() => setRetry(x => x + 1)}>重新打开</Button>
      </Stack>
    );
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        minHeight: 0,
      }}
    >
      <Tabs
        value={sheet}
        variant='scrollable'
        onChange={(_, n) => {
          setSheet(n);
          setPage(0);
        }}
      >
        {sheets.map((s, i) => (
          <Tab key={i} label={s.name} />
        ))}
      </Tabs>
      <TableContainer sx={{ flex: 1, minHeight: 0 }}>
        <Table stickyHeader size='small'>
          <TableHead>
            <TableRow>
              {sheets[sheet]?.columns.map((c, i) => (
                <TableCell key={i} sx={{ whiteSpace: 'nowrap' }}>
                  {c}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {sheets[sheet]?.rows
              .slice(page * 20, page * 20 + 20)
              .map((row, i) => (
                <TableRow key={i}>
                  {row.map((v, j) => (
                    <TableCell key={j}>
                      {v && typeof v === 'object'
                        ? '公式结果见下载文件'
                        : String(v ?? '')}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination
        component='div'
        count={sheets[sheet]?.rows.length || 0}
        page={page}
        rowsPerPage={20}
        rowsPerPageOptions={[20]}
        onPageChange={(_, p) => setPage(p)}
      />
    </Box>
  );
}
