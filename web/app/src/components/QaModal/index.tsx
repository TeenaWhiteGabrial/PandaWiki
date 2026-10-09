'use client';
import React, { useState, useEffect, useRef, useMemo } from 'react';
import { IconZhinengwenda, IconJinsousuo } from '@panda-wiki/icons';
import { useSearchParams } from 'next/navigation';
import {
  Box,
  IconButton,
  Tooltip,
  Typography,
  Modal,
  Stack,
  lighten,
  alpha,
  styled,
  Tabs,
  Tab,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import FullscreenIcon from '@mui/icons-material/Fullscreen';
import FullscreenExitIcon from '@mui/icons-material/FullscreenExit';
import AiQaContent from './AiQaContent';
import SearchDocContent from './SearchDocContent';
import { useStore } from '@/provider';

interface SearchSuggestion {
  id: string;
  title: string;
  description?: string;
  type?: 'recent' | 'suggestion' | 'trending';
}

interface QaModalProps {
  placeholder?: string;
  initialValue?: string;
  onSearch?: (value?: string, type?: 'search' | 'chat') => void;
  onSearchSuggestions?: (query: string) => Promise<SearchSuggestion[]>;
  defaultSuggestions?: SearchSuggestion[];
}

const StyledTabs = styled(Tabs)(({ theme }) => ({
  minHeight: 'auto',
  position: 'relative',
  borderRadius: '10px',
  padding: theme.spacing(0.5),
  border: `1px solid ${alpha(theme.palette.text.primary, 0.1)}`,
  '& .MuiTabs-indicator': {
    height: '100%',
    borderRadius: '8px',
    backgroundColor: theme.palette.primary.main,
    transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
    zIndex: 0,
  },
  '& .MuiTabs-flexContainer': {
    gap: theme.spacing(0.5),
    position: 'relative',
    zIndex: 1,
  },
}));

// 样式化的 Tab 组件 - 白色背景，圆角，深灰色文字
const StyledTab = styled(Tab)(({ theme }) => ({
  minHeight: 'auto',
  padding: theme.spacing(0.75, 2),
  borderRadius: '6px',
  backgroundColor: 'transparent',
  fontSize: 12,
  fontWeight: 400,
  textTransform: 'none',
  transition: 'color 0.3s ease-in-out',
  position: 'relative',
  zIndex: 1,
  lineHeight: 1,
  '&:hover': {
    color: theme.palette.text.primary,
  },
  '&.Mui-selected': {
    color: theme.palette.primary.contrastText,
    fontWeight: 500,
  },
}));

const QaModal: React.FC<QaModalProps> = () => {
  const { qaModalOpen, setQaModalOpen, kbDetail, mobile } = useStore();
  const [searchMode, setSearchMode] = useState<'chat' | 'search'>('chat');
  const [fullScreen, setFullScreen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const aiQaInputRef = useRef<HTMLInputElement>(null);
  const searchParams = useSearchParams();
  const onClose = () => {
    setQaModalOpen?.(false);
    setFullScreen(false);
  };

  const placeholder = useMemo(() => {
    return (
      kbDetail?.settings?.web_app_custom_style?.header_search_placeholder ||
      '搜索...'
    );
  }, [kbDetail]);

  const hotSearch = useMemo(() => {
    const bannerConfig = kbDetail?.settings?.web_app_landing_configs?.find(
      item => item.type === 'banner',
    );
    return bannerConfig?.banner_config?.hot_search || [];
  }, [kbDetail]);

  // modal打开时自动聚焦
  useEffect(() => {
    if (qaModalOpen) {
      setTimeout(() => {
        if (searchMode === 'chat') {
          aiQaInputRef.current?.querySelector('textarea')?.focus();
        } else {
          inputRef.current?.querySelector('input')?.focus();
        }
      }, 100);
    }
  }, [qaModalOpen, searchMode]);

  useEffect(() => {
    if (!qaModalOpen) {
      setFullScreen(false);
      const timer = setTimeout(() => {
        setSearchMode('chat');
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [qaModalOpen]);

  useEffect(() => {
    const cid = searchParams.get('cid');
    const ask = searchParams.get('ask');
    if (cid || ask) {
      setQaModalOpen?.(true);
    }
  }, []);

  return (
    <Modal
      open={qaModalOpen as boolean}
      onClose={(_, reason) => {
        if (reason === 'escapeKeyDown' && fullScreen) {
          setFullScreen(false);
        } else {
          onClose();
        }
      }}
      slotProps={{ backdrop: { sx: { backdropFilter: 'blur(3px)' } } }}
      sx={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        p: fullScreen ? 0 : { xs: 0, sm: 2.5 },
      }}
    >
      <Box
        role='dialog'
        aria-modal='true'
        aria-label='智能问答与文档搜索'
        sx={theme => ({
          display: 'flex',
          flexDirection: 'column',
          width: '100%',
          maxWidth: fullScreen ? 'none' : 1120,
          height: fullScreen
            ? '100dvh'
            : { xs: '100dvh', sm: 'min(900px, calc(100dvh - 40px))' },
          maxHeight: '100%',
          minHeight: 0,
          backgroundColor: lighten(theme.palette.background.default, 0.05),
          borderRadius: fullScreen ? 0 : { xs: 0, sm: '18px' },
          boxShadow: '0 24px 80px rgba(0, 0, 0, 0.22)',
          overflow: 'hidden',
          outline: 'none',
          pb: { xs: 1.5, sm: 2 },
        })}
        onClick={e => e.stopPropagation()}
      >
        {/* 顶部标签栏 */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1,
            flexShrink: 0,
            px: { xs: 1.5, sm: 3 },
            py: 1.5,
            borderBottom: '1px solid',
            borderColor: 'divider',
          }}
        >
          <StyledTabs
            value={searchMode}
            onChange={(_, value) => {
              setSearchMode(value as 'chat' | 'search');
            }}
            variant='scrollable'
            scrollButtons={false}
          >
            <StyledTab
              aria-label='智能问答'
              label={
                <Stack direction='row' gap={0.5} alignItems='center'>
                  <IconZhinengwenda sx={{ fontSize: 16 }} />
                  {!mobile && <span>智能问答</span>}
                </Stack>
              }
              value='chat'
            />
            <StyledTab
              aria-label='仅搜索文档'
              label={
                <Stack direction='row' gap={0.5} alignItems='center'>
                  <IconJinsousuo sx={{ fontSize: 16 }} />
                  {!mobile && <span>仅搜索文档</span>}
                </Stack>
              }
              value='search'
            />
          </StyledTabs>

          <Stack direction='row' gap={0.5} sx={{ flexShrink: 0 }}>
            <Tooltip title={fullScreen ? '退出全屏（Esc）' : '全屏显示'}>
              <IconButton
                aria-label={fullScreen ? '退出全屏' : '全屏显示'}
                aria-pressed={fullScreen}
                onClick={() => setFullScreen(value => !value)}
                size='small'
                sx={{ display: { xs: 'none', sm: 'inline-flex' } }}
              >
                {fullScreen ? <FullscreenExitIcon /> : <FullscreenIcon />}
              </IconButton>
            </Tooltip>
            <Tooltip title='关闭对话框'>
              <IconButton
                aria-label='关闭对话框'
                onClick={onClose}
                size='small'
              >
                <CloseIcon />
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>

        {/* 主内容区域 - 根据模式切换 */}
        <Box
          sx={{
            px: { xs: 1.5, sm: 3, lg: 4 },
            pt: 1.5,
            flex: 1,
            minHeight: 0,
            minWidth: 0,
            width: '100%',
            maxWidth: 1280,
            alignSelf: 'center',
            display: searchMode === 'chat' ? 'flex' : 'none',
            flexDirection: 'column',
          }}
        >
          <AiQaContent
            hotSearch={hotSearch}
            placeholder={placeholder}
            inputRef={aiQaInputRef}
          />
        </Box>
        <Box
          sx={{
            px: { xs: 1.5, sm: 3, lg: 4 },
            py: 2,
            flex: 1,
            minHeight: 0,
            minWidth: 0,
            overflow: 'auto',
            width: '100%',
            maxWidth: 1280,
            alignSelf: 'center',
            display: searchMode === 'search' ? 'flex' : 'none',
            flexDirection: 'column',
          }}
        >
          <SearchDocContent inputRef={inputRef} placeholder={placeholder} />
        </Box>

        {/* 底部AI生成提示 */}
        <Box
          sx={{
            px: 3,
            flexShrink: 0,
            pt: !kbDetail?.settings?.conversation_setting
              ?.copyright_hide_enabled
              ? 2
              : 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Typography
            variant='caption'
            sx={{
              color: 'text.disabled',
              fontSize: 12,
              display: 'flex',
              alignItems: 'center',
              gap: 1,
            }}
          >
            <Box>
              {!kbDetail?.settings?.conversation_setting
                ?.copyright_hide_enabled &&
                (kbDetail?.settings?.conversation_setting?.copyright_info ||
                  '本系统由内部知识库平台提供服务')}
            </Box>
          </Typography>
        </Box>
      </Box>
    </Modal>
  );
};

export default QaModal;
