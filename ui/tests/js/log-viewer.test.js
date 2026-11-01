'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

function loadLogViewer(storedWrap = null) {
    const storage = new Map();
    if (storedWrap !== null) storage.set('wanderShouldWrapCode', storedWrap);
    global.document = { addEventListener: () => {} };
    global.window = {
        addEventListener: () => {},
        removeEventListener: () => {},
        wanderScopeURL: (url) => url,
        localStorage: {
            getItem: (key) => storage.get(key) ?? null,
            setItem: (key, value) => storage.set(key, value),
        },
    };
    delete require.cache[require.resolve('../../static/js/alpine-patterns.js')];
    require('../../static/js/alpine-patterns.js');
    return { viewer: global.window.alpinePatterns.logViewer('alloc-1', 'web', 'stdout'), storage };
}

function assertLogQuery(viewer, mode, expected) {
    viewer.viewMode = mode;
    const query = new URL(viewer.buildUrl(), 'https://wander.test').searchParams;
    assert.deepEqual(Object.fromEntries(query), {
        task: 'web',
        type: 'stdout',
        ...expected,
    });
}

test.afterEach(() => {
    delete global.window;
    delete global.document;
});

test('builds inherited live, head, and tail log queries', () => {
    const { viewer } = loadLogViewer();
    assertLogQuery(viewer, 'live', { follow: 'true', origin: 'end', offset: '0' });
    assertLogQuery(viewer, 'head', { follow: 'false', origin: 'start', offset: '0' });
    assertLogQuery(viewer, 'tail', { follow: 'false', origin: 'end', offset: '50000' });
});

test('word wrapping defaults off and persists its current value', () => {
    const { viewer, storage } = loadLogViewer();
    viewer.init();
    assert.equal(viewer.wrapLines, false);
    viewer.toggleWrap();
    assert.equal(viewer.wrapLines, true);
    assert.equal(storage.get('wanderShouldWrapCode'), 'true');

    const restored = loadLogViewer('true').viewer;
    restored.init();
    assert.equal(restored.wrapLines, true);
});

test('a completed snapshot closes intentionally and does not remain in an error state', () => {
    const { viewer } = loadLogViewer();
    let disconnected = false;
    viewer.sseClient = { disconnect: () => { disconnected = true; } };
    viewer.connectionState = 'connected';
    viewer.finishSnapshot();
    assert.equal(disconnected, true);
    assert.equal(viewer.sseClient, null);
    assert.equal(viewer.connectionState, 'complete');
});

test('file preview streaming decodes chunks and keeps only the latest 50 KB', () => {
    loadLogViewer();
    const preview = global.window.alpinePatterns.filePreviewStream();
    preview.$root = {
        dataset: { fileStreamUrl: '/ui/allocations/alloc-1/files/stream' },
        querySelector: () => null,
    };
    preview.$nextTick = callback => callback();
    preview.init();
    preview.maxContentLength = 5;
    preview.decoder = new TextDecoder();
    global.window.atob = encoded => Buffer.from(encoded, 'base64').toString('binary');

    preview.appendEncodedChunk(Buffer.from('abc').toString('base64'));
    preview.appendEncodedChunk(Buffer.from('defgh').toString('base64'));

    assert.equal(preview.streamURL, '/ui/allocations/alloc-1/files/stream');
    assert.equal(preview.content, 'defgh');
});
