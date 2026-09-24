import { ITreeItem } from '@/api';
import {
  TreeMenuItem,
  TreeMenuOptions,
} from '@/components/Drag/DragTree/TreeMenu';
import { ConstsCrawlerSource } from '@/request/types';
import { useCallback } from 'react';
import type { DocTreeMenuHandlers } from './types';

export function useDocTreeMenu(
  handlers: DocTreeMenuHandlers,
  readOnly = false,
): (opra: TreeMenuOptions) => TreeMenuItem[] {
  const {
    handleUrl,
    handleBatchImport,
    handleFolderDownload,
    handleMove,
    handleDelete,
    handlePublish,
    handleRestudy,
    handleProperties,
    handleFrontDoc,
  } = handlers;

  return useCallback(
    (opra: TreeMenuOptions): TreeMenuItem[] => {
      const { item, createItem, renameItem, isEditing } = opra;
      const menuItems: TreeMenuItem[] = [];

      if (!readOnly && item.type === 1) {
        menuItems.push(
          { label: '创建文件夹', key: 'folder', onClick: () => createItem(1) },
          {
            label: '创建文档',
            key: 'doc',
            children: [
              {
                label: '创建富文本',
                key: 'rich_text',
                onClick: () => createItem(2, 'html'),
              },
              {
                label: '创建 Markdown',
                key: 'md',
                onClick: () => createItem(2, 'md'),
              },
            ],
          },
          {
            label: '导入文档',
            key: ConstsCrawlerSource.CrawlerSourceFile,
            onClick: () =>
              handleUrl(item, ConstsCrawlerSource.CrawlerSourceFile),
          },
          ...(handleBatchImport
            ? [
                {
                  label: '批量导入 ZIP',
                  key: 'batch-import-zip',
                  onClick: () => handleBatchImport(item),
                },
              ]
            : []),
          ...(handleFolderDownload
            ? [
                {
                  label: '下载文件夹',
                  key: 'download-folder',
                  onClick: () => handleFolderDownload(item),
                },
              ]
            : []),
          ...(handleMove
            ? [
                {
                  label: '移动',
                  key: 'move',
                  onClick: () => handleMove(item),
                },
              ]
            : []),
          {
            label: '目录权限',
            key: 'properties',
            onClick: () => handleProperties(item),
          },
        );
      }

      if (item.type === 2) {
        if (!readOnly) {
          if (item.status === 1 || item.status === 0) {
            menuItems.push({
              label: item.status === 1 ? '发布更新' : '发布文档',
              key: 'update_publish',
              onClick: () => handlePublish(item),
            });
          }
          if (handleRestudy && (item.status === 1 || item.status === 2)) {
            menuItems.push({
              label: '重建问答索引',
              key: 'restudy',
              onClick: () => handleRestudy(item),
            });
          }
          if (handleMove) {
            menuItems.push({
              label: '移动',
              key: 'move',
              onClick: () => handleMove(item),
            });
          }
          menuItems.push({
            label: '文档权限',
            key: 'properties',
            onClick: () => handleProperties(item),
          });
        }
        menuItems.push({
          label: '前台查看',
          key: 'front_doc',
          onClick: () => handleFrontDoc(item.id),
        });
      }

      if (!readOnly && !isEditing) {
        menuItems.push({ label: '重命名', key: 'rename', onClick: renameItem });
      }
      if (!readOnly) {
        menuItems.push({
          label: '删除',
          color: 'error',
          key: 'delete',
          onClick: () => handleDelete(item),
        });
      }

      return menuItems;
    },
    [
      handleUrl,
      handleBatchImport,
      handleFolderDownload,
      handleMove,
      handleDelete,
      handlePublish,
      handleRestudy,
      handleProperties,
      handleFrontDoc,
      readOnly,
    ],
  );
}
