/**
 * Gauge Chart Component for Resource Utilization
 *
 * D3-based gauge chart with support for:
 * - Semi-circular and circular gauge styles
 * - Auto color-coding based on thresholds
 * - Animated transitions
 * - Numeric value display in center
 * - Responsive resizing
 * - Dark mode support
 *
 * Dependencies:
 * - D3.js v7+
 * - chart-utils.js
 *
 * Usage:
 * ```js
 * import { renderGaugeChart } from '/static/js/gauge-chart.js';
 *
 * const chart = renderGaugeChart('gauge-container', {
 *   value: 65.5,
 *   minValue: 0,
 *   maxValue: 100,
 *   label: 'CPU Usage',
 *   showValue: true,
 *   animateOnLoad: true,
 *   style: 'semicircle',
 *   thresholds: {
 *     good: 50,
 *     warning: 75,
 *     danger: 90
 *   }
 * });
 *
 * // Update value
 * chart.updateValue(75.2);
 *
 * // Destroy when done
 * chart.destroy();
 * ```
 */

import {
    initChart,
    getChartColors
} from '/static/js/chart-utils.js';

/**
 * Get color based on value and thresholds
 *
 * @param {number} value - Current value
 * @param {number} maxValue - Maximum value
 * @param {Object} thresholds - Threshold configuration
 * @param {Object} colors - Color palette
 * @returns {string} Color hex code
 */
function getGaugeColor(value, maxValue, thresholds, colors) {
    const percentage = (value / maxValue) * 100;

    if (percentage < thresholds.good) {
        return colors.success;
    } else if (percentage < thresholds.warning) {
        return colors.warning;
    } else if (percentage < thresholds.danger) {
        return '#fb923c'; // orange (between warning and error)
    } else {
        return colors.error;
    }
}

/**
 * Render a gauge chart
 *
 * @param {string} containerId - ID of the container element
 * @param {Object} options - Chart configuration options
 * @param {number} options.value - Current value
 * @param {number} options.minValue - Minimum value (default: 0)
 * @param {number} options.maxValue - Maximum value (default: 100)
 * @param {string} options.label - Label text
 * @param {boolean} options.showValue - Show numeric value (default: true)
 * @param {boolean} options.animateOnLoad - Animate on load (default: true)
 * @param {string} options.style - 'semicircle' or 'circle' (default: 'semicircle')
 * @param {Object} options.thresholds - Color thresholds
 * @returns {Object} Chart instance with methods: updateValue, destroy
 */
