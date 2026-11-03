# Evaluation List Pagination

## Context

The evaluations handler already parses `page` and `per_page`, sorts the live
API response, bounds the requested page, and slices the result. The rendered
page receives only that slice, however, so users have no result count, page
size control, or way to navigate beyond the first page. The route parity
matrix therefore still classifies the evaluations list as partial.

## Research Findings

- `parseListViewOptions` and `paginationBounds` provide the same validated
  page contract used by other Wander list routes.
- The shared `components.Pagination` renders accessible previous, numbered,
  and next links and supports HTMX updates.
- Evaluation filtering and sorting happen before slicing, so pagination must
  retain `search`, `status`, `type`, `job`, `sort`, `dir`, namespace, and
  region parameters.
- The evaluations slide-out is a separate recent-items view. It should retain
  its compact cards and must not push page URLs during its periodic refresh.
- No new API or fabricated data is needed; the existing live evaluation read
  remains the source of truth.

## Acceptance Criteria

- [x] The full evaluations page shows a truthful result range and total.
- [x] Previous, numbered, and next controls navigate bounded pages without
      losing filters, sort order, namespace, or region.
- [x] Users can select 10, 25, 50, or 100 evaluations per page.
- [x] Out-of-range and malformed page parameters resolve deterministically.
- [x] The slide-out panel remains compact and does not render full-page
      pagination or push browser history.
- [x] Handler/render tests and an embedded Chromium workflow cover navigation.

## Technical Approach

1. Add evaluation list props containing the rendered evaluations, current
   view options, and pagination metadata.
2. Build scoped list URLs from validated query values and render the shared
   pagination component beneath the cards.
3. Add a small GET/HTMX page-size form that resets page selection while
   preserving the active view and scope.
4. Render the compact `List` partial for panel requests and the richer
   `ListContent` only for the full route.
5. Cover filter/sort retention, page bounds, panel separation, and live browser
   navigation.

## Testing Strategy

- Handler tests use a fake evaluations API with more records than one page and
  assert page membership, result counts, and retained query parameters.
- Template tests cover page-size selection and compact-panel separation.
- Chromium creates enough disposable evaluations, navigates pages, and checks
  that the URL and visible records change without browser errors.
- Run templ generation, UI Go tests, JavaScript tests, lint, security scans,
  Deslop, and the embedded Chromium release suite.

## Risks and Mitigations

- Evaluation lists can change between requests. Apply deterministic sorting
  with an ID tie-breaker before slicing.
- HTMX pagination can accidentally replace the wrong region. Target a stable
  evaluation-list container for list navigation and retain the full-page
  target for page-size form submissions.
- Panel polling must not inherit page controls. Keep panel rendering on the
  existing compact component.
