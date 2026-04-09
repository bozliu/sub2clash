# Security Policy

## Supported versions

Sub2Clash follows the latest tagged release on `main`.

## Reporting a vulnerability

Please do not open public issues for sensitive security reports.

Report privately through GitHub Security Advisories or contact the maintainer directly through GitHub.

Include:

- affected version or commit
- reproduction steps
- impact assessment
- whether sensitive subscription material is exposed

## Sensitive data handling

Sub2Clash is designed to minimize subscription exposure:

- upstream subscription URLs are encrypted at rest
- managed profile endpoints are read-only and unguessable
- example docs must use fake or redacted data

Operators are still responsible for:

- protecting `SUB2CLASH_ADMIN_TOKEN`
- rotating `SUB2CLASH_ENCRYPTION_KEY` securely
- securing their self-hosted deployment and logs
