import { NxpNode } from '../../../../../NxpNode.js';
import { uploadService } from '../../../../../services/uploadService.js';

/**
 * `PUT /__nxp/upload/:id/chunk/:index` — upload one chunk.
 *
 * The raw request body is streamed straight to disk (never buffered whole), so
 * large chunks do not sit in memory. Chunks may arrive in any order and may be
 * re-sent safely (idempotent).
 */
export default new NxpNode({
  name: 'nxp_upload_chunk',
  description: 'Stores a single chunk of a resumable upload (raw body, streamed to disk).',
  method: 'PUT',
  streamBody: true,
  schema: {
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => uploadService.putChunk(ctx.params.id, ctx.params.index, ctx.req)
});
