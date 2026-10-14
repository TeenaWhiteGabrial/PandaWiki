import { Suspense } from 'react';
import Login from '@/views/auth/login';

export default function LoginClientShell() {
  return (
    <Suspense fallback={<div style={{ minHeight: '100vh' }} />}>
      <Login />
    </Suspense>
  );
}
