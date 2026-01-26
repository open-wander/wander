/**
 * Alpine.js Component Patterns for Wander UI
 *
 * This file provides reusable Alpine.js data patterns for common UI components.
 * Alpine.js is used for client-side UI state management, while HTMX handles
 * server interactions and content updates.
 *
 * WHEN TO USE ALPINE vs HTMX:
 *
 * Use Alpine.js for:
 * - UI state (open/close dropdowns, modals, toggles)
 * - Active tab/selection tracking
 * - Client-side form validation before submission
 * - Animations and transitions
 * - Keyboard navigation handling
 * - Showing/hiding elements based on local state
 *
 * Use HTMX for:
 * - Fetching data from server
 * - Submitting forms
 * - Updating page content
 * - Server-side validation
 * - Navigation and routing
 *
 * HTMX + ALPINE INTEGRATION:
 * - Use x-init to reinitialize Alpine components after HTMX swaps
 * - HTMX can trigger Alpine methods via events
 * - Alpine can control when HTMX requests are sent
 */

/**
 * DROPDOWN PATTERN
 *
 * Features:
 * - Toggle open/close state
 * - Click outside to close
 * - Keyboard navigation (Escape to close, Arrow keys for items)
 * - Accessibility (ARIA attributes)
 *
 * Usage in Templ:
 * ```html
 * <div x-data="dropdown()">
 *   <button @click="toggle"
 *           @keydown.escape="close"
 *           :aria-expanded="open">
 *     Select Option
 *   </button>
 *
 *   <div x-show="open"
 *        x-cloak
 *        @click.away="close"
 *        role="menu">
 *     <a href="#" @click="selectItem('option1')" role="menuitem">Option 1</a>
 *     <a href="#" @click="selectItem('option2')" role="menuitem">Option 2</a>
 *   </div>
 * </div>
 * ```
 *
 * With HTMX (load options from server):
 * ```html
 * <div x-data="dropdown()" x-init="$watch('open', value => {
 *   if (value && !loaded) {
 *     htmx.ajax('GET', '/api/options', {target: '#dropdown-content'});
 *     loaded = true;
 *   }
 * })">
 *   <button @click="toggle">Select</button>
 *   <div x-show="open" @click.away="close" id="dropdown-content">
 *     <!-- HTMX will populate this -->
 *   </div>
 * </div>
 * ```
 */
function dropdown(initialOpen = false) {
    return {
        open: initialOpen,
        loaded: false,
        selectedValue: null,

        toggle() {
            this.open = !this.open;
        },

        close() {
            this.open = false;
        },

        selectItem(value) {
            this.selectedValue = value;
            this.close();
            // Optionally dispatch event for HTMX to listen to
            this.$dispatch('dropdown-selected', { value });
        },

        // Keyboard navigation helper
        handleKeydown(event) {
            if (event.key === 'Escape') {
                this.close();
            } else if (event.key === 'ArrowDown') {
                event.preventDefault();
                // Focus next item (implement based on your DOM structure)
            } else if (event.key === 'ArrowUp') {
                event.preventDefault();
                // Focus previous item
            }
        }
    };
}

/**
 * MODAL PATTERN
 *
 * Features:
 * - Open/close state management
 * - Escape key to close
 * - Focus trap concept (keep focus within modal)
 * - Prevent body scroll when open
 * - Backdrop click to close
 *
 * Usage in Templ:
 * ```html
 * <div x-data="modal()">
 *   <button @click="open">Open Modal</button>
 *
 *   <div x-show="isOpen"
 *        x-cloak
 *        @keydown.escape.window="close"
 *        class="fixed inset-0 z-50"
 *        x-transition>
 *     <!-- Backdrop -->
 *     <div @click="close" class="fixed inset-0 bg-black bg-opacity-50"></div>
 *
 *     <!-- Modal content -->
 *     <div class="relative bg-white rounded-lg p-6"
 *          @click.stop
 *          x-trap="isOpen">
 *       <button @click="close" aria-label="Close">X</button>
 *       <h2>Modal Title</h2>
 *       <div>Modal content here</div>
 *     </div>
 *   </div>
 * </div>
 * ```
 *
 * With HTMX (load modal content from server):
 * ```html
 * <div x-data="modal()">
 *   <button hx-get="/modal-content"
 *           hx-target="#modal-body"
 *           @click="open">Load Modal</button>
 *
 *   <div x-show="isOpen" @keydown.escape.window="close">
 *     <div @click="close" class="backdrop"></div>
 *     <div id="modal-body" @click.stop>
 *       <!-- HTMX loads content here -->
 *     </div>
 *   </div>
 * </div>
 * ```
 */
