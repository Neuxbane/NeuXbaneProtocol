import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'upload_file',
  description:
    'Uploads one or more files as multipart/form-data. Use the "file" field for each file; ' +
    'any other text field (e.g. "tags", "folder") is stored as metadata.',
  method: 'POST',
  schema: {
    type: 'object',
    properties: {
      tags: { type: 'string' },
      folder: { type: 'string' }
    },
    additionalProperties: true
  },
  handler: async (ctx) => {
    const contentType = ctx.req.headers['content-type'] || '';
    if (!contentType.toLowerCase().startsWith('multipart/form-data')) {
      throw new Error('415: Expected multipart/form-data. Send the file(s) as a multipart form upload.');
    }

    const user = ctx.session.get('user');

    const { fields, files } = await uploadService.handle(ctx.rawBody, ctx.req.headers, {
      meta: (textFields) => ({
        uploadedBy: user?.username || 'unknown',
        folder: textFields.folder || ctx.params.folder || null,
        tags: String(textFields.tags || ctx.params.tags || '')
          .split(',')
          .map((tag) => tag.trim())
          .filter(Boolean)
      })
    });

    if (files.length === 0) {
      throw new Error('400: No files received. Use multipart/form-data with a "file" field.');
    }

    return {
      success: true,
      message: `Stored ${files.length} file(s).`,
      uploadedBy: user?.username || 'unknown',
      fields,
      files
    };
  }
});
