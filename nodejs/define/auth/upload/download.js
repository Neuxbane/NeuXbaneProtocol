import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';
import { streamFile } from '../../../services/streamFile.js';

export default new NxpNode({
  name: 'download_upload',
  description:
    'Streams a stored file back to the client by id. Supports HTTP Range requests ' +
    'for resumable and parallel downloads.',
  method: 'GET',
  schema: {
    id: { type: 'string', required: true }
  },
  handler: async (ctx) => {
    const { record, filePath } = await uploadService.getPath(ctx.params.id);
    await streamFile(ctx, record, filePath);
  }
});
