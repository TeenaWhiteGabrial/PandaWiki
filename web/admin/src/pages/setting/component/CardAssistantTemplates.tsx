import SecureFilePreview from '@panda-wiki/file-preview';
import { useEffect, useRef, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd';
import httpClient, { ContentType } from '@/request/httpClient';
import { useAppSelector } from '@/store';
import { message } from '@/components/ui';
import {
  Department,
  PermissionLevel,
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
} from '@/request/Enterprise';

type Template = {
  id: string;
  name: string;
  version: number;
  upload_id: string;
  state: string;
  is_default: boolean;
  permissions: Record<string, unknown>;
};
type Uploaded = {
  id: string;
  name: string;
  status: string;
  error?: string;
  report?: {
    template_supported?: boolean;
    template_error?: string;
    template_notice?: string;
    layouts?: { index: number; name: string }[];
  };
};
export default function CardAssistantTemplates() {
  const kb = useAppSelector(s => s.config.kb_id);
  const [items, setItems] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [uploaded, setUploaded] = useState<Uploaded | null>(null);
  const [name, setName] = useState('');
  const [base, setBase] = useState<Template | null>(null);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [permissionTarget, setPermissionTarget] = useState<Template | null>(
    null,
  );
  const [previewTemplate, setPreviewTemplate] = useState<Template | null>(null);
  const [all, setAll] = useState(true);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const [depIds, setDepIds] = useState<string[]>([]);
  const [levelIds, setLevelIds] = useState<string[]>([]);
  const epoch = useRef(0);
  const request = <T,>(path: string, method = 'GET', body?: unknown) =>
    httpClient<T>({
      path: `/api/v1/assistant${path}`,
      query: { kb_id: kb },
      method,
      body,
      format: 'json',
      ...(body instanceof FormData
        ? { type: ContentType.FormData }
        : body !== undefined
          ? { type: ContentType.Json }
          : {}),
    });
  const load = async () => {
    setLoading(true);
    try {
      setItems(await request<Template[]>('/templates'));
      setError('');
    } catch (e) {
      setError((e as Error).message || '加载模板失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    if (!kb) return;
    void load();
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ])
      .then(([d, l]) => {
        setDepartments(d.departments);
        setLevels(l.levels);
      })
      .catch(() => {});
    return () => {
      epoch.current++;
    };
  }, [kb]);
  useEffect(() => {
    if (
      !uploaded ||
      !['queued', 'processing'].includes(uploaded.status) ||
      !open
    )
      return;
    const current = epoch.current;
    let timer: ReturnType<typeof setTimeout>;
    let canceled = false;
    const poll = async () => {
      try {
        const next = await request<Uploaded>(`/uploads/${uploaded.id}`);
        if (canceled || current !== epoch.current) return;
        setUploaded(next);
        if (next.status === 'ready' || next.status === 'failed') return;
      } catch (e) {
        if (!canceled) setError((e as Error).message);
      }
      if (!canceled) timer = setTimeout(poll, 2000);
    };
    timer = setTimeout(poll, 1500);
    return () => {
      canceled = true;
      clearTimeout(timer);
    };
  }, [uploaded?.id, uploaded?.status, open]);
  const begin = (t?: Template) => {
    epoch.current++;
    setBase(t || null);
    setName(t?.name || '');
    setUploaded(null);
    setError('');
    setAll(t?.permissions?.scope_type !== 'custom');
    setDepIds((t?.permissions?.allowed_department_ids as string[]) || []);
    setLevelIds(
      (t?.permissions?.allowed_permission_level_ids as string[]) || [],
    );
    setConfirmed(false);
    setOpen(true);
  };
  const flat = (
    ds: Department[],
    prefix = '',
  ): { label: string; value: string }[] =>
    ds.flatMap(d => [
      { label: prefix + d.name, value: d.id },
      ...flat(d.children || [], prefix + d.name + ' / '),
    ]);
  const save = async () => {
    if (!uploaded || !confirmed) return;
    setSaving(true);
    try {
      await request('/templates', 'POST', {
        id: base?.id,
        name,
        upload_id: uploaded.id,
        confirmed,
        permissions: all
          ? { scope_type: 'public', allow_all_users: true }
          : {
              scope_type: 'custom',
              allowed_department_ids: depIds,
              allowed_permission_level_ids: levelIds,
            },
      });
      message.success('已保存草稿，请发布后供用户使用');
      setOpen(false);
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };
  const state = async (t: Template, next: string, def = false) => {
    try {
      await request(`/templates/${t.id}/${t.version}`, 'PATCH', {
        state: next,
        is_default: def,
      });
      message.success('模板设置已更新');
      await load();
    } catch {}
  };
  return (
    <Card
      title='智能助手 · PPT 模板'
      extra={
        <Button type='primary' onClick={() => begin()}>
          上传模板
        </Button>
      }
      style={{ margin: 24 }}
    >
      <Space orientation='vertical' size={16} style={{ width: '100%' }}>
        <Alert
          type='info'
          showIcon
          title='企业模板与个人模板分开管理'
          description='这里发布的模板供授权用户选择。用户在对话中上传的个人模板不会自动进入此列表。新版本不会替换正在生成任务使用的版本。'
        />
        {error && !open && (
          <Alert
            type='error'
            title={error}
            action={<Button onClick={() => void load()}>重试</Button>}
          />
        )}
        <Table
          loading={loading}
          dataSource={items}
          rowKey={t => `${t.id}:${t.version}`}
          pagination={{ pageSize: 10 }}
          columns={[
            { title: '模板', dataIndex: 'name' },
            { title: '版本', render: (_, t) => <span>v{t.version}</span> },
            {
              title: '状态',
              render: (_, t) => (
                <Space>
                  <Tag color={t.state === 'published' ? 'green' : 'default'}>
                    {t.state === 'published'
                      ? '已发布'
                      : t.state === 'stopped'
                        ? '已停用'
                        : '草稿'}
                  </Tag>
                  {t.is_default && <Tag color='blue'>默认</Tag>}
                </Space>
              ),
            },
            {
              title: '可用范围',
              render: (_, t) =>
                t.permissions?.scope_type === 'custom'
                  ? '按部门 / 等级规则'
                  : '全员可用',
            },
            {
              title: '操作',
              render: (_, t) => (
                <Space wrap>
                  <Button
                    size='small'
                    onClick={() => {
                      setPreviewTemplate(t);
                      setUploaded({
                        id: t.upload_id,
                        name: t.name,
                        status: 'ready',
                      });
                      setPreview(true);
                    }}
                  >
                    预览
                  </Button>
                  <Button
                    size='small'
                    onClick={() => {
                      setPermissionTarget(t);
                      setAll(t.permissions?.scope_type !== 'custom');
                      setDepIds(
                        (t.permissions?.allowed_department_ids as string[]) ||
                          [],
                      );
                      setLevelIds(
                        (t.permissions
                          ?.allowed_permission_level_ids as string[]) || [],
                      );
                    }}
                  >
                    可用范围
                  </Button>
                  <Button size='small' onClick={() => begin(t)}>
                    上传新版本
                  </Button>
                  {t.state !== 'published' ? (
                    <Button
                      size='small'
                      onClick={() => void state(t, 'published')}
                    >
                      发布
                    </Button>
                  ) : (
                    <>
                      <Button
                        size='small'
                        disabled={t.is_default}
                        onClick={() => void state(t, 'published', true)}
                      >
                        设为默认
                      </Button>
                      <Button
                        size='small'
                        onClick={() => void state(t, 'stopped')}
                      >
                        停用
                      </Button>
                    </>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Space>
      <Modal
        title={base ? '上传模板新版本' : '上传 PPT 模板'}
        open={open}
        onCancel={() => {
          epoch.current++;
          setOpen(false);
        }}
        onOk={() => void save()}
        confirmLoading={saving}
        okText='保存草稿'
        okButtonProps={{
          disabled:
            !name.trim() ||
            !uploaded?.report?.template_supported ||
            !confirmed ||
            (!all && !depIds.length && !levelIds.length),
        }}
        width={760}
      >
        <Space orientation='vertical' style={{ width: '100%' }} size={16}>
          {error && <Alert type='error' title={error} />}
          <Input
            value={name}
            maxLength={80}
            placeholder='模板名称'
            onChange={e => setName(e.target.value)}
          />
          <Upload
            accept='.pptx'
            maxCount={1}
            showUploadList={false}
            beforeUpload={file => {
              if (file.size > 20 * 1024 * 1024) {
                message.error('模板最大20 MB');
                return false;
              }
              const current = ++epoch.current;
              setUploaded(null);
              setConfirmed(false);
              setSaving(true);
              const data = new FormData();
              data.append('file', file);
              void request<Uploaded>('/uploads', 'POST', data)
                .then(v => {
                  if (current === epoch.current) {
                    setUploaded(v);
                    if (!name) setName(file.name.replace(/\.pptx$/i, ''));
                  }
                })
                .catch(e => setError(e.message))
                .finally(() => setSaving(false));
              return false;
            }}
          >
            <Button>选择 PPTX 文件</Button>
          </Upload>
          <Typography.Text type='secondary'>
            首版支持16:9且有标题、正文占位符的模板。只使用母版与布局，不自动带入原正文、备注及隐藏页。
          </Typography.Text>
          {uploaded && (
            <>
              <Typography.Text>
                {uploaded.name} ·{' '}
                {uploaded.status === 'ready'
                  ? '检查完成'
                  : uploaded.status === 'failed'
                    ? '处理失败'
                    : '检测 / 解析中'}
              </Typography.Text>
              {['queued', 'processing'].includes(uploaded.status) && <Spin />}
              {(uploaded.error || uploaded.report?.template_error) && (
                <Alert
                  type='warning'
                  title={uploaded.error || uploaded.report?.template_error}
                />
              )}
            </>
          )}
          {uploaded?.report?.template_supported && (
            <>
              <Button
                onClick={() => {
                  setPreviewTemplate(null);
                  setPreview(true);
                }}
              >
                检查模板预览
              </Button>
              <Checkbox
                checked={confirmed}
                onChange={e => setConfirmed(e.target.checked)}
              >
                已确认版式和装饰素材可用，且不含不应传播的信息
              </Checkbox>
              <Typography.Text type='secondary'>
                支持版式：{uploaded.report.layouts?.map(l => l.name).join('、')}
              </Typography.Text>
            </>
          )}
          <Checkbox checked={all} onChange={e => setAll(e.target.checked)}>
            全员可用
          </Checkbox>
          {!all && (
            <>
              <Select
                mode='multiple'
                style={{ width: '100%' }}
                placeholder='可用部门（含下级）'
                value={depIds}
                onChange={setDepIds}
                options={flat(departments)}
              />
              <Select
                mode='multiple'
                style={{ width: '100%' }}
                placeholder='可用等级（与部门条件取交集）'
                value={levelIds}
                onChange={setLevelIds}
                options={levels.map(l => ({ value: l.id, label: l.name }))}
              />
            </>
          )}
        </Space>
      </Modal>
      <Modal
        title='模板可用范围'
        open={!!permissionTarget}
        onCancel={() => setPermissionTarget(null)}
        onOk={() => {
          if (!permissionTarget) return;
          void request(
            `/templates/${permissionTarget.id}/permissions`,
            'PUT',
            all
              ? { scope_type: 'public', allow_all_users: true }
              : {
                  scope_type: 'custom',
                  allowed_department_ids: depIds,
                  allowed_permission_level_ids: levelIds,
                },
          ).then(() => {
            setPermissionTarget(null);
            message.success('已更新全部版本的可用范围，历史产物也会重新校验');
            void load();
          });
        }}
        okButtonProps={{ disabled: !all && !depIds.length && !levelIds.length }}
      >
        <Space orientation='vertical' style={{ width: '100%' }}>
          <Alert
            type='warning'
            title='权限调整对该模板的全部版本生效，包含历史产物。停用只阻止新使用；这里调整的是访问权限。'
          />
          <Checkbox checked={all} onChange={e => setAll(e.target.checked)}>
            全员可用
          </Checkbox>
          {!all && (
            <>
              <Select
                mode='multiple'
                style={{ width: '100%' }}
                options={flat(departments)}
                value={depIds}
                onChange={setDepIds}
                placeholder='部门（含下级）'
              />
              <Select
                mode='multiple'
                style={{ width: '100%' }}
                options={levels.map(l => ({ value: l.id, label: l.name }))}
                value={levelIds}
                onChange={setLevelIds}
                placeholder='等级（与部门取交集）'
              />
            </>
          )}
        </Space>
      </Modal>
      <Modal
        title='模板样式预览'
        open={preview}
        onCancel={() => setPreview(false)}
        footer={<Button onClick={() => setPreview(false)}>关闭</Button>}
        width={950}
      >
        {uploaded && (
          <div style={{ height: '65vh' }}>
            <SecureFilePreview
              title='企业模板预览'
              kind='pdf'
              assetBase={window.__BASENAME__ || ''}
              url={
                previewTemplate
                  ? `${window.__BASENAME__ || ''}/api/v1/assistant/templates/${previewTemplate.id}/${previewTemplate.version}/content?kb_id=${encodeURIComponent(kb || '')}`
                  : `${window.__BASENAME__ || ''}/api/v1/assistant/uploads/${uploaded.id}/content?preview=true&kb_id=${encodeURIComponent(kb || '')}`
              }
            />
          </div>
        )}
      </Modal>
    </Card>
  );
}
