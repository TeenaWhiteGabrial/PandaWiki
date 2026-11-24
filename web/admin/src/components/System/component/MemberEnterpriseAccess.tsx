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
  getDeepestDepartmentIds,
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
  const [optionsStatus, setOptionsStatus] = useState<
    'loading' | 'ready' | 'error'
  >('loading');
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const {
    control,
    handleSubmit,
    reset,
    formState: { dirtyFields },
  } = useForm<FormValues>({
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
    let active = true;
    setOptionsStatus('loading');
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ])
      .then(([departmentResp, levelResp]) => {
        if (!active) return;
        const allDepartments = departmentResp.departments || [];
        setDepartments(allDepartments);
        setLevels(levelResp.levels || []);
        reset({
          permission_level_id: user.permission_level_id || '',
          department_ids: getDeepestDepartmentIds(
            user.department_ids || [],
            allDepartments,
          ),
        });
        setOptionsStatus('ready');
      })
      .catch(() => {
        if (active) setOptionsStatus('error');
      });
    return () => {
      active = false;
    };
  }, [open, user, reset]);

  if (!user) return null;
  const canEdit =
    currentUser?.role === ConstsUserRole.UserRoleAdmin ||
    user.role === ConstsUserRole.UserRoleUser;
  const formReady = optionsStatus === 'ready';

  return (
    <Modal
      title='编辑组织和权限'
      open={open}
      onCancel={onClose}
      okButtonProps={{ loading, disabled: !canEdit || !formReady }}
      onOk={handleSubmit(values => {
        if (!canEdit || !formReady) return;
        setLoading(true);
        const departmentIds = dirtyFields.department_ids
          ? [
              ...new Set([
                ...getAncestorIds(values.department_ids, departments),
                ...values.department_ids,
              ]),
            ]
          : user.department_ids || [];
        putApiV1EnterpriseUsersAccess({
          user_id: user.id || '',
          permission_level_id: values.permission_level_id || undefined,
          department_ids: departmentIds,
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
                disabled={!canEdit || !formReady}
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
              showFullPath
              label='所属部门'
              disabled={!canEdit || !formReady}
              placeholder={
                optionsStatus === 'loading' ? '正在加载部门…' : undefined
              }
              error={optionsStatus === 'error'}
              helperText={
                optionsStatus === 'error'
                  ? '读取部门或权限等级失败，请重新打开后重试'
                  : undefined
              }
            />
          )}
        />
      </FormItem>
    </Modal>
  );
};

export default MemberEnterpriseAccess;
