/**
 * Keeps request-scoped namespace and region explicit in Wander UI URLs.
 * The server remains authoritative; these values are the escaped selections
 * that SelectionMiddleware resolved for the current page.
 */
(function () {
    'use strict';

    const scopedAttributes = [
        'href',
        'action',
        'hx-get',
        'hx-post',
        'hx-put',
        'hx-patch',
        'hx-delete',
        'sse-connect',
        'ws-connect',
        'data-download-url',
    ];

    function selections() {
        const body = document.body;
        return body ? {
            namespace: body.dataset.wanderNamespace || '',
            region: body.dataset.wanderRegion || '',
        } : { namespace: '', region: '' };
    }

    function isApplicationURL(url) {
        const sameOrigin = url.origin === window.location.origin;
        const sameWebSocketHost = (url.protocol === 'ws:' || url.protocol === 'wss:') &&
            url.host === window.location.host;
        return (sameOrigin || sameWebSocketHost) &&
            (url.pathname.startsWith('/ui/') || url.pathname.startsWith('/api/')) &&
            !url.pathname.startsWith('/ui/static/');
    }

    function scopeURL(value) {
        if (typeof value !== 'string' || value === '' || value.startsWith('#')) return value;

        let url;
        try {
            url = new URL(value, window.location.origin);
        } catch (_) {
            return value;
        }
        if (!isApplicationURL(url)) return value;

        const scope = selections();
        for (const key of ['namespace', 'region']) {
            if (scope[key] && !url.searchParams.has(key)) url.searchParams.set(key, scope[key]);
        }

        if (url.protocol === 'http:' || url.protocol === 'https:') return `${url.pathname}${url.search}${url.hash}`;
        return url.toString();
    }

    function scopeElement(element) {
        if (!(element instanceof Element)) return;
        const candidates = [element, ...element.querySelectorAll(scopedAttributes.map(attribute => `[${attribute}]`).join(','))];
        for (const candidate of candidates) {
            for (const attribute of scopedAttributes) {
                if (!candidate.hasAttribute(attribute)) continue;
                const current = candidate.getAttribute(attribute);
                const scoped = scopeURL(current);
                if (scoped !== current) candidate.setAttribute(attribute, scoped);
            }
        }
    }

    const originalFetch = window.fetch.bind(window);
    window.fetch = function (input, init) {
        if (typeof input === 'string') return originalFetch(scopeURL(input), init);
        if (input instanceof URL) return originalFetch(new URL(scopeURL(input.toString()), window.location.origin), init);
        return originalFetch(input, init);
    };

    window.wanderScopeURL = scopeURL;

    function initialize() {
        scopeElement(document.documentElement);
        new MutationObserver(records => {
            for (const record of records) {
                if (record.type === 'attributes') scopeElement(record.target);
                for (const node of record.addedNodes) scopeElement(node);
            }
        }).observe(document.body, {
            attributes: true,
            attributeFilter: scopedAttributes,
            childList: true,
            subtree: true,
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initialize, { once: true });
    } else {
        initialize();
    }
    document.addEventListener('htmx:configRequest', event => {
        event.detail.path = scopeURL(event.detail.path);
    });
    document.addEventListener('htmx:load', event => scopeElement(event.detail.elt));
})();
