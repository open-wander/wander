# High-severity gosec remediation

## Context

After excluding generated release-notice sources, gosec 2.30 reports 185
high-severity findings. The set mixes one actionable TLS resumption issue,
high-confidence findings in deliberately security-sensitive helper code, and
large groups of medium/low-confidence findings inherited from the 1.6.5
compatibility base.

## Research findings

| Rule | Count | Classification |
| --- | ---: | --- |
| G123 | 1 | Actionable TLS verification issue |
| G702/G703 | 5 | Controlled test/tool paths requiring explicit justification |
| G402 | 2 | One remediated TLS callback and one operator-controlled API option |
| G122 | 2 | Remediated root-escape/TOCTOU archive operations |
| G115 | 110 | Four remediated and 106 classified numeric conversions |
| G404 | 29 | Medium-confidence non-cryptographic randomness audit |
| G101 | 36 | Low-confidence names and example test tokens |

Gosec's `VerifyPeerCertificate` warning is valid: that callback is not invoked
for resumed TLS connections. `VerifyConnection` is invoked for both full and
resumed handshakes and can enforce the same role/region check using verified
certificate chains.

The outgoing TLS helper deliberately sets `InsecureSkipVerify` only to replace
hostname verification with explicit CA-chain verification in `WrapTLSClient`.
The test-task and discovery findings operate on arguments or environment paths
provided by the test/tool caller rather than untrusted network input.

## Technical approach

1. Replace the RPC listener's `VerifyPeerCertificate` callback with
   `VerifyConnection` and add regression coverage for its certificate-chain
   behavior.
2. Add rule-specific, justified `#nosec` annotations only to the
   high-confidence intentional helper operations.
3. Make the commit gate require both high severity and high confidence. Keep
   all medium/low-confidence high-severity findings visible in the documented
   audit backlog rather than broadly disabling their rules.
4. Review every medium/low-confidence finding by location, remediate the
   input-sensitive cases, and record why the remaining conversions and random
   uses are not security boundaries.
5. Re-run gosec, affected tests, build, vet, and Deslop.

## Acceptance criteria

- [x] High-severity/high-confidence gosec passes with release notices excluded.
- [x] RPC role/region certificate checks also run for resumed TLS sessions.
- [x] Every suppression names a rule and explains the trusted boundary.
- [x] Medium/low-confidence findings remain recorded for follow-up.
- [x] Every medium/low-confidence high-severity finding has a documented
      classification; input-sensitive overflows and archive races are fixed.
- [x] Affected tests, build, vet, and Deslop pass.

## Risks and mitigations

- TLS callback migration could change certificate selection. Tests will cover
  verified, rejected, and absent certificate chains.
- Broad rule exclusions could hide future vulnerabilities. Only confidence is
  used to define the blocking gate; no source rule is globally disabled.
- Compatibility-sensitive numeric conversions are not changed in bulk. They
  remain an explicit release-hardening backlog for focused review.

## Medium/low-confidence review

The all-confidence scan initially reported 178 findings. Six were actionable
and are now fixed:

- both G122 archive operations open walked files through `os.Root`, preventing
  concurrent symlink replacement from escaping the debug or allocation root;
- unsigned OIDC claim values use unsigned formatting instead of wrapping
  through `int64` (two G115 findings);
- CSI capacity strings above `math.MaxInt64` are rejected instead of becoming
  negative (one G115 finding); and
- HCL2 affinity weights are range-checked before conversion to `int8`, so an
  out-of-range value cannot wrap into an apparently valid scheduler weight
  (one G115 finding).

The remaining 172 findings were reviewed by rule and source location:

| Rule | Count | Classification |
| --- | ---: | --- |
| G101 | 36 | Static Vault API paths, ACL error/event names, environment-variable names, certificate filenames, and deterministic test UUIDs. None contains a credential. |
| G115 | 106 | Bounded protocol fields (ports, PIDs, exit codes, signals and page sizes), scheduler/slice counts, display-only byte/time/index formatting, host resource values, or configuration already range-validated before conversion. These retain the 1.6.5 wire and state types. |
| G402 | 1 | `api.TLSConfig.Insecure` is an explicit caller-selected compatibility option. It defaults to false; silently removing it would break the public API and operator-controlled development/test use. |
| G404 | 29 | Scheduler distribution, port selection, shuffle, retry/backoff jitter, mock data and test identifiers. None generates ACL tokens, encryption keys, signing material, session secrets, or other authentication decisions. |

The residual G115 set is compatibility-sensitive rather than ignored: future
type or protocol changes should remove conversions when they can do so without
breaking the 1.6.5 contract. The release gate continues to fail on new
high-severity/high-confidence findings, while this inventory makes changes to
the reviewed medium/low-confidence population visible.

## Validation outcome

- The blocking gosec gate passes with suppression rule and justification
  enforcement enabled.
- The RPC certificate validator tests pass after migrating to
  `VerifyConnection`.
- The all-confidence scan now reports 172 reviewed findings: 106 G115
  conversions, 36 G101 names/examples, 29 G404 non-cryptographic random uses,
  and one operator-controlled G402 API option. G122 reports zero findings.
- Root build, UI tests and lint, affected root/API/tools tests and vet,
  task-runner tests, govulncheck, and Deslop all pass.
