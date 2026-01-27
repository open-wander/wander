/**
 * D3.js Chart Utilities for Wander UI
 *
 * This file provides helper functions for creating responsive D3.js charts
 * that integrate seamlessly with HTMX and Alpine.js.
 *
 * Features:
 * - Auto-initialization after HTMX swaps
 * - Responsive resize handling with ResizeObserver
 * - Common chart setup (margins, scales, axes)
 * - Chart lifecycle management (init, update, destroy)
 * - Dark mode support
 *
 * Dependencies:
 * - D3.js (must be loaded before this script)
 * - HTMX (for afterSwap event handling)
 *
 * Usage:
 *
 * 1. Basic chart initialization:
 * ```js
 * import { initChart } from '/static/js/chart-utils.js';
 *
 * function renderBarChart(container, dimensions) {
 *     const { width, height, margin } = dimensions;
 *
 *     const svg = d3.select(container)
 *         .append('svg')
 *         .attr('width', width)
 *         .attr('height', height);
 *
 *     const g = svg.append('g')
 *         .attr('transform', `translate(${margin.left},${margin.top})`);
 *
 *     // ... your D3 chart code
 * }
 *
 * initChart('my-chart', renderBarChart);
 * ```
 *
 * 2. Chart with data and update function:
 * ```js
 * const chartData = [10, 20, 30, 40, 50];
 *
 * initChart('my-chart', (container, dimensions) => {
 *     renderChart(container, dimensions, chartData);
 * });
 *
 * // Update chart when data changes
 * updateChart('my-chart', newData);
 * ```
 *
 * 3. Using common chart setup:
 * ```js
 * const config = createChartConfig('my-chart');
 * const svg = config.svg;
 * const g = config.g;
 * const { width, height } = config.dimensions;
 * ```
 */

// Registry to store chart instances and their render functions
const chartRegistry = new Map();

// Registry to store ResizeObservers for cleanup
const resizeObservers = new Map();

/**
 * Initialize a D3 chart with auto-resize support
 *
 * @param {string} chartId - The ID of the chart container element
 * @param {Function} renderFn - Function to render the chart (container, dimensions) => void
 * @param {Object} options - Configuration options
 * @param {Object} options.margin - Chart margins {top, right, bottom, left}
 * @param {boolean} options.responsive - Enable auto-resize (default: true)
 * @param {number} options.debounceMs - Debounce delay for resize events (default: 250ms)
 * @returns {Object} Chart instance with methods: update, destroy, getDimensions
 */
export function initChart(chartId, renderFn, options = {}) {
    const defaults = {
        margin: { top: 20, right: 20, bottom: 30, left: 40 },
        responsive: true,
        debounceMs: 250
    };

    const config = { ...defaults, ...options };

    // Get container element
    const container = document.getElementById(chartId);
    if (!container) {
        console.error(`Chart container not found: ${chartId}`);
        return null;
    }

    // Clear any existing chart
    clearChart(chartId);

    // Initial render
    const dimensions = getChartDimensions(chartId, config.margin);
    renderFn(container, dimensions);

    // Create chart instance
    const instance = {
        container,
        renderFn,
        config,
        destroyed: false,

        /**
         * Update the chart (re-render with current data)
         */
        update() {
            if (this.destroyed) return;
            clearChart(chartId, false); // Clear content but keep container
            const dims = getChartDimensions(chartId, config.margin);
            renderFn(container, dims);
        },

        /**
         * Destroy the chart and cleanup resources
         */
        destroy() {
            this.destroyed = true;
            clearChart(chartId);
            destroyChart(chartId);
        },

        /**
         * Get current chart dimensions
         */
        getDimensions() {
            return getChartDimensions(chartId, config.margin);
        }
    };

    // Store instance in registry
    chartRegistry.set(chartId, instance);

    // Setup resize observer if responsive
    if (config.responsive) {
        setupResizeObserver(chartId, instance, config.debounceMs);
    }

    return instance;
}

/**
 * Update an existing chart
 *
 * @param {string} chartId - The ID of the chart to update
 * @param {*} newData - Optional new data to pass to render function
 */
export function updateChart(chartId, newData) {
    const instance = chartRegistry.get(chartId);
    if (!instance) {
        console.warn(`Chart not found in registry: ${chartId}`);
        return;
    }

    instance.update(newData);
}

/**
 * Clear chart content
 *
 * @param {string} chartId - The ID of the chart to clear
 * @param {boolean} removeContainer - Whether to remove the container element (default: false)
 */
