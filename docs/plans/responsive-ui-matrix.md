# Responsive UI Matrix

## Context

The application shell has a mobile drawer, clipped breadcrumbs, responsive
selectors, horizontally scrollable tables, wrapping control bars, and bounded
allocation workspaces. These behaviors have individual tests but no single
matrix proving that they compose without document-level clipping or overflow.

## Acceptance Criteria

- [x] Phone (390 px), tablet (768 px), laptop (1366 px), and wide desktop
      (1920 px) render without document-level horizontal overflow.
- [x] The sidebar is off-canvas below the desktop breakpoint and can be opened,
      closed with Escape, and reached by its labelled toggle.
- [x] Breadcrumbs, connection state, scope controls, search, main content, and
      the full-width footer remain within the viewport.
- [x] Wide tables scroll inside their own container rather than widening the
      document.
- [x] The task-group scale dialog remains fully reachable at every viewport.
- [x] Allocation log, file, and exec workspaces remain reachable at phone and
      wide-desktop sizes.

## Technical Approach

Add a live Chromium matrix to the existing disposable-cluster release smoke.
Assert shell geometry and document scroll width, exercise the mobile drawer and
scale dialog, and sample allocation workspaces at the two size extremes.

## Boundary

This is a layout and reachability check, not visual snapshot testing. Local
horizontal scrolling is expected for data tables; document-level horizontal
scrolling is not.
