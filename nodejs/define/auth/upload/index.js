import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'upload_scope_root',
  description: 'Authenticated file storage scope. Upload, list, download and remove files.',
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        user: {
          type: 'object',
          properties: {
            username: { type: 'string' },
            role: { type: 'string' },
            loginAt: { type: 'string' }
          },
          required: ['username', 'role']
        },
        uploads: {
          type: 'object',
          properties: {
            directory: { type: 'string' },
            maxFileSize: { type: 'integer' },
            maxFiles: { type: 'integer' },
            count: { type: 'integer' },
            totalBytes: { type: 'integer' },
            uniqueBlobs: { type: 'integer' },
            storedBytes: { type: 'integer' },
            deduplicatedBytes: { type: 'integer' }
          },
          required: ['directory', 'maxFileSize', 'maxFiles', 'count', 'totalBytes']
        },
        children: { type: 'array', items: { type: 'string' } }
      },
      required: ['message', 'user', 'uploads', 'children'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    message: 'File upload hub.',
    user: ctx.session.get('user'),
    uploads: await uploadService.getState(),
    children: Array.from(ctx.targetNode.children.keys())
  })
});
