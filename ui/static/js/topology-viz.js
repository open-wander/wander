/**
 * Topology Visualization Component for Cluster View
 *
 * D3-based grid visualization showing:
 * - Datacenters as columns
 * - Nodes as cards/boxes within datacenters
 * - Color-coded status (ready=green, down=red, draining=yellow)
 * - Resource utilization bars
 * - Interactive tooltips
 * - Click to navigate to node detail
 * - Responsive sizing
 *
 * Dependencies:
 * - D3.js v7+
 * - chart-utils.js
 *
 * Data Format:
 * {
 *   datacenters: [
 *     {
 *       name: "dc1",
 *       region: "us-east",
 *       nodes: [
 *         {
 *           id: "node-123",
 *           name: "node-1",
 *           status: "ready",
 *           datacenter: "dc1",
 *           nodeClass: "compute",
 *           allocations: 5,
 *           cpuUsed: 1200,
 *           cpuTotal: 2400,
 *           memoryUsed: 4096000000,
 *           memoryTotal: 8192000000,
 *           lastHeartbeat: "2026-01-26T10:30:00Z"
 *         },
 *         ...
 *       ]
 *     },
 *     ...
 *   ]
 * }
 *
 * Usage:
 * ```js
 * import { TopologyViz } from '/static/js/topology-viz.js';
 *
 * const viz = new TopologyViz('topology-container', {
 *   nodeHeight: 120,
 *   nodeWidth: 180,
 *   padding: 16
 * });
 *
 * viz.render(data);
 *
 * // Update with new data
 * viz.update(newData);
 *
 * // Destroy when done
 * viz.destroy();
 * ```
 */

import {
    initChart,
    getChartColors,
    destroyChart
} from '/static/js/chart-utils.js';

/**
 * TopologyViz class for rendering cluster topology
 */
export class TopologyViz {
    /**
     * Create a new TopologyViz instance
     *
     * @param {string} containerId - ID of the container element
     * @param {Object} options - Visualization configuration options
     * @param {number} options.nodeHeight - Height of each node card (default: 120)
     * @param {number} options.nodeWidth - Width of each node card (default: 180)
     * @param {number} options.padding - Padding between elements (default: 16)
     * @param {number} options.dcHeaderHeight - Datacenter header height (default: 40)
     * @param {boolean} options.showTooltips - Enable tooltips (default: true)
     */
    constructor(containerId, options = {}) {
        this.containerId = containerId;
        this.data = null;
        this.chartInstance = null;

        this.options = {
            nodeHeight: 120,
            nodeWidth: 180,
            padding: 16,
            dcHeaderHeight: 40,
            showTooltips: true,
            ...options
        };

        // Bind render method to preserve context
        this.renderViz = this.renderViz.bind(this);
    }

    /**
     * Render the visualization with data
     *
     * @param {Object} data - Topology data
     */
    render(data) {
        this.data = this.processData(data);

        // Initialize chart with chart-utils
        this.chartInstance = initChart(
            this.containerId,
            this.renderViz,
            {
                margin: { top: 20, right: 20, bottom: 20, left: 20 },
                responsive: true,
                debounceMs: 300
            }
        );
    }

    /**
     * Update visualization with new data
     *
     * @param {Object} data - New topology data
     */
    update(data) {
        this.data = this.processData(data);
        if (this.chartInstance) {
            this.chartInstance.update();
        }
    }

    /**
     * Destroy visualization and cleanup resources
     */
    destroy() {
        if (this.chartInstance) {
            this.chartInstance.destroy();
            this.chartInstance = null;
        }
        this.hideTooltip();
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
        if (!data || !data.datacenters || !Array.isArray(data.datacenters)) {
            console.error('Invalid data format: expected { datacenters: [...] }');
            return { datacenters: [] };
        }

        return {
            datacenters: data.datacenters.map(dc => ({
                name: dc.name || 'Unknown',
                region: dc.region || '',
                nodes: (dc.nodes || []).map(node => ({
                    id: node.id || '',
                    name: node.name || 'Unknown',
                    status: node.status || 'unknown',
                    datacenter: node.datacenter || dc.name,
                    nodeClass: node.nodeClass || '',
                    allocations: node.allocations || 0,
                    cpuUsed: node.cpuUsed || 0,
                    cpuTotal: node.cpuTotal || 1,
                    memoryUsed: node.memoryUsed || 0,
                    memoryTotal: node.memoryTotal || 1,
                    lastHeartbeat: node.lastHeartbeat || ''
                }))
            }))
        };
    }

