import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ChatSessionMemory } from './chatSession.ts';

function browserTab(scope = '/kb', storage = new Map()) {
  const memory = new ChatSessionMemory(
    scope,
    () => ({
      getItem: key => storage.get(key) || null,
      setItem: (key, value) => storage.set(key, value),
    }),
    () => {},
  );
  return { memory, storage };
}

test('refresh restores the original conversation ID and nonce and resumes once', async () => {
  const first = browserTab();
  first.memory.acceptEvent('conversation_id', 'original');
  first.memory.acceptEvent('nonce', 'private-nonce');
  first.memory.acceptEvent('conversation_id', 'original');
  const refreshed = browserTab('/kb', first.storage);
  refreshed.memory.restore('original');
  let calls = 0;
  const resume = async (id, nonce) => {
    calls++;
    assert.equal(id, 'original');
    assert.equal(nonce, 'private-nonce');
    return { conversation_id: id, nonce };
  };
  assert.deepEqual(await refreshed.memory.resolve(resume), {
    conversation_id: 'original',
    nonce: 'private-nonce',
  });
  await refreshed.memory.resolve(resume);
  assert.equal(calls, 1);
});

test('shared history switches to an independent conversation and retains its new nonce', async () => {
  const tab = browserTab();
  tab.memory.restore('shared-original');
  const fork = {
    conversation_id: 'viewer-copy',
    nonce: 'viewer-nonce',
    forked: true,
  };
  assert.deepEqual(
    await tab.memory.resolve(async (id, nonce) => {
      assert.equal(id, 'shared-original');
      assert.equal(nonce, '');
      return fork;
    }),
    fork,
  );
  const refreshed = browserTab('/kb', tab.storage);
  refreshed.memory.restore('viewer-copy');
  await refreshed.memory.resolve(async (id, nonce) => {
    assert.equal(nonce, 'viewer-nonce');
    return { conversation_id: id, nonce };
  });
});

test('new conversation clears active identity while old history remains resumable', async () => {
  const tab = browserTab();
  tab.memory.acceptEvent('conversation_id', 'old');
  tab.memory.acceptEvent('nonce', 'old-nonce');
  tab.memory.reset();
  assert.deepEqual(
    await tab.memory.resolve(async () =>
      assert.fail('new chat must not resume old chat'),
    ),
    { conversation_id: '', nonce: '' },
  );
  tab.memory.restore('old');
  await tab.memory.resolve(async (id, nonce) => {
    assert.equal(nonce, 'old-nonce');
    return { conversation_id: id, nonce };
  });
});

test('late recovery cannot overwrite a newly selected conversation', async () => {
  const tab = browserTab();
  tab.memory.restore('old');
  let complete;
  const pending = tab.memory.resolve(
    () =>
      new Promise(resolve => {
        complete = resolve;
      }),
  );
  tab.memory.reset();
  tab.memory.acceptEvent('conversation_id', 'new');
  tab.memory.acceptEvent('nonce', 'new-nonce');
  complete({ conversation_id: 'old', nonce: 'old-nonce' });
  assert.equal(await pending, null);
  assert.deepEqual(
    await tab.memory.resolve(async () =>
      assert.fail('new chat must not resume'),
    ),
    { conversation_id: 'new', nonce: 'new-nonce' },
  );
});

test('storage is scoped by knowledge base path and unavailable storage allows owner recovery', async () => {
  const tab = browserTab();
  tab.memory.acceptEvent('conversation_id', 'id');
  tab.memory.acceptEvent('nonce', 'nonce');
  const other = browserTab('/other-kb', tab.storage);
  other.memory.restore('id');
  await other.memory.resolve(async (id, nonce) => {
    assert.equal(nonce, '');
    return { conversation_id: id, nonce: 'recovered' };
  });
  const blocked = new ChatSessionMemory(
    '/kb',
    () => {
      throw new Error('storage disabled');
    },
    () => {},
  );
  blocked.restore('id');
  assert.deepEqual(
    await blocked.resolve(async id => ({
      conversation_id: id,
      nonce: 'owner-recovered',
    })),
    { conversation_id: 'id', nonce: 'owner-recovered' },
  );
});

test('reopening the browser recovers history credentials from persistent storage', async () => {
  const persistent = new Map();
  const storage = values => ({
    getItem: key => values.get(key) || null,
    setItem: (key, value) => values.set(key, value),
  });
  const first = new ChatSessionMemory(
    '/kb',
    () => storage(new Map()),
    () => {},
    () => storage(persistent),
  );
  first.acceptEvent('conversation_id', 'old');
  first.acceptEvent('nonce', 'private-nonce');
  first.reset();
  const reopened = new ChatSessionMemory(
    '/kb',
    () => storage(new Map()),
    () => {},
    () => storage(persistent),
  );
  assert.deepEqual(reopened.historyCredentials(), [
    { id: 'old', nonce: 'private-nonce' },
  ]);
  reopened.restore('old');
  await reopened.resolve(async (id, nonce) => {
    assert.equal(nonce, 'private-nonce');
    return { conversation_id: id, nonce };
  });
});

test('selecting another history invalidates a pending resume', async () => {
  const { memory } = browserTab();
  memory.restore('first');
  let complete;
  const pending = memory.resolve(
    () =>
      new Promise(resolve => {
        complete = resolve;
      }),
  );
  memory.restore('second');
  complete({ conversation_id: 'first', nonce: 'first-nonce' });
  assert.equal(await pending, null);
  assert.equal(
    (
      await memory.resolve(async id => ({
        conversation_id: id,
        nonce: 'second-nonce',
      }))
    ).conversation_id,
    'second',
  );
});

test('history is bounded and damaged browser storage does not prevent new chats', async () => {
  const { memory, storage } = browserTab();
  for (let i = 0; i < 105; i++) {
    memory.reset();
    memory.acceptEvent('conversation_id', String(i));
    memory.acceptEvent('nonce', `nonce-${i}`);
  }
  assert.equal(memory.historyCredentials().length, 100);
  assert.equal(memory.historyCredentials()[0].id, '104');
  storage.set('panda_wiki_chat_history:/kb', 'not JSON');
  assert.deepEqual(memory.historyCredentials(), []);
  memory.reset();
  assert.equal(
    (await memory.resolve(async () => assert.fail())).conversation_id,
    '',
  );
});
