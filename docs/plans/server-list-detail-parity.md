# Feature: Server List and Detail Parity

## Context

Wander lists and inspects servers, but its current columns, sorting, pagination,
address formatting, and tag controls do not yet match the Nomad 1.6.5 UI that
0.1.0 replaces.

## Baseline findings

- The MPL-2.0 1.6.5 server list uses eight rows per page and defaults to leader
  sorting in descending order.
- Its columns are Name, Status, Leader, Address, Serf port, Datacenter, and
  Version. Version comes from the `build` member tag.
- Server detail displays the RPC address using the member `port` tag rather
  than the Serf membership port.
- Tags are sorted by name and every value has a copy-to-clipboard control.
- These files are not in `docs/licenses/baseline-busl-exclusions.txt`.

## Acceptance criteria

- [x] The server list exposes the inherited columns and leader-first default.
- [x] Sort links and pagination preserve region and support the existing
      Wander query contract as well as inherited sort aliases.
- [x] Pagination defaults to eight rows and handles invalid/out-of-range input
      truthfully.
- [x] Server detail shows the RPC address, sorted tags, and accessible copy
      controls with visible success/failure feedback.
- [x] Handler and browser coverage prove list, pagination, detail, and copy
      behavior without fabricated server data.

## Verification

- Focused handler/template tests for sorting, pagination, ports, and tags.
- Release Chromium coverage for columns and tag copying.
- `go test ./ui/...`, `make check`, vulnerability scan, and Deslop.
