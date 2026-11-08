(function (root, factory) {
    'use strict';

    const api = factory(root);
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    }
    if (root && root.document) {
        root.WanderTheme = api.createThemeController(root);
    }
}(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    const STORAGE_KEY = 'wander.theme';
    const SYSTEM = 'system';
    const LIGHT = 'light';
    const DARK = 'dark';
    const PREFERENCES = new Set([SYSTEM, LIGHT, DARK]);

    function normalizePreference(value) {
        return PREFERENCES.has(value) ? value : SYSTEM;
    }

    function createThemeController(environment) {
        const document = environment.document;
        const mediaQuery = typeof environment.matchMedia === 'function'
            ? environment.matchMedia('(prefers-color-scheme: dark)')
            : null;
        let preference = readPreference();

        function readPreference() {
            try {
                return normalizePreference(environment.localStorage.getItem(STORAGE_KEY));
            } catch (_) {
                return SYSTEM;
            }
        }

        function writePreference(value) {
            try {
                environment.localStorage.setItem(STORAGE_KEY, value);
            } catch (_) {
                // Storage may be unavailable in hardened or private contexts.
            }
        }

        function effectiveTheme() {
            if (preference !== SYSTEM) {
                return preference;
            }
            return mediaQuery && mediaQuery.matches ? DARK : LIGHT;
        }

        function syncControls() {
            if (!document.querySelectorAll) {
                return;
            }
            document.querySelectorAll('[data-theme-choice]').forEach(function (control) {
                const selected = control.getAttribute('data-theme-choice') === preference;
                control.setAttribute('aria-checked', selected ? 'true' : 'false');
                control.setAttribute('aria-pressed', selected ? 'true' : 'false');
                if ('checked' in control) {
                    control.checked = selected;
                }
            });
        }

        function dispatchChange(resolvedTheme) {
            if (!document.dispatchEvent || typeof environment.CustomEvent !== 'function') {
                return;
            }
            document.dispatchEvent(new environment.CustomEvent('wander:theme-change', {
                detail: { preference: preference, theme: resolvedTheme },
            }));
        }

        function apply() {
            const resolvedTheme = effectiveTheme();
            const rootElement = document.documentElement;
            rootElement.classList.toggle(DARK, resolvedTheme === DARK);
            rootElement.dataset.theme = resolvedTheme;
            rootElement.dataset.themePreference = preference;
            rootElement.style.colorScheme = resolvedTheme;
            syncControls();
            dispatchChange(resolvedTheme);
            return resolvedTheme;
        }

        function setPreference(value) {
            preference = normalizePreference(value);
            writePreference(preference);
            return apply();
        }

        function handleChoice(event) {
            const target = event.target && event.target.closest
                ? event.target.closest('[data-theme-choice]')
                : null;
            if (!target) {
                return;
            }
            setPreference(target.getAttribute('data-theme-choice'));
        }

        function handleSystemChange() {
            if (preference === SYSTEM) {
                apply();
            }
        }

        document.addEventListener('click', handleChoice);
        document.addEventListener('DOMContentLoaded', syncControls);
        document.addEventListener('htmx:afterSettle', syncControls);
        if (mediaQuery) {
            if (typeof mediaQuery.addEventListener === 'function') {
                mediaQuery.addEventListener('change', handleSystemChange);
            } else if (typeof mediaQuery.addListener === 'function') {
                mediaQuery.addListener(handleSystemChange);
            }
        }

        apply();

        return Object.freeze({
            apply: apply,
            getEffectiveTheme: effectiveTheme,
            getPreference: function () { return preference; },
            setPreference: setPreference,
        });
    }

    return Object.freeze({
        DARK: DARK,
        LIGHT: LIGHT,
        STORAGE_KEY: STORAGE_KEY,
        SYSTEM: SYSTEM,
        createThemeController: createThemeController,
        normalizePreference: normalizePreference,
    });
}));
