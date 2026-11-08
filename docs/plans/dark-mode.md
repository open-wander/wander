# Wander UI Dark Mode

## Status

In progress on `feature/dark-mode`. Wander 0.1.0 did not advertise or expose
dark mode because the existing dark utility classes covered only part of the
interface.

## Existing Foundation

- Tailwind is already configured with `darkMode: "class"`.
- Global typography, form, table, button, and input rules already include dark
  variants.
- Many newer page partials already include explicit `dark:` utilities.
- The application shell, dropdowns, dialogs, and several older detail views
  still contain light-only surfaces and must be completed before release.
- Wander's default content security policy permits same-origin scripts but not
  inline scripts, so first-paint initialization must use an external asset.

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

## Technical Approach

1. Add a small, dependency-free `theme.js` controller and load it before the
   stylesheets. It owns the `wander.theme` preference, resolves `System`
   through `prefers-color-scheme`, and applies the root `dark` class before
   first paint.
2. Expose explicit `System`, `Light`, and `Dark` controls through a reusable
   server-rendered component in the global header and settings page. Controls
   use data attributes and native ARIA states so they continue to work after
   HTMX swaps without inline JavaScript.
3. Complete dark variants in layers: application shell and shared components,
   operational allocation/job views, then remaining management and error
   views. Do not introduce broad CSS remapping rules that could overwrite
   intentional warning, error, success, or chart colors.
4. Keep terminal surfaces intentionally near-black in both themes while
   checking their surrounding controls, borders, and status text.
5. Regenerate committed Templ output and Tailwind CSS after template changes.

## Verification

- Unit-test preference validation, unavailable storage, explicit persistence,
  system preference resolution, and live operating-system changes.
- Template-test script ordering and accessible selector markup.
- Browser-test persistence across reload/navigation, reactive `System` mode,
  and representative jobs, allocations, clients, topology, ACL, variables,
  settings, dialog, and error surfaces.
- Run Go UI tests, JavaScript tests, the focused Playwright suite, and Deslop
  against the final diff.

## Work Checklist

- [x] Theme controller, first-paint bootstrap, and unit tests
- [x] Accessible global and settings selectors
- [ ] Shell and shared component dark variants
- [ ] Jobs, task groups, allocations, logs, exec, and files
- [ ] Clients, servers, services, topology, CSI, and evaluations
- [ ] Namespaces, variables, access control, settings, dialogs, and errors
- [ ] Browser route/interaction coverage in light and dark modes
- [ ] Documentation and final static-analysis pass

## Risks and Mitigations

- A corrupt or blocked `localStorage` must never prevent page rendering; all
  storage access is guarded and falls back to `System`.
- HTMX replaces page content without a full load; event delegation and a
  theme-change event keep newly swapped controls synchronized.
- Charts and terminals use their own color systems; they receive an explicit
  theme-change notification instead of relying only on Tailwind classes.
- Light-only utilities are fixed at their source rather than globally remapped,
  preserving semantic status colors and making future component ownership
  clear.

## Release Boundary

This work is intentionally deferred until after the stable 0.1.0 release. It
is a Wander UI enhancement and is not required for Nomad 1.6.5 operational
compatibility.
