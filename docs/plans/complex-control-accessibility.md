# Complex-Control Accessibility

## Context

Primary navigation has basic keyboard and naming coverage, but the 0.1.0
release also needs operable dialogs, menus, tabs, tables, editors, file trees,
logs, and exec controls. The review found click-only file rows, incomplete menu
keyboard behavior, and missing focus restoration and live-state semantics.

## Acceptance Criteria

- [x] Header menus expose their expanded state and support Arrow, Home, End,
      Escape, and focus-return behavior.
- [x] Tabs support standard arrow-key cycling with matching HTMX activation.
- [x] Dialogs trap focus, close with Escape, and return focus to their opener.
- [x] Tables and editors retain accessible names and keyboard-reachable controls.
- [x] Allocation directories and files expose button semantics and are
      keyboard operable.
- [x] Log and exec state/output regions expose useful names and live status.
- [x] A live Chromium flow verifies the complex controls without browser errors.

## Boundary

This work verifies semantic and keyboard operability. It does not claim formal
WCAG certification or replace testing with multiple screen-reader products.
