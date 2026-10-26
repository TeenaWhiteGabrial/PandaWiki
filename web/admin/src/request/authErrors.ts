type AuthErrorResponse = {
  status?: number;
  data?: { code?: number; success?: boolean };
  config?: { url?: string };
};

let redirecting = false;

// A background session check is navigation state, not a failed user operation.
// Endpoint-specific permission failures must still reach the normal error UI.
export const handleAdminAuthError = (response?: AuthErrorResponse): boolean => {
  if (!response || response.data?.success === true) return false;

  const base = (window.__BASENAME__ || '').replace(/\/$/, '');
  const loginPath = `${base}/login`;
  const requestPath = (response.config?.url || '').split('?')[0];
  if (requestPath === '/api/v1/user/login') return false;

  const sessionInvalid =
    response.data?.code === 40101 ||
    (response.status === 401 && requestPath === '/api/v1/user');
  const changeRequired =
    response.status === 403 && response.data?.code === 40302;
  const administratorDenied =
    response.status === 403 && response.data?.code === 40301;
  if (!sessionInvalid && !changeRequired && !administratorDenied) return false;

  if (sessionInvalid || administratorDenied) {
    try {
      localStorage.removeItem('panda_wiki_token');
    } catch {}
  }

  if (
    window.location.pathname.replace(/\/$/, '') !== loginPath &&
    !redirecting
  ) {
    redirecting = true;
    const query = changeRequired
      ? 'change_password=1'
      : `redirect=${encodeURIComponent(
          window.location.pathname.slice(base.length) + window.location.search,
        )}`;
    window.location.href = `${loginPath}?${query}`;
  }
  return !administratorDenied;
};
