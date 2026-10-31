# Allocation workspace scrolling and exec repair

## Context

The allocation detail tabs can be taller than the available application shell. The main flex child has vertical overflow enabled, but no `min-height: 0`, so its minimum content height can exceed the shell and be clipped instead of scrolling.

The exec tab creates two xterm instances. Alpine automatically invokes an `init()` method returned by `x-data`, and the template also invokes `init()` through `x-init`. The second instance owns the live WebSocket while the first stale instance remains visible, making a working shell appear stuck at "Connecting".

The smoke fixture's repeated `wander-ui-smoke` log records are intentional: the fixture emits that line every five seconds to exercise live log following.

## Acceptance criteria

- Allocation task, log, exec, file, and raw tab content can scroll within the application shell at desktop and smaller viewport heights.
- The global footer remains reachable and content is not clipped beneath it.
- Opening the exec tab creates exactly one xterm instance.
- Connecting exec reaches a shell prompt and terminal input/output is visible against the live smoke allocation.
- WebSocket URLs retain the selected namespace and region.
- Existing log following and the five-line minimum empty-log surface remain unchanged.

## Approach

1. Allow the shared main flex scroll region to shrink with `min-h-0`.
2. Rely on Alpine's automatic `init()` lifecycle for the exec component and remove the duplicate explicit invocation.
3. Add template and browser-level regression assertions for scrolling, a single xterm instance, and live exec input/output.

## Verification

- Run focused Go/template tests and UI unit checks.
- Build the embedded development UI.
- Exercise logs and exec with headless Chromium against a real local dev allocation.
- Run the relevant release smoke tests where practical.
- Scan changed code for duplication before committing.

## Risks

- Changing the shared main wrapper affects every page, so list/detail pages and the footer need a quick layout smoke check.
- xterm fitting depends on a measurable container; verify initial and resized terminal dimensions after removing the duplicate initialization.
