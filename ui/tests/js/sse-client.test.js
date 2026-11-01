'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const SSEClient = require('../../static/js/sse-client.js');

class FakeEventSource {
    static instances = [];

    constructor(url) {
        this.url = url;
        this.listeners = new Map();
        this.closed = false;
        FakeEventSource.instances.push(this);
    }

    addEventListener(name, callback) {
        this.listeners.set(name, callback);
    }

    close() {
        this.closed = true;
    }
}

test.beforeEach(() => {
    FakeEventSource.instances = [];
    global.EventSource = FakeEventSource;
    global.CustomEvent = class CustomEvent {
        constructor(name, options) {
            this.type = name;
            this.detail = options?.detail;
        }
    };
    global.window = { dispatchEvent: () => {} };
});

test.afterEach(() => {
    delete global.EventSource;
    delete global.CustomEvent;
    delete global.window;
});

test('reconnects a dropped stream and resets retry state after opening', async () => {
    let connects = 0;
    const client = new SSEClient('/logs', {
        initialRetryDelay: 1,
        maxRetryDelay: 1,
        maxRetries: 2,
        onConnect: () => { connects += 1; },
    });

    client.connect();
    const first = FakeEventSource.instances[0];
    first.onopen();
    assert.equal(client.getState(), 'connected');

    first.onerror(new Error('network lost'));
    assert.equal(first.closed, true);
    assert.equal(client.getState(), 'error');

    await new Promise(resolve => setTimeout(resolve, 10));
    const second = FakeEventSource.instances[1];
    assert.ok(second, 'a replacement EventSource should be created');
    second.onopen();
    assert.equal(client.getState(), 'connected');
    assert.equal(client.retryCount, 0);
    assert.equal(connects, 2);

    client.disconnect();
});

test('an intentional disconnect cancels pending retries', async () => {
    const client = new SSEClient('/logs', { initialRetryDelay: 5, maxRetries: 2 });
    client.connect();
    const first = FakeEventSource.instances[0];
    first.onopen();
    first.onerror(new Error('network lost'));

    client.disconnect();
    await new Promise(resolve => setTimeout(resolve, 15));

    assert.equal(FakeEventSource.instances.length, 1);
    assert.equal(client.getState(), 'disconnected');
});
