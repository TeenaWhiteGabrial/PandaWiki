'use client';

import { IconNav } from '@/components/icons';
import type { ITreeItem } from '@/assets/type';
import { useStore } from '@/provider';
import { filterTreeBySearch } from '@/utils';
import { addExpandState } from '@/utils/tree';
import SearchIcon from '@mui/icons-material/Search';
import { Box, Button, Skeleton, Stack, TextField } from '@mui/material';
import { IconXiajiantou } from '@panda-wiki/icons';
import { useDebounce } from 'ahooks';
import { useParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import CatalogFolder from './CatalogFolder';

const CatalogH5 = () => {
  const [open, setOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const params = useParams() || {};
  const id = params.id as string;
  const {
    tree: initialTree,
    kbDetail,
    nodeList,
    searchCatalog,
    lazyCatalog,
    authRevision,
    authInfo,
  } = useStore();
  const [searchTree, setSearchTree] = useState<ITreeItem[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchError, setSearchError] = useState(false);
  const [retry, setRetry] = useState(0);
  const debouncedSearchTerm = useDebounce(searchTerm, { wait: 300 });

  const catalogSetting = kbDetail?.settings?.catalog_settings;
  const catalogFolderExpand = catalogSetting?.catalog_folder !== 2;

  useEffect(() => {
    let disposed = false;
    setSearchTree([]);
    setSearchError(false);
    if (!debouncedSearchTerm || !searchCatalog) {
      setSearchLoading(false);
      return;
    }
    setSearchLoading(true);
    searchCatalog(debouncedSearchTerm)
      .then(result => {
        if (!disposed) setSearchTree(result);
      })
      .catch(() => {
        if (!disposed) setSearchError(true);
      })
      .finally(() => {
        if (!disposed) setSearchLoading(false);
      });
    return () => {
      disposed = true;
    };
  }, [debouncedSearchTerm, searchCatalog, authRevision, authInfo?.id, retry]);
  const tree = useMemo(() => {
    if (lazyCatalog)
      return debouncedSearchTerm ? searchTree : initialTree || [];
    const { tree: originalTree } = addExpandState(
      initialTree || [],
      id as string,
      catalogFolderExpand,
    );
    return filterTreeBySearch(originalTree, debouncedSearchTerm);
  }, [
    initialTree,
    id,
    catalogFolderExpand,
    debouncedSearchTerm,
    lazyCatalog,
    searchTree,
  ]);

  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [open]);

  useEffect(() => {
    if (id) {
      setOpen(false);
    }
  }, [id]);

  return (
    <Box
      sx={{
        position: 'sticky',
        top: nodeList?.length && nodeList?.length > 1 ? '108px' : '64px',
        width: '100%',
        zIndex: 2,
        bgcolor: 'background.paper3',
      }}
    >
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{
          py: 3,
          px: 3,
          position: 'relative',
          zIndex: 2,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
        onClick={() => setOpen(!open)}
      >
        <Stack direction='row' alignItems='center' gap={1}>
          <IconNav sx={{ fontSize: 18 }} />
          <Box
            sx={{
              fontSize: 20,
              fontWeight: 'bold',
              color: 'text.primary',
            }}
          >
            目录
          </Box>
        </Stack>
        <IconXiajiantou
          sx={{
            fontSize: 24,
            transform: open ? 'rotate(-180deg)' : 'rotate(0deg)',
            transition: 'transform 0.3s ease-in-out',
            cursor: 'pointer',
          }}
        />
      </Stack>
      <Box
        sx={{
          '--fallback-height': 'calc(100vh - 131px)',
          '--dynamic-height': 'calc(100dvh - 131px)',
          px: 3,
          height: open
            ? 'var(--dynamic-height, var(--fallback-height))'
            : '0px',
          paddingBottom: 'env(safe-area-inset-bottom)',
          transition: 'height 0.3s ease-in-out',
          bgcolor: 'background.paper3',
          overflowY: 'auto',
          overflowX: 'hidden',
          '&::-webkit-scrollbar': {
            display: 'none',
          },
          msOverflowStyle: 'none',
          scrollbarWidth: 'none',
        }}
      >
        <TextField
          slotProps={{
            input: {
              endAdornment: <SearchIcon sx={{ fontSize: 20 }} />,
            },
          }}
          size='small'
          placeholder='搜索'
          value={searchTerm}
          onChange={e => setSearchTerm(e.target.value)}
          sx={{
            width: '100%',
            mt: 2,
            mb: 1,
            '& .MuiOutlinedInput-root': {
              height: 36,
              fontSize: 14,
              pr: '18px',
              '& fieldset': {
                borderRadius: '10px',
                borderColor: 'divider',
                px: 2,
              },
            },
          }}
        />
        <Box sx={{ py: 3 }}>
          {searchLoading && (
            <>
              <Skeleton height={36} />
              <Skeleton height={36} />
            </>
          )}
          {searchError && (
            <Button onClick={() => setRetry(n => n + 1)}>
              搜索失败，点击重试
            </Button>
          )}
          {!searchLoading &&
            !searchError &&
            debouncedSearchTerm &&
            tree.length === 0 && (
              <Box sx={{ color: 'text.secondary', p: 2 }}>无搜索结果</Box>
            )}
          {tree.map(item => (
            <CatalogFolder
              key={`${debouncedSearchTerm}:${item.id}`}
              item={item}
              searchView={!!debouncedSearchTerm}
            />
          ))}
        </Box>
      </Box>
    </Box>
  );
};

export default CatalogH5;
