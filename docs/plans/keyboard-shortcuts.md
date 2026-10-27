# Keyboard Shortcut Compatibility

Status: Complete

## Context

Wander 0.1.0 must preserve the applicable global keyboard behavior observed in
the approved Nomad 1.6.5 black-box capture. The current UI contains an
incomplete Alpine handler, but it has no visible help panel or rebinding
workflow and assigns `g a` to Allocations instead of ACL tokens.

The existing `/` shortcut and Ctrl/Command+K alias remain part of Wander
because the global search field advertises keyboard access and removing it
would regress current behavior.

## Acceptance criteria

- `g j`, `g r`, `g s`, `g c`, `g t`, `g e`, and `g a` navigate to the
  matching major areas, with namespace and region scope preserved.
- Shift+Left/Right moves between the current page's visible tabs;
  Shift+Up/Down moves between visible main-navigation entries.
- `u` follows the last linked breadcrumb to the parent level.
- `/` focuses the global search field.
- `?` opens an accessible Keyboard Shortcuts dialog and Escape closes it.
- Users can enable or disable navigation shortcuts, rebind every action, reset
  defaults, and receive an actionable collision error.
- Preferences are versioned and persisted locally; corrupt or obsolete data
  falls back safely to defaults.
- Shortcuts do not fire while the user is typing or editing content.
- The implementation adds no npm dependency and has Node built-in tests for
  matching and preference validation plus Go render tests for the UI contract.

## Implementation

1. Replace the Alpine shortcut block with a dependency-free controller in a
   dedicated static JavaScript file.
2. Add a server-rendered dialog and a visible sidebar entry point.
3. Exercise pure key normalization, sequence matching, collision handling,
   and preference sanitization with `node:test`.
4. Verify rendered accessibility hooks in Go, then run the UI and repository
   quality gates before updating the parity documents.

## Risks

- Browser and operating-system shortcuts can conflict with custom bindings.
  Modifier combinations are accepted, but the UI warns on duplicates and
  always reserves Escape for dismissal.
- HTMX swaps page content without rebuilding the shell. Navigation actions
  therefore resolve tabs, breadcrumbs, and sidebar entries at invocation time.
- The help shortcut remains available while navigation shortcuts are disabled
  so the user can re-enable them without clearing browser storage.

## Restart checkpoint

Implemented in the Wander shell with dependency-free JavaScript tests, Go
render assertions, and the live embedded-cluster Chromium release suite. The
browser coverage verifies help, disable/re-enable, rebinding, scoped major-area
navigation, main-section and tab movement, parent navigation, and search focus.

The CSI plugin and volume detail compatibility gaps were completed on
2026-10-27. Registration and deregistration remain inherited API/CLI
operations rather than Nomad 1.6.5 browser requirements. The next focused UI
blocker is evaluation/deployment empty, denied, failed, blocked, and completed
state coverage recorded in the route-parity matrix and revival checklist.
