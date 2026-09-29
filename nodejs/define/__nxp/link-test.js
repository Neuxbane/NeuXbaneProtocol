import { NxpNode, file } from '../../NxpNode.js';
import { uploadService } from '../../services/uploadService.js';

/**
 * `GET /__nxp/link-test?id=<fileId>` — example of the response `file` contract.
 *
 * The handler returns a plain `{ id }` reference; the runtime serializes the
 * `file`-typed response field into `{ id, url, expiresAt }` where `url` is a
 * stateless signed link (`/__nxp/link/<token>`). The handler never builds URLs
 * or touches upload mechanics — it just declares "this field is a file".
 */
export default new NxpNode({
  name: 'nxp_link_test',
  description: 'Example route returning a file-typed response field (emits a signed link).',
  method: 'GET',
  schema: {
    request: {
      id: { type: 'string', required: true }
    },
    response: {
      type: 'object',
      properties: {
        link: file({ description: 'A stateless signed link to the stored file.' })
      },
      required: ['link'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    // Validate the file exists so a bad id yields 404 rather than a dead link.
    await uploadService.get(ctx.params.id);
    return { link: { id: ctx.params.id } };
  }
});
