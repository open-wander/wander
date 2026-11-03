'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

class FakeTerminal {
    constructor() {
        this.cols = 80;
        this.rows = 24;
        this.handlers = [];
        this.output = '';
    }

    open() {}
    clear() { this.output = ''; }
    focus() {}
    write(value) { this.output += value; }
    writeln(value = '') { this.output += `${value}\n`; }
    dispose() {}

    onData(handler) {
        this.handlers.push(handler);
        return { dispose: () => { this.handlers = this.handlers.filter(candidate => candidate !== handler); } };
    }

    emitData(value) {
        this.handlers.forEach(handler => handler(value));
    }
}

class FakeWebSocket {
    static OPEN = 1;
    static instances = [];

    constructor(url) {
        this.url = url;
        this.readyState = 0;
        this.sent = [];
        this.closed = false;
        FakeWebSocket.instances.push(this);
    }

    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.closed = true; this.readyState = 3; }
    open() { this.readyState = FakeWebSocket.OPEN; this.onopen(); }
    receive(message) { this.onmessage({ data: JSON.stringify(message) }); }
}

function loadExecTerminal() {
    const listeners = new Map();
    global.document = { addEventListener: () => {} };
    global.window = {
        location: { protocol: 'https:', host: 'wander.test' },
        wanderScopeURL: value => `${value}&namespace=team-a&region=region-a`,
        addEventListener: (name, handler) => listeners.set(name, handler),
        removeEventListener: name => listeners.delete(name),
        dispatchEvent: () => {},
    };
    global.Terminal = FakeTerminal;
    global.WebSocket = FakeWebSocket;
    global.CustomEvent = class CustomEvent { constructor(type, init) { this.type = type; this.detail = init.detail; } };
    FakeWebSocket.instances = [];
    delete require.cache[require.resolve('../../static/js/alpine-patterns.js')];
    require('../../static/js/alpine-patterns.js');

    const component = global.window.alpinePatterns.execTerminal('alloc-1', 'web', '/bin/sh');
    component.$refs = { terminal: {} };
    component.$nextTick = callback => callback();
    component.init();
    return component;
}

test.afterEach(() => {
    delete global.window;
    delete global.document;
    delete global.Terminal;
    delete global.WebSocket;
    delete global.CustomEvent;
});

test('sends one input subscription and scoped resize frames across reconnects', () => {
    const terminal = loadExecTerminal();
    terminal.connect();
    const first = FakeWebSocket.instances[0];
    first.open();
    assert.deepEqual(first.sent[0], { type: 'resize', cols: 80, rows: 24 });
    assert.match(first.url, /namespace=team-a&region=region-a/);

    terminal.terminal.emitData('first');
    assert.equal(first.sent.filter(message => message.type === 'input').length, 1);
    terminal.disconnect();
    assert.equal(terminal.connectionState, 'disconnected');
    assert.equal(first.sent.at(-1).type, 'close');

    terminal.connect();
    const second = FakeWebSocket.instances[1];
    second.open();
    terminal.terminal.emitData('second');
    assert.equal(second.sent.filter(message => message.type === 'input').length, 1);
    assert.equal(terminal.terminal.handlers.length, 1);
});

test('preserves command completion and non-zero exit status after socket close', () => {
    const terminal = loadExecTerminal();
    terminal.connect();
    const socket = FakeWebSocket.instances[0];
    socket.open();
    socket.receive({ type: 'exit', code: 7 });

    assert.equal(terminal.connectionState, 'complete');
    assert.equal(terminal.lastExitCode, 7);
    assert.equal(socket.closed, true);
    socket.onclose({ code: 1000 });
    assert.equal(terminal.connectionState, 'complete');
    assert.match(terminal.connectionMessage(), /exit code: 7/);
});

test('distinguishes unexpected socket loss from intentional disconnect', () => {
    const terminal = loadExecTerminal();
    terminal.connect();
    const lostSocket = FakeWebSocket.instances[0];
    lostSocket.open();
    lostSocket.onclose({ code: 1006 });
    assert.equal(terminal.connectionState, 'error');
    assert.match(terminal.connectionMessage(), /Connection lost/);

    terminal.connect();
    const closedSocket = FakeWebSocket.instances[1];
    closedSocket.open();
    terminal.disconnect();
    closedSocket.onclose({ code: 1000 });
    assert.equal(terminal.connectionState, 'disconnected');
});
