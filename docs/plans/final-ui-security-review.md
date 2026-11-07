# Final UI Security Review

## Scope

This review covers the browser-facing security boundary required for Wander
0.1.0: ACL token cookies, content security policy, WebSocket origin checks,
ACL and namespace scoping, user input, allocation file paths, and error
redaction. It applies to the Go/Templ/HTMX UI and the same-origin `/api` and
streaming routes embedded in the Wander binary.

## Results

| Boundary | Result | Evidence |
| --- | --- | --- |
| ACL token cookie | Pass | `wander_token` is `HttpOnly`, path `/`, `SameSite=Strict`, and `Secure` for direct TLS or `X-Forwarded-Proto: https`. Local HTTP remains available for the documented development flow. OIDC transaction cookies are `HttpOnly`, `SameSite=Lax`, short-lived, and `Secure` on HTTPS. |
| CSP | Pass | The default policy denies all unspecified sources, limits scripts and browser connections to `'self'`, disallows form submission and framing, and permits only the image/style exceptions required by the UI. Operators may explicitly override the inherited CSP configuration. |
| WebSocket origin | Pass | Exec uses Gorilla WebSocket's default same-origin policy; no permissive `CheckOrigin` override is installed. HTTPS/WSS transport is exercised by the release browser suite. |
| ACL and scope propagation | Pass | Request-scoped tokens, namespaces, and regions are propagated to API reads, writes, SSE streams, and exec. `TestACLMutations` exercises anonymous denial and authorized success against an ACL-enabled disposable cluster, including cross-namespace isolation. |
| Input validation | Pass | Mutation handlers validate methods, required fields, enum/range values, namespace/region selections, modify indexes, dispatch metadata, drain strategies, task names, and log/file modes before calling the cluster API. Validation errors returned to browsers are locally generated. |
| Allocation file paths | Pass | `allocationFilePath` rejects parent traversal and invalid task names before applying task scope. Preview types are allowlisted, downloads use safe attachment headers, and inline responses use `nosniff`. |
| Error redaction | Pass | Upstream cluster and stream errors are retained in structured server logs but replaced with stable public messages in HTML, JSON, SSE events, and HTMX trigger headers. Remaining direct `err.Error()` responses originate only from local validation functions with bounded messages. |

## Regression Gates

The following tests cover the security contract:

- `go test ./ui/handlers`
- `go test -tags ui ./command/agent -run TestUIRuntimeContract`
- `TestBrowserCookieSecurityAttributes` and `TestRequestUsesTLS`
- `TestExecHandlerUsesDefaultSameOriginPolicy`
- `TestAllocationFilePathScopesTaskAndRejectsTraversal`
- release E2E `TestACLMutations` and `TestTLSTransports`

The assembled embedded UI passed the final live gate on 2026-11-07. The seeded
crawler reached 94 pages, checked 93 unique links and 12 assets, and found no
failures. Chromium passed all 24 applicable release-smoke cases (the two CSI
cases were skipped because the disposable cluster has no CSI plugin or volume)
without console or CSP errors.
