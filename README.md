# Sub2Clash

Sub2Clash is a commercial-friendly, self-hosted subscription bridge for teams and operators who need to turn client-specific subscription links into Clash-compatible managed profiles.

It is built for cases where a provider link works in clients such as Shadowrocket, but does not directly work in Clash Verge or other Clash-based tools.

## What this tool does

Sub2Clash accepts an upstream subscription URL, fetches it with provider-aware strategies, parses the supported node formats, and emits:

- a stable managed Clash subscription URL
- a downloadable Clash YAML file
- optional subscription usage / expiry metadata when it can be derived safely

It is designed so users can keep sensitive subscription links inside their own machine or self-hosted environment.

## Who this is for

Sub2Clash is useful for:

- **VPN operators** who want a repeatable bridge from upstream subscriptions to Clash-compatible output
- **consultants and migration teams** moving users from mixed mobile clients to Clash Verge
- **internal tooling teams** who need a privacy-preserving managed subscription layer
- **support teams** who want a reusable SOP instead of one-off manual conversions

## Business and operational value

Sub2Clash helps teams:

- reduce manual subscription conversion work
- improve compatibility across Shadowrocket-style and Clash-style ecosystems
- keep sensitive upstream subscription links off end-user devices
- standardize onboarding and troubleshooting with a documented workflow
- offer a reusable, productized SOP that can support commercial service delivery

## What v1 supports

- Self-hosted deployment only
- Single-admin model
- CLI and web/API in one binary
- Managed Clash subscription URLs plus downloadable YAML
- Encrypted storage of upstream subscription URLs
- Background refresh every 24 hours by default

### Supported input families

- direct Clash YAML
- base64 subscription bundles
- mixed URI text with banner lines such as `STATUS=...`

### Supported node formats in v1

- `ss`
- `ssr`
- `vmess`
- `vless`
- `trojan`
- `hysteria2`
- `tuic`

Unsupported schemes are skipped with warnings instead of crashing the conversion.

## How the workflow works

1. A user provides an upstream subscription URL to the CLI or the admin web/API.
2. Sub2Clash tries a safe sequence of fetch strategies:
   - original URL
   - provider-specific suffixes such as `flag=clash`, `flag=shadowrocket`, `sub=1`, `clash=1`
   - user-agent fallback, including `Shadowrocket` when needed
3. The fetched body is normalized:
   - direct Clash YAML is parsed directly
   - base64 bundles are decoded
   - banner lines such as `STATUS=...` are separated from actual nodes
4. Supported URIs are parsed into a native proxy model.
5. Sub2Clash generates a Clash YAML profile in block style with:
   - `PROXY`
   - `AUTO`
   - `DIRECT`
   - `MATCH,PROXY`
6. The generated profile is stored and exposed through:
   - a managed Clash URL for Clash Verge auto-update
   - a downloadable YAML file
7. On the next refresh cycle, the last successful fetch strategy is tried first.

## Installation

### Option 1: Run the server with Docker

```bash
docker run --rm -p 8080:8080 \
  -e SUB2CLASH_ADMIN_TOKEN='example-admin-token' \
  -e SUB2CLASH_ENCRYPTION_KEY='sub2clash-dev-secret-material-01' \
  -e SUB2CLASH_PUBLIC_BASE_URL='http://127.0.0.1:8080' \
  ghcr.io/bozliu/sub2clash:latest
```

### Option 2: Build locally

```bash
git clone https://github.com/bozliu/sub2clash.git
cd sub2clash
go build ./cmd/sub2clash
```

## Required environment variables

- `SUB2CLASH_ADMIN_TOKEN`  
  Required for admin APIs and the admin web UI

- `SUB2CLASH_ENCRYPTION_KEY`  
  Required for stored profiles. Use a 32-byte raw string, base64 string, or 64-character hex string.

### Optional environment variables

- `SUB2CLASH_ADDR`  
  Default: `:8080`

- `SUB2CLASH_DATA_DIR`  
  Default: `~/.local/share/sub2clash`

- `SUB2CLASH_PUBLIC_BASE_URL`  
  Public base used in returned managed URLs

- `SUB2CLASH_UPSTREAM_PROXY`  
  Optional upstream HTTP/SOCKS proxy for environments that require it

## CLI usage

### One-shot conversion

```bash
sub2clash convert \
  --url 'https://provider.example/path/to/your-redacted-subscription-url' \
  --out ./demo-clash.yaml
```

### Add a managed profile

```bash
sub2clash profile add \
  --name 'Example Provider' \
  --url 'https://provider.example/path/to/your-redacted-subscription-url' \
  --refresh 24h
```

### Refresh a managed profile

```bash
sub2clash profile refresh prf_exampleid
```

### List profiles

```bash
sub2clash profile list
```

### Run the web/API service

```bash
SUB2CLASH_ADMIN_TOKEN='example-admin-token' \
SUB2CLASH_ENCRYPTION_KEY='sub2clash-dev-secret-material-01' \
SUB2CLASH_PUBLIC_BASE_URL='http://127.0.0.1:8080' \
sub2clash serve
```

## HTTP API

### Create a profile

```bash
curl -X POST http://127.0.0.1:8080/api/profiles \
  -H 'Authorization: Bearer example-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Example Provider",
    "url": "https://provider.example/path/to/your-redacted-subscription-url",
    "refresh_interval": "24h"
  }'
```

### Refresh a profile

```bash
curl -X POST http://127.0.0.1:8080/api/profiles/prf_exampleid/refresh \
  -H 'Authorization: Bearer example-admin-token'
```

### Use the managed Clash URL in Clash Verge

After profile creation, import the returned `managed_url` in Clash Verge as a remote profile.

## Admin web UI

Start the server and open the root page in your browser:

```text
http://127.0.0.1:8080/
```

Paste the admin token into the page, create a profile, and copy the managed Clash URL into Clash Verge.

## Privacy and security model

- upstream subscription URLs are encrypted at rest
- admin APIs require `SUB2CLASH_ADMIN_TOKEN`
- public profile URLs are read-only and unguessable
- the repo uses only redacted examples
- the project intentionally avoids bundling GPL-only conversion engines in order to stay commercially friendly

## Limitations

- v1 is self-hosted only
- v1 is single-admin, not multi-tenant SaaS
- some providers may still rely on client-specific quirks that require additional fetch rules
- unsupported URI schemes are skipped with warnings instead of converted

## Troubleshooting

### The link works in Shadowrocket but not in Clash

This is one of the main use cases for Sub2Clash. Providers sometimes gate responses on:

- user-agent
- query suffixes such as `flag=shadowrocket`
- mixed base64 bundle formats with metadata banners

Sub2Clash tries these strategies automatically.

### Clash Verge does not show the new profile immediately

Restart Clash Verge or refresh its remote profiles after creating a managed profile.

### A provider returns nodes but some are missing

Check warnings from:

- `sub2clash convert`
- `sub2clash profile refresh`
- `POST /api/profiles/:id/refresh`

This usually means the provider returned an unsupported scheme or malformed node.

## CI/CD

This repo includes:

- PR CI for format checks, tests, build, Docker smoke test, secret scan, and dependency/license checks
- tag-based releases for cross-platform binaries
- GHCR Docker image publishing

## Commercial use

Sub2Clash is built so operators and consultants can reuse it as a productized SOP:

- onboard upstream subscriptions
- standardize conversion behavior
- expose managed Clash links to downstream users
- reduce repeated support work across teams and clients

It is especially valuable when the business goal is operational consistency rather than one-off manual conversion.