    /**
     * Render the visualization (called by chart-utils)
     *
     * @private
     * @param {HTMLElement} container - Container element
     * @param {Object} dimensions - Chart dimensions
     */
    renderViz(container, dimensions) {
        if (!this.data || this.data.datacenters.length === 0) {
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
            .attr('class', 'topology-viz-svg');

        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);

        // Calculate layout
        const layout = this.calculateLayout(this.data.datacenters, innerWidth, innerHeight);

        // Render datacenters
        this.data.datacenters.forEach((dc, dcIndex) => {
            this.renderDatacenter(g, dc, dcIndex, layout, colors);
        });

        // Add zoom/pan support if content exceeds viewport
        if (layout.totalHeight > innerHeight || layout.totalWidth > innerWidth) {
            this.addZoomPan(svg, g, innerWidth, innerHeight);
        }
    }

    /**
     * Calculate layout for datacenters and nodes
     *
     * @private
     * @param {Array} datacenters - Datacenter data
     * @param {number} viewportWidth - Available width
     * @param {number} viewportHeight - Available height
     * @returns {Object} Layout information
     */
    calculateLayout(datacenters, viewportWidth, viewportHeight) {
        const { nodeWidth, nodeHeight, padding, dcHeaderHeight } = this.options;

        // Calculate datacenter widths
        const dcLayouts = datacenters.map(dc => {
            const nodeCount = dc.nodes.length;
            const nodesPerRow = Math.max(1, Math.floor(viewportWidth / (nodeWidth + padding)));
            const rows = Math.ceil(nodeCount / nodesPerRow);
            const actualNodesPerRow = Math.min(nodeCount, nodesPerRow);

            return {
                width: Math.max(nodeWidth + 2 * padding, actualNodesPerRow * (nodeWidth + padding) + padding),
                height: dcHeaderHeight + rows * (nodeHeight + padding) + padding,
                nodesPerRow: actualNodesPerRow,
                rows: rows
            };
        });

        // Position datacenters vertically (stacked)
        let yOffset = 0;
        const dcPositions = dcLayouts.map((layout, index) => {
            const pos = {
                x: 0,
                y: yOffset,
                ...layout
            };
            yOffset += layout.height + padding;
            return pos;
        });

        return {
            datacenters: dcPositions,
            totalWidth: Math.max(...dcLayouts.map(l => l.width)),
            totalHeight: yOffset - padding
        };
    }

    /**
     * Render a datacenter column
     *
     * @private
     */
    renderDatacenter(g, dc, dcIndex, layout, colors) {
        const dcLayout = layout.datacenters[dcIndex];
        const { nodeWidth, nodeHeight, padding, dcHeaderHeight } = this.options;

        // Datacenter group
        const dcGroup = g.append('g')
            .attr('class', `datacenter datacenter-${dcIndex}`)
            .attr('transform', `translate(${dcLayout.x},${dcLayout.y})`);

        // Datacenter background
        dcGroup.append('rect')
            .attr('x', 0)
            .attr('y', 0)
            .attr('width', dcLayout.width)
            .attr('height', dcLayout.height)
            .attr('rx', 8)
            .style('fill', colors.backgroundAlt)
            .style('stroke', colors.axis)
            .style('stroke-width', 1);

        // Datacenter header
        dcGroup.append('rect')
            .attr('x', 0)
            .attr('y', 0)
            .attr('width', dcLayout.width)
            .attr('height', dcHeaderHeight)
            .attr('rx', 8)
            .style('fill', colors.grid);

        dcGroup.append('text')
            .attr('x', padding)
            .attr('y', dcHeaderHeight / 2)
            .attr('dy', '0.35em')
            .style('fill', colors.text)
            .style('font-size', '14px')
            .style('font-weight', '600')
            .text(dc.name);

        dcGroup.append('text')
            .attr('x', dcLayout.width - padding)
            .attr('y', dcHeaderHeight / 2)
            .attr('dy', '0.35em')
            .attr('text-anchor', 'end')
            .style('fill', colors.textMuted)
            .style('font-size', '12px')
            .text(`${dc.nodes.length} node${dc.nodes.length !== 1 ? 's' : ''}`);

        // Render nodes
        dc.nodes.forEach((node, nodeIndex) => {
            const row = Math.floor(nodeIndex / dcLayout.nodesPerRow);
            const col = nodeIndex % dcLayout.nodesPerRow;
            const x = padding + col * (nodeWidth + padding);
            const y = dcHeaderHeight + padding + row * (nodeHeight + padding);

            this.renderNode(dcGroup, node, x, y, colors);
        });
    }

