import { ITreeItem } from '@/assets/type';
import { IconXiajiantou, IconWenjianjia, IconWenjian } from '@panda-wiki/icons';
import { useStore } from '@/provider';
import { useBasePath } from '@/hooks';
import { addOpacityToColor } from '@/utils';
import { Ellipsis } from '@ctzhian/ui';
import {
  Box,
  Chip,
  Skeleton,
  Button,
  Stack,
  useTheme,
  IconButton,
} from '@mui/material';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useState } from 'react';

interface CatalogFolderProps {
  item: ITreeItem;
  depth?: number;
  searchView?: boolean;
}

const FolderIcon = ({ item }: { item: ITreeItem }) =>
  item.emoji ? (
    <Box sx={{ flexShrink: 0, fontSize: 12 }}>{item.emoji}</Box>
  ) : (
    <IconWenjianjia sx={{ flexShrink: 0, fontSize: 12 }} />
  );

const CatalogFolder = ({
  item,
  depth = 1,
  searchView = false,
}: CatalogFolderProps) => {
  const [searchExpanded, setSearchExpanded] = useState(true);
  const expanded = searchView ? searchExpanded : item.expanded;
  const theme = useTheme();
  const {
    themeMode = 'light',
    setTree,
    lazyCatalog,
    toggleCatalogFolder,
    loadCatalogChildren,
  } = useStore();
  const params = useParams() || {};
  const activeId = params.id as string;
  const basePath = useBasePath();

  return (
    <Stack key={item.id} gap={0.5}>
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        gap={0.5}
        sx={{
          position: 'relative',
          lineHeight: '40px',
          cursor: 'pointer',
          borderRadius: '10px',
          color: activeId === item.id ? 'primary.main' : 'text.primary',
          bgcolor:
            activeId === item.id
              ? addOpacityToColor(theme.palette.primary.main, 0.08)
              : 'transparent',
          transition: 'all 0.2s ease-in-out',
          '&:hover': {
            color: activeId === item.id ? 'primary.main' : 'text.primary',
            bgcolor:
              activeId === item.id
                ? addOpacityToColor(theme.palette.primary.main, 0.08)
                : themeMode === 'dark'
                  ? '#394052'
                  : 'background.paper3',
          },
        }}
        id={`catalog-item-${item.id}`}
      >
        {item.type === 2 ? (
          <Box sx={{ flex: 1 }}>
            <Link href={`${basePath}/node/${item.id}`} prefetch={false}>
              <Box sx={{ pl: depth * 2, pr: 1 }}>
                <Stack direction='row' alignItems='center' gap={1}>
                  {item.emoji ? (
                    <Box sx={{ flexShrink: 0, fontSize: 14 }}>{item.emoji}</Box>
                  ) : (
                    <IconWenjian sx={{ flexShrink: 0, fontSize: 12 }} />
                  )}
                  <Ellipsis sx={{ flex: 1, width: 0, pr: 1 }}>
                    {item.name}
                  </Ellipsis>
                </Stack>
              </Box>
            </Link>
          </Box>
        ) : (
          <Stack
            direction='row'
            alignItems='center'
            justifyContent={'space-between'}
            sx={{ flex: 1, minWidth: 0, pl: depth * 2, pr: 1 }}
          >
            <Link
              href={item.kind === 'nav' ? '#' : `${basePath}/node/${item.id}`}
              onClick={e => {
                if (item.kind === 'nav') {
                  e.preventDefault();
                  if (searchView) setSearchExpanded(v => !v);
                  else toggleCatalogFolder?.(item);
                }
              }}
              prefetch={false}
              style={{ flex: 1, minWidth: 0, display: 'block' }}
            >
              <Stack
                direction='row'
                alignItems='center'
                gap={1}
                sx={{ flex: 1, minWidth: 0 }}
              >
                <FolderIcon item={item} />
                <Ellipsis sx={{ flex: 1, width: 0, pr: 1 }}>
                  {item.name}
                </Ellipsis>
                {item.pathOnly && (
                  <Chip
                    label='仅定位'
                    size='small'
                    variant='outlined'
                    sx={{ height: 20, fontSize: 11, flexShrink: 0 }}
                  />
                )}
              </Stack>
            </Link>

            {(item.children?.length ||
              (!searchView && (item.hasChildren || !lazyCatalog))) && (
              <IconButton
                size='small'
                sx={{
                  flexShrink: 0,
                  position: 'relative',
                  zIndex: 1,
                  '&:hover': {
                    color: 'primary.main',
                  },
                }}
                onClick={e => {
                  e.stopPropagation();
                  if (searchView) {
                    setSearchExpanded(v => !v);
                    return;
                  }
                  if (lazyCatalog) {
                    toggleCatalogFolder?.(item);
                    return;
                  }
                  if (item.type === 1) {
                    setTree?.(tree =>
                      (tree || []).map(node =>
                        toggleNodeExpanded(node, item.id),
                      ),
                    );
                    return;
                  }
                }}
              >
                <IconXiajiantou
                  sx={{
                    flexShrink: 0,
                    fontSize: 16,
                    transform: expanded ? 'none' : 'rotate(-90deg)',
                    transition: 'transform 0.2s',
                  }}
                />
              </IconButton>
            )}
          </Stack>
        )}
      </Stack>
      {expanded && item.childrenLoading && (
        <Stack sx={{ pl: (depth + 1) * 2 }}>
          <Skeleton width='75%' height={32} />
          <Skeleton width='55%' height={32} />
        </Stack>
      )}
      {expanded && item.childrenError && (
        <Button
          size='small'
          color='error'
          sx={{ ml: (depth + 1) * 2, alignSelf: 'flex-start' }}
          onClick={() => void loadCatalogChildren?.(item)}
        >
          加载失败，点击重试
        </Button>
      )}
      {item.children && item.children.length > 0 && expanded && (
        <Stack gap={0.5}>
          {item.children.map(child => (
            <CatalogFolder
              key={child.id}
              depth={depth + 1}
              item={child}
              searchView={searchView}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
};

const toggleNodeExpanded = (node: ITreeItem, id: string): ITreeItem => {
  if (node.id === id) {
    return { ...node, expanded: !node.expanded };
  }
  if (!node.children?.length) return node;
  return {
    ...node,
    children: node.children.map(child => toggleNodeExpanded(child, id)),
  };
};

export default CatalogFolder;
