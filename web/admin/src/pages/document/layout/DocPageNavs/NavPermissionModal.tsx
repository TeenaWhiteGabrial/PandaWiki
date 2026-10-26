import { useAppSelector } from '@/store';
import { Form } from '@/pages/setting/component/Common';
import { Department } from '@/request/Enterprise';
import {
  getApiV1NavPermission,
  patchApiV1NavPermission,
} from '@/request/NavCompat';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import {
  ConstsUserRole,
  V1NavListResp,
  DomainNodePermissions,
} from '@/request/types';
import { message, Modal } from '@/components/ui';
import { Alert, Stack } from '@mui/material';
import { useEffect, useState } from 'react';
import NodeManagementModal from '@/pages/document/component/NodeManagementModal';
import ReadingPermissionFields, {
  normalizeReadingPermissionValue,
  ReadingPermissionValue,
  readingPermissionPayload,
} from '@/pages/document/component/ReadingPermissionFields';
import ReadingPermissionPanel from '@/pages/document/component/ReadingPermissionPanel';
import {
  ManagementPermissionSection,
  PermissionTarget,
} from '@/pages/document/component/PermissionSettingsSections';

const emptyPermissions = (): ReadingPermissionValue => ({
  scope_type: 'public',
  level_restriction_mode: 'none',
  min_permission_level_id: '',
  allowed_permission_level_ids: [],
  allowed_department_ids: [],
  allowed_user_ids: [],
  allow_all_users: false,
  denied_user_ids: [],
});
interface NavPermissionModalProps {
  open: boolean;
  nav: V1NavListResp | null;
  kb_id: string;
  onClose: () => void;
  onSuccess: () => void;
}

export default function NavPermissionModal({
  open,
  nav,
  kb_id,
  onClose,
  onSuccess,
}: NavPermissionModalProps) {
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const publicAllowed = useAppSelector(
    state => state.config.user?.role === ConstsUserRole.UserRoleAdmin,
  );
  const canManagePermission = nav?.can_configure === true;
  const [loading, setLoading] = useState(false);
  const [readingDirty, setReadingDirty] = useState(false);
  const [savedPermissions, setSavedPermissions] =
    useState<DomainNodePermissions>({});
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [managementOpen, setManagementOpen] = useState(false);
  const [value, setValue] = useState<ReadingPermissionValue>(emptyPermissions);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<
    { id: string; name: string; rank: number }[]
  >([]);
  const [users, setUsers] = useState<{ id: string; account: string }[]>([]);
  useEffect(() => {
    if (!open || !nav?.id) {
      setManagementOpen(false);
      return;
    }
    let active = true;
    setReady(false);
    setLoadError(false);
    setLoading(true);
    setValue(emptyPermissions());
    setReadingDirty(false);
    setSavedPermissions({});
    Promise.all([
      getNodePermissionOptions(kb_id),
      getApiV1NavPermission({ kb_id, id: nav.id }),
    ])
      .then(([options, response]) => {
        if (!active) return;
        setDepartments(
          options.departments.filter(item => item.status === 'active'),
        );
        setLevels(options.levels.filter(item => item.status === 'active'));
        setUsers(options.users);
        const permissions = response.permissions || {};
        setSavedPermissions(permissions);
        setValue(normalizeReadingPermissionValue(permissions, options.levels));
        setReady(true);
      })
      .catch(() => {
        if (active) setLoadError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [open, nav?.id, kb_id]);
  const hasIncompleteLevelRestriction =
    (value.level_restriction_mode === 'minimum' &&
      !value.min_permission_level_id) ||
    (value.level_restriction_mode === 'selected' &&
      value.allowed_permission_level_ids.length === 0);
  const incompleteAudience =
    value.scope_type === 'custom' &&
    !value.allow_all_users &&
    value.level_restriction_mode === 'none' &&
    !value.allowed_department_ids.length &&
    !value.allowed_user_ids.length;
  const submit = async () => {
    if (!nav?.id || !ready) return;
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
    setLoading(true);
    try {
      await patchApiV1NavPermission({
        kb_id,
        id: nav.id,
        permissions: readingPermissionPayload(value),
      });
      message.success('目录查阅权限已更新');
      setReadingDirty(false);
      onSuccess();
    } catch {
      // 请求层已展示保存失败原因，保留当前设置以便重试。
    } finally {
      setLoading(false);
    }
  };
  return (
    <>
      <Modal
        title='目录权限'
        open={open}
        width={720}
        centered
        okText='保存查阅权限'
        okButtonProps={{
          loading,
          disabled:
            !ready ||
            !canManagePermission ||
            !readingDirty ||
            incompleteAudience ||
            !!value.level_restriction_needs_review ||
            hasIncompleteLevelRestriction,
        }}
        onCancel={onClose}
        onOk={submit}
      >
        <Stack
          gap={2}
          sx={{ maxHeight: 'calc(100dvh - 200px)', overflowY: 'auto', pr: 0.5 }}
        >
          <PermissionTarget names={[nav?.name || '未命名栏目']} kind='栏目' />
          {loadError && (
            <Alert severity='error'>权限加载失败，请关闭窗口后重试。</Alert>
          )}
          {!canManagePermission && (
            <Alert severity='info'>
              你可以查阅此栏目，但没有修改栏目权限的管理范围。
            </Alert>
          )}
          <ReadingPermissionPanel
            open={open}
            ready={ready}
            kbID={kb_id}
            ids={nav?.id ? [nav.id] : []}
            targetType='nav'
            canEdit={canManagePermission}
            dirty={readingDirty}
            invalid={
              readingDirty &&
              (!!value.level_restriction_needs_review ||
                hasIncompleteLevelRestriction ||
                incompleteAudience)
            }
            draft={readingPermissionPayload(value)}
            saved={savedPermissions}
            levels={levels}
            departments={departments}
            users={users}
            levelsDisabled={levelsDisabled}
            onLocate={onClose}
          >
            <Form labelWidth={100} gap={2}>
              <ReadingPermissionFields
                value={value}
                publicAllowed={publicAllowed}
                allUsersAllowed={publicAllowed}
                onChange={patch => {
                  setValue(prev => ({ ...prev, ...patch }));
                  setReadingDirty(true);
                }}
                levelsDisabled={levelsDisabled}
                levels={levels}
                departments={departments}
                users={users}
                disabled={!ready || loading || !canManagePermission}
              />
            </Form>
          </ReadingPermissionPanel>
          <ManagementPermissionSection
            onConfigure={() => setManagementOpen(true)}
            disabled={!ready || loading || !canManagePermission}
          />
        </Stack>
      </Modal>
      {/* 独立渲染，避免父弹窗关闭时缓存内容导致子弹窗无法关闭。 */}
      <NodeManagementModal
        open={open && managementOpen}
        kbID={kb_id}
        nodeIDs={nav?.id ? [nav.id] : []}
        targetType='nav'
        onClose={() => setManagementOpen(false)}
        onSuccess={onSuccess}
      />
    </>
  );
}
