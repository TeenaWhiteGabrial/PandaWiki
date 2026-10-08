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
import { Box, Button, Stack, Typography } from '@mui/material';
import { Select } from 'antd';
import { isAdministrator } from '@/utils/permission';
import { useEffect, useMemo, useState } from 'react';

const subjectLabels = {
  permission_level: '用户等级',
  user: '指定用户',
  department: '部门',
};
const scopeOptions = [
  { value: 'current_and_descendants', label: '当前及下级目录' },
  { value: 'current', label: '当前目录' },
];

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
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const [users, setUsers] = useState<
    { id: string; name: string; administrator: boolean }[]
  >([]);
  const [loading, setLoading] = useState(false);
  const [ready, setReady] = useState(false);
  const [canConfigure, setCanConfigure] = useState(false);
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );
  const nodeKey = nodeIDs.join(',');
  useEffect(() => {
    if (!open || !nodeIDs[0]) return;
    let active = true;
    setReady(false);
    setLoading(true);
    const getManagement =
      targetType === 'nav' ? getNavManagement : getNodeManagement;
    getManagement({ kb_id: kbID, id: nodeIDs[0] })
      .then(async response => {
        if (!active) return;
        setCanConfigure(response.can_configure);
        setGrants(response.grants || []);
        if (!response.can_configure) return;
        const options = await getNodePermissionOptions(kbID);
        if (!active) return;
        setDepartments(
          options.departments.filter(item => item.status === 'active'),
        );
        setLevels(options.levels.filter(item => item.status === 'active'));
        setUsers(
          options.users.map(item => ({
            id: item.id,
            name: item.name || item.account,
            administrator: isAdministrator(item.role),
          })),
        );
        setReady(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [open, kbID, nodeKey, targetType]);
  const update = (
    index: number,
    patch: Partial<DomainDocumentManagementGrant>,
  ) =>
    setGrants(prev =>
      prev.map((grant, i) => (i === index ? { ...grant, ...patch } : grant)),
    );
  const submit = () => {
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
      okButtonProps={{ loading, disabled: !ready || !canConfigure }}
    >
      <Stack gap={2}>
        <Typography fontSize={13} color='text.secondary'>
          只有管理员和超级管理员可以管理文档。以下授权仅对管理员账号生效，普通用户不会因部门、等级或指定用户授权获得管理权限。
        </Typography>
        <Typography fontSize={13} color='text.secondary'>
          每条规则可选择多个管理员、等级或部门，目录权限范围默认“当前及下级目录”。“当前目录”包含本目录及直属文档，不包含下级目录。已有管理员的全局管理权限不会因这些规则被缩小。
        </Typography>
        {nodeIDs.length > 1 && (
          <Typography color='warning.main' fontSize={13}>
            批量保存会将以下规则替换到所选 {nodeIDs.length}{' '}
            个节点；当前展示第一个节点的配置。
          </Typography>
        )}
        {!canConfigure && !loading && (
          <Typography fontSize={13}>
            只有知识库管理员可分配管理权限。
            {targetType === 'node' && '继承的授权由上级目录配置。'}
          </Typography>
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
                <Select
                  disabled={!canConfigure}
                  style={{ width: 160 }}
                  value={grant.subject_type}
                  options={Object.entries(subjectLabels).map(
                    ([value, label]) => ({ value, label }),
                  )}
                  onChange={value =>
                    update(index, { subject_type: value, subject_ids: [] })
                  }
                />
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
                  departments={departmentTree}
                  flatDepartments={flatDepartments}
                  value={grant.subject_ids}
                  onChange={value => update(index, { subject_ids: value })}
                  label='授权部门'
                  size='small'
                  disabled={!canConfigure}
                />
              ) : (
                <Select
                  disabled={!canConfigure}
                  mode='multiple'
                  showSearch
                  optionFilterProp='label'
                  style={{ width: '100%' }}
                  value={grant.subject_ids}
                  placeholder={`选择${subjectLabels[grant.subject_type]}`}
                  options={
                    grant.subject_type === 'user'
                      ? users
                          .filter(
                            user =>
                              user.administrator ||
                              grant.subject_ids.includes(user.id),
                          )
                          .map(user => ({
                            value: user.id,
                            label: user.administrator
                              ? user.name
                              : `${user.name}（普通用户，授权不生效）`,
                            disabled: !user.administrator,
                          }))
                      : levels.map(level => ({
                          value: level.id,
                          label: level.name,
                        }))
                  }
                  onChange={value => update(index, { subject_ids: value })}
                />
              )}
              <Stack direction='row' alignItems='center' gap={1}>
                <Typography fontSize={13}>目录权限范围</Typography>
                <Select
                  disabled={!canConfigure}
                  style={{ minWidth: 190 }}
                  value={grant.directory_scope || 'current_and_descendants'}
                  options={scopeOptions}
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
          没有规则表示当前节点不额外授予管理权限；
          {targetType === 'node' && '上级目录授权及'}
          已有知识库管理员权限仍有效。保存查阅权限不会修改管理权限。
        </Typography>
      </Stack>
    </Modal>
  );
}
