export async function clearCookie() {
  if (typeof window === 'undefined') {
  } else {
    document.cookie =
      '_pw_auth_session=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
    document.cookie =
      'panda_wiki_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
  }
}
