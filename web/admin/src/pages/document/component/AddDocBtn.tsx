import TreeMenu, { TreeMenuItem } from '@/components/Drag/DragTree/TreeMenu';
import { ConstsCrawlerSource } from '@/request';
import { Box, Button } from '@mui/material';
import { useState } from 'react';
import AddDocByType from './AddDocByType';
import DocAddByCustomText from './DocAddByCustomText';
import BatchZipImport from './BatchZipImport';

interface InputContentProps {
  exportFile?: boolean;
  refresh?: () => void;
  disabled?: boolean;
  context?: React.ReactElement<{ onClick?: any; 'aria-describedby'?: any }>;
  createLocal?: (node: {
    id: string;
    name: string;
    type: 1 | 2;
    emoji?: string;
    parentId?: string | null;
    content_type?: string;
  }) => void;
  scrollTo?: (id: string) => void;
  hidden?: boolean;
  parentId?: string | null;
  parentName?: string;
  navId?: string | null;
  batchImport?: boolean;
}

const AddDocBtn = ({
  exportFile = true,
  refresh,
  disabled = false,
  context,
  createLocal,
  scrollTo,
  hidden = false,
  parentId = null,
  parentName = '根目录',
  navId,
  batchImport = true,
}: InputContentProps) => {
  if (hidden) return null;

  const [customDocOpen, setCustomDocOpen] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [key, setKey] = useState<ConstsCrawlerSource | null>(null);
  const [docFileKey, setDocFileKey] = useState<1 | 2>(1);
  const [batchImportOpen, setBatchImportOpen] = useState(false);

  const menuItems: TreeMenuItem[] = [
    {
      key: 'docFile',
      label: '创建文件夹',
      onClick: () => {
        setDocFileKey(1);
        setCustomDocOpen(true);
      },
    },
    {
      key: 'next-line',
      label: '创建文档',
      onClick: () => {
        setDocFileKey(2);
        setCustomDocOpen(true);
      },
    },
    ...(exportFile
      ? [
          {
            key: ConstsCrawlerSource.CrawlerSourceFile,
            label: '导入文档',
            onClick: () => {
              setUploadOpen(true);
              setKey(ConstsCrawlerSource.CrawlerSourceFile);
            },
          },
        ]
      : []),
  ];

  const close = () => {
    setUploadOpen(false);
    setCustomDocOpen(false);
  };

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <TreeMenu
        menu={menuItems}
        context={
          context || (
            <Button variant='contained' disabled={disabled}>
              创建文档
            </Button>
          )
        }
      />
      {batchImport && (
        <Button
          variant='outlined'
          size='small'
          disabled={disabled}
          onClick={() => setBatchImportOpen(true)}
        >
          批量导入 ZIP
        </Button>
      )}
      {key && (
        <AddDocByType
          type={key}
          open={uploadOpen}
          refresh={refresh}
          onCancel={close}
          parentId={parentId}
        />
      )}
      <BatchZipImport
        open={batchImportOpen}
        parentId={parentId}
        parentName={parentName}
        navId={navId}
        onClose={() => setBatchImportOpen(false)}
        refresh={refresh}
      />
      <DocAddByCustomText
        type={docFileKey}
        open={customDocOpen}
        refresh={refresh}
        onCreated={node => {
          createLocal?.(node);
          scrollTo?.(node.id);
        }}
        onClose={() => setCustomDocOpen(false)}
      />
    </Box>
  );
};

export default AddDocBtn;
