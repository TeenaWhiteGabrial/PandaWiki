import Card from '@/components/Card';
import { getApiV1NodeStats } from '@/request/Node';
import { useAppSelector } from '@/store';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import {
  Box,
  Button,
  ButtonBase,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
} from '@mui/material';
import CloudUploadOutlinedIcon from '@mui/icons-material/CloudUploadOutlined';
import AccountTreeOutlinedIcon from '@mui/icons-material/AccountTreeOutlined';
import ViewListOutlinedIcon from '@mui/icons-material/ViewListOutlined';
import ManageSearchOutlinedIcon from '@mui/icons-material/ManageSearchOutlined';
import { useCallback, useEffect, useState } from 'react';
import DocSearch from './DocSearch';

interface DocPageHeaderProps {
  onPublishClick: () => void;
  onRestudyClick?: () => void;
  /** 变更时触发重新拉取统计 */
  refreshTrigger?: number;
  viewMode?: 'tree' | 'explorer';
  onViewModeChange?: (mode: 'tree' | 'explorer') => void;
  onUploadTaskClick?: () => void;
  onLearningProgressClick?: () => void;
}

const DocPageHeader = ({
  onPublishClick,
  onRestudyClick,
  refreshTrigger,
  viewMode = 'tree',
  onViewModeChange,
  onUploadTaskClick,
  onLearningProgressClick,
}: DocPageHeaderProps) => {
  const { kb_id, isRefreshDocList, user, kbDetail } = useAppSelector(
    state => state.config,
  );
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);
  const [stats, setStats] = useState({
    unreleased_nav_count: 0,
    unpublished_count: 0,
    unstudied_count: 0,
  });

  const getStats = useCallback(() => {
    if (!kb_id) return;
    getApiV1NodeStats({ kb_id }).then(res => {
      if (res) {
        setStats({
          unreleased_nav_count: res.unreleased_nav_count ?? 0,
          unpublished_count: res.unpublished_count ?? 0,
          unstudied_count: res.unstudied_count ?? 0,
        });
      }
    });
  }, [kb_id]);

  useEffect(() => {
    if (kb_id) getStats();
  }, [kb_id, getStats]);

  useEffect(() => {
    if (isRefreshDocList) getStats();
  }, [isRefreshDocList, getStats]);

  useEffect(() => {
    if (refreshTrigger !== undefined && refreshTrigger > 0) getStats();
  }, [refreshTrigger, getStats]);

  const hasPublishPending =
    stats.unpublished_count > 0 || stats.unreleased_nav_count > 0;
  const hasIndexPending = stats.unstudied_count > 0;

  return (
    <Card>
      <Stack
        direction={'row'}
        alignItems={'center'}
        justifyContent={'space-between'}
        sx={{ p: 2 }}
      >
        <Stack
          direction={'row'}
          alignItems={'center'}
          gap={0}
          sx={{ fontSize: 16, fontWeight: 700 }}
        >
          <Box>目录</Box>
          {!readOnlyUser && (hasPublishPending || hasIndexPending) && (
            <>
              <Stack
                direction={'row'}
                alignItems={'center'}
                gap={0}
                sx={{ ml: 2 }}
              >
                {stats.unreleased_nav_count > 0 && (
                  <Box
                    sx={{
                      color: 'error.main',
                      fontSize: 12,
                      fontWeight: 'normal',
                    }}
                  >
                    {stats.unreleased_nav_count} 个 目录未发布，
                  </Box>
                )}
                {stats.unpublished_count > 0 && (
                  <Box
                    sx={{
                      color: 'error.main',
                      fontSize: 12,
                      fontWeight: 'normal',
                    }}
                  >
                    {stats.unpublished_count} 个 文档/文件夹未发布，
                  </Box>
                )}
                {hasIndexPending && (
                  <Box
                    sx={{
                      color: 'warning.main',
                      fontSize: 12,
                      fontWeight: 'normal',
                    }}
                  >
                    {stats.unstudied_count} 个 文档问答索引待更新，
                  </Box>
                )}
              </Stack>
              {hasPublishPending && (
                <ButtonBase
                  disableRipple
                  sx={{
                    fontSize: 12,
                    fontWeight: 400,
                    color: 'primary.main',
                  }}
                  onClick={onPublishClick}
                >
                  去发布
                </ButtonBase>
              )}
              {hasIndexPending && (
                <ButtonBase
                  disableRipple
                  sx={{
                    ml: hasPublishPending ? 1 : 0,
                    fontSize: 12,
                    fontWeight: 400,
                    color: 'primary.main',
                  }}
                  onClick={onRestudyClick}
                >
                  去重建索引
                </ButtonBase>
              )}
            </>
          )}
        </Stack>
        <Stack direction='row' alignItems='center' gap={1.5}>
          {!readOnlyUser && (
            <>
              <Button
                size='small'
                startIcon={<CloudUploadOutlinedIcon />}
                onClick={onUploadTaskClick}
              >
                上传任务
              </Button>
              <Button
                size='small'
                startIcon={<ManageSearchOutlinedIcon />}
                onClick={onLearningProgressClick}
              >
                后台任务
              </Button>
            </>
          )}
          <ToggleButtonGroup
            size='small'
            exclusive
            value={viewMode}
            onChange={(_, value: 'tree' | 'explorer' | null) => {
              if (value) onViewModeChange?.(value);
            }}
            aria-label='左侧内容展示模式'
          >
            <ToggleButton value='tree' aria-label='目录树模式'>
              <AccountTreeOutlinedIcon fontSize='small' />
            </ToggleButton>
            <ToggleButton value='explorer' aria-label='资源管理器模式'>
              <ViewListOutlinedIcon fontSize='small' />
            </ToggleButton>
          </ToggleButtonGroup>
          <DocSearch />
        </Stack>
      </Stack>
    </Card>
  );
};

export default DocPageHeader;
