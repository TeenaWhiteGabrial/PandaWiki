'use client';

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useEffect, useRef, useState } from 'react';
import type {
  PDFDocumentProxy,
  PDFDocumentLoadingTask,
  RenderTask,
} from 'pdfjs-dist';
import { loadPreviewContent, type AccessError } from './load-content.mjs';

export type { AccessError } from './load-content.mjs';
export interface SecureFilePreviewProps {
  url: string;
  headers?: Record<string, string>;
  kind: 'pdf' | 'text' | 'html';
  title?: string;
  assetBase?: string;
  onAccessError?: (error: AccessError) => void;
}

export default function SecureFilePreview({
  url,
  headers,
  kind,
  title = '文档预览',
  assetBase = '',
  onAccessError,
}: SecureFilePreviewProps) {
  const [data, setData] = useState<Uint8Array | null>(null);
  const [error, setError] = useState('');
  const [progress, setProgress] = useState('正在读取文件…');
  const [attempt, setAttempt] = useState(0);
  const onAccessErrorRef = useRef(onAccessError);
  onAccessErrorRef.current = onAccessError;
  const headerKey = JSON.stringify(headers || {});
  useEffect(() => {
    const controller = new AbortController();
    setData(null);
    setError('');
    setProgress('正在读取文件…');
    loadPreviewContent({
      url,
      headers: JSON.parse(headerKey),
      kind,
      signal: controller.signal,
      onProgress: (loaded, total) =>
        setProgress(
          total
            ? `正在读取文件 ${Math.round((loaded / total) * 100)}%`
            : `已读取 ${(loaded / 1048576).toFixed(1)} MB`,
        ),
      onAccessError: value => onAccessErrorRef.current?.(value),
    })
      .then(value => {
        if (!controller.signal.aborted) setData(value);
      })
      .catch(err => {
        if (!controller.signal.aborted) setError(err.message || '文件加载失败');
      });
    return () => controller.abort();
  }, [url, headerKey, kind, attempt]);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: 360,
        height: '100%',
        flex: 1,
        minWidth: 0,
        bgcolor: 'background.paper',
        borderRadius: 2,
        overflow: 'hidden',
      }}
    >
      {error ? (
        <Stack gap={2} sx={{ p: 3 }}>
          <Alert severity='error'>{error}</Alert>
          <Button onClick={() => setAttempt(n => n + 1)}>重新加载</Button>
        </Stack>
      ) : !data ? (
        <Stack
          gap={2}
          alignItems='center'
          justifyContent='center'
          sx={{ minHeight: 360 }}
        >
          <CircularProgress size={28} />
          <Typography color='text.secondary'>{progress}</Typography>
        </Stack>
      ) : kind === 'pdf' ? (
        <PdfCanvas
          data={data}
          title={title}
          assetBase={assetBase}
          onError={setError}
        />
      ) : (
        <TextPreview data={data} kind={kind} title={title} />
      )}
    </Box>
  );
}

function TextPreview({
  data,
  kind,
  title,
}: {
  data: Uint8Array;
  kind: 'text' | 'html';
  title: string;
}) {
  let text: string;
  try {
    text = new TextDecoder('utf-8', { fatal: true }).decode(data);
  } catch {
    text = new TextDecoder('gb18030').decode(data);
  }
  if (kind === 'html') {
    const policy =
      "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'";
    return (
      <Box
        component='iframe'
        title={title}
        sandbox=''
        referrerPolicy='no-referrer'
        srcDoc={`<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="${policy}"></head><body>${text}</body></html>`}
        sx={{ width: '100%', flex: 1, minHeight: '65vh', border: 0 }}
      />
    );
  }
  return (
    <Box
      component='pre'
      sx={{
        m: 0,
        p: 3,
        whiteSpace: 'pre-wrap',
        overflowWrap: 'anywhere',
        overflow: 'auto',
        fontFamily: 'monospace',
        flex: 1,
      }}
    >
      {text}
    </Box>
  );
}

