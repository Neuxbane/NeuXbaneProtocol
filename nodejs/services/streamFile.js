import { createReadStream } from 'node:fs';
import { pipeline } from 'node:stream/promises';

/**
 * Parse a single-range `Range: bytes=...` header.
 * @returns {{start:number,end:number}|null|'invalid'} null = no range,
 *   'invalid' = unsatisfiable (caller should send 416).
 */
export function parseRange(header, size) {
  if (!header) return null;
  const match = /^bytes=(\d*)-(\d*)$/.exec(String(header).trim());
  if (!match) return 'invalid';

  const [, rawStart, rawEnd] = match;
  if (rawStart === '' && rawEnd === '') return 'invalid';

  let start;
  let end;
  if (rawStart === '') {
    // Suffix range: last N bytes.
    const suffix = Number(rawEnd);
    if (suffix === 0) return 'invalid';
    start = Math.max(size - suffix, 0);
    end = size - 1;
  } else {
    start = Number(rawStart);
    end = rawEnd === '' ? size - 1 : Number(rawEnd);
  }

  if (start > end || start >= size) return 'invalid';
  return { start, end: Math.min(end, size - 1) };
}

/**
 * Stream a stored file to the response, honouring HTTP Range requests.
 *
 * Sets `Accept-Ranges`, `Content-Type`, `Content-Disposition`, and either a
 * `206 Partial Content` (with `Content-Range`) or a `416` for an unsatisfiable
 * range. Writes directly to `ctx.res`, so the caller must not also respond.
 *
 * @param {Object} ctx - Request context (needs `req`, `res`, `setHeader`).
 * @param {Object} record - Stored file metadata (`size`, `mimeType`, `originalName`).
 * @param {string} filePath - Absolute path to the stored payload.
 */
export async function streamFile(ctx, record, filePath) {
  const total = record.size;

  ctx.setHeader('Content-Type', record.mimeType || 'application/octet-stream');
  ctx.setHeader('Accept-Ranges', 'bytes');
  ctx.setHeader(
    'Content-Disposition',
    `attachment; filename="${String(record.originalName).replace(/"/g, '')}"`
  );

  const range = parseRange(ctx.req.headers.range, total);

  if (range === 'invalid') {
    ctx.setHeader('Content-Range', `bytes */${total}`);
    ctx.respond(416, '');
    return;
  }

  if (range) {
    const { start, end } = range;
    ctx.setHeader('Content-Range', `bytes ${start}-${end}/${total}`);
    ctx.setHeader('Content-Length', String(end - start + 1));
    ctx.res.writeHead(206);
    await pipeline(createReadStream(filePath, { start, end }), ctx.res);
    return;
  }

  ctx.setHeader('Content-Length', String(total));
  await pipeline(createReadStream(filePath), ctx.res);
}
