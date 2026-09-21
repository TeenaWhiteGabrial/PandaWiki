import { useAppSelector } from '@/store';
import { Form, FormItem } from '@/pages/setting/component/Common';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import {
  Department,
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
} from '@/request/Enterprise';
import {
  getApiV1NavPermission,
  patchApiV1NavPermission,
} from '@/request/NavCompat';
import { getApiV1UserListCompat } from '@/request/userCompat';
import { V1NavListResp } from '@/request/types';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  getAncestorIds,
} from '@/utils/enterprise';
import { message, Modal } from '@/components/ui';
import { Select } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

type FormValues = {
  scope_type: string;
  min_permission_level_id: string;
  allowed_department_ids: string[];
  allowed_user_ids: string[];
};

interface NavPermissionModalProps {
  open: boolean;
  nav: V1NavListResp | null;
  kb_id: string;
  onClose: () => void;
  onSuccess: () => void;
}

const NavPermissionModal = ({
  open,
  nav,
  kb_id,
  onClose,
  onSuccess,
}: NavPermissionModalProps) => {
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const [loading, setLoading] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [permissionLevels, setPermissionLevels] = useState<
    { id: string; name: string; rank: number }[]
  >([]);
  const [users, setUsers] = useState<{ id: string; account: string }[]>([]);
  const { control, handleSubmit, reset, setValue, watch } = useForm<FormValues>(
    {
      defaultValues: {
        scope_type: 'public',
        min_permission_level_id: '',
        allowed_department_ids: [],
        allowed_user_ids: [],
      },
    },
  );

  const scopeType = watch('scope_type');
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );

  useEffect(() => {
    if (!open || !nav?.id) return;
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
      getApiV1UserListCompat(),
      getApiV1NavPermission({ kb_id, id: nav.id }),
    ]).then(([departmentResp, levelResp, userResp, permissionResp]) => {
      setDepartments(departmentResp.departments || []);
      setPermissionLevels(
        (levelResp.levels || []).map(item => ({
          id: item.id,
          name: item.name,
          rank: item.rank,
        })),
      );
      setUsers(
        (userResp.users || []).map(item => ({
          id: item.id || '',
          account: item.account || '',
        })),
      );
      const permissions = permissionResp.permissions || {};
      setValue('scope_type', permissions.scope_type || 'public');
      setValue(
        'min_permission_level_id',
        permissions.min_permission_level_id || '',
      );
      setValue(
        'allowed_department_ids',
        permissions.allowed_department_ids || [],
      );
      setValue('allowed_user_ids', permissions.allowed_user_ids || []);
    });
  }, [open, nav?.id, kb_id, setValue]);

  useEffect(() => {
    if (!open) {
      reset();
    }
  }, [open, reset]);

  const submit = handleSubmit(values => {
    if (!nav?.id) return;
    setLoading(true);
    patchApiV1NavPermission({
      kb_id,
      id: nav.id,
      permissions: {
        scope_type: values.scope_type,
        min_permission_level_id: values.min_permission_level_id || undefined,
        allowed_department_ids: [
          ...new Set([
            ...getAncestorIds(values.allowed_department_ids, departments),
            ...values.allowed_department_ids,
          ]),
        ],
        allowed_user_ids: values.allowed_user_ids,
      },
    })
      .then(() => {
        message.success('栏目权限已更新');
        onSuccess();
      })
      .finally(() => setLoading(false));
  });

  return (
    <Modal
      title='栏目权限'
      open={open}
      width={700}
      okButtonProps={{ loading }}
      onCancel={onClose}
      onOk={submit}
    >
      <Form labelWidth={100} gap={3}>
        <FormItem label='栏目类型'>
          <Controller
            name='scope_type'
            control={control}
            render={({ field }) => (
              <Select
                value={field.value}
                onChange={field.onChange}
                style={{ width: '100%' }}
                options={[
                  { value: 'public', label: '公共栏目' },
                  { value: 'department', label: '部门栏目' },
                ]}
              />
            )}
          />
        </FormItem>
        {!levelsDisabled && (
          <FormItem label='最低权限等级'>
            <Controller
              name='min_permission_level_id'
              control={control}
              render={({ field }) => (
                <Select
                  value={field.value}
                  onChange={field.onChange}
                  style={{ width: '100%' }}
                  options={[
                    { value: '', label: '不限制' },
                    ...permissionLevels.map(item => ({
                      value: item.id,
                      label: `${item.name}（${item.rank}）`,
                    })),
                  ]}
                />
              )}
            />
          </FormItem>
        )}
        {scopeType === 'department' && (
          <>
            <FormItem label='允许部门'>
              <Controller
                name='allowed_department_ids'
                control={control}
                render={({ field }) => (
                  <DepartmentTreeSelect
                    departments={departmentTree}
                    flatDepartments={flatDepartments}
                    value={field.value || []}
                    onChange={field.onChange}
                    multiple
                    label='允许部门'
                    size='small'
                  />
                )}
              />
            </FormItem>
            <FormItem label='允许用户'>
              <Controller
                name='allowed_user_ids'
                control={control}
                render={({ field }) => (
                  <Select
                    mode='multiple'
                    value={field.value || []}
                    onChange={field.onChange}
                    style={{ width: '100%' }}
                    options={users.map(item => ({
                      value: item.id,
                      label: item.account,
                    }))}
                  />
                )}
              />
            </FormItem>
          </>
        )}
      </Form>
    </Modal>
  );
};

export default NavPermissionModal;
