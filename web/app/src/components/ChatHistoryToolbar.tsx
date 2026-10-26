'use client';

import { useEffect, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  Pagination,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import HistoryIcon from '@mui/icons-material/History';
import AddIcon from '@mui/icons-material/Add';
import CloseIcon from '@mui/icons-material/Close';
import dayjs from 'dayjs';
import httpRequest, { ContentType } from '@/request/httpClient';
import { useStore } from '@/provider';
import type { HistoryCredential } from '@/utils/chatSession';

type HistoryItem = {
  id: string;
  subject: string;
  created_at: string;
  updated_at: string;
};
type HistoryPage = { data: HistoryItem[]; total: number };

export default function ChatHistoryToolbar({
  conversationId,
  disabled,
  credentials,
  onSelect,
  onNew,
}: {
  conversationId: string;
  disabled: boolean;
  credentials: () => HistoryCredential[];
  onSelect: (id: string) => Promise<boolean>;
  onNew: () => void;
}) {
  const { authInfo } = useStore();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<HistoryPage>({ data: [], total: 0 });
  const [loading, setLoading] = useState(false);
  const [selecting, setSelecting] = useState('');
  const [error, setError] = useState('');
  const [refresh, setRefresh] = useState(0);
  const credentialsRef = useRef(credentials);
  credentialsRef.current = credentials;

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(draft.trim());
      setPage(1);
    }, 250);
    return () => clearTimeout(timer);
  }, [draft]);

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setLoading(true);
    setError('');
    httpRequest<{ data: HistoryPage }>({
      path: '/share/v1/conversation/history',
      method: 'POST',
      body: { page, per_page: 20, query, sessions: credentialsRef.current() },
      type: ContentType.Json,
      format: 'json',
      isAlert: false,
      signal: controller.signal,
    })
      .then(data => {
        if (!controller.signal.aborted) setResult(data);
      })
      .catch(error => {
        if (!controller.signal.aborted)
          setError(error?.message || '加载历史记录失败，请重试');
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [open, page, query, refresh, authInfo?.id]);

  const select = async (id: string) => {
    setSelecting(id);
    try {
      if (await onSelect(id)) setOpen(false);
    } finally {
      setSelecting('');
    }
  };

  return (
    <>
      <Stack
        direction='row'
        justifyContent='space-between'
        alignItems='center'
        sx={{ mb: 1.5, px: 1 }}
      >
        <Button
          size='small'
          startIcon={<HistoryIcon />}
          disabled={disabled}
          onClick={() => {
            setPage(1);
            setOpen(true);
          }}
        >
          历史记录
        </Button>
        <Button
          size='small'
          startIcon={<AddIcon />}
          disabled={disabled}
          onClick={onNew}
        >
          新会话
        </Button>
      </Stack>
      <Dialog
        open={open}
        onClose={() => {
          if (!selecting) setOpen(false);
        }}
        fullWidth
        maxWidth='sm'
        aria-labelledby='chat-history-title'
      >
        <DialogTitle id='chat-history-title' sx={{ pr: 6 }}>
          历史记录
          <IconButton
            aria-label='关闭历史记录'
            disabled={!!selecting}
            onClick={() => setOpen(false)}
            sx={{ position: 'absolute', right: 12, top: 12 }}
          >
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent sx={{ pb: 2 }}>
          <Typography variant='body2' color='text.secondary' sx={{ mb: 2 }}>
            {authInfo?.id
              ? '点击记录可查看历史问答，并在原会话中继续提问。'
              : '请先登录后查看历史记录并继续提问。'}
          </Typography>
          <TextField
            autoFocus
            fullWidth
            size='small'
            placeholder='搜索会话标题'
            inputProps={{ 'aria-label': '搜索会话标题', maxLength: 200 }}
            value={draft}
            disabled={!!selecting}
            onChange={event => setDraft(event.target.value)}
          />
          <Box
            sx={{
              minHeight: 180,
              maxHeight: '50vh',
              overflowY: 'auto',
              mt: 1.5,
            }}
            aria-busy={loading || !!selecting}
          >
            {loading ? (
              <Stack alignItems='center' sx={{ py: 6 }}>
                <CircularProgress size={24} />
                <Typography variant='body2' sx={{ mt: 1 }}>
                  正在加载历史记录…
                </Typography>
              </Stack>
            ) : error ? (
              <Alert
                severity='error'
                action={
                  <Button onClick={() => setRefresh(value => value + 1)}>
                    重试
                  </Button>
                }
              >
                {error}
              </Alert>
            ) : result.data.length === 0 ? (
              <Typography color='text.secondary' align='center' sx={{ py: 6 }}>
                {query
                  ? '没有匹配的历史记录'
                  : '暂无历史记录，开始提问后会自动保存'}
              </Typography>
            ) : (
              <List disablePadding>
                {result.data.map(item => (
                  <ListItemButton
                    key={item.id}
                    selected={item.id === conversationId}
                    disabled={!!selecting}
                    onClick={() => void select(item.id)}
                    sx={{ borderRadius: 1, mb: 0.5 }}
                  >
                    <ListItemText
                      primary={item.subject || '未命名会话'}
                      secondary={`最近对话：${dayjs(item.updated_at || item.created_at).format('YYYY-MM-DD HH:mm')}`}
                      primaryTypographyProps={{ noWrap: true, fontSize: 14 }}
                      secondaryTypographyProps={{ fontSize: 12 }}
                    />
                    {selecting === item.id ? (
                      <CircularProgress size={18} />
                    ) : item.id === conversationId ? (
                      <Typography
                        variant='caption'
                        color='primary'
                        sx={{ ml: 1, flexShrink: 0 }}
                      >
                        当前会话
                      </Typography>
                    ) : null}
                  </ListItemButton>
                ))}
              </List>
            )}
          </Box>
          {!loading && !error && result.total > 20 && (
            <Stack alignItems='center' sx={{ mt: 2 }}>
              <Pagination
                size='small'
                page={page}
                count={Math.ceil(result.total / 20)}
                disabled={!!selecting}
                onChange={(_, value) => setPage(value)}
              />
            </Stack>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
