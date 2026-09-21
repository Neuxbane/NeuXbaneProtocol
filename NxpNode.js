export class NxpNode {
  /**
   * @param {Object} options
   * @param {string} options.name - Action or Module name
   * @param {string} options.description - Semantic description for LLMs and humans
   * @param {string} [options.method='ANY'] - HTTP Method constraint ('GET', 'POST', 'ANY')
   * @param {Object} [options.schema={}] - Parameter/body contract for AI tools
   * @param {Function} [options.guard] - Middleware interceptor (only active if node is an index.js)
   * @param {Function} [options.handler] - Terminal endpoint logic
   */
  constructor({
    name,
    description,
    method = 'ANY',
    schema = {},
    guard = null,
    handler = null
  }) {
    this.name = name;
    this.description = description;
    this.method = method.toUpperCase();
    this.schema = schema;
    this.guard = guard;
    this.handler = handler;
    this.isScopeRoot = false;
    this.children = new Map();
  }

  mount(segment, childNode) {
    this.children.set(segment, childNode);
    return this;
  }

  isLeaf() {
    return this.children.size === 0;
  }

  resolvePipeline(segments, currentPath = '') {
    const pipeline = [{ node: this, path: currentPath || '/' }];
    if (segments.length === 0) return pipeline;

    const [head, ...tail] = segments;
    const nextNode = this.children.get(head);
    if (!nextNode) return null;

    const nextPipeline = nextNode.resolvePipeline(tail, `${currentPath}/${head}`);
    if (!nextPipeline) return null;

    return pipeline.concat(nextPipeline);
  }

  describe(currentPath = '/', activeParentGuards = []) {
    const effectiveGuards = [...activeParentGuards];
    if (this.isScopeRoot && typeof this.guard === 'function') {
      effectiveGuards.push(currentPath);
    }

    const manifest = {
      path: currentPath,
      name: this.name,
      description: this.description,
      method: this.method,
      isScopeRoot: this.isScopeRoot,
      enforcesGuardHere: this.isScopeRoot && typeof this.guard === 'function',
      inheritedGuards: effectiveGuards,
      isLeaf: this.isLeaf(),
      schema: this.schema,
      children: {}
    };

    for (const [key, child] of this.children.entries()) {
      const childPath = currentPath === '/' ? `/${key}` : `${currentPath}/${key}`;
      manifest.children[key] = child.describe(childPath, effectiveGuards);
    }

    return manifest;
  }
}
