import router from '@/router';
import { theme } from '@/themes';
import { ThemeProvider } from '@/components/ui';
import { useRoutes } from 'react-router-dom';

import '@ctzhian/tiptap/dist/index.css';

function App() {
  try {
    localStorage.removeItem('panda_wiki_token');
  } catch {}
  for (const name of ['panda_wiki_token', 'token', '_pw_auth_session']) {
    document.cookie = `${name}=; Max-Age=0; path=/; SameSite=Strict`;
  }
  const routerView = useRoutes(router);

  return (
    <ThemeProvider theme={theme} defaultMode='light' storageManager={null}>
      {routerView}
    </ThemeProvider>
  );
}

export default App;
