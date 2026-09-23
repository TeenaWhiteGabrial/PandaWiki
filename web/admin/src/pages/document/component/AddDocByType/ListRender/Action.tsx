import {
  ConstsCrawlerSource,
  postApiV1CrawlerExport,
  postApiV1CrawlerParse,
  postApiV1Node,
} from '@/request';
import { uploadAttachment } from '@/api';
import { useAppSelector } from '@/store';
import { message } from '@/components/ui';
import {
  alpha,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Stack,
  useTheme,
} from '@mui/material';
import { useCallback, useEffect, useMemo } from 'react';
import { ListDataItem } from '..';
import { useGlobalQueue } from '../hooks/useGlobalQueue';
import { pollCrawlerResults } from '../util';

interface BatchActionBarProps {
  loading: boolean;
  data: ListDataItem[];
  setData: React.Dispatch<React.SetStateAction<ListDataItem[]>>;
  checked: string[];
  setChecked: React.Dispatch<React.SetStateAction<string[]>>;
  type: ConstsCrawlerSource;
  isSupportSelect: boolean;
  parent_id: string | null;
  nav_id?: string | null;
  queue: ReturnType<typeof useGlobalQueue>;
}

const BatchActionBar = (props: BatchActionBarProps) => {
  const theme = useTheme();
  const { kb_id, nav_id: selectedNavId } = useAppSelector(
    state => state.config,
  );
  const {
    data,
    loading,
    setData,
    setChecked,
    checked,
    type,
    isSupportSelect,
    parent_id,
    nav_id,
    queue,
  } = props;
  const targetNavId = nav_id || selectedNavId;
  const isFileImportType = type === ConstsCrawlerSource.CrawlerSourceFile;

  const getFileExt = useCallback((name?: string) => {
    return (name?.split('.').pop() || '').trim().toLowerCase();
  }, []);

  const getFileBaseName = useCallback((name?: string) => {
    if (!name) {
      return '';
    }
    const index = name.lastIndexOf('.');
    return index > 0 ? name.slice(0, index) : name;
  }, []);

  const isMarkdownFile = useCallback(
    (item: ListDataItem) => {
      const ext = getFileExt(item.file_name || item.title || item.file_type);
      return ext === 'md' || ext === 'markdown' || ext === 'txt';
    },
    [getFileExt],
  );

  const buildAttachmentMeta = useCallback(
    (item: ListDataItem) => {
      const normalizedFileType = getFileExt(
        item.file_name || item.title || item.file_type,
      );
      return {
        file_key: item.file_key || item.id,
        file_name: item.file_name || item.title,
        file_type: normalizedFileType,
        // PDF 使用浏览器原生预览；其他格式不做预览转换，只保留原文件。
        ...(normalizedFileType === 'pdf'
          ? { preview_key: item.file_key || item.id }
          : {}),
      };
    },
    [getFileExt],
  );

  const {
    uploadErrorCount,
    parseErrorCount,
    importErrorCount,
    parsedCount,
    importedCount,
    loadingCount,
  } = useMemo(() => {
    return {
      uploadErrorCount: data.filter(item => item.status === 'upload-error')
        .length,
      parseErrorCount: data.filter(item => item.status === 'parse-error')
        .length,
      parsedCount: data.filter(item => item.status === 'parsed').length,
      importErrorCount: data.filter(item => item.status === 'import-error')
        .length,
      importedCount: data.filter(item => item.status === 'imported').length,
      loadingCount: data.filter(item =>
        ['uploading', 'parsing', 'importing'].includes(item.status),
      ).length,
    };
  }, [data]);

  const hasMarkdownFiles = useMemo(() => {
    return data.some(item => item.file && isMarkdownFile(item));
  }, [data, isMarkdownFile]);

  const hasNonMarkdownFiles = useMemo(() => {
    return data.some(
      item =>
        item.file &&
        !!(item.file_name || item.title || item.file_type) &&
        !isMarkdownFile(item),
    );
  }, [data, isMarkdownFile]);

  /**
   * 通用解析函数 - 用于解析文档
   * @param items 需要解析的文档列表
   * @param parseKey 解析时使用的 key 字段名，默认为 'title'
   */
  const handleParse = useCallback(
    async (items: ListDataItem[]) => {
      const itemUuids = items.map(item => item.uuid);

      // 将状态修改为 'parsing'
      setData(prev =>
        prev.map(item =>
          itemUuids.includes(item.uuid)
            ? { ...item, status: 'parsing', summary: '', progress: undefined }
            : item,
        ),
      );

      // 使用队列控制并发请求
      await Promise.all(
        items.map(item =>
          queue.enqueue(async () => {
            try {
              const resp = await postApiV1CrawlerParse({
                crawler_source: type,
                key: item.id!,
                kb_id,
                filename: item.file_type ? `file.${item.file_type}` : undefined,
              });

              const title =
                type === ConstsCrawlerSource.CrawlerSourceFile
                  ? item.title
                  : resp.docs?.value?.title || item.title;

              // 更新为解析成功状态
              setData(prev =>
                prev.map(prevItem =>
                  prevItem.uuid === item.uuid
                    ? {
                        ...prevItem,
                        platform_id: resp.id!,
                        id: resp.docs?.value?.id || '',
                        title,
                        import_kind: isFileImportType
                          ? 'document-with-attachment'
                          : prevItem.import_kind,
                        summary: isFileImportType
                          ? '将生成文件转换后的 Markdown 文档，并附带原始附件'
                          : resp.docs?.value?.summary || '',
                        status: 'parsed',
                      }
                    : prevItem,
                ),
              );
            } catch (error) {
              // 更新为错误状态
              setData(prev =>
                prev.map(prevItem =>
                  prevItem.uuid === item.uuid
                    ? {
                        ...prevItem,
                        status: 'parse-error',
                        summary:
                          error instanceof Error
                            ? error.message
                            : '操作失败，请稍后重试',
                      }
                    : prevItem,
                ),
              );
            }
          }),
        ),
      );
    },
    [setData, type, kb_id, queue],
  );

  const handleBatchImport = useCallback(async () => {
    // 步骤1: 将所有状态为 'parsed' 或 'import-error' 的数据修改为 'importing'
    let itemsToImport = data.filter(item =>
      ['parsed', 'import-error'].includes(item.status),
    );

    // 如果支持选择，则只处理选中的数据
    if (isSupportSelect) {
      itemsToImport = itemsToImport.filter(item => checked.includes(item.uuid));
    }

    // 过滤掉文件夹中 folderReq 为 false 的项
    itemsToImport = itemsToImport.filter(
      item => item.file || item.folderReq !== false,
    );

    if (itemsToImport.length === 0) {
      message.warning('请选择需要导入的文档');
      return;
    }

    const itemUuids = itemsToImport.map(item => item.uuid);

    setData(prev =>
      prev.map(item =>
        itemUuids.includes(item.uuid) &&
        ['parsed', 'import-error'].includes(item.status)
          ? { ...item, status: 'importing' }
          : item,
      ),
    );

    const idMapping = new Map<string, string>();

    for (const item of itemsToImport) {
      await queue.enqueue(async () => {
        try {
          let actualParentId: string | undefined = undefined;
          if (item.parent_id) {
            const mappedParentId = idMapping.get(item.parent_id);
            if (mappedParentId) {
              actualParentId = mappedParentId;
            } else {
              actualParentId = parent_id || undefined;
            }
          } else {
            actualParentId = parent_id || undefined;
          }

          if (!item.file) {
            const nodeResp = await postApiV1Node({
              name: item.title!,
              content: '',
              parent_id: actualParentId,
              type: 1, // 文件夹类型
              kb_id,
              nav_id: targetNavId || '',
            });

            const oldId = item.id!; // 保存原平台 ID
            const newId = nodeResp.id; // 新系统节点 ID

            // 更新映射表
            idMapping.set(oldId, newId);

            setData(prev =>
              prev.map(prevItem => {
                if (prevItem.uuid === item.uuid) {
                  return { ...prevItem, status: 'imported', id: newId };
                } else if (prevItem.parent_id === oldId) {
                  return { ...prevItem, parent_id: newId };
                }
                return prevItem;
              }),
            );
          } else {
            const markdown = isMarkdownFile(item);
            let nextContent = item.content || '';
            if (!markdown) {
              // 文件型文档保留原文件用于预览，同时提取一份内部 Markdown
              // 作为后续发布和智能问答的知识内容。
              try {
                const parseResp = await postApiV1CrawlerParse({
                  crawler_source: type,
                  key: item.file_key || item.id || '',
                  kb_id,
                  filename: item.file_name || item.title || 'file',
                });
                const exportResp = await postApiV1CrawlerExport({
                  kb_id,
                  id: parseResp.id || '',
                  doc_id: parseResp.docs?.value?.id || item.file_key || '',
                  file_type: item.file_type || '',
                });
                if (exportResp.task_id) {
                  const result = await pollCrawlerResults(exportResp.task_id);
                  nextContent = result.content || '';
                }
              } catch (error) {
                // 不阻止原文件导入；没有提取内容时仍可预览和下载。
                nextContent = '';
                console.warn('文件文本提取失败，文档仍将以只读文件导入', error);
              }
            }
            if (markdown && !nextContent.trim()) {
              throw new Error('未获取到 Markdown 内容');
            }
            const attachment = markdown
              ? undefined
              : await buildAttachmentMeta(item);
            const nodeResp = await postApiV1Node({
              name: item.title!,
              content: markdown ? nextContent : '',
              content_type: markdown ? 'md' : 'file',
              meta: attachment
                ? {
                    ...attachment,
                    // 文件型文档直接绑定 OSS 文件，不创建空白正文。
                    file_readonly: true,
                    content_type: 'file',
                  }
                : undefined,
              parent_id: actualParentId,
              type: 2,
              kb_id,
              nav_id: targetNavId || '',
            });

            setData(prev =>
              prev.map(prevItem =>
                prevItem.uuid === item.uuid
                  ? { ...prevItem, status: 'imported', id: nodeResp.id }
                  : prevItem,
              ),
            );
          }
        } catch (error) {
          setData(prev =>
            prev.map(prevItem =>
              prevItem.uuid === item.uuid
                ? {
                    ...prevItem,
                    status: 'import-error',
                    summary:
                      error instanceof Error
                        ? error.message
                        : item.file
                          ? '导入文件失败'
                          : '创建文件夹失败',
                  }
                : prevItem,
            ),
          );
        }
      });
    }
  }, [
    buildAttachmentMeta,
    checked,
    data,
    isMarkdownFile,
    isSupportSelect,
    kb_id,
    targetNavId,
    parent_id,
    queue,
    setData,
  ]);

  const handleBatchParse = useCallback(async () => {
    if (isFileImportType) {
      const failedUploads = data.filter(item => item.status === 'upload-error');
      if (failedUploads.length === 0) {
        message.warning('没有需要重试的文件');
        return;
      }
      setData(prev =>
        prev.map(item =>
          item.status === 'upload-error'
            ? { ...item, status: 'common', summary: '', progress: 0 }
            : item,
        ),
      );
      return;
    }

    // 筛选所有状态为 'parse-error' 的数据
    let itemsToParse = data.filter(item => item.status === 'parse-error');

    // 如果支持选择，则只处理选中的数据
    if (isSupportSelect) {
      itemsToParse = itemsToParse.filter(item => checked.includes(item.uuid));
    }

    if (itemsToParse.length === 0) {
      message.warning(
        isFileImportType ? '请选择需要重试的文件' : '请选择需要解析的文档',
      );
      return;
    }

    handleParse(itemsToParse);
  }, [checked, data, handleParse, isFileImportType, isSupportSelect, setData]);

  /**
   * 文件上传函数 - 上传文件到服务器
   * @param items 需要上传的文件列表
   */
  const handleUploadFile = useCallback(
    async (items: ListDataItem[]) => {
      const itemUuids = items.map(item => item.uuid);
      setData(prev =>
        prev.map(item =>
          itemUuids.includes(item.uuid)
            ? { ...item, status: 'uploading', summary: '', progress: 0 }
            : item,
        ),
      );
      const uploadedUuids: string[] = []; // 记录成功上传的文件 UUID
      const uploadedFileMap = new Map<
        string,
        { key: string; filename: string; fileType: string }
      >();

      // 批量上传文件
      await Promise.all(
        items.map(item =>
          queue.enqueue(async () => {
            try {
              if (!item.fileData) {
                throw new Error('找不到待上传的本地文件，请重新选择');
              }
              const originalType = getFileExt(item.fileData.name);
              if (
                originalType === 'md' ||
                originalType === 'markdown' ||
                originalType === 'txt'
              ) {
                const content = await item.fileData.text();
                if (!content.trim()) {
                  throw new Error('文本文件内容为空');
                }
                const title = getFileBaseName(item.fileData.name);
                setData(prev =>
                  prev.map(prevItem =>
                    prevItem.uuid === item.uuid
                      ? {
                          ...prevItem,
                          title,
                          file_name: item.fileData?.name,
                          file_type: originalType,
                          content,
                          import_kind: 'markdown',
                          summary: '将直接生成可编辑的文本文档',
                          status: 'parsed',
                          progress: undefined,
                        }
                      : prevItem,
                  ),
                );
                return;
              }

              // 上传文件并监听进度
              const formData = new FormData();
              formData.append('file', item.fileData);
              formData.append('kb_id', kb_id);
              const resp = await uploadAttachment(formData, {
                onUploadProgress: progressEvent => {
                  const percentCompleted = progressEvent.progress;

                  // 更新进度
                  setData(prev =>
                    prev.map(prevItem =>
                      prevItem.uuid === item.uuid
                        ? { ...prevItem, progress: percentCompleted }
                        : prevItem,
                    ),
                  );
                },
              });

              if (!resp.key || !resp.filename) {
                throw new Error('文件上传返回缺少关键信息');
              }

              // 上传成功，保存 key 和文件类型
              setData(prev =>
                prev.map(prevItem =>
                  prevItem.uuid === item.uuid
                    ? {
                        ...prevItem,
                        id: resp.key,
                        file_key: resp.key,
                        file_name: resp.filename,
                        file_type: resp.filename?.split('.').pop(),
                        progress: 100,
                      }
                    : prevItem,
                ),
              );

              // 记录上传成功的 UUID
              uploadedUuids.push(item.uuid);
              uploadedFileMap.set(item.uuid, {
                key: resp.key,
                filename: resp.filename,
                fileType: getFileExt(resp.filename),
              });
            } catch (error) {
              // 上传失败
              setData(prev =>
                prev.map(prevItem =>
                  prevItem.uuid === item.uuid
                    ? {
                        ...prevItem,
                        status: 'upload-error',
                        summary:
                          error instanceof Error
                            ? error.message
                            : '文件上传失败',
                        progress: undefined,
                      }
                    : prevItem,
                ),
              );
            }
          }),
        ),
      );

      // 文件上传完成后，直接在前端区分 markdown 与附件
      if (uploadedUuids.length > 0) {
        await Promise.all(
          uploadedUuids.map(async uuid => {
            const uploadedItem = items.find(item => item.uuid === uuid);
            const uploadedInfo = uploadedFileMap.get(uuid);
            if (!uploadedItem?.fileData) {
              return;
            }
            const fileName =
              uploadedInfo?.filename || uploadedItem.fileData.name;
            const title = getFileBaseName(fileName);
            const fileType = uploadedInfo?.fileType || getFileExt(fileName);
            setData(prev =>
              prev.map(prevItem =>
                prevItem.uuid === uuid
                  ? {
                      ...prevItem,
                      title: prevItem.title || title,
                      file_name: prevItem.file_name || fileName,
                      file_type: prevItem.file_type || fileType,
                      import_kind: 'file-document',
                      summary: '文件已上传，将保留为只读原文件',
                      status: 'parsed',
                      progress: undefined,
                    }
                  : prevItem,
              ),
            );
          }),
        );
      }
    },
    [getFileBaseName, getFileExt, queue, setData],
  );

  const handleToggleSelectAll = useCallback(() => {
    const canSelectData = data.filter(item => item.folderReq);
    setChecked(prev => {
      if (prev.length === canSelectData.length && canSelectData.length > 0) {
        return [];
      }
      return canSelectData.map(item => item.uuid);
    });
  }, [data, setChecked]);

  // 计算全选状态
  const isAllChecked = useMemo(() => {
    return data.length > 0 && checked.length === data.length;
  }, [data.length, checked.length]);

  // 计算半选状态
  const isIndeterminate = useMemo(() => {
    return checked.length > 0 && checked.length < data.length;
  }, [data.length, checked.length]);

  // 监听新文件，自动触发上传
  useEffect(() => {
    if (data.length > 0) {
      const unUploadData = data.filter(item => item.status === 'common');

      if (unUploadData.length > 0) {
        handleUploadFile(unUploadData);
      }
    }
  }, [data, handleUploadFile]);

  // 文件上传完成后自动创建文档，不再要求用户手动点击“批量导入”。
  useEffect(() => {
    if (!isFileImportType) return;
    if (data.some(item => item.status === 'importing')) return;
    if (!data.some(item => item.status === 'parsed')) return;
    void handleBatchImport();
  }, [data, handleBatchImport, isFileImportType]);

  return (
    <Stack
      gap={1}
      sx={{
        p: 1,
        pr: 2,
        borderBottom: '1px solid',
        borderColor: 'divider',
        position: 'sticky',
        top: 0,
        bgcolor: 'background.default',
        zIndex: 1,
      }}
    >
      {isFileImportType && (hasMarkdownFiles || hasNonMarkdownFiles) && (
        <Box
          sx={{
            px: 1.5,
            py: 1,
            borderRadius: 1,
            fontSize: 12,
            color: 'text.secondary',
            bgcolor: alpha(theme.palette.info.main, 0.08),
          }}
        >
          文件上传后会直接创建对应文档：Markdown/TXT
          保留为可编辑文档，其他格式保留为只读原文件；PDF
          可直接预览，其他格式不生成预览。
        </Box>
      )}
      <Stack
        direction='row'
        alignItems='center'
        justifyContent={'space-between'}
        gap='10px'
      >
        <Stack
          direction='row'
          gap={1}
          alignItems='center'
          sx={{ lineHeight: 1 }}
        >
          {isSupportSelect && (
            <Stack
              direction='row'
              gap={1}
              alignItems='center'
              sx={{
                fontSize: 14,
                cursor: 'pointer',
                userSelect: 'none',
              }}
              onClick={handleToggleSelectAll}
            >
              <Checkbox
                aria-labelledby='checked-all-label'
                edge='start'
                size='small'
                checked={isAllChecked}
                indeterminate={isIndeterminate}
                tabIndex={-1}
                disableRipple
                sx={{ ml: '2px' }}
                inputProps={{ 'aria-labelledby': 'checked-all-label' }}
              />
              <Box>全选</Box>
            </Stack>
          )}
          {importedCount > 0 ? (
            <Box
              sx={{
                fontSize: 12,
                color: 'success.main',
                bgcolor: alpha(theme.palette.success.main, 0.1),
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              导入成功：{importedCount}{' '}
              {data.length - importedCount > 0 && <> / {data.length}</>}
            </Box>
          ) : (
            <Box
              sx={{
                fontSize: 12,
                color: 'text.disabled',
                bgcolor: 'background.paper2',
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              未导入：{data.length - importedCount}
            </Box>
          )}
          {parseErrorCount > 0 && (
            <Box
              sx={{
                fontSize: 12,
                color: 'error.main',
                bgcolor: alpha(theme.palette.error.main, 0.1),
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              {isFileImportType ? '处理失败' : '解析失败'}：{parseErrorCount}
            </Box>
          )}
          {uploadErrorCount > 0 && (
            <Box
              sx={{
                fontSize: 12,
                color: 'error.main',
                bgcolor: alpha(theme.palette.error.main, 0.1),
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              上传失败：{uploadErrorCount}
            </Box>
          )}
          {importErrorCount > 0 && (
            <Box
              sx={{
                fontSize: 12,
                color: 'error.main',
                bgcolor: alpha(theme.palette.error.main, 0.1),
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              导入失败：{importErrorCount}
            </Box>
          )}
          {loadingCount > 0 && (
            <Stack
              direction='row'
              gap={1}
              alignItems='center'
              sx={{
                fontSize: 12,
                color: 'warning.main',
                bgcolor: alpha(theme.palette.warning.main, 0.1),
                px: 1,
                py: 0.5,
                borderRadius: 1,
              }}
            >
              <CircularProgress
                size={12}
                sx={{ color: theme.palette.warning.main }}
              />
              处理中：{loadingCount}
            </Stack>
          )}
        </Stack>
        <Stack direction='row' gap={2} alignItems='center'>
          {!isFileImportType && (
            <Button
              size='small'
              color='primary'
              disabled={
                (isFileImportType
                  ? uploadErrorCount === 0
                  : parseErrorCount === 0) || loading
              }
              sx={{ minWidth: 0, p: 0, color: 'primary.main' }}
              onClick={handleBatchParse}
            >
              {isFileImportType ? '批量重试' : '批量解析'}
            </Button>
          )}
          <Button
            size='small'
            color='primary'
            disabled={
              isFileImportType ||
              !(parsedCount > 0 || importErrorCount > 0) ||
              loading
            }
            onClick={handleBatchImport}
            sx={{ minWidth: 0, p: 0, color: 'primary.main' }}
          >
            {parsedCount === 0 && importErrorCount > 0
              ? '重试导入'
              : '批量导入'}
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
};

export default BatchActionBar;
