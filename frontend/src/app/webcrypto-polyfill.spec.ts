describe('Application WebCrypto polyfill', () => {
  it('generates UUIDs with the version 4 format', () => {
    const first = window.crypto.randomUUID();
    const second = window.crypto.randomUUID();
    const uuidV4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
    expect(first).toMatch(uuidV4);
    expect(second).toMatch(uuidV4);
    expect(first).not.toBe(second);
  });

  it('matches the RFC 7636 SHA-256 PKCE challenge test vector', async () => {
    const verifier = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
    const digest = await window.crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
    const challenge = btoa(String.fromCharCode(...new Uint8Array(digest)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    expect(challenge).toBe('E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
  });

  it('round-trips synthetic AES-GCM data and rejects a modified ciphertext', async () => {
    const key = await window.crypto.subtle.generateKey(
      { name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt'],
    );
    const iv = window.crypto.getRandomValues(new Uint8Array(12));
    const plaintext = new TextEncoder().encode('Synthetic record for a disposable crypto test');
    const ciphertext = await window.crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, plaintext);
    const decrypted = await window.crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ciphertext);
    expect(new Uint8Array(decrypted)).toEqual(plaintext);
    const modified = new Uint8Array(ciphertext);
    modified[0] ^= 1;
    await expectAsync(window.crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, modified))
      .toBeRejected();
  });
});
