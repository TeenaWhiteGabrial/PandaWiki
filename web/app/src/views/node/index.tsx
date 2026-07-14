'use client';

import DocFab from '@/components/docFab';
import ErrorComponent from '@/components/error';
import ScrollToTopFab from '@/components/scrollToTopFab';
import { useBasePath } from '@/hooks/useBasePath';
import { getShareV1NodeDetail } from '@/request/ShareNode';
import type { V1ShareNodeDetailResp } from '@/request/types';
import { getDocContentSx } from '@/utils/getDocContentSx';
import { extractMarkdownToc } from '@/utils/markdownToc';
import useCopy from '@/hooks/useCopy';
import { useStore } from '@/provider';
import { ConstsCopySetting } from '@/request/types';
import { TocList, useTiptap, UseTiptapReturn } from '@ctzhian/tiptap';
import { Box } from '@mui/material';
import { usePathname } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import DocAnchor from './DocAnchor';
import DocContent from './DocContent';

const nodeDetailCache = new Map<string, V1ShareNodeDetailResp>();
const nodeDetailPromiseCache = new Map<
  string,
  Promise<V1ShareNodeDetailResp>
>();

const loadNodeDetail = async (nodeId: string) => {
  const cached = nodeDetailCache.get(nodeId);
  if (cached) {
    return cached;
  }

  const pending = nodeDetailPromiseCache.get(nodeId);
  if (pending) {
    return pending;
  }

  const promise = getShareV1NodeDetail({ id: nodeId, format: 'json' })
    .then(res => {
      const detail = (res as V1ShareNodeDetailResp) ?? undefined;
      if (!detail) {
        throw new Error(`node detail is empty: ${nodeId}`);
      }
      nodeDetailCache.set(nodeId, detail);
      return detail;
    })
    .finally(() => {
      nodeDetailPromiseCache.delete(nodeId);
    });

  nodeDetailPromiseCache.set(nodeId, promise);
  return promise;
};

const Doc = ({
  nodeId,
  node,
  error,
}: {
  nodeId?: string;
  node?: V1ShareNodeDetailResp;
  error?: Partial<Error> & { digest?: string } & { code?: number | string };
}) => {
  const { kbDetail, mobile, catalogWidth } = useStore();
  const [loading, setLoading] = useState(true);
  const [editorReady, setEditorReady] = useState(false);
  const [headings, setHeadings] = useState<TocList>([]);
  const [characterCount, setCharacterCount] = useState(0);
  const [nodeState, setNodeState] = useState<V1ShareNodeDetailResp | undefined>(
    node,
  );
  const [pageError, setPageError] = useState<typeof error>(error);
  const pathname = usePathname();
  const currentNode = nodeState;
  const isMarkdown = useMemo(() => {
    return currentNode?.meta?.content_type === 'md';
  }, [currentNode?.meta?.content_type]);
  const baseUrl = useBasePath();
  const isReadonlyFilePreview = useMemo(() => {
    return !!currentNode?.meta?.file_key && !!currentNode?.meta?.file_readonly;
  }, [currentNode?.meta?.file_key, currentNode?.meta?.file_readonly]);
  const shouldUseTiptap = useMemo(() => {
    return !isReadonlyFilePreview && !isMarkdown;
  }, [isMarkdown, isReadonlyFilePreview]);

  useEffect(() => {
    setNodeState(node);
    if (node?.id) {
      nodeDetailCache.set(node.id, node);
    }
  }, [node]);

  useEffect(() => {
    if (!nodeId) return;
    if (node?.id === nodeId) return;

    const cached = nodeDetailCache.get(nodeId);
    setNodeState(cached);
    setLoading(true);
    setEditorReady(false);
    setHeadings([]);
    setCharacterCount(0);
  }, [node?.id, nodeId]);

  useEffect(() => {
    setPageError(error);
  }, [error]);

  useEffect(() => {
    if (node || !nodeId) {
      return;
    }

    let disposed = false;
    setLoading(true);
    setPageError(undefined);

    loadNodeDetail(nodeId)
      .then(res => {
        if (!disposed) {
          setNodeState(res);
          setLoading(false);
        }
      })
      .catch(err => {
        if (!disposed) {
          setPageError(err as Error);
          setLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [node, nodeId]);

  useEffect(() => {
    if (isReadonlyFilePreview || isMarkdown) {
      setLoading(false);
      setEditorReady(false);
      setHeadings(
        isMarkdown ? extractMarkdownToc(currentNode?.content || '') : [],
      );
      setCharacterCount(currentNode?.content?.length ?? 0);
    }
  }, [currentNode?.content, isMarkdown, isReadonlyFilePreview]);
  const tiptapRef = useTiptap({
    content: shouldUseTiptap ? currentNode?.content || '' : '',
    editable: false,
    contentType: 'html',
    immediatelyRender: false,
    baseUrl: baseUrl,
    onTocUpdate: (toc: TocList) => {
      setHeadings(toc);
    },
    onBeforeCreate: () => {
      setLoading(true);
      setEditorReady(false);
    },
    onCreate: ({ editor }) => {
      setLoading(false);
      setEditorReady(true);
      setCharacterCount((editor.storage as any).characterCount.characters());
    },
  });
  const editorRef = (
    !shouldUseTiptap
      ? ({ editor: null } as unknown as UseTiptapReturn)
      : tiptapRef
  ) as UseTiptapReturn;

  const docWidth = useMemo(() => {
    return kbDetail?.settings?.theme_and_style?.doc_width || 'full';
  }, [kbDetail]);

  useCopy({
    mode:
      kbDetail?.settings?.copy_setting !== ConstsCopySetting.CopySettingDisabled
        ? 'allow'
        : 'disable',
    blockContextMenuWhenDisabled: false,
    suffix:
      kbDetail?.settings?.copy_setting === ConstsCopySetting.CopySettingAppend
        ? `\n\n-----------------------------------------\n内容来自 ${typeof window !== 'undefined' ? window.location.href : ''}`
        : '',
  });

  useEffect(() => {
    if (!shouldUseTiptap || !currentNode || !editorRef.editor) return;

    setEditorReady(false);
    setHeadings([]);
    requestAnimationFrame(() => {
      editorRef.setContent(currentNode?.content || '', 'html');
      setEditorReady(true);
    });
  }, [currentNode, editorRef.editor, shouldUseTiptap]);

  useEffect(() => {
    document.querySelector('#scroll-container')?.scrollTo({ top: 0 });
  }, [pathname]);

  return (
    <>
      {pageError ? (
        <Box
          sx={{
            height: '100%',
            ...getDocContentSx({
              docWidth,
              mobile: mobile ?? false,
              catalogWidth: catalogWidth ?? 260,
              variant: 'error',
            }),
          }}
        >
          <ErrorComponent error={pageError} />
        </Box>
      ) : (
        <>
          <DocContent
            info={currentNode}
            docWidth={docWidth}
            editorRef={editorRef}
            editorReady={editorReady}
            characterCount={characterCount}
            loading={loading && !isReadonlyFilePreview}
            headings={headings}
          />
          {!mobile && !isReadonlyFilePreview && headings.length > 0 && (
            <DocAnchor headings={headings} />
          )}
          <DocFab />
          {!mobile && <ScrollToTopFab />}
        </>
      )}
    </>
  );
};

export default Doc;
