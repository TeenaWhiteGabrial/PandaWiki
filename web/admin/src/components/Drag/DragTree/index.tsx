import { ITreeItem } from '@/api';
import {
  SortableTree,
  SortableTreeHandle,
  TreeItems,
} from '@/components/TreeDragSortable';
import { ItemChangedReason } from '@/components/TreeDragSortable/types';
import { postApiV1NodeMove, postApiV1NodeMoveNav } from '@/request/Node';
import { useAppSelector } from '@/store';
import { AppContext, DragTreeProps, getSiblingItemIds } from '@/utils/drag';
import { forwardRef, useImperativeHandle, useRef } from 'react';
import TreeItem from './TreeItem';

export type DragTreeHandle = {
  scrollToItem: (itemId: string) => void;
};

const DragTree = forwardRef<DragTreeHandle, DragTreeProps>(
  (
    {
      data,
      menu,
      updateData,
      refresh,
      ui = 'move',
      readOnly = false,
      loading = false,
      selected,
      onSelectChange,
      supportSelect = true,
      relativeSelect = true,
      selectionModel = 'cascade-parent-sync',
      disabled,
      virtualized = false,
      virtualizedHeight,
      canRootHaveChildren,
      registerDragHandlers,
      onItemClick,
      onExpand,
    },
    ref,
  ) => {
    const { kb_id } = useAppSelector(state => state.config);
    const sortableTreeRef = useRef<SortableTreeHandle>(null);

    // 暴露滚动方法
    useImperativeHandle(ref, () => ({
      scrollToItem: (itemId: string) => {
        sortableTreeRef.current?.scrollToItem(itemId);
      },
    }));

    return (
      <AppContext.Provider
        value={{
          ui,
          menu,
          data,
          updateData,
          refresh,
          readOnly,
          loading,
          selected,
          onSelectChange,
          supportSelect,
          relativeSelect,
          selectionModel,
          disabled,
          scrollToItem: (itemId: string) => {
            sortableTreeRef.current?.scrollToItem(itemId);
          },
          onItemClick,
          onExpand,
        }}
      >
        <SortableTree
          ref={sortableTreeRef}
          disableSorting={readOnly}
          canRootHaveChildren={canRootHaveChildren}
          registerDragHandlers={registerDragHandlers}
          items={data.map(it => ({ ...it }))}
          onItemsChanged={(
            newItems: TreeItems<ITreeItem>,
            reason: ItemChangedReason<ITreeItem>,
          ) => {
            if (reason.type === 'dropped') {
              const { draggedItem, droppedToParent } = reason;
              if (draggedItem.kind === 'nav') {
                updateData?.(newItems);
                return;
              }
              const id = String(draggedItem.id);
              const parentId =
                typeof draggedItem.parentId === 'string'
                  ? draggedItem.parentId
                  : '';
              const parent = droppedToParent as ITreeItem | null;
              const targetNavId =
                parent?.kind === 'nav' ? parent.nav_id || parent.id : '';
              if (
                targetNavId &&
                draggedItem.nav_id &&
                targetNavId !== draggedItem.nav_id
              ) {
                postApiV1NodeMoveNav({
                  ids: [id],
                  kb_id,
                  nav_id: String(targetNavId),
                }).then(() => {
                  updateData?.(newItems);
                  refresh?.();
                });
                return;
              }
              const targetParentId = parent?.kind === 'nav' ? '' : parentId;
              const { prevItemId, nextItemId } = getSiblingItemIds(
                newItems,
                id,
              );
              postApiV1NodeMove({
                id,
                parent_id: targetParentId,
                next_id: nextItemId as string,
                prev_id: prevItemId as string,
                kb_id: kb_id,
              }).then(() => {
                updateData?.(newItems);
                refresh?.();
              });
            } else {
              updateData?.(newItems);
              if (reason.type === 'expanded') onExpand?.(reason.item);
            }
          }}
          TreeItemComponent={TreeItem}
          virtualized={virtualized}
          virtualizedHeight={virtualizedHeight}
        />
      </AppContext.Provider>
    );
  },
);

DragTree.displayName = 'DragTree';

export default DragTree;
