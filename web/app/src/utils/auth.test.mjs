import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  AUTH_CLEARED_EVENT,
  clearAuthStorage,
  clearLegacyCredentials,
} from './auth.ts';

function browserState(run) {
  const oldWindow = globalThis.window,
    oldDocument = globalThis.document;
  const values = new Map(),
    deleted = [],
    events = [];
  globalThis.window = {
    localStorage: {
      getItem() {
        throw new Error('browser code must not read a login credential');
      },
      removeItem: key => values.delete(key),
      setItem: (key, value) => values.set(key, value),
    },
    dispatchEvent: event => events.push(event.type),
  };
  globalThis.document = {
    get cookie() {
      throw new Error('browser code must not read a login cookie');
    },
    set cookie(value) {
      deleted.push(value);
    },
  };
  try {
    run({ values, deleted, events });
  } finally {
    if (oldWindow === undefined) delete globalThis.window;
    else globalThis.window = oldWindow;
    if (oldDocument === undefined) delete globalThis.document;
    else globalThis.document = oldDocument;
  }
}
test('SSR cleanup does not access browser storage', () => {
  clearAuthStorage();
  clearLegacyCredentials();
});
test('migration discards readable credentials without reading or adopting them', () => {
  browserState(({ values, deleted, events }) => {
    values.set('panda_wiki_token', 'legacy-secret');
    values.set('theme_mode', 'dark');
    clearLegacyCredentials();
    assert.equal(values.has('panda_wiki_token'), false);
    assert.equal(values.get('theme_mode'), 'dark');
    assert.equal(deleted.length, 3);
    assert.ok(deleted.every(value => !value.includes('legacy-secret')));
    assert.deepEqual(events, []);
  });
});
test('clearing identity sends only a non-secret cross-tab notification', () => {
  browserState(({ values, deleted, events }) => {
    clearAuthStorage();
    assert.match(values.get('pw-reader-auth-change'), /^\d+$/);
    assert.equal(deleted.length, 3);
    assert.deepEqual(events, [AUTH_CLEARED_EVENT]);
    assert.ok(deleted.every(value => !value.startsWith('pw_reader_session=')));
  });
});
test('blocked storage cannot prevent identity reset', () => {
  browserState(({ deleted, events }) => {
    window.localStorage.removeItem = window.localStorage.setItem = () => {
      throw new Error('blocked');
    };
    clearAuthStorage();
    assert.equal(deleted.length, 3);
    assert.deepEqual(events, [AUTH_CLEARED_EVENT]);
  });
});
