/**
 * CodeMirror 6 Bundle for Wander UI
 *
 * This is a minimal CodeMirror 6 implementation for viewing/editing job definitions.
 * For production, consider using a proper bundler to create a smaller optimized bundle.
 *
 * Features:
 * - Basic editor with syntax highlighting
 * - Line numbers with gutter
 * - Read-only mode toggle
 * - HCL and JSON language support (basic highlighting)
 * - Bracket matching (highlights matching brackets)
 * - Tab key support
 * - Ctrl/Cmd+S save shortcut
 * - Light and dark themes
 */

// Since CodeMirror 6 is designed as ES modules, we'll create a lightweight
// syntax highlighting solution using a custom implementation for HCL/JSON
// that integrates well with Alpine.js

(function(window) {
  'use strict';

  // Token types for syntax highlighting
  const TokenType = {
    KEYWORD: 'keyword',
    STRING: 'string',
    NUMBER: 'number',
    COMMENT: 'comment',
    OPERATOR: 'operator',
    PUNCTUATION: 'punctuation',
    IDENTIFIER: 'identifier',
    ATTRIBUTE: 'attribute',
    FUNCTION: 'function',
    DEFAULT: 'default'
  };

  // HCL keywords
  const HCL_KEYWORDS = new Set([
    'job', 'group', 'task', 'service', 'check', 'template', 'artifact',
    'logs', 'resources', 'network', 'constraint', 'affinity', 'spread',
    'volume', 'vault', 'consul', 'update', 'migrate', 'reschedule',
    'periodic', 'parameterized', 'scaling', 'multiregion', 'region',
    'datacenters', 'type', 'priority', 'all_at_once', 'namespace',
    'meta', 'env', 'config', 'driver', 'image', 'port', 'count',
    'cpu', 'memory', 'disk', 'device', 'lifecycle', 'sidecar_task',
    'sidecar_service', 'upstreams', 'connect', 'gateway', 'proxy',
    'true', 'false', 'null'
  ]);

  // JSON keywords
  const JSON_KEYWORDS = new Set(['true', 'false', 'null']);

  /**
   * Tokenizer for HCL syntax
   */
  function tokenizeHCL(code) {
    const tokens = [];
    let i = 0;
    const len = code.length;

    while (i < len) {
      const char = code[i];
      const remaining = code.slice(i);

      // Whitespace
      if (/\s/.test(char)) {
        let ws = '';
        while (i < len && /\s/.test(code[i])) {
          ws += code[i];
          i++;
        }
        tokens.push({ type: 'whitespace', value: ws });
        continue;
      }

      // Single-line comment
      if (remaining.startsWith('#') || remaining.startsWith('//')) {
        let comment = '';
        while (i < len && code[i] !== '\n') {
          comment += code[i];
          i++;
        }
        tokens.push({ type: TokenType.COMMENT, value: comment });
        continue;
      }

      // Multi-line comment
      if (remaining.startsWith('/*')) {
        let comment = '';
        while (i < len && !code.slice(i).startsWith('*/')) {
          comment += code[i];
          i++;
        }
        if (i < len) {
          comment += '*/';
          i += 2;
        }
        tokens.push({ type: TokenType.COMMENT, value: comment });
        continue;
      }

      // String (double-quoted)
      if (char === '"') {
        let str = '"';
        i++;
        while (i < len && code[i] !== '"') {
          if (code[i] === '\\' && i + 1 < len) {
            str += code[i] + code[i + 1];
            i += 2;
          } else {
            str += code[i];
            i++;
          }
        }
        if (i < len) {
          str += '"';
          i++;
        }
        tokens.push({ type: TokenType.STRING, value: str });
        continue;
      }

      // Heredoc
      if (remaining.startsWith('<<')) {
        let heredoc = '<<';
        i += 2;
        // Get the delimiter
        let delimiter = '';
        while (i < len && /[A-Za-z_]/.test(code[i])) {
          delimiter += code[i];
          heredoc += code[i];
          i++;
        }
        // Find the end of heredoc
        const endPattern = new RegExp('\\n' + delimiter + '(?=\\s|$)');
        const match = code.slice(i).match(endPattern);
        if (match) {
          const endIdx = match.index + 1 + delimiter.length;
          heredoc += code.slice(i, i + endIdx);
          i += endIdx;
        } else {
          // Just take the rest if no end found
          heredoc += code.slice(i);
          i = len;
        }
        tokens.push({ type: TokenType.STRING, value: heredoc });
        continue;
      }

      // Number
      if (/[0-9]/.test(char) || (char === '-' && /[0-9]/.test(code[i + 1]))) {
        let num = '';
        if (char === '-') {
          num = '-';
          i++;
        }
        while (i < len && /[0-9.eE+-]/.test(code[i])) {
          num += code[i];
          i++;
        }
        // Check for unit suffix (e.g., 500M, 10G)
        if (i < len && /[a-zA-Z]/.test(code[i])) {
          while (i < len && /[a-zA-Z]/.test(code[i])) {
            num += code[i];
            i++;
          }
        }
        tokens.push({ type: TokenType.NUMBER, value: num });
        continue;
      }

      // Identifier or keyword
      if (/[a-zA-Z_]/.test(char)) {
        let ident = '';
        while (i < len && /[a-zA-Z0-9_-]/.test(code[i])) {
          ident += code[i];
          i++;
        }
        if (HCL_KEYWORDS.has(ident.toLowerCase())) {
          tokens.push({ type: TokenType.KEYWORD, value: ident });
        } else {
          tokens.push({ type: TokenType.IDENTIFIER, value: ident });
        }
        continue;
      }

      // Operators
      if (/[=!<>+\-*/%&|^]/.test(char)) {
        let op = char;
        i++;
        // Check for multi-character operators
        if (i < len && /[=&|]/.test(code[i])) {
          op += code[i];
          i++;
        }
        tokens.push({ type: TokenType.OPERATOR, value: op });
        continue;
      }

      // Punctuation
      if (/[{}[\](),:.?]/.test(char)) {
        tokens.push({ type: TokenType.PUNCTUATION, value: char });
        i++;
        continue;
      }

      // Default: unknown character
      tokens.push({ type: TokenType.DEFAULT, value: char });
      i++;
    }

    return tokens;
  }

  /**
   * Tokenizer for JSON syntax
   */
  function tokenizeJSON(code) {
    const tokens = [];
    let i = 0;
    const len = code.length;

    while (i < len) {
      const char = code[i];

      // Whitespace
      if (/\s/.test(char)) {
        let ws = '';
        while (i < len && /\s/.test(code[i])) {
          ws += code[i];
          i++;
        }
        tokens.push({ type: 'whitespace', value: ws });
        continue;
      }

      // String
      if (char === '"') {
        let str = '"';
        i++;
        while (i < len && code[i] !== '"') {
          if (code[i] === '\\' && i + 1 < len) {
            str += code[i] + code[i + 1];
            i += 2;
          } else {
            str += code[i];
            i++;
          }
        }
        if (i < len) {
          str += '"';
          i++;
        }
        // Check if this is a key (followed by colon)
        let j = i;
        while (j < len && /\s/.test(code[j])) j++;
        if (code[j] === ':') {
          tokens.push({ type: TokenType.ATTRIBUTE, value: str });
        } else {
          tokens.push({ type: TokenType.STRING, value: str });
        }
        continue;
      }

      // Number
      if (/[0-9-]/.test(char)) {
        let num = '';
        while (i < len && /[0-9.eE+-]/.test(code[i])) {
          num += code[i];
          i++;
        }
        tokens.push({ type: TokenType.NUMBER, value: num });
        continue;
      }

      // Keywords (true, false, null)
      if (/[a-z]/.test(char)) {
        let word = '';
        while (i < len && /[a-z]/.test(code[i])) {
          word += code[i];
          i++;
        }
        if (JSON_KEYWORDS.has(word)) {
          tokens.push({ type: TokenType.KEYWORD, value: word });
        } else {
          tokens.push({ type: TokenType.DEFAULT, value: word });
        }
        continue;
      }

      // Punctuation
      if (/[{}[\],:.]/.test(char)) {
        tokens.push({ type: TokenType.PUNCTUATION, value: char });
        i++;
        continue;
      }

      // Default
      tokens.push({ type: TokenType.DEFAULT, value: char });
      i++;
    }

    return tokens;
  }

  /**
   * Escape HTML special characters
   */
  function escapeHtml(text) {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  /**
   * Convert tokens to highlighted HTML
   */
  function tokensToHtml(tokens) {
    return tokens.map(token => {
      const escaped = escapeHtml(token.value);
      if (token.type === 'whitespace') {
        return escaped;
      }
      return `<span class="cm-${token.type}">${escaped}</span>`;
    }).join('');
  }

  /**
   * WanderEditor - A lightweight code editor component
   */
  class WanderEditor {
    constructor(container, options = {}) {
      this.container = typeof container === 'string'
        ? document.querySelector(container)
        : container;

      this.options = {
        language: options.language || 'hcl',
        readOnly: options.readOnly !== false,
        lineNumbers: options.lineNumbers !== false,
        theme: options.theme || 'light',
        value: options.value || '',
        onChange: options.onChange || (() => {}),
        onSave: options.onSave || (() => {}),
        minHeight: options.minHeight || 'var(--workspace-surface-height)',
        maxHeight: options.maxHeight || 'var(--workspace-surface-height)'
      };

      this.value = this.options.value;
      this.isEditing = !this.options.readOnly;

      this.init();
    }

    init() {
      this.container.innerHTML = '';
      this.container.className = 'wander-editor';

      // Create the editor structure
      this.wrapper = document.createElement('div');
      this.wrapper.className = `wander-editor-wrapper theme-${this.options.theme}`;
      this.wrapper.style.cssText = `
        min-height: ${this.options.minHeight};
        max-height: ${this.options.maxHeight};
        overflow: auto;
        border: 1px solid var(--border-color, #e5e7eb);
        border-radius: 0.375rem;
        background: var(--bg-color, #ffffff);
        font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
        font-size: 13px;
        line-height: 1.5;
      `;

      this.editorArea = document.createElement('div');
      this.editorArea.className = 'wander-editor-area';
      this.editorArea.style.cssText = `
        display: flex;
        padding: 0.5rem 0;
      `;

      // Line numbers
      if (this.options.lineNumbers) {
        this.lineNumbers = document.createElement('div');
        this.lineNumbers.className = 'wander-line-numbers';
        this.lineNumbers.style.cssText = `
          padding: 0 0.5rem;
          text-align: right;
          color: #9ca3af;
          user-select: none;
          border-right: 1px solid #e5e7eb;
          min-width: 3rem;
        `;
        this.editorArea.appendChild(this.lineNumbers);
      }

      // Code container
      this.codeContainer = document.createElement('div');
      this.codeContainer.className = 'wander-code-container';
      this.codeContainer.style.cssText = `
        flex: 1;
        padding: 0 1rem;
        overflow-x: auto;
        position: relative;
      `;

      // Highlighted code display
      this.highlightedCode = document.createElement('pre');
      this.highlightedCode.className = 'wander-highlighted-code';
      this.highlightedCode.style.cssText = `
        margin: 0;
        white-space: pre;
        word-wrap: normal;
      `;

      // Hidden textarea for editing
      this.textarea = document.createElement('textarea');
      this.textarea.className = 'wander-textarea';
      this.textarea.style.cssText = `
        position: absolute;
        top: 0;
        left: 1rem;
        right: 0;
        bottom: 0;
        width: calc(100% - 1rem);
        height: 100%;
        padding: 0;
        margin: 0;
        border: none;
        outline: none;
        resize: none;
        background: transparent;
        color: transparent;
        caret-color: #000;
        font: inherit;
        line-height: inherit;
        white-space: pre;
        overflow: hidden;
        z-index: 1;
      `;
      this.textarea.spellcheck = false;
      this.textarea.autocomplete = 'off';
      this.textarea.autocapitalize = 'off';

      this.codeContainer.appendChild(this.highlightedCode);
      this.codeContainer.appendChild(this.textarea);
      this.editorArea.appendChild(this.codeContainer);
      this.wrapper.appendChild(this.editorArea);
      this.container.appendChild(this.wrapper);

      // Set initial value
      this.setValue(this.value);
      this.setReadOnly(this.options.readOnly);

      // Event listeners
      this.textarea.addEventListener('input', () => {
        this.value = this.textarea.value;
        this.highlight();
        this.updateLineNumbers();
        this.options.onChange(this.value);
      });

      this.textarea.addEventListener('scroll', () => {
        this.highlightedCode.scrollTop = this.textarea.scrollTop;
        this.highlightedCode.scrollLeft = this.textarea.scrollLeft;
      });

      this.textarea.addEventListener('keydown', (e) => {
        // Tab handling
        if (e.key === 'Tab') {
          e.preventDefault();
          const start = this.textarea.selectionStart;
          const end = this.textarea.selectionEnd;
          this.textarea.value = this.textarea.value.substring(0, start) + '  ' + this.textarea.value.substring(end);
          this.textarea.selectionStart = this.textarea.selectionEnd = start + 2;
          this.value = this.textarea.value;
          this.highlight();
          this.options.onChange(this.value);
        }

        // Save shortcut (Ctrl/Cmd + S)
        if ((e.ctrlKey || e.metaKey) && e.key === 's') {
          e.preventDefault();
          this.options.onSave(this.value);
        }
      });

      // Bracket matching on cursor position change
      this.textarea.addEventListener('click', () => this.highlightMatchingBracket());
      this.textarea.addEventListener('keyup', (e) => {
        // Update bracket matching on navigation keys
        if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(e.key)) {
          this.highlightMatchingBracket();
        }
      });

      // Sync scroll
      this.wrapper.addEventListener('scroll', () => {
        if (this.lineNumbers) {
          this.lineNumbers.style.transform = `translateY(-${this.wrapper.scrollTop}px)`;
        }
      });
    }

    setValue(value) {
      this.value = value || '';
      this.textarea.value = this.value;
      this.highlight();
      this.updateLineNumbers();
    }

    getValue() {
      return this.value;
    }

    setLanguage(language) {
      this.options.language = language;
      this.highlight();
    }

    setReadOnly(readOnly) {
      this.options.readOnly = readOnly;
      this.isEditing = !readOnly;
      this.textarea.readOnly = readOnly;
      this.textarea.style.display = readOnly ? 'none' : 'block';
      this.wrapper.style.cursor = readOnly ? 'default' : 'text';
    }

    highlight() {
      const tokenizer = this.options.language === 'json' ? tokenizeJSON : tokenizeHCL;
      const tokens = tokenizer(this.value);
      this.highlightedCode.innerHTML = tokensToHtml(tokens);
    }

    updateLineNumbers() {
      if (!this.lineNumbers) return;

      const lines = this.value.split('\n');
      this.lineNumbers.innerHTML = lines
        .map((_, i) => `<div style="height: 1.5em;">${i + 1}</div>`)
        .join('');
    }

    focus() {
      if (!this.options.readOnly) {
        this.textarea.focus();
      }
    }

    highlightMatchingBracket() {
      const pos = this.textarea.selectionStart;
      const text = this.value;
      const brackets = { '(': ')', '[': ']', '{': '}' };
      const closingBrackets = { ')': '(', ']': '[', '}': '{' };

      // Clear previous bracket highlights
      this.clearBracketHighlights();

      // Check character at cursor and before cursor
      const charAtCursor = text[pos];
      const charBeforeCursor = pos > 0 ? text[pos - 1] : '';

      let bracketPos = -1;
      let matchPos = -1;

      // Check if cursor is on or after a bracket
      if (brackets[charAtCursor]) {
        bracketPos = pos;
        matchPos = this.findMatchingBracket(text, pos, charAtCursor, brackets[charAtCursor], 1);
      } else if (closingBrackets[charAtCursor]) {
        bracketPos = pos;
        matchPos = this.findMatchingBracket(text, pos, charAtCursor, closingBrackets[charAtCursor], -1);
      } else if (brackets[charBeforeCursor]) {
        bracketPos = pos - 1;
        matchPos = this.findMatchingBracket(text, pos - 1, charBeforeCursor, brackets[charBeforeCursor], 1);
      } else if (closingBrackets[charBeforeCursor]) {
        bracketPos = pos - 1;
        matchPos = this.findMatchingBracket(text, pos - 1, charBeforeCursor, closingBrackets[charBeforeCursor], -1);
      }

      if (bracketPos >= 0 && matchPos >= 0) {
        this.applyBracketHighlight(bracketPos, matchPos);
      } else if (bracketPos >= 0 && matchPos === -1) {
        // Unmatched bracket - highlight as error
        this.applyBracketHighlight(bracketPos, -1, true);
      }
    }

    findMatchingBracket(text, startPos, openBracket, closeBracket, direction) {
      let depth = 1;
      let pos = startPos + direction;
      const len = text.length;

      // Skip content inside strings and comments
      let inString = false;
      let inComment = false;
      let inMultiComment = false;

      while (pos >= 0 && pos < len && depth > 0) {
        const char = text[pos];
        const prevChar = pos > 0 ? text[pos - 1] : '';
        const nextChar = pos < len - 1 ? text[pos + 1] : '';

        // Handle string detection
        if (char === '"' && prevChar !== '\\' && !inComment && !inMultiComment) {
          inString = !inString;
        }

        // Handle single-line comment
        if (!inString && !inMultiComment && (char === '#' || (char === '/' && nextChar === '/'))) {
          if (direction === 1) {
            // Skip to end of line
            while (pos < len && text[pos] !== '\n') pos++;
          }
        }

        // Handle multi-line comment
        if (!inString && char === '/' && nextChar === '*') {
          inMultiComment = true;
        }
        if (inMultiComment && char === '*' && nextChar === '/') {
          inMultiComment = false;
          pos += direction;
        }

        if (!inString && !inComment && !inMultiComment) {
          if (char === openBracket) {
            depth++;
          } else if (char === closeBracket) {
            depth--;
          }
        }

        if (depth > 0) {
          pos += direction;
        }
      }

      return depth === 0 ? pos : -1;
    }

    clearBracketHighlights() {
      const highlights = this.highlightedCode.querySelectorAll('.cm-bracket-match, .cm-bracket-error');
      highlights.forEach(el => {
        el.classList.remove('cm-bracket-match', 'cm-bracket-error');
      });
    }

    applyBracketHighlight(pos1, pos2, isError = false) {
      // Re-render with bracket highlighting
      const tokenizer = this.options.language === 'json' ? tokenizeJSON : tokenizeHCL;
      const tokens = tokenizer(this.value);

      // Build HTML with bracket highlighting
      let currentPos = 0;
      const parts = [];

      for (const token of tokens) {
        const tokenStart = currentPos;
        const tokenEnd = currentPos + token.value.length;
        const escaped = escapeHtml(token.value);

        if (token.type === 'whitespace') {
          parts.push(escaped);
        } else {
          let className = `cm-${token.type}`;

          // Check if this token contains a bracket that should be highlighted
          if (token.type === TokenType.PUNCTUATION) {
            if (tokenStart === pos1 || tokenStart === pos2) {
              className += isError && pos2 === -1 ? ' cm-bracket-error' : ' cm-bracket-match';
            }
          }

          parts.push(`<span class="${className}">${escaped}</span>`);
        }

        currentPos = tokenEnd;
      }

      this.highlightedCode.innerHTML = parts.join('');
    }

    destroy() {
      this.container.innerHTML = '';
    }
  }

  // CSS styles for syntax highlighting
  const styles = document.createElement('style');
  styles.textContent = `
    .wander-editor {
      --bg-color: #ffffff;
      --border-color: #e5e7eb;
      --text-color: #1f2937;
    }

    .wander-editor-wrapper.theme-dark {
      --bg-color: #1e1e1e;
      --border-color: #374151;
      --text-color: #d4d4d4;
    }

    .wander-editor .cm-keyword {
      color: #8b5cf6;
      font-weight: 500;
    }

    .wander-editor .cm-string {
      color: #059669;
    }

    .wander-editor .cm-number {
      color: #2563eb;
    }

    .wander-editor .cm-comment {
      color: #6b7280;
      font-style: italic;
    }

    .wander-editor .cm-operator {
      color: #f59e0b;
    }

    .wander-editor .cm-punctuation {
      color: #6b7280;
    }

    .wander-editor .cm-identifier {
      color: #1f2937;
    }

    .wander-editor .cm-attribute {
      color: #0891b2;
    }

    .wander-editor .cm-function {
      color: #7c3aed;
    }

    .wander-editor-wrapper.theme-dark .cm-keyword {
      color: #c586c0;
    }

    .wander-editor-wrapper.theme-dark .cm-string {
      color: #6a9955;
    }

    .wander-editor-wrapper.theme-dark .cm-number {
      color: #b5cea8;
    }

    .wander-editor-wrapper.theme-dark .cm-comment {
      color: #6a9955;
    }

    .wander-editor-wrapper.theme-dark .cm-operator {
      color: #d4d4d4;
    }

    .wander-editor-wrapper.theme-dark .cm-punctuation {
      color: #d4d4d4;
    }

    .wander-editor-wrapper.theme-dark .cm-identifier {
      color: #9cdcfe;
    }

    .wander-editor-wrapper.theme-dark .cm-attribute {
      color: #9cdcfe;
    }

    .wander-editor .wander-highlighted-code {
      margin: 0;
      padding: 0;
      border: 0;
      border-radius: 0;
      overflow: visible;
      background: transparent;
      color: var(--text-color, #1f2937);
      font: inherit;
      line-height: inherit;
    }

    /* Bracket matching */
    .wander-editor .cm-bracket-match {
      background-color: rgba(34, 197, 94, 0.3);
      border-radius: 2px;
      outline: 1px solid rgba(34, 197, 94, 0.5);
    }

    .wander-editor .cm-bracket-error {
      background-color: rgba(239, 68, 68, 0.3);
      border-radius: 2px;
      outline: 1px solid rgba(239, 68, 68, 0.5);
    }

    .wander-editor-wrapper.theme-dark .cm-bracket-match {
      background-color: rgba(34, 197, 94, 0.4);
      outline: 1px solid rgba(34, 197, 94, 0.6);
    }

    .wander-editor-wrapper.theme-dark .cm-bracket-error {
      background-color: rgba(239, 68, 68, 0.4);
      outline: 1px solid rgba(239, 68, 68, 0.6);
    }
  `;
  document.head.appendChild(styles);

  // Alpine.js integration
  if (window.Alpine) {
    // Register a magic property for creating editors
    window.Alpine.magic('editor', () => {
      return (container, options) => new WanderEditor(container, options);
    });
  }

  // Export to window
  window.WanderEditor = WanderEditor;

  // Alpine.js data function for job definition editor
  window.jobDefinitionEditor = function(initialValue, format, jobId) {
    return {
      editor: null,
      isEditing: false,
      format: format || 'hcl',
      jobId: jobId || '',
      originalValue: initialValue || '',
      currentValue: initialValue || '',
      definitions: { hcl: '', json: '' },
      hasChanges: false,
      isSaving: false,
      isPlanning: false,
      isRunning: false,
      showPlanModal: false,
      showRunModal: false,
      planResult: {},
      error: null,

      init() {
        const definitionSource = this.$refs.definitionSource;
        if (definitionSource) {
          this.originalValue = definitionSource.value;
          this.currentValue = definitionSource.value;
        }
        this.definitions = {
          hcl: this.$refs.hclSource?.value || '',
          json: this.$refs.jsonSource?.value || ''
        };
        this.format = this.$el.dataset.format || this.format;
        this.jobId = this.$el.dataset.jobId || this.jobId;
		this.definitions[this.format] ||= this.originalValue;

        this.$nextTick(() => {
          const editorContainer = this.$refs?.editorContainer;
          if (!editorContainer?.isConnected) return;
          this.editor = new WanderEditor(editorContainer, {
            value: this.originalValue,
            language: this.format,
            readOnly: true,
            lineNumbers: true,
            onChange: (value) => {
              this.currentValue = value;
              this.hasChanges = value !== this.originalValue;
            },
            onSave: () => {
              if (this.isEditing && this.hasChanges) {
                this.saveChanges();
              }
            }
          });
        });
      },

      toggleEdit() {
        this.isEditing = !this.isEditing;
        if (this.editor) {
          this.editor.setReadOnly(!this.isEditing);
          if (this.isEditing) {
            this.editor.focus();
          }
        }
      },

      cancelEdit() {
        this.isEditing = false;
        this.currentValue = this.originalValue;
        this.hasChanges = false;
        if (this.editor) {
          this.editor.setValue(this.originalValue);
          this.editor.setReadOnly(true);
        }
      },

      setFormat(newFormat) {
        if (newFormat === this.format) return;
        if (this.isEditing && this.hasChanges) {
          this.error = 'Save or cancel your changes before switching formats.';
          return;
        }
        const nextValue = this.definitions[newFormat];
        if (!nextValue) {
          this.error = `${newFormat.toUpperCase()} source is not available for this job.`;
          return;
        }

        this.error = null;
        this.format = newFormat;
        this.originalValue = nextValue;
        this.currentValue = nextValue;
        this.hasChanges = false;
        if (this.editor) {
          this.editor.setLanguage(newFormat);
          this.editor.setValue(nextValue);
        }
      },

      async saveChanges() {
        if (!this.hasChanges || this.isSaving) return;

        this.isSaving = true;
        this.error = null;

        try {
          const response = await fetch(`/api/jobs/${this.jobId}/definition`, {
            method: 'PUT',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              definition: this.currentValue,
              format: this.format
            })
          });

          if (!response.ok) {
            throw new Error('Failed to save job definition');
          }

          this.finishSave();

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to save job definition: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isSaving = false;
        }
      },

      finishSave() {
        this.originalValue = this.currentValue;
        this.definitions[this.format] = this.currentValue;
        this.hasChanges = false;
        this.isEditing = false;
        this.editor?.setReadOnly(true);
        window.dispatchEvent(new CustomEvent('wander:toast', {
          detail: { message: 'Job definition saved successfully', type: 'success' }
        }));
      },

      downloadDefinition() {
        const blob = new Blob([this.currentValue], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `${this.jobId}.${this.format}`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      },

      copyToClipboard() {
        navigator.clipboard.writeText(this.currentValue).then(() => {
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Copied to clipboard',
              type: 'success'
            }
          }));
        }).catch(() => {
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to copy to clipboard',
              type: 'error'
            }
          }));
        });
      },

      async planJob() {
        if (this.isPlanning || this.isRunning) return;

        this.isPlanning = true;
        this.error = null;

        try {
          const response = await fetch(`/ui/jobs/${this.jobId}/plan`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              definition: this.currentValue,
              format: this.format
            })
          });

          const result = await response.json();

          if (!response.ok || result.error) {
            throw new Error(result.error || 'Failed to plan job');
          }

          this.planResult = result;
          this.showPlanModal = true;

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to plan job: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isPlanning = false;
        }
      },

      closePlanModal() {
        this.showPlanModal = false;
        this.planResult = {};
      },

      showRunConfirm() {
        this.showRunModal = true;
      },

      closeRunModal() {
        this.showRunModal = false;
      },

      async runJob() {
        if (this.isRunning) return;

        this.isRunning = true;
        this.error = null;

        try {
          const response = await fetch(`/ui/jobs/${this.jobId}/run`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              definition: this.currentValue,
              format: this.format
            })
          });

          const result = await response.json();

          if (!response.ok || result.error) {
            throw new Error(result.error || 'Failed to run job');
          }

          // Close the modal
          this.closeRunModal();

          // Update original value since job was submitted
          this.originalValue = this.currentValue;
          this.hasChanges = false;

          // Show success message
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: `Job submitted successfully. Evaluation ID: ${result.evalId}`,
              type: 'success'
            }
          }));

          // Optionally exit edit mode
          this.isEditing = false;
          if (this.editor) {
            this.editor.setReadOnly(true);
          }

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to run job: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isRunning = false;
        }
      }
    };
  };

  /**
   * Alpine.js data component for policy editor
   * @param {string} initialRules - Initial HCL rules content
   * @param {string} initialDescription - Initial policy description
   * @param {string} policyName - Policy name
   * @returns {object} Alpine.js component data
   */
  window.policyEditor = function(initialRules, initialDescription, policyName) {
    return {
      editor: null,
      isEditing: false,
      policyName: policyName || '',
      description: initialDescription || '',
      originalDescription: initialDescription || '',
      originalRules: initialRules || '',
      currentRules: initialRules || '',
      hasChanges: false,
      isSaving: false,
      isDeleting: false,
      showDeleteModal: false,
      error: null,

      init() {
        const rulesSource = this.$refs.rulesSource;
        const descriptionSource = this.$refs.descriptionSource;
        if (rulesSource) {
          this.originalRules = rulesSource.value;
          this.currentRules = rulesSource.value;
        }
        if (descriptionSource) {
          this.description = descriptionSource.value;
          this.originalDescription = descriptionSource.value;
        }
        this.policyName = this.$el.dataset.policyName || this.policyName;

        this.$nextTick(() => {
          this.editor = new WanderEditor(this.$refs.editorContainer, {
            value: this.originalRules,
            language: 'hcl',
            readOnly: true,
            lineNumbers: true,
            onChange: (value) => {
              this.currentRules = value;
              this.updateHasChanges();
            },
            onSave: () => {
              if (this.isEditing && this.hasChanges) {
                this.saveChanges();
              }
            }
          });
        });
      },

      updateHasChanges() {
        this.hasChanges = this.currentRules !== this.originalRules ||
                          this.description !== this.originalDescription;
      },

      toggleEdit() {
        this.isEditing = !this.isEditing;
        if (this.editor) {
          this.editor.setReadOnly(!this.isEditing);
          if (this.isEditing) {
            this.editor.focus();
          }
        }
      },

      cancelEdit() {
        this.isEditing = false;
        this.currentRules = this.originalRules;
        this.description = this.originalDescription;
        this.hasChanges = false;
        this.error = null;
        if (this.editor) {
          this.editor.setValue(this.originalRules);
          this.editor.setReadOnly(true);
        }
      },

      async saveChanges() {
        if (this.isSaving || !this.hasChanges) return;

        this.isSaving = true;
        this.error = null;

        try {
          const formData = new FormData();
          formData.append('description', this.description);
          formData.append('rules', this.currentRules);

          const response = await fetch(`/ui/access-control/policies/${this.policyName}`, {
            method: 'POST',
            body: formData
          });

          if (!response.ok) {
            const text = await response.text();
            throw new Error(text || 'Failed to update policy');
          }

          // Update original values since save was successful
          this.originalRules = this.currentRules;
          this.originalDescription = this.description;
          this.hasChanges = false;
          this.isEditing = false;

          if (this.editor) {
            this.editor.setReadOnly(true);
          }

          // Dispatch toast event
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Policy updated successfully',
              type: 'success'
            }
          }));

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to update policy: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isSaving = false;
        }
      },

      copyToClipboard() {
        navigator.clipboard.writeText(this.currentRules).then(() => {
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Policy rules copied to clipboard',
              type: 'success'
            }
          }));
        }).catch(() => {
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to copy to clipboard',
              type: 'error'
            }
          }));
        });
      },

      showDeleteConfirm() {
        this.showDeleteModal = true;
      },

      closeDeleteModal() {
        this.showDeleteModal = false;
      },

      async deletePolicy() {
        if (this.isDeleting) return;

        this.isDeleting = true;
        this.error = null;

        try {
          const response = await fetch(`/ui/access-control/policies/${this.policyName}`, {
            method: 'DELETE'
          });

          if (!response.ok) {
            const text = await response.text();
            throw new Error(text || 'Failed to delete policy');
          }

          // Policy deleted successfully, redirect to policies list
          window.location.href = '/ui/access-control/policies';

        } catch (err) {
          this.error = err.message;
          this.closeDeleteModal();
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to delete policy: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isDeleting = false;
        }
      }
    };
  };

  /**
   * Alpine.js component for job submission editor (new jobs).
   * Similar to jobDefinitionEditor but for creating new jobs without a jobId.
   *
   * @param {string} initialValue - Initial editor content (usually empty)
   * @param {string} format - Initial format ('hcl' or 'json')
   * @param {string} namespace - Nomad namespace for job submission
   * @returns {object} Alpine.js component
   */
  window.jobSubmissionEditor = function(initialValue, format, namespace) {
    return {
      editor: null,
      format: format || 'hcl',
      namespace: namespace || 'default',
      currentValue: initialValue || '',
      hasContent: false,
      isPlanning: false,
      isRunning: false,
      showPlanModal: false,
      showRunModal: false,
      planResult: {},
      error: null,

      init() {
        this.$nextTick(() => {
          this.editor = new WanderEditor(this.$refs.editorContainer, {
            value: this.currentValue,
            language: this.format,
            readOnly: false,
            lineNumbers: true,
            onChange: (value) => {
              this.currentValue = value;
              this.hasContent = value.trim().length > 0;
            },
            onSave: () => {
              // Ctrl+S to plan the job
              if (this.hasContent) {
                this.planJob();
              }
            }
          });
          this.hasContent = this.currentValue.trim().length > 0;
        });
      },

      setFormat(newFormat) {
        this.format = newFormat;
        if (this.editor) {
          this.editor.setLanguage(newFormat);
        }
      },

      clearEditor() {
        this.currentValue = '';
        this.hasContent = false;
        if (this.editor) {
          this.editor.setValue('');
        }
      },

      // Job templates library
      getTemplates() {
        return [
          {
            id: 'service',
            name: 'Service Job',
            description: 'Long-running web server with health checks and rolling updates',
            hcl: `job "web-service" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"

  # Rolling update configuration
  update {
    max_parallel      = 1
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 1
  }

  group "web" {
    count = 3

    network {
      port "http" {
        to = 8080
      }
    }

    service {
      name = "web-service"
      port = "http"
      tags = ["web", "production"]

      check {
        name     = "health"
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "server" {
      driver = "docker"

      config {
        image = "nginx:alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }

      env {
        ENVIRONMENT = "production"
      }
    }
  }
}`
          },
          {
            id: 'batch',
            name: 'Batch Job',
            description: 'One-time task that runs to completion',
            hcl: `job "data-processor" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "batch"

  group "processor" {
    count = 1

    # Restart on failure
    restart {
      attempts = 3
      interval = "5m"
      delay    = "30s"
      mode     = "fail"
    }

    task "process" {
      driver = "docker"

      config {
        image = "alpine:latest"
        command = "sh"
        args = ["-c", "echo 'Processing data...' && sleep 10"]
      }

      resources {
        cpu    = 500
        memory = 256
      }

      # Job will complete when task exits successfully
      lifecycle {
        hook = "prestart"
      }
    }
  }
}`
          },
          {
            id: 'system',
            name: 'System Job',
            description: 'Daemon that runs on all nodes (monitoring, logging)',
            hcl: `job "node-exporter" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "system"

  group "monitoring" {
    # No count specified - runs on all nodes

    network {
      port "metrics" {
        static = 9100
        to     = 9100
      }
    }

    service {
      name = "node-exporter"
      port = "metrics"
      tags = ["monitoring", "metrics"]

      check {
        name     = "metrics-alive"
        type     = "http"
        path     = "/metrics"
        interval = "30s"
        timeout  = "5s"
      }
    }

    task "exporter" {
      driver = "docker"

      config {
        image = "prom/node-exporter:latest"
        ports = ["metrics"]
        args = [
          "--path.rootfs=/host"
        ]
        volumes = [
          "/:/host:ro,rslave"
        ]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}`
          },
          {
            id: 'periodic',
            name: 'Periodic Job',
            description: 'Cron-like scheduled task that runs periodically',
            hcl: `job "backup" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "batch"

  # Run every day at 2 AM
  periodic {
    cron             = "0 2 * * *"
    prohibit_overlap = true
    time_zone        = "America/New_York"
  }

  group "backup" {
    count = 1

    task "run-backup" {
      driver = "docker"

      config {
        image = "alpine:latest"
        command = "sh"
        args = [
          "-c",
          "echo 'Running backup at' $(date) && sleep 30 && echo 'Backup complete'"
        ]
      }

      resources {
        cpu    = 500
        memory = 256
      }

      # Environment variables for backup configuration
      env {
        BACKUP_TARGET = "s3://my-backup-bucket"
        RETENTION_DAYS = "30"
      }
    }
  }
}`
          },
          {
            id: 'parameterized',
            name: 'Parameterized Job',
            description: 'Job template that accepts parameters at dispatch time',
            hcl: `job "batch-processor" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "batch"

  # Accept parameters when dispatching
  parameterized {
    payload       = "required"
    meta_required = ["task_type", "priority"]
    meta_optional = ["callback_url"]
  }

  group "processor" {
    count = 1

    task "process" {
      driver = "docker"

      config {
        image = "alpine:latest"
        command = "sh"
        args = [
          "-c",
          "echo 'Processing task:' $NOMAD_META_TASK_TYPE && cat $NOMAD_TASK_DIR/payload"
        ]
      }

      # Dispatch meta available as env vars
      template {
        data = <<EOF
TASK_TYPE={{ env "NOMAD_META_TASK_TYPE" }}
PRIORITY={{ env "NOMAD_META_PRIORITY" }}
{{ if env "NOMAD_META_CALLBACK_URL" }}
CALLBACK_URL={{ env "NOMAD_META_CALLBACK_URL" }}
{{ end }}
EOF
        destination = "local/config.env"
        env         = true
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`
          },
          {
            id: 'connect',
            name: 'Service Mesh Job',
            description: 'Service with Consul Connect for secure service-to-service communication',
            hcl: `job "api-service" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"

  group "api" {
    count = 2

    network {
      mode = "bridge"
      port "api" {
        to = 8080
      }
    }

    service {
      name = "api-service"
      port = "8080"

      # Enable Consul Connect
      connect {
        sidecar_service {
          proxy {
            # Upstream services this service needs to connect to
            upstreams {
              destination_name = "database"
              local_bind_port  = 5432
            }
            upstreams {
              destination_name = "cache"
              local_bind_port  = 6379
            }
          }
        }
      }

      check {
        name     = "api-health"
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "api" {
      driver = "docker"

      config {
        image = "my-api:latest"
      }

      resources {
        cpu    = 500
        memory = 512
      }

      env {
        DATABASE_HOST = "localhost"
        DATABASE_PORT = "5432"
        CACHE_HOST    = "localhost"
        CACHE_PORT    = "6379"
      }
    }
  }
}`
          }
        ];
      },

      loadTemplate(templateId) {
        const templates = this.getTemplates();
        const template = templates.find(t => t.id === templateId);

        if (!template) {
          console.error('Template not found:', templateId);
          return;
        }

        // For now, we only have HCL templates
        // In the future, we could add JSON versions
        const content = template.hcl;

        this.currentValue = content;
        this.hasContent = true;
        if (this.editor) {
          this.editor.setValue(content);
        }

        // If format is JSON, we could attempt to convert or show a message
        // For now, switch to HCL when loading a template
        if (this.format !== 'hcl') {
          this.setFormat('hcl');
        }

        window.dispatchEvent(new CustomEvent('wander:toast', {
          detail: {
            message: `Loaded template: ${template.name}`,
            type: 'success'
          }
        }));
      },

      loadExample() {
        // Load the basic service template as the example
        this.loadTemplate('service');
      },

      uploadFile(event) {
        const file = event.target.files[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (e) => {
          this.currentValue = e.target.result;
          this.hasContent = true;
          if (this.editor) {
            this.editor.setValue(this.currentValue);
          }

          // Auto-detect format from file extension
          const ext = file.name.split('.').pop().toLowerCase();
          if (ext === 'json') {
            this.setFormat('json');
          } else {
            this.setFormat('hcl');
          }

          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: `Loaded file: ${file.name}`,
              type: 'success'
            }
          }));
        };
        reader.onerror = () => {
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to read file',
              type: 'error'
            }
          }));
        };
        reader.readAsText(file);

        // Reset the file input so the same file can be loaded again
        event.target.value = '';
      },

      async planJob() {
        if (this.isPlanning || this.isRunning || !this.hasContent) return;

        this.isPlanning = true;
        this.error = null;

        try {
          // For new job submission, parse the job to extract the ID.
          // The backend will use the job ID from the definition for the plan endpoint.
          // We use a placeholder "new-job" path which the backend will handle.
          const response = await fetch('/ui/jobs/_new/plan', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              definition: this.currentValue,
              format: this.format
            })
          });

          const result = await response.json();

          if (!response.ok || result.error) {
            throw new Error(result.error || 'Failed to plan job');
          }

          this.planResult = result;
          this.showPlanModal = true;

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to plan job: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isPlanning = false;
        }
      },

      closePlanModal() {
        this.showPlanModal = false;
        this.planResult = {};
      },

      showRunConfirm() {
        this.showRunModal = true;
      },

      closeRunModal() {
        this.showRunModal = false;
      },

      async runJob() {
        if (this.isRunning || !this.hasContent) return;

        this.isRunning = true;
        this.error = null;

        try {
          const response = await fetch('/ui/jobs/_new/run', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              definition: this.currentValue,
              format: this.format
            })
          });

          const result = await response.json();

          if (!response.ok || result.error) {
            throw new Error(result.error || 'Failed to submit job');
          }

          // Close the modal
          this.closeRunModal();

          // Show success message
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: `Job submitted successfully. Job ID: ${result.jobId}. Evaluation ID: ${result.evalId}`,
              type: 'success',
              duration: 5000
            }
          }));

          // Redirect to the job detail page after a short delay
          setTimeout(() => {
            window.location.href = `/ui/jobs/${result.jobId}`;
          }, 1500);

        } catch (err) {
          this.error = err.message;
          window.dispatchEvent(new CustomEvent('wander:toast', {
            detail: {
              message: 'Failed to submit job: ' + err.message,
              type: 'error'
            }
          }));
        } finally {
          this.isRunning = false;
        }
      }
    };
  };

})(window);
