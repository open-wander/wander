'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

function loadSearchBar() {
    global.document = { addEventListener: () => {}, activeElement: null };
    const triggered = [];
    global.window = {
        htmx: { trigger: (_element, eventName) => { triggered.push(eventName); } },
    };
    delete require.cache[require.resolve('../../static/js/alpine-patterns.js')];
    require('../../static/js/alpine-patterns.js');
    return { search: global.window.alpinePatterns.searchBar(), triggered };
}

test.afterEach(() => {
    delete global.window;
    delete global.document;
});

test('clear retriggers the keyup event configured by SearchBar', () => {
    const { search, triggered } = loadSearchBar();
    search.$refs = {
        input: {
            value: 'query',
            focus: () => {},
            getAttribute: name => name === 'hx-get' ? '/ui/search' : null,
        },
        clearButton: { style: {} },
        hint: { style: {} },
    };

    search.clear();
    assert.equal(search.$refs.input.value, '');
    assert.deepEqual(triggered, ['keyup']);
});
