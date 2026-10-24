/**
 * Memory Utilization Chart Component
 *
 * D3-based area chart for visualizing memory usage over time with:
 * - Time-series data on X axis
 * - Memory usage (bytes) on Y axis with human-readable formatting (KB, MB, GB)
 * - Area chart showing used vs allocated memory
 * - Color gradient based on utilization percentage
 * - Interactive tooltips showing used/allocated/percentage
 * - Grid lines for readability
 * - Responsive resizing
 * - Dark mode support
 * - Auto-update support for real-time data
 *
 * Dependencies:
 * - D3.js v7+
 * - chart-utils.js
 *
 * Data Format:
 * {
 *   data: [
 *     {
 *       timestamp: "2024-01-01T12:00:00Z",  // ISO 8601 timestamp
 *       used: 1073741824,                    // Used memory in bytes
 *       allocated: 2147483648                // Allocated memory in bytes
 *     },
 *     ...
 *   ]
 * }
 *
 * Usage:
 * ```js
 * import { MemoryChart } from '/static/js/memory-chart.js';
 *
 * const chart = new MemoryChart('chart-container', {
 *   showGrid: true,
 *   maxMemory: 4294967296, // 4GB in bytes
 *   marginLeft: 60
 * });
 *
 * chart.render(data);
 *
 * // Update with new data
 * chart.update(newData);
 *
 * // Destroy when done
 * chart.destroy();
 * ```
 */

import {
    initChart,
    getChartColors,
    createScale,
    styleAxis,
    getChartDimensions
} from '/static/js/chart-utils.js';

/**
 * Format bytes to human-readable string
 *
 * @param {number} bytes - Number of bytes
 * @param {number} decimals - Number of decimal places (default: 1)
 * @returns {Object} { value, unit, formatted }
 */
function formatBytes(bytes, decimals = 1) {
    if (bytes === 0) return { value: 0, unit: 'B', formatted: '0 B' };
    if (bytes < 0) return { value: 0, unit: 'B', formatted: '0 B' };

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];

    const i = Math.floor(Math.log(bytes) / Math.log(k));
    const value = parseFloat((bytes / Math.pow(k, i)).toFixed(dm));
    const unit = sizes[i];

    return {
        value,
        unit,
        formatted: `${value} ${unit}`
    };
}

/**
 * Get color based on memory utilization percentage
 *
 * @param {number} percentage - Utilization percentage (0-100)
 * @returns {string} Color hex code
 */
function getUtilizationColor(percentage) {
    if (percentage < 50) {
        return '#22c55e'; // green-500
    } else if (percentage < 75) {
        return '#f59e0b'; // yellow-500
    } else if (percentage < 90) {
        return '#f97316'; // orange-500
    } else {
        return '#ef4444'; // red-500
    }
}

/**
 * MemoryChart class for rendering memory utilization charts
 */
export class MemoryChart {
    /**
     * Create a new MemoryChart instance
     *
     * @param {string} containerId - ID of the container element
     * @param {Object} options - Chart configuration options
     * @param {boolean} options.showGrid - Show grid lines (default: true)
     * @param {number} options.maxMemory - Maximum memory in bytes for Y axis scaling
     * @param {number} options.marginTop - Top margin (default: 20)
     * @param {number} options.marginRight - Right margin (default: 20)
     * @param {number} options.marginBottom - Bottom margin (default: 40)
     * @param {number} options.marginLeft - Left margin (default: 60)
     * @param {string} options.xAxisLabel - X axis label
     * @param {string} options.yAxisLabel - Y axis label (default: "Memory")
     * @param {boolean} options.showAllocated - Show allocated memory line (default: true)
     * @param {Function} options.dateFormatter - Function to format X axis dates
     */
    constructor(containerId, options = {}) {
        this.containerId = containerId;
        this.data = null;
        this.chartInstance = null;

        this.options = {
            showGrid: true,
            maxMemory: null,
            marginTop: 20,
            marginRight: 20,
            marginBottom: 40,
            marginLeft: 60,
            xAxisLabel: '',
            yAxisLabel: 'Memory',
            showAllocated: true,
            dateFormatter: d3.timeFormat('%H:%M'),
            ...options
        };

        // Bind render method to preserve context
        this.renderChart = this.renderChart.bind(this);
    }

