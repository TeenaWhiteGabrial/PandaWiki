export class PreviewLoadError extends Error {
  constructor(message, status = 0, data = {}) {
    super(message);
    this.name = 'PreviewLoadError';
    this.status = status;
    this.data = data;
  }
}

export async function loadPreviewContent({
  url,
  headers,
  kind,
  signal,
  onProgress,
  onAccessError,
  timeoutMs = 45000,
}) {
  const limit = kind === 'pdf' ? 100 * 1024 * 1024 : 5 * 1024 * 1024;
  const controller = new AbortController();
  const abort = () => controller.abort();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);
  signal?.addEventListener('abort', abort, { once: true });
  if (signal?.aborted) controller.abort();
  let reader;
  try {
    const response = await fetch(url, {
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
      signal: controller.signal,
    });
    const type = (response.headers.get('content-type') || '').toLowerCase();
    if (!response.ok || type.includes('application/json')) {
      let data = {};
      try {
        data = await response.json();
      } catch {}
      if (data.trace_id) console.error('file-preview trace_id:', data.trace_id);
      onAccessError?.({ status: response.status, data });
      const fallback = {
        401: '请登录后查看文件',
        403: '没有此文档的查阅权限',
        404: '文件不存在，可能尚未迁移或已被删除',
        413: '文件过大，请下载原文件查看',
        415: '文件格式不支持预览',
        504: '文件读取超时，请重试',
      };
      throw new PreviewLoadError(
        data.message || fallback[response.status] || '文件读取失败，请重试',
        response.status,
        data,
      );
    }
    if (kind === 'pdf' && !type.startsWith('application/pdf'))
      throw new PreviewLoadError('服务器未返回 PDF 内容，请下载原文件检查');
    if (kind !== 'pdf' && !type.startsWith('text/plain'))
      throw new PreviewLoadError('服务器未返回可安全预览的文本');
    const total = Number(response.headers.get('content-length')) || 0;
    if (total > limit) {
      controller.abort();
      throw new PreviewLoadError('文件过大，请下载原文件查看', 413);
    }
    if (!response.body) throw new PreviewLoadError('文件内容为空');
    reader = response.body.getReader();
    const chunks = [];
    let loaded = 0;
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      loaded += value.byteLength;
      if (loaded > limit) {
        controller.abort();
        throw new PreviewLoadError('文件过大，请下载原文件查看', 413);
      }
      chunks.push(value);
      onProgress?.(loaded, total);
    }
    if (total && loaded !== total)
      throw new PreviewLoadError('文件传输不完整，请重试');
    const data = new Uint8Array(loaded);
    let offset = 0;
    for (const chunk of chunks) {
      data.set(chunk, offset);
      offset += chunk.length;
    }
    if (
      kind === 'pdf' &&
      !new TextDecoder('ascii').decode(data.subarray(0, 1024)).includes('%PDF-')
    )
      throw new PreviewLoadError('文件内容不是有效的 PDF');
    return data;
  } catch (error) {
    if (error instanceof PreviewLoadError) throw error;
    if (timedOut) throw new PreviewLoadError('文件加载超时，请重试', 408);
    if (signal?.aborted) throw error;
    throw new PreviewLoadError('无法加载文件，请检查网络后重试');
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener('abort', abort);
    if (reader) await reader.cancel().catch(() => {});
  }
}