function modal(initialOpen = false) {
    return {
        isOpen: initialOpen,

        open() {
            this.isOpen = true;
            // Prevent body scroll
            document.body.style.overflow = 'hidden';
            // Optionally set focus to first focusable element
            this.$nextTick(() => {
                const firstInput = this.$root.querySelector('input, button, textarea, select');
                if (firstInput) firstInput.focus();
            });
        },

        close() {
            this.isOpen = false;
            // Restore body scroll
            document.body.style.overflow = '';
        },

        // Call this when modal content changes (e.g., after HTMX swap)
        reinit() {
            if (this.isOpen) {
                this.$nextTick(() => {
                    const firstInput = this.$root.querySelector('input, button, textarea, select');
                    if (firstInput) firstInput.focus();
                });
            }
        }
    };
}

/**
 * TABS PATTERN
 *
 * Features:
 * - Track active tab state
 * - Switch between tabs
 * - Keyboard navigation (Arrow keys)
 * - HTMX integration for lazy-loading tab content
 * - URL hash sync (optional)
 *
 * Usage in Templ (Static tabs):
 * ```html
 * <div x-data="tabs(['overview', 'details', 'settings'])">
 *   <div role="tablist">
 *     <button @click="setActive('overview')"
 *             :class="{ 'active': activeTab === 'overview' }"
 *             role="tab"
 *             :aria-selected="activeTab === 'overview'">
 *       Overview
 *     </button>
 *     <button @click="setActive('details')"
 *             :class="{ 'active': activeTab === 'details' }"
 *             role="tab">
 *       Details
 *     </button>
 *     <button @click="setActive('settings')"
 *             :class="{ 'active': activeTab === 'settings' }"
 *             role="tab">
 *       Settings
 *     </button>
 *   </div>
 *
 *   <div role="tabpanel" x-show="activeTab === 'overview'">
 *     Overview content
 *   </div>
 *   <div role="tabpanel" x-show="activeTab === 'details'">
 *     Details content
 *   </div>
 *   <div role="tabpanel" x-show="activeTab === 'settings'">
 *     Settings content
 *   </div>
 * </div>
 * ```
 *
 * With HTMX (Lazy-load tab content):
 * ```html
 * <div x-data="tabs(['overview', 'details', 'settings'], { lazyLoad: true })">
 *   <div role="tablist">
 *     <button @click="loadTab('overview', '/tabs/overview')"
 *             :class="{ 'active': activeTab === 'overview' }">
 *       Overview
 *     </button>
 *     <button @click="loadTab('details', '/tabs/details')"
 *             :class="{ 'active': activeTab === 'details' }">
 *       Details
 *     </button>
 *   </div>
 *
 *   <div id="tab-content" role="tabpanel">
 *     <!-- HTMX loads content here -->
 *   </div>
 * </div>
 * ```
 */
function tabs(tabList = [], options = {}) {
    const defaults = {
        defaultTab: tabList[0] || null,
        lazyLoad: false,
        syncWithHash: false,
        contentTarget: '#tab-content'
    };

    const config = { ...defaults, ...options };

    return {
        activeTab: config.defaultTab,
        loadedTabs: new Set([config.defaultTab]),
        tabList: tabList,

        init() {
            // Sync with URL hash if enabled
            if (config.syncWithHash && window.location.hash) {
                const hashTab = window.location.hash.substring(1);
                if (this.tabList.includes(hashTab)) {
                    this.activeTab = hashTab;
                }
            }

            // Listen for hash changes
            if (config.syncWithHash) {
                window.addEventListener('hashchange', () => {
                    const hashTab = window.location.hash.substring(1);
                    if (this.tabList.includes(hashTab)) {
                        this.setActive(hashTab);
                    }
                });
            }
        },

        setActive(tab) {
            if (!this.tabList.includes(tab)) return;

            this.activeTab = tab;
            this.loadedTabs.add(tab);

            // Update URL hash if enabled
            if (config.syncWithHash) {
                window.location.hash = tab;
            }

            // Dispatch event for external listeners
            this.$dispatch('tab-changed', { tab });
        },

        // For HTMX integration: load tab content from server
        loadTab(tab, url) {
            this.setActive(tab);

            // Only fetch if lazy loading and not already loaded
            if (config.lazyLoad && !this.loadedTabs.has(tab)) {
                htmx.ajax('GET', url, {
                    target: config.contentTarget,
                    swap: 'innerHTML'
                });
                this.loadedTabs.add(tab);
            }
        },

        // Keyboard navigation
        handleKeydown(event, currentTab) {
            const currentIndex = this.tabList.indexOf(currentTab);

            if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
                event.preventDefault();
                const nextIndex = (currentIndex + 1) % this.tabList.length;
                this.setActive(this.tabList[nextIndex]);
                // Focus the next tab button
                event.target.nextElementSibling?.focus();
            } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
                event.preventDefault();
                const prevIndex = currentIndex === 0 ? this.tabList.length - 1 : currentIndex - 1;
                this.setActive(this.tabList[prevIndex]);
                // Focus the previous tab button
                event.target.previousElementSibling?.focus();
            } else if (event.key === 'Home') {
                event.preventDefault();
                this.setActive(this.tabList[0]);
            } else if (event.key === 'End') {
                event.preventDefault();
                this.setActive(this.tabList[this.tabList.length - 1]);
            }
        },

        isActive(tab) {
            return this.activeTab === tab;
        }
    };
}