    /**
     * Render a node card
     *
     * @private
     */
    renderNode(parent, node, x, y, colors) {
        const { nodeWidth, nodeHeight } = this.options;

        // Node group
        const nodeGroup = parent.append('g')
            .attr('class', `node node-${node.status}`)
            .attr('transform', `translate(${x},${y})`)
            .style('cursor', 'pointer')
            .on('click', () => {
                window.location.href = `/ui/clients/${node.id}`;
            });

        // Node background
        const statusColor = this.getStatusColor(node.status, colors);
        nodeGroup.append('rect')
            .attr('x', 0)
            .attr('y', 0)
            .attr('width', nodeWidth)
            .attr('height', nodeHeight)
            .attr('rx', 6)
            .style('fill', colors.background)
            .style('stroke', statusColor)
            .style('stroke-width', 2)
            .on('mouseenter', (event) => {
                d3.select(event.currentTarget)
                    .style('stroke-width', 3)
                    .style('filter', 'drop-shadow(0 4px 6px rgba(0,0,0,0.1))');
                if (this.options.showTooltips) {
                    this.showTooltip(event, node, colors);
                }
            })
            .on('mouseleave', (event) => {
                d3.select(event.currentTarget)
                    .style('stroke-width', 2)
                    .style('filter', 'none');
                this.hideTooltip();
            });

        // Status indicator
        nodeGroup.append('circle')
            .attr('cx', 8)
            .attr('cy', 8)
            .attr('r', 4)
            .style('fill', statusColor);

        // Node name
        nodeGroup.append('text')
            .attr('x', nodeWidth / 2)
            .attr('y', 20)
            .attr('text-anchor', 'middle')
            .style('fill', colors.text)
            .style('font-size', '12px')
            .style('font-weight', '600')
            .text(this.truncateText(node.name, 20));

        // Node ID (truncated)
        nodeGroup.append('text')
            .attr('x', nodeWidth / 2)
            .attr('y', 35)
            .attr('text-anchor', 'middle')
            .style('fill', colors.textMuted)
            .style('font-size', '10px')
            .style('font-family', 'monospace')
            .text(node.id.substring(0, 8));

        // Allocations count
        nodeGroup.append('text')
            .attr('x', 8)
            .attr('y', 50)
            .style('fill', colors.textMuted)
            .style('font-size', '10px')
            .text('Allocations');

        nodeGroup.append('text')
            .attr('x', nodeWidth - 8)
            .attr('y', 50)
            .attr('text-anchor', 'end')
            .style('fill', colors.text)
            .style('font-size', '11px')
            .style('font-weight', '600')
            .text(node.allocations);

        // CPU usage bar
        const cpuPercent = this.calculatePercentage(node.cpuUsed, node.cpuTotal);
        const cpuColor = this.getResourceColor(cpuPercent, colors);

        nodeGroup.append('text')
            .attr('x', 8)
            .attr('y', 68)
            .style('fill', colors.textMuted)
            .style('font-size', '10px')
            .text('CPU');

        nodeGroup.append('text')
            .attr('x', nodeWidth - 8)
            .attr('y', 68)
            .attr('text-anchor', 'end')
            .style('fill', colors.text)
            .style('font-size', '10px')
            .text(`${cpuPercent}%`);

        // CPU bar background
        nodeGroup.append('rect')
            .attr('x', 8)
            .attr('y', 72)
            .attr('width', nodeWidth - 16)
            .attr('height', 6)
            .attr('rx', 3)
            .style('fill', colors.grid);

        // CPU bar foreground
        nodeGroup.append('rect')
            .attr('x', 8)
            .attr('y', 72)
            .attr('width', 0)
            .attr('height', 6)
            .attr('rx', 3)
            .style('fill', cpuColor)
            .transition()
            .duration(800)
            .ease(d3.easeCubicOut)
            .attr('width', (nodeWidth - 16) * cpuPercent / 100);

        // Memory usage bar
        const memPercent = this.calculatePercentage(node.memoryUsed, node.memoryTotal);
        const memColor = this.getResourceColor(memPercent, colors);

        nodeGroup.append('text')
            .attr('x', 8)
            .attr('y', 90)
            .style('fill', colors.textMuted)
            .style('font-size', '10px')
            .text('Memory');

        nodeGroup.append('text')
            .attr('x', nodeWidth - 8)
            .attr('y', 90)
            .attr('text-anchor', 'end')
            .style('fill', colors.text)
            .style('font-size', '10px')
            .text(`${memPercent}%`);

        // Memory bar background
        nodeGroup.append('rect')
            .attr('x', 8)
            .attr('y', 94)
            .attr('width', nodeWidth - 16)
            .attr('height', 6)
            .attr('rx', 3)
            .style('fill', colors.grid);

        // Memory bar foreground
        nodeGroup.append('rect')
            .attr('x', 8)
            .attr('y', 94)
            .attr('width', 0)
            .attr('height', 6)
            .attr('rx', 3)
            .style('fill', memColor)
            .transition()
            .duration(800)
            .ease(d3.easeCubicOut)
            .attr('width', (nodeWidth - 16) * memPercent / 100);

        // Node class badge (if present)
        if (node.nodeClass) {
            nodeGroup.append('rect')
                .attr('x', 8)
                .attr('y', nodeHeight - 20)
                .attr('width', nodeWidth - 16)
                .attr('height', 14)
                .attr('rx', 3)
                .style('fill', colors.grid);

            nodeGroup.append('text')
                .attr('x', nodeWidth / 2)
                .attr('y', nodeHeight - 20 + 7)
                .attr('dy', '0.35em')
                .attr('text-anchor', 'middle')
                .style('fill', colors.text)
                .style('font-size', '9px')
                .text(this.truncateText(node.nodeClass, 18));
        }
    }

