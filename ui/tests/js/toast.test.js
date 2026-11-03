'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

function loadToastNormalizer() {
    global.document = { addEventListener: () => {} };
    global.window = {};
    delete require.cache[require.resolve('../../static/js/alpine-patterns.js')];
    require('../../static/js/alpine-patterns.js');
    return global.window.alpinePatterns.normalizeToast;
}

test.afterEach(() => {
    delete global.window;
    delete global.document;
});

test('normalizes server type and documented variant toast payloads', () => {
    const normalizeToast = loadToastNormalizer();
    assert.deepEqual(normalizeToast({ type: 'error', message: 'failed' }), {
        type: 'error', message: 'failed', variant: 'error',
    });
    assert.equal(normalizeToast({ variant: 'success', type: 'error' }).variant, 'success');
    assert.equal(normalizeToast({}).variant, 'info');
});
