'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const shortcuts = require('../../static/js/keyboard-shortcuts.js');

test('default bindings preserve the Nomad 1.6.5 navigation contract', () => {
    const expectedBindings = [
        ['jobs', 'g j'], ['storage', 'g r'], ['servers', 'g s'], ['clients', 'g c'],
        ['topology', 'g t'], ['evaluations', 'g e'], ['accessControl', 'g a'],
        ['nextTab', 'shift+arrowright'], ['previousTab', 'shift+arrowleft'],
        ['nextSection', 'shift+arrowdown'], ['previousSection', 'shift+arrowup'],
        ['showHelp', '?'], ['upLevel', 'u'], ['focusSearch', '/'],
    ];
    assert.deepEqual(Object.entries(shortcuts.DEFAULT_BINDINGS), expectedBindings);
});

test('normalizes printable keys and modified arrows', () => {
    assert.equal(shortcuts.normalizeEventKey({ key: 'G' }), 'g');
    assert.equal(shortcuts.normalizeEventKey({ key: '?', shiftKey: true }), '?');
    assert.equal(shortcuts.normalizeEventKey({ key: '/', shiftKey: true }), '?');
    assert.equal(shortcuts.normalizeEventKey({ key: 'ArrowRight', shiftKey: true }), 'shift+arrowright');
    assert.equal(shortcuts.normalizeEventKey({ key: 'Shift', shiftKey: true }), '');
});

test('detects form fields and editable content', () => {
    assert.equal(shortcuts.isEditableTarget({ tagName: 'INPUT' }), true);
    assert.equal(shortcuts.isEditableTarget({ tagName: 'DIV', isContentEditable: true }), true);
    assert.equal(shortcuts.isEditableTarget({ tagName: 'BUTTON' }), false);
    assert.equal(shortcuts.isEditableTarget(null), false);
});

test('preserves explicit selection scope on navigation targets', () => {
    const scope = { namespace: 'team-a', region: 'region-a' };
    assert.equal(
        shortcuts.addSelectionScope('/ui/jobs', 'https://wander.test', scope),
        '/ui/jobs?namespace=team-a&region=region-a',
    );
    assert.equal(
        shortcuts.addSelectionScope('/ui/jobs?namespace=default', 'https://wander.test', scope),
        '/ui/jobs?namespace=default&region=region-a',
    );
    assert.equal(
        shortcuts.addSelectionScope('https://example.test/ui/jobs', 'https://wander.test', scope),
        'https://example.test/ui/jobs',
    );
});

test('derives the parent route when HTMX leaves root-only breadcrumbs', () => {
    assert.equal(shortcuts.parentPath('/ui/jobs/example'), '/ui/jobs');
    assert.equal(shortcuts.parentPath('/ui/allocations/abc/logs'), '/ui/allocations/abc');
    assert.equal(shortcuts.parentPath('/ui/jobs'), '/ui');
});

test('sanitizes corrupt and colliding preferences', () => {
    const defaults = { jobs: 'g j', clients: 'g c' };
    assert.deepEqual(
        shortcuts.sanitizePreferences({ enabled: false, bindings: { jobs: 'x y' } }, defaults),
        { enabled: false, bindings: { jobs: 'x y', clients: 'g c' } },
    );
    assert.deepEqual(
        shortcuts.sanitizePreferences({ enabled: false, bindings: { jobs: 'g c' } }, defaults),
        { enabled: true, bindings: defaults },
    );
    assert.deepEqual(
        shortcuts.sanitizePreferences({ bindings: { jobs: '<script>' } }, defaults),
        { enabled: true, bindings: defaults },
    );
    assert.deepEqual(
        shortcuts.sanitizePreferences({ bindings: { jobs: 'ctrl++k' } }, defaults),
        { enabled: true, bindings: defaults },
    );
    assert.deepEqual(
        shortcuts.sanitizePreferences({ bindings: { jobs: 'g' } }, defaults),
        { enabled: true, bindings: defaults },
    );
});

test('matches single keys, chords, and retries a mismatched chord tail', () => {
    const matches = [];
    const scheduled = [];
    const matcher = shortcuts.createSequenceMatcher(
        { jobs: 'g j', clients: 'c', focus: '/' },
        (id) => matches.push(id),
        {
            setTimeout: (callback) => {
                scheduled.push(callback);
                return scheduled.length;
            },
            clearTimeout: () => {},
        },
    );

    assert.equal(matcher.push('g'), 'pending');
    assert.equal(matcher.push('j'), 'matched');
    assert.deepEqual(matches, ['jobs']);
    assert.equal(matcher.push('/'), 'matched');
    assert.equal(matcher.push('g'), 'pending');
    assert.equal(matcher.push('c'), 'matched');
    assert.deepEqual(matches, ['jobs', 'focus', 'clients']);
});

test('clears an incomplete chord after the timeout', () => {
    let expire;
    const matcher = shortcuts.createSequenceMatcher(
        { jobs: 'g j' },
        () => assert.fail('incomplete chord must not match'),
        {
            setTimeout: (callback) => {
                expire = callback;
                return 1;
            },
            clearTimeout: () => {},
        },
    );

    matcher.push('g');
    assert.deepEqual(matcher.getBuffer(), ['g']);
    expire();
    assert.deepEqual(matcher.getBuffer(), []);
});

test('the Alpine pattern bundle no longer exports the removed handler', () => {
    const previousWindow = global.window;
    const previousDocument = global.document;
    global.window = {};
    global.document = { addEventListener: () => {} };
    try {
        assert.doesNotThrow(() => require('../../static/js/alpine-patterns.js'));
        assert.equal(global.window.alpinePatterns.keyboardShortcuts, undefined);
    } finally {
        global.window = previousWindow;
        global.document = previousDocument;
    }
});
