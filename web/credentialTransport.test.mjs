import assert from 'node:assert/strict';
import {
  constants,
  createDecipheriv,
  generateKeyPairSync,
  privateDecrypt,
} from 'node:crypto';
import { test } from 'node:test';
import { encryptCredentialRequest } from './credentialTransport.mjs';

test('HTTP credential body is encrypted and bound to its application route', async t => {
  const { publicKey, privateKey } = generateKeyPairSync('rsa', {
    modulusLength: 2048,
  });
  const challenge = 'synthetic-challenge';
  const page = 'http://wiki.test:3010';
  const path = '/share/v1/auth/change_password';
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url.href, page + '/wiki/share/v1/auth/credential-challenge');
    assert.equal(options.cache, 'no-store');
    return Response.json({
      success: true,
      data: {
        challenge,
        public_key: publicKey.export({ type: 'spki', format: 'pem' }),
      },
    });
  });
  globalThis.window = { location: { href: page + '/wiki' } };
  t.after(() => delete globalThis.window);
  const original = {
    current_password: '合成旧密码42!',
    new_password: '合成新密码87!',
  };
  const sealed = await encryptCredentialRequest(
    path,
    '/wiki' + path,
    'POST',
    original,
  );
  assert.deepEqual(Object.keys(sealed).sort(), [
    'challenge',
    'ciphertext',
    'encrypted_key',
    'iv',
  ]);
  const key = privateDecrypt(
    {
      key: privateKey,
      padding: constants.RSA_PKCS1_OAEP_PADDING,
      oaepHash: 'sha256',
    },
    Buffer.from(sealed.encrypted_key, 'base64'),
  );
  const ciphertext = Buffer.from(sealed.ciphertext, 'base64');
  const decipher = createDecipheriv(
    'aes-256-gcm',
    key,
    Buffer.from(sealed.iv, 'base64'),
  );
  decipher.setAAD(Buffer.from('POST ' + path + '\n' + challenge));
  decipher.setAuthTag(ciphertext.subarray(-16));
  assert.deepEqual(
    JSON.parse(
      Buffer.concat([
        decipher.update(ciphertext.subarray(0, -16)),
        decipher.final(),
      ]),
    ),
    original,
  );
  assert.equal(
    encryptCredentialRequest(
      '/share/v1/node/tree',
      '/wiki/share/v1/node/tree',
      'GET',
    ),
    undefined,
  );
});
