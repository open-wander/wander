/**
 * ANSI Escape Code Parser
 *
 * Converts ANSI escape sequences to HTML with CSS classes for terminal color rendering.
 * Designed for high-performance log streaming with proper error handling.
 *
 * Features:
 * - Standard colors (30-37, 40-47)
 * - Bright colors (90-97, 100-107)
 * - Text styles (bold, dim, italic, underline)
 * - Nested style handling
 * - Graceful handling of malformed sequences
 * - XSS protection via HTML escaping
 *
 * Usage:
 * ```js
 * const ansiParser = new AnsiParser();
 * const html = ansiParser.parse("\x1b[31mError:\x1b[0m Something went wrong");
 * // Returns: '<span class="ansi-fg-red">Error:</span> Something went wrong'
 * ```
 */

class AnsiParser {
    constructor() {
        // ANSI escape sequence regex: ESC [ ... m
        // Matches: \x1b[...m or \033[...m
        this.ansiRegex = /\x1b\[([0-9;]*)m/g;

        // Map ANSI codes to CSS classes
        this.styleMap = {
            // Reset
            0: 'reset',

            // Text styles
            1: 'bold',
            2: 'dim',
            3: 'italic',
            4: 'underline',

            // Foreground colors (30-37)
            30: 'fg-black',
            31: 'fg-red',
            32: 'fg-green',
            33: 'fg-yellow',
            34: 'fg-blue',
            35: 'fg-magenta',
            36: 'fg-cyan',
            37: 'fg-white',

            // Background colors (40-47)
            40: 'bg-black',
            41: 'bg-red',
            42: 'bg-green',
            43: 'bg-yellow',
            44: 'bg-blue',
            45: 'bg-magenta',
            46: 'bg-cyan',
            47: 'bg-white',

            // Bright foreground colors (90-97)
            90: 'fg-bright-black',
            91: 'fg-bright-red',
            92: 'fg-bright-green',
            93: 'fg-bright-yellow',
            94: 'fg-bright-blue',
            95: 'fg-bright-magenta',
            96: 'fg-bright-cyan',
            97: 'fg-bright-white',

            // Bright background colors (100-107)
            100: 'bg-bright-black',
            101: 'bg-bright-red',
            102: 'bg-bright-green',
            103: 'bg-bright-yellow',
            104: 'bg-bright-blue',
            105: 'bg-bright-magenta',
            106: 'bg-bright-cyan',
            107: 'bg-bright-white',
        };

        // Track current active styles
        this.currentStyles = new Set();
    }

    /**
     * Parse ANSI escape codes and convert to HTML
     * @param {string} text - Text with ANSI escape codes
     * @returns {string} HTML string with CSS classes
     */
    parse(text) {
        if (!text) return '';

        // Escape HTML to prevent XSS
        const escaped = this.escapeHtml(text);

        // Track styles and build HTML
        const segments = [];
        let lastIndex = 0;
        let match;

        // Reset state for each parse
        this.currentStyles = new Set();

        // Use a copy of the regex to avoid issues with global flag
        const regex = new RegExp(this.ansiRegex.source, this.ansiRegex.flags);

        while ((match = regex.exec(escaped)) !== null) {
            const beforeMatch = escaped.substring(lastIndex, match.index);

            // Add text before this ANSI code
            if (beforeMatch) {
                segments.push(this.wrapWithStyles(beforeMatch));
            }

            // Process ANSI codes
            const codes = match[1];
            this.processAnsiCodes(codes);

            lastIndex = regex.lastIndex;
        }

        // Add remaining text
        const remaining = escaped.substring(lastIndex);
        if (remaining) {
            segments.push(this.wrapWithStyles(remaining));
        }

        // Close any open spans
        if (this.currentStyles.size > 0) {
            segments.push('</span>'.repeat(this.getOpenSpanCount()));
        }

        return segments.join('');
    }

    /**
     * Process ANSI code sequence (e.g., "1;31" for bold red)
     * @param {string} codes - Semicolon-separated ANSI codes
     */
    processAnsiCodes(codes) {
        if (!codes) {
            // Empty code is treated as reset
            this.currentStyles.clear();
            return;
        }

        const codeArray = codes.split(';').map(c => parseInt(c, 10));

        for (const code of codeArray) {
            if (code === 0) {
                // Reset all styles
                this.currentStyles.clear();
            } else if (this.styleMap[code]) {
                // Add style to current set
                this.currentStyles.add(this.styleMap[code]);
            }
            // Ignore unknown codes
        }
    }

    /**
     * Wrap text with current style spans
     * @param {string} text - Text to wrap
     * @returns {string} HTML with style spans
     */
    wrapWithStyles(text) {
        if (this.currentStyles.size === 0) {
            return text;
        }

        // Build CSS classes from current styles
        const classes = Array.from(this.currentStyles)
            .map(s => `ansi-${s}`)
            .join(' ');

        return `<span class="${classes}">${text}</span>`;
    }

    /**
     * Count open span tags (for closing at end)
     * @returns {number} Number of open spans
     */
    getOpenSpanCount() {
        return this.currentStyles.size > 0 ? 1 : 0;
    }

    /**
     * Escape HTML special characters to prevent XSS
     * @param {string} text - Text to escape
     * @returns {string} Escaped text
     */
    escapeHtml(text) {
        const htmlEscapes = {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#39;'
        };

        return text.replace(/[&<>"']/g, char => htmlEscapes[char]);
    }

    /**
     * Strip all ANSI codes from text (useful for downloading logs)
     * @param {string} text - Text with ANSI codes
     * @returns {string} Plain text without ANSI codes
     */
    static strip(text) {
        if (!text) return '';
        return text.replace(/\x1b\[[0-9;]*m/g, '');
    }
}

// Make available globally
if (typeof window !== 'undefined') {
    window.AnsiParser = AnsiParser;
}

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AnsiParser;
}
