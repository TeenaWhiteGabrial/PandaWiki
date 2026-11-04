import assert from 'node:assert/strict';
import test from 'node:test';
import { assistantTaskView, assistantNotice } from './assistantFeedback.mjs';

test('technical failures remain paused, never shown as completed or raw errors', () => {
  for (const error of [
    'max_tokens reached 16000',
    'Unexpected EOF in JSON',
    'HTTP 502 internal-service',
    '模型返回格式无效，请重试',
  ]) {
    const view = assistantTaskView({
      state: 'failed',
      stage: '需求计划格式校验失败',
      error,
    });
    assert.equal(view.label, '已暂停');
    assert.ok(view.detail.includes('继续'));
    assert.ok(!JSON.stringify(view).includes(error));
  }
  assert.equal(
    assistantTaskView({ state: 'running', stage: '修正需求计划格式' }).label,
    '正在调整内容方案',
  );
  assert.equal(
    assistantTaskView({ state: 'running', stage: 'Unexpected JSON error' })
      .label,
    '正在处理',
  );
});

test('permissions, cancellation, and missing materials retain truthful business states', () => {
  assert.equal(
    assistantTaskView({ state: 'completed', error: '来源权限已变化' }).label,
    '资料范围已更新',
  );
  assert.equal(
    assistantTaskView({ state: 'canceled', error: 'old technical error' })
      .label,
    '已结束',
  );
  for (const state of [
    'awaiting_input',
    'awaiting_sources',
    'awaiting_outline',
  ]) {
    assert.ok(assistantTaskView({ state }).label.startsWith('等待'));
  }
});

test('request and stream issues disclose no raw diagnostics and explain next actions', () => {
  for (const operation of [
    'submit',
    'continue',
    'upload',
    'settings',
    'preview',
    'download',
    'chat',
    'history',
    'sync',
  ]) {
    const text = assistantNotice(operation, {
      message: 'private model JSON/token error',
      trace_id: 'secret-trace',
    });
    assert.ok(text.length > 0);
    assert.doesNotMatch(text, /private|JSON|token|secret-trace|失败|错误/);
  }
  assert.ok(assistantNotice('download', { code: 403 }).includes('可访问'));
  assert.ok(assistantNotice('continue', { code: 401 }).includes('登录'));
});
