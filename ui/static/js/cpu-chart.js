/**
 * CPU Utilization Chart Component
 *
 * D3-based area chart specifically designed for CPU utilization metrics.
 *
 * Features:
 * - Time-series CPU percentage data (0-100%)
 * - Area chart with gradient fill
 * - Dynamic color thresholds:
 *   - Green: < 50%
 *   - Yellow: 50-75%
 *   - Orange: 75-90%
 *   - Red: >= 90%
 * - Interactive tooltips on hover
 * - Grid lines for readability
 * - Responsive resizing
 * - Real-time data updates
 * - Dark mode support
 *
 * Dependencies:
 * - D3.js v7+
 * - chart-utils.js
 *
 * Data Format:
 * {
 *   data: [
 *     { date: "2024-01-01T12:00:00Z", value: 45.5 },
 *     { date: "2024-01-01T12:01:00Z", value: 62.3 },
 *     ...
 *   ]
 * }
 *
 * Usage:
 * ```js
 * import { CPUChart } from '/static/js/cpu-chart.js';
 *
 * const chart = new CPUChart('chart-container', {
 *   showGrid: true,
 *   curveType: 'curveMonotoneX'
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
 * Get color for CPU utilization threshold
 *
 * @param {number} value - CPU percentage (0-100)
 * @param {boolean} isDark - Whether dark mode is active
 * @returns {string} Color hex code
 */
function getCPUColor(value, isDark = false) {
    if (value >= 90) {
        return isDark ? '#f87171' : '#ef4444'; // red
    } else if (value >= 75) {
        return isDark ? '#fb923c' : '#f97316'; // orange
    } else if (value >= 50) {
        return isDark ? '#fbbf24' : '#f59e0b'; // yellow
    } else {
        return isDark ? '#4ade80' : '#22c55e'; // green
    }
}

/**
 * CPUChart class for rendering CPU utilization area charts
 */
