import fs from 'node:fs';
import fsp from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { EventEmitter } from 'node:events';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { fileURLToPath } from 'node:url';
import busboy from 'busboy';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const UPLOAD_DIR = process.env.NXP_UPLOAD_DIR
  ? path.resolve(process.env.NXP_UPLOAD_DIR)
  : path.join(__dirname, '..', 'uploads');

export const MAX_FILE_SIZE = Number(process.env.NXP_MAX_FILE_SIZE) || 10 * 1024 * 1024;
export const MAX_FILES = Number(process.env.NXP_MAX_FILES) || 10;

/** Default chunk size for resumable uploads (5 MiB). */
export const CHUNK_SIZE = Number(process.env.NXP_CHUNK_SIZE) || 5 * 1024 * 1024;
/** Stale chunked sessions older than this are swept (24h). */
export const CHUNK_TTL = Number(process.env.NXP_CHUNK_TTL) || 24 * 60 * 60 * 1000;

const META_SUFFIX = '.meta.json';
const SESSION_FILE = 'session.json';
const CHUNK_DIR = path.join(UPLOAD_DIR, '.chunks');
const ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Content-addressed blob store. Every distinct payload is written exactly once
 * under `blobs/<sha256>`, and any number of metadata records may reference the
 * same blob. This removes duplicate storage when the same bytes are uploaded
 * more than once.
 */
const BLOB_DIR_NAME = 'blobs';
const BLOB_DIR = path.join(UPLOAD_DIR, BLOB_DIR_NAME);
const TEMP_PREFIX = '.tmp-';
/** Orphan blobs newer than this are left alone so in-flight uploads aren't swept (1h). */
const BLOB_GC_GRACE = Number(process.env.NXP_BLOB_GC_GRACE) || 60 * 60 * 1000;

/** Strip path separators and control characters so a name cannot escape the upload dir. */
export function sanitizeFilename(name = '') {
  const base = path.basename(String(name));
  const cleaned = base
    .replace(/[\u0000-\u001f\u007f]/g, '')
    .replace(/[^a-zA-Z0-9._-]+/g, '_')
    .replace(/^[._]+/, '')
    .slice(0, 120);
  return cleaned || 'file';
}

function assertId(id) {
  if (!ID_PATTERN.test(String(id))) {
    throw new Error('400: Invalid file id.');
  }
}

function metaPath(id) {
  return path.join(UPLOAD_DIR, `${id}${META_SUFFIX}`);
}

/** Stored (relative) path for a content-addressed blob, e.g. `blobs/<sha256>`. */
function blobRelPath(hashHex) {
  return path.posix.join(BLOB_DIR_NAME, hashHex);
}

/** Absolute path for a content-addressed blob. */
function blobPath(hashHex) {
  return path.join(BLOB_DIR, hashHex);
}

/** Absolute path to a blob from its stored (relative) name, with traversal guard. */
function resolveStoredPath(storedName) {
  const resolved = path.resolve(UPLOAD_DIR, storedName);
  if (resolved !== UPLOAD_DIR && !resolved.startsWith(UPLOAD_DIR + path.sep)) {
    throw new Error(`400: Invalid stored path for '${storedName}'.`);
  }
  return resolved;
}

function assertUploadId(id) {
  if (!ID_PATTERN.test(String(id))) {
    throw new Error('400: Invalid upload session id.');
  }
}

function sessionDir(uploadId) {
  return path.join(CHUNK_DIR, uploadId);
}

function sessionPath(uploadId) {
  return path.join(sessionDir(uploadId), SESSION_FILE);
}

function chunkPath(uploadId, index) {
  return path.join(sessionDir(uploadId), `${index}.part`);
}

/**
 * Filesystem-backed file upload storage.
 * Parses multipart/form-data with busboy and writes each file plus a JSON
 * metadata sidecar (`<uuid>.meta.json`) into the upload directory.
 */
class UploadService extends EventEmitter {
  constructor() {
    super();
    this.dirReady = null;
    this.maxFileSize = MAX_FILE_SIZE;
    this.maxFiles = MAX_FILES;
    this.locks = new Map();
  }

