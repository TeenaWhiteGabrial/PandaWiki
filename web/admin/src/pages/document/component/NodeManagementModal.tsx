import { Modal, message } from '@/components/ui';
import { Department, PermissionLevel } from '@/request/Enterprise';
import {
  editNodeManagement,
  editNavManagement,
  getNodeManagement,
  getNavManagement,
  getNodePermissionOptions,
} from '@/request/NodeManagement';
import { DomainDocumentManagementGrant } from '@/request/types';
import {
  Alert,
  Button,
  CircularProgress,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  PermissionDialogTabs,
  PermissionObjectName,
  PermissionHelp,
  permissionTableSx,
} from './PermissionDialogLayout';
import { Select } from 'antd';
import { isAdministrator } from '@/utils/permission';
import { useEffect, useMemo, useState } from 'react';

const subjectLabels = {
  permission_level: '用户等级',
  user: '指定管理员',
  department: '部门',
};
function expandDepartments(
  ids: string[] | undefined,
  departments: Department[],
) {
  const expanded = new Set(ids || []);
  for (const id of ids || []) {
    const assigned = departments.find(item => item.id === id);
    if (!assigned) continue;
    for (const department of departments) {
      if (
        assigned.path === department.path ||
        assigned.path.startsWith(`${department.path}/`)
      ) {
        expanded.add(department.id);
      }
    }
  }
  return expanded;
}

function userMatchesGrant(
  grant: DomainDocumentManagementGrant,
  user: {
    id: string;
    permission_level_id?: string;
    department_ids?: string[];
  },
  departments: Department[],
) {
  if (grant.subject_type === 'user') return grant.subject_ids.includes(user.id);
  if (grant.subject_type === 'permission_level') {
    return (
      !!user.permission_level_id &&
      grant.subject_ids.includes(user.permission_level_id)
    );
  }
  const departmentIDs = expandDepartments(user.department_ids, departments);
  return grant.subject_ids.some(id => departmentIDs.has(id));
}

function describeGrant(
  grant: DomainDocumentManagementGrant,
  users: { id: string; name: string }[],
  departments: Department[],
  levels: PermissionLevel[],
) {
  const options =
    grant.subject_type === 'user'
      ? users
      : grant.subject_type === 'department'
        ? departments
        : levels;
  const names = grant.subject_ids
    .map(id => options.find(item => item.id === id)?.name || `对象ID ${id}`)
    .join('、');
  return grant.subject_type === 'user'
    ? names || '未选择管理员'
    : `${subjectLabels[grant.subject_type]}：${names || '未配置授权对象'}`;
}

function getRequestErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  if (!error || typeof error !== 'object') return '';

  const value = error as {
    message?: unknown;
    data?: { message?: unknown };
    response?: { data?: { message?: unknown }; statusText?: unknown };
  };
  if (typeof value.message === 'string') return value.message;
  if (typeof value.data?.message === 'string') return value.data.message;
  if (typeof value.response?.data?.message === 'string') {
    return value.response.data.message;
  }
  if (typeof value.response?.statusText === 'string') {
    return value.response.statusText;
  }
  return '';
}

