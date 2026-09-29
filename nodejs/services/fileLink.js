import crypto from 'node:crypto';

/**
 * Stateless signed file links.
 *
 * A link is `base64url(payload).base64url(hmacSHA256(payload, LINK_KEY))`.
 * The payload is JSON `{ id, exp, scope }` where `exp` is an absolute epoch
 * (ms). Verification is pure cryptography: the server stores NO per-request
 * state, exactly like the encrypted session cookie. Anyone holding a valid,
 * unexpired token can fetch the file; tampering or expiry yields `403`.
 *
 * The signing key is derived from `NXP_SESSION_SECRET` with a domain-separation
 * suffix so a file link can never be confused with a session cookie.
 */

const SECRET = process.env.NXP_SESSION_SECRET || crypto.randomBytes(32).toString('hex');
const LINK_KEY = crypto.createHash('sha256').update(`${SECRET}:filelink`).digest();

/** Default link lifetime in ms (15 minutes). Override with `NXP_LINK_TTL`. */
export const LINK_TTL = Number(process.env.NXP_LINK_TTL) || 15 * 60 * 1000;

function base64url(buffer) {
  return Buffer.from(buffer).toString('base64url');
}

function sign(payloadB64) {
  return crypto.createHmac('sha256', LINK_KEY).update(payloadB64).digest();
}

/**
 * Create a signed link token for a stored file.
 *
 * @param {string} id - Stored file id.
 * @param {Object} [options]
 * @param {number} [options.ttl] - Lifetime in ms (defaults to `LINK_TTL`).
 * @param {string} [options.scope] - Example permission tag (e.g. `read`).
 * @returns {{ token: string, expiresAt: string }} The token and its ISO expiry.
 */
export function signFileRef(id, { ttl = LINK_TTL, scope = 'read' } = {}) {
  if (!id || typeof id !== 'string') {
    throw new Error('400: A file id is required to sign a link.');
  }

  const exp = Date.now() + ttl;
  const payload = base64url(JSON.stringify({ id, exp, scope }));
  const signature = base64url(sign(payload));

  return {
    token: `${payload}.${signature}`,
    expiresAt: new Date(exp).toISOString()
  };
}

/**
 * Verify a signed link token.
 *
 * @param {string} token - The `payload.signature` token.
 * @returns {{ id: string, exp: number, scope: string }} The decoded payload.
 * @throws {Error} `403: ...` when the token is malformed, tampered, or expired.
 */
export function verifyFileRef(token) {
  if (!token || typeof token !== 'string') {
    throw new Error('403: Missing file link token.');
  }

  const [payload, signature] = token.split('.');
  if (!payload || !signature) {
    throw new Error('403: Malformed file link token.');
  }

  const expected = sign(payload);
  const provided = Buffer.from(signature, 'base64url');

  // Constant-time compare; length mismatch is an automatic failure.
  if (provided.length !== expected.length || !crypto.timingSafeEqual(provided, expected)) {
    throw new Error('403: Invalid file link signature.');
  }

  let decoded;
  try {
    decoded = JSON.parse(Buffer.from(payload, 'base64url').toString('utf8'));
  } catch {
    throw new Error('403: Malformed file link payload.');
  }

  if (!decoded || typeof decoded.id !== 'string') {
    throw new Error('403: Malformed file link payload.');
  }

  if (typeof decoded.exp !== 'number' || Date.now() > decoded.exp) {
    throw new Error('403: File link has expired.');
  }

  return { id: decoded.id, exp: decoded.exp, scope: decoded.scope || 'read' };
}

/**
 * Build the public URL for a signed link token.
 */
export function linkUrl(token) {
  return `/__nxp/link/${token}`;
}