  async ensureDir() {
    if (!this.dirReady) {
      this.dirReady = fsp.mkdir(BLOB_DIR, { recursive: true });
    }
    return this.dirReady;
  }

  /**
   * Parse a multipart/form-data body and persist every uploaded file.
   * @param {Buffer} rawBody - Raw request body.
   * @param {Object} headers - Request headers (must include content-type).
   * @param {Object} [options]
   * @param {Object|Function} [options.meta] - Extra metadata merged into every file
   *   record. A function receives `(fields, record)` and returns the extra metadata.
   * @param {number} [options.maxFileSize] - Per-file cap in bytes.
   * @param {number} [options.maxFiles] - Maximum number of files per request.
   * @returns {Promise<{ fields: Object, files: Object[] }>}
   */
  async handle(rawBody, headers, options = {}) {
    await this.ensureDir();

    const maxFileSize = options.maxFileSize ?? this.maxFileSize;
    const maxFiles = options.maxFiles ?? this.maxFiles;
    const resolveMeta = typeof options.meta === 'function'
      ? options.meta
      : () => (options.meta || {});

    const fields = {};
    const files = [];
    const jobs = [];
    let failure = null;
    let filesLimitHit = false;

    const bb = busboy({ headers, limits: { fileSize: maxFileSize, files: maxFiles } });

    bb.on('field', (name, value) => {
      fields[name] = Object.hasOwn(fields, name)
        ? [].concat(fields[name], value)
        : value;
    });

    bb.on('filesLimit', () => {
      filesLimitHit = true;
    });

    bb.on('file', (name, stream, info) => {
      if (filesLimitHit) {
        stream.resume();
        return;
      }
      const job = this.#persist(name, stream, info, maxFileSize)
        .then((record) => {
          if (record) files.push(record);
        })
        .catch((error) => {
          failure = failure || error;
        });
      jobs.push(job);
    });

    const parsed = new Promise((resolve, reject) => {
      bb.on('close', resolve);
      bb.on('error', reject);
    });

    Readable.from([rawBody]).pipe(bb);

    try {
      await parsed;
      await Promise.all(jobs);
    } catch (error) {
      failure = failure || error;
    }

    if (filesLimitHit && !failure) {
      failure = new Error(`413: Too many files. Maximum is ${maxFiles} per request.`);
    }

    if (failure) throw failure;

    // Text fields are known once parsing finishes, so merge metadata last.
    const enriched = [];
    for (const record of files) {
      const merged = { ...record, ...resolveMeta(fields, record) };
      await fsp.writeFile(metaPath(merged.id), JSON.stringify(merged, null, 2), 'utf8');
      enriched.push(merged);
      this.emit('upload', merged);
    }

    return { fields, files: enriched };
  }

  async #persist(fieldName, stream, info, maxFileSize) {
    const originalName = sanitizeFilename(info.filename || 'file');
    const id = crypto.randomUUID();
    // Stream into a unique temp file first: the final blob name is derived from
    // the content hash, which is only known once the whole file has been read.
    const temp = path.join(BLOB_DIR, `${TEMP_PREFIX}${id}`);

    const hash = crypto.createHash('sha256');
    let size = 0;
    let truncated = false;

    stream.on('data', (chunk) => {
      hash.update(chunk);
      size += chunk.length;
    });
    stream.on('limit', () => {
      truncated = true;
    });

    try {
      await pipeline(stream, fs.createWriteStream(temp));
    } catch (error) {
      await fsp.rm(temp, { force: true });
      throw error;
    }

    if (truncated) {
      await fsp.rm(temp, { force: true });
      throw new Error(
        `413: File '${originalName}' exceeds the maximum allowed size of ${maxFileSize} bytes.`
      );
    }

    const hex = hash.digest('hex');
    const { storedName, deduplicated } = await this.#commitBlob(temp, hex);

    const record = {
      id,
      field: fieldName,
      originalName,
      storedName,
      mimeType: info.mimeType || 'application/octet-stream',
      size,
      checksum: `sha256:${hex}`,
      deduplicated,
      uploadedAt: new Date().toISOString()
    };

