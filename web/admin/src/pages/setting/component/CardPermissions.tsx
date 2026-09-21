import { useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Modal,
  Space,
  Spin,
  Switch,
  Typography,
} from 'antd';
import httpClient, { ContentType } from '@/request/httpClient';
import { useAppDispatch, useAppSelector } from '@/store';
import { setUser } from '@/store/slices/config';
import { message } from '@/components/ui';

export default function CardPermissions() {
  const dispatch = useAppDispatch();
  const user = useAppSelector(state => state.config.user);
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);
  const load = async () => {
    setLoading(true);
    setError(false);
    try {
      const result = await httpClient<{ enabled: boolean }>({
        path: '/api/v1/system/permissions',
        method: 'GET',
        format: 'json',
      });
      setEnabled(result.enabled);
      dispatch(
        setUser({ ...user, permission_levels_disabled: !result.enabled }),
      );
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  const change = (next: boolean) => {
    Modal.confirm({
      title: next ? '恢复等级权限？' : '暂时关闭等级权限？',
      content: next
        ? '将重新应用已保存的最低权限等级规则，并恢复等级设置入口。部门权限不受影响。'
        : '关闭后，阅读和问答不再按权限等级过滤。部门及个人授权仍然生效，原等级数据保留。',
      okText: next ? '确认启用' : '确认关闭',
      cancelText: '取消',
      onOk: async () => {
        setSaving(true);
        try {
          const result = await httpClient<{ enabled: boolean }>({
            path: '/api/v1/system/permissions',
            method: 'PUT',
            body: { enabled: next },
            type: ContentType.Json,
            format: 'json',
          });
          setEnabled(result.enabled);
          dispatch(
            setUser({ ...user, permission_levels_disabled: !result.enabled }),
          );
          message.success('权限配置已保存并生效');
        } finally {
          setSaving(false);
        }
      },
    });
  };
  return (
    <Card title='权限功能' style={{ maxWidth: 900, margin: '24px auto' }}>
      <Spin spinning={loading}>
        <Space direction='vertical' size={24} style={{ width: '100%' }}>
          {error && (
            <Alert
              type='error'
              title='读取配置失败'
              action={<Button onClick={() => void load()}>重试</Button>}
            />
          )}
          <Space size={16}>
            <Typography.Text strong>启用等级权限</Typography.Text>
            <Switch
              checked={enabled}
              disabled={loading || error || saving}
              loading={saving}
              checkedChildren='开启'
              unCheckedChildren='关闭'
              onChange={change}
            />
          </Space>
          <Typography.Paragraph type='secondary'>
            仅控制等级权限。部门管理、部门授权和个人授权始终保留；关闭时保留原等级规则，之后可恢复。
          </Typography.Paragraph>
          <Alert
            type='info'
            showIcon
            title='管理员登录与 API 认证始终保留'
            description='保存后自动生效，无需编辑配置文件或重启。其他已打开的管理页面刷新后同步；后台服务会在约 1 秒内同步。'
          />
        </Space>
      </Spin>
    </Card>
  );
}
