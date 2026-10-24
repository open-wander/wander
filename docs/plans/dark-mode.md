# Wander UI Dark Mode

## Status

Post-0.1 feature. Wander 0.1.0 does not advertise or expose dark mode because
the current dark utility classes cover only part of the interface. A toggle
must not be added until the complete UI can switch themes consistently.

## Scope

- Add a three-way `System`, `Light`, and `Dark` theme selector.
- Persist an explicit choice locally while continuing to follow operating
  system changes when `System` is selected.
- Apply the selected theme before first paint to avoid a light-theme flash.
- Cover the application shell, navigation, cards, tables, forms, dialogs,
  menus, status badges, editors, log and exec terminals, file browser,
  topology, charts, empty states, and error states.
- Keep the implementation compatible with Wander's content security policy
  and avoid adding a runtime or npm dependency solely for theme selection.
- Verify readable focus, hover, selected, disabled, warning, error, and success
  states against WCAG AA contrast expectations.

## Acceptance Criteria

- Theme selection is available from every authenticated page and is keyboard
  and screen-reader accessible.
- The selected theme survives navigation and browser restarts.
- `System` follows the browser's `prefers-color-scheme` value and reacts when
  that value changes.
- No supported page displays a mixture of light and dark surfaces.
- Code editors, streamed logs, and exec terminals remain legible in all three
  modes.
- Browser tests cover first paint, persistence, system-theme changes, and the
  main UI routes in both light and dark modes.

## Release Boundary

This work is intentionally deferred until after the stable 0.1.0 release. It
is a Wander UI enhancement and is not required for Nomad 1.6.5 operational
compatibility.
