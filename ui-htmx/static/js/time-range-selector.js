/**
 * Time Range Selector Utilities
 *
 * Helper functions for time range calculations and formatting.
 * Used by the TimeRangeSelector component for advanced time range operations.
 *
 * Dependencies: None (vanilla JavaScript)
 *
 * Usage:
 * ```js
 * import { parseDuration, formatTimeRange, calculateTimeRange } from '/static/js/time-range-selector.js';
 *
 * const range = calculateTimeRange('1h');
 * console.log(range); // { start: Date, end: Date }
 *
 * const formatted = formatTimeRange(range.start, range.end);
 * console.log(formatted); // "Jan 26, 2:00 PM - 3:00 PM"
 * ```
 */

/**
 * Parse duration string to milliseconds
 *
 * @param {string} duration - Duration string (e.g., "5m", "1h", "24h", "7d")
 * @returns {number} Duration in milliseconds
 */
export function parseDuration(duration) {
    const match = duration.match(/^(\d+)([mhdw])$/);

    if (!match) {
        console.error('Invalid duration format:', duration);
        return 3600000; // Default to 1 hour
    }

    const value = parseInt(match[1]);
    const unit = match[2];

    const multipliers = {
        'm': 60 * 1000,           // minutes
        'h': 60 * 60 * 1000,      // hours
        'd': 24 * 60 * 60 * 1000, // days
        'w': 7 * 24 * 60 * 60 * 1000 // weeks
    };

    return value * (multipliers[unit] || 3600000);
}

/**
 * Calculate time range from duration
 *
 * @param {string} duration - Duration string (e.g., "5m", "1h", "24h")
 * @param {Date} [endTime] - End time (defaults to now)
 * @returns {Object} { start: Date, end: Date }
 */
export function calculateTimeRange(duration, endTime = null) {
    const end = endTime || new Date();
    const durationMs = parseDuration(duration);
    const start = new Date(end.getTime() - durationMs);

    return { start, end };
}

/**
 * Format time range for display
 *
 * @param {Date} startTime - Start date/time
 * @param {Date} endTime - End date/time
 * @param {Object} options - Formatting options
 * @param {boolean} options.short - Use short format (default: false)
 * @param {boolean} options.includeDate - Include date in format (default: auto)
 * @returns {string} Formatted time range string
 */
export function formatTimeRange(startTime, endTime, options = {}) {
    const { short = false, includeDate = null } = options;

    // Auto-determine if we should include date
    const shouldIncludeDate = includeDate !== null
        ? includeDate
        : !isSameDay(startTime, endTime);

    const dateOptions = {
        month: short ? 'short' : 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
    };

    if (!shouldIncludeDate) {
        delete dateOptions.month;
        delete dateOptions.day;
    }

    const startStr = startTime.toLocaleString('en-US', dateOptions);
    const endStr = endTime.toLocaleString('en-US', dateOptions);

    return `${startStr} - ${endStr}`;
}

/**
 * Check if two dates are on the same day
 *
 * @param {Date} date1 - First date
 * @param {Date} date2 - Second date
 * @returns {boolean} True if same day
 */
function isSameDay(date1, date2) {
    return date1.getFullYear() === date2.getFullYear() &&
           date1.getMonth() === date2.getMonth() &&
           date1.getDate() === date2.getDate();
}

/**
 * Format duration to human-readable string
 *
 * @param {string} duration - Duration string (e.g., "5m", "1h", "24h")
 * @returns {string} Human-readable duration (e.g., "5 minutes", "1 hour")
 */
export function formatDuration(duration) {
    const match = duration.match(/^(\d+)([mhdw])$/);

    if (!match) {
        return duration;
    }

    const value = parseInt(match[1]);
    const unit = match[2];

    const units = {
        'm': value === 1 ? 'minute' : 'minutes',
        'h': value === 1 ? 'hour' : 'hours',
        'd': value === 1 ? 'day' : 'days',
        'w': value === 1 ? 'week' : 'weeks'
    };

    return `${value} ${units[unit] || ''}`;
}

/**
 * Convert time range to URL query parameters
 *
 * @param {Date} startTime - Start date/time
 * @param {Date} endTime - End date/time
 * @param {boolean} useISO - Use ISO 8601 format (default: true)
 * @returns {Object} Query parameters object
 */
export function timeRangeToParams(startTime, endTime, useISO = true) {
    if (useISO) {
        return {
            start: startTime.toISOString(),
            end: endTime.toISOString()
        };
    } else {
        return {
            start: Math.floor(startTime.getTime() / 1000), // Unix timestamp
            end: Math.floor(endTime.getTime() / 1000)
        };
    }
}

