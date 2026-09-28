import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'list_uploads',
  description: 'Lists metadata for every stored file.',
  method: 'GET',
  handler: async (ctx) => {
    const files = await uploadService.list();
    return {
      count: files.length,
      ...(await uploadService.getState()),
      files
    };
  }
});
