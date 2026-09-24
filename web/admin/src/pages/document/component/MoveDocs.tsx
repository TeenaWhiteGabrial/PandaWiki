import { ITreeItem } from '@/api';
import Card from '@/components/Card';
import DragTree from '@/components/Drag/DragTree';
import { message, Modal } from '@/components/ui';
import {
  getApiV1NodeListGroupNav,
  postApiV1NodeBatchMove,
  postApiV1NodeMoveNav,
} from '@/request/Node';
import { DomainNodeListItemResp } from '@/request/types';
import { useAppSelector } from '@/store';
import { collapseAllFolders, convertToTree } from '@/utils/drag';
import { Box, Stack } from '@mui/material';
import { useEffect, useState } from 'react';

interface DocDeleteProps {
  open: boolean;
  onClose: () => void;
  data: DomainNodeListItemResp[];
  selected: DomainNodeListItemResp[];
  currentNavId?: string | null;
  onMoved?: (payload: { ids: string[]; parentId: string }) => void;
  refresh: () => void;
}

type MoveTarget = {
  id: string;
  navId: string;
  parentId: string;
  kind: 'nav' | 'folder';
};

const getNavVirtualID = (navID: string) => `nav:${navID}`;

const findTreeItem = (items: ITreeItem[], id: string): ITreeItem | null => {
  for (const item of items) {
    if (item.id === id) return item;
    if (item.children?.length) {
      const found = findTreeItem(item.children, id);
      if (found) return found;
    }
  }
  return null;
};

const collectDescendantIDs = (
  items: DomainNodeListItemResp[],
  rootIDs: Set<string>,
) => {
  const result = new Set(rootIDs);
  let changed = true;
  while (changed) {
    changed = false;
    items.forEach(item => {
      if (!item.id || !item.parent_id) return;
      if (result.has(item.parent_id) && !result.has(item.id)) {
        result.add(item.id);
        changed = true;
      }
    });
  }
  return result;
};

const MoveDocs = ({
  open,
  onClose,
  selected,
  currentNavId,
  onMoved,
  refresh,
}: DocDeleteProps) => {
  const { kb_id, nav_id } = useAppSelector(state => state.config);
  const effectiveNavId = currentNavId || nav_id;
  const [tree, setTree] = useState<ITreeItem[]>([]);
  const [target, setTarget] = useState<MoveTarget | null>(null);
  const [treeLoading, setTreeLoading] = useState(false);

  const buildTargetTree = async () => {
    if (!kb_id) return;
    setTreeLoading(true);
    try {
      const groups = await getApiV1NodeListGroupNav({ kb_id });
      const selectedFolderIDs = new Set(
        selected
          .filter(item => item.type === 1 && item.id)
          .map(item => item.id!),
      );
      const allNodes = (groups || []).flatMap(group => group.list || []);
      const excludedIDs = collectDescendantIDs(allNodes, selectedFolderIDs);
      const nextTree = (groups || [])
        .slice()
        .sort((a, b) => (a.position || 0) - (b.position || 0))
        .map(group => {
          const groupNavID = group.nav_id || '';
          const folders = (group.list || []).filter(
            item =>
              item.type === 1 &&
              item.id &&
              !excludedIDs.has(item.id) &&
              item.nav_id === groupNavID,
          );
          const children = collapseAllFolders(
            convertToTree(folders).map(item => ({
              ...item,
              nav_id: item.nav_id || groupNavID,
            })),
            true,
          ) as ITreeItem[];
          return {
            id: getNavVirtualID(groupNavID),
            name: group.nav_name || '未命名目录',
            level: 0,
            order: group.position,
            type: 1 as const,
            kind: 'nav' as const,
            nav_id: groupNavID,
            children,
            canHaveChildren: true,
            collapsed: true,
          };
        });
      setTree(nextTree);
    } finally {
      setTreeLoading(false);
    }
  };

  useEffect(() => {
    if (open) {
      buildTargetTree();
    }
  }, [open, kb_id, selected]);

  const handleOk = () => {
    const ids = selected.filter(it => it.type === 1 && it.id).map(it => it.id!);
    selected
      .filter(it => it.type === 2 && it.id)
      .forEach(it => {
        if (!ids.includes(it.parent_id!)) {
          ids.push(it.id!);
        }
      });

    if (!target) {
      message.error('请选择移动路径');
      return;
    }

    const isSameNav =
      effectiveNavId && target.navId && String(effectiveNavId) === target.navId;

    if (target.kind === 'nav') {
      if (isSameNav) {
        postApiV1NodeBatchMove({ ids, parent_id: '', kb_id }).then(() => {
          message.success('移动成功');
          onClose();
          onMoved?.({ ids, parentId: '' });
          refresh();
        });
        return;
      }

      postApiV1NodeMoveNav({
        ids,
        kb_id,
        nav_id: target.navId,
      }).then(() => {
        message.success('移动成功');
        onClose();
        refresh();
      });
      return;
    }

    postApiV1NodeBatchMove({
      ids,
      parent_id: target.parentId,
      kb_id,
    }).then(() => {
      message.success('移动成功');
      onClose();
      onMoved?.({ ids, parentId: target.parentId });
      refresh();
    });
  };

  useEffect(() => {
    if (!open) {
      setTarget(null);
      setTree([]);
    }
  }, [open]);

  return (
    <Modal
      title='移动文档/文件夹'
      open={open}
      onCancel={onClose}
      onOk={handleOk}
    >
      <Box sx={{ fontSize: 14, color: 'text.secondary', mb: 1 }}>
        已选中
        <Box component={'span'} sx={{ fontWeight: 700, color: 'primary.main' }}>
          {' '}
          {selected.length}{' '}
        </Box>
        个文档/文件夹，移动到
      </Box>
      <Card
        sx={{
          bgcolor: 'background.paper3',
          p: 1.5,
          maxHeight: 260,
          overflowY: 'auto',
        }}
      >
        {treeLoading ? (
          <Box sx={{ fontSize: 14, color: 'text.secondary', p: 1 }}>
            目录加载中...
          </Box>
        ) : (
          <Stack gap={1}>
            {tree.length > 0 ? (
              <DragTree
                ui='select'
                selected={target ? [target.id] : []}
                data={tree}
                readOnly={true}
                relativeSelect={false}
                onSelectChange={(_, id = '') => {
                  if (!id) return;
                  if (target?.id === id) {
                    setTarget(null);
                    return;
                  }
                  const item = findTreeItem(tree, id);
                  if (!item) return;
                  const itemNavId =
                    item.nav_id ||
                    (item.kind === 'nav' ? item.id.replace(/^nav:/, '') : '');
                  setTarget({
                    id: item.id,
                    navId: itemNavId,
                    parentId: item.kind === 'nav' ? '' : item.id,
                    kind: item.kind === 'nav' ? 'nav' : 'folder',
                  });
                }}
              />
            ) : (
              <Box sx={{ fontSize: 14, color: 'text.secondary', p: 1 }}>
                暂无可用目录
              </Box>
            )}
          </Stack>
        )}
      </Card>
    </Modal>
  );
};

export default MoveDocs;
