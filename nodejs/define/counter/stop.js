import { NxpNode } from '../../NxpNode.js';
import { counterService } from '../../services/counterService.js';

export default new NxpNode({
  name: 'counter_stop',
  description: 'Stops the background counting script.',
  method: 'POST',
  handler: async () => {
    const state = counterService.stop();
    return {
      success: true,
      message: 'Counter stopped',
      ...state
    };
  }
});
