'use client';

import { FooterSetting } from '@/assets/type';
import EmptyDocPlaceholder from '@/components/emptyDocPlaceholder';
import { FooterProvider } from '@/components/footer';
import Header from '@/components/header';
import { CONTENT_GAP } from '@/constant';
import { useSyncNavByDocId } from '@/hooks/useSyncNavByDocId';
import { useStore } from '@/provider';
import Catalog from '@/views/node/Catalog';
import CatalogH5 from '@/views/node/CatalogH5';
import NavBar from '@/views/node/NavBar';
import { Box, Stack } from '@mui/material';
import { usePathname } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

const useMounted = () => {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return mounted;
};

const PCLayout = ({ children }: { children: React.ReactNode }) => {
  const { tree, kbDetail, catalogWidth = 260 } = useStore();
  const pathname = usePathname();
  const hasTree = !!tree?.length;
  const isNodeDetailPage = pathname?.includes('/node/');
  const docWidth = useMemo(
    () => kbDetail?.settings?.theme_and_style?.doc_width || 'full',
    [kbDetail],
  );

  return (
    <Stack sx={{ height: '100vh', overflow: 'auto' }} id='scroll-container'>
      <Header isDocPage={true} />
      <NavBar docWidth={docWidth} catalogWidth={catalogWidth} />
      {!hasTree && !isNodeDetailPage ? (
        <EmptyDocPlaceholder />
      ) : (
        <Stack sx={{ flex: 1, px: 5, alignItems: 'center' }}>
          <Stack
            direction='row'
            justifyContent='center'
            alignItems='flex-start'
            gap={`${CONTENT_GAP}px`}
            sx={{
              pt: '50px',
              pb: 10,
              flex: 1,
              width: '100%',
            }}
          >
            {hasTree && <Catalog />}
            {children}
          </Stack>
        </Stack>
      )}

      <FooterProvider isDocPage={true} />
    </Stack>
  );
};

const MobileLayout = ({
  children,
  footerSetting,
}: {
  children?: React.ReactNode;
  footerSetting?: FooterSetting | null;
}) => {
  const { tree } = useStore();
  const pathname = usePathname();
  const hasTree = !!tree?.length;
  const isNodeDetailPage = pathname?.includes('/node/');
  return (
    <Stack
      sx={{
        position: 'relative',
        height: '100vh',
        overflow: 'auto',
        zIndex: 1,
      }}
    >
      <Box sx={{ flex: 1 }}>
        <Header />
        <NavBar />
        {!hasTree && !isNodeDetailPage ? (
          <EmptyDocPlaceholder mobile />
        ) : (
          <>
            {hasTree && <CatalogH5 />}
            {children}
          </>
        )}
      </Box>

      <Box
        sx={{
          mt: 5,
          bgcolor: 'background.paper3',
          ...(footerSetting?.footer_style === 'complex' && {
            borderTop: '1px solid',
            borderColor: 'divider',
          }),
        }}
      >
        <FooterProvider />
      </Box>
    </Stack>
  );
};

export default function NodeClientLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { mobile, kbDetail } = useStore();
  const footerSetting = kbDetail?.settings?.footer_settings;
  const mounted = useMounted();
  useSyncNavByDocId();

  if (!mounted) {
    return (
      <Box
        sx={{
          minHeight: '100vh',
          bgcolor: 'background.paper',
        }}
      />
    );
  }

  return (
    <>
      {mobile ? (
        <MobileLayout footerSetting={footerSetting}>{children}</MobileLayout>
      ) : (
        <PCLayout>{children}</PCLayout>
      )}
    </>
  );
}
