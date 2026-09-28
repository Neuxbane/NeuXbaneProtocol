import { createReadStream } from 'node:fs';
import { pipeline } from 'node:stream/promises';
import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'download_upload',
  description: 'Streams a stored file back to the client by id.',
  method: 'GET',
  schema: {
    id: { type: 'string', required: true }
  },
  handler: async (ctx) => {
    const { record, filePath } = await uploadService.getPath(ctx.params.id);

    ctx.setHeader('Content-Type', record.mimeType || 'application/octet-stream');
    ctx.setHeader('Content-Length', String(record.size));
    ctx.setHeader(
      'Content-Disposition',
      `attachment; filename="${record.originalName.replace(/"/g, '')}"`
    );

    await pipeline(createReadStream(filePath), ctx.res);
  }
});
