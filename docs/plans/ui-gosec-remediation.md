# Security: UI Gosec Remediation

## Context

The release UI currently has 23 medium-or-higher `gosec` findings. Some expose
real hardening gaps, while others are analyzer false positives around dynamic
cookie flags, escaped HTML, inherited header names, or developer tooling. The
0.1.0 release needs an auditable clean scan without hiding unrelated rules.

## Research findings

- Job versions, task restart counts, and plan annotations originate as
  `uint64`, but several UI models narrow them to `int` without checking.
- Allocation downloads construct `Content-Disposition` manually and omit
  `X-Content-Type-Options: nosniff`.
- ACL and selection cookies already set `HttpOnly`, `SameSite`, and a dynamic
  `Secure` flag; `gosec` cannot prove the request-dependent value.
- The development server uses `http.ListenAndServe` without explicit
  timeouts, while the UI test server already contains the desired hardened
  `http.Server` pattern.
- `ui/handlers/example.go` and `placeholderHandler` are unregistered
  scaffolding. Their string-built HTML accounts for real taint findings and is
  unnecessary product code.
- Gosec supports narrow `#nosec RULE -- justification` annotations for
  manually verified false positives. Go's `http.Server` exposes explicit
  header, read, write, and idle timeout controls.

## Acceptance criteria

- [x] No unchecked integer narrowing remains in the reported UI paths.
- [x] Allocation downloads use safe content-disposition encoding and disable
      MIME sniffing.
- [x] Cookie construction is shared, tested for HTTP and HTTPS, and any G124
      annotations identify only the verified dynamic-`Secure` false positive.
- [x] The development server has explicit timeouts.
- [x] Unregistered example and placeholder handlers are removed.
- [x] Trusted-tool and escaped-output findings have rule-specific,
      justified annotations only where a code fix would not improve safety.
- [x] `gosec -severity medium -exclude-generated ./ui/...` has no
      unsuppressed findings.
- [x] UI tests, `make check`, `govulncheck`, and Deslop pass.

## Technical approach

1. Preserve unsigned API values in view models and responses; perform checked
   conversion only where the inherited API requires `int`.
2. Encode attachment filenames with `mime.FormatMediaType`, force download
   semantics, and set `nosniff`.
3. Centralize browser cookie attributes and cover their transport-dependent
   security flag.
4. Apply the established test-server timeout configuration to the development
   server.
5. Delete dead demonstration handlers and their obsolete tests.
6. Validate the link-checker output target and document the remaining safe
   analyzer exceptions inline.

## Testing strategy

- Unit tests for integer boundaries, cookie attributes, safe attachment
  headers, and path validation.
- Existing handler/template tests after view-model type changes.
- Full UI Go tests and live release Chromium smoke tests.
- Medium/high `gosec`, `govulncheck`, `make check`, and Deslop scans.

## Risks and mitigations

- Changing version types touches templates and route parsing; preserve URL and
  JSON wire representations and exercise version tests.
- `Secure` cookies cannot be unconditional because local HTTP development is
  supported; keep the request-dependent behavior and test both transports.
- Security suppressions can become broad technical debt; require an explicit
  rule ID and justification on the exact reported line.
