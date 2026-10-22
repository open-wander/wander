# Feature: Live namespaces in the variable form

## Context

The variable creation form currently presents fabricated `production`,
`staging`, and `development` namespace choices. This can submit a variable to a
namespace that does not exist and breaks the request-scoped namespace model.

## Research Findings

- `BaseLayoutProps` already loads the namespaces visible to the current request
  and ACL token.
- The header uses that same list for namespace selection.
- Variable creation redirects to its detail page, so the selected namespace
  must remain explicit in that redirect.

## Acceptance Criteria

- [x] The form contains only namespaces returned for the current request.
- [x] The request-scoped namespace is selected by default.
- [x] The create redirect retains the variable namespace.
- [x] A browser test creates, reads, updates, and deletes a variable in
      `team-a` and proves it is absent from `default` while it exists.

## Technical Approach

1. Pass request-scoped namespace data from the handler into the create partial.
2. Build select options from live namespace metadata, retaining the current
   namespace as a safe fallback.
3. Include the created namespace in the detail redirect.
4. Extend the disposable-cluster Playwright workflow.

## Testing Strategy

- Unit test namespace option construction and fallback behavior.
- Run the Variables Playwright workflow against the disposable cluster.
- Run UI handler tests, template generation, and deslop on changed files.

## Risks and Mitigations

- Namespace listing may be unavailable during startup; keep only the current
  request namespace as a fallback instead of inventing alternatives.
- Cross-namespace redirects can lose scope; retain it in the redirect query.
