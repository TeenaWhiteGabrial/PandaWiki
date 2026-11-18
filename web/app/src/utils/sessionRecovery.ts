import { clearAuthStorage } from './auth';

const identityPath = '/share/v1/auth/user';
let verification: Promise<void> | undefined;
let redirecting = false;

export const redirectToReaderLogin = () => {
  if (typeof window === 'undefined' || redirecting) return;
  const base = (window._BASE_PATH_ || '').replace(/\/$/, '');
  const { pathname, search } = window.location;
  const path =
    base && (pathname === base || pathname.startsWith(`${base}/`))
      ? pathname.slice(base.length) || '/'
      : pathname;
  if (path !== '/auth/login') redirecting = true;
  clearAuthStorage();
  if (path !== '/auth/login') {
    window.location.href = `${base}/auth/login?redirect=${encodeURIComponent(path + search)}`;
  }
};

// Use raw fetch, rather than the API client, to avoid recursive 401 recovery.
// Share the complete verification (including logout) across concurrent callers.
export const handleReaderAuthFailure = async (
  status: number,
  code?: number,
  path?: string,
): Promise<void> => {
  if (typeof window === 'undefined' || redirecting) return;
  // A rejected login attempt must remain on the login form with its own error.
  if (path?.startsWith('/share/v1/auth/login/')) return;
  if (code === 40101 || (status === 401 && path === identityPath)) {
    redirectToReaderLogin();
    return;
  }
  if (status !== 401) return;
  if (!verification) {
    verification = (async () => {
      try {
        const base = (window._BASE_PATH_ || '').replace(/\/$/, '');
        const response = await fetch(`${base}${identityPath}`, {
          method: 'GET',
          credentials: 'same-origin',
          cache: 'no-store',
          redirect: 'error',
          headers: { 'X-PW-CSRF': '1' },
          signal: AbortSignal.timeout(8000),
        });
        const body = await response.json().catch(() => undefined);
        if (response.status === 401 || body?.code === 40101) {
          redirectToReaderLogin();
        }
      } catch {
        // Network errors, timeouts and unavailable identity services do not
        // prove that a session expired. Preserve the original request error.
      }
    })().finally(() => {
      verification = undefined;
    });
  }
  await verification;
};
