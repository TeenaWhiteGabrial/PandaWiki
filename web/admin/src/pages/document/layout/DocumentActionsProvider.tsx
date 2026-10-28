import type { ITreeItem } from '@/api';
import { message, Modal } from '@/components/ui';
import type { TreeMenuOptions } from '@/components/Drag/DragTree/TreeMenu';
import { getReaderDocumentUrl } from '@/utils/readerUrl';
import { deleteApiV1NavDelete } from '@/request/Nav';
import { postApiV1NodeRestudy } from '@/request/Node';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import { getApiV1FileDownloadUrl } from '@/request/File';
import {
  getApiV1FileFolderDownload,
  postApiV1FileFolderDownload,
  type FolderDownloadTask,
} from '@/request/BatchImport';
import {
  ConstsCrawlerSource,
  type DomainNodeListItemResp,
  type GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
  type V1NavListResp,
} from '@/request/types';
import type { PermissionAudienceCatalog } from '../utils/permissionTags';
import DocAddByCustomText from '../component/DocAddByCustomText';
import AddDocByType from '../component/AddDocByType';
import BatchZipImport from '../component/BatchZipImport';
import DocDelete from '../component/DocDelete';
import MoveDocs from '../component/MoveDocs';
import DocPropertiesModal from '../component/DocPropertiesModal';
import FileStorageInfo from '../editor/edit/FileStorageInfo';
import NavEditModal from './DocPageNavs/NavEditModal';
import NavPermissionModal from './DocPageNavs/NavPermissionModal';
import { useDocTreeMenu } from './DocPageList/useDocTreeMenu';
import {
  Box,
  Button,
  LinearProgress,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';

interface Props {
  children: ReactNode;
  kbId: string;
  viewerId: string;
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  readOnly: boolean;
  wikiUrl: string;
  refresh: () => void;
  onPublish: (ids?: string[]) => void;
  onNodesDeleted: (ids: string[]) => void;
  onNavDeleted: (id: string) => void;
  onViewDocument: (item: DomainNodeListItemResp) => void;
}

type Target = { parentId: string | null; navId: string; name: string };
type CreateTarget = Target & { type: 1 | 2; contentType: string };
type RenameTarget = DomainNodeListItemResp & { meta: { emoji?: string } };

const ActionsContext = createContext<ReturnType<typeof useActions> | null>(
  null,
);
export const useDocumentActions = () => {
  const actions = useContext(ActionsContext);
  if (!actions) throw new Error('DocumentActionsProvider is required');
  return actions;
};

function useActions({
  kbId: kb_id,
  viewerId,
  groups,
  readOnly,
  wikiUrl,
  refresh: reload,
  onPublish,
  onNodesDeleted,
  onNavDeleted,
  onViewDocument,
}: Omit<Props, 'children'>) {
  const [mutationVersion, setMutationVersion] = useState(0);
  const refresh = () => {
    setMutationVersion(version => version + 1);
    reload();
  };
  const [permissionCatalog, setPermissionCatalog] =
    useState<PermissionAudienceCatalog>({
      departments: {},
      levels: {},
      users: {},
    });
  const [createTarget, setCreateTarget] = useState<CreateTarget | null>(null);
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null);
  const [importTarget, setImportTarget] = useState<Target | null>(null);
  const [zipTarget, setZipTarget] = useState<Target | null>(null);
  const [deleteItems, setDeleteItems] = useState<DomainNodeListItemResp[]>([]);
  const [moveItems, setMoveItems] = useState<DomainNodeListItemResp[]>([]);
  const [propertyItems, setPropertyItems] = useState<DomainNodeListItemResp[]>(
    [],
  );
  const [navEditor, setNavEditor] = useState<{
    nav: V1NavListResp | null;
  } | null>(null);
  const [permissionNav, setPermissionNav] = useState<V1NavListResp | null>(
    null,
  );
  const [deletingNav, setDeletingNav] = useState<V1NavListResp | null>(null);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('');
  const [folderDownloadTask, setFolderDownloadTask] =
    useState<FolderDownloadTask | null>(null);
  const [folderDownloadOpen, setFolderDownloadOpen] = useState(false);
  const [folderDownloadURL, setFolderDownloadURL] = useState('');
  const [storageInfoOpen, setStorageInfoOpen] = useState(false);
  const [storageInfoFileKey, setStorageInfoFileKey] = useState('');

  useEffect(() => {
    let active = true;
    setPermissionCatalog({ departments: {}, levels: {}, users: {} });
    if (kb_id)
      getNodePermissionOptions(kb_id)
        .then(options => {
          if (!active) return;
          setPermissionCatalog({
            departments: Object.fromEntries(
              [
                ...(options.departments || []),
                ...(options.management_departments || []),
              ].map(item => [item.id, item.name]),
            ),
            levels: Object.fromEntries(
              (options.levels || []).map(item => [item.id, item.name]),
            ),
            users: Object.fromEntries(
              [
                ...(options.users || []),
                ...(options.management_users || []),
              ].map(item => [item.id, item.name || item.account]),
            ),
          });
        })
        .catch(() => {
          /* Fall back to generic permission labels. */
        });
    return () => {
      active = false;
    };
  }, [kb_id, viewerId]);

  const canManage = (item: { can_manage?: boolean }) =>
    !readOnly && item.can_manage !== false;
  const targetOf = (item: ITreeItem): Target => ({
    parentId: item.kind === 'nav' ? null : item.id,
    navId:
      item.nav_id || (item.kind === 'nav' ? item.id.replace(/^nav:/, '') : ''),
    name: item.name || '根目录',
  });
  const dataOf = (item: ITreeItem): DomainNodeListItemResp =>
    groups
      .flatMap(group => group.list || [])
      .find(node => node.id === item.id) || item;
  const openCreate = (item: ITreeItem, type: 1 | 2, contentType = 'html') => {
    if (
      !canManage(item) ||
      (type === 1
        ? item.can_create_folder === false
        : item.can_create_document === false)
    )
      return;
    setCreateTarget({ ...targetOf(item), type, contentType });
  };
  const openImport = (item: ITreeItem) => {
    if (canManage(item) && item.can_create_document !== false)
      setImportTarget(targetOf(item));
  };
  const openZipImport = (item: ITreeItem) => {
    if (canManage(item) && item.can_create_folder !== false)
      setZipTarget(targetOf(item));
  };
  const openMove = (items: DomainNodeListItemResp[]) => {
    if (items.length && items.every(canManage)) setMoveItems(items);
  };
  const openDelete = (items: DomainNodeListItemResp[]) => {
    if (items.length && items.every(canManage)) setDeleteItems(items);
  };
  const openAddNav = () => {
    if (
      !readOnly &&
      (!groups.length || groups.some(group => group.can_configure !== false))
    )
      setNavEditor({ nav: null });
  };
  const handleFolderDownload = useCallback(
    async (item: ITreeItem) => {
      if (!kb_id || !item.id) return;
      const popup = window.open('about:blank', '_blank');
      try {
        setFolderDownloadURL('');
        const task = await postApiV1FileFolderDownload({
          kb_id,
          ...(item.kind === 'nav'
            ? { nav_id: item.nav_id || item.id }
            : { node_id: item.id }),
        });
        setFolderDownloadTask(task);
        setFolderDownloadOpen(true);
        message.info('文件夹压缩任务已创建，可继续进行其他操作');
        let attempts = 0;
        const poll = async () => {
          attempts += 1;
          const result = await getApiV1FileFolderDownload(task.id);
          setFolderDownloadTask(result.task);
          if (result.task.status === 'completed' && result.download_url) {
            setFolderDownloadURL(result.download_url);
            if (popup) popup.location.href = result.download_url;
            message.success('文件夹压缩包已生成');
            return;
          }
          if (
            result.task.status === 'failed' ||
            result.task.status === 'expired'
          ) {
            popup?.close();
            message.error(result.task.error_message || '文件夹下载任务失败');
            return;
          }
          if (attempts < 720) window.setTimeout(poll, 5000);
          else popup?.close();
        };
        window.setTimeout(poll, 1000);
      } catch (error) {
        popup?.close();
        message.error(
          error instanceof Error ? error.message : '创建下载任务失败',
        );
      }
    },
    [kb_id],
  );

  const viewDocument = (item: ITreeItem) => {
    if (item.placeholder || item.type !== 2) return;
    onViewDocument(dataOf(item));
  };

  const baseMenu = useDocTreeMenu(
    {
      handleUrl: item => openImport(item),
      handleBatchImport: openZipImport,
      handleFolderDownload,
      handleDownloadFile: async item => {
        if (!item.file_key) return;
        const { url } = await getApiV1FileDownloadUrl({
          key: item.file_key,
          attachment: true,
          filename: item.file_name || item.name,
        });
        window.location.assign(url);
      },
      handleFileStorageInfo: item => {
        if (!item.file_key) return;
        setStorageInfoFileKey(item.file_key);
        setStorageInfoOpen(true);
      },
      handleMove: item => openMove([dataOf(item)]),
      handleView: viewDocument,
      handleDelete: item => openDelete([dataOf(item)]),
      handleProperties: item => setPropertyItems([dataOf(item)]),
      handlePublish: item => onPublish([item.id]),
      handleEdit: item => window.open(`/doc/editor/${item.id}`, '_blank'),
      handleRestudy: item => {
        if (!kb_id) return;
        postApiV1NodeRestudy({
          kb_id,
          node_ids: [item.id],
          priority: 100,
        }).then(() => {
          message.success('正在重建问答索引');
          refresh();
        });
      },
      handleFrontDoc: id => {
        const item = groups
          .flatMap(group => group.list || [])
          .find(node => node.id === id);
        if (item?.status !== 2 && !item?.publisher_id) {
          message.warning('当前文档未发布，无法查看用户端文档');
          return;
        }
        const url = getReaderDocumentUrl(wikiUrl, id);
        if (url) window.open(url, '_blank');
        else
          message.warning(
            '未配置有效的用户端地址，请在“设置 → 门户网站 → 网站基本信息”中设置“网址绝对路径前缀”',
          );
      },
    },
    readOnly,
  );

  // Both renderers use these exact actions and dialogs. View callbacks such as
  // inline tree creation must not override the document management workflow.
  const menu = (options: TreeMenuOptions) => {
    const { item } = options;
    if (!canManage(item) || item.placeholder) return [];
    const actions = baseMenu({
      ...options,
      createItem: (type, contentType) => openCreate(item, type, contentType),
      renameItem: () =>
        setRenameTarget({ ...dataOf(item), meta: { emoji: item.emoji } }),
    });
    if (item.kind !== 'nav') return actions;
    const group = groups.find(
      value => value.nav_id === item.nav_id || value.nav_id === item.id,
    );
    if (!group) return [];
    // The permission dialog needs the server's capability flags as well as
    // identity; dropping can_configure makes even a super-admin read-only.
    const nav: V1NavListResp = {
      id: group.nav_id,
      name: group.nav_name,
      permissions: group.permissions,
      can_manage: group.can_manage,
      can_configure: group.can_configure,
      can_create_document: group.can_create_document,
      can_create_folder: group.can_create_folder,
    };
    return [
      ...actions.filter(
        action =>
          !['move', 'rename', 'properties', 'delete'].includes(action.key),
      ),
      ...(group.can_configure !== false
        ? [
            {
              key: 'edit',
              label: '修改目录',
              onClick: () => setNavEditor({ nav }),
            },
            {
              key: 'permission',
              label: '目录权限',
              onClick: () => setPermissionNav(nav),
            },
            ...(groups.length > 1
              ? [
                  {
                    key: 'delete',
                    label: '删除目录',
                    color: 'error' as const,
                    onClick: () => {
                      setDeleteConfirmInput('');
                      setDeletingNav(nav);
                    },
                  },
                ]
              : []),
          ]
        : []),
    ];
  };

  const closeDeleteNav = () => {
    setDeletingNav(null);
    setDeleteConfirmInput('');
  };
  const confirmDeleteNav = () => {
    if (
      !deletingNav?.id ||
      !kb_id ||
      deleteConfirmInput !== (deletingNav.name || '未命名')
    )
      return;
    const id = deletingNav.id;
    deleteApiV1NavDelete({ id, kb_id }).then(() => {
      message.success('删除成功');
      closeDeleteNav();
      onNavDeleted(id);
      refresh();
    });
  };

  const dialogs = (
    <>
      <DocAddByCustomText
        open={!!createTarget || !!renameTarget}
        data={renameTarget || undefined}
        type={createTarget?.type || (renameTarget?.type as 1 | 2) || 2}
        contentType={createTarget?.contentType}
        parentId={createTarget?.parentId || undefined}
        navId={createTarget?.navId}
        autoJump={false}
        refresh={refresh}
        onClose={() => {
          setCreateTarget(null);
          setRenameTarget(null);
        }}
      />
      <AddDocByType
        type={ConstsCrawlerSource.CrawlerSourceFile}
        open={!!importTarget}
        parentId={importTarget?.parentId || null}
        navId={importTarget?.navId}
        refresh={refresh}
        onCancel={() => setImportTarget(null)}
      />
      <BatchZipImport
        open={!!zipTarget}
        parentId={zipTarget?.parentId || null}
        parentName={zipTarget?.name}
        navId={zipTarget?.navId}
        refresh={refresh}
        onClose={() => setZipTarget(null)}
      />
      <DocDelete
        open={deleteItems.length > 0}
        data={deleteItems}
        onClose={() => setDeleteItems([])}
        onDeleted={ids => {
          onNodesDeleted(ids);
          refresh();
        }}
      />
      <MoveDocs
        open={moveItems.length > 0}
        data={groups.flatMap(group => group.list || [])}
        selected={moveItems}
        currentNavId={moveItems[0]?.nav_id}
        refresh={refresh}
        onClose={() => setMoveItems([])}
      />
      <DocPropertiesModal
        open={propertyItems.length > 0}
        data={propertyItems}
        isBatch={propertyItems.length > 1}
        onCancel={() => setPropertyItems([])}
        onOk={() => {
          setPropertyItems([]);
          refresh();
        }}
      />
      <NavEditModal
        open={!!navEditor}
        nav={navEditor?.nav || null}
        kb_id={kb_id}
        onClose={() => setNavEditor(null)}
        onSuccess={() => {
          setNavEditor(null);
          refresh();
        }}
      />
      <NavPermissionModal
        open={!!permissionNav}
        nav={permissionNav}
        kb_id={kb_id}
        onClose={() => setPermissionNav(null)}
        onSuccess={() => {
          setPermissionNav(null);
          refresh();
        }}
      />
      <Modal
        title='文件夹下载'
        open={folderDownloadOpen && !!folderDownloadTask}
        width={460}
        footer={null}
        onCancel={() => {
          setFolderDownloadOpen(false);
          setFolderDownloadURL('');
        }}
      >
        {folderDownloadTask && (
          <Stack gap={1.5}>
            <Box sx={{ color: 'text.secondary', fontSize: 14 }}>
              {folderDownloadTask.status === 'completed'
                ? '压缩包已生成，可点击下方按钮下载。'
                : folderDownloadTask.status === 'failed'
                  ? folderDownloadTask.error_message || '压缩任务失败'
                  : '正在后台生成压缩包，可以关闭此窗口继续操作。'}
            </Box>
            {folderDownloadTask.status === 'waiting' ||
            folderDownloadTask.status === 'processing' ? (
              <>
                <LinearProgress
                  variant='determinate'
                  value={
                    folderDownloadTask.total_files > 0
                      ? (folderDownloadTask.processed_files /
                          folderDownloadTask.total_files) *
                        100
                      : 0
                  }
                />
                <Typography variant='body2' color='text.secondary'>
                  已处理 {folderDownloadTask.processed_files} /{' '}
                  {folderDownloadTask.total_files} 个文件
                </Typography>
              </>
            ) : null}
            {folderDownloadTask.status === 'completed' && folderDownloadURL ? (
              <Button
                component='a'
                href={folderDownloadURL}
                target='_blank'
                rel='noreferrer'
                variant='contained'
              >
                下载 ZIP
              </Button>
            ) : null}
          </Stack>
        )}
      </Modal>
      <Modal
        title='文件存储地址'
        open={storageInfoOpen}
        width={720}
        footer={null}
        onCancel={() => {
          setStorageInfoOpen(false);
          setStorageInfoFileKey('');
        }}
      >
        <FileStorageInfo fileKey={storageInfoFileKey} />
      </Modal>

      <Modal
        title='确认删除目录？'
        open={!!deletingNav}
        width={480}
        okText='确认删除'
        okButtonProps={{
          color: 'error',
          disabled: deleteConfirmInput !== (deletingNav?.name || '未命名'),
        }}
        onCancel={closeDeleteNav}
        onOk={confirmDeleteNav}
      >
        <Box sx={{ fontSize: 14, color: 'text.secondary', mb: 2 }}>
          删除目录「{deletingNav?.name || '未命名'}
          」将同步删除目录下所有文档，且不可恢复。请输入目录名称确认。
        </Box>
        <TextField
          fullWidth
          size='small'
          value={deleteConfirmInput}
          placeholder='请输入当前目录名称'
          onChange={event => setDeleteConfirmInput(event.target.value)}
        />
      </Modal>
    </>
  );

  return {
    menu,
    viewDocument,
    mutationVersion,
    permissionCatalog,
    openCreate,
    openImport,
    openZipImport,
    openMove,
    openDelete,
    openAddNav,
    dialogs,
  };
}

export default function DocumentActionsProvider({ children, ...props }: Props) {
  const actions = useActions(props);
  return (
    <ActionsContext.Provider value={actions}>
      {children}
      {actions.dialogs}
    </ActionsContext.Provider>
  );
}
