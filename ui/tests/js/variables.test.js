'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

function loadVariablePatterns() {
    global.document = {
        addEventListener: () => {},
        getElementById: () => null,
    };
    global.window = { setTimeout: () => {} };
    delete require.cache[require.resolve('../../static/js/alpine-patterns.js')];
    require('../../static/js/alpine-patterns.js');
    return global.window.alpinePatterns;
}

test.afterEach(() => {
    delete global.window;
    delete global.document;
    delete global.navigator;
});

test('synchronizes variable items between list and JSON modes', () => {
    const creator = loadVariablePatterns().variableCreator();
    creator.items = [
        { id: 1, key: 'username', value: 'alice' },
        { id: 2, key: 'region', value: 'eu-central' },
    ];

    creator.switchEditorMode('json');
    assert.equal(creator.editorMode, 'json');
    assert.deepEqual(JSON.parse(creator.jsonText), { username: 'alice', region: 'eu-central' });

    creator.jsonText = '{"username":"bob","enabled":"true"}';
    creator.switchEditorMode('list');
    assert.equal(creator.editorMode, 'list');
    assert.deepEqual(creator.items.map(({ key, value }) => ({ key, value })), [
        { key: 'username', value: 'bob' },
        { key: 'enabled', value: 'true' },
    ]);
});

test('rejects invalid JSON value types before submission', () => {
    const creator = loadVariablePatterns().variableCreator();
    creator.editorMode = 'json';
    creator.jsonText = '{"count":2}';
    let prevented = false;

    assert.equal(creator.prepareSubmit({ preventDefault: () => { prevented = true; } }), false);
    assert.equal(prevented, true);
    assert.equal(creator.jsonError, 'Value for count must be a string.');
});

test('rejects duplicate list keys before conversion', () => {
    const creator = loadVariablePatterns().variableCreator();
    creator.items = [
        { id: 1, key: 'token', value: 'one' },
        { id: 2, key: ' token ', value: 'two' },
    ];
    let prevented = false;

    assert.equal(creator.prepareSubmit({ preventDefault: () => { prevented = true; } }), false);
    assert.equal(prevented, true);
    assert.equal(creator.jsonError, 'Duplicate item key: token');
});

test('reveals and copies individual values and complete JSON', async () => {
    const copied = [];
    global.navigator = { clipboard: { writeText: async value => { copied.push(value); } } };
    const creator = loadVariablePatterns().variableCreator();
    const item = { id: 1, key: 'token', value: 'secret', revealed: false };
    creator.items = [item];

    creator.toggleItemVisibility(item);
    assert.equal(item.revealed, true);
    await creator.copyItem(item);
    assert.equal(creator.copiedItemId, item.id);
    await creator.copyItemsJSON();
    assert.equal(creator.copiedJSON, true);
    assert.deepEqual(copied, ['secret', '{\n  "token": "secret"\n}']);
});

test('protects unsaved variable edits and clears state after a successful save', () => {
    const creator = loadVariablePatterns().variableCreator();
    creator.$refs = { modifyIndex: { value: '41' } };
    creator.markDirty();
    let prevented = false;
    let stopped = false;
    window.confirm = () => false;
    creator.guardNavigation({
        target: { closest: () => ({ href: '/ui/variables' }) },
        preventDefault: () => { prevented = true; },
        stopImmediatePropagation: () => { stopped = true; },
    });
    assert.equal(prevented, true);
    assert.equal(stopped, true);
    assert.equal(creator.dirty, true);

    creator.handleMutationResult({
        detail: {
            successful: true,
            xhr: { getResponseHeader: () => '42' },
        },
    });
    assert.equal(creator.dirty, false);
    assert.equal(creator.$refs.modifyIndex.value, '42');
});
