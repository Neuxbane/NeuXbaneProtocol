import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'delete_upload',
  description: 'Removes a stored file and its metadata by id.',
  method: 'POST',
  schema: {
    id: { type: 'string', required: true }
  },
  handler: async (ctx) => {
    const record = await uploadService.remove(ctx.params.id);
    return {
      success: true,
      message: `Deleted '${record.originalName}'.`,
      id: record.id
    };
  }
});
