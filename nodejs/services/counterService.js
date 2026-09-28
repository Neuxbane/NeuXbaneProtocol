import { EventEmitter } from 'node:events';

class CounterService extends EventEmitter {
  constructor() {
    super();
    this.count = 0;
    this.intervalId = null;
    this.intervalMs = 1000;
  }

  get isRunning() {
    return this.intervalId !== null;
  }

  getState() {
    return {
      count: this.count,
      running: this.isRunning,
      intervalMs: this.intervalMs
    };
  }

  start(intervalMs = 1000) {
    if (this.isRunning) {
      return this.getState();
    }

    this.intervalMs = Math.max(50, Number(intervalMs) || 1000);
    this.intervalId = setInterval(() => {
      this.count += 1;
      this.emit('tick', this.getState());
    }, this.intervalMs);

    if (typeof this.intervalId.unref === 'function') {
      this.intervalId.unref();
    }

    this.emit('start', this.getState());
    return this.getState();
  }

  stop() {
    if (!this.isRunning) {
      return this.getState();
    }

    clearInterval(this.intervalId);
    this.intervalId = null;
    this.emit('stop', this.getState());
    return this.getState();
  }

  reset(to = 0) {
    this.count = Number(to) || 0;
    this.emit('reset', this.getState());
    return this.getState();
  }
}

export const counterService = new CounterService();