function PdfCanvas({
  data,
  title,
  assetBase,
  onError,
}: {
  data: Uint8Array;
  title: string;
  assetBase: string;
  onError: (error: string) => void;
}) {
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [width, setWidth] = useState(800);
  const [busy, setBusy] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const canvas = useRef<HTMLCanvasElement>(null);
  const viewport = useRef<HTMLDivElement>(null);
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = viewport.current;
    if (!element) return;
    const observer = new ResizeObserver(() =>
      setWidth(Math.max(240, element.clientWidth - 32)),
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    const changed = () =>
      setFullscreen(document.fullscreenElement === root.current);
    document.addEventListener('fullscreenchange', changed);
    return () => document.removeEventListener('fullscreenchange', changed);
  }, []);
  useEffect(() => {
    let disposed = false;
    let task: PDFDocumentLoadingTask | undefined;
    const timeout = window.setTimeout(() => {
      if (!disposed) {
        onError('PDF 解析超时，请下载原文件查看');
        void task?.destroy().catch(() => {});
      }
    }, 30000);
    setPdf(null);
    setBusy(true);
    setPageNumber(1);
    const assets = `${assetBase.replace(/\/$/, '')}/pdfjs/`;
    import('pdfjs-dist')
      .then(lib => {
        if (disposed) return;
        lib.GlobalWorkerOptions.workerSrc = `${assets}pdf.worker.min.js?v=${lib.version}`;
        task = lib.getDocument({
          data,
          // PDF.js 6 no longer uses JS eval. Only render to canvas; do not
          // instantiate its scripting manager, forms or annotation actions.
          enableXfa: false,
          stopAtErrors: true,
          maxImageSize: 16 * 1024 * 1024,
          canvasMaxAreaInBytes: 64 * 1024 * 1024,
          cMapUrl: `${assets}cmaps/`,
          cMapPacked: true,
          standardFontDataUrl: `${assets}standard_fonts/`,
          wasmUrl: `${assets}wasm/`,
        });
        return task.promise;
      })
      .then(value => {
        clearTimeout(timeout);
        if (disposed || !value) return;
        if (value.numPages > 2000) {
          onError('PDF 页数过多，请下载原文件查看');
          void task?.destroy().catch(() => {});
          return;
        }
        setPdf(value);
      })
      .catch(err => {
        clearTimeout(timeout);
        if (!disposed) {
          console.error('PDF preview parse error:', err);
          onError(
            err?.name === 'PasswordException'
              ? '此 PDF 已加密，请下载后使用密码打开'
              : 'PDF 损坏、格式不兼容或阅读器资源加载失败，请重试或下载原文件检查',
          );
        }
      });
    return () => {
      disposed = true;
      clearTimeout(timeout);
      void task?.destroy().catch(() => {});
    };
  }, [data, assetBase, onError]);
  useEffect(() => {
    if (!pdf || !canvas.current) return;
    let disposed = false;
    let task: RenderTask | undefined;
    setBusy(true);
    const timeout = window.setTimeout(() => {
      if (!disposed) {
        task?.cancel();
        onError('此页渲染超时，请下载原文件查看');
      }
    }, 20000);
    pdf
      .getPage(pageNumber)
      .then(page => {
        if (disposed || !canvas.current) return;
        const original = page.getViewport({ scale: 1 });
        const scale = Math.min(
          (width / original.width) * zoom,
          8192 / Math.max(original.width, original.height),
        );
        const view = page.getViewport({ scale });
        const ratio = Math.min(
          window.devicePixelRatio || 1,
          2,
          8192 / Math.max(view.width, view.height),
          Math.sqrt((16 * 1024 * 1024) / (view.width * view.height)),
        );
        const element = canvas.current;
        element.width = Math.max(1, Math.floor(view.width * ratio));
        element.height = Math.max(1, Math.floor(view.height * ratio));
        element.style.width = `${view.width}px`;
        element.style.height = `${view.height}px`;
        task = page.render({
          canvas: element,
          viewport: view,
          transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0],
        });
        return task.promise;
      })
      .then(() => {
        clearTimeout(timeout);
        if (!disposed) setBusy(false);
      })
      .catch(err => {
        clearTimeout(timeout);
        if (!disposed && err?.name !== 'RenderingCancelledException')
          onError('此页渲染失败，请重试或下载原文件检查');
      });
    return () => {
      disposed = true;
      clearTimeout(timeout);
      task?.cancel();
    };
  }, [pdf, pageNumber, zoom, width, onError]);
  return (
    <Stack
      ref={root}
      sx={{ flex: 1, minHeight: 0, bgcolor: 'background.paper' }}
    >
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='center'
        gap={1}
        flexWrap='wrap'
        sx={{ p: 1, borderBottom: 1, borderColor: 'divider' }}
      >
        <Button
          size='small'
          disabled={!pdf || pageNumber <= 1}
          onClick={() => setPageNumber(n => n - 1)}
        >
          上一页
        </Button>
        <TextField
          size='small'
          type='number'
          value={pageNumber}
          disabled={!pdf}
          slotProps={{
            htmlInput: {
              min: 1,
              max: pdf?.numPages || 1,
              'aria-label': '页码',
            },
          }}
          sx={{ width: 80 }}
          onChange={e => {
            const n = Number(e.target.value);
            if (pdf && n >= 1 && n <= pdf.numPages) setPageNumber(n);
          }}
        />
        <Typography variant='body2'>/ {pdf?.numPages || '—'} 页</Typography>
        <Button
          size='small'
          disabled={!pdf || pageNumber >= pdf.numPages}
          onClick={() => setPageNumber(n => n + 1)}
        >
          下一页
        </Button>
        <Button
          size='small'
          aria-label='缩小'
          disabled={zoom <= 0.5}
          onClick={() => setZoom(z => Math.max(0.5, z - 0.25))}
        >
          −
        </Button>
        <Typography variant='body2'>{Math.round(zoom * 100)}%</Typography>
        <Button
          size='small'
          aria-label='放大'
          disabled={zoom >= 2}
          onClick={() => setZoom(z => Math.min(2, z + 0.25))}
        >
          ＋
        </Button>
        <Button
          size='small'
          onClick={() => {
            if (document.fullscreenElement === root.current)
              void document.exitFullscreen();
            else
              void root.current
                ?.requestFullscreen()
                .catch(() => onError('浏览器不支持全屏，请使用页面全屏预览'));
          }}
        >
          {fullscreen ? '退出全屏' : '全屏'}
        </Button>
      </Stack>
      <Box
        ref={viewport}
        sx={{
          position: 'relative',
          flex: 1,
          minHeight: 360,
          maxHeight: fullscreen ? 'none' : '75vh',
          overflow: 'auto',
          p: 2,
          bgcolor: '#eceff1',
          textAlign: 'center',
        }}
      >
        {busy && (
          <Stack alignItems='center' sx={{ py: 2 }}>
            <CircularProgress size={24} />
            <Typography variant='body2'>正在渲染第 {pageNumber} 页…</Typography>
          </Stack>
        )}
        <canvas
          ref={canvas}
          aria-label={`${title}，第 ${pageNumber} 页`}
          style={{
            display: busy ? 'none' : 'inline-block',
            background: '#fff',
          }}
        />
      </Box>
    </Stack>
  );
}
