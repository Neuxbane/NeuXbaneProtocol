import { NxpNode, file } from '../../NxpNode.js';
import { uploadService } from '../../services/uploadService.js';

/**
 * `POST /__nxp/describe-file` — example of the request `file` contract.
 *
 * The handler declares `ref` as a `file` field and receives a **resolved stored
 * record** in `ctx.params.ref` — it never sees multipart bodies, chunk state, or
 * upload ids. The runtime accepts either a signed link token or a bare file id
 * (see `uploadService.resolveRef`).
 */
export default new NxpNode({
  name: 'nxp_describe_file',
  description: 'Example route declaring a file-typed request field (auto-resolved to a record).',
  method: 'POST',
  schema: {
    request: {
      type: 'object',
      properties: {
        ref: file({ required: true, description: 'A signed link token or a stored file id.' })
      },
      required: ['ref'],
      additionalProperties: false
    },
    response: {
      type: 'object',
      properties: {
        id: { type: 'string' },
        originalName: { type: 'string' },
        size: { type: 'integer' },
        checksum: { type: 'string' }
      },
      required: ['id', 'originalName', 'size', 'checksum'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => {
    // `ctx.params.ref` is already a resolved record thanks to the `file` type.
    const record = ctx.params.ref;
    return {
      id: record.id,
      originalName: record.originalName,
      size: record.size,
      checksum: record.checksum
    };
  }
});
