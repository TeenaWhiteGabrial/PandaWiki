'use client';

import dynamic from 'next/dynamic';

const Login = dynamic(() => import('@/views/auth/login'), {
  ssr: false,
  loading: () => <div style={{ minHeight: '100vh' }} />,
});

export default function LoginClientShell() {
  return <Login />;
}
