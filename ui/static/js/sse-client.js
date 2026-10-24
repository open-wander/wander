/**
 * SSE Client - Auto-reconnecting EventSource wrapper for Wander UI
 *
 * Provides:
 * - Automatic reconnection with exponential backoff
 * - Connection state management
 * - Event handler registration
 * - HTMX integration
 * - Debug logging
 *
 * Example usage:
 *
 *   const client = new SSEClient('/stream/logs?alloc_id=123', {
 *     onConnect: () => console.log('Connected!'),
 *     onDisconnect: () => console.log('Disconnected'),
 *     maxRetries: 10,
 *     debug: true
 *   });
 *
 *   client.on('log', (data) => {
 *     console.log('Received log:', data);
 *   });
 *
 *   client.connect();
 */

class SSEClient {
    constructor(url, options = {}) {
        this.url = url;
        this.options = {
            // Reconnection options
            initialRetryDelay: options.initialRetryDelay || 1000, // 1 second
            maxRetryDelay: options.maxRetryDelay || 30000, // 30 seconds
            maxRetries: options.maxRetries || Infinity,
            backoffMultiplier: options.backoffMultiplier || 2,

            // Lifecycle callbacks
            onConnect: options.onConnect || null,
            onDisconnect: options.onDisconnect || null,
            onError: options.onError || null,
            onMaxRetries: options.onMaxRetries || null,

            // Debug logging
            debug: options.debug || false,

            // EventSource options
            withCredentials: options.withCredentials || false,
        };

        // State
        this.eventSource = null;
        this.state = 'disconnected'; // disconnected, connecting, connected, error
        this.retryCount = 0;
        this.retryDelay = this.options.initialRetryDelay;
        this.retryTimer = null;
        this.intentionallyClosed = false;

        // Event handlers
        this.eventHandlers = new Map();
    }

    /**
     * Register an event handler
     * @param {string} event - Event name to listen for
     * @param {function} handler - Handler function (data) => void
     */
    on(event, handler) {
        if (!this.eventHandlers.has(event)) {
            this.eventHandlers.set(event, []);
        }
        this.eventHandlers.get(event).push(handler);

        // If already connected, register with existing EventSource
        if (this.eventSource && this.state === 'connected') {
            this.eventSource.addEventListener(event, (e) => {
                this._handleEvent(event, e);
            });
        }

        return this;
    }

    /**
     * Unregister an event handler
     * @param {string} event - Event name
     * @param {function} handler - Handler function to remove (optional, removes all if not provided)
     */
    off(event, handler) {
        if (!handler) {
            this.eventHandlers.delete(event);
        } else {
            const handlers = this.eventHandlers.get(event);
            if (handlers) {
                const index = handlers.indexOf(handler);
                if (index > -1) {
                    handlers.splice(index, 1);
                }
                if (handlers.length === 0) {
                    this.eventHandlers.delete(event);
                }
            }
        }
        return this;
    }

    /**
     * Connect to the SSE endpoint
     */
    connect() {
        if (this.state === 'connected' || this.state === 'connecting') {
            this._log('Already connected or connecting');
            return;
        }

        this.intentionallyClosed = false;
        this._connect();
    }

    /**
     * Disconnect from the SSE endpoint
     */
    disconnect() {
        this.intentionallyClosed = true;
        this._clearRetryTimer();
        this._closeEventSource();
        this._setState('disconnected');

        // Dispatch global disconnect event
        if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('sse:disconnect', {
                detail: { id: this.url, url: this.url }
            }));
        }
    }

    /**
     * Get current connection state
     * @returns {string} State: disconnected, connecting, connected, error
     */
    getState() {
        return this.state;
    }

    /**
     * Internal connect logic
     */
    _connect() {
        this._setState('connecting');
        this._log(`Connecting to ${this.url}`);

        try {
            this.eventSource = new EventSource(this.url, {
                withCredentials: this.options.withCredentials
            });

            // Set up standard handlers
            this.eventSource.onopen = () => this._handleOpen();
            this.eventSource.onerror = (e) => this._handleError(e);

            // Register all event handlers
            for (const [event, handlers] of this.eventHandlers.entries()) {
                this.eventSource.addEventListener(event, (e) => {
                    this._handleEvent(event, e);
                });
            }

        } catch (error) {
            this._log('Failed to create EventSource:', error);
            this._scheduleReconnect();
        }
    }

    /**
     * Handle successful connection
     */
    _handleOpen() {
        this._log('Connected');
        this._setState('connected');
        this.retryCount = 0;
        this.retryDelay = this.options.initialRetryDelay;

        // Dispatch global connection event for connection status monitoring
        if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('sse:connect', {
                detail: { id: this.url, url: this.url }
            }));
        }

        if (this.options.onConnect) {
            this.options.onConnect();
        }
    }

    /**
     * Handle connection error
     */
    _handleError(error) {
        if (this.intentionallyClosed) {
            return;
        }

        this._log('Connection error:', error);

        // EventSource automatically reconnects, but we want our own logic
        this._closeEventSource();

        if (this.options.onError) {
            this.options.onError(error);
        }

        if (this.state === 'connected') {
            this._log('Connection lost, will attempt to reconnect');

            // Dispatch global disconnect event
            if (typeof window !== 'undefined') {
                window.dispatchEvent(new CustomEvent('sse:disconnect', {
                    detail: { id: this.url, url: this.url }
                }));
            }

            if (this.options.onDisconnect) {
                this.options.onDisconnect();
            }
        }

        this._setState('error');

        // Dispatch global error event
        if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('sse:error', {
                detail: { id: this.url, url: this.url, error: error }
            }));
        }

        this._scheduleReconnect();
    }

    /**
     * Handle incoming event
     */
    _handleEvent(event, e) {
        const handlers = this.eventHandlers.get(event);
        if (!handlers) {
            return;
        }

        // Parse data if it's JSON
        let data = e.data;
        try {
            data = JSON.parse(e.data);
        } catch {
            // Not JSON, use as-is
        }

        // Call all handlers for this event
        for (const handler of handlers) {
            try {
                handler(data, e);
            } catch (error) {
                this._log(`Error in event handler for '${event}':`, error);
            }
        }
    }

    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    _scheduleReconnect() {
        if (this.intentionallyClosed) {
            return;
        }

        if (this.retryCount >= this.options.maxRetries) {
            this._log('Max retries reached, giving up');
            this._setState('disconnected');
            if (this.options.onMaxRetries) {
                this.options.onMaxRetries();
            }
            return;
        }

        this.retryCount++;
        this._log(`Reconnecting in ${this.retryDelay}ms (attempt ${this.retryCount})`);

        this.retryTimer = setTimeout(() => {
            this._connect();
        }, this.retryDelay);

        // Exponential backoff
        this.retryDelay = Math.min(
            this.retryDelay * this.options.backoffMultiplier,
            this.options.maxRetryDelay
        );
    }

    /**
     * Clear retry timer
     */
    _clearRetryTimer() {
        if (this.retryTimer) {
            clearTimeout(this.retryTimer);
            this.retryTimer = null;
        }
    }

    /**
     * Close EventSource connection
     */
    _closeEventSource() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
    }

    /**
     * Set connection state
     */
    _setState(newState) {
        if (this.state !== newState) {
            this._log(`State: ${this.state} -> ${newState}`);
            this.state = newState;
        }
    }

    /**
     * Debug logging
     */
    _log(...args) {
        if (this.options.debug) {
            console.log('[SSEClient]', ...args);
        }
    }
}

