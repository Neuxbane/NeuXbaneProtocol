import { NxpNode } from '../../NxpNode.js';

/**
 * Reserved runtime namespace, defined in the tree like any other scope.
 *
 * `/__nxp` hosts the resumable upload protocol and stateless signed-link
 * downloads. It is intentionally public: the capability to fetch a file is the
 * signed token itself, so no login is required to redeem a link. Upload
 * sessions still require an authenticated user (see `upload/index.js`).
 */
export default new NxpNode({
  name: 'nxp_scope_root',
  description:
    'Reserved runtime namespace: resumable chunked uploads and stateless signed-link downloads.',
  schema: {
    response: {
      type: 'object',
      properties: {
        message: { type: 'string' },
        children: { type: 'array', items: { type: 'string' } }
      },
      required: ['message', 'children'],
      additionalProperties: false
    }
  },
  handler: async (ctx) => ({
    message: 'Reserved NXP runtime namespace.',
    children: Array.from(ctx.targetNode.children.keys())
  })
});
