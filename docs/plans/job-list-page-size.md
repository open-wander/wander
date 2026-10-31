# Job list page-size parity

## Context

The jobs handler already paginates locally, but the page does not expose the
page-size control observed in the approved Nomad 1.6.5 black-box evidence.
Users also lose the current page size when they apply filters or sort a column.

## Research findings

- Job pagination currently defaults to 10 and caps values at 100.
- The shared pagination component carries `per_page` between page links.
- Job filter and sort URLs are assembled separately, so both need the selected
  page size explicitly.
- A page-size change should return to page 1 to avoid landing beyond the new
  result range.

## Acceptance criteria

- [x] The jobs page offers 10, 25, 50, and 100 rows per page.
- [x] Applying filters and sorting preserve the selected page size.
- [x] Changing page size resets pagination to the first page.
- [x] Invalid or excessive values retain the existing safe parsing behavior.
- [x] Handler and live Chromium tests cover the control and query state.
- [x] The route parity matrix records the completed page-size scope.

## Technical approach

1. Render `per_page` in the existing jobs filter form using the parsed
   pagination value.
2. Include `per_page` in job sort base URLs while leaving the shared
   pagination component responsible for numbered-page links.
3. Extend handler and live-browser coverage, then update parity documentation.

## Testing

- Focused jobs handler tests for selected value and preserved sort URLs.
- Live Chromium selection check against the disposable development cluster.
- Full UI tests, generation, formatting, and Deslop review.

## Risks

- Duplicate `per_page` query values can make navigation ambiguous. Keep it out
  of pagination base URLs because the shared component appends it itself.
- Arbitrary existing query values may not match a visible option. The parser
  remains authoritative and the template can expose the normalized value.
