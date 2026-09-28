import { NxpNode, arrayOf } from '../../../NxpNode.js';
import { formService, FORM_LIMIT } from '../../../services/formService.js';

/**
 * Bounded collection: the schema caps an input array at the collection limit
 * with `maxItems`, and the service rejects a batch that would overflow the
 * running total.
 */
export default new NxpNode({
  name: 'form_add_bounded',
  description: `Appends an array of entries to the bounded collection (limit ${FORM_LIMIT}).`,
  method: 'POST',
  schema: {
    type: 'object',
    properties: {
      items: arrayOf(
        { oneOf: [{ type: 'object' }, { type: 'string' }, { type: 'number' }] },
        { minItems: 1, maxItems: FORM_LIMIT }
      )
    },
    required: ['items'],
    additionalProperties: false
  },
  handler: async (ctx) => {
    const added = formService.add(ctx.params.items);
    return {
      success: true,
      message: `Added ${added.length} entr${added.length === 1 ? 'y' : 'ies'}.`,
      added,
      ...formService.getState()
    };
  }
});
