import Card from '@/components/Card';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import DragTree from '@/components/Drag/DragTree';
import { Form, FormItem } from '@/pages/setting/component/Common';
import {
  Department,
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
} from '@/request/Enterprise';
import { getApiV1NavPermission } from '@/request/NavCompat';
import { getApiV1UserListCompat } from '@/request/userCompat';
import { putApiV1NodeDetail } from '@/request/Node';
import {
  getApiV1NodePermission,
  patchApiV1NodePermissionEdit,
} from '@/request/NodePermission';
import {
  createNodeSummaryStream,
  subscribeNodeSummaryStream,
  type StreamSummaryEvent,
} from '@/request/nodeStream';
import { DomainNodeListItemResp, DomainNodeType } from '@/request/types';
import { useAppSelector } from '@/store';
import { convertToTree } from '@/utils/drag';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  getAncestorIds,
} from '@/utils/enterprise';
import { filterEmptyFolders } from '@/utils/tree';
import { Modal, message } from '@/components/ui';
import { Select, Switch } from 'antd';
import { Box, Button, Stack, TextField, styled } from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { IconShuaxin } from '@panda-wiki/icons';
import SSEClient from '@/utils/fetch';

interface DocPropertiesModalProps {
  open: boolean;
  onCancel: () => void;
  onOk: () => void;
  isBatch?: boolean;
  data: DomainNodeListItemResp[];
}

type PermissionScopeType = 'public' | 'custom';

type FormValues = {
  name: string;
  scope_type: PermissionScopeType;
  inherit: boolean;
  min_permission_level_id: string;
  allowed_department_ids: string[];
  allowed_user_ids: string[];
  summary: string;
};

const normalizePermissionScopeType = (
  scopeType?: string,
): PermissionScopeType =>
  scopeType === 'custom' || scopeType === 'department' ? 'custom' : 'public';

const hasExplicitPermissionRule = (permissions?: {
  scope_type?: string;
  min_permission_level_id?: string;
  min_permission_level_rank?: number;
  allowed_department_ids?: string[];
  allowed_user_ids?: string[];
  denied_department_ids?: string[];
  denied_user_ids?: string[];
}) =>
  !!permissions?.scope_type ||
  !!permissions?.min_permission_level_id ||
  !!permissions?.min_permission_level_rank ||
  !!permissions?.allowed_department_ids?.length ||
  !!permissions?.allowed_user_ids?.length ||
  !!permissions?.denied_department_ids?.length ||
  !!permissions?.denied_user_ids?.length;