export class CPUChart {
    /**
     * Create a new CPUChart instance
     *
     * @param {string} containerId - ID of the container element
     * @param {Object} options - Chart configuration options
     * @param {boolean} options.showGrid - Show grid lines (default: true)
     * @param {string} options.curveType - D3 curve type (default: 'curveMonotoneX')
     * @param {number} options.marginTop - Top margin (default: 20)
     * @param {number} options.marginRight - Right margin (default: 20)
     * @param {number} options.marginBottom - Bottom margin (default: 40)
     * @param {number} options.marginLeft - Left margin (default: 50)
     * @param {string} options.xAxisLabel - X axis label
     * @param {string} options.yAxisLabel - Y axis label (default: "CPU %")
     * @param {Function} options.dateFormatter - Function to format X axis dates
     */
    constructor(containerId, options = {}) {
        this.containerId = containerId;
        this.data = null;
        this.chartInstance = null;

        this.options = {
            showGrid: true,
            curveType: 'curveMonotoneX',
            marginTop: 20,
            marginRight: 20,
            marginBottom: 40,
            marginLeft: 50,
            xAxisLabel: '',
            yAxisLabel: 'CPU %',
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
     * @returns {Object} Processed data
     */
    processData(data) {
        if (!data || !data.data || !Array.isArray(data.data)) {
            console.error('Invalid data format: expected { data: [...] }');
            return { data: [], currentValue: 0 };
        }

        // Parse dates and sort by date
        const parsedData = data.data
            .map(d => ({
                date: new Date(d.date),
                value: Math.max(0, Math.min(100, +d.value)) // Clamp to 0-100
            }))
            .filter(d => !isNaN(d.date) && !isNaN(d.value))
            .sort((a, b) => a.date - b.date);

        // Get current value (most recent data point)
        const currentValue = parsedData.length > 0
            ? parsedData[parsedData.length - 1].value
            : 0;

        return { data: parsedData, currentValue };
    }

    /**
     * Render the chart (called by chart-utils)
     *
     * @private
     * @param {HTMLElement} container - Container element
     * @param {Object} dimensions - Chart dimensions
     */
    renderChart(container, dimensions) {
        if (!this.data || this.data.data.length === 0) {
            this.renderEmptyState(container, dimensions);
            return;
        }

        const { width, height, innerWidth, innerHeight, margin } = dimensions;
        const colors = getChartColors();
        const isDark = document.documentElement.classList.contains('dark');

        // Get color based on current CPU value
        const cpuColor = getCPUColor(this.data.currentValue, isDark);

        // Create SVG
        const svg = d3.select(container)
            .append('svg')
            .attr('width', width)
            .attr('height', height)
            .attr('class', 'cpu-chart-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);

        // Create scales
        const xScale = createScale(
            dimensions,
            d3.extent(this.data.data, d => d.date),
            'time',
            'x'
        );

        // Y scale always 0-100% for CPU
        const yScale = createScale(
            dimensions,
            [0, 100],
            'linear',
            'y'
        );

        // Draw grid lines
        if (this.options.showGrid) {
            this.drawGrid(g, xScale, yScale, innerWidth, innerHeight, colors);
        }

        // Create line generator
        const curve = d3[this.options.curveType] || d3.curveMonotoneX;
        const line = d3.line()
            .x(d => xScale(d.date))
            .y(d => yScale(d.value))
            .curve(curve);

        // Create area generator
        const area = d3.area()
            .x(d => xScale(d.date))
            .y0(innerHeight)
            .y1(d => yScale(d.value))
            .curve(curve);

        // Create gradient based on CPU color
        this.createGradient(svg, 'cpu-gradient', cpuColor);

        // Draw area fill
        g.append('path')
            .datum(this.data.data)
            .attr('class', 'cpu-area')
            .attr('d', area)
            .style('fill', 'url(#cpu-gradient)')
            .style('opacity', 0.3);

        // Draw line
        g.append('path')
            .datum(this.data.data)
            .attr('class', 'cpu-line')
            .attr('d', line)
            .style('fill', 'none')
            .style('stroke', cpuColor)
            .style('stroke-width', 2)
            .style('stroke-linejoin', 'round')
            .style('stroke-linecap', 'round');

        // Draw data points (only show every nth point if there are many)
        const showEveryNth = Math.ceil(this.data.data.length / 20);
        const visiblePoints = this.data.data.filter((d, i) => i % showEveryNth === 0);

        g.selectAll('.cpu-dot')
            .data(visiblePoints)
            .join('circle')
            .attr('class', 'cpu-dot')
            .attr('cx', d => xScale(d.date))
            .attr('cy', d => yScale(d.value))
            .attr('r', 3)
            .style('fill', cpuColor)
            .style('stroke', colors.background)
            .style('stroke-width', 2)
            .style('cursor', 'pointer')
            .on('mouseenter', (event, d) => {
                this.showTooltip(event, d, colors, isDark);
            })
            .on('mouseleave', () => {
                this.hideTooltip();
            });

        // Add threshold lines
        this.drawThresholdLines(g, yScale, innerWidth, colors);

        // Draw axes
        const xAxis = d3.axisBottom(xScale)
            .ticks(Math.min(8, Math.floor(innerWidth / 100)))
            .tickFormat(this.options.dateFormatter);

        const yAxis = d3.axisLeft(yScale)
            .ticks(5)
            .tickFormat(d => `${d}%`);

        const xAxisGroup = g.append('g')
            .attr('class', 'x-axis')
            .attr('transform', `translate(0,${innerHeight})`)
            .call(xAxis);

        const yAxisGroup = g.append('g')
            .attr('class', 'y-axis')
            .call(yAxis);

        styleAxis(xAxisGroup, colors);
        styleAxis(yAxisGroup, colors);

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

        // Add current value indicator
        this.drawCurrentValueIndicator(g, innerWidth, this.data.currentValue, cpuColor, colors);
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
                    .ticks(5)
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
     * Draw threshold indicator lines
     *
     * @private
     */
    drawThresholdLines(g, yScale, width, colors) {
        const thresholds = [
            { value: 50, color: colors.warning, opacity: 0.15 },
            { value: 75, color: '#fb923c', opacity: 0.15 },
            { value: 90, color: colors.error, opacity: 0.2 }
        ];

        thresholds.forEach(threshold => {
            g.append('line')
                .attr('class', 'threshold-line')
                .attr('x1', 0)
                .attr('x2', width)
                .attr('y1', yScale(threshold.value))
                .attr('y2', yScale(threshold.value))
                .style('stroke', threshold.color)
                .style('stroke-width', 1)
                .style('stroke-dasharray', '4,4')
                .style('opacity', threshold.opacity);
        });
    }

    /**
     * Draw current value indicator
     *
     * @private
     */
    drawCurrentValueIndicator(g, width, value, color, colors) {
        const indicator = g.append('g')
            .attr('class', 'current-value-indicator')
            .attr('transform', `translate(${width}, -10)`);

        // Background rectangle
        indicator.append('rect')
            .attr('x', -50)
            .attr('y', 0)
            .attr('width', 50)
            .attr('height', 24)
            .attr('rx', 4)
            .style('fill', color)
            .style('opacity', 0.9);

        // Value text
        indicator.append('text')
            .attr('x', -25)
            .attr('y', 16)
            .style('text-anchor', 'middle')
            .style('fill', '#ffffff')
            .style('font-size', '12px')
            .style('font-weight', '600')
            .text(`${value.toFixed(1)}%`);
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
            .style('stop-opacity', 0);
    }

    /**
     * Show tooltip
     *
     * @private
     */
    showTooltip(event, data, colors, isDark) {
        // Remove existing tooltip
        this.hideTooltip();

        const cpuColor = getCPUColor(data.value, isDark);

        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'cpu-chart-tooltip')
            .style('position', 'absolute')
            .style('pointer-events', 'none')
            .style('background', colors.background)
            .style('border', `1px solid ${colors.axis}`)
            .style('border-radius', '6px')
            .style('padding', '8px 12px')
            .style('font-size', '12px')
            .style('box-shadow', '0 2px 8px rgba(0,0,0,0.15)')
            .style('z-index', '1000')
            .style('opacity', 0);

        tooltip.html(`
            <div style="color: ${colors.textMuted}; margin-bottom: 4px;">
                ${this.options.dateFormatter(data.date)}
            </div>
            <div style="color: ${cpuColor}; font-weight: 600;">
                CPU: ${data.value.toFixed(1)}%
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
        d3.selectAll('.cpu-chart-tooltip').remove();
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
            .text('No CPU data available');
    }
}

/**
 * Fetch and render CPU chart from URL
 *
 * @param {string} containerId - Container element ID
 * @param {string} dataUrl - URL to fetch data from
 * @param {Object} options - Chart options
 * @returns {Promise<CPUChart>} Chart instance
 */
export async function renderCPUChartFromUrl(containerId, dataUrl, options = {}) {
    try {
        const response = await fetch(dataUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const chart = new CPUChart(containerId, options);
        chart.render(data);

        return chart;
    } catch (error) {
        console.error('Failed to load CPU chart data:', error);
        throw error;
    }
}

// Export for global access
if (typeof window !== 'undefined') {
    window.CPUChart = CPUChart;
    window.renderCPUChartFromUrl = renderCPUChartFromUrl;
}
