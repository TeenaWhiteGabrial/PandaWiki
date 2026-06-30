'use client';
import { createTheme, CssVarsThemeOptions } from '@mui/material';
import { zhCN } from '@mui/material/locale';
import { zhCN as CuiZhCN } from '@ctzhian/ui/dist/local';
import { darkPalette, lightPalette } from '@panda-wiki/themes';

const createComponentStyleOverrides = (
  defaultColor: boolean = true,
): CssVarsThemeOptions['components'] => ({
  MuiInputBase: {
    styleOverrides: {
      root: ({ theme }) => ({
        borderRadius: '10px !important',
        '.MuiOutlinedInput-notchedOutline': {
          borderColor: theme.palette.divider,
        },
        '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
          borderColor: 'var(--mui-palette-text-primary) !important',
          borderWidth: '1px !important',
        },
      }),
    },
  },
  MuiSvgIcon: {
    styleOverrides: {
      root: {
        fontSize: '1em',
      },
    },
  },

  MuiButton: {
    defaultProps: {
      color: defaultColor ? 'primary' : 'dark',
    },
    styleOverrides: {
      root: {
        fontWeight: 400,
        borderRadius: '10px',
        boxShadow: 'none',
        '&:hover': {
          boxShadow: 'none',
        },
      },
    },
  },
  MuiLink: {
    styleOverrides: {
      root: {
        textDecoration: 'none',
      },
    },
  },
  MuiAccordion: {
    styleOverrides: {
      root: {
        padding: '24px',
        borderRadius: '10px !important',
        border: '1px solid',
        backgroundColor: 'var(--mui-palette-background-paper)',
        borderColor: 'var(--mui-palette-divider)',
        boxShadow: 'none',
        '&.Mui-expanded': {
          margin: 0,
        },
      },
    },
  },
  MuiAccordionSummary: {
    styleOverrides: {
      root: {
        margin: 0,
        padding: 0,
        minHeight: '0 !important',
        transition: 'all 0.3s',
        '&.Mui-expanded': {
          minHeight: 0,
          paddingBottom: '8px',
        },
        '&:before': {
          display: 'none',
        },
      },
      content: {
        margin: 0,
        fontSize: 20,
        lineHeight: '28px',
        '&.Mui-expanded': {
          margin: 0,
        },
      },
    },
  },
  MuiAccordionDetails: {
    styleOverrides: {
      root: {
        borderTop: '1px solid',
        borderColor: 'var(--mui-palette-divider)',
        padding: 0,
        paddingTop: '24px',
      },
    },
  },
  MuiFormLabel: {
    styleOverrides: {
      asterisk: ({ theme }) => ({
        color: theme.palette.error.main,
      }),
    },
  },
});

const lightThemeOptions = [
  {
    cssVariables: true,
    typography: {
      fontFamily: 'var(--font-alibaba-puhuiti), PingFang SC, sans-serif',
    },
    palette: lightPalette,
    components: createComponentStyleOverrides(false),
  },
  zhCN,
  CuiZhCN,
];

const darkThemeOptions = [
  {
    cssVariables: true,
    typography: {
      fontFamily: 'var(--font-alibaba-puhuiti), PingFang SC, sans-serif',
    },
    palette: darkPalette,
    components: createComponentStyleOverrides(true),
  },
  zhCN,
  CuiZhCN,
];

const lightTheme = createTheme(...(lightThemeOptions as any));

const darkTheme = createTheme(...(darkThemeOptions as any));

const lightThemeWidget = createTheme(
  // @ts-ignore
  {
    ...lightThemeOptions[0],
    cssVariables: {
      cssVarPrefix: 'widget',
    },
  },
  ...lightThemeOptions.slice(1),
);

const darkThemeWidget = createTheme(
  // @ts-ignore
  {
    ...darkThemeOptions[0],
    cssVariables: {
      cssVarPrefix: 'widget',
    },
  },
  ...darkThemeOptions.slice(1),
);

export {
  darkTheme,
  lightTheme,
  darkThemeOptions,
  lightThemeOptions,
  lightThemeWidget,
  darkThemeWidget,
  createComponentStyleOverrides,
};