const StyledText = styled('div')(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: 16,
}));

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
  const [loading, setLoading] = useState(false);
  const [summaryLoading, setSummaryLoading] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [permissionLevels, setPermissionLevels] = useState<
    { id: string; name: string; rank: number }[]
  >([]);
  const [users, setUsers] = useState<{ id: string; account: string }[]>([]);
  const [navScopeType, setNavScopeType] =
    useState<PermissionScopeType>('public');
  const [parentPermissionSummary, setParentPermissionSummary] = useState<{
    departmentIds: string[];
    departmentNames: string[];
    minPermissionLevelId: string;
  }>({
    departmentIds: [],
    departmentNames: [],
    minPermissionLevelId: '',
  });
  const sseClientRef = useRef<SSEClient<StreamSummaryEvent> | null>(null);
  const {
    control,
    handleSubmit,
    setValue,
    reset,
    watch,
    formState: { errors },
  } = useForm<FormValues>({
    defaultValues: {
      name: '',
      scope_type: 'public',
      inherit: true,
      min_permission_level_id: '',
      allowed_department_ids: [] as string[],
      allowed_user_ids: [] as string[],
      summary: '',
    },
  });

  const inheritPermission = watch('inherit');
  const scopeType = watch('scope_type');
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );
  const selectedLevel = permissionLevels.find(
    item => item.id === parentPermissionSummary.minPermissionLevelId,
  );
  const availablePermissionLevels = parentPermissionSummary.minPermissionLevelId
    ? permissionLevels.filter(item =>
        selectedLevel ? item.rank >= selectedLevel.rank : true,
      )
    : permissionLevels;
  const availableDepartments =
    scopeType === 'custom' && parentPermissionSummary.departmentIds.length > 0
      ? flatDepartments.filter(item =>
          parentPermissionSummary.departmentIds.includes(item.id),
        )
      : flatDepartments;
  const isFolder = data[0]?.type === DomainNodeType.NodeTypeFolder;
  const modalTitle = isBatch
    ? '批量设置权限'
    : isFolder
      ? '目录权限'
      : '文档权限';

  const onGenerateSummary = () => {
    setSummaryLoading(true);
    let nextSummary = '';
    sseClientRef.current?.unsubscribe();
    sseClientRef.current = createNodeSummaryStream({
      onComplete: () => setSummaryLoading(false),
      onError: error => {
        setSummaryLoading(false);
        message.error(error.message || '生成摘要失败');
      },
    });
    setValue('summary', '');
    subscribeNodeSummaryStream(
      sseClientRef.current,
      {
        ids: [data[0].id!],
        kb_id: kb_id!,
      },
      event => {
        if (event.type === 'data') {
          nextSummary += event.content || '';
          setValue('summary', nextSummary);
          return;
        }
        if (event.type === 'error') {
          setSummaryLoading(false);
          message.error(event.content || event.error || '生成摘要失败');
          sseClientRef.current?.unsubscribe();
        }
      },
    );
  };

  const onSubmit = handleSubmit(values => {
    setLoading(true);
    const permissions = values.inherit
      ? {
          inherit: true,
        }
      : {
          scope_type: values.scope_type,
          inherit: false,
          min_permission_level_id: values.min_permission_level_id || undefined,
          allowed_department_ids:
            values.scope_type === 'custom'
              ? [
                  ...new Set([
                    ...getAncestorIds(
                      values.allowed_department_ids || [],
                      departments,
                    ),
                    ...(values.allowed_department_ids || []),
                  ]),
                ]
              : [],
          allowed_user_ids:
            values.scope_type === 'custom' ? values.allowed_user_ids || [] : [],
        };
    Promise.all([
      patchApiV1NodePermissionEdit({
        kb_id: kb_id!,
        ids: data.map(item => item.id!),
        permissions,
      } as never),

      !isBatch
        ? putApiV1NodeDetail({
            id: data[0].id!,
            name: values.name,
            summary: values.summary,
            kb_id: kb_id!,
            nav_id: data[0].nav_id || nav_id || '',
          })
        : undefined,
    ])
      .then(() => {
        message.success('编辑成功');
        onOk();
      })
      .finally(() => setLoading(false));
  });

  const tree = filterEmptyFolders(convertToTree(data));

  useEffect(() => {
    if (open && data) {
      Promise.all([
        getApiV1EnterpriseDepartments(),
        getApiV1EnterprisePermissionLevels(),
        getApiV1UserListCompat(),
        !isBatch && data[0]?.nav_id
          ? getApiV1NavPermission({ kb_id: kb_id!, id: data[0].nav_id })
          : Promise.resolve(null),
        !isBatch && data[0]?.parent_id
          ? getApiV1NodePermission({ kb_id: kb_id!, id: data[0].parent_id })
          : Promise.resolve(null),
      ]).then(
        ([
          departmentResp,
          levelResp,
          userResp,
          navPermissionResp,
          parentNodePermissionResp,
        ]) => {
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
          const parentPermissions =
            (parentNodePermissionResp?.effective_permissions ||
              parentNodePermissionResp?.permissions ||
              navPermissionResp?.permissions ||
              {}) as {
              scope_type?: string;
              allowed_department_ids?: string[];
              min_permission_level_id?: string;
            };
          const scope = normalizePermissionScopeType(
            parentPermissions?.scope_type,
          );
          const parentDepartmentIDs =
            parentPermissions?.allowed_department_ids || [];
          setNavScopeType(scope);
          setValue('scope_type', scope);
          setParentPermissionSummary({
            departmentIds: parentDepartmentIDs,
            departmentNames: (departmentResp.departments || [])
              .filter(item => parentDepartmentIDs.includes(item.id))
              .map(item => item.name),
            minPermissionLevelId:
              parentPermissions?.min_permission_level_id || '',
          });
        },
      );
      if (isBatch) return;
      setValue('name', data[0].name!);
      setValue('summary', data[0].summary!);
      getApiV1NodePermission({
        kb_id: kb_id!,
        id: data[0].id!,
      }).then(res => {
        const permissions = res.permissions as
          | {
              inherit?: boolean;
              scope_type?: string;
              min_permission_level_id?: string;
              min_permission_level_rank?: number;
              allowed_department_ids?: string[];
              allowed_user_ids?: string[];
              denied_department_ids?: string[];
              denied_user_ids?: string[];
            }
          | undefined;
        if (permissions) {
          setValue(
            'inherit',
            permissions.inherit ?? !hasExplicitPermissionRule(permissions),
          );
          setValue(
            'scope_type',
            normalizePermissionScopeType(permissions.scope_type),
          );
          setValue(
            'min_permission_level_id',
            permissions.min_permission_level_id || '',
          );
          setValue(
            'allowed_department_ids',
            permissions.allowed_department_ids || [],
          );
          setValue('allowed_user_ids', permissions.allowed_user_ids || []);
        }
      });
    }
  }, [open, data, kb_id, isBatch, nav_id, reset, setValue]);

  useEffect(() => {
    if (!open) {
      reset();
    }
  }, [open]);

  useEffect(() => {
    return () => {
      sseClientRef.current?.unsubscribe();
    };
  }, []);

  return (
    <Modal
      title={modalTitle}
      open={open}
      onCancel={() => {
        sseClientRef.current?.unsubscribe();
        onCancel();
      }}
      width={700}
      okButtonProps={{
        loading: loading,
      }}
      onOk={onSubmit}
    >
      {isBatch && (
        <>
          <Box sx={{ fontSize: 14, mb: 1, color: 'text.secondary' }}>
            已选中
            <Box
              component={'span'}
              sx={{ fontWeight: 700, color: 'primary.main', px: 0.5 }}
            >
              {data.length}
            </Box>
            个节点，设置权限
          </Box>
          <Card
            sx={{
              p: 2,
              maxHeight: '300px',
              overflowY: 'auto',
              bgcolor: 'background.paper3',
              '& .dndkit-drag-handle': {
                top: '-2px !important',
              },
            }}
          >
            <DragTree data={tree} readOnly={true} supportSelect={false} />
          </Card>
        </>
      )}

      <Form labelWidth={100} gap={3}>
        {!isBatch && (
          <>
            <FormItem label='文档名称' required>
              <Controller
                name='name'
                control={control}
                rules={{ required: '文档名称不能为空' }}
                render={({ field }) => (
                  <TextField
                    {...field}
                    fullWidth
                    error={!!errors.name}
                    helperText={errors.name?.message as string}
                  />
                )}
              />
            </FormItem>
            <FormItem label='创建时间'>
              <StyledText>
                {data?.[0]?.created_at
                  ? dayjs(data[0].created_at).format('YYYY-MM-DD HH:mm:ss')
                  : '-'}
              </StyledText>
            </FormItem>
            <FormItem label='创建者'>
              <StyledText>{data?.[0]?.creator}</StyledText>
            </FormItem>
            <FormItem label='修改时间'>
              <StyledText>
                {data?.[0]?.updated_at
                  ? dayjs(data[0].updated_at).format('YYYY-MM-DD HH:mm:ss')
                  : '-'}
              </StyledText>
            </FormItem>
            <FormItem label='修改者'>
              <StyledText>{data?.[0]?.editor}</StyledText>
            </FormItem>
          </>
        )}
        <FormItem label='权限方式'>
          <Controller
            name='inherit'
            control={control}
            render={({ field }) => (
              <Stack direction='row' alignItems='center' gap={1}>
                <Switch
                  checked={field.value}
                  onChange={field.onChange}
                  checkedChildren='继承'
                  unCheckedChildren='单独'
                />
                <StyledText>
                  {field.value ? '继承父级权限' : '单独设置当前节点权限'}
                </StyledText>
              </Stack>
            )}
          />
        </FormItem>

        {!inheritPermission && (
          <FormItem label='权限范围'>
            <Controller
              name='scope_type'
              control={control}
              render={({ field }) => (
                <Select
                  value={field.value}
                  onChange={field.onChange}
                  style={{ width: '100%' }}
                  options={[
                    { value: 'public', label: '公开' },
                    { value: 'custom', label: '自定义权限' },
                  ]}
                />
              )}
            />
          </FormItem>
        )}

        <FormItem label='父级规则'>
          <Stack gap={0.5}>
            <StyledText>
              {navScopeType === 'custom'
                ? `自定义权限：${
                    parentPermissionSummary.departmentNames.length > 0
                      ? parentPermissionSummary.departmentNames.join(' / ')
                      : '未限制部门'
                  }`
                : '公开：全员可见'}
            </StyledText>
            {!levelsDisabled && (
              <StyledText>
                等级权限：
                {selectedLevel
                  ? `${selectedLevel.name}（${selectedLevel.rank}）及以上`
                  : '不限制'}
              </StyledText>
            )}
          </Stack>
        </FormItem>

        {!inheritPermission && !levelsDisabled && (
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
                    ...availablePermissionLevels.map(item => ({
                      value: item.id,
                      label: `${item.name}（${item.rank}）`,
                    })),
                  ]}
                />
              )}
            />
          </FormItem>
        )}

        {!inheritPermission && scopeType === 'custom' && (
          <>
            <FormItem label='允许部门'>
              <Controller
                name='allowed_department_ids'
                control={control}
                render={({ field }) => (
                  <DepartmentTreeSelect
                    departments={departmentTree}
                    flatDepartments={availableDepartments}
                    value={field.value || []}
                    onChange={field.onChange}
                    multiple
                    label='允许部门'
                    size='small'
                    excludeIds={flatDepartments
                      .filter(
                        d => !availableDepartments.some(ad => ad.id === d.id),
                      )
                      .map(d => d.id)}
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

        {!isBatch && (
          <FormItem label='内容摘要' sx={{ alignItems: 'flex-start' }}>
            <Controller
              name='summary'
              control={control}
              render={({ field }) => (
                <Stack sx={{ flex: 1 }} alignItems='flex-start'>
                  <TextField
                    {...field}
                    fullWidth
                    error={!!errors.name}
                    helperText={errors.name?.message as string}
                    multiline
                    minRows={4}
                  />
                  <Button
                    sx={{ minWidth: 'auto', mt: 1 }}
                    onClick={onGenerateSummary}
                    disabled={loading || summaryLoading}
                    startIcon={
                      <IconShuaxin
                        sx={{
                          fontSize: '16px !important',
                          ...(summaryLoading
                            ? { animation: 'loadingRotate 1s linear infinite' }
                            : {}),
                        }}
                      />
                    }
                  >
                    AI 生成
                  </Button>
                </Stack>
              )}
            />
          </FormItem>
        )}
      </Form>
    </Modal>
  );
};

export default DocPropertiesModal;
