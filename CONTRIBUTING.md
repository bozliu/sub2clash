# Contributing to Sub2Clash

Thanks for helping improve Sub2Clash.

## Development workflow

1. Fork the repo or create a feature branch prefixed with `boz1iu/`.
2. Set the required environment variables:
   - `SUB2CLASH_ENCRYPTION_KEY`
   - `SUB2CLASH_ADMIN_TOKEN`
3. Run tests with `go test ./...`.
4. Run formatting checks with `gofmt -l .`.
5. Open a pull request with:
   - problem statement
   - implementation summary
   - test evidence
   - security/privacy impact if relevant

## Ground rules

- Never commit real subscription URLs, tokens, API keys, cookies, logs, or screenshots.
- Use only redacted or synthetic test fixtures.
- Preserve the self-hosted privacy model.
- Prefer additive changes with tests over silent behavioral changes.

## Scope for v1

- Supported subscription outputs target Clash-compatible YAML.
- Unsupported upstream schemes should degrade into warnings, not panics.
- Admin APIs must stay authenticated; managed profile URLs must stay read-only.