/**
 * TOGGLE PATTERN
 *
 * Simple boolean toggle for switches, checkboxes, show/hide elements.
 *
 * Usage:
 * ```html
 * <div x-data="toggle(false)">
 *   <button @click="toggleState" :aria-pressed="state">
 *     <span x-show="state">ON</span>
 *     <span x-show="!state">OFF</span>
 *   </button>
 * </div>
 * ```
 */
function toggle(initialState = false) {
    return {
        state: initialState,

        toggleState() {
            this.state = !this.state;
            this.$dispatch('toggle-changed', { state: this.state });
        },

        setState(value) {
            this.state = Boolean(value);
        }
    };
}

/**
 * ACCORDION PATTERN
 *
 * For expandable/collapsible sections.
 *
 * Usage:
 * ```html
 * <div x-data="accordion(['section1', 'section2', 'section3'], { multiple: false })">
 *   <div>
 *     <button @click="toggleSection('section1')"
 *             :aria-expanded="isExpanded('section1')">
 *       Section 1
 *     </button>
 *     <div x-show="isExpanded('section1')" x-collapse>
 *       Section 1 content
 *     </div>
 *   </div>
 *
 *   <div>
 *     <button @click="toggleSection('section2')">Section 2</button>
 *     <div x-show="isExpanded('section2')" x-collapse>
 *       Section 2 content
 *     </div>
 *   </div>
 * </div>
 * ```
 */
function accordion(sections = [], options = {}) {
    const defaults = {
        multiple: false,  // Allow multiple sections open at once
        defaultOpen: []   // Array of initially open sections
    };

    const config = { ...defaults, ...options };

    return {
        expanded: new Set(config.defaultOpen),

        toggleSection(section) {
            if (this.expanded.has(section)) {
                this.expanded.delete(section);
            } else {
                if (!config.multiple) {
                    this.expanded.clear();
                }
                this.expanded.add(section);
            }

            // Force reactivity
            this.expanded = new Set(this.expanded);
        },

        isExpanded(section) {
            return this.expanded.has(section);
        },

        expandAll() {
            if (config.multiple) {
                this.expanded = new Set(sections);
            }
        },

        collapseAll() {
            this.expanded = new Set();
        }
    };
}

/**
 * FORM VALIDATION PATTERN
 *
 * Client-side validation before HTMX submission.
 *
 * Usage:
 * ```html
 * <form x-data="formValidation()" @submit="validateAndSubmit">
 *   <input type="text"
 *          x-model="fields.username"
 *          @blur="validateField('username', required)"
 *          :class="{ 'error': errors.username }">
 *   <span x-show="errors.username" x-text="errors.username"></span>
 *
 *   <button type="submit" :disabled="!isValid">Submit</button>
 * </form>
 * ```
 *
 * Then add HTMX attributes to the form:
 * hx-post="/submit" hx-target="#result"
 */
function formValidation() {
    return {
        fields: {},
        errors: {},
        isValid: true,

        validateField(fieldName, validator) {
            const value = this.fields[fieldName];
            const error = validator(value);

            if (error) {
                this.errors[fieldName] = error;
            } else {
                delete this.errors[fieldName];
            }

            this.updateValidity();
            return !error;
        },

        validateAndSubmit(event) {
            // Validate all fields before allowing HTMX to submit
            const allValid = Object.keys(this.fields).every(field =>
                this.validateField(field, this.validators[field] || (() => null))
            );

            if (!allValid) {
                event.preventDefault();
                return false;
            }

            // HTMX will handle the actual submission
            return true;
        },

        updateValidity() {
            this.isValid = Object.keys(this.errors).length === 0;
        },

        reset() {
            this.fields = {};
            this.errors = {};
            this.isValid = true;
        }
    };
}

// Validator helper functions
const validators = {
    required: (value) => !value ? 'This field is required' : null,
    email: (value) => !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value) ? 'Invalid email address' : null,
    minLength: (min) => (value) => value && value.length < min ? `Minimum length is ${min}` : null,
    maxLength: (max) => (value) => value && value.length > max ? `Maximum length is ${max}` : null,
    pattern: (regex, message) => (value) => !regex.test(value) ? message : null
};

