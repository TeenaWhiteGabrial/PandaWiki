import { Modal, message } from '@/components/ui';
import {
  getApiV1NavPermission,
  patchApiV1NavPermission,
} from '@/request/NavCompat';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import {
  getApiV1NodePermission,
  patchApiV1NodePermissionEdit,
} from '@/request/NodePermission';
import {
  previewReadingAudience,
  ReadingAudienceResult,
} from '@/request/ReadingAudience';
import type { DomainNodePermissions, DomainReadingRule } from '@/request/types';
import { useAppSelector } from '@/store';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Stack,
  Typography,
} from '@mui/material';
import { Select } from 'antd';
import { Fragment, useEffect, useMemo, useState } from 'react';
import {
  describeReadingPolicy,
  hasLocalReadingPolicy,
  incompleteReadingRule,
  independentReadingPolicy,
  newReadingRule,
  readingRuleKind,
} from '../utils/readingRules';
import NodeManagementModal from './NodeManagementModal';
import ReadingRuleEditor from './ReadingRuleEditor';
import {
  PermissionDialogTabs,
  PermissionObjectName,
  PermissionHelp,
  permissionTableSx,
} from './PermissionDialogLayout';

type Options = Awaited<ReturnType<typeof getNodePermissionOptions>>;
const inherited = (): DomainNodePermissions => ({
  reading_version: 2,
  inherit: true,
});
export default function ReadingRulesModal({
  open,
  kbID,
  ids,
  names,
  kind,
  canConfigure,
  onClose,
  onSuccess,
}: {
  open: boolean;
  kbID: string;
  ids: string[];
  names: string[];
  kind: '目录' | '文档' | '栏目';
  canConfigure: boolean;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const nav = kind === '栏目';
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const [options, setOptions] = useState<Options>();
  const [draft, setDraft] = useState<DomainNodePermissions>(inherited);
  const [parent, setParent] = useState<DomainNodePermissions>({});
  const [legacy, setLegacy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [saving, setSaving] = useState(false);
  const [managementOpen, setManagementOpen] = useState(false);
  const [audience, setAudience] = useState<ReadingAudienceResult>();
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState(false);
  const [showUsers, setShowUsers] = useState(false);
  const [showExclusions, setShowExclusions] = useState(false);
  const [editor, setEditor] = useState<{
    index: number | null;
    value: DomainReadingRule;
  } | null>(null);
  const idsKey = JSON.stringify(ids);
  const targetType = nav ? 'nav' : 'node';
  useEffect(() => {
    setManagementOpen(false);
    if (!open || !kbID || !ids.length) return;
    let active = true;
    setLoading(true);
    setError(false);
    setDirty(false);
    setAudience(undefined);
    setShowUsers(false);
    setShowExclusions(false);
    setEditor(null);
    const detail = nav
      ? getApiV1NavPermission({ kb_id: kbID, id: ids[0] })
      : getApiV1NodePermission({ kb_id: kbID, id: ids[0] });
    Promise.all([
      getNodePermissionOptions(kbID),
      detail,
      nav
        ? Promise.resolve(undefined)
        : previewReadingAudience({
            kb_id: kbID,
            ids,
            target_type: 'node',
            permissions: inherited(),
          }),
    ])
      .then(([choices, response, inheritedAudience]) => {
        if (!active) return;
        setOptions(choices);
        const permissions = response.permissions || {};
        const effective =
          'effective_permissions' in response
            ? response.effective_permissions || permissions
            : permissions;
        const isInherited =
          !nav &&
          (permissions.inherit === true ||
            (permissions.inherit === undefined &&
              !hasLocalReadingPolicy(permissions)));
        setDraft(
          isInherited ? inherited() : independentReadingPolicy(effective),
        );
        setParent(inheritedAudience?.permissions || {});
        setLegacy(permissions.reading_version !== 2);
      })
      .catch(() => {
        if (active) setError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
    // Targets are compared by IDs so parent rerenders do not erase unsaved edits.
  }, [open, kbID, idsKey, nav]);
  const invalid =
    !draft.inherit && !!draft.reading_rules?.some(incompleteReadingRule);
  useEffect(() => {
    if (!open || loading || error || !options || invalid) {
      setAudience(undefined);
      setPreviewBusy(false);
      return;
    }
    const controller = new AbortController();
    setPreviewBusy(true);
    setPreviewError(false);
    const timer = setTimeout(() => {
      previewReadingAudience(
        {
          kb_id: kbID,
          ids,
          target_type: targetType,
          ...(dirty ? { permissions: draft } : {}),
          include_users: true,
          page_size: 100,
        },
        controller.signal,
      )
        .then(result => {
          if (!controller.signal.aborted) setAudience(result);
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            setAudience(undefined);
            setPreviewError(true);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setPreviewBusy(false);
        });
    }, 350);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [
    open,
    loading,
    error,
    options,
    invalid,
    kbID,
    idsKey,
    targetType,
    dirty,
    draft,
  ]);
  const catalogs = useMemo(
    () => ({
      departments: Object.fromEntries(
        (options?.departments || []).map(item => [item.id, item.name]),
      ),
      levels: Object.fromEntries(
        (options?.levels || []).map(item => [item.id, item.name]),
      ),
      users: Object.fromEntries(
        (options?.users || []).map(item => [
          item.id,
          item.name || item.account,
        ]),
      ),
    }),
    [options],
  );
  const departments = (options?.departments || []).filter(
    d => d.status === 'active',
  );
  const userOptions = (options?.users || []).map(user => ({
    value: user.id,
    label:
      user.name && user.name !== user.account
        ? `${user.name}（${user.account}）`
        : user.account,
  }));
  const update = (value: DomainNodePermissions) => {
    setDraft(value);
    setDirty(true);
  };
  const updateRule = (index: number, rule: DomainReadingRule) =>
    update({
      ...draft,
      reading_rules: (draft.reading_rules || []).map((item, i) =>
        i === index ? rule : item,
      ),
    });
  const save = async () => {
    if (!dirty) {
      onClose();
      return;
    }
    if (!canConfigure || loading || saving || error || invalid || editor)
      return;
    setSaving(true);
    try {
      if (nav)
        await patchApiV1NavPermission({
          kb_id: kbID,
          id: ids[0],
          permissions: draft,
        });
      else
        await patchApiV1NodePermissionEdit({
          kb_id: kbID,
          ids,
          permissions: draft,
        });
      message.success('权限已保存');
      onSuccess();
      onClose();
    } catch {
      /* Request layer displays the server error. Keep edits for retry. */
    } finally {
      setSaving(false);
    }
  };
  const displayPolicy = draft.inherit
    ? independentReadingPolicy(parent)
    : draft;
  const rows = displayPolicy.reading_rules || [];
  const excludedNames = (displayPolicy.denied_user_ids || []).map(
    id => catalogs.users[id] || id,
  );
  const ruleDescription = (rule: DomainReadingRule) => {
    const type = readingRuleKind(rule),
      p = rule.permissions;
    if (type === 'condition')
      return (
        [
          p.allowed_department_ids?.length
            ? `部门：${p.allowed_department_ids.map(id => catalogs.departments[id] || id).join('、')}（含子部门）`
            : '',
          !levelsDisabled && p.allowed_permission_level_ids?.length
            ? `等级：${p.allowed_permission_level_ids.map(id => catalogs.levels[id] || id).join('、')}`
            : '',
        ]
          .filter(Boolean)
          .join(' 且 ') ||
        (levelsDisabled ? '全员（等级限制未启用）' : '未设置授权对象')
      );
    if (type === 'users')
      return `用户：${(p.allowed_user_ids || []).map(id => catalogs.users[id] || id).join('、')}`;
    if (type === 'all') return '全员（需登录）';
    return describeReadingPolicy(p, catalogs);
  };
  const renderEditor = () =>
    editor && (
      <ReadingRuleEditor
        value={editor.value}
        adding={editor.index === null}
        departments={departments}
        levels={options?.levels || []}
        users={userOptions}
        levelsDisabled={levelsDisabled}
        documentOnly={kind === '文档'}
        onChange={value => setEditor({ ...editor, value })}
        onCancel={() => setEditor(null)}
        onConfirm={() => {
          if (incompleteReadingRule(editor.value)) return;
          if (editor.index === null)
            update({
              ...draft,
              reading_rules: [...(draft.reading_rules || []), editor.value],
            });
          else updateRule(editor.index, editor.value);
          setEditor(null);
        }}
      />
    );
  return (
    <>
      <Modal
        open={open && !managementOpen}
        width={800}
        title={`${kind}权限`}
        onCancel={onClose}
        onOk={save}
        okText={dirty ? '保存' : '关闭'}
        okButtonProps={{
          loading: saving,
          disabled:
            saving ||
            !!editor ||
            (dirty && (loading || error || invalid || !canConfigure)),
        }}
      >
        <Stack gap={2} sx={{ maxHeight: '70vh', overflowY: 'auto' }}>
          <PermissionObjectName names={names} />
          <PermissionDialogTabs
            value='reading'
            onReading={() => {}}
            onManagement={() => setManagementOpen(true)}
            managementDisabled={!canConfigure || dirty || !!editor}
          />
          {loading ? (
            <Stack alignItems='center' py={5}>
              <CircularProgress size={24} />
            </Stack>
          ) : error ? (
            <Alert severity='error'>权限加载失败，请关闭后重试。</Alert>
          ) : (
            <>
              {ids.length > 1 && (
                <Typography fontSize={12} color='warning.main'>
                  保存将替换所选 {ids.length} 项的查阅规则，当前显示第一项。
                </Typography>
              )}
              <Stack
                direction='row'
                alignItems='center'
                gap={1}
                justifyContent='space-between'
              >
                <Stack direction='row' alignItems='center' gap={0.75}>
                  {!nav && (
                    <>
                      <Checkbox
                        aria-label='使用上级设置'
                        slotProps={{ input: { 'aria-label': '使用上级设置' } }}
                        size='small'
                        checked={!!draft.inherit}
                        disabled={!canConfigure || !!editor}
                        onChange={(_, checked) => {
                          update(
                            checked
                              ? inherited()
                              : independentReadingPolicy(parent),
                          );
                          setShowExclusions(false);
                        }}
                        sx={{ p: 0.5, ml: -0.5 }}
                      />
                      <Typography fontSize={13}>使用上级设置</Typography>
                    </>
                  )}
                  {!canConfigure && (
                    <Typography color='text.secondary' fontSize={12}>
                      只读
                    </Typography>
                  )}
                  {nav && <Typography fontSize={13}>查阅规则</Typography>}
                </Stack>
                <PermissionHelp />
              </Stack>
              {!draft.inherit && legacy && dirty && (
                <Typography fontSize={12} color='warning.main'>
                  保存后使用独立规则，不再随上级权限变化。
                </Typography>
              )}
              <TableContainer
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                }}
              >
                <Table
                  size='small'
                  sx={permissionTableSx}
                  aria-label='查阅授权规则'
                >
                  <TableHead>
                    <TableRow>
                      <TableCell>授权对象（满足任意一条）</TableCell>
                      {kind !== '文档' && (
                        <TableCell sx={{ width: 150 }}>生效范围</TableCell>
                      )}
                      <TableCell align='right' sx={{ width: 110 }}>
                        操作
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {rows.map((rule, index) => {
                      const legacyRule = readingRuleKind(rule) === 'legacy';
                      return (
                        <Fragment key={index}>
                          <TableRow>
                            <TableCell>
                              <Typography
                                fontSize={13}
                                sx={{ lineHeight: 1.7 }}
                              >
                                {ruleDescription(rule)}
                              </Typography>
                              {legacyRule && (
                                <Typography
                                  fontSize={11}
                                  color='text.secondary'
                                >
                                  组合授权
                                </Typography>
                              )}
                            </TableCell>
                            {kind !== '文档' && (
                              <TableCell>
                                <Typography fontSize={13}>
                                  {rule.include_descendants
                                    ? '当前及下级目录'
                                    : '仅当前目录'}
                                </Typography>
                              </TableCell>
                            )}
                            <TableCell
                              align='right'
                              sx={{ whiteSpace: 'nowrap' }}
                            >
                              {draft.inherit ? (
                                <Typography
                                  fontSize={12}
                                  color='text.secondary'
                                >
                                  继承
                                </Typography>
                              ) : (
                                <>
                                  <Tooltip
                                    title={
                                      legacyRule
                                        ? '组合条件可保留或移除，添加规则可重新配置'
                                        : ''
                                    }
                                  >
                                    <span>
                                      <Button
                                        size='small'
                                        color='inherit'
                                        disabled={
                                          !canConfigure ||
                                          legacyRule ||
                                          !!editor
                                        }
                                        sx={{ minWidth: 0, px: 0.75 }}
                                        onClick={() =>
                                          setEditor({
                                            index,
                                            value: JSON.parse(
                                              JSON.stringify(rule),
                                            ),
                                          })
                                        }
                                      >
                                        编辑
                                      </Button>
                                    </span>
                                  </Tooltip>
                                  <Button
                                    size='small'
                                    color='inherit'
                                    disabled={!canConfigure || !!editor}
                                    sx={{
                                      minWidth: 0,
                                      px: 0.75,
                                      color: 'text.secondary',
                                      '&:hover': { color: 'error.main' },
                                    }}
                                    onClick={() =>
                                      update({
                                        ...draft,
                                        reading_rules:
                                          draft.reading_rules?.filter(
                                            (_, i) => i !== index,
                                          ),
                                      })
                                    }
                                  >
                                    移除
                                  </Button>
                                </>
                              )}
                            </TableCell>
                          </TableRow>
                          {editor?.index === index && (
                            <TableRow>
                              <TableCell
                                colSpan={kind === '文档' ? 2 : 3}
                                sx={{ p: '0 !important' }}
                              >
                                {renderEditor()}
                              </TableCell>
                            </TableRow>
                          )}
                        </Fragment>
                      );
                    })}
                    {!rows.length && (
                      <TableRow>
                        <TableCell colSpan={kind === '文档' ? 2 : 3}>
                          <Typography
                            color='text.secondary'
                            fontSize={13}
                            py={2}
                            textAlign='center'
                          >
                            未授权普通用户查阅
                          </Typography>
                        </TableCell>
                      </TableRow>
                    )}
                    {editor?.index === null && (
                      <TableRow>
                        <TableCell
                          colSpan={kind === '文档' ? 2 : 3}
                          sx={{ p: '0 !important' }}
                        >
                          {renderEditor()}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
              {!draft.inherit && (
                <Button
                  size='small'
                  variant='outlined'
                  sx={{ alignSelf: 'flex-start' }}
                  disabled={!canConfigure || !!editor || rows.length >= 100}
                  onClick={() =>
                    setEditor({ index: null, value: newReadingRule() })
                  }
                >
                  添加规则
                </Button>
              )}
              <Stack gap={1}>
                <Stack direction='row' gap={1.5} alignItems='baseline'>
                  <Typography
                    fontSize={13}
                    color='text.secondary'
                    flexShrink={0}
                  >
                    排除人员
                  </Typography>
                  <Typography
                    fontSize={13}
                    sx={{ flex: 1, overflowWrap: 'anywhere' }}
                  >
                    {excludedNames.join('、') || '无'}
                  </Typography>
                  {!draft.inherit && (
                    <Button
                      size='small'
                      color='inherit'
                      disabled={!canConfigure || !!editor}
                      sx={{ minWidth: 0 }}
                      onClick={() => setShowExclusions(!showExclusions)}
                    >
                      {showExclusions ? '收起' : '设置'}
                    </Button>
                  )}
                </Stack>
                {showExclusions && !draft.inherit && (
                  <Select
                    aria-label='排除人员'
                    mode='multiple'
                    showSearch
                    optionFilterProp='label'
                    placeholder='搜索姓名或账号'
                    value={draft.denied_user_ids || []}
                    options={userOptions}
                    disabled={!canConfigure}
                    onChange={ids => update({ ...draft, denied_user_ids: ids })}
                  />
                )}
                {!!displayPolicy.denied_department_ids?.length && (
                  <Stack direction='row' alignItems='baseline' gap={1.5}>
                    <Typography
                      fontSize={13}
                      color='text.secondary'
                      flexShrink={0}
                    >
                      排除部门
                    </Typography>
                    <Typography fontSize={13} sx={{ flex: 1 }}>
                      {displayPolicy.denied_department_ids
                        .map(id => catalogs.departments[id] || id)
                        .join('、')}
                    </Typography>
                    {!draft.inherit && (
                      <Button
                        size='small'
                        color='inherit'
                        disabled={!canConfigure || !!editor}
                        onClick={() =>
                          update({ ...draft, denied_department_ids: [] })
                        }
                      >
                        清除
                      </Button>
                    )}
                  </Stack>
                )}
              </Stack>
              <Box
                aria-live='polite'
                sx={{
                  borderTop: '1px solid',
                  borderColor: 'divider',
                  pt: 1.25,
                }}
              >
                <Stack direction='row' alignItems='center' gap={1}>
                  <Typography
                    fontSize={12}
                    color={previewError ? 'error.main' : 'text.secondary'}
                  >
                    {previewBusy
                      ? '正在计算可查阅范围…'
                      : previewError
                        ? '可查阅范围计算失败'
                        : audience
                          ? `可查阅 ${audience.total} 人`
                          : invalid
                            ? '请补全授权对象'
                            : '暂无查阅名单'}
                  </Typography>
                  {!!audience && !previewBusy && (
                    <Button
                      size='small'
                      color='inherit'
                      sx={{ fontSize: 12, minWidth: 0 }}
                      onClick={() => setShowUsers(!showUsers)}
                    >
                      {showUsers ? '收起名单' : '查看名单'}
                    </Button>
                  )}
                  {editor && (
                    <Typography
                      fontSize={12}
                      color='text.secondary'
                      sx={{ ml: 'auto' }}
                    >
                      请先确认或取消规则编辑
                    </Typography>
                  )}
                </Stack>
                {showUsers && audience && !previewBusy && (
                  <Box
                    sx={{
                      maxHeight: 150,
                      overflowY: 'auto',
                      mt: 1,
                      display: 'grid',
                      gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
                      gap: 0.75,
                    }}
                  >
                    {audience.users?.map(user => (
                      <Typography key={user.id} fontSize={12}>
                        {user.name && user.name !== user.account
                          ? `${user.name}（${user.account}）`
                          : user.account}
                      </Typography>
                    ))}
                    {audience.total > (audience.users?.length || 0) && (
                      <Typography fontSize={12} color='text.secondary'>
                        展示前 100 人，共 {audience.total} 人
                      </Typography>
                    )}
                  </Box>
                )}
              </Box>
            </>
          )}
        </Stack>
      </Modal>
      <NodeManagementModal
        open={open && managementOpen}
        kbID={kbID}
        nodeIDs={ids}
        targetType={targetType}
        targetNames={names}
        onBack={() => setManagementOpen(false)}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </>
  );
}
