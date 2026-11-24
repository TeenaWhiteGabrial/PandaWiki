'use client';
import { darkTheme, lightTheme } from '@/theme';
import { ThemeProvider } from '@ctzhian/ui';
import { createTheme } from '@mui/material';
import Cookies from 'js-cookie';
import { createContext, useContext, useEffect, useMemo, useState } from 'react';

const ThemeContext = createContext<{
  themeMode: 'light' | 'dark';
  setThemeMode: (themeMode: 'light' | 'dark') => void;
}>({
  themeMode: 'light',
  setThemeMode: () => {},
});

export const useThemeStore = () => {
  return useContext(ThemeContext);
};

export const ThemeStoreProvider = ({
  children,
  themeMode: initialThemeMode,
}: {
  themeMode: 'light' | 'dark';
  children: React.ReactNode;
}) => {
  const [themeMode, setThemeMode] = useState<'light' | 'dark'>(
    initialThemeMode,
  );
  const theme = useMemo(() => {
    return createTheme(themeMode === 'dark' ? darkTheme : lightTheme);
  }, [themeMode]);

  useEffect(() => {
    Cookies.set('theme_mode', themeMode, {
      expires: 365 * 10,
      sameSite: 'Strict',
    });
  }, [themeMode]);

  return (
    <ThemeContext.Provider value={{ themeMode, setThemeMode }}>
      <ThemeProvider theme={theme}>{children}</ThemeProvider>
    </ThemeContext.Provider>
  );
};
