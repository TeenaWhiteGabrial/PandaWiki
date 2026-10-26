import { message } from '@/components/ui';
import {
  getApiV1FileStorageInfo,
  FileStorageInfo as FileStorageInfoData,
} from '@/request/File';
import { useAppSelector } from '@/store';
import {
  Alert,
  ButtonBase,
  Box,
  Button,
  Chip,
  Collapse,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useId, useRef, useState } from 'react';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';

interface FileStorageInfoProps {
  fileKey?: string;
}

const FileStorageInfo = ({ fileKey }: FileStorageInfoProps) => {
  const { kb_id: kbId = '' } = useAppSelector(state => state.config);
  const [info, setInfo] = useState<FileStorageInfoData | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const contentId = useId();
  const requestId = useRef(0);

  const load = useCallback(async () => {
    const currentRequestId = ++requestId.current;
    if (!fileKey || !kbId) {
      setInfo(null);
      setError('');
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    setInfo(null);
    try {
      const result = await getApiV1FileStorageInfo({
        kb_id: kbId,
        key: fileKey,
      });
      if (currentRequestId === requestId.current) setInfo(result);
    } catch (loadError) {
      if (currentRequestId === requestId.current) {
        setInfo(null);
        setError(
          loadError instanceof Error
            ? loadError.message
            : '无法读取文件的对象存储信息',
        );
      }
    } finally {
      if (currentRequestId === requestId.current) setLoading(false);
    }
  }, [fileKey, kbId]);

  useEffect(() => {
    if (!expanded) {
      requestId.current += 1;
      setLoading(false);
      return;
    }
    void load();
    return () => {
      requestId.current += 1;
    };
  }, [expanded, load]);

  const copyValue = async (value: string, label: string) => {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      message.success(`${label}已复制`);
    } catch {
      message.error('复制失败，请手动复制');
    }
  };

  if (!fileKey) return null;

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        bgcolor: 'background.paper',
        overflow: 'hidden',
      }}
    >
      <ButtonBase
        aria-expanded={expanded}
        aria-controls={contentId}
        onClick={() => setExpanded(value => !value)}
        sx={{
          width: '100%',
          px: 2,
          py: 1.5,
          textAlign: 'left',
          justifyContent: 'space-between',
        }}
      >
        <Typography variant='body2' fontWeight={600}>
          文件存储信息
        </Typography>
        <Stack direction='row' alignItems='center' spacing={0.5}>
          <Typography variant='caption' color='text.secondary'>
            {expanded ? '收起' : '点击查看'}
          </Typography>
          <ExpandMoreIcon
            fontSize='small'
            sx={{
              transform: expanded ? 'rotate(180deg)' : 'none',
              transition: theme => theme.transitions.create('transform'),
            }}
          />
        </Stack>
      </ButtonBase>
      <Collapse in={expanded} unmountOnExit>
        <Box id={contentId} sx={{ px: 2, pb: 2 }}>
          <Stack spacing={1.25}>
            <Stack direction='row' alignItems='center' spacing={1}>
              {info && (
                <Chip
                  size='small'
                  color={
                    !info.available
                      ? 'warning'
                      : info.storage_type === 'oss'
                        ? 'primary'
                        : 'default'
                  }
                  label={
                    info.available
                      ? info.storage_type === 'oss'
                        ? 'OSS'
                        : 'MinIO'
                      : info.active_storage_type === 'oss'
                        ? '非 OSS'
                        : '非 MinIO'
                  }
                />
              )}
              {loading && (
                <Typography variant='caption' color='text.secondary'>
                  正在查询…
                </Typography>
              )}
            </Stack>

            {error && (
              <Alert
                severity='warning'
                action={
                  <Button
                    color='inherit'
                    size='small'
                    onClick={() => void load()}
                  >
                    重试
                  </Button>
                }
              >
                {error}
              </Alert>
            )}

            {info && (
              <>
                <TextField
                  label='对象 Key'
                  size='small'
                  value={info.object_key}
                  fullWidth
                  InputProps={{ readOnly: true }}
                />
                {info.available ? (
                  <>
                    <TextField
                      label={
                        info.object_url
                          ? `${info.storage_type === 'oss' ? 'OSS' : 'MinIO'} 访问地址`
                          : `${info.storage_type === 'oss' ? 'OSS' : 'MinIO'} 对象位置（桶 / Key）`
                      }
                      size='small'
                      value={
                        info.object_url ||
                        info.object_location ||
                        info.object_key
                      }
                      fullWidth
                      InputProps={{ readOnly: true }}
                    />
                    <Stack direction='row' spacing={1}>
                      <Button
                        size='small'
                        variant='outlined'
                        onClick={() =>
                          void copyValue(
                            info.object_url ||
                              info.object_location ||
                              info.object_key,
                            '对象位置',
                          )
                        }
                      >
                        复制对象位置
                      </Button>
                      {info.object_url ? (
                        <Button
                          size='small'
                          variant='outlined'
                          component='a'
                          href={info.object_url}
                          target='_blank'
                          rel='noreferrer'
                        >
                          打开访问地址
                        </Button>
                      ) : null}
                    </Stack>
                    {!info.object_url ? (
                      <Typography variant='caption' color='text.secondary'>
                        未配置公开访问域名，已显示存储桶和对象路径。
                      </Typography>
                    ) : null}
                    <TextField
                      label='临时访问链接（30 分钟有效）'
                      size='small'
                      value={info.url}
                      fullWidth
                      InputProps={{ readOnly: true }}
                    />
                    <Stack direction='row' spacing={1}>
                      <Button
                        size='small'
                        variant='outlined'
                        onClick={() => void copyValue(info.url, '临时访问链接')}
                      >
                        复制链接
                      </Button>
                      <Button
                        size='small'
                        variant='outlined'
                        component='a'
                        href={info.url}
                        target='_blank'
                        rel='noreferrer'
                      >
                        打开链接
                      </Button>
                    </Stack>
                  </>
                ) : (
                  <Alert severity='info'>
                    当前启用的
                    {info.active_storage_type === 'oss' ? ' OSS' : ' MinIO'}
                    中未找到此文件，已标记为“非
                    OSS”。系统不会从未启用的存储读取文件。
                  </Alert>
                )}
              </>
            )}
          </Stack>
        </Box>
      </Collapse>
    </Box>
  );
};

export default FileStorageInfo;
