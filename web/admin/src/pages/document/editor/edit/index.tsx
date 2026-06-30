import { getApiV1NodeDetail } from '@/request/Node';
import {
  getApiV1NodeCollabCompat,
  NodeCollabResp,
} from '@/request/collabCompat';
import { V1NodeDetailResp } from '@/request/types';
import { useAppSelector } from '@/store';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { Box } from '@mui/material';
import { useEffect, useState } from 'react';
import { useOutletContext, useParams, useSearchParams } from 'react-router-dom';
import { WrapContext } from '..';
import FilePreview from './FilePreview';
import LoadingEditorWrap from './Loading';
import EditorWrap from './Wrap';

const editorPerfLog = (event: string, payload?: Record<string, unknown>) => {
  try {
    if (
      !(
        import.meta.env.DEV || localStorage.getItem('panda-editor-perf') === '1'
      )
    ) {
      return;
    }
  } catch {
    if (!import.meta.env.DEV) return;
  }
  console.info(`[PandaWiki Editor Perf] ${event}`, {
    ts: Math.round(performance.now()),
    ...payload,
  });
};

type CollabNodeDetailResp = V1NodeDetailResp & {
  collab?: NodeCollabResp;
};

const Edit = () => {
  const { id = '' } = useParams();
  const [searchParams] = useSearchParams();
  const { kb_id = '', user, kbDetail } = useAppSelector(state => state.config);
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);
  const { setNodeDetail } = useOutletContext<WrapContext>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<CollabNodeDetailResp | null>(null);
  const isReadonlyFilePreview =
    !!detail?.meta?.file_key && !!detail?.meta?.file_readonly;

  const getDetail = () => {
    const startedAt = performance.now();
    editorPerfLog('detail-load:start', { id, kb_id, readOnlyUser });
    setLoading(true);
    getApiV1NodeDetail({
      id,
      kb_id,
    })
      .then(async res => {
        editorPerfLog('detail-load:response', {
          id,
          durationMs: Math.round(performance.now() - startedAt),
          hasDetail: !!res,
        });
        let collab: NodeCollabResp | undefined = (res as CollabNodeDetailResp)
          .collab;
        if (!readOnlyUser) {
          const collabStartedAt = performance.now();
          try {
            collab = collab || (await getApiV1NodeCollabCompat({ id, kb_id }));
            editorPerfLog('detail-collab:loaded', {
              id,
              durationMs: Math.round(performance.now() - collabStartedAt),
              enabled: !!collab?.enabled,
              roomId: collab?.room_id,
            });
          } catch {
            editorPerfLog('detail-collab:failed', {
              id,
              durationMs: Math.round(performance.now() - collabStartedAt),
            });
            collab = undefined;
          }
        }
        const urlType = searchParams.get('type');
        const nextDetail = {
          ...res,
          meta: {
            ...res.meta,
            ...(urlType === 'html' || urlType === 'md'
              ? { content_type: urlType }
              : {}),
          },
          collab,
        };
        setDetail(nextDetail);
        setNodeDetail(nextDetail);
        editorPerfLog('detail-load:ready', {
          id,
          durationMs: Math.round(performance.now() - startedAt),
          contentType: nextDetail.meta?.content_type,
          contentLength: nextDetail.content?.length || 0,
          collabEnabled: !!nextDetail.collab?.enabled,
        });
        setTimeout(() => {
          window.scrollTo({ top: 0, behavior: 'smooth' });
        }, 0);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    if (id && kb_id) {
      getDetail();
    }
  }, [id, kb_id, readOnlyUser, searchParams]);

  return (
    <Box
      sx={{
        position: 'relative',
        flexGrow: 1,
        /* Give a remote user a caret */
        '& .collaboration-carets__caret': {
          borderLeft: '1px solid #fff',
          borderRight: '1px solid #fff',
          marginLeft: '-1px',
          marginRight: '-1px',
          pointerEvents: 'none',
          position: 'relative',
          wordBreak: 'normal',
        },
        /* Render the username above the caret */
        '& .collaboration-carets__label': {
          borderRadius: '0 3px 3px 3px',
          color: '#fff',
          fontSize: '12px',
          fontStyle: 'normal',
          fontWeight: '600',
          left: '-1px',
          lineHeight: 'normal',
          padding: '0.1rem 0.3rem',
          position: 'absolute',
          top: '1.4em',
          userSelect: 'none',
          whiteSpace: 'nowrap',
        },
      }}
    >
      {loading ? <LoadingEditorWrap /> : null}
      {!loading && detail ? (
        isReadonlyFilePreview ? (
          <FilePreview detail={detail} />
        ) : (
          <EditorWrap detail={detail} readOnlyUser={readOnlyUser} />
        )
      ) : null}
    </Box>
  );
};

export default Edit;
