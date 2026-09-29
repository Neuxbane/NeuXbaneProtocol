import { NxpNode } from '../../../../NxpNode.js';
import { uploadService } from '../../../../services/uploadService.js';

/**
 * `GET /__nxp/upload/:id` — session status.
 *
 * Reports which chunks have arrived and which are still missing, so a client
 * that lost its connection can resume from exactly where it stopped.
 */
export default new NxpNode({
  name: 'nxp_upload_status',
  description: 'Reports received and missing chunks for a resumable upload session.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => uploadService.status(ctx.params.id)
});
