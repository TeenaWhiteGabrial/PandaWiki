'use client';

import dynamic from 'next/dynamic';

const NodeClientLayout = dynamic(() => import('./NodeClientLayout'), {
  ssr: false,
  loading: () => <div style={{ minHeight: '100vh' }} />,
});

export default function NodeClientLayoutShell({
  children,
}: {
  children: React.ReactNode;
}) {
  return <NodeClientLayout>{children}</NodeClientLayout>;
}