/**
 * HTMX SSE Extension
 *
 * Provides declarative SSE connections via HTML attributes:
 *
 * <div hx-ext="sse" sse-connect="/stream/logs?alloc_id=123">
 *   <div sse-swap="log" hx-swap="beforeend">
 *     <!-- Log entries will be inserted here -->
 *   </div>
 * </div>
 *
 * Attributes:
 * - sse-connect: URL to connect to
 * - sse-swap: Event name to swap on
 * - sse-disconnect: Disconnect on element removal
 * - sse-retry: Max number of retry attempts (default: infinite)
 * - sse-debug: Enable debug logging
 */
(function() {
    if (typeof htmx === 'undefined') {
        console.warn('HTMX SSE Extension: htmx not found, extension not loaded');
        return;
    }

    htmx.defineExtension('sse', {
        onEvent: function(name, evt) {
            const elt = evt.detail.elt;

            if (name === 'htmx:beforeProcessNode') {
                // Initialize SSE connection when element is processed
                const connectUrl = htmx.closest(elt, '[sse-connect]')?.getAttribute('sse-connect');
                if (connectUrl && !elt._sseClient) {
                    this._initSSEClient(elt, connectUrl);
                }
            } else if (name === 'htmx:beforeCleanupElement') {
                // Clean up SSE connection when element is removed
                if (elt._sseClient) {
                    elt._sseClient.disconnect();
                    delete elt._sseClient;
                }
            }
        },

        _initSSEClient: function(elt, url) {
            const debug = elt.hasAttribute('sse-debug');
            const maxRetries = parseInt(elt.getAttribute('sse-retry')) || Infinity;

            const client = new SSEClient(url, {
                debug: debug,
                maxRetries: maxRetries,
                onConnect: () => {
                    elt.classList.add('sse-connected');
                    elt.classList.remove('sse-disconnected', 'sse-error');
                    htmx.trigger(elt, 'sse:connect');
                },
                onDisconnect: () => {
                    elt.classList.add('sse-disconnected');
                    elt.classList.remove('sse-connected');
                    htmx.trigger(elt, 'sse:disconnect');
                },
                onError: () => {
                    elt.classList.add('sse-error');
                    htmx.trigger(elt, 'sse:error');
                },
                onMaxRetries: () => {
                    htmx.trigger(elt, 'sse:maxretries');
                }
            });

            // Find all elements with sse-swap attribute
            const swapElements = elt.querySelectorAll('[sse-swap]');
            swapElements.forEach(swapElt => {
                const eventName = swapElt.getAttribute('sse-swap');
                client.on(eventName, (data) => {
                    // Create a fake HTMX response
                    let content = data;
                    if (typeof data === 'object') {
                        content = JSON.stringify(data);
                    }

                    // Trigger HTMX swap
                    htmx.trigger(swapElt, 'htmx:beforeSwap', {
                        xhr: { response: content },
                        target: swapElt
                    });

                    // Perform the swap
                    const swapStyle = swapElt.getAttribute('hx-swap') || 'innerHTML';
                    htmx.ajax('GET', '', {
                        target: swapElt,
                        swap: swapStyle,
                        source: content
                    });
                });
            });

            // Store client reference and connect
            elt._sseClient = client;
            client.connect();
        }
    });
})();

/**
 * Alpine.js SSE Integration
 *
 * Provides Alpine.js magic property for SSE connections:
 *
 * <div x-data="{ logs: [] }" x-init="$sse('/stream/logs').on('log', (data) => logs.push(data))">
 *   <template x-for="log in logs">
 *     <div x-text="log.message"></div>
 *   </template>
 * </div>
 */
if (typeof Alpine !== 'undefined') {
    Alpine.magic('sse', () => {
        return (url, options = {}) => {
            const client = new SSEClient(url, {
                debug: options.debug || false,
                maxRetries: options.maxRetries || Infinity,
                ...options
            });

            client.connect();

            return client;
        };
    });
}

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SSEClient;
}
