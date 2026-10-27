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

const Doc = ({
  nodeId,
  node,
  error,
}: {
  nodeId?: string;
  node?: V1ShareNodeDetailResp;
  error?: Partial<Error> & { digest?: string } & { code?: number | string };
}) => {
  const {
    kbDetail,
    mobile,
    catalogWidth,
    authInfo,
    authRevision = 0,
    setLoadingNodeId,
  } = useStore();
  const requestedNodeId = nodeId || node?.id;
  const requestScope = `${authInfo?.id || 'anonymous'}:${authRevision}:${requestedNodeId || ''}`;
  const [loading, setLoading] = useState(true);
  const [editorReady, setEditorReady] = useState(false);
  const [headings, setHeadings] = useState<TocList>([]);
  const [characterCount, setCharacterCount] = useState(0);
  const [nodeState, setNodeState] = useState<V1ShareNodeDetailResp | undefined>(
    node,
  );
  const [loadedScope, setLoadedScope] = useState(requestScope);
  const [pageError, setPageError] = useState<typeof error>(error);
  const pathname = usePathname();
  const isCurrentScope = loadedScope === requestScope;
  const currentNode = isCurrentScope ? nodeState : undefined;
  const currentError = isCurrentScope ? pageError : undefined;
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
    if (!requestedNodeId) {
      return;
    }

    let disposed = false;
    setLoadingNodeId?.(requestedNodeId);
    // Always reauthorize the document on navigation or identity changes.
    // A process-wide cache keyed only by document ID can replay content
    // obtained by a previous login without consulting the permission API.
    setLoadedScope(requestScope);
    setNodeState(undefined);
    setLoading(true);
    setPageError(undefined);
    setEditorReady(false);
    setHeadings([]);
    setCharacterCount(0);

    getShareV1NodeDetail(
      { id: requestedNodeId, format: 'json' },
      { cache: 'no-store' },
    )
      .then(res => {
        if (!disposed) {
          const detail = res as V1ShareNodeDetailResp | undefined;
          if (!detail) throw new Error('文档内容为空');
          setNodeState(detail);
          setLoading(false);
        }
      })
      .catch(err => {
        if (!disposed) {
          setPageError(err as Error);
          setLoading(false);
        }
      })
      .finally(() => {
        if (!disposed)
          setLoadingNodeId?.(current =>
            current === requestedNodeId ? undefined : current,
          );
      });

    return () => {
      disposed = true;
      setLoadingNodeId?.(current =>
        current === requestedNodeId ? undefined : current,
      );
    };
  }, [requestedNodeId, requestScope, setLoadingNodeId]);

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
      {currentError ? (
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
          <ErrorComponent error={currentError} />
        </Box>
      ) : (
        <>
          <DocContent
            info={currentNode}
            docWidth={docWidth}
            editorRef={editorRef}
            editorReady={editorReady}
            characterCount={characterCount}
            loading={!isCurrentScope || (loading && !isReadonlyFilePreview)}
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
