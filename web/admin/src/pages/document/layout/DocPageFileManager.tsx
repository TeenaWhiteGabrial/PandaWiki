import AddDocBtn from '@/pages/document/component/AddDocBtn';
import BatchZipImport from '@/pages/document/component/BatchZipImport';
import DocAddByCustomText from '@/pages/document/component/DocAddByCustomText';
import type { ITreeItem } from '@/api';
import TreeMenu from '@/components/Drag/DragTree/TreeMenu';
import type { DocTreeMenuFn } from './DocPageList/types';
import { putApiV1NodeDetail } from '@/request/Node';
import { useAppSelector } from '@/store';
import type {
  DomainNodeListItemResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import {
  Box,
  Breadcrumbs,
  Button,
  Card,
  Chip,
  Divider,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import DocumentTypeIcon from '@/components/DocumentTypeIcon';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import NavigateNextIcon from '@mui/icons-material/NavigateNext';
import { IconGengduo, IconJiahao } from '@panda-wiki/icons';
import { useEffect, useMemo, useState } from 'react';

interface DocPageFileManagerProps {
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  navId?: string;
  folderId: string;
  onFolderChange: (id: string) => void;
  onDocumentSelect: (item: DomainNodeListItemResp) => void;
  refresh: () => void;
  readOnlyUser?: boolean;
  fill?: boolean;
  menu?: DocTreeMenuFn;
}

type ExplorerItem = DomainNodeListItemResp & {
  explorerKind?: 'nav';
};

const NAV_PATH_PREFIX = 'nav:';

const DocPageFileManager = ({
  groups,
  navId,
  folderId,
  onFolderChange,
  onDocumentSelect,
  refresh,
  readOnlyUser = false,
  fill = false,
  menu,
}: DocPageFileManagerProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [batchImportParentId, setBatchImportParentId] = useState<string | null>(
    null,
  );
  const [batchImportOpen, setBatchImportOpen] = useState(false);
  const [createType, setCreateType] = useState<1 | 2 | null>(null);
  const [createParentId, setCreateParentId] = useState<string | null>(null);

  useEffect(() => {
    setSelectedId(null);
  }, [folderId, navId]);

  const nodes = useMemo(
    () =>
      groups.flatMap(group => (group.list || []) as DomainNodeListItemResp[]),
    [groups],
  );

  const nodeMap = useMemo(
    () => new Map(nodes.map(item => [item.id || '', item])),
    [nodes],
  );

  const isNavPath = folderId.startsWith(NAV_PATH_PREFIX);
  const activeNavId = useMemo(() => {
    if (isNavPath) return folderId.slice(NAV_PATH_PREFIX.length);
    if (folderId) return nodeMap.get(folderId)?.nav_id || navId;
    return navId;
  }, [folderId, isNavPath, navId, nodeMap]);

  const activeNodes = useMemo(
    () => nodes.filter(item => item.nav_id === activeNavId),
    [activeNavId, nodes],
  );

  const rootName = useMemo(() => '根目录', []);

  const currentItems = useMemo((): ExplorerItem[] => {
    if (!folderId) {
      return groups
        .map(group => ({
          id: `${NAV_PATH_PREFIX}${group.nav_id || ''}`,
          name: group.nav_name || '未命名目录',
          nav_id: group.nav_id,
          parent_id: '',
          type: 1,
          kind: 'nav' as const,
          explorerKind: 'nav' as const,
        }))
        .filter(item => item.nav_id);
    }

    return activeNodes
      .filter(item =>
        isNavPath
          ? !(item.parent_id || '') || item.parent_id === activeNavId
          : (item.parent_id || '') === folderId,
      )
      .sort((a, b) => {
        if (a.type !== b.type) return a.type === 1 ? -1 : 1;
        return (a.name || '').localeCompare(b.name || '', 'zh-CN');
      });
  }, [activeNavId, activeNodes, folderId, groups, isNavPath]);

  const breadcrumbs = useMemo(() => {
    const result: ExplorerItem[] = [];
    // 资源管理器根层展示所有目录，此时不能把当前选中的 nav_id
    // 自动追加到面包屑，否则会造成“根目录 > Local Wiki”的假层级。
    if (!folderId) return result;

    if (isNavPath) {
      const group = groups.find(item => item.nav_id === activeNavId);
      if (group?.nav_id) {
        result.push({
          id: `${NAV_PATH_PREFIX}${group.nav_id}`,
          name: group.nav_name || '未命名目录',
          nav_id: group.nav_id,
          type: 1,
          explorerKind: 'nav',
        });
      }
      return result;
    }

    let current = folderId ? nodeMap.get(folderId) : undefined;
    while (current) {
      result.unshift(current);
      current = current.parent_id ? nodeMap.get(current.parent_id) : undefined;
    }
    const breadcrumbNavId = result[0]?.nav_id || activeNavId;
    const group = groups.find(item => item.nav_id === breadcrumbNavId);
    if (group?.nav_id) {
      result.unshift({
        id: `${NAV_PATH_PREFIX}${group.nav_id}`,
        name: group.nav_name || '未命名目录',
        nav_id: group.nav_id,
        type: 1,
        explorerKind: 'nav',
      });
    }
    return result;
  }, [activeNavId, folderId, groups, isNavPath, nodeMap]);

  const currentParentName = useMemo(() => {
    if (!folderId) return rootName;
    if (isNavPath) {
      return (
        groups.find(item => item.nav_id === activeNavId)?.nav_name || rootName
      );
    }
    return breadcrumbs[breadcrumbs.length - 1]?.name || rootName;
  }, [activeNavId, breadcrumbs, folderId, groups, isNavPath, rootName]);

  // 一级文件夹的 parent_id 可能记录为当前目录 nav_id；在资源管理器中，
  // 这个值应当视为虚拟根目录，而不是一个可继续进入的文件夹。
  const parentFolderId = useMemo(() => {
    if (isNavPath) return '';
    const current = nodeMap.get(folderId);
    if (!current) return '';
    if (current.parent_id && current.parent_id !== current.nav_id) {
      return current.parent_id;
    }
    return current.nav_id ? `${NAV_PATH_PREFIX}${current.nav_id}` : '';
  }, [folderId, isNavPath, nodeMap]);

  return (
    <Card
      sx={{
        flex: fill ? 1 : '0 0 auto',
        width: fill ? '100%' : undefined,
        minWidth: 0,
        height: '100%',
        overflow: 'hidden',
      }}
    >
      <Stack sx={{ height: '100%' }}>
        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
          sx={{ px: 2, py: 1.25 }}
        >
          <Stack direction='row' alignItems='center' gap={1} minWidth={0}>
            <Tooltip title='返回上一级'>
              <span>
                <IconButton
                  size='small'
                  disabled={!folderId}
                  onClick={() => onFolderChange(parentFolderId)}
                >
                  <ArrowBackIcon fontSize='small' />
                </IconButton>
              </span>
            </Tooltip>
            <Breadcrumbs
              separator={<NavigateNextIcon fontSize='small' />}
              sx={{ minWidth: 0 }}
            >
              <Typography
                variant='body2'
                noWrap
                sx={{ cursor: 'pointer', fontWeight: folderId ? 400 : 600 }}
                onClick={() => onFolderChange('')}
              >
                <HomeOutlinedIcon
                  sx={{ fontSize: 17, verticalAlign: 'middle', mr: 0.5 }}
                />
                {rootName}
              </Typography>
              {breadcrumbs.map(item => (
                <Typography
                  key={item.id}
                  variant='body2'
                  noWrap
                  sx={{
                    cursor: 'pointer',
                    fontWeight: item.id === folderId ? 600 : 400,
                    maxWidth: 180,
                  }}
                  onClick={() => onFolderChange(item.id || '')}
                >
                  {item.name}
                </Typography>
              ))}
            </Breadcrumbs>
          </Stack>
          {!readOnlyUser && (
            <AddDocBtn
              parentId={isNavPath ? null : folderId || null}
              parentName={currentParentName}
              refresh={refresh}
              disabled={false}
            />
          )}
        </Stack>
        <Divider />
        <Stack
          direction='row'
          alignItems='center'
          sx={{ px: 1.5, py: 0.75, color: 'text.secondary', fontSize: 12 }}
        >
          <Box sx={{ flex: 1 }}>名称</Box>
          <Box sx={{ width: 88, textAlign: 'right' }}>状态</Box>
        </Stack>
        <Box sx={{ flex: 1, overflow: 'auto', p: 1.5 }}>
          {currentItems.length === 0 ? (
            <Stack
              alignItems='center'
              justifyContent='center'
              sx={{ height: '100%', color: 'text.secondary' }}
            >
              <FolderOutlinedIcon sx={{ fontSize: 48, opacity: 0.35 }} />
              <Typography variant='body2'>此目录为空</Typography>
            </Stack>
          ) : (
            <Stack gap={0.5}>
              {currentItems.map(item => {
                const isFolder = item.type === 1;
                const treeItem: ITreeItem = {
                  ...(item as unknown as ITreeItem),
                  id: item.id || '',
                  name: item.name || '',
                  type: isFolder ? 1 : 2,
                  parentId: item.parent_id,
                  kind: item.explorerKind ? 'nav' : undefined,
                };
                const actionMenu = menu
                  ? menu({
                      item: treeItem,
                      createItem: type => {
                        setCreateParentId(
                          item.explorerKind ? null : item.id || null,
                        );
                        setCreateType(type);
                      },
                      renameItem: () => {
                        setEditingId(item.id || null);
                        setEditingName(item.name || '');
                      },
                      isEditing: editingId === item.id,
                      removeItem: () => undefined,
                    }).map(action => {
                      if (action.key === 'batch-import-zip') {
                        return {
                          ...action,
                          onClick: () => {
                            setBatchImportParentId(
                              item.explorerKind ? null : item.id || null,
                            );
                            setBatchImportOpen(true);
                          },
                        };
                      }
                      if (action.key === 'folder') {
                        return {
                          ...action,
                          onClick: () => {
                            setCreateParentId(
                              item.explorerKind ? null : item.id || null,
                            );
                            setCreateType(1);
                          },
                        };
                      }
                      if (action.key === 'doc') {
                        return {
                          ...action,
                          children: action.children?.map(child => ({
                            ...child,
                            onClick: () => {
                              setCreateParentId(
                                item.explorerKind ? null : item.id || null,
                              );
                              setCreateType(2);
                            },
                          })),
                        };
                      }
                      return action;
                    })
                  : [];
                return (
                  <Stack
                    key={item.id}
                    direction='row'
                    alignItems='center'
                    gap={1.5}
                    sx={{
                      px: 1.5,
                      py: 1.25,
                      borderRadius: 1,
                      bgcolor:
                        selectedId === item.id
                          ? 'action.selected'
                          : 'transparent',
                      cursor: 'pointer',
                      '&:hover': { bgcolor: 'action.hover' },
                    }}
                    onClick={() => setSelectedId(item.id || null)}
                    onDoubleClick={() => {
                      setSelectedId(item.id || null);
                      if (isFolder) onFolderChange(item.id || '');
                      else onDocumentSelect(item);
                    }}
                  >
                    {isFolder ? (
                      <FolderOutlinedIcon
                        sx={{ color: 'warning.main', fontSize: 28 }}
                      />
                    ) : (
                      <DocumentTypeIcon
                        contentType={item.content_type}
                        fileName={item.name}
                      />
                    )}
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      {editingId === item.id ? (
                        <Stack direction='row' alignItems='center' gap={1}>
                          <TextField
                            size='small'
                            value={editingName}
                            autoFocus
                            onChange={event =>
                              setEditingName(event.target.value)
                            }
                            onClick={event => event.stopPropagation()}
                            onKeyDown={event => {
                              if (event.key === 'Enter') {
                                event.preventDefault();
                                if (!editingName.trim() || !kb_id || !item.id)
                                  return;
                                putApiV1NodeDetail({
                                  id: item.id,
                                  kb_id,
                                  nav_id: item.nav_id || navId || '',
                                  name: editingName.trim(),
                                }).then(() => {
                                  setEditingId(null);
                                  refresh();
                                });
                              }
                              if (event.key === 'Escape') setEditingId(null);
                            }}
                            sx={{ flex: 1 }}
                          />
                          <Button
                            size='small'
                            onClick={event => {
                              event.stopPropagation();
                              if (!editingName.trim() || !kb_id || !item.id)
                                return;
                              putApiV1NodeDetail({
                                id: item.id,
                                kb_id,
                                nav_id: item.nav_id || navId || '',
                                name: editingName.trim(),
                              }).then(() => {
                                setEditingId(null);
                                refresh();
                              });
                            }}
                          >
                            保存
                          </Button>
                        </Stack>
                      ) : (
                        <Typography variant='body2' noWrap title={item.name}>
                          {item.name || '未命名'}
                        </Typography>
                      )}
                    </Box>
                    {isFolder && !item.explorerKind && !readOnlyUser && (
                      <AddDocBtn
                        parentId={item.id}
                        refresh={refresh}
                        exportFile={false}
                        batchImport={false}
                        context={
                          <IconButton
                            size='small'
                            onClick={event => event.stopPropagation()}
                            title='新建'
                          >
                            <IconJiahao sx={{ fontSize: 18 }} />
                          </IconButton>
                        }
                      />
                    )}
                    {actionMenu.length > 0 && (
                      <Box onClick={event => event.stopPropagation()}>
                        <TreeMenu
                          menu={actionMenu}
                          context={
                            <IconButton size='small' title='更多'>
                              <IconGengduo sx={{ fontSize: 18 }} />
                            </IconButton>
                          }
                        />
                      </Box>
                    )}
                    {!isFolder && (
                      <Chip
                        size='small'
                        variant='outlined'
                        label={item.status === 2 ? '已发布' : '未发布'}
                        color={item.status === 2 ? 'success' : 'default'}
                      />
                    )}
                  </Stack>
                );
              })}
            </Stack>
          )}
        </Box>
      </Stack>
      <BatchZipImport
        open={batchImportOpen}
        parentId={batchImportParentId}
        refresh={refresh}
        onClose={() => {
          setBatchImportOpen(false);
          setBatchImportParentId(null);
        }}
      />
      <DocAddByCustomText
        type={createType || 2}
        open={createType !== null}
        parentId={createParentId || undefined}
        refresh={refresh}
        onClose={() => {
          setCreateType(null);
          setCreateParentId(null);
        }}
      />
    </Card>
  );
};

export default DocPageFileManager;
