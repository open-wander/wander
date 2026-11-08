'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const theme = require('../../static/js/theme.js');

function environment(options = {}) {
    const listeners = {};
    const mediaListeners = [];
    const classes = new Set();
    const stored = new Map(Object.entries(options.storage || {}));
    const controls = options.controls || [];
    const document = {
        documentElement: {
            classList: {
                toggle(name, enabled) {
                    if (enabled) classes.add(name);
                    else classes.delete(name);
                },
            },
            dataset: {},
            style: {},
        },
        addEventListener(name, listener) { listeners[name] = listener; },
        dispatchEvent(event) { listeners[event.type]?.(event); },
        querySelectorAll() { return controls; },
    };
    const media = {
        matches: options.systemDark || false,
        addEventListener(name, listener) {
            if (name === 'change') mediaListeners.push(listener);
        },
    };
    return {
        document,
        localStorage: {
            getItem(key) {
                if (options.storageError) throw new Error('blocked');
                return stored.get(key) || null;
            },
            setItem(key, value) {
                if (options.storageError) throw new Error('blocked');
                stored.set(key, value);
            },
        },
        matchMedia: () => media,
        CustomEvent: class CustomEvent {
            constructor(type, init) {
                this.type = type;
                this.detail = init.detail;
            }
        },
        classes,
        listeners,
        media,
        mediaListeners,
        stored,
    };
}

test('normalizes unknown and missing preferences to system', () => {
    assert.equal(theme.normalizePreference(null), theme.SYSTEM);
    assert.equal(theme.normalizePreference('sepia'), theme.SYSTEM);
    assert.equal(theme.normalizePreference(theme.DARK), theme.DARK);
});

test('applies an explicit stored preference before controls are available', () => {
    const env = environment({ storage: { [theme.STORAGE_KEY]: theme.DARK } });
    const controller = theme.createThemeController(env);

    assert.equal(controller.getPreference(), theme.DARK);
    assert.equal(controller.getEffectiveTheme(), theme.DARK);
    assert.equal(env.classes.has('dark'), true);
    assert.equal(env.document.documentElement.dataset.theme, theme.DARK);
    assert.equal(env.document.documentElement.style.colorScheme, theme.DARK);
});

test('system preference follows live color-scheme changes', () => {
    const env = environment({ systemDark: true });
    const controller = theme.createThemeController(env);

    assert.equal(controller.getEffectiveTheme(), theme.DARK);
    env.media.matches = false;
    env.mediaListeners[0]();
    assert.equal(controller.getEffectiveTheme(), theme.LIGHT);
    assert.equal(env.classes.has('dark'), false);
});

test('explicit preference persists and ignores later system changes', () => {
    const env = environment({ systemDark: true });
    const controller = theme.createThemeController(env);

    controller.setPreference(theme.LIGHT);
    assert.equal(env.stored.get(theme.STORAGE_KEY), theme.LIGHT);
    env.media.matches = true;
    env.mediaListeners[0]();
    assert.equal(controller.getEffectiveTheme(), theme.LIGHT);
});

test('blocked storage falls back safely and still permits selection', () => {
    const env = environment({ storageError: true, systemDark: false });
    const controller = theme.createThemeController(env);

    assert.doesNotThrow(() => controller.setPreference(theme.DARK));
    assert.equal(controller.getEffectiveTheme(), theme.DARK);
    assert.equal(env.classes.has('dark'), true);
});

test('synchronizes accessible state after selection and HTMX swaps', () => {
    function control(value) {
        const attributes = { 'data-theme-choice': value };
        return {
            getAttribute: (name) => attributes[name],
            setAttribute: (name, newValue) => { attributes[name] = newValue; },
            attributes,
        };
    }
    const light = control(theme.LIGHT);
    const dark = control(theme.DARK);
    const env = environment({ controls: [light, dark] });
    const controller = theme.createThemeController(env);

    controller.setPreference(theme.DARK);
    env.listeners['htmx:afterSettle']();
    assert.equal(light.attributes['aria-checked'], 'false');
    assert.equal(dark.attributes['aria-checked'], 'true');
    assert.equal(dark.attributes['aria-pressed'], 'true');
});
