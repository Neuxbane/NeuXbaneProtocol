import crypto from 'node:crypto';
import { EventEmitter } from 'node:events';

/**
 * Append-only collection with an optional stored-entry limit.
 *
 * A limit of `null` means unlimited. The service enforces the limit itself
 * (defense in depth) in addition to the request schema's `maxItems`.
 */
class FormService extends EventEmitter {
  constructor({ limit = null } = {}) {
    super();
    this.limit = limit;
    this.entries = [];
  }

  add(input) {
    const items = Array.isArray(input) ? input : [input];

    if (this.limit != null && this.entries.length + items.length > this.limit) {
      throw new Error(
        `400: Limit exceeded. This collection accepts at most ${this.limit} entries ` +
        `(currently ${this.entries.length}, attempted to add ${items.length}).`
      );
    }

    const created = items.map((item) => {
      const entry = {
        id: crypto.randomUUID(),
        value: item,
        createdAt: new Date().toISOString()
      };
      this.entries.push(entry);
      return entry;
    });

    this.emit('add', { added: created, state: this.getState() });
    return created;
  }

  list() {
    return [...this.entries];
  }

  clear() {
    const removed = this.entries.length;
    this.entries = [];
    this.emit('clear', { removed, state: this.getState() });
    return this.getState();
  }

  getState() {
    return {
      limit: this.limit,
      unlimited: this.limit == null,
      count: this.entries.length,
      remaining: this.limit == null ? null : Math.max(0, this.limit - this.entries.length),
      entries: this.entries
    };
  }
}

export const FORM_LIMIT = Number(process.env.NXP_FORM_LIMIT) || 5;

/** Bounded collection: rejects writes that would exceed FORM_LIMIT. */
export const formService = new FormService({ limit: FORM_LIMIT });

/** Unbounded collection: same semantics with no cap. */
export const unboundedFormService = new FormService({ limit: null });

export { FormService };
