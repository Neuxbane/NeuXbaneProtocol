import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

/**
 * Resumable upload scope.
 *
 * `POST /__nxp/upload` starts a session and returns the chunk plan.
 * `GET  /__nxp/upload` lists active sessions.
 *
 * The guard requires an authenticated user: a file must have an owner so the
 * example access policy (`uploadService.canAccess`) has something to check.
 */
export default new NxpNode({
  name: 'nxp_upload_scope',
  description: 'Resumable chunked upload sessions.',
  method: 'ANY',
  guard: (ctx) => {
    if (!ctx.session.get('user')) {
      throw new Error('401: Authentication required to start an upload.');
    }
  },
  schema: {
    request: {
      type: 'object',
      properties: {
        originalName: { type: 'string' },
        size: { type: 'integer' },
        chunkSize: { type: 'integer' },
        mimeType: { type: 'string' },
        field: { type: 'string' },
        meta: { type: 'object' }
      },
      additionalProperties: true
    },
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => {
    if (ctx.req.method === 'POST') {
      const user = ctx.session.get('user');
      return uploadService.initChunked({
        originalName: ctx.params.originalName,
        size: ctx.params.size,
        chunkSize: ctx.params.chunkSize,
        mimeType: ctx.params.mimeType,
        field: ctx.params.field,
        meta: { uploadedBy: user?.username || 'unknown', ...(ctx.params.meta || {}) }
      });
    }

    return { sessions: await uploadService.listSessions() };
  }
});
