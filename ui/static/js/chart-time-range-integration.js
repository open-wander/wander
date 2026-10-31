/**
 * Chart Time Range Integration
 *
 * Helper utilities for integrating TimeRangeSelector with chart components.
 * Provides event listeners and URL management for chart data updates.
 *
 * Usage:
 * ```js
 * import { setupChartTimeRange } from '/static/js/chart-time-range-integration.js';
 *
 * // In Alpine.js chart component
 * setupChartTimeRange(this, {
 *   baseURL: '/api/client/123/cpu',
 *   onRangeChange: (detail) => {
 *     console.log('Range changed:', detail);
 *   }
 * });
 * ```
 */

/**
 * Setup time range integration for a chart component
 *
 * This function should be called from within an Alpine.js component's init() method.
 * It listens for 'timerange-change' events and updates the chart's data URL.
 *
 * @param {Object} alpineContext - Alpine.js component context (this)
 * @param {Object} options - Configuration options
 * @param {string} options.baseURL - Base URL for chart data (without query params)
 * @param {string} options.eventName - Custom event name (default: 'timerange-change')
 * @param {Function} options.onRangeChange - Callback when range changes
 * @param {Function} options.buildURL - Custom URL builder function
 * @returns {Function} Cleanup function to remove event listener
 */
export function setupChartTimeRange(alpineContext, options = {}) {
    const {
        baseURL,
        eventName = 'timerange-change',
        onRangeChange = null,
        buildURL = null
    } = options;

    if (!baseURL) {
        console.error('setupChartTimeRange: baseURL is required');
        return () => {};
    }

    // Default URL builder - appends start/end as query params
    const defaultBuildURL = (base, startTime, endTime) => {
        const url = new URL(base, window.location.origin);
        url.searchParams.set('start', startTime.toISOString());
        url.searchParams.set('end', endTime.toISOString());
        return url.toString();
    };

    const urlBuilder = buildURL || defaultBuildURL;

    // Listen for time range changes
    const handleRangeChange = (event) => {
        const { startTime, endTime, range, label } = event.detail;

        // Build new data URL
        const newDataURL = urlBuilder(baseURL, startTime, endTime);

        // Update Alpine context
        if (alpineContext.dataURL !== undefined) {
            alpineContext.dataURL = newDataURL;
        }

        // Refresh chart if method exists
        if (typeof alpineContext.refreshChart === 'function') {
            alpineContext.refreshChart();
        } else if (typeof alpineContext.loadChart === 'function') {
            alpineContext.loadChart();
        }

        // Call custom callback
        if (onRangeChange) {
            onRangeChange(event.detail);
        }

        console.log(`Chart time range changed to ${label} (${range})`);
    };

    // Add event listener to document or component element
    const target = alpineContext.$el || document;
    target.addEventListener(eventName, handleRangeChange);

    // Return cleanup function
    return () => {
        target.removeEventListener(eventName, handleRangeChange);
    };
}

/**
 * Create a time-range-aware chart wrapper
 *
 * This extends an existing Alpine.js chart data object with time range support.
 *
 * @param {Object} chartData - Existing Alpine.js chart data object
 * @param {Object} options - Configuration options
 * @param {string} options.baseURL - Base URL for chart data
 * @param {string} options.eventName - Custom event name
 * @returns {Object} Extended chart data object
 */
export function createTimeRangeChart(chartData, options = {}) {
    const { baseURL, eventName = 'timerange-change' } = options;

    return {
        ...chartData,
        baseDataURL: baseURL || chartData.dataURL || '',
        timeRangeCleanup: null,

        // Override init to add time range support
        init() {
            // Call original init if it exists
            if (chartData.init) {
                chartData.init.call(this);
            }

            // Setup time range integration
            this.timeRangeCleanup = setupChartTimeRange(this, {
                baseURL: this.baseDataURL,
                eventName: eventName
            });
        },

        // Override destroy to cleanup listeners
        destroy() {
            // Call original destroy if it exists
            if (chartData.destroy) {
                chartData.destroy.call(this);
            }

            // Cleanup time range listener
            if (this.timeRangeCleanup) {
                this.timeRangeCleanup();
            }
        }
    };
}

/**
 * Build URL with time range parameters
 *
 * Helper function to construct URLs with start/end time query parameters.
 *
 * @param {string} baseURL - Base URL (can include existing query params)
 * @param {Date} startTime - Start time
 * @param {Date} endTime - End time
 * @param {Object} options - Additional options
 * @param {boolean} options.useUnixTimestamp - Use Unix timestamps instead of ISO (default: false)
 * @param {Object} options.additionalParams - Additional query parameters to include
 * @returns {string} Complete URL with time range parameters
 */
export function buildTimeRangeURL(baseURL, startTime, endTime, options = {}) {
    const {
        useUnixTimestamp = false,
        additionalParams = {}
    } = options;

    const url = new URL(baseURL, window.location.origin);

    // Add time range parameters
    if (useUnixTimestamp) {
        url.searchParams.set('start', Math.floor(startTime.getTime() / 1000));
        url.searchParams.set('end', Math.floor(endTime.getTime() / 1000));
    } else {
        url.searchParams.set('start', startTime.toISOString());
        url.searchParams.set('end', endTime.toISOString());
    }

    // Add additional parameters
    Object.entries(additionalParams).forEach(([key, value]) => {
        url.searchParams.set(key, value);
    });

    return url.toString();
}

/**
 * Dispatch time range change event
 *
 * Helper to manually trigger a time range change event.
 * Useful for programmatic time range updates.
 *
 * @param {string} range - Range ID
 * @param {string} label - Human-readable label
 * @param {Date} startTime - Start time
 * @param {Date} endTime - End time
 * @param {Object} options - Event options
 * @param {string} options.eventName - Custom event name
 * @param {HTMLElement} options.target - Event target element
 */
export function dispatchTimeRangeChange(range, label, startTime, endTime, options = {}) {
    const {
        eventName = 'timerange-change',
        target = document
    } = options;

    const event = new CustomEvent(eventName, {
        detail: {
            range,
            label,
            startTime,
            endTime,
            duration: range
        },
        bubbles: true,
        composed: true
    });

    target.dispatchEvent(event);
}

/**
 * Example: Multi-chart synchronization
 *
 * Setup multiple charts to respond to the same time range selector.
 *
 * @param {Array<Object>} charts - Array of Alpine.js chart contexts
 * @param {Object} options - Configuration options
 */
export function syncChartsWithTimeRange(charts, options = {}) {
    const { eventName = 'timerange-change' } = options;

    const cleanupFunctions = charts.map(chart => {
        if (!chart.baseDataURL) {
            console.warn('Chart missing baseDataURL, skipping time range sync');
            return () => {};
        }

        return setupChartTimeRange(chart, {
            baseURL: chart.baseDataURL,
            eventName: eventName
        });
    });

    // Return cleanup function for all charts
    return () => {
        cleanupFunctions.forEach(cleanup => cleanup());
    };
}

// Export for global access
if (typeof window !== 'undefined') {
    window.ChartTimeRangeIntegration = {
        setupChartTimeRange,
        createTimeRangeChart,
        buildTimeRangeURL,
        dispatchTimeRangeChange,
        syncChartsWithTimeRange
    };
}