export default function NodeManagementModal({
  open,
  kbID,
  nodeIDs,
  targetType = 'node',
  targetNames,
  onBack,
  onClose,
  onSuccess,
}: {
  open: boolean;
  kbID: string;
  nodeIDs: string[];
  targetType?: 'node' | 'nav';
  targetNames?: string[];
  onBack?: () => void;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [grants, setGrants] = useState<DomainDocumentManagementGrant[]>([]);
  const [sources, setSources] = useState<
    { id: string; name: string; grants: DomainDocumentManagementGrant[] }[]
  >([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const [users, setUsers] = useState<
    {
      id: string;
      name: string;
      permission_level_id?: string;
      department_ids?: string[];
    }[]
  >([]);
  const [loading, setLoading] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [canConfigure, setCanConfigure] = useState(false);
  const [documentOnly, setDocumentOnly] = useState(false);
  const nodeKey = nodeIDs.join(',');

  useEffect(() => {
    if (!open || !kbID || !nodeIDs.length) return;
    let active = true;
    setLoading(true);
    setDirty(false);
    setReady(false);
    setCanConfigure(false);
    setLoadError('');
    setGrants([]);
    setSources([]);
    setUsers([]);
    setDocumentOnly(false);
    const get = targetType === 'nav' ? getNavManagement : getNodeManagement;
    Promise.all(nodeIDs.map(id => get({ kb_id: kbID, id })))
      .then(async responses => {
        if (!active) return;
        setCanConfigure(responses.every(item => item.can_configure));
        setDocumentOnly(responses.some(item => item.node_type === 2));
        setGrants(responses[0].grants || []);
        // A batch can have different ancestors: only display a final audience
        // for an individual target, never present their union as a shared scope.
        setSources(
          responses.length === 1 ? responses[0].inherited_sources || [] : [],
        );
        const options = await getNodePermissionOptions(kbID);
        if (!active) return;
        if (!Array.isArray(options?.management_users))
          throw new Error('管理员列表未加载');
        setUsers(
          options.management_users
            .filter(user => isAdministrator(user.role))
            .map(user => ({
              id: user.id,
              name:
                user.name && user.name !== user.account
                  ? `${user.name}（${user.account}）`
                  : user.account,
              permission_level_id: user.permission_level_id,
              department_ids: user.department_ids,
            })),
        );
        setDepartments(options.departments || []);
        setLevels(options.levels || []);
        setReady(true);
      })
      .catch(error => {
        if (active) setLoadError(getRequestErrorMessage(error) || '请稍后重试');
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [open, kbID, nodeKey, targetType, attempt]);

  const effectiveUsers = useMemo(() => {
    const all = [...sources.flatMap(source => source.grants), ...grants];
    return users.filter(user =>
      all.some(grant => userMatchesGrant(grant, user, departments)),
    );
  }, [sources, grants, users, departments]);
  const invalid = grants.some(
    grant =>
      !grant.subject_ids.length ||
      (grant.subject_type === 'user' &&
        grant.subject_ids.some(id => !users.some(user => user.id === id))),
  );
  const updateGrants = (value: DomainDocumentManagementGrant[]) => {
    setGrants(value);
    setDirty(true);
  };
  const save = async () => {
    if (!dirty) {
      onClose();
      return;
    }
    if (!ready || !canConfigure || invalid) return;
    setLoading(true);
    try {
      if (targetType === 'nav')
        await editNavManagement({ kb_id: kbID, id: nodeIDs[0], grants });
      else await editNodeManagement({ kb_id: kbID, ids: nodeIDs, grants });
      message.success('文档管理权限已保存');
      onClose();
      onSuccess();
    } finally {
      setLoading(false);
    }
  };
  const inheritedRows = sources.flatMap(source =>
    source.grants.map((grant, index) => ({ source, grant, index })),
  );
  return (
    <Modal
      open={open}
      title='管理权限'
      width={800}
      onCancel={onClose}
      onOk={save}
      okText={dirty ? '保存' : '关闭'}
      okButtonProps={{
        loading,
        disabled: loading || (dirty && (!ready || !canConfigure || invalid)),
      }}
    >
      <Stack gap={2} sx={{ maxHeight: '70vh', overflowY: 'auto' }}>
        {!!targetNames?.length && <PermissionObjectName names={targetNames} />}
        {onBack && (
          <PermissionDialogTabs
            value='management'
            onReading={onBack}
            onManagement={() => {}}
            readingDisabled={dirty}
          />
        )}
        {loading && !ready ? (
          <Stack alignItems='center' py={5}>
            <CircularProgress size={24} />
          </Stack>
        ) : (
          <>
            {loadError && (
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
                {loadError}
              </Alert>
            )}
            {nodeIDs.length > 1 && (
              <Typography fontSize={12} color='warning.main'>
                保存将替换 {nodeIDs.length}{' '}
                项的本级授权，当前显示第一项；各自的上级授权继续生效。
              </Typography>
            )}
            <Stack
              direction='row'
              alignItems='center'
              justifyContent='space-between'
            >
              <Typography fontSize={13}>
                {documentOnly ? '使用所属目录的管理授权' : '管理员授权'}
              </Typography>
              <PermissionHelp management />
            </Stack>
            <TableContainer
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
              }}
            >
              <Table
                size='small'
                sx={permissionTableSx}
                aria-label='管理授权列表'
              >
                <TableHead>
                  <TableRow>
                    <TableCell>管理员</TableCell>
                    <TableCell sx={{ width: 145 }}>授权来源</TableCell>
                    <TableCell sx={{ width: 135 }}>生效范围</TableCell>
                    <TableCell align='right' sx={{ width: 65 }}>
                      操作
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {inheritedRows.map(({ source, grant, index }) => (
                    <TableRow key={`${source.id}:${index}`}>
                      <TableCell>
                        {describeGrant(grant, users, departments, levels)}
                      </TableCell>
                      <TableCell>
                        <Typography fontSize={13}>
                          {source.name || source.id}
                        </Typography>
                        <Typography fontSize={11} color='text.secondary'>
                          继承
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {grant.directory_scope === 'current'
                          ? '仅当前目录'
                          : '当前及下级目录'}
                      </TableCell>
                      <TableCell align='right'>
                        <Tooltip title='请在授权来源处修改'>
                          <Typography
                            component='span'
                            fontSize={12}
                            color='text.secondary'
                          >
                            只读
                          </Typography>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  ))}
                  {grants.map((grant, index) => (
                    <TableRow key={`local:${index}`}>
                      <TableCell>
                        {grant.subject_type === 'user' && !documentOnly ? (
                          <Select
                            aria-label={`本级管理员${index + 1}`}
                            mode='multiple'
                            maxTagCount='responsive'
                            showSearch
                            optionFilterProp='label'
                            disabled={!ready || !canConfigure || loading}
                            style={{ width: '100%' }}
                            value={grant.subject_ids}
                            options={[
                              ...users.map(user => ({
                                value: user.id,
                                label: user.name,
                              })),
                              ...grant.subject_ids
                                .filter(
                                  id => !users.some(user => user.id === id),
                                )
                                .map(id => ({
                                  value: id,
                                  label: `${id}（不可选）`,
                                  disabled: true,
                                })),
                            ]}
                            placeholder='搜索姓名或账号'
                            onChange={subject_ids =>
                              updateGrants(
                                grants.map((item, i) =>
                                  i === index ? { ...item, subject_ids } : item,
                                ),
                              )
                            }
                          />
                        ) : (
                          <>
                            <Typography fontSize={13}>
                              {describeGrant(grant, users, departments, levels)}
                            </Typography>
                            <Typography fontSize={11} color='text.secondary'>
                              历史授权
                            </Typography>
                          </>
                        )}
                      </TableCell>
                      <TableCell>本级授权</TableCell>
                      <TableCell>
                        {grant.directory_scope === 'current' ? (
                          <Stack gap={0.5}>
                            <Typography fontSize={13}>仅当前目录</Typography>
                            {!documentOnly && grant.subject_type === 'user' && (
                              <Button
                                size='small'
                                color='inherit'
                                disabled={!canConfigure || loading}
                                sx={{
                                  p: 0,
                                  minWidth: 0,
                                  alignSelf: 'flex-start',
                                  fontSize: 12,
                                }}
                                onClick={() =>
                                  updateGrants(
                                    grants.map((item, i) =>
                                      i === index
                                        ? {
                                            ...item,
                                            directory_scope:
                                              'current_and_descendants',
                                          }
                                        : item,
                                    ),
                                  )
                                }
                              >
                                扩展到下级
                              </Button>
                            )}
                          </Stack>
                        ) : (
                          '当前及下级目录'
                        )}
                      </TableCell>
                      <TableCell align='right'>
                        {!documentOnly && (
                          <Button
                            size='small'
                            color='inherit'
                            sx={{
                              minWidth: 0,
                              p: 0,
                              color: 'text.secondary',
                              '&:hover': { color: 'error.main' },
                            }}
                            disabled={!canConfigure || loading}
                            onClick={() =>
                              updateGrants(grants.filter((_, i) => i !== index))
                            }
                          >
                            移除
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                  {ready && !inheritedRows.length && !grants.length && (
                    <TableRow>
                      <TableCell colSpan={4}>
                        <Typography
                          textAlign='center'
                          color='text.secondary'
                          fontSize={13}
                          py={2}
                        >
                          未设置额外管理员
                        </Typography>
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </TableContainer>
            {!documentOnly && canConfigure && (
              <Button
                size='small'
                variant='outlined'
                disabled={!ready || loading}
                sx={{ alignSelf: 'flex-start' }}
                onClick={() =>
                  updateGrants([
                    ...grants,
                    {
                      subject_type: 'user',
                      subject_ids: [],
                      directory_scope: 'current_and_descendants',
                    },
                  ])
                }
              >
                添加管理员
              </Button>
            )}
            {documentOnly && grants.length > 0 && (
              <Button
                size='small'
                color='inherit'
                disabled={!canConfigure || loading}
                sx={{ alignSelf: 'flex-start' }}
                onClick={() => updateGrants([])}
              >
                清空历史文档授权
              </Button>
            )}
            {invalid && (
              <Typography fontSize={12} color='error.main'>
                请补全管理员，或移除不可用的账号。
              </Typography>
            )}
            {ready && (
              <Typography
                fontSize={12}
                color='text.secondary'
                sx={{
                  borderTop: '1px solid',
                  borderColor: 'divider',
                  pt: 1.25,
                }}
              >
                {nodeIDs.length === 1
                  ? `当前规则命中 ${effectiveUsers.length} 位管理员；超级管理员始终可管理。`
                  : '上级继承的管理员需在授权来源处修改。'}
              </Typography>
            )}
          </>
        )}
      </Stack>
    </Modal>
  );
}
