import { Modal } from '@/components/ui';
import type { Department } from '@/request/Enterprise';
import {
  previewReadingAudience,
  type ReadingAudienceResult,
} from '@/request/ReadingAudience';
import type { DomainNodePermissions } from '@/request/types';
import { useAppDispatch } from '@/store';
import { setNavId } from '@/store/slices/config';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import { Input, Pagination, Table } from 'antd';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { ReadingPermissionSection } from './PermissionSettingsSections';
import ReadingPermissionSummary from './ReadingPermissionSummary';

export default function ReadingPermissionPanel({
  open,
  ready,
  kbID,
  ids,
  targetType = 'node',
  canEdit,
  dirty,
  invalid,
  draft,
  saved,
  usingParent,
  levels,
  departments,
  users,
  levelsDisabled,
  onLocate,
  children,
}: {
  open: boolean;
  ready: boolean;
  kbID: string;
  ids: string[];
  targetType?: 'node' | 'nav';
  canEdit: boolean;
  dirty: boolean;
  invalid?: boolean;
  draft: DomainNodePermissions;
  saved: DomainNodePermissions;
  usingParent?: boolean;
  levels: { id: string; name: string; rank: number }[];
  departments: Department[];
  users: { id: string; account: string }[];
  levelsDisabled?: boolean;
  onLocate: () => void;
  children: ReactNode;
}) {
  const [editing, setEditing] = useState(false);
  const [audienceOpen, setAudienceOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ReadingAudienceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const previousPolicy = useRef('');
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const identity = `${targetType}:${ids.join(',')}`;
  const policyKey = JSON.stringify(dirty ? draft : null);
  const requestKey = JSON.stringify({
    kb_id: kbID,
    ids,
    target_type: targetType,
    ...(dirty ? { permissions: draft } : {}),
    include_users: audienceOpen,
    keyword: audienceOpen ? keyword : '',
    page: audienceOpen ? page : 1,
    page_size: 20,
  });
  useEffect(() => {
    setEditing(false);
    setAudienceOpen(false);
    setKeyword('');
    setPage(1);
    setResult(null);
    previousPolicy.current = '';
  }, [open, identity]);
  useEffect(() => {
    setPage(1);
  }, [policyKey]);
  useEffect(() => {
    if (!open || !ready || !canEdit || !kbID || !ids.length || invalid) return;
    let active = true;
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    // Avoid keeping an old policy's count/summary visible while a new preview
    // is being calculated, including during the debounce interval.
    const previewIdentity = `${kbID}:${identity}:${policyKey}`;
    if (previousPolicy.current !== previewIdentity) {
      setResult(null);
      previousPolicy.current = previewIdentity;
    }
    const timeout = setTimeout(
      () => {
        previewReadingAudience(JSON.parse(requestKey), controller.signal)
          .then(response => {
            if (active) setResult(response);
          })
          .catch(() => {
            if (active && !controller.signal.aborted) setError(true);
          })
          .finally(() => {
            if (active) setLoading(false);
          });
      },
      dirty || keyword ? 300 : 0,
    );
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [open, ready, canEdit, kbID, identity, requestKey, invalid, attempt]);
  const pending = !ready || (canEdit && !result && !error);
  const source = result?.source;
  const locateSource = () => {
    if (!source) return;
    dispatch(setNavId(source.nav_id || source.id));
    navigate(
      source.target_type === 'node'
        ? `/doc/editor/${source.id}`
        : '/doc/editor',
    );
    onLocate();
  };
  const departmentName = (departmentIDs?: string[]) => {
    const selected = departments.filter(item =>
      departmentIDs?.includes(item.id),
    );
    // Stored reader contexts include ancestor assignments. Show a department
    // path once, rather than presenting ancestors as parallel memberships.
    return (
      selected
        .filter(
          item =>
            !selected.some(
              child =>
                child.id !== item.id && child.path.startsWith(`${item.path}/`),
            ),
        )
        .map(item => {
          return (
            departments
              .filter(
                parent =>
                  item.path === parent.path ||
                  item.path.startsWith(`${parent.path}/`),
              )
              .sort((a, b) => a.path.length - b.path.length)
              .map(parent => parent.name)
              .join(' / ') || item.name
          );
        })
        .join('、') || '未分配'
    );
  };
  return (
    <>
      <ReadingPermissionSection
        action={
          canEdit && (
            <Button
              size='small'
              disabled={!ready}
              onClick={() => setEditing(value => !value)}
            >
              {editing ? '收起设置' : '修改'}
            </Button>
          )
        }
      >
        {(usingParent || dirty) && (
          <Stack direction='row' gap={1} flexWrap='wrap' alignItems='center'>
            {usingParent && (
              <Tooltip
                title={
                  source ? `来源：${source.name}，点击定位` : '使用上级设置'
                }
              >
                <Chip
                  size='small'
                  label='使用上级设置'
                  variant='outlined'
                  onClick={source ? locateSource : undefined}
                />
              </Tooltip>
            )}
            {dirty && (
              <Chip
                size='small'
                color='warning'
                variant='outlined'
                label='未保存预览'
              />
            )}
          </Stack>
        )}
        {invalid ? (
          <Alert severity='warning'>请完成下方查阅范围配置后查看预览。</Alert>
        ) : pending ? (
          <Stack direction='row' gap={1} alignItems='center'>
            <CircularProgress size={16} />
            <Typography fontSize={13} color='text.secondary'>
              正在计算查阅范围…
            </Typography>
          </Stack>
        ) : error && !result ? (
          <Alert
            severity='error'
            action={
              <Button
                size='small'
                onClick={() => setAttempt(value => value + 1)}
              >
                重试
              </Button>
            }
          >
            查阅范围计算失败，暂时无法确认实际名单。
          </Alert>
        ) : (
          <>
            <ReadingPermissionSummary
              permissions={result?.permissions || saved}
              summary={result?.summary}
              levels={levels}
              departments={departments}
              users={users}
              levelsDisabled={levelsDisabled}
            />
            {result && (
              <Stack
                direction='row'
                justifyContent='space-between'
                alignItems='center'
                sx={{
                  borderTop: '1px solid',
                  borderColor: 'divider',
                  pt: 1.25,
                }}
              >
                <Typography
                  fontSize={13}
                  color={
                    result.total === 0 && !result.unrestricted
                      ? 'warning.main'
                      : 'text.secondary'
                  }
                >
                  {ids.length > 1 ? '可查阅全部所选内容：' : '当前可查阅：'}
                  <Box
                    component='span'
                    sx={{ color: 'text.primary', fontWeight: 600 }}
                  >
                    {result.total}
                  </Box>{' '}
                  人
                </Typography>
                <Button
                  size='small'
                  onClick={() => {
                    setPage(1);
                    setKeyword('');
                    setAudienceOpen(true);
                  }}
                >
                  查看名单
                </Button>
              </Stack>
            )}
            {result?.total === 0 && !result.unrestricted && (
              <Alert severity='warning'>当前没有用户可以查阅。</Alert>
            )}
          </>
        )}
        {editing && (
          <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
            {children}
          </Box>
        )}
      </ReadingPermissionSection>
      <Modal
        title={dirty ? '可查阅名单 · 未保存预览' : '可查阅名单'}
        open={open && audienceOpen}
        onCancel={() => setAudienceOpen(false)}
        footer={null}
        width={760}
      >
        <Stack gap={2}>
          <Input.Search
            allowClear
            placeholder='搜索账号或姓名'
            value={keyword}
            onChange={event => {
              setKeyword(event.target.value);
              setPage(1);
            }}
          />
          <Table
            size='small'
            rowKey='id'
            loading={loading || pending}
            dataSource={result?.users || []}
            pagination={false}
            scroll={{ y: 340 }}
            locale={{
              emptyText: error
                ? '名单加载失败'
                : keyword
                  ? '没有匹配的用户'
                  : '当前没有用户可以查阅',
            }}
            columns={[
              { title: '账号', dataIndex: 'account', width: 150 },
              {
                title: '姓名',
                dataIndex: 'name',
                width: 120,
                render: name => name || '—',
              },
              {
                title: '所属部门',
                key: 'departments',
                render: (_, user) => departmentName(user.department_ids),
              },
              ...(!levelsDisabled
                ? [
                    {
                      title: '用户等级',
                      key: 'level',
                      width: 110,
                      render: (
                        _: unknown,
                        user: { permission_level_id?: string },
                      ) =>
                        levels.find(
                          level => level.id === user.permission_level_id,
                        )?.name || '未分配',
                    },
                  ]
                : []),
            ]}
          />
          {error && (
            <Alert
              severity='error'
              action={
                <Button
                  size='small'
                  onClick={() => setAttempt(value => value + 1)}
                >
                  重试
                </Button>
              }
            >
              名单加载失败，请重试。
            </Alert>
          )}
          <Stack
            direction='row'
            gap={1}
            alignItems='center'
            justifyContent='space-between'
          >
            <Typography fontSize={12} color='text.secondary'>
              共 {result?.filtered_total || 0} 人 · 统计当前系统账号
              {ids.length > 1 ? '，需能查阅全部所选内容' : ''}
            </Typography>
            <Pagination
              hideOnSinglePage
              simple
              current={page}
              pageSize={20}
              total={result?.filtered_total || 0}
              onChange={setPage}
              showSizeChanger={false}
            />
          </Stack>
        </Stack>
      </Modal>
    </>
  );
}
