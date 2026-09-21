import { useAppSelector } from '@/store';
import Card from '@/components/Card';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { FormItem } from '@/components/Form';
import { AntdTable, Modal } from '@/components/ui';
import type { ColumnType } from '@/components/ui';
import {
  deleteApiV1EnterpriseDepartments,
  deleteApiV1EnterprisePermissionLevels,
  Department,
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
  PermissionLevel,
  postApiV1EnterpriseDepartments,
  postApiV1EnterprisePermissionLevels,
  putApiV1EnterpriseDepartments,
  putApiV1EnterprisePermissionLevels,
} from '@/request/Enterprise';
import { Select } from 'antd';
import {
  Box,
  Button,
  IconButton,
  Stack,
  Switch,
  TextField,
} from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  buildDepartmentTree,
  flattenExpandedDepartmentTree,
  flattenDepartmentTree,
} from '@/utils/enterprise';
import { KeyboardArrowDown, KeyboardArrowRight } from '@mui/icons-material';

type DepartmentForm = {
  id?: string;
  name: string;
  parent_id: string;
  sort: number;
  status: 'active' | 'disabled';
};

type PermissionLevelForm = {
  id?: string;
  name: string;
  code: string;
  rank: number;
  description: string;
  status: 'active' | 'disabled';
  is_default: boolean;
};

type DepartmentRow = Omit<Department, 'children'> & {
  hasChildren: boolean;
};

