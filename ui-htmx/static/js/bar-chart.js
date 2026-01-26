/**
 * Bar Chart Component for Comparing Values
 *
 * D3-based bar chart with support for:
 * - Vertical and horizontal orientations
 * - Single or multi-color bars
 * - Interactive hover tooltips
 * - Value labels on bars
 * - Grid lines
 * - Responsive resizing
 * - Dark mode support
 *
 * Dependencies:
 * - D3.js v7+
 * - chart-utils.js
 *
 * Data Format:
 * {
 *   data: [
 *     { label: "Category A", value: 150, color: "#3b82f6" },
 *     { label: "Category B", value: 220, color: "#22c55e" },
 *     ...
 *   ],
 *   maxValue: 250  // Optional
 * }
 *
 * Usage:
 * ```js
 * import { BarChart } from '/static/js/bar-chart.js';
 *
 * const chart = new BarChart('chart-container', {
 *   orientation: 'vertical',
 *   showValues: true,
 *   showGrid: true,
 *   colorMode: 'single',
 *   color: '#3b82f6'
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
    styleAxis
} from '/static/js/chart-utils.js';

/**
 * BarChart class for rendering bar charts
 */
export class BarChart {
    /**
     * Create a new BarChart instance
     *
     * @param {string} containerId - ID of the container element
     * @param {Object} options - Chart configuration options
     * @param {string} options.orientation - 'vertical' or 'horizontal' (default: 'vertical')
     * @param {boolean} options.showValues - Show value labels on bars (default: true)
     * @param {boolean} options.showGrid - Show grid lines (default: true)
     * @param {string} options.colorMode - 'single' or 'multi' (default: 'single')
     * @param {string} options.color - Single color for bars
     * @param {string} options.valueFormatter - D3 format string (default: ',')
     * @param {number} options.maxBarWidth - Maximum bar width in pixels (default: 50)
     * @param {number} options.barPadding - Bar padding 0-1 (default: 0.1)
     * @param {number} options.marginTop - Top margin (default: 20)
     * @param {number} options.marginRight - Right margin (default: 20)
     * @param {number} options.marginBottom - Bottom margin (default: 50)
     * @param {number} options.marginLeft - Left margin (default: 60)
     * @param {string} options.xAxisLabel - X axis label
     * @param {string} options.yAxisLabel - Y axis label
     */
    constructor(containerId, options = {}) {
        this.containerId = containerId;
        this.data = null;
        this.chartInstance = null;

        this.options = {
            orientation: 'vertical',
            showValues: true,
            showGrid: true,
            colorMode: 'single',
            color: '',
            valueFormatter: ',',
            maxBarWidth: 50,
            barPadding: 0.1,
            marginTop: 20,
            marginRight: 20,
            marginBottom: 50,
            marginLeft: 60,
            xAxisLabel: '',
            yAxisLabel: '',
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
            return { data: [], maxValue: 0 };
        }

        const colors = getChartColors();
        const defaultColor = this.options.color || colors.primary;

        // Process each data item
        const processedData = data.data.map((item, index) => {
            const value = +item.value || 0;
            let color = defaultColor;

            // Multi-color mode: use item color or auto-assign from palette
            if (this.options.colorMode === 'multi') {
                color = item.color || colors.series[index % colors.series.length];
            }

            return {
                label: item.label || `Item ${index + 1}`,
                value: value,
                color: color
            };
        });

        // Calculate max value
        const maxValue = data.maxValue || d3.max(processedData, d => d.value) || 100;

        return {
            data: processedData,
            maxValue: maxValue
        };
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
        const isVertical = this.options.orientation === 'vertical';

        // Create SVG
        const svg = d3.select(container)
            .append('svg')
            .attr('width', width)
            .attr('height', height)
            .attr('class', 'bar-chart-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);

        // Create scales
        let xScale, yScale, xAxis, yAxis;

        if (isVertical) {
            // Vertical bars: categories on X, values on Y
            xScale = createScale(
                dimensions,
                this.data.data.map(d => d.label),
                'band',
                'x'
            );

            // Limit bar width
            const bandwidth = Math.min(xScale.bandwidth(), this.options.maxBarWidth);
            xScale.bandwidth = () => bandwidth;

            yScale = createScale(
                dimensions,
                [0, this.data.maxValue * 1.1], // Add 10% padding
                'linear',
                'y'
            );

            xAxis = d3.axisBottom(xScale);
            yAxis = d3.axisLeft(yScale)
                .ticks(6)
                .tickFormat(d3.format(this.options.valueFormatter));
        } else {
            // Horizontal bars: values on X, categories on Y
            xScale = createScale(
                dimensions,
                [0, this.data.maxValue * 1.1],
                'linear',
                'x'
            );

            yScale = createScale(
                { innerWidth: innerHeight, innerHeight: innerWidth },
                this.data.data.map(d => d.label),
                'band',
                'x'
            );

            // Limit bar width
            const bandwidth = Math.min(yScale.bandwidth(), this.options.maxBarWidth);
            yScale.bandwidth = () => bandwidth;

            xAxis = d3.axisBottom(xScale)
                .ticks(6)
                .tickFormat(d3.format(this.options.valueFormatter));
            yAxis = d3.axisLeft(yScale);
        }

        // Draw grid lines
        if (this.options.showGrid) {
            this.drawGrid(g, xScale, yScale, innerWidth, innerHeight, colors, isVertical);
        }

        // Draw bars
        const bars = g.selectAll('.bar')
            .data(this.data.data)
            .join('g')
            .attr('class', 'bar');

        if (isVertical) {
            bars.append('rect')
                .attr('x', d => xScale(d.label) + (xScale.bandwidth() - Math.min(xScale.bandwidth(), this.options.maxBarWidth)) / 2)
                .attr('y', innerHeight)
                .attr('width', Math.min(xScale.bandwidth(), this.options.maxBarWidth))
                .attr('height', 0)
                .style('fill', d => d.color)
                .style('cursor', 'pointer')
                .on('mouseenter', (event, d) => {
                    this.showTooltip(event, d, colors);
                    d3.select(event.currentTarget)
                        .style('opacity', 0.8);
                })
                .on('mouseleave', (event) => {
                    this.hideTooltip();
                    d3.select(event.currentTarget)
                        .style('opacity', 1);
                })
                .transition()
                .duration(800)
                .ease(d3.easeCubicOut)
                .attr('y', d => yScale(d.value))
                .attr('height', d => innerHeight - yScale(d.value));

            // Value labels
            if (this.options.showValues) {
                bars.append('text')
                    .attr('x', d => xScale(d.label) + Math.min(xScale.bandwidth(), this.options.maxBarWidth) / 2)
                    .attr('y', innerHeight)
                    .attr('text-anchor', 'middle')
                    .style('fill', colors.text)
                    .style('font-size', '11px')
                    .style('font-weight', '600')
                    .style('opacity', 0)
                    .text(d => d3.format(this.options.valueFormatter)(d.value))
                    .transition()
                    .duration(800)
                    .ease(d3.easeCubicOut)
                    .attr('y', d => yScale(d.value) - 5)
                    .style('opacity', 1);
            }
        } else {
            bars.append('rect')
                .attr('x', 0)
                .attr('y', d => yScale(d.label) + (yScale.bandwidth() - Math.min(yScale.bandwidth(), this.options.maxBarWidth)) / 2)
                .attr('width', 0)
                .attr('height', Math.min(yScale.bandwidth(), this.options.maxBarWidth))
                .style('fill', d => d.color)
                .style('cursor', 'pointer')
                .on('mouseenter', (event, d) => {
                    this.showTooltip(event, d, colors);
                    d3.select(event.currentTarget)
                        .style('opacity', 0.8);
                })
                .on('mouseleave', (event) => {
                    this.hideTooltip();
                    d3.select(event.currentTarget)
                        .style('opacity', 1);
                })
                .transition()
                .duration(800)
                .ease(d3.easeCubicOut)
                .attr('width', d => xScale(d.value));

            // Value labels
            if (this.options.showValues) {
                bars.append('text')
                    .attr('x', 0)
                    .attr('y', d => yScale(d.label) + Math.min(yScale.bandwidth(), this.options.maxBarWidth) / 2 + 4)
                    .attr('text-anchor', 'start')
                    .style('fill', colors.text)
                    .style('font-size', '11px')
                    .style('font-weight', '600')
                    .style('opacity', 0)
                    .text(d => d3.format(this.options.valueFormatter)(d.value))
                    .transition()
                    .duration(800)
                    .ease(d3.easeCubicOut)
                    .attr('x', d => xScale(d.value) + 5)
                    .style('opacity', 1);
            }
        }

        // Draw axes
        const xAxisGroup = g.append('g')
            .attr('class', 'x-axis')
            .attr('transform', `translate(0,${innerHeight})`)
            .call(xAxis);

        const yAxisGroup = g.append('g')
            .attr('class', 'y-axis')
            .call(yAxis);

        styleAxis(xAxisGroup, colors);
        styleAxis(yAxisGroup, colors);

        // Rotate X axis labels for vertical orientation if they're too long
        if (isVertical && this.data.data.some(d => d.label.length > 10)) {
            xAxisGroup.selectAll('text')
                .attr('transform', 'rotate(-45)')
                .style('text-anchor', 'end')
                .attr('dx', '-0.5em')
                .attr('dy', '0.15em');
        }

        // Add axis labels
        if (this.options.xAxisLabel) {
            g.append('text')
                .attr('class', 'x-axis-label')
                .attr('x', innerWidth / 2)
                .attr('y', innerHeight + margin.bottom - 5)
                .style('text-anchor', 'middle')
                .style('fill', colors.text)
                .style('font-size', '12px')
                .style('font-weight', '500')
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
                .style('font-weight', '500')
                .text(this.options.yAxisLabel);
        }
    }

    /**
     * Draw grid lines
     *
     * @private
     */
    drawGrid(g, xScale, yScale, width, height, colors, isVertical) {
        if (isVertical) {
            // Horizontal grid lines for vertical bars
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
        } else {
            // Vertical grid lines for horizontal bars
            g.append('g')
                .attr('class', 'grid grid-vertical')
                .attr('transform', `translate(0,${height})`)
                .call(
                    d3.axisBottom(xScale)
                        .ticks(6)
                        .tickSize(-height)
                        .tickFormat('')
                )
                .call(g => g.select('.domain').remove())
                .call(g => g.selectAll('.tick line')
                    .style('stroke', colors.grid)
                    .style('stroke-opacity', 0.5)
                );
        }
    }

    /**
     * Show tooltip
     *
     * @private
     */
    showTooltip(event, data, colors) {
        // Remove existing tooltip
        this.hideTooltip();

        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'bar-chart-tooltip')
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
            <div style="color: ${colors.text}; font-weight: 600; margin-bottom: 4px;">
                ${data.label}
            </div>
            <div style="color: ${data.color}; font-weight: 600;">
                ${d3.format(this.options.valueFormatter)(data.value)}
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
        d3.selectAll('.bar-chart-tooltip').remove();
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
            .text('No data available');
    }
}

/**
 * Fetch and render bar chart from URL
 *
 * @param {string} containerId - Container element ID
 * @param {string} dataUrl - URL to fetch data from
 * @param {Object} options - Chart options
 * @returns {Promise<BarChart>} Chart instance
 */
export async function renderBarChartFromUrl(containerId, dataUrl, options = {}) {
    try {
        const response = await fetch(dataUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const chart = new BarChart(containerId, options);
        chart.render(data);

        return chart;
    } catch (error) {
        console.error('Failed to load chart data:', error);
        throw error;
    }
}

// Export for global access
if (typeof window !== 'undefined') {
    window.BarChart = BarChart;
    window.renderBarChartFromUrl = renderBarChartFromUrl;
}
