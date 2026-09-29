import { NxpNode } from '../../../../NxpNode.js';
import { uploadService } from '../../../../services/uploadService.js';

/**
 * `POST /__nxp/upload/:id/complete` — assemble the final file.
 *
 * Concatenates the chunks in index order, verifies the checksum, writes the
 * metadata sidecar, and removes the session. Returns `409` if any chunk is
 * still missing.
 */
export default new NxpNode({
  name: 'nxp_upload_complete',
  description: 'Assembles all chunks into the final stored file.',
  method: 'POST',
  schema: {
    response: {
      type: 'object',
      properties: {
        success: { type: 'boolean' },
        file: { type: 'object' }
      },
      required: ['success', 'file'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    const record = await uploadService.finalize(ctx.params.id);
    return { success: true, file: record };
  }
});