export function clearChart(chartId, removeContainer = false) {
    const container = document.getElementById(chartId);
    if (!container) return;

    // Remove all child elements
    while (container.firstChild) {
        container.removeChild(container.firstChild);
    }

    if (removeContainer) {
        container.remove();
    }
}

/**
 * Destroy a chart instance and cleanup resources
 *
 * @param {string} chartId - The ID of the chart to destroy
 */
export function destroyChart(chartId) {
    // Cleanup resize observer
    const observer = resizeObservers.get(chartId);
    if (observer) {
        observer.disconnect();
        resizeObservers.delete(chartId);
    }

    // Remove from registry
    chartRegistry.delete(chartId);
}

/**
 * Get chart dimensions from container
 *
 * @param {string} chartId - The ID of the chart container
 * @param {Object} margin - Chart margins {top, right, bottom, left}
 * @returns {Object} Dimensions object with width, height, innerWidth, innerHeight, margin
 */
export function getChartDimensions(chartId, margin = { top: 20, right: 20, bottom: 30, left: 40 }) {
    const container = document.getElementById(chartId);
    if (!container) {
        console.error(`Chart container not found: ${chartId}`);
        return null;
    }

    const rect = container.getBoundingClientRect();
    const width = rect.width || 600; // Fallback width
    const height = rect.height || 400; // Fallback height

    return {
        width,
        height,
        innerWidth: width - margin.left - margin.right,
        innerHeight: height - margin.top - margin.bottom,
        margin
    };
}

/**
 * Create a standard chart configuration with SVG and g element
 *
 * @param {string} chartId - The ID of the chart container
 * @param {Object} options - Configuration options
 * @param {Object} options.margin - Chart margins
 * @param {string} options.class - CSS class for SVG element
 * @returns {Object} Config object with svg, g, dimensions, container
 */
export function createChartConfig(chartId, options = {}) {
    const defaults = {
        margin: { top: 20, right: 20, bottom: 30, left: 40 },
        class: 'chart-svg'
    };

    const config = { ...defaults, ...options };
    const container = document.getElementById(chartId);
    if (!container) {
        console.error(`Chart container not found: ${chartId}`);
        return null;
    }

    const dimensions = getChartDimensions(chartId, config.margin);

    const svg = d3.select(container)
        .append('svg')
        .attr('width', dimensions.width)
        .attr('height', dimensions.height)
        .attr('class', config.class);

    const g = svg.append('g')
        .attr('transform', `translate(${dimensions.margin.left},${dimensions.margin.top})`);

    return {
        svg,
        g,
        dimensions,
        container,
        margin: config.margin
    };
}

/**
 * Setup ResizeObserver for responsive charts
 *
 * @private
 * @param {string} chartId - The ID of the chart container
 * @param {Object} instance - Chart instance
 * @param {number} debounceMs - Debounce delay in milliseconds
 */
function setupResizeObserver(chartId, instance, debounceMs) {
    const container = instance.container;
    let resizeTimer;

    const observer = new ResizeObserver(() => {
        // Debounce resize events
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => {
            if (!instance.destroyed) {
                instance.update();
            }
        }, debounceMs);
    });

    observer.observe(container);
    resizeObservers.set(chartId, observer);
}

/**
 * Check if dark mode is active
 *
 * @returns {boolean} True if dark mode is active
 */
export function isDarkMode() {
    return document.documentElement.classList.contains('dark');
}

/**
 * Get theme-aware colors for charts
 *
 * @returns {Object} Color palette for current theme
 */
export function getChartColors() {
    const dark = isDarkMode();

    return {
        // Text colors
        text: dark ? '#e5e5e5' : '#171717',
        textMuted: dark ? '#a3a3a3' : '#737373',

        // Axis colors
        axis: dark ? '#404040' : '#e5e5e5',
        grid: dark ? '#262626' : '#f5f5f5',

        // Primary colors for data visualization
        primary: dark ? '#60a5fa' : '#3b82f6',
        success: dark ? '#4ade80' : '#22c55e',
        warning: dark ? '#fbbf24' : '#f59e0b',
        error: dark ? '#f87171' : '#ef4444',
        info: dark ? '#38bdf8' : '#0ea5e9',

        // Data series colors (colorblind-friendly palette)
        series: dark
            ? ['#60a5fa', '#4ade80', '#fbbf24', '#f87171', '#a78bfa', '#38bdf8', '#fb923c', '#f472b6']
            : ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#0ea5e9', '#f97316', '#ec4899'],

        // Background
        background: dark ? '#171717' : '#ffffff',
        backgroundAlt: dark ? '#262626' : '#fafafa'
    };
}