    /**
     * Render the chart with data
     *
     * @param {Object} data - Chart data
     */
    render(data) {
        this.data = this.processData(data);

        // Initialize chart with chart-utils
        this.chartInstance = initChart(
            this.containerId,
            this.renderChart,
            {
                margin: {
                    top: this.options.marginTop,
                    right: this.options.marginRight,
                    bottom: this.options.marginBottom,
                    left: this.options.marginLeft
                },
                responsive: true
            }
        );
    }

    /**
     * Update chart with new data
     *
     * @param {Object} data - New chart data
     */
    update(data) {
        this.data = this.processData(data);
        if (this.chartInstance) {
            this.chartInstance.update();
        }
    }

    /**
     * Destroy chart and cleanup resources
     */
    destroy() {
        if (this.chartInstance) {
            this.chartInstance.destroy();
            this.chartInstance = null;
        }
        this.data = null;
    }

    /**
     * Process and validate input data
     *
     * @private
     * @param {Object} data - Raw data
     * @returns {Array} Processed data points
     */
    processData(data) {
        if (!data || !data.data || !Array.isArray(data.data)) {
            console.error('Invalid data format: expected { data: [...] }');
            return [];
        }

        // Parse and validate data points
        const processedData = data.data
            .map(d => ({
                timestamp: new Date(d.timestamp),
                used: +d.used || 0,
                allocated: +d.allocated || 0,
                percentage: d.allocated > 0 ? (d.used / d.allocated) * 100 : 0
            }))
            .filter(d => !isNaN(d.timestamp))
            .sort((a, b) => a.timestamp - b.timestamp);

        return processedData;
    }