const EnterpriseAccess = () => {
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const [loading, setLoading] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const [departmentModalOpen, setDepartmentModalOpen] = useState(false);
  const [levelModalOpen, setLevelModalOpen] = useState(false);
  const [editingDepartment, setEditingDepartment] = useState<Department | null>(
    null,
  );
  const [departmentModalMode, setDepartmentModalMode] = useState<
    'root' | 'child' | 'edit'
  >('root');
  const [parentDepartment, setParentDepartment] = useState<Department | null>(
    null,
  );
  const [editingLevel, setEditingLevel] = useState<PermissionLevel | null>(
    null,
  );
  const [expandedDepartmentIds, setExpandedDepartmentIds] = useState<
    Set<string>
  >(new Set());

  const {
    control: departmentControl,
    handleSubmit: handleDepartmentSubmit,
    reset: resetDepartment,
  } = useForm<DepartmentForm>({
    defaultValues: {
      name: '',
      parent_id: '',
      sort: 0,
      status: 'active',
    },
  });

  const {
    control: levelControl,
    handleSubmit: handleLevelSubmit,
    reset: resetLevel,
  } = useForm<PermissionLevelForm>({
    defaultValues: {
      name: '',
      code: '',
      rank: 0,
      description: '',
      status: 'active',
      is_default: false,
    },
  });

  const fetchData = () => {
    setLoading(true);
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ])
      .then(([departmentResp, levelResp]) => {
        const nextDepartments = departmentResp.departments || [];
        setDepartments(nextDepartments);
        const nextTree = buildDepartmentTree(nextDepartments);
        const expandedIds = new Set<string>();
        const collect = (nodes: Department[]) => {
          nodes.forEach(node => {
            if (node.children?.length) {
              expandedIds.add(node.id);
              collect(node.children);
            }
          });
        };
        collect(nextTree);
        setExpandedDepartmentIds(expandedIds);
        setLevels(levelResp.levels || []);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchData();
  }, []);

  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );
  const visibleDepartments = useMemo(
    () => flattenExpandedDepartmentTree(departmentTree, expandedDepartmentIds),
    [departmentTree, expandedDepartmentIds],
  );
  const visibleDepartmentRows = useMemo(
    () =>
      visibleDepartments.map(({ children, ...rest }) => ({
        ...rest,
        hasChildren: !!children?.length,
      })),
    [visibleDepartments],
  );

  const departmentColumns: ColumnType<DepartmentRow>[] = useMemo(
    () => [
      {
        title: '部门名称',
        dataIndex: 'name',
        render: (value: string, record) => {
          const isExpanded = expandedDepartmentIds.has(record.id);
          const depth = record.path
            ? Math.max(0, record.path.split('/').length - 1)
            : 0;
          return (
            <Stack direction='row' alignItems='center' spacing={0.5}>
              <Box sx={{ width: `${depth * 24}px`, flexShrink: 0 }} />
              {record.hasChildren ? (
                <IconButton
                  size='small'
                  sx={{ p: 0.25 }}
                  onClick={() => {
                    setExpandedDepartmentIds(prev => {
                      const next = new Set(prev);
                      if (next.has(record.id)) {
                        next.delete(record.id);
                      } else {
                        next.add(record.id);
                      }
                      return next;
                    });
                  }}
                >
                  {isExpanded ? (
                    <KeyboardArrowDown fontSize='small' />
                  ) : (
                    <KeyboardArrowRight fontSize='small' />
                  )}
                </IconButton>
              ) : (
                <Box sx={{ width: 26, flexShrink: 0 }} />
              )}
              <Box>{value}</Box>
            </Stack>
          );
        },
      },
      {
        title: '父部门',
        dataIndex: 'parent_id',
        render: (value: string) =>
          departments.find(item => item.id === value)?.name || '-',
      },
      {
        title: '状态',
        dataIndex: 'status',
        render: (value: string) => (value === 'active' ? '启用' : '停用'),
      },
      {
        title: '操作',
        dataIndex: 'action',
        render: (_: unknown, record: Department) => (
          <Stack direction='row' gap={2}>
            <Button
              size='small'
              sx={{ p: 0, minWidth: 'auto' }}
              onClick={() => {
                setDepartmentModalMode('child');
                setParentDepartment(record);
                setEditingDepartment(null);
                resetDepartment({
                  name: '',
                  parent_id: record.id,
                  sort: 0,
                  status: 'active',
                });
                setDepartmentModalOpen(true);
              }}
            >
              添加子部门
            </Button>
            <Button
              size='small'
              sx={{ p: 0, minWidth: 'auto' }}
              onClick={() => {
                setDepartmentModalMode('edit');
                setParentDepartment(
                  record.parent_id
                    ? departments.find(item => item.id === record.parent_id) ||
                        null
                    : null,
                );
                setEditingDepartment(record);
                resetDepartment({
                  id: record.id,
                  name: record.name,
                  parent_id: record.parent_id || '',
                  sort: record.sort,
                  status: record.status,
                });
                setDepartmentModalOpen(true);
              }}
            >
              编辑
            </Button>
            <Button
              size='small'
              color='error'
              sx={{ p: 0, minWidth: 'auto' }}
              onClick={() => {
                Modal.confirm({
                  title: '确认删除部门？',
                  content: `删除“${record.name}”后将无法恢复，请确认该部门没有用户。`,
                  okText: '确认删除',
                  cancelText: '取消',
                  okButtonProps: { sx: { bgcolor: 'error.main' } },
                  onOk: async () => {
                    try {
                      await deleteApiV1EnterpriseDepartments(record.id);
                      fetchData();
                    } catch {
                      // API error is shown by the shared HTTP client.
                    }
                  },
                });
              }}
            >
              删除
            </Button>
          </Stack>
        ),
      },
    ],
    [departments, expandedDepartmentIds, resetDepartment],
  );

  const levelColumns: ColumnType<PermissionLevel>[] = useMemo(
    () => [
      { title: '名称', dataIndex: 'name' },
      { title: '编码', dataIndex: 'code' },
      { title: '等级', dataIndex: 'rank' },
      { title: '说明', dataIndex: 'description' },
      {
        title: '默认',
        dataIndex: 'is_default',
        render: (value: boolean) => (value ? '是' : '否'),
      },
      {
        title: '状态',
        dataIndex: 'status',
        render: (value: string) => (value === 'active' ? '启用' : '停用'),
      },
      {
        title: '操作',
        dataIndex: 'action',
        render: (_: unknown, record: PermissionLevel) => (
          <Stack direction='row' gap={2}>
            <Button
              size='small'
              sx={{ p: 0, minWidth: 'auto' }}
              onClick={() => {
                setEditingLevel(record);
                resetLevel({
                  id: record.id,
                  name: record.name,
                  code: record.code,
                  rank: record.rank,
                  description: record.description,
                  status: record.status,
                  is_default: record.is_default,
                });
                setLevelModalOpen(true);
              }}
            >
              编辑
            </Button>
            <Button
              size='small'
              color='error'
              sx={{ p: 0, minWidth: 'auto' }}
              onClick={() =>
                deleteApiV1EnterprisePermissionLevels(record.id).then(fetchData)
              }
            >
              删除
            </Button>
          </Stack>
        ),
      },
    ],
    [resetLevel],
  );

  return (
    <Stack gap={2}>
      <Card sx={{ p: 2, border: '1px solid', borderColor: 'divider' }}>
        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
          sx={{ mb: 2 }}
        >
          <Box sx={{ fontSize: 14, fontWeight: 'bold' }}>部门管理</Box>
          <Button
            size='small'
            variant='outlined'
            onClick={() => {
              setDepartmentModalMode('root');
              setParentDepartment(null);
              setEditingDepartment(null);
              resetDepartment({
                name: '',
                parent_id: '',
                sort: 0,
                status: 'active',
              });
              setDepartmentModalOpen(true);
            }}
          >
            添加一级部门
          </Button>
        </Stack>
        <AntdTable
          rowKey='id'
          dataSource={visibleDepartmentRows}
          columns={departmentColumns}
          loading={loading}
          pagination={false}
          size='small'
        />
      </Card>

      <Card
        sx={{
          display: levelsDisabled ? 'none' : undefined,
          p: 2,
          border: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
          sx={{ mb: 2 }}
        >
          <Box sx={{ fontSize: 14, fontWeight: 'bold' }}>权限等级管理</Box>
          <Button
            size='small'
            variant='outlined'
            onClick={() => {
              setEditingLevel(null);
              resetLevel({
                name: '',
                code: '',
                rank: 0,
                description: '',
                status: 'active',
                is_default: false,
              });
              setLevelModalOpen(true);
            }}
          >
            添加等级
          </Button>
        </Stack>
        <AntdTable
          rowKey='id'
          dataSource={levels}
          columns={levelColumns}
          loading={loading}
          pagination={false}
          size='small'
        />
      </Card>

      <Modal
        title={
          departmentModalMode === 'edit'
            ? '编辑部门'
            : departmentModalMode === 'child'
              ? '添加子部门'
              : '添加一级部门'
        }
        open={departmentModalOpen}
        onCancel={() => setDepartmentModalOpen(false)}
        onOk={handleDepartmentSubmit(values => {
          const payload = {
            ...values,
            parent_id: values.parent_id || undefined,
            sort: Number(values.sort) || 0,
          };
          const request = editingDepartment
            ? putApiV1EnterpriseDepartments(payload)
            : postApiV1EnterpriseDepartments(payload);
          request.then(() => {
            setDepartmentModalOpen(false);
            fetchData();
          });
        })}
      >
        <FormItem label='部门名称' required>
          <Controller
            name='name'
            control={departmentControl}
            rules={{ required: '部门名称不能为空' }}
            render={({ field, fieldState }) => (
              <TextField
                {...field}
                fullWidth
                error={!!fieldState.error}
                helperText={fieldState.error?.message}
              />
            )}
          />
        </FormItem>
        {departmentModalMode === 'root' && (
          <FormItem label='父部门' sx={{ mt: 2 }}>
            <TextField value='无' fullWidth disabled />
          </FormItem>
        )}
        {departmentModalMode === 'child' && (
          <FormItem label='父部门' sx={{ mt: 2 }}>
            <TextField
              value={parentDepartment?.name || ''}
              fullWidth
              disabled
            />
          </FormItem>
        )}
        {departmentModalMode === 'edit' && (
          <FormItem label='父部门' sx={{ mt: 2 }}>
            <Controller
              name='parent_id'
              control={departmentControl}
              render={({ field }) => (
                <DepartmentTreeSelect
                  departments={departmentTree}
                  flatDepartments={flatDepartments}
                  value={field.value ? [field.value] : []}
                  onChange={newValue => {
                    field.onChange(newValue[0] || '');
                  }}
                  multiple={false}
                  label='父部门'
                  excludeIds={
                    editingDepartment
                      ? [
                          editingDepartment.id,
                          ...flatDepartments
                            .filter(d => d.parent_id === editingDepartment.id)
                            .map(d => d.id),
                        ]
                      : []
                  }
                />
              )}
            />
          </FormItem>
        )}
        <FormItem label='排序' sx={{ mt: 2 }}>
          <Controller
            name='sort'
            control={departmentControl}
            render={({ field }) => (
              <TextField {...field} fullWidth type='number' />
            )}
          />
        </FormItem>
        <FormItem label='状态' sx={{ mt: 2 }}>
          <Controller
            name='status'
            control={departmentControl}
            render={({ field }) => (
              <Select
                value={field.value}
                onChange={field.onChange}
                style={{ width: '100%' }}
                options={[
                  { value: 'active', label: '启用' },
                  { value: 'disabled', label: '停用' },
                ]}
              />
            )}
          />
        </FormItem>
      </Modal>

      <Modal
        title={editingLevel ? '编辑权限等级' : '添加权限等级'}
        open={levelModalOpen}
        onCancel={() => setLevelModalOpen(false)}
        onOk={handleLevelSubmit(values => {
          const payload = {
            ...values,
            rank: Number(values.rank) || 0,
          };
          const request = editingLevel
            ? putApiV1EnterprisePermissionLevels(payload)
            : postApiV1EnterprisePermissionLevels(payload);
          request.then(() => {
            setLevelModalOpen(false);
            fetchData();
          });
        })}
      >
        <FormItem label='名称' required>
          <Controller
            name='name'
            control={levelControl}
            rules={{ required: '名称不能为空' }}
            render={({ field, fieldState }) => (
              <TextField
                {...field}
                fullWidth
                error={!!fieldState.error}
                helperText={fieldState.error?.message}
              />
            )}
          />
        </FormItem>
        <FormItem label='编码' required sx={{ mt: 2 }}>
          <Controller
            name='code'
            control={levelControl}
            rules={{ required: '编码不能为空' }}
            render={({ field, fieldState }) => (
              <TextField
                {...field}
                fullWidth
                error={!!fieldState.error}
                helperText={fieldState.error?.message}
              />
            )}
          />
        </FormItem>
        <FormItem label='等级' sx={{ mt: 2 }}>
          <Controller
            name='rank'
            control={levelControl}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                type='number'
                onChange={event =>
                  field.onChange(
                    event.target.value === '' ? 0 : Number(event.target.value),
                  )
                }
              />
            )}
          />
        </FormItem>
        <FormItem label='说明' sx={{ mt: 2 }}>
          <Controller
            name='description'
            control={levelControl}
            render={({ field }) => (
              <TextField {...field} fullWidth multiline minRows={3} />
            )}
          />
        </FormItem>
        <FormItem label='状态' sx={{ mt: 2 }}>
          <Controller
            name='status'
            control={levelControl}
            render={({ field }) => (
              <Select
                value={field.value}
                onChange={field.onChange}
                style={{ width: '100%' }}
                options={[
                  { value: 'active', label: '启用' },
                  { value: 'disabled', label: '停用' },
                ]}
              />
            )}
          />
        </FormItem>
        <FormItem label='默认等级' sx={{ mt: 2 }}>
          <Controller
            name='is_default'
            control={levelControl}
            render={({ field }) => (
              <Switch
                checked={!!field.value}
                onChange={(_, checked) => field.onChange(checked)}
              />
            )}
          />
        </FormItem>
      </Modal>
    </Stack>
  );
};

export default EnterpriseAccess;
