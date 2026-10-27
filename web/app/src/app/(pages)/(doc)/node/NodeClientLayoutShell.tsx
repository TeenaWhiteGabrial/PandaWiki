'use client';

import dynamic from 'next/dynamic';
import { CircularProgress, Stack } from '@mui/material';

const NodeClientLayout = dynamic(() => import('./NodeClientLayout'), {
  ssr: false,
  loading: () => (
    <Stack
      alignItems='center'
      justifyContent='center'
      gap={2}
      sx={{ minHeight: '100vh' }}
      role='status'
    >
      <CircularProgress size={28} />
      正在加载目录…
    </Stack>
  ),
});

export default function NodeClientLayoutShell({
  children,
}: {
  children: React.ReactNode;
}) {
  return <NodeClientLayout>{children}</NodeClientLayout>;
}
