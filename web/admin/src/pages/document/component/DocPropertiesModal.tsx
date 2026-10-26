import { Modal, message } from '@/components/ui';
import { Form } from '@/pages/setting/component/Common';
import { Department } from '@/request/Enterprise';
import { getApiV1NavPermission } from '@/request/NavCompat';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import {
  getApiV1NodePermission,
  patchApiV1NodePermissionEdit,
} from '@/request/NodePermission';
import {
  ConstsUserRole,
  DomainNodeListItemResp,
  DomainNodePermissions,
  DomainNodeType,
} from '@/request/types';
import { useAppSelector } from '@/store';
import { Alert, Button, Stack } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import {
  selectableReadingDepartments,
  selectableReadingLevels,
} from '../utils/readingPermissionOptions';
import NodeManagementModal from './NodeManagementModal';
import ReadingPermissionFields, {
  makeInheritedReadingPermissionValue,
  normalizeReadingPermissionValue,
  ReadingPermissionValue,
  readingPermissionPayload,
} from './ReadingPermissionFields';
import ReadingPermissionPanel from './ReadingPermissionPanel';
import {
  ManagementPermissionSection,
  PermissionTarget,
} from './PermissionSettingsSections';

interface DocPropertiesModalProps {
  open: boolean;
  onCancel: () => void;
  onOk: () => void;
  isBatch?: boolean;
  data: DomainNodeListItemResp[];
}

const emptyValue = (): ReadingPermissionValue => ({
  scope_type: 'public',
  level_restriction_mode: 'none',
  min_permission_level_id: '',
  allowed_permission_level_ids: [],
  allowed_department_ids: [],
  allowed_user_ids: [],
  allow_all_users: false,
  denied_user_ids: [],
});

const hasExplicitPermissionRule = (permissions: DomainNodePermissions) =>
  !!permissions.scope_type ||
  !!permissions.allowed_permission_level_ids?.length ||
  !!permissions.min_permission_level_id ||
  !!permissions.min_permission_level_rank ||
  !!permissions.allowed_department_ids?.length ||
  !!permissions.allowed_user_ids?.length ||
  !!permissions.allow_all_users ||
  !!permissions.denied_department_ids?.length ||
  !!permissions.denied_user_ids?.length;

const isUnrestrictedPermission = (
  permissions: DomainNodePermissions,
  levelsDisabled = false,
): boolean => {
  const scope = permissions.scope_type;
  const ownRuleIsUnrestricted = permissions.allow_all_users
    ? !permissions.denied_department_ids?.length &&
      !permissions.denied_user_ids?.length
    : (!scope || scope === 'public' || scope === 'custom') &&
      !permissions.min_permission_level_id &&
      !permissions.min_permission_level_rank &&
      (levelsDisabled || !permissions.allowed_permission_level_ids?.length) &&
      !permissions.allowed_department_ids?.length &&
      !permissions.allowed_user_ids?.length &&
      !permissions.denied_department_ids?.length &&
      !permissions.denied_user_ids?.length;
  return (
    ownRuleIsUnrestricted &&
    (permissions.constraints || []).every(constraint =>
      isUnrestrictedPermission(constraint, levelsDisabled),
    )
  );
};