/**
 * Parse URL query parameters to time range
 *
 * @param {URLSearchParams|Object} params - URL parameters
 * @returns {Object|null} { start: Date, end: Date } or null if invalid
 */
export function paramsToTimeRange(params) {
    const startParam = params.get ? params.get('start') : params.start;
    const endParam = params.get ? params.get('end') : params.end;

    if (!startParam || !endParam) {
        return null;
    }

    try {
        let start, end;

        // Try parsing as ISO 8601 string
        if (isNaN(startParam)) {
            start = new Date(startParam);
            end = new Date(endParam);
        } else {
            // Parse as Unix timestamp
            start = new Date(parseInt(startParam) * 1000);
            end = new Date(parseInt(endParam) * 1000);
        }

        if (isNaN(start.getTime()) || isNaN(end.getTime())) {
            return null;
        }

        return { start, end };
    } catch (error) {
        console.error('Failed to parse time range params:', error);
        return null;
    }
}

/**
 * Round time to nearest interval
 *
 * @param {Date} date - Date to round
 * @param {number} intervalMs - Interval in milliseconds
 * @param {string} direction - 'floor', 'ceil', or 'round' (default: 'round')
 * @returns {Date} Rounded date
 */
export function roundTime(date, intervalMs, direction = 'round') {
    const time = date.getTime();

    let rounded;
    switch (direction) {
        case 'floor':
            rounded = Math.floor(time / intervalMs) * intervalMs;
            break;
        case 'ceil':
            rounded = Math.ceil(time / intervalMs) * intervalMs;
            break;
        case 'round':
        default:
            rounded = Math.round(time / intervalMs) * intervalMs;
            break;
    }

    return new Date(rounded);
}

/**
 * Get preset time ranges
 *
 * @returns {Array<Object>} Array of preset time range options
 */
export function getPresetRanges() {
    return [
        { id: '5m', label: '5 minutes', duration: '5m' },
        { id: '15m', label: '15 minutes', duration: '15m' },
        { id: '30m', label: '30 minutes', duration: '30m' },
        { id: '1h', label: '1 hour', duration: '1h' },
        { id: '3h', label: '3 hours', duration: '3h' },
        { id: '6h', label: '6 hours', duration: '6h' },
        { id: '12h', label: '12 hours', duration: '12h' },
        { id: '24h', label: '24 hours', duration: '24h' },
        { id: '7d', label: '7 days', duration: '7d' },
        { id: '30d', label: '30 days', duration: '30d' }
    ];
}

/**
 * Validate time range
 *
 * @param {Date} startTime - Start date/time
 * @param {Date} endTime - End date/time
 * @param {Object} options - Validation options
 * @param {Date} options.minStart - Minimum allowed start time
 * @param {Date} options.maxEnd - Maximum allowed end time (default: now)
 * @param {number} options.maxDuration - Maximum duration in milliseconds
 * @returns {Object} { valid: boolean, error: string }
 */
export function validateTimeRange(startTime, endTime, options = {}) {
    const {
        minStart = null,
        maxEnd = new Date(),
        maxDuration = null
    } = options;

    if (!(startTime instanceof Date) || !(endTime instanceof Date)) {
        return { valid: false, error: 'Invalid date objects' };
    }

    if (isNaN(startTime.getTime()) || isNaN(endTime.getTime())) {
        return { valid: false, error: 'Invalid date values' };
    }

    if (startTime >= endTime) {
        return { valid: false, error: 'Start time must be before end time' };
    }

    if (minStart && startTime < minStart) {
        return { valid: false, error: `Start time cannot be before ${minStart.toLocaleString()}` };
    }

    if (endTime > maxEnd) {
        return { valid: false, error: `End time cannot be after ${maxEnd.toLocaleString()}` };
    }

    if (maxDuration) {
        const duration = endTime.getTime() - startTime.getTime();
        if (duration > maxDuration) {
            return { valid: false, error: `Duration cannot exceed ${formatDuration(maxDuration)}` };
        }
    }

    return { valid: true };
}

// Export for global access
if (typeof window !== 'undefined') {
    window.TimeRangeUtils = {
        parseDuration,
        calculateTimeRange,
        formatTimeRange,
        formatDuration,
        timeRangeToParams,
        paramsToTimeRange,
        roundTime,
        getPresetRanges,
        validateTimeRange
    };
}
