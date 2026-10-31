/**
 * Line Chart Component for Time-Series Data
 *
 * D3-based line chart with support for:
 * - Time-series data on X axis
 * - Multiple data series
 * - Interactive tooltips
 * - Gradient fills
 * - Legend
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
 *   series: [
 *     {
 *       name: "Series 1",
 *       color: "#3b82f6", // Optional, auto-assigned if not provided
 *       data: [
 *         { date: "2024-01-01", value: 100 },
 *         { date: "2024-01-02", value: 150 },
 *         ...
 *       ]
 *     },
 *     ...
 *   ]
 * }
 *
 * Usage:
 * ```js
 * import { LineChart } from '/static/js/line-chart.js';
 *
 * const chart = new LineChart('chart-container', {
 *   showLegend: true,
 *   showGrid: true,
 *   showGradient: true,
 *   curveType: 'curveMonotoneX',
 *   marginLeft: 50
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
 * LineChart class for rendering time-series line charts
 */
export class LineChart {
    /**
     * Create a new LineChart instance
     *
     * @param {string} containerId - ID of the container element
     * @param {Object} options - Chart configuration options
     * @param {boolean} options.showLegend - Show legend for multiple series (default: true)
     * @param {boolean} options.showGrid - Show grid lines (default: true)
     * @param {boolean} options.showGradient - Show gradient fill under lines (default: true)
     * @param {string} options.curveType - D3 curve type (default: 'curveMonotoneX')
     * @param {number} options.marginTop - Top margin (default: 20)
     * @param {number} options.marginRight - Right margin (default: 20)
     * @param {number} options.marginBottom - Bottom margin (default: 40)
     * @param {number} options.marginLeft - Left margin (default: 50)
     * @param {string} options.xAxisLabel - X axis label
     * @param {string} options.yAxisLabel - Y axis label
     * @param {Function} options.valueFormatter - Function to format Y axis values
     * @param {Function} options.dateFormatter - Function to format X axis dates
     */
    constructor(containerId, options = {}) {
        this.containerId = containerId;
        this.data = null;
        this.chartInstance = null;

        this.options = {
            showLegend: true,
            showGrid: true,
            showGradient: true,
            curveType: 'curveMonotoneX',
            marginTop: 20,
            marginRight: 20,
            marginBottom: 40,
            marginLeft: 50,
            xAxisLabel: '',
            yAxisLabel: '',
            valueFormatter: d3.format(','),
            dateFormatter: d3.timeFormat('%b %d'),
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
        if (!data || !data.series || !Array.isArray(data.series)) {
            console.error('Invalid data format: expected { series: [...] }');
            return { series: [] };
        }

        const colors = getChartColors();

        // Process each series
        const processedSeries = data.series.map((series, index) => {
            if (!series.data || !Array.isArray(series.data)) {
                console.warn(`Series ${index} has invalid data`);
                return { ...series, data: [] };
            }

            // Parse dates and sort by date
            const parsedData = series.data
                .map(d => ({
                    date: new Date(d.date),
                    value: +d.value
                }))
                .filter(d => !isNaN(d.date) && !isNaN(d.value))
                .sort((a, b) => a.date - b.date);

            return {
                name: series.name || `Series ${index + 1}`,
                color: series.color || colors.series[index % colors.series.length],
                data: parsedData
            };
        });

        return { series: processedSeries };
    }

    /**
     * Render the chart (called by chart-utils)
     *
     * @private
     * @param {HTMLElement} container - Container element
     * @param {Object} dimensions - Chart dimensions
     */
    renderChart(container, dimensions) {
        if (!this.data || this.data.series.length === 0) {
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
            .attr('class', 'line-chart-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);

        // Get all data points for domain calculation
        const allData = this.data.series.flatMap(s => s.data);
        if (allData.length === 0) {
            this.renderEmptyState(container, dimensions);
            return;
        }

        // Create scales
        const xScale = createScale(
            dimensions,
            d3.extent(allData, d => d.date),
            'time',
            'x'
        );

        const yExtent = d3.extent(allData, d => d.value);
        const yPadding = (yExtent[1] - yExtent[0]) * 0.1;
        const yScale = createScale(
            dimensions,
            [
                Math.max(0, yExtent[0] - yPadding),
                yExtent[1] + yPadding
            ],
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

        // Create area generator for gradient fill
        const area = d3.area()
            .x(d => xScale(d.date))
            .y0(innerHeight)
            .y1(d => yScale(d.value))
            .curve(curve);

        // Draw each series
        this.data.series.forEach((series, index) => {
            if (series.data.length === 0) return;

            const seriesGroup = g.append('g')
                .attr('class', `line-series line-series-${index}`);

            // Draw gradient fill
            if (this.options.showGradient) {
                this.createGradient(svg, `gradient-${index}`, series.color);

                seriesGroup.append('path')
                    .datum(series.data)
                    .attr('class', 'line-area')
                    .attr('d', area)
                    .style('fill', `url(#gradient-${index})`)
                    .style('opacity', 0.2);
            }

            // Draw line
            seriesGroup.append('path')
                .datum(series.data)
                .attr('class', 'line-path')
                .attr('d', line)
                .style('fill', 'none')
                .style('stroke', series.color)
                .style('stroke-width', 2)
                .style('stroke-linejoin', 'round')
                .style('stroke-linecap', 'round');

            // Draw data points
            seriesGroup.selectAll('.line-dot')
                .data(series.data)
                .join('circle')
                .attr('class', 'line-dot')
                .attr('cx', d => xScale(d.date))
                .attr('cy', d => yScale(d.value))
                .attr('r', 3)
                .style('fill', series.color)
                .style('stroke', colors.background)
                .style('stroke-width', 2)
                .style('cursor', 'pointer')
                .on('mouseenter', (event, d) => {
                    this.showTooltip(event, d, series, colors);
                })
                .on('mouseleave', () => {
                    this.hideTooltip();
                });
        });

        // Draw axes
        const xAxis = d3.axisBottom(xScale)
            .ticks(Math.min(8, Math.floor(innerWidth / 100)))
            .tickFormat(this.options.dateFormatter);

        const yAxis = d3.axisLeft(yScale)
            .ticks(6)
            .tickFormat(this.options.valueFormatter);

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

        // Draw legend
        if (this.options.showLegend && this.data.series.length > 1) {
            this.drawLegend(g, innerWidth, colors);
        }
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
            .style('stop-opacity', 0.6);

        gradient.append('stop')
            .attr('offset', '100%')
            .style('stop-color', color)
            .style('stop-opacity', 0);
    }

    /**
     * Draw legend
     *
     * @private
     */
    drawLegend(g, width, colors) {
        const legend = g.append('g')
            .attr('class', 'legend')
            .attr('transform', `translate(${width - 120}, 0)`);

        this.data.series.forEach((series, index) => {
            const legendRow = legend.append('g')
                .attr('class', 'legend-item')
                .attr('transform', `translate(0, ${index * 20})`)
                .style('cursor', 'pointer');

            legendRow.append('rect')
                .attr('width', 12)
                .attr('height', 12)
                .attr('rx', 2)
                .style('fill', series.color);

            legendRow.append('text')
                .attr('x', 18)
                .attr('y', 10)
                .style('fill', colors.text)
                .style('font-size', '12px')
                .style('font-weight', '500')
                .text(series.name);
        });
    }

    /**
     * Show tooltip
     *
     * @private
     */
    showTooltip(event, data, series, colors) {
        // Remove existing tooltip
        this.hideTooltip();

        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'line-chart-tooltip')
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
            <div style="color: ${series.color}; font-weight: 600;">
                ${series.name}: ${this.options.valueFormatter(data.value)}
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
        d3.selectAll('.line-chart-tooltip').remove();
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
 * Fetch and render line chart from URL
 *
 * @param {string} containerId - Container element ID
 * @param {string} dataUrl - URL to fetch data from
 * @param {Object} options - Chart options
 * @returns {Promise<LineChart>} Chart instance
 */
export async function renderLineChartFromUrl(containerId, dataUrl, options = {}) {
    try {
        const response = await fetch(dataUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const chart = new LineChart(containerId, options);
        chart.render(data);

        return chart;
    } catch (error) {
        console.error('Failed to load chart data:', error);
        throw error;
    }
}

// Export for global access
if (typeof window !== 'undefined') {
    window.LineChart = LineChart;
    window.renderLineChartFromUrl = renderLineChartFromUrl;
}