/**
 * HTMX INTEGRATION HELPERS
 *
 * Listen to HTMX events and interact with Alpine components.
 */

// Reinitialize Alpine components after HTMX swaps
document.addEventListener('htmx:afterSwap', (event) => {
    // Alpine will automatically pick up x-data in the swapped content
    // But you can manually reinit specific components if needed
});

// Show loading state in Alpine during HTMX requests
document.addEventListener('htmx:beforeRequest', (event) => {
    // Set Alpine loading state if the component has one
    const alpineComponent = event.target.closest('[x-data]');
    if (alpineComponent && alpineComponent.__x) {
        const data = alpineComponent.__x.$data;
        if ('loading' in data) {
            data.loading = true;
        }
    }
});

document.addEventListener('htmx:afterRequest', (event) => {
    const alpineComponent = event.target.closest('[x-data]');
    if (alpineComponent && alpineComponent.__x) {
        const data = alpineComponent.__x.$data;
        if ('loading' in data) {
            data.loading = false;
        }
    }
});

/**
 * TOAST NOTIFICATION STORE
 *
 * Global Alpine.js store for managing toast notifications.
 *
 * Features:
 * - Add toasts with auto-dismiss
 * - Manual dismiss
 * - Progress bar animation
 * - Stacking with proper spacing
 * - HTMX integration via HX-Trigger
 *
 * Usage from Alpine components:
 * ```html
 * <button @click="$store.toasts.add({
 *   variant: 'success',
 *   title: 'Success',
 *   message: 'Operation completed',
 *   duration: 3000
 * })">Show Toast</button>
 * ```
 *
 * Usage from JavaScript:
 * ```js
 * window.toastStore.add({
 *   variant: 'error',
 *   title: 'Error',
 *   message: 'Something went wrong',
 *   duration: 5000
 * });
 * ```
 *
 * Usage from HTMX (server response header):
 * ```
 * HX-Trigger: {"showToast": {"variant": "info", "title": "Info", "message": "Data loaded", "duration": 3000}}
 * ```
 *
 * Variants: "info", "success", "warning", "error"
 * Duration: milliseconds (0 = manual dismiss only)
 */
document.addEventListener('alpine:init', () => {
    Alpine.store('toasts', {
        list: [],
        counter: 0,

        /**
         * Add a new toast notification
         * @param {Object} toast - Toast configuration
         * @param {string} toast.variant - Toast type: "info", "success", "warning", "error"
         * @param {string} toast.title - Toast title (optional)
         * @param {string} toast.message - Toast message (optional)
         * @param {number} toast.duration - Auto-dismiss duration in ms (0 = manual only)
         * @returns {number} Toast ID
         */
        add(toast) {
            const id = ++this.counter;
            const duration = toast.duration || 0;

            const newToast = {
                id,
                variant: toast.variant || 'info',
                title: toast.title || '',
                message: toast.message || '',
                duration,
                remaining: duration,
                visible: true,
                timerId: null,
                intervalId: null
            };

            this.list.push(newToast);

            // Auto-dismiss if duration is set
            if (duration > 0) {
                this.startAutoDismiss(id, duration);
            }

            return id;
        },

        /**
         * Remove a toast by ID
         * @param {number} id - Toast ID
         */
        remove(id) {
            const index = this.list.findIndex(t => t.id === id);
            if (index === -1) return;

            const toast = this.list[index];

            // Clear timers
            if (toast.timerId) clearTimeout(toast.timerId);
            if (toast.intervalId) clearInterval(toast.intervalId);

            // Hide with animation, then remove from list
            toast.visible = false;

            // Remove from list after animation completes
            setTimeout(() => {
                const currentIndex = this.list.findIndex(t => t.id === id);
                if (currentIndex !== -1) {
                    this.list.splice(currentIndex, 1);
                }
            }, 300); // Match animation duration
        },

        /**
         * Start auto-dismiss timer for a toast
         * @param {number} id - Toast ID
         * @param {number} duration - Duration in ms
         */
        startAutoDismiss(id, duration) {
            const toast = this.list.find(t => t.id === id);
            if (!toast) return;

            // Update progress bar every 50ms
            const updateInterval = 50;
            toast.intervalId = setInterval(() => {
                toast.remaining = Math.max(0, toast.remaining - updateInterval);
            }, updateInterval);

            // Auto-dismiss after duration
            toast.timerId = setTimeout(() => {
                this.remove(id);
            }, duration);
        },

        /**
         * Remove all toasts
         */
        clear() {
            // Clear all timers
            this.list.forEach(toast => {
                if (toast.timerId) clearTimeout(toast.timerId);
                if (toast.intervalId) clearInterval(toast.intervalId);
            });
            this.list = [];
        }
    });
});

