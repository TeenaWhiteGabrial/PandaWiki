const credentialPaths = [
  '/api/v1/user/login',
  '/api/v1/user/reset_password',
  '/api/v1/user/create',
  '/api/v1/user/change_password',
  '/share/v1/auth/change_password',
  '/share/v1/auth/login/account',
  '/share/v1/auth/login/simple',
];

async function sealCredentials(method, path, body, challenge, publicKey) {
  const { default: forge } = await import('node-forge');
  const randomBytes = size =>
    String.fromCharCode(
      ...globalThis.crypto.getRandomValues(new Uint8Array(size)),
    );
  const key = randomBytes(32);
  const iv = randomBytes(12);
  const gcm = forge.cipher.createCipher('AES-GCM', key);
  gcm.start({
    iv,
    additionalData: method.toUpperCase() + ' ' + path + '\n' + challenge,
    tagLength: 128,
  });
  gcm.update(
    forge.util.createBuffer(forge.util.encodeUtf8(JSON.stringify(body))),
  );
  if (!gcm.finish()) throw new Error('Credential encryption failed');
  const wrappedKey = forge.pki
    .publicKeyFromPem(publicKey)
    .encrypt(key, 'RSA-OAEP', {
      md: forge.md.sha256.create(),
      mgf1: { md: forge.md.sha256.create() },
      seed: randomBytes(32),
    });
  return {
    challenge,
    encrypted_key: forge.util.encode64(wrappedKey),
    iv: forge.util.encode64(iv),
    ciphertext: forge.util.encode64(
      gcm.output.getBytes() + gcm.mode.tag.getBytes(),
    ),
  };
}

// Only credential requests fetch a single-use challenge or load the crypto code.
export function encryptCredentialRequest(path, url, method, body, signal) {
  const canonical = credentialPaths.find(item =>
    path.split('?')[0].endsWith(item),
  );
  if (!canonical) return undefined;
  return (async () => {
    const page = new URL(window.location.href);
    const target = new URL(url, page);
    if (target.origin !== page.origin || !target.pathname.endsWith(canonical)) {
      throw new Error('Unexpected credential endpoint');
    }
    const prefix = target.pathname.slice(0, -canonical.length);
    const endpoint = canonical.startsWith('/share/')
      ? '/share/v1/auth/credential-challenge'
      : '/api/v1/user/credential-challenge';
    const response = await fetch(new URL(prefix + endpoint, page), {
      cache: 'no-store',
      credentials: 'same-origin',
      redirect: 'error',
      signal,
    });
    if (!response.ok) throw new Error('Credential encryption unavailable');
    const payload = await response.json();
    const { challenge, public_key } = payload.data || {};
    if (!payload.success || !challenge || !public_key) {
      throw new Error('Invalid credential challenge');
    }
    return sealCredentials(method, canonical, body, challenge, public_key);
  })();
}

export const credentialEncryptionMessage = '密码加密失败，请稍后重新提交';
