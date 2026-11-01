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

test('a task restart reconnects live mode without clearing displayed logs', () => {
    const { viewer } = loadLogViewer();
    const existing = { message: 'before restart' };
    viewer.logs = [existing];
    let reconnects = 0;
    viewer.connect = () => { reconnects += 1; };

    viewer.restartLiveStream();
    assert.equal(reconnects, 1);
    assert.deepEqual(viewer.logs, [existing]);

    viewer.viewMode = 'head';
    viewer.restartLiveStream();
    assert.equal(reconnects, 1);
});

test('stream failures remain visible until manually reconnected', () => {
    const { viewer } = loadLogViewer();
    let disconnected = false;
    viewer.sseClient = { disconnect: () => { disconnected = true; } };

    viewer.handleStreamError({ message: 'upstream failed' });
    assert.equal(disconnected, true);
    assert.equal(viewer.connectionState, 'error');
    assert.equal(viewer.connectionError, 'upstream failed');
});

test('pause buffers lines, resume flushes them, and clear resets both queues', () => {
    const { viewer } = loadLogViewer();
    viewer.$nextTick = callback => callback();
    viewer.isAtBottom = false;
    viewer.isPaused = true;

    viewer.handleLogLine({ message: 'buffered one' });
    viewer.handleLogLine({ message: 'buffered two' });
    assert.equal(viewer.logs.length, 0);
    assert.equal(viewer.bufferedCount, 2);

    viewer.togglePause();
    assert.deepEqual(viewer.logs.map(line => line.message), ['buffered one', 'buffered two']);
    assert.equal(viewer.bufferedCount, 0);

    viewer.isPaused = true;
    viewer.handleLogLine({ message: 'discarded' });
    viewer.clearLogs();
    assert.equal(viewer.logs.length, 0);
    assert.equal(viewer.bufferedCount, 0);
});

test('download uses plain selected log text and a scoped filename', async () => {
    const { viewer } = loadLogViewer();
    let downloaded;
    let clicked = false;
    let anchor;
    global.document.createElement = () => {
        anchor = { click: () => { clicked = true; } };
        return anchor;
    };
    global.document.body = { appendChild: () => {}, removeChild: () => {} };
    const originalCreateObjectURL = URL.createObjectURL;
    const originalRevokeObjectURL = URL.revokeObjectURL;
    URL.createObjectURL = blob => { downloaded = blob; return 'blob:wander-log'; };
    URL.revokeObjectURL = () => {};

    try {
        viewer.logs = [
            { rawMessage: 'first line', timestamp: 'ignored' },
            { rawMessage: 'second line', timestamp: 'ignored' },
        ];
        viewer.downloadLogs();
        assert.equal(clicked, true);
        assert.equal(anchor.download, 'alloc-1-web-stdout.log');
        assert.equal(await downloaded.text(), 'first line\nsecond line');
    } finally {
        URL.createObjectURL = originalCreateObjectURL;
        URL.revokeObjectURL = originalRevokeObjectURL;
    }
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
