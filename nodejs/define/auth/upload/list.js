import { NxpNode, arrayOf } from '../../../NxpNode.js';
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
        uniqueBlobs: { type: 'integer' },
        storedBytes: { type: 'integer' },
        deduplicatedBytes: { type: 'integer' },
        chunkSize: { type: 'integer' },
        files: arrayOf({ type: 'file' })
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
