import { NxpNode } from '../../../NxpNode.js';
import { uploadService } from '../../../services/uploadService.js';
import { verifyFileRef } from '../../../services/fileLink.js';
import { streamFile } from '../../../services/streamFile.js';

/**
 * `GET /__nxp/link/:token` — redeem a stateless signed link.
 *
 * The token itself is the capability: it is verified cryptographically (HMAC
 * over the session secret) and carries its own expiry, so the server stores no
 * per-request state. A tampered or expired token yields `403`. No login is
 * required — anyone holding a valid link can fetch the file.
 */
export default new NxpNode({
  name: 'nxp_link',
  description: 'Redeems a stateless signed file link and streams the file.',
  method: 'GET',
  schema: {
    response: {
      type: 'object',
      additionalProperties: true
    }
  },
  handler: async (ctx) => {
    const { id } = verifyFileRef(ctx.params.token);
    const { record, filePath } = await uploadService.getPath(id);
    await streamFile(ctx, record, filePath);
  }
});