const userMatchesPermission = (
  permissions: DomainNodePermissions,
  user: { id: string; permission_level_id?: string; department_ids?: string[] },
  levels: { id: string; rank: number }[],
  departments: Department[],
  levelsDisabled: boolean,
): boolean => {
  if (
    (permissions.constraints || []).some(
      constraint =>
        !userMatchesPermission(
          constraint,
          user,
          levels,
          departments,
          levelsDisabled,
        ),
    )
  ) {
    return false;
  }
  const rank =
    levels.find(level => level.id === user.permission_level_id)?.rank || 0;
  if (user.id && permissions.denied_user_ids?.includes(user.id)) return false;
  const departmentIDs = new Set(user.department_ids || []);
  const userDepartments = departments.filter(department =>
    (user.department_ids || []).some(id => {
      const assigned = departments.find(item => item.id === id);
      return (
        assigned &&
        (id === department.id ||
          assigned.ancestor_ids?.includes(department.id) ||
          assigned.path === department.path ||
          assigned.path.startsWith(`${department.path}/`))
      );
    }),
  );
  for (const department of userDepartments) {
    departmentIDs.add(department.id);
    for (const ancestorID of department.ancestor_ids || []) {
      departmentIDs.add(ancestorID);
    }
  }
  if (permissions.denied_department_ids?.some(id => departmentIDs.has(id))) {
    return false;
  }
  if (permissions.allow_all_users) return true;
  const scope = permissions.scope_type;
  if (user.id && permissions.allowed_user_ids?.includes(user.id)) return true;
  if (scope === 'permission_level') {
    return (
      levelsDisabled ||
      (!!user.permission_level_id &&
        !!permissions.allowed_permission_level_ids?.includes(
          user.permission_level_id,
        ) &&
        (!permissions.min_permission_level_rank ||
          rank >= permissions.min_permission_level_rank))
    );
  }
  if (scope === 'user') {
    return false;
  }
  if (scope === 'department') {
    return (
      !!permissions.allowed_department_ids?.some(id => departmentIDs.has(id)) &&
      (levelsDisabled ||
        !permissions.min_permission_level_rank ||
        rank >= permissions.min_permission_level_rank)
    );
  }

  const allowedDepartments = permissions.allowed_department_ids || [];
  const allowedLevels = permissions.allowed_permission_level_ids || [];
  const departmentRestricted = allowedDepartments.length > 0;
  const levelRestricted = !levelsDisabled && allowedLevels.length > 0;
  const minimumRank = !levelsDisabled
    ? permissions.min_permission_level_rank || 0
    : 0;
  if (!departmentRestricted && !levelRestricted && !minimumRank) {
    return !permissions.allowed_user_ids?.length;
  }
  if (
    departmentRestricted &&
    !allowedDepartments.some(id => departmentIDs.has(id))
  ) {
    return false;
  }
  if (
    levelRestricted &&
    (!user.permission_level_id ||
      !allowedLevels.includes(user.permission_level_id))
  ) {
    return false;
  }
  if (minimumRank && rank < minimumRank) return false;
  return true;
};