// Make toast store available globally for JavaScript access
if (typeof window !== 'undefined') {
    // Wait for Alpine to initialize
    document.addEventListener('alpine:init', () => {
        window.toastStore = Alpine.store('toasts');
    });
}

// HTMX integration: Listen for showToast events
document.addEventListener('htmx:afterRequest', (event) => {
    // Check if the response has a showToast trigger
    const xhr = event.detail.xhr;
    const triggerHeader = xhr.getResponseHeader('HX-Trigger');

    if (!triggerHeader) return;

    try {
        const triggers = JSON.parse(triggerHeader);

        // Handle showToast trigger
        if (triggers.showToast) {
            // Dispatch custom event that ToastContainer listens to
            window.dispatchEvent(new CustomEvent('show-toast', {
                detail: triggers.showToast
            }));
        }
    } catch (e) {
        // Trigger header might be a simple string, not JSON
        // In that case, we ignore it for toast purposes
    }
});

/**
 * LOG VIEWER PATTERN
 *
 * Real-time log streaming via SSE with auto-scroll, pause/resume, and download.
 *
 * Features:
 * - SSE connection with auto-reconnect
 * - Auto-scroll to bottom (can be paused)
 * - Line buffering when paused
 * - Download logs as text file
 * - Clear logs
 * - Timestamp toggle
 * - Connection state indicator
 *
 * Usage in Templ:
 * ```html
 * <div x-data="logViewer('alloc-123', 'nginx', 'stdout')"
 *      x-init="connect()"
 *      x-on:beforeunload.window="disconnect()">
 *   <div x-ref="logContainer" @scroll="handleScroll()" class="overflow-auto h-96">
 *     <template x-for="(line, index) in logs" :key="index">
 *       <div x-text="line.message"></div>
 *     </template>
 *   </div>
 * </div>
 * ```
 */
