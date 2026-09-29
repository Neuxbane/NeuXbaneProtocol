import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'list_uploads',
  description: 'Lists metadata for every stored file.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      properties: {
        count: { type: 'integer' },
        directory: { type: 'string' },
        maxFileSize: { type: 'integer' },
        maxFiles: { type: 'integer' },
        totalBytes: { type: 'integer' },
        chunkSize: { type: 'integer' },
        files: {
          type: 'array',
          items: {
            type: 'object',
            properties: {
              id: { type: 'string' },
              field: { type: 'string' },
              originalName: { type: 'string' },
              storedName: { type: 'string' },
              mimeType: { type: 'string' },
              size: { type: 'integer' },
              checksum: { type: 'string' },
              uploadedAt: { type: 'string' }
            },
            required: ['id', 'originalName', 'storedName', 'size', 'checksum']
          }
        }
      },
      required: ['count', 'directory', 'maxFileSize', 'maxFiles', 'totalBytes', 'files'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const files = await uploadService.list();
    return {
      count: files.length,
      ...(await uploadService.getState()),
      files
    };
  }
});
