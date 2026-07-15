import ErrorComponent from '@/components/error';
import StoreProvider from '@/provider';
import { ThemeStoreProvider } from '@/provider/themeStore';
import { getFreshWebAppInfo } from '@/request/appInfo';
import Script from 'next/script';
import { Box } from '@mui/material';
import { AppRouterCacheProvider } from '@mui/material-nextjs/v16-appRouter';
import type { Metadata, Viewport } from 'next';
import localFont from 'next/font/local';
import { headers, cookies } from 'next/headers';
import { getSelectorsByUserAgent } from 'react-device-detect';
import { getBasePath, getImagePath } from '@/utils';
import './globals.css';

const alibabaPuHuiTi = localFont({
  variable: '--font-alibaba-puhuiti',
  src: [
    {
      path: '../assets/fonts/AlibabaPuHuiTi-Bold.ttf',
      weight: '700',
    },
    {
      path: '../assets/fonts/AlibabaPuHuiTi-Regular.ttf',
      weight: '400',
    },
  ],
});

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
};

export async function generateMetadata(): Promise<Metadata> {
  const kbDetail: any = await getFreshWebAppInfo();
  const basePath = getBasePath(kbDetail?.base_url || '');
  const logo = getImagePath(kbDetail?.settings?.icon || '', basePath);
  const favicon = getImagePath(
    kbDetail?.settings?.favicon || kbDetail?.settings?.icon || '',
    basePath,
  );
  return {
    metadataBase: new URL(process.env.TARGET || ''),
    title: kbDetail?.settings?.title || '知识库',
    description: kbDetail?.settings?.desc || '',
    keywords: kbDetail?.settings?.keyword || '',
    icons: {
      icon: favicon || `${basePath}/favicon.png`,
    },
    openGraph: {
      title: kbDetail?.settings?.title || '知识库',
      description: kbDetail?.settings?.desc || '',
      images: logo ? [logo] : [],
    },
  };
}

const Layout = async ({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) => {
  const headersList = await headers();
  const userAgent = headersList.get('user-agent');
  const cookieStore = await cookies();
  const themeMode = (cookieStore.get('theme_mode')?.value || 'light') as
    | 'light'
    | 'dark';

  let error: any = null;

  const kbDetailResolve = await getFreshWebAppInfo()
    .then(value => ({ status: 'fulfilled', value }) as const)
    .catch(reason => ({ status: 'rejected', reason }) as const);
  const kbDetail: any =
    kbDetailResolve.status === 'fulfilled' ? kbDetailResolve.value : undefined;

  if (kbDetailResolve.status === 'rejected') {
    error = kbDetailResolve.reason;
  }

  const { isMobile } = getSelectorsByUserAgent(userAgent || '') || {
    isMobile: false,
  };

  const basePath = getBasePath(kbDetail?.base_url || '');

  return (
    <html lang='en'>
      <Script
        id='base-path'
        dangerouslySetInnerHTML={{
          __html: `window._BASE_PATH_ = '${basePath}';`,
        }}
      />
      <body
        className={`${alibabaPuHuiTi.variable} ${themeMode === 'dark' ? 'dark' : 'light'}`}
      >
        <AppRouterCacheProvider>
          <ThemeStoreProvider themeMode={themeMode}>
            <StoreProvider
              kbDetail={kbDetail}
              themeMode={themeMode || 'light'}
              mobile={isMobile}
            >
              <Box
                sx={{
                  bgcolor: 'background.paper',
                  height: error ? '100vh' : 'auto',
                }}
                id='app-theme-root'
              >
                {error ? <ErrorComponent error={error} /> : children}
              </Box>
            </StoreProvider>
          </ThemeStoreProvider>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
};

export default Layout;
