import { getApiV1NodeDetail } from '@/request/Node';
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

const Edit = () => {
  const { id = '' } = useParams();
  const [searchParams] = useSearchParams();
  const { kb_id = '', user, kbDetail } = useAppSelector(state => state.config);
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);
  const { setNodeDetail } = useOutletContext<WrapContext>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<V1NodeDetailResp | null>(null);
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
      .then(res => {
        editorPerfLog('detail-load:response', {
          id,
          durationMs: Math.round(performance.now() - startedAt),
          hasDetail: !!res,
        });
        const urlType = searchParams.get('type');
        const nextDetail = {
          ...res,
          meta: {
            ...res.meta,
            ...(urlType === 'html' || urlType === 'md'
              ? { content_type: urlType }
              : {}),
          },
        };
        setDetail(nextDetail);
        setNodeDetail(nextDetail);
        editorPerfLog('detail-load:ready', {
          id,
          durationMs: Math.round(performance.now() - startedAt),
          contentType: nextDetail.meta?.content_type,
          contentLength: nextDetail.content?.length || 0,
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
      }}
    >
      {loading ? <LoadingEditorWrap /> : null}
      {!loading && detail ? (
        isReadonlyFilePreview ? (
          <FilePreview detail={detail} />
        ) : (
          <EditorWrap
            detail={detail}
            readOnlyUser={readOnlyUser || detail.can_manage === false}
          />
        )
      ) : null}
    </Box>
  );
};

export default Edit;