function logViewer(allocId, initialTask, initialLogType) {
    return {
        // Configuration
        allocId: allocId,
        selectedTask: initialTask || '',
        logType: initialLogType || 'stdout',
        maxLines: 10000,

        // State
        logs: [],
        bufferedLogs: [],
        sseClient: null,
        connectionState: 'disconnected', // disconnected, connecting, connected, error
        isPaused: false,
        isAtBottom: true,
        showTimestamps: false,
        lastUpdate: null,
        ansiParser: null,

        // Computed
        get bufferedCount() {
            return this.bufferedLogs.length;
        },

        // Initialize ANSI parser
        init() {
            if (typeof AnsiParser !== 'undefined') {
                this.ansiParser = new AnsiParser();
            }
        },

        // Build SSE URL
        buildUrl() {
            const params = new URLSearchParams({
                task: this.selectedTask,
                type: this.logType,
            });
            return `/ui/allocations/${this.allocId}/logs/stream?${params.toString()}`;
        },

        // Connect to SSE stream
        connect() {
            this.disconnect();
            this.connectionState = 'connecting';

            const url = this.buildUrl();

            if (typeof SSEClient !== 'undefined') {
                // Use SSEClient class if available
                this.sseClient = new SSEClient(url, {
                    debug: false,
                    maxRetries: 10,
                    onConnect: () => {
                        this.connectionState = 'connected';
                    },
                    onDisconnect: () => {
                        if (this.connectionState === 'connected') {
                            this.connectionState = 'error';
                        }
                    },
                    onError: () => {
                        this.connectionState = 'error';
                    },
                    onMaxRetries: () => {
                        this.connectionState = 'disconnected';
                    }
                });

                // Listen for log events
                this.sseClient.on('log', (data) => {
                    this.handleLogLine(data);
                });

                // Listen for error events
                this.sseClient.on('error', (data) => {
                    console.error('Log stream error:', data);
                });

                this.sseClient.connect();
            } else {
                // Fallback to raw EventSource
                this.sseClient = new EventSource(url);

                this.sseClient.onopen = () => {
                    this.connectionState = 'connected';
                };

                this.sseClient.onerror = () => {
                    this.connectionState = 'error';
                };

                this.sseClient.addEventListener('log', (event) => {
                    try {
                        const data = JSON.parse(event.data);
                        this.handleLogLine(data);
                    } catch (e) {
                        this.handleLogLine({ message: event.data });
                    }
                });
            }
        },

        // Disconnect from SSE stream
        disconnect() {
            if (this.sseClient) {
                if (this.sseClient.disconnect) {
                    this.sseClient.disconnect();
                } else if (this.sseClient.close) {
                    this.sseClient.close();
                }
                this.sseClient = null;
            }
            this.connectionState = 'disconnected';
        },

        // Handle incoming log line
        handleLogLine(data) {
            const rawMessage = data.message || data;
            const parsedMessage = this.ansiParser
                ? this.ansiParser.parse(rawMessage)
                : this.escapeHtml(rawMessage);

            const line = {
                timestamp: data.timestamp || this.formatTimestamp(new Date()),
                message: parsedMessage,
                rawMessage: rawMessage, // Keep raw for downloading
                level: data.level || null
            };

            this.lastUpdate = new Date();

            if (this.isPaused) {
                this.bufferedLogs.push(line);
            } else {
                this.appendLine(line);
            }
        },

        // Escape HTML for safety when ANSI parser is unavailable
        escapeHtml(text) {
            const htmlEscapes = {
                '&': '&amp;',
                '<': '&lt;',
                '>': '&gt;',
                '"': '&quot;',
                "'": '&#39;'
            };
            return String(text).replace(/[&<>"']/g, char => htmlEscapes[char]);
        },

        // Append line to logs
        appendLine(line) {
            this.logs.push(line);

            // Trim old logs if exceeding max
            if (this.logs.length > this.maxLines) {
                this.logs.splice(0, this.logs.length - this.maxLines);
            }

            // Auto-scroll if at bottom
            if (this.isAtBottom) {
                this.$nextTick(() => {
                    this.scrollToBottom();
                });
            }
        },

        // Switch task
        switchTask() {
            this.clearLogs();
            this.connect();
        },

        // Set log type (stdout/stderr)
        setLogType(type) {
            if (this.logType !== type) {
                this.logType = type;
                this.clearLogs();
                this.connect();
            }
        },

        // Toggle pause state
        togglePause() {
            this.isPaused = !this.isPaused;

            if (!this.isPaused) {
                // Flush buffered logs
                this.bufferedLogs.forEach(line => this.appendLine(line));
                this.bufferedLogs = [];
            }
        },

        // Clear all logs
        clearLogs() {
            this.logs = [];
            this.bufferedLogs = [];
        },

        // Handle scroll events
        handleScroll() {
            const container = this.$refs.logContainer;
            if (!container) return;

            const threshold = 50; // pixels from bottom
            const atBottom = container.scrollHeight - container.scrollTop - container.clientHeight < threshold;

            this.isAtBottom = atBottom;

            // Auto-resume when user scrolls to bottom
            if (atBottom && this.isPaused && this.bufferedLogs.length > 0) {
                this.togglePause();
            }
        },

        // Scroll to bottom
        scrollToBottom() {
            const container = this.$refs.logContainer;
            if (container) {
                container.scrollTop = container.scrollHeight;
                this.isAtBottom = true;
            }
        },

        // Download logs as text file
        downloadLogs() {
            const lines = this.logs.map(line => {
                // Use raw message if available (to strip ANSI codes), otherwise strip from parsed
                const plainMessage = line.rawMessage
                    ? (typeof AnsiParser !== 'undefined' ? AnsiParser.strip(line.rawMessage) : line.rawMessage)
                    : line.message.replace(/<[^>]*>/g, ''); // Strip HTML tags as fallback

                if (this.showTimestamps && line.timestamp) {
                    return `${line.timestamp} ${plainMessage}`;
                }
                return plainMessage;
            });

            const content = lines.join('\n');
            const blob = new Blob([content], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);

            const a = document.createElement('a');
            a.href = url;
            a.download = `${this.allocId.substring(0, 8)}-${this.selectedTask}-${this.logType}.log`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        },

        // Format timestamp for display
        formatTimestamp(date) {
            const pad = (n) => n.toString().padStart(2, '0');
            return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
                   `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
        },

        // Format time for "last update" display
        formatTime(date) {
            if (!date) return '';
            const now = new Date();
            const diff = Math.floor((now - date) / 1000);

            if (diff < 5) return 'just now';
            if (diff < 60) return `${diff}s ago`;
            if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
            return date.toLocaleTimeString();
        }
    };
}

/**
 * EXEC TERMINAL PATTERN
 *
 * Interactive terminal emulator using xterm.js for exec sessions.
 *
 * Features:
 * - xterm.js terminal emulator
 * - WebSocket connection for bidirectional I/O
 * - Auto-fit to container size
 * - Window resize handling
 * - Connection state management
 * - Disconnect on page unload
 *
 * Usage in Templ:
 * ```html
 * <div x-data="execTerminal('alloc-123', 'nginx', '/bin/sh')"
 *      x-init="init()"
 *      x-on:beforeunload.window="disconnect()">
 *   <div x-ref="terminal" style="height: 500px;"></div>
 *   <button @click="toggleConnection()">Connect/Disconnect</button>
 * </div>
 * ```
 *
 * Dependencies:
 * - xterm.js (loaded as /static/js/xterm.min.js)
 * - xterm-addon-fit (loaded as /static/js/xterm-addon-fit.min.js)
 *
 * Note: The WebSocket connection logic is prepared here but will be
 * fully implemented in Task 72 (WebSocket Exec Handler).
 */
function execTerminal(allocId, initialTask, initialCommand) {
    return {
        // Configuration
        allocId: allocId,
        selectedTask: initialTask || '',
        command: initialCommand || '/bin/sh',

        // State
        terminal: null,
        fitAddon: null,
        websocket: null,
        connectionState: 'disconnected', // disconnected, connecting, connected, error
        resizeObserver: null,
        resizeTimeout: null, // For debouncing resize events
        lastSentDims: { cols: 0, rows: 0 }, // Track last sent dimensions to avoid duplicates

        // Initialize xterm.js terminal
        init() {
            // Check if xterm.js is loaded
            if (typeof Terminal === 'undefined') {
                console.error('xterm.js not loaded. Make sure xterm.min.js is included.');
                return;
            }

            // Create terminal instance with dark theme
            this.terminal = new Terminal({
                cursorBlink: true,
                cursorStyle: 'block',
                fontSize: 14,
                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                theme: {
                    background: '#1e1e1e',
                    foreground: '#d4d4d4',
                    cursor: '#d4d4d4',
                    cursorAccent: '#1e1e1e',
                    black: '#000000',
                    red: '#cd3131',
                    green: '#0dbc79',
                    yellow: '#e5e510',
                    blue: '#2472c8',
                    magenta: '#bc3fbc',
                    cyan: '#11a8cd',
                    white: '#e5e5e5',
                    brightBlack: '#666666',
                    brightRed: '#f14c4c',
                    brightGreen: '#23d18b',
                    brightYellow: '#f5f543',
                    brightBlue: '#3b8eea',
                    brightMagenta: '#d670d6',
                    brightCyan: '#29b8db',
                    brightWhite: '#ffffff'
                },
                scrollback: 10000,
                allowProposedApi: true
            });

            // Load fit addon if available
            if (typeof FitAddon !== 'undefined') {
                this.fitAddon = new FitAddon.FitAddon();
                this.terminal.loadAddon(this.fitAddon);
            }

            // Open terminal in the container
            const container = this.$refs.terminal;
            if (container) {
                this.terminal.open(container);

                // Initial fit
                this.$nextTick(() => {
                    this.fit();
                });

                // Set up resize observer for auto-fit
                this.setupResizeObserver(container);
            }

            // Handle window resize
            window.addEventListener('resize', () => {
                this.fit();
            });

            // Write welcome message
            this.terminal.writeln('\x1b[1;34m=== Wander Exec Terminal ===\x1b[0m');
            this.terminal.writeln('');
            this.terminal.writeln('Click \x1b[1;32mConnect\x1b[0m to start an exec session.');
            this.terminal.writeln('');
        },

        // Fit terminal to container
        fit() {
            if (this.fitAddon) {
                try {
                    this.fitAddon.fit();

                    // Debounce resize events to server to avoid flooding
                    if (this.connectionState === 'connected' && this.websocket) {
                        // Clear any pending resize
                        if (this.resizeTimeout) {
                            clearTimeout(this.resizeTimeout);
                        }

                        // Debounce: wait 100ms before sending resize
                        this.resizeTimeout = setTimeout(() => {
                            const dims = { cols: this.terminal.cols, rows: this.terminal.rows };
                            // Only send if dimensions actually changed
                            if (dims.cols !== this.lastSentDims.cols || dims.rows !== this.lastSentDims.rows) {
                                this.sendResize(dims);
                                this.lastSentDims = dims;
                            }
                        }, 100);
                    }
                } catch (e) {
                    console.warn('Failed to fit terminal:', e);
                }
            }
        },

        // Set up ResizeObserver for container
        setupResizeObserver(container) {
            if (typeof ResizeObserver !== 'undefined') {
                this.resizeObserver = new ResizeObserver(() => {
                    this.fit();
                });
                this.resizeObserver.observe(container);
            }
        },

        // Toggle connection state
        toggleConnection() {
            if (this.connectionState === 'connected') {
                this.disconnect();
            } else if (this.connectionState === 'disconnected' || this.connectionState === 'error') {
                this.connect();
            }
        },

        // Build WebSocket URL for exec
        buildWebSocketUrl() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const host = window.location.host;
            const cmd = encodeURIComponent(this.command || '/bin/sh');
            return `${protocol}//${host}/ui/allocations/${this.allocId}/exec/ws?task=${this.selectedTask}&command=${cmd}`;
        },

        // Connect to exec session via WebSocket
        connect() {
            if (this.connectionState === 'connecting' || this.connectionState === 'connected') {
                return;
            }

            this.connectionState = 'connecting';
            this.terminal.clear();
            this.terminal.writeln('\x1b[33mConnecting to exec session...\x1b[0m');
            this.terminal.writeln(`\x1b[90mTask: ${this.selectedTask}\x1b[0m`);
            this.terminal.writeln(`\x1b[90mCommand: ${this.command || '/bin/sh'}\x1b[0m`);
            this.terminal.writeln('');

            try {
                const url = this.buildWebSocketUrl();
                this.websocket = new WebSocket(url);

                this.websocket.onopen = () => {
                    this.connectionState = 'connected';
                    this.terminal.writeln('\x1b[32mConnected!\x1b[0m');
                    this.terminal.writeln('');

                    // Focus terminal
                    this.terminal.focus();

                    // Send initial resize with current dimensions
                    const initialDims = { cols: this.terminal.cols, rows: this.terminal.rows };
                    this.sendResize(initialDims);
                    this.lastSentDims = initialDims;
                };

                this.websocket.onmessage = (event) => {
                    // Handle incoming data from exec session
                    if (event.data instanceof Blob) {
                        // Binary data
                        event.data.text().then(text => {
                            this.terminal.write(text);
                        });
                    } else {
                        // Text data - could be JSON for control messages or raw output
                        try {
                            const msg = JSON.parse(event.data);
                            if (msg.type === 'output' && msg.data) {
                                this.terminal.write(msg.data);
                            } else if (msg.type === 'error') {
                                this.terminal.writeln(`\x1b[31mError: ${msg.message}\x1b[0m`);
                            } else if (msg.type === 'exit') {
                                this.terminal.writeln('');
                                this.terminal.writeln(`\x1b[33mSession ended (exit code: ${msg.code || 0})\x1b[0m`);
                                this.disconnect();
                            }
                        } catch (e) {
                            // Not JSON, treat as raw terminal output
                            this.terminal.write(event.data);
                        }
                    }
                };

                this.websocket.onerror = (error) => {
                    console.error('WebSocket error:', error);
                    this.terminal.writeln('');
                    this.terminal.writeln('\x1b[31mConnection error occurred.\x1b[0m');
                    this.connectionState = 'error';
                };

                this.websocket.onclose = (event) => {
                    if (this.connectionState === 'connected') {
                        this.terminal.writeln('');
                        this.terminal.writeln('\x1b[33mConnection closed.\x1b[0m');
                    }
                    this.connectionState = 'disconnected';
                    this.websocket = null;
                };

                // Handle terminal input - send to WebSocket
                this.terminal.onData(data => {
                    if (this.websocket && this.websocket.readyState === WebSocket.OPEN) {
                        // Send as JSON message with type 'input'
                        this.websocket.send(JSON.stringify({ type: 'input', data: data }));
                    }
                });

            } catch (e) {
                console.error('Failed to connect:', e);
                this.terminal.writeln(`\x1b[31mFailed to connect: ${e.message}\x1b[0m`);
                this.connectionState = 'error';
            }
        },

        // Send resize message to server
        sendResize(dims) {
            if (this.websocket && this.websocket.readyState === WebSocket.OPEN) {
                this.websocket.send(JSON.stringify({
                    type: 'resize',
                    cols: dims.cols,
                    rows: dims.rows
                }));
            }
        },

        // Disconnect from exec session
        disconnect() {
            // Clear any pending resize timeout
            if (this.resizeTimeout) {
                clearTimeout(this.resizeTimeout);
                this.resizeTimeout = null;
            }

            if (this.websocket) {
                // Send close message if possible
                if (this.websocket.readyState === WebSocket.OPEN) {
                    try {
                        this.websocket.send(JSON.stringify({ type: 'close' }));
                    } catch (e) {
                        // Ignore send errors during close
                    }
                }
                this.websocket.close();
                this.websocket = null;
            }
            this.connectionState = 'disconnected';
            this.lastSentDims = { cols: 0, rows: 0 };
        },

        // Cleanup on destroy
        destroy() {
            this.disconnect();

            if (this.resizeObserver) {
                this.resizeObserver.disconnect();
                this.resizeObserver = null;
            }

            if (this.terminal) {
                this.terminal.dispose();
                this.terminal = null;
            }
        }
    };
}

/**
 * GLOBAL ALPINE CONFIGURATION
 *
 * Make patterns available globally if Alpine is loaded.
 */
if (typeof window !== 'undefined') {
    window.alpinePatterns = {
        dropdown,
        modal,
        tabs,
        toggle,
        accordion,
        formValidation,
        validators,
        logViewer,
        execTerminal
    };
}
