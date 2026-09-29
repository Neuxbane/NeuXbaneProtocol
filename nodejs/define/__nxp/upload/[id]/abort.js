import { NxpNode } from '../../../../NxpNode.js';
import { uploadService } from '../../../../services/uploadService.js';

/**
 * `DELETE /__nxp/upload/:id` — abort and discard a session.
 */
export default new NxpNode({
  name: 'nxp_upload_abort',
  description: 'Aborts a resumable upload session and discards its chunks.',
  method: 'DELETE',
  schema: {
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => uploadService.abort(ctx.params.id)
});
