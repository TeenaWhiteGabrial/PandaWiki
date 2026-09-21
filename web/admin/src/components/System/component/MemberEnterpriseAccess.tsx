import { FormItem } from '@/components/Form';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { Modal } from '@/components/ui';
import {
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
  putApiV1EnterpriseUsersAccess,
} from '@/request/Enterprise';
import type { Department, PermissionLevel } from '@/request/Enterprise';
import type { UserListItemCompat } from '@/request/userCompat';
import { ConstsUserRole } from '@/request/types';
import { useAppSelector } from '@/store';
import { Select } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  getAncestorIds,
} from '@/utils/enterprise';

type MemberEnterpriseAccessProps = {
  open: boolean;
  user: UserListItemCompat | null;
  onClose: () => void;
  refresh: () => void;
};

type FormValues = {
  permission_level_id: string;
  department_ids: string[];
};

const MemberEnterpriseAccess = ({
  open,
  user,
  onClose,
  refresh,
}: MemberEnterpriseAccessProps) => {
  const currentUser = useAppSelector(state => state.config.user);
  const [loading, setLoading] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const { control, handleSubmit, reset } = useForm<FormValues>({
    defaultValues: {
      permission_level_id: '',
      department_ids: [],
    },
  });

  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );

  useEffect(() => {
    if (!open || !user) return;
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ]).then(([departmentResp, levelResp]) => {
      setDepartments(departmentResp.departments || []);
      setLevels(levelResp.levels || []);
      reset({
        permission_level_id: user.permission_level_id || '',
        department_ids: user.department_ids || [],
      });
    });
  }, [open, user, reset]);

  if (!user) return null;
  const canEdit =
    currentUser?.role === ConstsUserRole.UserRoleAdmin ||
    user.role === ConstsUserRole.UserRoleUser;

  return (
    <Modal
      title='编辑组织和权限'
      open={open}
      onCancel={onClose}
      okButtonProps={{ loading, disabled: !canEdit }}
      onOk={handleSubmit(values => {
        if (!canEdit) return;
        setLoading(true);
        // 补全祖先部门 ID
        const ancestorIds = getAncestorIds(values.department_ids, departments);
        const fullDepartmentIds = [
          ...new Set([...ancestorIds, ...values.department_ids]),
        ];
        putApiV1EnterpriseUsersAccess({
          user_id: user.id || '',
          permission_level_id: values.permission_level_id || undefined,
          department_ids: fullDepartmentIds,
        })
          .then(() => {
            refresh();
            onClose();
          })
          .finally(() => setLoading(false));
      })}
    >
      {!currentUser?.permission_levels_disabled && (
        <FormItem label='权限等级'>
          <Controller
            name='permission_level_id'
            control={control}
            render={({ field }) => (
              <Select
                value={field.value}
                onChange={field.onChange}
                style={{ width: '100%' }}
                options={[
                  { value: '', label: '默认' },
                  ...levels.map(item => ({
                    value: item.id,
                    label: `${item.name}（${item.rank}）`,
                  })),
                ]}
              />
            )}
          />
        </FormItem>
      )}
      <FormItem label='所属部门' sx={{ mt: 2 }}>
        <Controller
          name='department_ids'
          control={control}
          render={({ field }) => (
            <DepartmentTreeSelect
              departments={departmentTree}
              flatDepartments={flatDepartments}
              value={field.value || []}
              onChange={field.onChange}
              multiple={false}
              label='所属部门'
              disabled={!canEdit}
            />
          )}
        />
      </FormItem>
    </Modal>
  );
};

export default MemberEnterpriseAccess;
