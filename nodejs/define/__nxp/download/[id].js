import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';
import { streamFile } from '../../../services/streamFile.js';

/**
 * `GET /__nxp/download/:id` — stream a stored file by id.
 *
 * Supports HTTP Range requests for resumable and parallel downloads. Access is
 * governed by the example policy in `uploadService.canAccess`: public files are
 * open, otherwise the owner or an admin may read.
 */
export default new NxpNode({
  name: 'nxp_download',
  description: 'Streams a stored file by id, with HTTP Range support.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => {
    const { record, filePath } = await uploadService.getPath(ctx.params.id);
    const user = ctx.session.get('user');

    if (!uploadService.canAccess(record, user)) {
      throw new Error('403: You do not have access to this file.');
    }

    await streamFile(ctx, record, filePath);
  }
});