/**
 * Create responsive scales for common chart types
 *
 * @param {Object} dimensions - Chart dimensions
 * @param {Array} domain - Data domain [min, max] or array of categories
 * @param {string} scaleType - Type of scale: 'linear', 'band', 'time', 'log'
 * @param {string} axis - Axis orientation: 'x' or 'y'
 * @returns {Function} D3 scale function
 */
export function createScale(dimensions, domain, scaleType = 'linear', axis = 'x') {
    const { innerWidth, innerHeight } = dimensions;
    const range = axis === 'x' ? [0, innerWidth] : [innerHeight, 0];

    switch (scaleType) {
        case 'linear':
            return d3.scaleLinear()
                .domain(domain)
                .range(range)
                .nice();

        case 'band':
            return d3.scaleBand()
                .domain(domain)
                .range(range)
                .padding(0.1);

        case 'time':
            return d3.scaleTime()
                .domain(domain)
                .range(range);

        case 'log':
            return d3.scaleLog()
                .domain(domain)
                .range(range)
                .nice();

        default:
            console.warn(`Unknown scale type: ${scaleType}, using linear`);
            return d3.scaleLinear()
                .domain(domain)
                .range(range)
                .nice();
    }
}

/**
 * Apply theme-aware axis styling
 *
 * @param {Selection} axis - D3 selection of axis element
 * @param {Object} colors - Color palette from getChartColors()
 */
export function styleAxis(axis, colors = getChartColors()) {
    axis.selectAll('path, line')
        .style('stroke', colors.axis)
        .style('stroke-width', 1);

    axis.selectAll('text')
        .style('fill', colors.text)
        .style('font-size', '12px');
}

/**
 * HTMX Integration: Auto-reinitialize charts after swaps
 *
 * This listens for HTMX afterSwap events and reinitializes any charts
 * that have the data-chart-init attribute with a render function name.
 *
 * For Alpine.js chart components (LineChart, GaugeChart, BarChart):
 * - These are automatically reinitialized by Alpine after HTMX swaps
 * - The chart-utils will handle the D3 rendering when Alpine calls the init functions
 *
 * Usage in Templ (manual chart initialization):
 * <div id="my-chart" data-chart-init="renderMyChart"></div>
 *
 * The render function should be available globally:
 * window.renderMyChart = function(container, dimensions) { ... }
 */
if (typeof document !== 'undefined') {
    document.addEventListener('htmx:afterSwap', (event) => {
        // Find all chart containers in the swapped content
        const swappedElement = event.detail.target;
        const charts = swappedElement.querySelectorAll('[data-chart-container]');

        charts.forEach(chart => {
            const chartId = chart.id;
            const initFnName = chart.dataset.chartInit;

            if (!chartId) {
                console.warn('Chart container missing ID attribute');
                return;
            }

            if (!initFnName) {
                // No auto-init function specified
                // Chart is likely managed by Alpine.js (x-data component)
                // Alpine will handle initialization automatically
                return;
            }

            // Get the render function from window
            const renderFn = window[initFnName];
            if (typeof renderFn !== 'function') {
                console.error(`Chart init function not found: ${initFnName}`);
                return;
            }

            // Initialize the chart
            initChart(chartId, renderFn);
        });

        // Reinitialize Alpine.js components in swapped content
        // This ensures chart components like LineChart, GaugeChart, BarChart work after HTMX swaps
        if (typeof window.Alpine !== 'undefined') {
            window.Alpine.initTree(swappedElement);
        }
    });

    // Cleanup charts when elements are removed
    document.addEventListener('htmx:beforeSwap', (event) => {
        const targetElement = event.detail.target;
        const charts = targetElement.querySelectorAll('[data-chart-container]');

        charts.forEach(chart => {
            const chartId = chart.id;
            if (chartId && chartRegistry.has(chartId)) {
                destroyChart(chartId);
            }
        });
    });
}

/**
 * Export utilities object for global access
 */
if (typeof window !== 'undefined') {
    window.chartUtils = {
        initChart,
        updateChart,
        clearChart,
        destroyChart,
        getChartDimensions,
        createChartConfig,
        isDarkMode,
        getChartColors,
        createScale,
        styleAxis
    };
}
