import { Box } from '@mui/material';
import { useMemo } from 'react';
import { useLocation } from 'react-router-dom';

const PATH_TITLE_MAP: Record<string, string> = {
  '/': '文档',
  '/stat': '统计',
  '/conversation': '问答',
  '/comment': '评论',
  '/release': '发布',
  '/setting': '设置',
};

const Bread = () => {
  const { pathname } = useLocation();
  const title = useMemo(() => {
    const matched = Object.entries(PATH_TITLE_MAP).find(([path]) => {
      if (path === '/') {
        return pathname === '/';
      }
      return pathname.startsWith(path);
    });
    return matched?.[1] || '知识库管理台';
  }, [pathname]);

  return (
    <Box
      sx={{
        flexGrow: 1,
        fontSize: '14px',
        fontWeight: 'bold',
        color: 'text.primary',
        lineHeight: '22px',
      }}
    >
      {title}
    </Box>
  );
};

export default Bread;
