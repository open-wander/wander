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
        validators
    };
}