    /**
     * Render the chart (called by chart-utils)
     *
     * @private
     * @param {HTMLElement} container - Container element
     * @param {Object} dimensions - Chart dimensions
     */
    renderChart(container, dimensions) {
        if (!this.data || this.data.length === 0) {
            this.renderEmptyState(container, dimensions);
            return;
        }

        const { width, height, innerWidth, innerHeight, margin } = dimensions;
        const colors = getChartColors();

        // Create SVG
        const svg = d3.select(container)
            .append('svg')
            .attr('width', width)
            .attr('height', height)
            .attr('class', 'memory-chart-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);

        // Create scales
        const xScale = createScale(
            dimensions,
            d3.extent(this.data, d => d.timestamp),
            'time',
            'x'
        );

        // Y scale for memory (bytes)
        const maxUsed = d3.max(this.data, d => d.used);
        const maxAllocated = d3.max(this.data, d => d.allocated);
        const maxValue = this.options.maxMemory || Math.max(maxUsed, maxAllocated);

        const yScale = createScale(
            dimensions,
            [0, maxValue * 1.1], // Add 10% padding
            'linear',
            'y'
        );

        // Draw grid lines
        if (this.options.showGrid) {
            this.drawGrid(g, xScale, yScale, innerWidth, innerHeight, colors);
        }

        // Create area generator for used memory
        const areaUsed = d3.area()
            .x(d => xScale(d.timestamp))
            .y0(innerHeight)
            .y1(d => yScale(d.used))
            .curve(d3.curveMonotoneX);

        // Create line generator for allocated memory
        const lineAllocated = d3.line()
            .x(d => xScale(d.timestamp))
            .y(d => yScale(d.allocated))
            .curve(d3.curveMonotoneX);

        // Calculate average utilization for gradient color
        const avgUtilization = d3.mean(this.data, d => d.percentage);
        const gradientColor = getUtilizationColor(avgUtilization);

        // Create gradient for used memory area
        this.createGradient(svg, 'memory-used-gradient', gradientColor);

        // Draw used memory area
        g.append('path')
            .datum(this.data)
            .attr('class', 'memory-area-used')
            .attr('d', areaUsed)
            .style('fill', 'url(#memory-used-gradient)')
            .style('opacity', 0.6);

        // Draw used memory line
        g.append('path')
            .datum(this.data)
            .attr('class', 'memory-line-used')
            .attr('d', d3.line()
                .x(d => xScale(d.timestamp))
                .y(d => yScale(d.used))
                .curve(d3.curveMonotoneX)
            )
            .style('fill', 'none')
            .style('stroke', gradientColor)
            .style('stroke-width', 2)
            .style('stroke-linejoin', 'round')
            .style('stroke-linecap', 'round');

        // Draw allocated memory line if enabled
        if (this.options.showAllocated) {
            g.append('path')
                .datum(this.data)
                .attr('class', 'memory-line-allocated')
                .attr('d', lineAllocated)
                .style('fill', 'none')
                .style('stroke', colors.textMuted)
                .style('stroke-width', 1.5)
                .style('stroke-dasharray', '4,4')
                .style('stroke-linejoin', 'round')
                .style('stroke-linecap', 'round');
        }

        // Add interactive overlay for tooltips
        this.addInteractiveOverlay(g, xScale, yScale, innerWidth, innerHeight, colors);

        // Draw axes
        // Determine appropriate unit for Y axis based on max value
        const maxFormatted = formatBytes(maxValue);
        const yAxisFormatter = (bytes) => {
            const formatted = formatBytes(bytes, 0);
            // Use the same unit as max value for consistency
            const valueInMaxUnit = bytes / Math.pow(1024, ['B', 'KB', 'MB', 'GB', 'TB'].indexOf(maxFormatted.unit));
            return valueInMaxUnit.toFixed(0);
        };

        const xAxis = d3.axisBottom(xScale)
            .ticks(Math.min(8, Math.floor(innerWidth / 100)))
            .tickFormat(this.options.dateFormatter);

        const yAxis = d3.axisLeft(yScale)
            .ticks(6)
            .tickFormat(yAxisFormatter);

        const xAxisGroup = g.append('g')
            .attr('class', 'x-axis')
            .attr('transform', `translate(0,${innerHeight})`)
            .call(xAxis);

        const yAxisGroup = g.append('g')
            .attr('class', 'y-axis')
            .call(yAxis);

        styleAxis(xAxisGroup, colors);
        styleAxis(yAxisGroup, colors);

        // Add Y axis unit label
        g.append('text')
            .attr('class', 'y-axis-unit')
            .attr('x', -10)
            .attr('y', -5)
            .style('text-anchor', 'end')
            .style('fill', colors.textMuted)
            .style('font-size', '11px')
            .text(`(${maxFormatted.unit})`);

        // Add axis labels
        if (this.options.xAxisLabel) {
            g.append('text')
                .attr('class', 'x-axis-label')
                .attr('x', innerWidth / 2)
                .attr('y', innerHeight + margin.bottom - 5)
                .style('text-anchor', 'middle')
                .style('fill', colors.text)
                .style('font-size', '12px')
                .text(this.options.xAxisLabel);
        }

        if (this.options.yAxisLabel) {
            g.append('text')
                .attr('class', 'y-axis-label')
                .attr('transform', 'rotate(-90)')
                .attr('x', -innerHeight / 2)
                .attr('y', -margin.left + 15)
                .style('text-anchor', 'middle')
                .style('fill', colors.text)
                .style('font-size', '12px')
                .text(this.options.yAxisLabel);
        }

        // Add legend
        this.drawLegend(g, innerWidth, colors, gradientColor);
    }

    /**
     * Draw grid lines
     *
     * @private
     */
    drawGrid(g, xScale, yScale, width, height, colors) {
        // Horizontal grid lines
        g.append('g')
            .attr('class', 'grid grid-horizontal')
            .call(
                d3.axisLeft(yScale)
                    .ticks(6)
                    .tickSize(-width)
                    .tickFormat('')
            )
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line')
                .style('stroke', colors.grid)
                .style('stroke-opacity', 0.5)
            );

        // Vertical grid lines
        g.append('g')
            .attr('class', 'grid grid-vertical')
            .attr('transform', `translate(0,${height})`)
            .call(
                d3.axisBottom(xScale)
                    .ticks(8)
                    .tickSize(-height)
                    .tickFormat('')
            )
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line')
                .style('stroke', colors.grid)
                .style('stroke-opacity', 0.5)
            );
    }

    /**
     * Create gradient definition
     *
     * @private
     */
    createGradient(svg, id, color) {
        const gradient = svg.append('defs')
            .append('linearGradient')
            .attr('id', id)
            .attr('x1', '0%')
            .attr('x2', '0%')
            .attr('y1', '0%')
            .attr('y2', '100%');

        gradient.append('stop')
            .attr('offset', '0%')
            .style('stop-color', color)
            .style('stop-opacity', 0.8);

        gradient.append('stop')
            .attr('offset', '100%')
            .style('stop-color', color)
            .style('stop-opacity', 0.1);
    }

    /**
     * Add interactive overlay for tooltips
     *
     * @private
     */
    addInteractiveOverlay(g, xScale, yScale, width, height, colors) {
        const bisect = d3.bisector(d => d.timestamp).left;

        // Add invisible overlay for mouse events
        const overlay = g.append('rect')
            .attr('class', 'memory-chart-overlay')
            .attr('width', width)
            .attr('height', height)
            .style('fill', 'none')
            .style('pointer-events', 'all');

        // Focus line
        const focusLine = g.append('line')
            .attr('class', 'memory-focus-line')
            .style('stroke', colors.textMuted)
            .style('stroke-width', 1)
            .style('stroke-dasharray', '3,3')
            .style('opacity', 0)
            .attr('y1', 0)
            .attr('y2', height);

        // Focus circles
        const focusCircleUsed = g.append('circle')
            .attr('class', 'memory-focus-circle-used')
            .attr('r', 4)
            .style('opacity', 0);

        const focusCircleAllocated = g.append('circle')
            .attr('class', 'memory-focus-circle-allocated')
            .attr('r', 4)
            .style('opacity', 0);

        const self = this;

        overlay.on('mousemove', function(event) {
            const [mouseX] = d3.pointer(event);
            const x0 = xScale.invert(mouseX);
            const i = bisect(self.data, x0);

            if (i === 0 || i >= self.data.length) return;

            // Find closest data point
            const d0 = self.data[i - 1];
            const d1 = self.data[i];
            const d = x0 - d0.timestamp > d1.timestamp - x0 ? d1 : d0;

            // Update focus elements
            const x = xScale(d.timestamp);
            const yUsed = yScale(d.used);
            const yAllocated = yScale(d.allocated);

            focusLine
                .attr('x1', x)
                .attr('x2', x)
                .style('opacity', 1);

            const usedColor = getUtilizationColor(d.percentage);
            focusCircleUsed
                .attr('cx', x)
                .attr('cy', yUsed)
                .style('fill', usedColor)
                .style('stroke', colors.background)
                .style('stroke-width', 2)
                .style('opacity', 1);

            if (self.options.showAllocated) {
                focusCircleAllocated
                    .attr('cx', x)
                    .attr('cy', yAllocated)
                    .style('fill', colors.textMuted)
                    .style('stroke', colors.background)
                    .style('stroke-width', 2)
                    .style('opacity', 1);
            }

            self.showTooltip(event, d, usedColor, colors);
        });

        overlay.on('mouseleave', () => {
            focusLine.style('opacity', 0);
            focusCircleUsed.style('opacity', 0);
            focusCircleAllocated.style('opacity', 0);
            self.hideTooltip();
        });
    }

    /**
     * Draw legend
     *
     * @private
     */
    drawLegend(g, width, colors, usedColor) {
        const legend = g.append('g')
            .attr('class', 'legend')
            .attr('transform', `translate(${width - 150}, 0)`);

        // Used memory legend
        const usedLegend = legend.append('g')
            .attr('class', 'legend-item')
            .attr('transform', 'translate(0, 0)');

        usedLegend.append('rect')
            .attr('width', 12)
            .attr('height', 12)
            .attr('rx', 2)
            .style('fill', usedColor);

        usedLegend.append('text')
            .attr('x', 18)
            .attr('y', 10)
            .style('fill', colors.text)
            .style('font-size', '12px')
            .style('font-weight', '500')
            .text('Used');

        // Allocated memory legend
        if (this.options.showAllocated) {
            const allocatedLegend = legend.append('g')
                .attr('class', 'legend-item')
                .attr('transform', 'translate(0, 20)');

            allocatedLegend.append('line')
                .attr('x1', 0)
                .attr('x2', 12)
                .attr('y1', 6)
                .attr('y2', 6)
                .style('stroke', colors.textMuted)
                .style('stroke-width', 1.5)
                .style('stroke-dasharray', '4,4');

            allocatedLegend.append('text')
                .attr('x', 18)
                .attr('y', 10)
                .style('fill', colors.text)
                .style('font-size', '12px')
                .style('font-weight', '500')
                .text('Allocated');
        }
    }

    /**
     * Show tooltip
     *
     * @private
     */
    showTooltip(event, data, usedColor, colors) {
        // Remove existing tooltip
        this.hideTooltip();

        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'memory-chart-tooltip')
            .style('position', 'absolute')
            .style('pointer-events', 'none')
            .style('background', colors.background)
            .style('border', `1px solid ${colors.axis}`)
            .style('border-radius', '6px')
            .style('padding', '10px 12px')
            .style('font-size', '12px')
            .style('box-shadow', '0 2px 8px rgba(0,0,0,0.15)')
            .style('z-index', '1000')
            .style('opacity', 0);

        const usedFormatted = formatBytes(data.used);
        const allocatedFormatted = formatBytes(data.allocated);
        const percentage = data.percentage.toFixed(1);

        tooltip.html(`
            <div style="color: ${colors.textMuted}; margin-bottom: 6px; font-weight: 500;">
                ${this.options.dateFormatter(data.timestamp)}
            </div>
            <div style="color: ${usedColor}; margin-bottom: 3px;">
                <strong>Used:</strong> ${usedFormatted.formatted}
            </div>
            <div style="color: ${colors.textMuted}; margin-bottom: 3px;">
                <strong>Allocated:</strong> ${allocatedFormatted.formatted}
            </div>
            <div style="color: ${colors.text}; font-weight: 600; margin-top: 4px; padding-top: 4px; border-top: 1px solid ${colors.axis};">
                ${percentage}% utilized
            </div>
        `);

        // Position tooltip
        const tooltipNode = tooltip.node();
        const rect = tooltipNode.getBoundingClientRect();
        const x = event.pageX + 10;
        const y = event.pageY - rect.height - 10;

        tooltip
            .style('left', `${x}px`)
            .style('top', `${y}px`)
            .transition()
            .duration(200)
            .style('opacity', 1);
    }

    /**
     * Hide tooltip
     *
     * @private
     */
    hideTooltip() {
        d3.selectAll('.memory-chart-tooltip').remove();
    }

    /**
     * Render empty state
     *
     * @private
     */
    renderEmptyState(container, dimensions) {
        const { width, height } = dimensions;
        const colors = getChartColors();

        const svg = d3.select(container)
            .append('svg')
            .attr('width', width)
            .attr('height', height);

        const g = svg.append('g')
            .attr('transform', `translate(${width / 2},${height / 2})`);

        g.append('text')
            .attr('text-anchor', 'middle')
            .style('fill', colors.textMuted)
            .style('font-size', '14px')
            .text('No memory data available');
    }
}

/**
 * Fetch and render memory chart from URL
 *
 * @param {string} containerId - Container element ID
 * @param {string} dataUrl - URL to fetch data from
 * @param {Object} options - Chart options
 * @returns {Promise<MemoryChart>} Chart instance
 */
export async function renderMemoryChartFromUrl(containerId, dataUrl, options = {}) {
    try {
        const response = await fetch(dataUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const chart = new MemoryChart(containerId, options);
        chart.render(data);

        return chart;
    } catch (error) {
        console.error('Failed to load memory chart data:', error);
        throw error;
    }
}

// Export for global access
if (typeof window !== 'undefined') {
    window.MemoryChart = MemoryChart;
    window.renderMemoryChartFromUrl = renderMemoryChartFromUrl;
}