    /**
     * Get color for node status
     *
     * @private
     */
    getStatusColor(status, colors) {
        switch (status.toLowerCase()) {
            case 'ready':
                return colors.success;
            case 'down':
                return colors.error;
            case 'draining':
                return colors.warning;
            default:
                return colors.axis;
        }
    }

    /**
     * Get color for resource usage
     *
     * @private
     */
    getResourceColor(percentage, colors) {
        if (percentage >= 90) {
            return colors.error;
        } else if (percentage >= 75) {
            return colors.warning;
        }
        return colors.success;
    }

    /**
     * Calculate percentage
     *
     * @private
     */
    calculatePercentage(used, total) {
        if (total === 0) return 0;
        return Math.round((used / total) * 100);
    }

    /**
     * Truncate text to max length
     *
     * @private
     */
    truncateText(text, maxLength) {
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength - 1) + '…';
    }

    /**
     * Format bytes to human-readable string
     *
     * @private
     */
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    /**
     * Format time ago
     *
     * @private
     */
    formatTimeAgo(isoString) {
        if (!isoString) return 'Never';

        const date = new Date(isoString);
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return `${seconds}s ago`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        return `${Math.floor(seconds / 86400)}d ago`;
    }

    /**
     * Show tooltip
     *
     * @private
     */
    showTooltip(event, node, colors) {
        // Remove existing tooltip
        this.hideTooltip();

        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'topology-tooltip')
            .style('position', 'absolute')
            .style('pointer-events', 'none')
            .style('background', colors.background)
            .style('border', `1px solid ${colors.axis}`)
            .style('border-radius', '8px')
            .style('padding', '12px')
            .style('font-size', '12px')
            .style('box-shadow', '0 4px 12px rgba(0,0,0,0.15)')
            .style('z-index', '1000')
            .style('max-width', '250px')
            .style('opacity', 0);

        const cpuPercent = this.calculatePercentage(node.cpuUsed, node.cpuTotal);
        const memPercent = this.calculatePercentage(node.memoryUsed, node.memoryTotal);

        tooltip.html(`
            <div style="color: ${colors.text}; font-weight: 600; margin-bottom: 8px; border-bottom: 1px solid ${colors.grid}; padding-bottom: 6px;">
                ${node.name}
            </div>
            <div style="color: ${colors.textMuted}; font-size: 11px; margin-bottom: 8px;">
                ${node.id}
            </div>
            <div style="margin-bottom: 4px;">
                <span style="color: ${colors.textMuted};">Status:</span>
                <span style="color: ${this.getStatusColor(node.status, colors)}; font-weight: 600; margin-left: 4px;">
                    ${node.status}
                </span>
            </div>
            <div style="margin-bottom: 4px;">
                <span style="color: ${colors.textMuted};">Datacenter:</span>
                <span style="color: ${colors.text}; margin-left: 4px;">${node.datacenter}</span>
            </div>
            ${node.nodeClass ? `
                <div style="margin-bottom: 4px;">
                    <span style="color: ${colors.textMuted};">Class:</span>
                    <span style="color: ${colors.text}; margin-left: 4px;">${node.nodeClass}</span>
                </div>
            ` : ''}
            <div style="margin-bottom: 4px;">
                <span style="color: ${colors.textMuted};">Allocations:</span>
                <span style="color: ${colors.text}; font-weight: 600; margin-left: 4px;">${node.allocations}</span>
            </div>
            <div style="margin-bottom: 4px;">
                <span style="color: ${colors.textMuted};">CPU:</span>
                <span style="color: ${this.getResourceColor(cpuPercent, colors)}; font-weight: 600; margin-left: 4px;">
                    ${cpuPercent}%
                </span>
                <span style="color: ${colors.textMuted}; font-size: 10px; margin-left: 4px;">
                    (${node.cpuUsed} / ${node.cpuTotal} MHz)
                </span>
            </div>
            <div style="margin-bottom: 8px;">
                <span style="color: ${colors.textMuted};">Memory:</span>
                <span style="color: ${this.getResourceColor(memPercent, colors)}; font-weight: 600; margin-left: 4px;">
                    ${memPercent}%
                </span>
                <span style="color: ${colors.textMuted}; font-size: 10px; margin-left: 4px;">
                    (${this.formatBytes(node.memoryUsed)} / ${this.formatBytes(node.memoryTotal)})
                </span>
            </div>
            <div style="color: ${colors.textMuted}; font-size: 10px; padding-top: 6px; border-top: 1px solid ${colors.grid};">
                Last heartbeat: ${this.formatTimeAgo(node.lastHeartbeat)}
            </div>
        `);

        // Position tooltip
        const tooltipNode = tooltip.node();
        const rect = tooltipNode.getBoundingClientRect();
        const x = event.pageX + 15;
        const y = event.pageY - rect.height / 2;

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
        d3.selectAll('.topology-tooltip').remove();
    }

    /**
     * Add zoom and pan support
     *
     * @private
     */
    addZoomPan(svg, g, width, height) {
        const zoom = d3.zoom()
            .scaleExtent([0.5, 2])
            .on('zoom', (event) => {
                g.attr('transform', event.transform);
            });

        svg.call(zoom);
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
            .text('No datacenters or nodes available');
    }
}

/**
 * Fetch and render topology from URL
 *
 * @param {string} containerId - Container element ID
 * @param {string} dataUrl - URL to fetch data from
 * @param {Object} options - Visualization options
 * @returns {Promise<TopologyViz>} Visualization instance
 */
export async function renderTopologyFromUrl(containerId, dataUrl, options = {}) {
    try {
        const response = await fetch(dataUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const viz = new TopologyViz(containerId, options);
        viz.render(data);

        return viz;
    } catch (error) {
        console.error('Failed to load topology data:', error);
        throw error;
    }
}

// Export for global access
if (typeof window !== 'undefined') {
    window.TopologyViz = TopologyViz;
    window.renderTopologyFromUrl = renderTopologyFromUrl;
}
