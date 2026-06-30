import {
  ThemeProvider as MuiThemeProvider,
  type StorageManager,
  type Theme,
} from '@mui/material/styles';
import type { ReactNode } from 'react';

interface ThemeProviderProps {
  children: ReactNode;
  theme: Theme;
  defaultMode?: 'light' | 'dark' | 'system';
  storageManager?: StorageManager | null;
}

const ThemeProvider = ({
  children,
  theme,
  defaultMode,
  storageManager,
}: ThemeProviderProps) => {
  return (
    <MuiThemeProvider
      theme={theme}
      defaultMode={defaultMode}
      storageManager={storageManager ?? undefined}
    >
      {children}
    </MuiThemeProvider>
  );
};

export default ThemeProvider;