export function renderGaugeChart(containerId, options = {}) {
    const defaults = {
        value: 0,
        minValue: 0,
        maxValue: 100,
        label: '',
        showValue: true,
        animateOnLoad: true,
        style: 'semicircle',
        thresholds: {
            good: 50,
            warning: 75,
            danger: 90
        }
    };

    const config = { ...defaults, ...options };
    let currentValue = config.value;
    let chartInstance = null;

    // Render function for initChart
    function renderGauge(container, dimensions) {
        const { width, height } = dimensions;
        const colors = getChartColors();

        // Calculate dimensions based on style
        const isSemicircle = config.style === 'semicircle';
        const radius = Math.min(width, height * (isSemicircle ? 2 : 1)) / 2 - 20;
        const cx = width / 2;
        const cy = isSemicircle ? height - 20 : height / 2;

        // Arc angles
        const startAngle = isSemicircle ? -Math.PI / 2 : -Math.PI;
        const endAngle = isSemicircle ? Math.PI / 2 : Math.PI;
        const angleRange = endAngle - startAngle;

        // Create SVG
        const svg = d3.select(container)
            .append('svg')
            .attr('width', width)
            .attr('height', height)
            .attr('class', 'gauge-chart-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${cx},${cy})`);

        // Background arc (track)
        const backgroundArc = d3.arc()
            .innerRadius(radius * 0.75)
            .outerRadius(radius)
            .startAngle(startAngle)
            .endAngle(endAngle);

        g.append('path')
            .attr('class', 'gauge-background')
            .attr('d', backgroundArc)
            .style('fill', colors.grid)
            .style('opacity', 0.3);

        // Value arc (foreground)
        const valueArc = d3.arc()
            .innerRadius(radius * 0.75)
            .outerRadius(radius)
            .startAngle(startAngle)
            .cornerRadius(5);

        const valuePath = g.append('path')
            .attr('class', 'gauge-value')
            .style('fill', getGaugeColor(currentValue, config.maxValue, config.thresholds, colors));

        // Animate value arc
        const valueRange = config.maxValue - config.minValue;
        const targetAngle = startAngle + (currentValue - config.minValue) / valueRange * angleRange;

        if (config.animateOnLoad) {
            valuePath
                .datum({ endAngle: startAngle })
                .attr('d', valueArc)
                .transition()
                .duration(1000)
                .ease(d3.easeCubicOut)
                .attrTween('d', function(d) {
                    const interpolate = d3.interpolate(d.endAngle, targetAngle);
                    return function(t) {
                        d.endAngle = interpolate(t);
                        return valueArc(d);
                    };
                });
        } else {
            valuePath
                .datum({ endAngle: targetAngle })
                .attr('d', valueArc);
        }

        // Center text (value)
        if (config.showValue) {
            const percentage = ((currentValue - config.minValue) / valueRange * 100).toFixed(1);

            const textGroup = g.append('g')
                .attr('class', 'gauge-text')
                .attr('text-anchor', 'middle');

            textGroup.append('text')
                .attr('class', 'gauge-value-text')
                .attr('y', isSemicircle ? -10 : 0)
                .style('fill', colors.text)
                .style('font-size', `${Math.min(radius / 3, 36)}px`)
                .style('font-weight', '700')
                .text(config.animateOnLoad ? '0.0%' : `${percentage}%`)
                .transition()
                .duration(config.animateOnLoad ? 1000 : 0)
                .tween('text', function() {
                    const i = d3.interpolate(0, parseFloat(percentage));
                    return function(t) {
                        this.textContent = `${i(t).toFixed(1)}%`;
                    };
                });

            // Label below value
            if (config.label) {
                textGroup.append('text')
                    .attr('class', 'gauge-label')
                    .attr('y', isSemicircle ? 15 : 25)
                    .style('fill', colors.textMuted)
                    .style('font-size', `${Math.min(radius / 6, 14)}px`)
                    .style('font-weight', '500')
                    .text(config.label);
            }
        }

        // Store references for updates
        valuePath.node()._gaugeArc = valueArc;
        valuePath.node()._gaugeDatum = valuePath.datum();
    }

    // Initialize chart
    chartInstance = initChart(containerId, renderGauge, {
        margin: { top: 10, right: 10, bottom: 10, left: 10 },
        responsive: true
    });

    // Return chart instance with methods
    return {
        chartInstance,

        /**
         * Update gauge value with animation
         *
         * @param {number} newValue - New value
         * @param {boolean} animate - Animate transition (default: true)
         */
        updateValue(newValue, animate = true) {
            currentValue = newValue;
            config.value = newValue;

            const container = document.getElementById(containerId);
            if (!container) return;

            const svg = d3.select(container).select('svg');
            const colors = getChartColors();

            // Update value arc
            const valuePath = svg.select('.gauge-value');
            const arc = valuePath.node()?._gaugeArc;
            const datum = valuePath.node()?._gaugeDatum;

            if (arc && datum) {
                const isSemicircle = config.style === 'semicircle';
                const startAngle = isSemicircle ? -Math.PI / 2 : -Math.PI;
                const endAngle = isSemicircle ? Math.PI / 2 : Math.PI;
                const angleRange = endAngle - startAngle;
                const valueRange = config.maxValue - config.minValue;
                const targetAngle = startAngle + (newValue - config.minValue) / valueRange * angleRange;

                // Update color
                const newColor = getGaugeColor(newValue, config.maxValue, config.thresholds, colors);
                valuePath.style('fill', newColor);

                if (animate) {
                    valuePath
                        .transition()
                        .duration(750)
                        .ease(d3.easeCubicOut)
                        .attrTween('d', function() {
                            const interpolate = d3.interpolate(datum.endAngle, targetAngle);
                            return function(t) {
                                datum.endAngle = interpolate(t);
                                return arc(datum);
                            };
                        });
                } else {
                    datum.endAngle = targetAngle;
                    valuePath.attr('d', arc(datum));
                }
            }

            // Update text
            const valueText = svg.select('.gauge-value-text');
            if (valueText.size() > 0) {
                const valueRange = config.maxValue - config.minValue;
                const percentage = ((newValue - config.minValue) / valueRange * 100).toFixed(1);

                if (animate) {
                    const currentText = valueText.text();
                    const currentValue = parseFloat(currentText) || 0;

                    valueText
                        .transition()
                        .duration(750)
                        .tween('text', function() {
                            const i = d3.interpolate(currentValue, parseFloat(percentage));
                            return function(t) {
                                this.textContent = `${i(t).toFixed(1)}%`;
                            };
                        });
                } else {
                    valueText.text(`${percentage}%`);
                }
            }
        },

        /**
         * Destroy chart and cleanup resources
         */
        destroy() {
            if (chartInstance) {
                chartInstance.destroy();
                chartInstance = null;
            }
        }
    };
}

// Export for global access
if (typeof window !== 'undefined') {
    window.renderGaugeChart = renderGaugeChart;
}
