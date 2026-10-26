export const AUTH_CLEARED_EVENT = 'panda-wiki-auth-cleared';

export const clearLegacyCredentials = () => {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.removeItem('panda_wiki_token');
  } catch {}
  for (const name of ['panda_wiki_token', 'token', '_pw_auth_session']) {
    document.cookie = `${name}=; Max-Age=0; path=/; SameSite=Strict`;
  }
};

export const clearAuthStorage = () => {
  if (typeof window === 'undefined') return;
  clearLegacyCredentials();
  // This is only an identity-change notification, never a credential.
  try {
    window.localStorage.setItem('pw-reader-auth-change', String(Date.now()));
  } catch {}
  window.dispatchEvent(new Event(AUTH_CLEARED_EVENT));
};
