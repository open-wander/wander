# Wander Security Policy

## Supported versions

Security fixes are provided for the latest stable Wander 0.1.x release. A
release candidate may receive a fix before the stable release, but previews
and superseded release candidates are not supported production versions.

| Version | Supported |
| --- | --- |
| Latest 0.1.x release | Yes |
| Current 0.1.0 release candidate | Until superseded |
| Preview releases and older candidates | No |

## Report a vulnerability privately

Do not open a public issue for a suspected vulnerability or include secrets,
tokens, exploit details, or affected deployment information in an issue or
discussion.

Use [GitHub private vulnerability reporting][private-report] to send the
Wander maintainers a confidential report. Include, where possible:

- the affected Wander version and deployment platform;
- the security impact and prerequisites;
- reproducible steps or a minimal proof of concept;
- relevant configuration with credentials and private data removed; and
- any known mitigations or disclosure constraints.

If GitHub cannot accept the report, open a public issue containing no sensitive
details and ask a maintainer to establish a private channel. Do not describe
the vulnerability in that issue.

## Response process

The maintainers aim to:

- acknowledge a private report within three business days;
- provide an initial assessment within seven business days;
- share progress at least every 14 days while remediation is active; and
- coordinate a release and public advisory when a fix or mitigation is ready.

These are response targets rather than guarantees. Complex reports, incomplete
reproductions, dependency coordination, or maintainer availability can change
the timeline. The private advisory remains the authoritative communication
channel for the report.

## Coordinated disclosure

Please keep the report confidential until the maintainers and reporter agree
that users have had a reasonable opportunity to apply a fix or mitigation.
The advisory will describe affected versions, impact, remediation, and any
necessary upgrade instructions. Reporter credit is optional and will follow
the reporter's preference.

[private-report]: https://github.com/open-wander/wander/security/advisories/new