const DocPropertiesModal = ({
  open,
  onCancel,
  data,
  onOk,
  isBatch = false,
}: DocPropertiesModalProps) => {
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const publicAllowed = useAppSelector(
    state => state.config.user?.role === ConstsUserRole.UserRoleAdmin,
  );
  const { kb_id, nav_id } = useAppSelector(state => state.config);
  const [managementOpen, setManagementOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [readingDirty, setReadingDirty] = useState(false);
  const [savedPermissions, setSavedPermissions] =
    useState<DomainNodePermissions>({});
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [value, setValue] = useState<ReadingPermissionValue>(emptyValue);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<
    { id: string; name: string; rank: number }[]
  >([]);
  const [users, setUsers] = useState<
    {
      id: string;
      account: string;
      permission_level_id?: string;
      department_ids?: string[];
    }[]
  >([]);
  const canManagePermission =
    data.length > 0 && data.every(item => item.can_manage);
  const [parentPermissions, setParentPermissions] =
    useState<DomainNodePermissions>({});
  const [parentPermissionGroups, setParentPermissionGroups] = useState<
    DomainNodePermissions[]
  >([]);
  const isFolder = data[0]?.type === DomainNodeType.NodeTypeFolder;
  const narrowedOptions = useMemo(() => {
    const eligible = users.filter(user =>
      parentPermissionGroups.every(parent =>
        userMatchesPermission(
          parent,
          user,
          levels,
          departments,
          !!levelsDisabled,
        ),
      ),
    );
    const eligibleIDs = new Set(eligible.map(user => user.id));
    const eligibleLevels = selectableReadingLevels(
      levels,
      parentPermissionGroups,
      !!levelsDisabled,
    );
    const activeLevels = selectableReadingLevels(levels, [], !!levelsDisabled);
    const eligibleLevelIDs = new Set(eligibleLevels.map(level => level.id));
    const minimumPermissionLevels = eligibleLevels.filter(level =>
      activeLevels.every(
        candidate =>
          candidate.rank < level.rank || eligibleLevelIDs.has(candidate.id),
      ),
    );
    const eligibleDepartments = selectableReadingDepartments(
      departments,
      parentPermissionGroups,
    );
    return {
      users: eligible,
      departments: eligibleDepartments,
      permissionLevels: eligibleLevels,
      minimumPermissionLevels,
      restrictedUserIDs: users
        .filter(user => !eligibleIDs.has(user.id))
        .map(user => user.id)
        .concat(value.allowed_user_ids.filter(id => !eligibleIDs.has(id))),
      restrictedDepartmentIDs: departments
        .filter(
          department =>
            !eligibleDepartments.some(item => item.id === department.id),
        )
        .map(department => department.id)
        .concat(
          value.allowed_department_ids.filter(
            id => !eligibleDepartments.some(item => item.id === id),
          ),
        ),
      restrictedPermissionLevelIDs: levels
        .filter(level => !eligibleLevels.some(item => item.id === level.id))
        .map(level => level.id)
        .concat(
          value.allowed_permission_level_ids.filter(
            id => !eligibleLevels.some(item => item.id === id),
          ),
        ),
      restrictedMinimumPermissionLevelIDs: levels
        .filter(
          level => !minimumPermissionLevels.some(item => item.id === level.id),
        )
        .map(level => level.id)
        .concat(
          value.min_permission_level_id &&
            !minimumPermissionLevels.some(
              item => item.id === value.min_permission_level_id,
            )
            ? [value.min_permission_level_id]
            : [],
        ),
    };
  }, [
    users,
    levels,
    departments,
    parentPermissionGroups,
    levelsDisabled,
    value.allowed_user_ids,
    value.allowed_department_ids,
    value.allowed_permission_level_ids,
    value.min_permission_level_id,
  ]);
  const invalidAudienceSelections = useMemo(() => {
    const restrictedUsers = new Set(narrowedOptions.restrictedUserIDs);
    const restrictedDepartments = new Set(
      narrowedOptions.restrictedDepartmentIDs,
    );
    const restrictedLevels = new Set(
      narrowedOptions.restrictedPermissionLevelIDs,
    );
    return {
      users:
        value.scope_type === 'user' || value.scope_type === 'custom'
          ? value.allowed_user_ids.filter(id => restrictedUsers.has(id))
          : [],
      departments:
        value.scope_type === 'department' || value.scope_type === 'custom'
          ? value.allowed_department_ids.filter(id =>
              restrictedDepartments.has(id),
            )
          : [],
      levels:
        value.scope_type === 'custom'
          ? value.allowed_permission_level_ids.filter(id =>
              restrictedLevels.has(id),
            )
          : [],
      minimumLevel:
        value.scope_type === 'custom' &&
        value.level_restriction_mode === 'minimum' &&
        narrowedOptions.restrictedMinimumPermissionLevelIDs.includes(
          value.min_permission_level_id,
        ),
    };
  }, [value, narrowedOptions]);
  const hasIncompleteLevelRestriction =
    (value.level_restriction_mode === 'minimum' &&
      !value.min_permission_level_id) ||
    (value.level_restriction_mode === 'selected' &&
      value.allowed_permission_level_ids.length === 0);
  const hasInvalidAudienceSelections =
    !!value.level_restriction_needs_review ||
    hasIncompleteLevelRestriction ||
    invalidAudienceSelections.users.length > 0 ||
    invalidAudienceSelections.departments.length > 0 ||
    invalidAudienceSelections.levels.length > 0;
  const hasInvalidAudienceSelection =
    hasInvalidAudienceSelections || invalidAudienceSelections.minimumLevel;
  const incompleteAudience =
    value.scope_type === 'custom' &&
    !value.allow_all_users &&
    value.level_restriction_mode === 'none' &&
    !value.allowed_department_ids.length &&
    !value.allowed_user_ids.length;
  const canSetPublic =
    publicAllowed &&
    parentPermissionGroups.every(parent =>
      isUnrestrictedPermission(parent, !!levelsDisabled),
    );
  const inheritedValue = useMemo(
    () => makeInheritedReadingPermissionValue(parentPermissions, levels),
    [parentPermissions, levels],
  );
  const modalTitle = isBatch
    ? '批量设置权限'
    : isFolder
      ? '目录权限'
      : '文档权限';

  useEffect(() => {
    if (!open || !data.length || !kb_id) {
      setManagementOpen(false);
      return;
    }
    let active = true;
    setReady(false);
    setLoadError(false);
    setValue(emptyValue());
    setReadingDirty(false);
    setSavedPermissions({});
    setParentPermissions({});
    setParentPermissionGroups([]);
    const node = data[0];
    const parentPermissionRequests = new Map<
      string,
      Promise<DomainNodePermissions>
    >();
    const loadParentPermissions = (item: DomainNodeListItemResp) => {
      const currentNavID = item.nav_id || nav_id;
      // 栏目根的空 ID、栏目 ID 和旧 nav:ID 都不属于 nodes 父节点。
      const parentNodeID =
        item.parent_id &&
        !item.parent_id.startsWith('nav:') &&
        item.parent_id !== currentNavID
          ? item.parent_id
          : '';
      const key = parentNodeID
        ? `node:${parentNodeID}`
        : currentNavID
          ? `nav:${currentNavID}`
          : '';
      if (!key) return Promise.resolve({});
      const cached = parentPermissionRequests.get(key);
      if (cached) return cached;
      const request = parentNodeID
        ? getApiV1NodePermission({ kb_id, id: parentNodeID }).then(
            response =>
              response.effective_permissions || response.permissions || {},
          )
        : getApiV1NavPermission({ kb_id, id: currentNavID }).then(
            response => response.permissions || {},
          );
      parentPermissionRequests.set(key, request);
      return request;
    };
    Promise.all([
      getNodePermissionOptions(kb_id),
      Promise.all(data.map(loadParentPermissions)),
      !isBatch
        ? getApiV1NodePermission({ kb_id, id: node.id! })
        : Promise.resolve(null),
    ])
      .then(([options, parentPermissionResults, response]) => {
        if (!active) return;
        setDepartments(options.departments || []);
        setLevels(options.levels || []);
        setUsers(options.users || []);
        setParentPermissionGroups(parentPermissionResults);
        setParentPermissions(parentPermissionResults[0] || {});
        const permissions = response?.permissions || {};
        setSavedPermissions(response?.effective_permissions || permissions);
        const hasLocalRule =
          permissions.inherit === false ||
          (permissions.inherit === undefined &&
            hasExplicitPermissionRule(permissions));
        if (hasLocalRule) {
          setValue(
            normalizeReadingPermissionValue(permissions, options.levels),
          );
        } else {
          setValue(emptyValue());
          setReadingDirty(false);
          setSavedPermissions({});
        }
        setReady(true);
      })
      .catch(() => {
        if (active) setLoadError(true);
      });
    return () => {
      active = false;
    };
  }, [open, data, kb_id, isBatch, nav_id]);

  const submit = async () => {
    if (!ready || saving || !kb_id) return;
    if (value.level_restriction_needs_review || hasIncompleteLevelRestriction) {
      message.warning('请完成等级范围选择后再保存');
      return;
    }
    const scope = value.scope_type;
    const hasLevelRestriction =
      value.level_restriction_mode === 'minimum'
        ? !!value.min_permission_level_id
        : value.level_restriction_mode === 'selected'
          ? value.allowed_permission_level_ids.length > 0
          : false;
    if (
      scope === 'custom' &&
      !value.allow_all_users &&
      !hasLevelRestriction &&
      !value.allowed_department_ids.length &&
      !value.allowed_user_ids.length
    ) {
      message.warning('请至少设置一个部门、等级限制或指定用户');
      return;
    }
    setSaving(true);
    try {
      await patchApiV1NodePermissionEdit({
        kb_id,
        ids: data.map(item => item.id!),
        permissions: readingPermissionPayload(value, true),
      });
      message.success('查阅权限已保存');
      setReadingDirty(false);
      onOk();
    } catch {
      // 请求层已展示保存失败原因，保留当前设置以便重试。
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Modal
        title={modalTitle}
        open={open}
        onCancel={onCancel}
        width={720}
        centered
        okText='保存查阅权限'
        okButtonProps={{
          loading: saving,
          disabled:
            !ready ||
            !canManagePermission ||
            !readingDirty ||
            hasInvalidAudienceSelection ||
            incompleteAudience,
        }}
        onOk={submit}
      >
        <Stack
          gap={2}
          sx={{ maxHeight: 'calc(100dvh - 200px)', overflowY: 'auto', pr: 0.5 }}
        >
          <PermissionTarget
            names={data.map(item => item.name || '未命名')}
            kind={isFolder ? '目录' : '文档'}
          />
          {loadError && (
            <Alert severity='error'>权限加载失败，请关闭窗口后重试。</Alert>
          )}
          <ReadingPermissionPanel
            open={open}
            ready={ready}
            kbID={kb_id || ''}
            ids={data.map(item => item.id!).filter(Boolean)}
            canEdit={canManagePermission}
            dirty={readingDirty}
            invalid={
              readingDirty &&
              (hasInvalidAudienceSelection || incompleteAudience)
            }
            draft={readingPermissionPayload(value, true)}
            saved={savedPermissions}
            usingParent={value.scope_type === 'public'}
            levels={levels}
            departments={departments}
            users={users}
            levelsDisabled={levelsDisabled}
            onLocate={onCancel}
          >
            <Form labelWidth={100} gap={2}>
              <ReadingPermissionFields
                value={value}
                publicAllowed={canSetPublic}
                neutralAudienceAllowed
                inheritedValue={inheritedValue}
                onChange={patch => {
                  setValue(prev => ({ ...prev, ...patch }));
                  setReadingDirty(true);
                }}
                levelsDisabled={levelsDisabled}
                levels={levels}
                permissionLevels={narrowedOptions.permissionLevels}
                minimumPermissionLevels={
                  narrowedOptions.minimumPermissionLevels
                }
                allPermissionLevels={levels}
                departments={departments}
                users={users}
                restrictedPermissionLevelIDs={
                  narrowedOptions.restrictedPermissionLevelIDs
                }
                restrictedMinimumPermissionLevelIDs={
                  narrowedOptions.restrictedMinimumPermissionLevelIDs
                }
                restrictedDepartmentIDs={
                  narrowedOptions.restrictedDepartmentIDs
                }
                restrictedUserIDs={narrowedOptions.restrictedUserIDs}
                disabled={!ready || saving || !canManagePermission}
              />
            </Form>
            {hasInvalidAudienceSelection && (
              <Alert severity='warning' icon={false}>
                请修正以下查阅规则后再保存：
                {[
                  value.level_restriction_needs_review &&
                    '历史等级规则需要重新选择',
                  invalidAudienceSelections.departments.length > 0 &&
                    `部门（${invalidAudienceSelections.departments
                      .map(
                        id =>
                          departments.find(item => item.id === id)?.name ||
                          `ID ${id}`,
                      )
                      .join('、')}）`,
                  invalidAudienceSelections.levels.length > 0 &&
                    `用户等级（${invalidAudienceSelections.levels
                      .map(
                        id =>
                          levels.find(item => item.id === id)?.name ||
                          `ID ${id}`,
                      )
                      .join('、')}）`,
                  invalidAudienceSelections.minimumLevel &&
                    `最低等级（${levels.find(item => item.id === value.min_permission_level_id)?.name || `ID ${value.min_permission_level_id}`}）`,
                  invalidAudienceSelections.users.length > 0 &&
                    `用户（${invalidAudienceSelections.users
                      .map(
                        id =>
                          users.find(item => item.id === id)?.account ||
                          `ID ${id}`,
                      )
                      .join('、')}）`,
                ]
                  .filter(Boolean)
                  .join('；')}
                。
                <Button
                  size='small'
                  onClick={() => {
                    setReadingDirty(true);
                    setValue(prev => ({
                      ...prev,
                      level_restriction_mode:
                        prev.level_restriction_needs_review ||
                        invalidAudienceSelections.minimumLevel
                          ? 'none'
                          : prev.level_restriction_mode,
                      level_restriction_needs_review: false,
                      min_permission_level_id:
                        prev.level_restriction_needs_review ||
                        invalidAudienceSelections.minimumLevel
                          ? ''
                          : prev.min_permission_level_id,
                      allowed_department_ids:
                        prev.allowed_department_ids.filter(
                          id =>
                            !invalidAudienceSelections.departments.includes(id),
                        ),
                      allowed_permission_level_ids:
                        prev.allowed_permission_level_ids.filter(
                          id => !invalidAudienceSelections.levels.includes(id),
                        ),
                      allowed_user_ids: prev.allowed_user_ids.filter(
                        id => !invalidAudienceSelections.users.includes(id),
                      ),
                    }));
                  }}
                >
                  移除这些对象
                </Button>
              </Alert>
            )}
          </ReadingPermissionPanel>
          <ManagementPermissionSection
            onConfigure={() => setManagementOpen(true)}
            disabled={!ready || saving || !canManagePermission}
          />
        </Stack>
      </Modal>
      {/* 独立渲染，避免父弹窗关闭时缓存内容导致子弹窗无法关闭。 */}
      <NodeManagementModal
        open={open && managementOpen}
        kbID={kb_id!}
        nodeIDs={data.map(item => item.id!).filter(Boolean)}
        onClose={() => setManagementOpen(false)}
        onSuccess={onOk}
      />
    </>
  );
};

export default DocPropertiesModal;
