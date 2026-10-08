import { useAppSelector } from '@/store';
import { Form } from '@/pages/setting/component/Common';
import { Department } from '@/request/Enterprise';
import {
  getApiV1NavPermission,
  patchApiV1NavPermission,
} from '@/request/NavCompat';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import { V1NavListResp } from '@/request/types';
import { message, Modal } from '@/components/ui';
import { Alert, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import NodeManagementModal from '@/pages/document/component/NodeManagementModal';
import ReadingPermissionFields, {
  ReadingPermissionValue,
  ReadingScope,
} from '@/pages/document/component/ReadingPermissionFields';
import ReadingPermissionSummary from '@/pages/document/component/ReadingPermissionSummary';
import {
  ManagementPermissionSection,
  PermissionTarget,
  ReadingPermissionSection,
} from '@/pages/document/component/PermissionSettingsSections';

const emptyPermissions = (): ReadingPermissionValue => ({
  scope_type: 'public',
  min_permission_level_id: '',
  allowed_permission_level_ids: [],
  allowed_department_ids: [],
  allowed_user_ids: [],
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
  const [loading, setLoading] = useState(false);
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
        const scope = [
          'custom',
          'permission_level',
          'department',
          'user',
        ].includes(permissions.scope_type || '')
          ? (permissions.scope_type as ReadingScope)
          : 'public';
        setValue({
          scope_type: scope,
          min_permission_level_id: permissions.min_permission_level_id || '',
          allowed_permission_level_ids:
            permissions.allowed_permission_level_ids || [],
          allowed_department_ids: permissions.allowed_department_ids || [],
          allowed_user_ids: permissions.allowed_user_ids || [],
        });
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
  const submit = async () => {
    if (!nav?.id || !ready) return;
    const scope = value.scope_type;
    if (
      (scope === 'permission_level' &&
        !value.allowed_permission_level_ids.length) ||
      (scope === 'department' && !value.allowed_department_ids.length) ||
      (scope === 'user' && !value.allowed_user_ids.length)
    ) {
      message.warning('请选择查阅权限的授权对象');
      return;
    }
    setLoading(true);
    try {
      await patchApiV1NavPermission({
        kb_id,
        id: nav.id,
        permissions: {
          scope_type: scope,
          min_permission_level_id:
            scope === 'public' || scope === 'custom'
              ? value.min_permission_level_id || undefined
              : undefined,
          allowed_permission_level_ids:
            scope === 'permission_level'
              ? value.allowed_permission_level_ids
              : [],
          allowed_department_ids:
            scope === 'department' || scope === 'custom'
              ? value.allowed_department_ids
              : [],
          allowed_user_ids:
            scope === 'user' || scope === 'custom'
              ? value.allowed_user_ids
              : [],
        },
      });
      message.success('目录查阅权限已更新');
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
        okButtonProps={{ loading, disabled: !ready }}
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
          <ReadingPermissionSection>
            <Typography fontSize={13} color='text.secondary'>
              这是一级栏目，没有上级目录。请选择可以查阅栏目内容的用户。
            </Typography>
            <Form labelWidth={100} gap={2}>
              <ReadingPermissionFields
                value={value}
                onChange={patch => setValue(prev => ({ ...prev, ...patch }))}
                levelsDisabled={levelsDisabled}
                levels={levels}
                departments={departments}
                users={users}
                disabled={!ready || loading}
              />
            </Form>
            {ready ? (
              <ReadingPermissionSummary
                permissions={value}
                label='当前设置允许查阅的用户'
                levels={levels}
                departments={departments}
                users={users}
                levelsDisabled={levelsDisabled}
              />
            ) : (
              <Typography fontSize={13} color='text.secondary'>
                {loadError ? '暂时无法显示查阅规则。' : '正在读取查阅规则…'}
              </Typography>
            )}
            <Typography fontSize={12} color='text.secondary'>
              栏目内的所有文档都受此规则约束。文档单独设置查阅权限时，用户需要同时满足两处规则。
            </Typography>
          </ReadingPermissionSection>
          <ManagementPermissionSection
            onConfigure={() => setManagementOpen(true)}
            disabled={!ready || loading}
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
