# OIDC browser login restoration

Status: complete (2026-10-26)

## Compatibility target

Restore the Nomad 1.6.5 OSS browser behavior observed through the approved
black-box process:

- `/ui/settings/tokens` remains the OIDC callback and token sign-in page.
- unauthenticated users see one sign-in action for each configured OIDC auth
  method;
- JWT-only auth methods do not add an interactive browser action;
- manual ACL token sign-in and sign-out continue to work.

## Implementation boundary

Use only Wander's inherited MPL-2.0 public API client and independently written
UI handlers/templates. Do not inspect or copy the Nomad 1.6.5 web UI source.

## Work plan

- [x] Add the compatible settings/tokens route and OIDC method discovery.
- [x] Start OIDC authentication with a cryptographic client nonce and a short-
      lived, signed, HttpOnly browser transaction cookie.
- [x] Complete the callback through the inherited ACL OIDC API, clear the
      one-time transaction, and store the returned ACL secret in Wander's
      existing secure token cookie.
- [x] Cover OIDC-only filtering, request/callback payloads, cookie attributes,
      tampering, missing/replayed transactions, and provider failures.
- [x] Run UI tests, security/static checks, license-boundary checks, and Deslop.
- [x] Update the revival checklist and route parity matrix with the evidence.

## Restart checkpoint

The implementation is complete and ready to commit. The inherited
`api.ACLAuthMethods` and `api.ACLAuth` clients provide method listing,
auth-URL generation, and callback completion. The compatible redirect URI is
the browser-visible `{scheme}://{host}/ui/settings/tokens`.

Verification completed on 2026-10-26:

- `go test ./ui/...`
- `go vet ./ui/...`
- `gosec -quiet -severity medium ./ui/...`
- `govulncheck ./ui/...`
- `env GOCACHE=/private/tmp/wander-go-cache make check`
- `bash ci/check-license-boundary.sh`
- Deslop generation 10: no identical or nearly-identical findings in the new
  OIDC handler or its tests; remaining matches are informational only.
