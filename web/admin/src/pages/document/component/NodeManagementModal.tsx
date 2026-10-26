import { Modal, message } from '@/components/ui';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { Department, PermissionLevel } from '@/request/Enterprise';
import {
  editNodeManagement,
  editNavManagement,
  getNodeManagement,
  getNavManagement,
  getNodePermissionOptions,
} from '@/request/NodeManagement';
import { DomainDocumentManagementGrant } from '@/request/types';
import { buildDepartmentTree, flattenDepartmentTree } from '@/utils/enterprise';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { Select } from 'antd';
import { isAdministrator } from '@/utils/permission';
import { useEffect, useMemo, useState } from 'react';

const subjectLabels = {
  permission_level: '用户等级',
  user: '指定管理员',
  department: '部门',
};
const scopeOptions = [
  { value: 'current_and_descendants', label: '当前及下级目录' },
];
const legacyCurrentScopeOption = {
  value: 'current',
  label: '当前目录（历史规则，保留中）',
  disabled: true,
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
  return `${subjectLabels[grant.subject_type]}：${names || '未配置授权对象'}`;
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
  onClose,
  onSuccess,
}: {
  open: boolean;
  kbID: string;
  nodeIDs: string[];
  targetType?: 'node' | 'nav';
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [grants, setGrants] = useState<DomainDocumentManagementGrant[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [allDepartments, setAllDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const [users, setUsers] = useState<
    {
      id: string;
      name: string;
      administrator: boolean;
      permission_level_id?: string;
      department_ids?: string[];
    }[]
  >([]);
  const [loading, setLoading] = useState(false);
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [canConfigure, setCanConfigure] = useState(false);
  const [parentRuleCount, setParentRuleCount] = useState(0);
  const [hasParentFolder, setHasParentFolder] = useState(false);
  const [parentScopeMissing, setParentScopeMissing] = useState(false);
  const [parentGrantConstraints, setParentGrantConstraints] = useState<
    DomainDocumentManagementGrant[][]
  >([]);
  const nodeKey = nodeIDs.join(',');
  useEffect(() => {
    if (!open || !nodeIDs[0]) return;
    let active = true;
    let loadingManagementOptions = false;
    setReady(false);
    setLoadError('');
    setCanConfigure(false);
    setHasParentFolder(false);
    setParentScopeMissing(false);
    setParentGrantConstraints([]);
    setParentRuleCount(0);
    setLoading(true);
    const getManagement =
      targetType === 'nav' ? getNavManagement : getNodeManagement;
    Promise.all(nodeIDs.map(id => getManagement({ kb_id: kbID, id })))
      .then(async responses => {
        if (!active) return;
        const response = responses[0];
        const canConfigureAll = responses.every(item => item.can_configure);
        setCanConfigure(response.can_configure);
        setGrants(response.grants || []);
        const inheritedConstraints = responses.flatMap(
          item => item.parent_grant_constraints || [],
        );
        setParentRuleCount(inheritedConstraints.length);
        setParentGrantConstraints(inheritedConstraints);
        const hasFolderParent = responses.some(item => item.has_parent_folder);
        setHasParentFolder(hasFolderParent);
        setParentScopeMissing(
          responses.some(
            item =>
              item.has_parent_folder &&
              !(item.parent_grant_constraints || []).length,
          ),
        );
        if (!canConfigureAll) {
          setCanConfigure(false);
          return;
        }
        loadingManagementOptions = true;
        const options = await getNodePermissionOptions(kbID);
        if (!active) return;
        if (!options || typeof options !== 'object') {
          throw new Error('接口未返回可委派对象数据');
        }
        const availableDepartments = Array.isArray(options.departments)
          ? options.departments
          : [];
        const manageableDepartments = Array.isArray(
          options.management_departments,
        )
          ? options.management_departments
          : [];
        const availableLevels = Array.isArray(options.levels)
          ? options.levels
          : [];
        const manageableUsers = Array.isArray(options.management_users)
          ? options.management_users
          : [];
        setAllDepartments(
          availableDepartments.filter(item => item.status === 'active'),
        );
        setDepartments(
          manageableDepartments.filter(item => item.status === 'active'),
        );
        setLevels(
          options.can_delegate_management_by_level
            ? availableLevels.filter(item => item.status === 'active')
            : [],
        );
        setUsers(
          manageableUsers.map(item => ({
            id: item.id,
            name: item.name || item.account,
            administrator: isAdministrator(item.role),
            permission_level_id: item.permission_level_id,
            department_ids: item.department_ids,
          })),
        );
        setReady(true);
      })
      .catch(error => {
        if (active) {
          const endpoint = loadingManagementOptions
            ? 'GET /api/v1/node/permission/options'
            : `GET /api/v1/${targetType === 'nav' ? 'nav' : 'node'}/management`;
          const detail = getRequestErrorMessage(error);
          setLoadError(
            `${loadingManagementOptions ? '未能获取可委派对象' : '未能读取当前目录的管理权限'}，请检查 ${endpoint} 请求。${detail ? ` 接口详情：${detail}` : ''}`,
          );
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [open, kbID, nodeKey, targetType, loadAttempt]);
  const narrowedManagementOptions = useMemo(() => {
    if (hasParentFolder && parentScopeMissing) {
      return { users: [], departments: [], levels: [] };
    }
    const eligibleUsers = users.filter(user =>
      parentGrantConstraints.every(group =>
        group.some(grant => userMatchesGrant(grant, user, allDepartments)),
      ),
    );
    const eligibleIDs = new Set(eligibleUsers.map(user => user.id));
    const eligibleDepartments = departments.filter(department => {
      const members = users.filter(user =>
        (user.department_ids || []).some(id => {
          const assigned = allDepartments.find(item => item.id === id);
          return (
            assigned &&
            (assigned.path === department.path ||
              assigned.path.startsWith(`${department.path}/`))
          );
        }),
      );
      return (
        members.length > 0 && members.every(user => eligibleIDs.has(user.id))
      );
    });
    const eligibleLevels = levels.filter(level => {
      const members = users.filter(
        user => user.permission_level_id === level.id,
      );
      return (
        members.length > 0 && members.every(user => eligibleIDs.has(user.id))
      );
    });
    return {
      users: eligibleUsers,
      departments: eligibleDepartments,
      levels: eligibleLevels,
    };
  }, [
    users,
    departments,
    levels,
    parentGrantConstraints,
    allDepartments,
    hasParentFolder,
    parentScopeMissing,
  ]);
  const managementDepartmentTree = useMemo(() => {
    const eligibleIDs = new Set(
      narrowedManagementOptions.departments.map(item => item.id),
    );
    const selectedIDs = new Set(
      grants
        .filter(grant => grant.subject_type === 'department')
        .flatMap(grant => grant.subject_ids),
    );
    return buildDepartmentTree(
      allDepartments.map(department =>
        !eligibleIDs.has(department.id) && selectedIDs.has(department.id)
          ? { ...department, name: `${department.name}（超出上级授权范围）` }
          : department,
      ),
    );
  }, [allDepartments, grants, narrowedManagementOptions.departments]);
  const managementFlatDepartments = useMemo(
    () => flattenDepartmentTree(managementDepartmentTree),
    [managementDepartmentTree],
  );
  const hasInvalidManagementGrants = grants.some(grant => {
    const eligible =
      grant.subject_type === 'user'
        ? narrowedManagementOptions.users
        : grant.subject_type === 'department'
          ? narrowedManagementOptions.departments
          : narrowedManagementOptions.levels;
    const eligibleIDs = new Set(eligible.map(item => item.id));
    return grant.subject_ids.some(id => !eligibleIDs.has(id));
  });
  const update = (
    index: number,
    patch: Partial<DomainDocumentManagementGrant>,
  ) =>
    setGrants(prev =>
      prev.map((grant, i) => (i === index ? { ...grant, ...patch } : grant)),
    );
  const submit = () => {
    if (hasInvalidManagementGrants) {
      message.warning('已有管理授权超出上级范围，请移除标注的对象后再保存');
      return;
    }
    if (grants.some(grant => grant.subject_ids.length === 0)) {
      message.warning('请选择每条规则的授权对象，或删除空规则');
      return;
    }
    setLoading(true);
    const save =
      targetType === 'nav'
        ? editNavManagement({ kb_id: kbID, id: nodeIDs[0], grants })
        : editNodeManagement({ kb_id: kbID, ids: nodeIDs, grants });
    save
      .then(() => {
        message.success('文档管理权限已保存');
        onClose();
        onSuccess();
      })
      .finally(() => setLoading(false));
  };
  return (
    <Modal
      open={open}
      title='文档管理权限'
      width={760}
      onCancel={onClose}
      onOk={submit}
      okButtonProps={{
        loading,
        disabled: !ready || !canConfigure || hasInvalidManagementGrants,
      }}
    >
      <Stack
        gap={2}
        sx={{ maxHeight: 'calc(100dvh - 220px)', overflowY: 'auto', pr: 0.5 }}
      >
        <Typography fontSize={13} color='text.secondary'>
          管理权限独立于查阅权限。获得管理授权的管理员可以在管理端查看和管理授权范围内的文档；这不会授予终端用户查阅或问答权限。新规则只能指定管理员账号。
        </Typography>
        {targetType === 'node' && (
          <Typography fontSize={13} color='text.secondary'>
            对当前目录生效的上级管理规则会逐级同时校验；本级只能在这些规则共同允许的管理员范围内继续收紧。
          </Typography>
        )}
        {targetType === 'node' &&
          nodeIDs.length === 1 &&
          parentRuleCount > 0 && (
            <Box
              sx={{
                p: 1.5,
                borderRadius: 1.5,
                bgcolor: 'background.paper3',
              }}
            >
              <Typography fontSize={13} fontWeight={600} sx={{ mb: 0.5 }}>
                上级有效管理范围
              </Typography>
              <Typography fontSize={12} color='text.secondary' sx={{ mb: 1 }}>
                每一级上级授权都必须满足；同一级列出的授权对象满足其一即可。
              </Typography>
              <Stack gap={0.5}>
                {parentGrantConstraints.map((group, index) => (
                  <Typography key={index} fontSize={13}>
                    第 {index + 1} 级：
                    {group
                      .map(grant =>
                        describeGrant(grant, users, allDepartments, levels),
                      )
                      .join('；或 ')}
                  </Typography>
                ))}
              </Stack>
            </Box>
          )}
        {targetType === 'node' && nodeIDs.length === 1 && !hasParentFolder && (
          <Typography fontSize={13} color='text.secondary'>
            一级目录没有上级文件夹管理范围限制，可将权限分配给任意管理员。
          </Typography>
        )}
        {targetType === 'node' && parentScopeMissing && (
          <Typography fontSize={13} color='warning.main'>
            上级目录尚无可委派的管理员范围。请先在上级目录配置管理权限，再为此目录分配管理员。
          </Typography>
        )}
        {hasInvalidManagementGrants && (
          <Typography fontSize={13} color='warning.main'>
            已有管理授权超出上级范围，已在授权对象旁标出。移除或更换这些对象后才能保存。
          </Typography>
        )}
        {targetType === 'node' && nodeIDs.length > 1 && (
          <Typography fontSize={13} color='text.secondary'>
            批量保存时，每个节点都会单独校验其上级管理范围；只有同时满足所有节点上级授权的管理员范围，才能保存。
          </Typography>
        )}
        {targetType === 'node' &&
          nodeIDs.length === 1 &&
          parentRuleCount > 0 && (
            <Typography fontSize={13} color='text.secondary'>
              下方候选管理员已按以上上级范围收窄，服务端保存时还会再次校验。本级留空表示不额外增加管理授权限制。
            </Typography>
          )}
        <Typography fontSize={13} color='text.secondary'>
          管理员可将管理权限委派给管理员；子目录只能选择符合全部上级管理规则的管理员。新规则只能指定管理员账号，权限范围固定为“当前及下级目录”。历史“当前目录”规则继续按原范围生效；如需扩大范围，可显式改为“当前及下级目录”。历史部门或用户等级规则可保留或删除，不能新增或修改。
        </Typography>
        {nodeIDs.length > 1 && (
          <Typography color='warning.main' fontSize={13}>
            批量保存会将以下规则替换到所选 {nodeIDs.length}{' '}
            个节点；当前展示第一个节点的配置。
          </Typography>
        )}
        {!canConfigure && !loading && (
          <Typography fontSize={13}>
            只有超级管理员或已获授权的管理员可以配置此处的管理权限。
            {targetType === 'node' && '继承的授权由上级目录配置。'}
          </Typography>
        )}
        {loading && !ready && (
          <Typography fontSize={13} color='text.secondary'>
            正在加载当前目录的管理权限和可委派对象，完成后即可添加规则。
          </Typography>
        )}
        {loadError && (
          <Alert
            severity='error'
            action={
              <Button
                color='inherit'
                size='small'
                onClick={() => setLoadAttempt(value => value + 1)}
              >
                重试
              </Button>
            }
          >
            管理授权配置未加载完成，暂时无法添加规则。{loadError}
          </Alert>
        )}
        {grants.map((grant, index) => (
          <Box
            key={index}
            sx={{
              p: 1.5,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
            }}
          >
            <Stack gap={1.25}>
              <Stack
                direction='row'
                alignItems='center'
                justifyContent='space-between'
              >
                <Typography fontSize={14} fontWeight={600}>
                  {subjectLabels[grant.subject_type]}
                </Typography>
                <Button
                  size='small'
                  disabled={!canConfigure}
                  color='error'
                  onClick={() =>
                    setGrants(prev => prev.filter((_, i) => i !== index))
                  }
                >
                  删除规则
                </Button>
              </Stack>
              {grant.subject_type === 'department' ? (
                <DepartmentTreeSelect
                  multiple
                  departments={managementDepartmentTree}
                  flatDepartments={managementFlatDepartments}
                  value={grant.subject_ids}
                  excludeIds={allDepartments
                    .filter(
                      department =>
                        !narrowedManagementOptions.departments.some(
                          item => item.id === department.id,
                        ),
                    )
                    .map(department => department.id)}
                  onChange={value => update(index, { subject_ids: value })}
                  label='授权部门'
                  size='small'
                  disabled
                />
              ) : (
                <Select
                  disabled={!canConfigure || grant.subject_type !== 'user'}
                  mode='multiple'
                  showSearch
                  optionFilterProp='label'
                  style={{ width: '100%' }}
                  value={grant.subject_ids}
                  placeholder={`选择${subjectLabels[grant.subject_type]}`}
                  options={
                    grant.subject_type === 'user'
                      ? [
                          ...narrowedManagementOptions.users.map(user => ({
                            value: user.id,
                            label: user.name,
                          })),
                          ...users
                            .filter(
                              user =>
                                grant.subject_ids.includes(user.id) &&
                                !narrowedManagementOptions.users.some(
                                  eligible => eligible.id === user.id,
                                ),
                            )
                            .map(user => ({
                              value: user.id,
                              label: `${user.name}（超出上级授权范围）`,
                              disabled: true,
                            })),
                        ]
                      : [
                          ...narrowedManagementOptions.levels.map(level => ({
                            value: level.id,
                            label: level.name,
                          })),
                          ...levels
                            .filter(
                              level =>
                                grant.subject_ids.includes(level.id) &&
                                !narrowedManagementOptions.levels.some(
                                  eligible => eligible.id === level.id,
                                ),
                            )
                            .map(level => ({
                              value: level.id,
                              label: `${level.name}（超出上级授权范围）`,
                              disabled: true,
                            })),
                        ]
                  }
                  onChange={value => update(index, { subject_ids: value })}
                />
              )}
              <Stack direction='row' alignItems='center' gap={1}>
                <Typography fontSize={13}>目录权限范围</Typography>
                <Select
                  disabled={!canConfigure || grant.subject_type !== 'user'}
                  style={{ minWidth: 190 }}
                  value={grant.directory_scope || 'current_and_descendants'}
                  options={
                    grant.directory_scope === 'current'
                      ? [...scopeOptions, legacyCurrentScopeOption]
                      : scopeOptions
                  }
                  onChange={value => update(index, { directory_scope: value })}
                />
              </Stack>
            </Stack>
          </Box>
        ))}
        {canConfigure && (
          <Button
            variant='outlined'
            disabled={!ready || loading}
            sx={{ alignSelf: 'flex-start' }}
            onClick={() =>
              setGrants(prev => [
                ...prev,
                {
                  subject_type: 'user',
                  subject_ids: [],
                  directory_scope: 'current_and_descendants',
                },
              ])
            }
          >
            添加管理授权规则
          </Button>
        )}
        <Typography fontSize={12} color='text.secondary'>
          未添加规则表示本级不额外收紧管理范围；
          {targetType === 'node' && '上级目录授权仍有效；'}
          超级管理员的全局权限仍有效。保存查阅权限不会修改管理权限。
        </Typography>
      </Stack>
    </Modal>
  );
}
