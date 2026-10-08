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
  DomainNodeListItemResp,
  DomainNodeType,
  DomainNodePermissions,
} from '@/request/types';
import { useAppSelector } from '@/store';
import {
  Alert,
  Box,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  Typography,
} from '@mui/material';
import { useEffect, useState } from 'react';
import NodeManagementModal from './NodeManagementModal';
import ReadingPermissionFields, {
  ReadingPermissionValue,
  ReadingScope,
} from './ReadingPermissionFields';
import ReadingPermissionSummary from './ReadingPermissionSummary';
import {
  ManagementPermissionSection,
  PermissionTarget,
  ReadingPermissionSection,
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
  min_permission_level_id: '',
  allowed_permission_level_ids: [],
  allowed_department_ids: [],
  allowed_user_ids: [],
});

const hasExplicitPermissionRule = (permissions: DomainNodePermissions) =>
  !!permissions.scope_type ||
  !!permissions.allowed_permission_level_ids?.length ||
  !!permissions.min_permission_level_id ||
  !!permissions.min_permission_level_rank ||
  !!permissions.allowed_department_ids?.length ||
  !!permissions.allowed_user_ids?.length ||
  !!permissions.denied_department_ids?.length ||
  !!permissions.denied_user_ids?.length;

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
  const { kb_id, nav_id } = useAppSelector(state => state.config);
  const [managementOpen, setManagementOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [inherit, setInherit] = useState(true);
  const [value, setValue] = useState<ReadingPermissionValue>(emptyValue);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<
    { id: string; name: string; rank: number }[]
  >([]);
  const [users, setUsers] = useState<{ id: string; account: string }[]>([]);
  const [parentPermissions, setParentPermissions] =
    useState<DomainNodePermissions>({});
  const isFolder = data[0]?.type === DomainNodeType.NodeTypeFolder;
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
    setInherit(true);
    setValue(emptyValue());
    setParentPermissions({});
    const node = data[0];
    const currentNavID = node.nav_id || nav_id;
    // 栏目根的空 ID、栏目 ID 和旧 nav:ID 都不属于 nodes 父节点。
    const parentNodeID =
      node.parent_id &&
      !node.parent_id.startsWith('nav:') &&
      node.parent_id !== currentNavID
        ? node.parent_id
        : '';
    Promise.all([
      getNodePermissionOptions(kb_id),
      !isBatch && currentNavID
        ? getApiV1NavPermission({ kb_id, id: currentNavID })
        : Promise.resolve(null),
      !isBatch && parentNodeID
        ? getApiV1NodePermission({ kb_id, id: parentNodeID })
        : Promise.resolve(null),
      !isBatch
        ? getApiV1NodePermission({ kb_id, id: node.id! })
        : Promise.resolve(null),
    ])
      .then(([options, navResponse, parentResponse, response]) => {
        if (!active) return;
        setDepartments(options.departments || []);
        setLevels(options.levels || []);
        setUsers(options.users || []);
        setParentPermissions(
          parentResponse?.effective_permissions ||
            parentResponse?.permissions ||
            navResponse?.permissions ||
            {},
        );
        const permissions = response?.permissions || {};
        setInherit(
          permissions.inherit ?? !hasExplicitPermissionRule(permissions),
        );
        const scope = [
          'custom',
          'department',
          'permission_level',
          'user',
        ].includes(permissions.scope_type || '')
          ? (permissions.scope_type as ReadingScope)
          : permissions.allowed_department_ids?.length ||
              permissions.allowed_user_ids?.length
            ? 'custom'
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
      });
    return () => {
      active = false;
    };
  }, [open, data, kb_id, isBatch, nav_id]);

  const submit = async () => {
    if (!ready || saving || !kb_id) return;
    const scope = value.scope_type;
    if (
      !inherit &&
      ((scope === 'permission_level' &&
        !value.allowed_permission_level_ids.length) ||
        (scope === 'department' && !value.allowed_department_ids.length) ||
        (scope === 'user' && !value.allowed_user_ids.length))
    ) {
      message.warning('请选择可以查阅的对象');
      return;
    }
    setSaving(true);
    try {
      await patchApiV1NodePermissionEdit({
        kb_id,
        ids: data.map(item => item.id!),
        permissions: inherit
          ? { inherit: true }
          : {
              inherit: false,
              scope_type: scope,
              min_permission_level_id:
                scope === 'custom' || scope === 'public'
                  ? value.min_permission_level_id || undefined
                  : undefined,
              allowed_permission_level_ids:
                scope === 'permission_level'
                  ? value.allowed_permission_level_ids
                  : [],
              allowed_department_ids:
                scope === 'custom' || scope === 'department'
                  ? value.allowed_department_ids
                  : [],
              allowed_user_ids:
                scope === 'custom' || scope === 'user'
                  ? value.allowed_user_ids
                  : [],
            },
      });
      message.success('查阅权限已保存');
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
        okButtonProps={{ loading: saving, disabled: !ready }}
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
          <ReadingPermissionSection>
            <Typography fontSize={13} color='text.secondary'>
              {isBatch
                ? '以下设置会应用到全部所选文档和目录。'
                : isFolder
                  ? '控制哪些用户可以查看目录内的内容。'
                  : '控制哪些用户可以查看这份文档。'}
            </Typography>
            <RadioGroup
              aria-label='查阅权限设置方式'
              value={inherit ? 'inherit' : 'custom'}
              onChange={(_, next) => setInherit(next === 'inherit')}
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' },
                gap: 1.5,
              }}
            >
              {[
                {
                  value: 'inherit',
                  title: '跟随上级',
                  description: '使用上级的规则，上级调整后自动跟随。',
                },
                {
                  value: 'custom',
                  title: '单独设置',
                  description: '为当前文档或目录选择可查阅的用户。',
                },
              ].map(option => (
                <FormControlLabel
                  key={option.value}
                  value={option.value}
                  disabled={!ready || saving}
                  control={<Radio size='small' />}
                  sx={{
                    m: 0,
                    p: 1,
                    alignItems: 'flex-start',
                    border: '1px solid',
                    borderColor:
                      (inherit ? 'inherit' : 'custom') === option.value
                        ? 'primary.main'
                        : 'divider',
                    borderRadius: 1.5,
                  }}
                  label={
                    <Box sx={{ pt: 0.75 }}>
                      <Typography fontSize={14} fontWeight={500}>
                        {option.title}
                      </Typography>
                      <Typography fontSize={12} color='text.secondary'>
                        {option.description}
                      </Typography>
                    </Box>
                  }
                />
              ))}
            </RadioGroup>
            {!ready ? (
              <Typography fontSize={13} color='text.secondary'>
                {loadError ? '暂时无法显示查阅规则。' : '正在读取查阅规则…'}
              </Typography>
            ) : inherit ? (
              isBatch ? (
                <Alert severity='info' icon={false}>
                  每个文档或目录将分别使用各自上级的查阅规则。
                </Alert>
              ) : (
                <ReadingPermissionSummary
                  permissions={parentPermissions}
                  label='上级允许查阅的用户'
                  levels={levels}
                  departments={departments}
                  users={users}
                  levelsDisabled={levelsDisabled}
                />
              )
            ) : (
              <>
                <Form labelWidth={100} gap={2}>
                  <ReadingPermissionFields
                    value={value}
                    onChange={patch =>
                      setValue(prev => ({ ...prev, ...patch }))
                    }
                    levelsDisabled={levelsDisabled}
                    levels={levels}
                    departments={departments}
                    users={users}
                    disabled={saving}
                  />
                </Form>
                <ReadingPermissionSummary
                  permissions={value}
                  label='当前设置允许查阅的用户'
                  levels={levels}
                  departments={departments}
                  users={users}
                  levelsDisabled={levelsDisabled}
                />
              </>
            )}
            <Typography fontSize={12} color='text.secondary'>
              访问文档时，还需满足所属一级栏目的查阅限制。
              {isFolder && '下级文档和目录选择“跟随上级”时，会使用这里的规则。'}
            </Typography>
          </ReadingPermissionSection>
          <ManagementPermissionSection
            onConfigure={() => setManagementOpen(true)}
            disabled={!ready || saving}
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
