import router from '@/router';
import { useAppDispatch } from '@/store';
import { theme } from '@/themes';
import { ThemeProvider } from '@ctzhian/ui';
import { useEffect } from 'react';
import { useLocation, useRoutes } from 'react-router-dom';

import { setLicense } from './store/slices/config';

import '@ctzhian/tiptap/dist/index.css';

function App() {
  const location = useLocation();
  const { pathname } = location;
  const dispatch = useAppDispatch();
  const routerView = useRoutes(router);
  const loginPage = pathname.includes('/login');
  const onlyAllowShareApi = loginPage;

  const token = localStorage.getItem('panda_wiki_token') || '';

  useEffect(() => {
    if (token) {
      dispatch(
        setLicense({
          edition: 0,
          expired_at: 0,
          started_at: 0,
        }),
      );
    }
  }, [dispatch, token]);

  if (!token && !onlyAllowShareApi) {
    window.location.href = window.__BASENAME__ + '/login';
    return null;
  }

  return (
    <ThemeProvider theme={theme} defaultMode='light' storageManager={null}>
      {routerView}
    </ThemeProvider>
  );
}

export default App;
