import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';

export default new NxpNode({
  name: 'upload_scope_root',
  description: 'Authenticated file storage scope. Upload, list, download and remove files.',
  handler: async (ctx) => ({
    message: 'File upload hub.',
    user: ctx.session.get('user'),
    uploads: await uploadService.getState(),
    children: Array.from(ctx.targetNode.children.keys())
  })
});
