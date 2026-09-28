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

const META_SUFFIX = '.meta.json';
const ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

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
  }

  async ensureDir() {
    if (!this.dirReady) {
      this.dirReady = fsp.mkdir(UPLOAD_DIR, { recursive: true });
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
    const extension = path.extname(originalName).slice(0, 16);
    const storedName = `${id}${extension}`;
    const target = path.join(UPLOAD_DIR, storedName);

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

    await pipeline(stream, fs.createWriteStream(target));

    if (truncated) {
      await fsp.rm(target, { force: true });
      throw new Error(
        `413: File '${originalName}' exceeds the maximum allowed size of ${maxFileSize} bytes.`
      );
    }

    const record = {
      id,
      field: fieldName,
      originalName,
      storedName,
      mimeType: info.mimeType || 'application/octet-stream',
      size,
      checksum: `sha256:${hash.digest('hex')}`,
      uploadedAt: new Date().toISOString()
    };

    return record;
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
    const filePath = path.join(UPLOAD_DIR, record.storedName);
    try {
      await fsp.access(filePath);
    } catch {
      throw new Error(`404: Stored payload for id '${id}' is missing.`);
    }
    return { record, filePath };
  }

  async remove(id) {
    const record = await this.get(id);
    await fsp.rm(path.join(UPLOAD_DIR, record.storedName), { force: true });
    await fsp.rm(metaPath(id), { force: true });
    this.emit('delete', record);
    return record;
  }

  async getState() {
    const files = await this.list();
    return {
      directory: UPLOAD_DIR,
      maxFileSize: this.maxFileSize,
      maxFiles: this.maxFiles,
      count: files.length,
      totalBytes: files.reduce((sum, file) => sum + (file.size || 0), 0)
    };
  }
}

export const uploadService = new UploadService();