    return record;
  }

  /**
   * Publish a fully written temp file as a content-addressed blob.
   *
   * If a blob with the same hash already exists, the temp copy is discarded and
   * the existing blob is reused (`deduplicated: true`). Otherwise the temp file
   * is atomically renamed into place.
   *
   * @param {string} tempPath - Path to the temp file (already fully written).
   * @param {string} hashHex - Lowercase hex sha256 of the temp file's contents.
   * @returns {Promise<{ storedName: string, deduplicated: boolean }>}
   */
  async #commitBlob(tempPath, hashHex) {
    const storedName = blobRelPath(hashHex);
    const target = blobPath(hashHex);

    // Reuse an existing blob when we can; this is the deduplication win.
    try {
      await fsp.access(target);
      await fsp.rm(tempPath, { force: true });
      return { storedName, deduplicated: true };
    } catch {
      // Not present yet — try to publish it.
    }

    try {
      await fsp.rename(tempPath, target);
      return { storedName, deduplicated: false };
    } catch (error) {
      // A concurrent identical upload may have published the blob first.
      const exists = await fsp.stat(target).then(() => true).catch(() => false);
      await fsp.rm(tempPath, { force: true });
      if (exists) return { storedName, deduplicated: true };
      throw error;
    }
  }

  async list() {
    await this.ensureDir();
    const entries = await fsp.readdir(UPLOAD_DIR);
    const records = [];

    for (const entry of entries) {
      if (!entry.endsWith(META_SUFFIX)) continue;
      try {
        const raw = await fsp.readFile(path.join(UPLOAD_DIR, entry), 'utf8');
        records.push(JSON.parse(raw));
      } catch {
        // Ignore unreadable or partially written metadata.
      }
    }

    return records.sort((a, b) => String(a.uploadedAt).localeCompare(String(b.uploadedAt)));
  }

  async get(id) {
    assertId(id);
    try {
      const raw = await fsp.readFile(metaPath(id), 'utf8');
      return JSON.parse(raw);
    } catch {
      throw new Error(`404: No uploaded file found with id '${id}'.`);
    }
  }

  async getPath(id) {
    const record = await this.get(id);
    const filePath = resolveStoredPath(record.storedName);
    try {
      await fsp.access(filePath);
    } catch {
      throw new Error(`404: Stored payload for id '${id}' is missing.`);
    }
    return { record, filePath };
  }

  async remove(id) {
    const record = await this.get(id);
    await fsp.rm(metaPath(id), { force: true });
    await this.#releaseBlob(record.storedName);
    this.emit('delete', record);
    return record;
  }

  /**
   * Delete a blob only when no remaining metadata record references it. Because
   * blobs are shared by content hash, removing one record must not remove the
   * payload another record still depends on.
   *
   * @param {string} storedName - Stored (relative) blob path.
   * @returns {Promise<boolean>} True when the blob was deleted, false if kept.
   */
  async #releaseBlob(storedName) {
    if (!storedName) return false;
    // The record's own meta has already been removed by the caller, so any match
    // here is a different record that still shares the blob.
    const stillReferenced = (await this.list()).some((r) => r.storedName === storedName);
    if (stillReferenced) return false;
    await fsp.rm(resolveStoredPath(storedName), { force: true });
    return true;
  }

  async getState() {
    const files = await this.list();
    const totalBytes = files.reduce((sum, file) => sum + (file.size || 0), 0);

    // Physical bytes actually on disk: each distinct blob counted once.
    const blobs = new Map();
    for (const file of files) {
      if (file.storedName && !blobs.has(file.storedName)) blobs.set(file.storedName, file.size || 0);
    }
    let storedBytes = 0;
    for (const storedName of blobs.keys()) {
      try {
        storedBytes += (await fsp.stat(resolveStoredPath(storedName))).size;
      } catch {
        storedBytes += blobs.get(storedName); // legacy/missing: fall back to logical size
      }
    }

    return {
      directory: UPLOAD_DIR,
      maxFileSize: this.maxFileSize,
      maxFiles: this.maxFiles,
      chunkSize: CHUNK_SIZE,
      count: files.length,
      totalBytes,
      uniqueBlobs: blobs.size,
      storedBytes,
      deduplicatedBytes: Math.max(totalBytes - storedBytes, 0)
    };
  }

  /**
   * EXAMPLE access policy. Real deployments would consult a database.
   *
   * A file is accessible when any of the following holds:
   *   - it is marked `public`,
   *   - the requester is the recorded owner (`uploadedBy`),
   *   - the requester is an `admin`.
   *
   * @param {Object} record - Stored file metadata.
   * @param {Object|null} user - `session.get('user')` (may be null for anonymous).
   * @returns {boolean}
   */
  canAccess(record, user) {
    if (!record) return false;
    if (record.public === true) return true;
    if (!user) return false;
    if (user.role === 'admin') return true;
    return record.uploadedBy === user.username;
  }

  /**
   * EXAMPLE ownership transfer. Real deployments would persist this in a DB.
   * Returns a shallow copy with the new owner; callers decide whether to persist.
   *
   * @param {Object} record - Stored file metadata.
   * @param {string} username - New owner username.
   * @returns {Object} A copy of the record with `uploadedBy` set.
   */
  transferOwnership(record, username) {
    if (!record) throw new Error('404: No file to transfer.');
    if (!username) throw new Error('400: A target username is required.');
    return { ...record, uploadedBy: username };
  }

  /**
   * Resolve a file reference to a stored record. Accepts either a signed link
   * token (verified cryptographically) or a raw file id.
   *
   * @param {string} ref - A signed token or a file id.
   * @returns {Promise<Object>} The stored file record.
   */
  async resolveRef(ref) {
    if (!ref || typeof ref !== 'string') {
      throw new Error('400: A file reference is required.');
    }
    // A signed token contains a `.` separator; a bare id does not.
    if (ref.includes('.')) {
      const { verifyFileRef } = await import('./fileLink.js');
      const { id } = verifyFileRef(ref);
      return this.get(id);
    }
    return this.get(ref);
  }

  // ---------------------------------------------------------------------------
  // Chunked / resumable uploads
  // ---------------------------------------------------------------------------

  /**
   * Start a resumable upload session. The client uploads `totalChunks` parts
   * (each `chunkSize` bytes, last one may be shorter) then calls `finalize`.
   *
   * @param {Object} options
   * @param {string} options.originalName - Client-supplied file name.
   * @param {number} options.size - Total file size in bytes.
   * @param {number} [options.chunkSize] - Chunk size (defaults to CHUNK_SIZE).
   * @param {string} [options.mimeType] - MIME type.
   * @param {string} [options.field] - Form field name the file belongs to.
   * @param {Object} [options.meta] - Extra metadata merged into the final record.
   * @returns {Promise<Object>} Session descriptor.
   */
  async initChunked({ originalName, size, chunkSize, mimeType, field, meta } = {}) {
    await this.ensureDir();
    await this.sweepStale();

    const total = Number(size);
    if (!Number.isFinite(total) || total < 0) {
      throw new Error('400: A non-negative `size` is required to start an upload.');
    }
    if (total > this.maxFileSize) {
      throw new Error(
        `413: File exceeds the maximum allowed size of ${this.maxFileSize} bytes.`
      );
    }

    const effectiveChunk = Math.min(
      Math.max(Number(chunkSize) || CHUNK_SIZE, 1),
      this.maxFileSize
    );
    const totalChunks = total === 0 ? 0 : Math.ceil(total / effectiveChunk);

    const uploadId = crypto.randomUUID();
    const session = {
      uploadId,
      originalName: sanitizeFilename(originalName || 'file'),
      size: total,
      chunkSize: effectiveChunk,
      totalChunks,
      mimeType: mimeType || 'application/octet-stream',
      field: field || 'file',
      meta: meta && typeof meta === 'object' ? meta : {},
      received: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };

    await fsp.mkdir(sessionDir(uploadId), { recursive: true });
    await this.#writeSession(session);

    return this.#describeSession(session);
  }

  async getSession(uploadId) {
    assertUploadId(uploadId);
    try {
      const raw = await fsp.readFile(sessionPath(uploadId), 'utf8');
      return JSON.parse(raw);
    } catch {
      throw new Error(`404: No upload session found with id '${uploadId}'.`);
    }
  }

  async status(uploadId) {
    const session = await this.getSession(uploadId);
    return this.#describeSession(session);
  }

  /**
   * Store a single chunk. Idempotent: re-uploading the same index overwrites it.
   * @param {string} uploadId
   * @param {number} index - Zero-based chunk index.
   * @param {Buffer|Readable} data - Chunk bytes (Buffer) or a stream.
   * @returns {Promise<Object>} Updated session descriptor.
   */
  async putChunk(uploadId, index, data) {
    const session = await this.getSession(uploadId);
    const idx = Number(index);

    if (!Number.isInteger(idx) || idx < 0 || idx >= session.totalChunks) {
      throw new Error(
        `400: Chunk index ${index} is out of range (0..${Math.max(session.totalChunks - 1, 0)}).`
      );
    }

    return this.#withSessionLock(uploadId, async () => {
      const target = chunkPath(uploadId, idx);
      const source = Buffer.isBuffer(data) || typeof data === 'string'
        ? Readable.from([data])
        : data;

      let written = 0;
      source.on('data', (chunk) => {
        written += chunk.length;
      });

      await pipeline(source, fs.createWriteStream(target));

      const expected = this.#chunkLength(session, idx);
      if (written !== expected) {
        await fsp.rm(target, { force: true });
        throw new Error(
          `400: Chunk ${idx} is ${written} bytes but ${expected} were expected.`
        );
      }

      if (!session.received.includes(idx)) session.received.push(idx);
      session.received.sort((a, b) => a - b);
      session.updatedAt = new Date().toISOString();
      await this.#writeSession(session);

      return this.#describeSession(session);
    });
  }

  /**
   * Assemble all chunks into the final stored file and write its metadata.
   * @returns {Promise<Object>} The stored file record.
   */
  async finalize(uploadId) {
    const session = await this.getSession(uploadId);
    const missing = this.#missingChunks(session);
    if (missing.length) {
      throw new Error(
        `409: Upload incomplete. Missing chunk(s): ${missing.join(', ')}.`
      );
    }

    await this.ensureDir();

    return this.#withSessionLock(uploadId, async () => {
      const id = crypto.randomUUID();
      // Assemble into a temp file; the blob name comes from the content hash.
      const temp = path.join(BLOB_DIR, `${TEMP_PREFIX}${id}`);

      const hash = crypto.createHash('sha256');
      let size = 0;
      const out = fs.createWriteStream(temp);

      try {
        for (let i = 0; i < session.totalChunks; i += 1) {
          const part = chunkPath(uploadId, i);
          const stream = fs.createReadStream(part);
          stream.on('data', (chunk) => {
            hash.update(chunk);
            size += chunk.length;
          });
          await pipeline(stream, out, { end: false });
        }
        await new Promise((resolve, reject) => {
          out.end((error) => (error ? reject(error) : resolve()));
        });
      } catch (error) {
        await fsp.rm(temp, { force: true });
        throw error;
      }

      const hex = hash.digest('hex');
      const { storedName, deduplicated } = await this.#commitBlob(temp, hex);

      const record = {
        id,
        field: session.field,
        originalName: session.originalName,
        storedName,
        mimeType: session.mimeType,
        size,
        checksum: `sha256:${hex}`,
        deduplicated,
        uploadedAt: new Date().toISOString(),
        ...session.meta
      };

      await fsp.writeFile(metaPath(id), JSON.stringify(record, null, 2), 'utf8');
      await fsp.rm(sessionDir(uploadId), { recursive: true, force: true });
      this.emit('upload', record);

      return record;
    });
  }

  async abort(uploadId) {
    const session = await this.getSession(uploadId);
    await fsp.rm(sessionDir(uploadId), { recursive: true, force: true });
    this.emit('abort', session);
    return { uploadId, aborted: true };
  }

  async listSessions() {
    await this.ensureDir();
    let entries = [];
    try {
      entries = await fsp.readdir(CHUNK_DIR, { withFileTypes: true });
    } catch {
      return [];
    }

    const sessions = [];
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      try {
        const raw = await fsp.readFile(sessionPath(entry.name), 'utf8');
        sessions.push(this.#describeSession(JSON.parse(raw)));
      } catch {
        // Ignore partially written sessions.
      }
    }

    return sessions.sort((a, b) => String(a.createdAt).localeCompare(String(b.createdAt)));
  }

  /** Remove sessions older than CHUNK_TTL. */
  async sweepStale(now = Date.now()) {
    const sessions = await this.listSessions();
    const removed = [];
    for (const session of sessions) {
      const age = now - Date.parse(session.updatedAt || session.createdAt);
      if (Number.isFinite(age) && age > CHUNK_TTL) {
        await fsp.rm(sessionDir(session.uploadId), { recursive: true, force: true });
        removed.push(session.uploadId);
      }
    }
    // Also reclaim blobs no metadata references (e.g. abandoned temp files).
    await this.gcBlobs(now);
    return removed;
  }

  /**
   * Delete content-addressed blobs that no metadata record references, plus
   * orphaned temp files. Blobs modified within `BLOB_GC_GRACE` are skipped so
   * an upload that has written its blob but not yet its metadata isn't swept.
   *
   * @param {number} [now] - Reference time in ms (defaults to `Date.now()`).
   * @returns {Promise<string[]>} Names of the removed blob files.
   */
  async gcBlobs(now = Date.now()) {
    await this.ensureDir();

    const referenced = new Set();
    for (const file of await this.list()) {
      if (file.storedName) referenced.add(file.storedName);
    }

    let entries = [];
    try {
      entries = await fsp.readdir(BLOB_DIR, { withFileTypes: true });
    } catch {
      return [];
    }

    const removed = [];
    for (const entry of entries) {
      if (!entry.isFile()) continue;
      const full = path.join(BLOB_DIR, entry.name);
      if (referenced.has(blobRelPath(entry.name))) continue;

      let stat;
      try {
        stat = await fsp.stat(full);
      } catch {
        continue; // vanished concurrently
      }
      if (now - stat.mtimeMs < BLOB_GC_GRACE) continue;

      await fsp.rm(full, { force: true });
      removed.push(entry.name);
    }
    return removed;
  }

  #chunkLength(session, index) {
    if (index < session.totalChunks - 1) return session.chunkSize;
    return session.size - session.chunkSize * (session.totalChunks - 1);
  }

  #missingChunks(session) {
    const missing = [];
    for (let i = 0; i < session.totalChunks; i += 1) {
      if (!session.received.includes(i)) missing.push(i);
    }
    return missing;
  }

  #describeSession(session) {
    const missing = this.#missingChunks(session);
    const bytesReceived = session.received.reduce(
      (sum, idx) => sum + this.#chunkLength(session, idx),
      0
    );
    return {
      uploadId: session.uploadId,
      originalName: session.originalName,
      size: session.size,
      chunkSize: session.chunkSize,
      totalChunks: session.totalChunks,
      mimeType: session.mimeType,
      field: session.field,
      received: [...session.received],
      missing,
      complete: missing.length === 0,
      bytesReceived,
      createdAt: session.createdAt,
      updatedAt: session.updatedAt
    };
  }

  async #writeSession(session) {
    await fsp.writeFile(sessionPath(session.uploadId), JSON.stringify(session, null, 2), 'utf8');
  }

  /** Serialize mutations to a single session so parallel chunk writes don't clobber. */
  async #withSessionLock(uploadId, fn) {
    const previous = this.locks.get(uploadId) || Promise.resolve();
    const run = previous.then(fn, fn);
    this.locks.set(
      uploadId,
      run.then(
        () => {},
        () => {}
      )
    );
    try {
      return await run;
    } finally {
      if (this.locks.get(uploadId) === run) this.locks.delete(uploadId);
    }
  }
}

export const uploadService = new UploadService();
