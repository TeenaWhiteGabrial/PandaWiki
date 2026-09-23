import {
  ZipUploadTask,
  postApiV1FileUploadZipPartComplete,
  postApiV1FileUploadZipTaskComplete,
  postApiV1FileUploadZipTaskRetry,
  putApiV1FileUploadZipPart,
} from '@/request/BatchImport';

export const ZIP_UPLOAD_STORAGE_PREFIX = 'panda-wiki-zip-upload:';

export const parseZipUploadedParts = (
  value?: ZipUploadTask['uploaded_parts'],
): Record<string, string> => {
  if (!value) return {};
  if (typeof value !== 'string') return value || {};
  try {
    return JSON.parse(value || '{}') || {};
  } catch {
    return {};
  }
};

export const getZipUploadedPartCount = (task: ZipUploadTask) => {
  if (typeof task.uploaded_part_count === 'number') {
    return task.uploaded_part_count;
  }
  return Object.keys(parseZipUploadedParts(task.uploaded_parts)).length;
};

export const getZipMissingParts = (task: ZipUploadTask) => {
  if (task.missing_parts?.length) return task.missing_parts;
  const uploadedParts = parseZipUploadedParts(task.uploaded_parts);
  const missing: number[] = [];
  for (let part = 1; part <= task.total_parts; part += 1) {
    if (!uploadedParts[String(part)]) missing.push(part);
  }
  return missing;
};

export const canResumeZipUpload = (task: ZipUploadTask) =>
  Boolean(task.can_resume) ||
  (['waiting', 'uploading', 'interrupted', 'failed'].includes(task.status) &&
    getZipUploadedPartCount(task) < task.total_parts);

export const assertSameZipFile = (task: ZipUploadTask, file: File) => {
  if (file.name !== task.filename || file.size !== task.size) {
    throw new Error('请选择同一个 ZIP 文件继续上传');
  }
};

export interface UploadZipTaskPartsOptions {
  file: File;
  task: ZipUploadTask;
  onProgress?: (progress: number, task: ZipUploadTask) => void;
  onTaskUpdate?: (task: ZipUploadTask) => void;
}

const clampPercent = (value: number) =>
  Number.isFinite(value) ? Math.max(0, Math.min(100, Math.round(value))) : 0;

export const uploadZipTaskParts = async ({
  file,
  task,
  onProgress,
  onTaskUpdate,
}: UploadZipTaskPartsOptions) => {
  assertSameZipFile(task, file);

  let currentTask =
    task.status === 'failed' || task.status === 'interrupted'
      ? await postApiV1FileUploadZipTaskRetry(task.id)
      : task;

  onTaskUpdate?.(currentTask);
  onProgress?.(
    currentTask.size
      ? clampPercent(
          ((currentTask.uploaded_bytes || 0) * 100) / currentTask.size,
        )
      : 0,
    currentTask,
  );

  for (let part = 1; part <= currentTask.total_parts; part += 1) {
    const uploadedParts = parseZipUploadedParts(currentTask.uploaded_parts);
    const start = (part - 1) * currentTask.part_size;
    const end = Math.min(file.size, start + currentTask.part_size);
    if (uploadedParts[String(part)]) {
      onProgress?.(clampPercent((end * 100) / file.size), currentTask);
      continue;
    }

    let uploaded = false;
    let lastError: unknown;
    for (let attempt = 1; attempt <= 3 && !uploaded; attempt += 1) {
      try {
        const partResult = await putApiV1FileUploadZipPart(
          currentTask.id,
          part,
          file.slice(start, end),
          (loaded, total) => {
            const currentPartBytes =
              total > 0 ? (loaded / total) * (end - start) : loaded;
            onProgress?.(
              clampPercent(((start + currentPartBytes) * 100) / file.size),
              currentTask,
            );
          },
        );
        currentTask = await postApiV1FileUploadZipPartComplete(
          currentTask.id,
          part,
          {
            etag: partResult.etag,
            size: end - start,
          },
        );
        onTaskUpdate?.(currentTask);
        uploaded = true;
      } catch (error) {
        lastError = error;
        if (attempt < 3) {
          await new Promise(resolve =>
            window.setTimeout(resolve, attempt * 1000),
          );
        }
      }
    }

    if (!uploaded) {
      throw lastError instanceof Error
        ? lastError
        : new Error(`第 ${part} 个分片上传失败`);
    }
  }

  currentTask = await postApiV1FileUploadZipTaskComplete(currentTask.id);
  onTaskUpdate?.(currentTask);
  onProgress?.(100, currentTask);
  localStorage.removeItem(`${ZIP_UPLOAD_STORAGE_PREFIX}${currentTask.id}`);
  return currentTask;
};
