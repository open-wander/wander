(function (root, factory) {
    const api = factory();
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }
    root.WanderKeyboardShortcuts = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    const STORAGE_KEY = 'wander.keyboard-shortcuts.v1';
    const BUFFER_TIMEOUT = 750;
    const DEFAULT_BINDINGS = Object.freeze({
        jobs: 'g j',
        storage: 'g r',
        servers: 'g s',
        clients: 'g c',
        topology: 'g t',
        evaluations: 'g e',
        accessControl: 'g a',
        nextTab: 'shift+arrowright',
        previousTab: 'shift+arrowleft',
        nextSection: 'shift+arrowdown',
        previousSection: 'shift+arrowup',
        showHelp: '?',
        upLevel: 'u',
        focusSearch: '/',
    });

    function normalizeEventKey(event) {
        if (!event || typeof event.key !== 'string') {
            return '';
        }

        const key = event.key.toLowerCase();
        if (['shift', 'control', 'alt', 'meta', 'capslock', 'tab'].includes(key)) {
            return '';
        }
        if (key === '/' && event.shiftKey) {
            return '?';
        }
        if (key === '?' || key === '/') {
            return key;
        }

        const modifiers = [];
        if (event.ctrlKey) modifiers.push('ctrl');
        if (event.altKey) modifiers.push('alt');
        if (event.metaKey) modifiers.push('meta');
        if (event.shiftKey) modifiers.push('shift');
        modifiers.push(key === ' ' ? 'space' : key);
        return modifiers.join('+');
    }

    function isEditableTarget(target) {
        if (!target || typeof target !== 'object') {
            return false;
        }
        if (target.isContentEditable) {
            return true;
        }
        const tagName = typeof target.tagName === 'string' ? target.tagName.toLowerCase() : '';
        if (['input', 'textarea', 'select'].includes(tagName)) {
            return true;
        }
        return typeof target.closest === 'function' && Boolean(target.closest('[contenteditable="true"]'));
    }

    function isValidBinding(binding) {
        if (typeof binding !== 'string' || binding.length === 0 || binding.length > 64) {
            return false;
        }
        const tokens = binding.split(' ');
        if (tokens.length > 2 || tokens.some((token) => token.length === 0)) {
            return false;
        }
        const modifierNames = new Set(['ctrl', 'alt', 'meta', 'shift']);
        return tokens.every((token) => {
            const parts = token.split('+');
            const key = parts.pop();
            const modifiers = new Set(parts);
            return Boolean(key) && /^[a-z0-9?/.-]+$/.test(key) && !modifierNames.has(key) &&
                modifiers.size === parts.length && parts.every((part) => modifierNames.has(part));
        });
    }

    function bindingsConflict(first, second) {
        return first === second || first.startsWith(second + ' ') || second.startsWith(first + ' ');
    }

    function addSelectionScope(value, origin, selections) {
        try {
            const url = new URL(value, origin);
            if (url.origin !== origin || !url.pathname.startsWith('/ui/')) return value;
            for (const key of ['namespace', 'region']) {
                if (selections[key] && !url.searchParams.has(key)) url.searchParams.set(key, selections[key]);
            }
            return `${url.pathname}${url.search}${url.hash}`;
        } catch (_) {
            return value;
        }
    }

    function parentPath(pathname) {
        const segments = pathname.split('/').filter(Boolean);
        if (segments.length <= 1) return '/ui/';
        segments.pop();
        return '/' + segments.join('/');
    }

    function sanitizePreferences(value, defaults = DEFAULT_BINDINGS) {
        const fallback = { enabled: true, bindings: { ...defaults } };
        if (!value || typeof value !== 'object') {
            return fallback;
        }

        const bindings = { ...defaults };
        if (value.bindings && typeof value.bindings === 'object') {
            for (const id of Object.keys(defaults)) {
                const candidate = value.bindings[id];
                if (candidate !== undefined) {
                    if (!isValidBinding(candidate)) {
                        return fallback;
                    }
                    bindings[id] = candidate.toLowerCase();
                }
            }
        }

        const configured = Object.values(bindings);
        for (let index = 0; index < configured.length; index += 1) {
            if (configured.slice(index + 1).some((binding) => bindingsConflict(configured[index], binding))) {
                return fallback;
            }
        }
        return {
            enabled: typeof value.enabled === 'boolean' ? value.enabled : true,
            bindings,
        };
    }

    function createSequenceMatcher(bindings, onMatch, timers = {}) {
        const schedule = timers.setTimeout || setTimeout;
        const cancel = timers.clearTimeout || clearTimeout;
        let buffer = [];
        let timeoutID = null;

        function clear() {
            buffer = [];
            if (timeoutID !== null) {
                cancel(timeoutID);
                timeoutID = null;
            }
        }

        function matchBuffer() {
            const sequence = buffer.join(' ');
            const entry = Object.entries(bindings).find(([, binding]) => binding === sequence);
            if (entry) {
                clear();
                onMatch(entry[0]);
                return 'matched';
            }
            if (Object.values(bindings).some((binding) => binding.startsWith(sequence + ' '))) {
                if (timeoutID !== null) cancel(timeoutID);
                timeoutID = schedule(clear, BUFFER_TIMEOUT);
                return 'pending';
            }
            return 'none';
        }

        return {
            push(token) {
                if (!token) return 'none';
                buffer.push(token);
                let result = matchBuffer();
                if (result === 'none' && buffer.length > 1) {
                    clear();
                    buffer.push(token);
                    result = matchBuffer();
                }
                if (result === 'none') clear();
                return result;
            },
            clear,
            getBuffer() {
                return [...buffer];
            },
        };
    }

    function displayBinding(binding) {
        return binding.split(' ').map((token) => token.split('+').map((part) => {
            const names = {
                arrowleft: '←', arrowright: '→', arrowup: '↑', arrowdown: '↓',
                shift: 'Shift', ctrl: 'Ctrl', alt: 'Alt', meta: 'Meta', space: 'Space',
            };
            return names[part] || (part.length === 1 ? part.toUpperCase() : part);
        }).join('+')).join(' then ');
    }

    function createController(win, doc) {
        const dialog = doc.querySelector('[data-keyboard-shortcuts-dialog]');
        if (!dialog) return null;

        let preferences = loadPreferences(win);
        let matcher;
        let returnFocus = null;
        let captureAction = '';
        let captureTokens = [];
        let captureTimeout = null;
        const enabledInput = dialog.querySelector('[data-shortcuts-enabled]');
        const status = dialog.querySelector('[data-shortcuts-status]');

        function loadPreferences(browser) {
            try {
                return sanitizePreferences(JSON.parse(browser.localStorage.getItem(STORAGE_KEY)));
            } catch (_) {
                return sanitizePreferences(null);
            }
        }

        function savePreferences() {
            try {
                win.localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
            } catch (_) {
                setStatus('Your browser could not save this preference.', true);
            }
        }

        function setStatus(message, isError) {
            if (!status) return;
            status.textContent = message;
            status.classList.toggle('text-red-700', Boolean(isError));
            status.classList.toggle('text-neutral-600', !isError);
        }

        function updateBindingLabels() {
            dialog.querySelectorAll('[data-shortcut-id]').forEach((button) => {
                const binding = preferences.bindings[button.dataset.shortcutId];
                if (binding) button.textContent = displayBinding(binding);
            });
            if (enabledInput) enabledInput.checked = preferences.enabled;
        }

        function rebuildMatcher() {
            const activeBindings = preferences.enabled
                ? preferences.bindings
                : { showHelp: preferences.bindings.showHelp };
            matcher = createSequenceMatcher(activeBindings, runAction);
        }

        function scopedURL(path) {
            const scoped = typeof win.wanderScopeURL === 'function' ? win.wanderScopeURL(path) : path;
            return addSelectionScope(scoped, win.location.origin, {
                namespace: doc.body.dataset.wanderNamespace,
                region: doc.body.dataset.wanderRegion,
            });
        }

        function navigate(path) {
            win.location.assign(scopedURL(path));
        }

        function visible(elements) {
            return Array.from(elements).filter((element) => !element.hidden && element.getClientRects().length > 0);
        }

        function cycle(elements, currentIndex, offset) {
            if (elements.length === 0) return;
            const start = currentIndex >= 0 ? currentIndex : 0;
            elements[(start + offset + elements.length) % elements.length].click();
        }

        function cycleTabs(offset) {
            const active = doc.querySelector('[role="tablist"] [role="tab"][aria-selected="true"]');
            if (!active) return;
            const tabList = active.closest('[role="tablist"]');
            const tabs = visible(tabList.querySelectorAll('[role="tab"]'));
            cycle(tabs, tabs.indexOf(active), offset);
        }

        function cycleSections(offset) {
            const links = visible(doc.querySelectorAll('#sidebar nav a[href]'));
            const pathname = win.location.pathname;
            const current = links.findIndex((link) =>
                link.getAttribute('aria-current') === 'page' || new URL(link.href, win.location.origin).pathname === pathname
            );
            cycle(links, current, offset);
        }

        function upLevel() {
            const links = visible(doc.querySelectorAll('#header nav[aria-label="Breadcrumb"] a[href]'));
            const linkedParent = links.length > 0 ? links[links.length - 1].getAttribute('href') : '';
            const linkedPath = linkedParent ? new URL(linkedParent, win.location.origin).pathname : '';
            navigate(linkedPath === '/ui/' || linkedPath === '/ui' ? parentPath(win.location.pathname) : linkedParent);
        }

        function focusSearch() {
            const input = doc.querySelector('input[type="search"], input[placeholder*="Search"]');
            if (input) {
                input.focus();
                if (typeof input.select === 'function') input.select();
            }
        }

        const actions = {
            jobs: () => navigate('/ui/jobs'),
            storage: () => navigate('/ui/csi/volumes'),
            servers: () => navigate('/ui/servers'),
            clients: () => navigate('/ui/clients'),
            topology: () => navigate('/ui/topology'),
            evaluations: () => navigate('/ui/evaluations'),
            accessControl: () => navigate('/ui/settings/tokens'),
            nextTab: () => cycleTabs(1),
            previousTab: () => cycleTabs(-1),
            nextSection: () => cycleSections(1),
            previousSection: () => cycleSections(-1),
            showHelp: open,
            upLevel,
            focusSearch,
        };

        function runAction(id) {
            if (!preferences.enabled && id !== 'showHelp') return false;
            if (!actions[id]) return false;
            actions[id]();
            return true;
        }

        function open() {
            returnFocus = doc.activeElement;
            captureAction = '';
            captureTokens = [];
            updateBindingLabels();
            setStatus('Select a shortcut to change its keys.', false);
            dialog.hidden = false;
            doc.body.style.overflow = 'hidden';
            const closeButton = dialog.querySelector('[data-shortcuts-close]');
            if (closeButton) closeButton.focus();
        }

        function close() {
            if (captureTimeout !== null) win.clearTimeout(captureTimeout);
            captureTimeout = null;
            captureAction = '';
            captureTokens = [];
            dialog.hidden = true;
            doc.body.style.overflow = '';
            if (returnFocus && typeof returnFocus.focus === 'function') returnFocus.focus();
            if (dialog.contains(doc.activeElement) && typeof doc.activeElement.blur === 'function') {
                doc.activeElement.blur();
            }
        }

        function commitBinding(binding) {
            if (captureTimeout !== null) win.clearTimeout(captureTimeout);
            captureTimeout = null;
            const collision = Object.entries(preferences.bindings).find(
                ([id, existing]) => id !== captureAction && bindingsConflict(existing, binding)
            );
            if (collision) {
                setStatus('That key sequence is already assigned. Choose another.', true);
                captureTokens = [];
                return;
            }
            preferences.bindings[captureAction] = binding;
            captureAction = '';
            captureTokens = [];
            savePreferences();
            updateBindingLabels();
            rebuildMatcher();
            setStatus('Shortcut updated.', false);
        }

        function handleCapture(event, token) {
            event.preventDefault();
            event.stopPropagation();
            if (event.key === 'Escape') {
                captureAction = '';
                captureTokens = [];
                setStatus('Shortcut change cancelled.', false);
                return;
            }
            if (event.key === 'Enter' && captureTokens.length === 1) {
                commitBinding(captureTokens[0]);
                return;
            }
            if (!token) return;
            captureTokens.push(token);
            if (captureTokens.length === 2 || token.includes('+') || token === '?' || token === '/') {
                commitBinding(captureTokens.join(' '));
                return;
            }
            setStatus('Press a second key, or Enter to use ' + displayBinding(token) + '.', false);
            if (captureTimeout !== null) win.clearTimeout(captureTimeout);
            captureTimeout = win.setTimeout(() => {
                if (captureAction && captureTokens.length === 1) commitBinding(captureTokens[0]);
            }, BUFFER_TIMEOUT);
        }

        function trapFocus(event) {
            if (event.key !== 'Tab') return false;
            const focusable = visible(dialog.querySelectorAll('button, input, [href], [tabindex]:not([tabindex="-1"])'));
            if (focusable.length === 0) return false;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey && doc.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && doc.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
            return true;
        }

        function handleKeyDown(event) {
            if (event.repeat) return;
            const token = normalizeEventKey(event);
            if (captureAction) {
                handleCapture(event, token);
                return;
            }
            if (!dialog.hidden) {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    close();
                } else {
                    trapFocus(event);
                }
                return;
            }
            if (isEditableTarget(event.target)) return;
            if (preferences.enabled && (token === 'ctrl+k' || token === 'meta+k')) {
                event.preventDefault();
                focusSearch();
                return;
            }
            const result = matcher.push(token);
            if (result !== 'none') {
                event.preventDefault();
                event.stopImmediatePropagation();
            }
        }

        dialog.addEventListener('click', (event) => {
            const bindingButton = event.target.closest('[data-shortcut-id]');
            if (bindingButton) {
                captureAction = bindingButton.dataset.shortcutId;
                captureTokens = [];
                setStatus('Type the new shortcut. Press Escape to cancel.', false);
                return;
            }
            if (event.target.closest('[data-shortcuts-close]') || event.target.hasAttribute('data-shortcuts-backdrop')) {
                close();
            }
        });
        dialog.querySelector('[data-shortcuts-reset]').addEventListener('click', () => {
            preferences.bindings = { ...DEFAULT_BINDINGS };
            savePreferences();
            updateBindingLabels();
            rebuildMatcher();
            setStatus('Default shortcuts restored.', false);
        });
        enabledInput.addEventListener('change', () => {
            preferences.enabled = enabledInput.checked;
            savePreferences();
            rebuildMatcher();
            setStatus(preferences.enabled ? 'Keyboard shortcuts enabled.' : 'Navigation shortcuts disabled. The help shortcut remains available.', false);
        });
        doc.querySelectorAll('[data-open-keyboard-shortcuts]').forEach((button) => button.addEventListener('click', open));
        doc.addEventListener('keydown', handleKeyDown, true);
        rebuildMatcher();
        updateBindingLabels();

        return { open, close, getPreferences: () => ({ enabled: preferences.enabled, bindings: { ...preferences.bindings } }) };
    }

    function init(win) {
        return createController(win, win.document);
    }

    if (typeof window !== 'undefined' && window.document) {
        if (window.document.readyState === 'loading') {
            window.document.addEventListener('DOMContentLoaded', () => init(window), { once: true });
        } else {
            init(window);
        }
    }

    return {
        DEFAULT_BINDINGS,
        addSelectionScope,
        bindingsConflict,
        createSequenceMatcher,
        displayBinding,
        isEditableTarget,
        isValidBinding,
        normalizeEventKey,
        parentPath,
        sanitizePreferences,
    };
});
