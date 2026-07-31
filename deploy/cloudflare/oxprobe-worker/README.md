<!--
Unstable Build LLC ("COMPANY") CONFIDENTIAL

Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

NOTICE: All information contained herein is, and remains the property of COMPANY.
The intellectual and technical concepts contained herein are proprietary to
COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
and are protected by trade secret or copyright law. Dissemination of this information
or reproduction of this material is strictly forbidden unless prior written permission
is obtained from COMPANY. Access to the source code contained herein is hereby
forbidden to anyone except current COMPANY employees, managers or contractors who
have executed Confidentiality and Non-disclosure agreements explicitly covering such access.

The copyright notice above does not evidence any actual or intended publication or
disclosure of this source code, which includes information that is confidential and/or
proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
-->

# oxprobe Cloudflare Worker

This worker runs the Cloudflare-compatible subset of `cmd/oxprobe` from
Cloudflare Cron Triggers. It exists as an external vantage point: if Google Cloud
or Cloud Run is impaired, Cloudflare can still run the synthetic probe and page
PagerDuty.

The worker intentionally stores runtime secrets in Cloudflare Worker secrets, not
Google Secret Manager.

## Probes

Implemented:

- `dns_api` via Cloudflare DNS-over-HTTPS
- `oauth_config`
- `auth0`
- `downloads_cdn` via range GETs for each manifest
- `deep` `/health?verbose=1` with `X-Probe-Secret`, fanned into
  `deep.<layer>` results
- `pkg_download` (critical): fetches the signed `ada` download URL that ox-api
  mints in the verbose `/health` report (`signed_downloads[<arch>]`) and does an
  anonymous GET, proving the full `pkg install ada` path. ox-api owns signing;
  the worker only verifies the signed URL is usable.
- `pkg_latest` (critical): for every package we own, on every arch, resolves the
  advertised `Latest` to a bundle download URL and range-GETs the artifact. The
  release index, the bundle record, and the bucket object are owned by different
  systems, so a `Latest` pointer can outlive its artifact — a publish that
  registers bundle metadata but never uploads the tarball breaks
  `pkg install <name>` on that arch while every other layer stays green. The
  check reports every broken arch/package pair, not just the first.

  The owned set is **discovered per run**, not pinned, using the same rules as
  `deploy/package_versions.sh`: list each arch's registry, drop the tree-sitter
  grammar packages (`Metadata.language == "true"`), keep the toolchains in
  `OWNED_LANGUAGES`, and skip packages with no `Latest` for that arch. Only the
  darwin-arm64 registry carries the language marker, so the marker is unioned
  across archs before filtering — without that, the unmarked archs pull ~300
  grammars into the set. A pinned list would silently stop covering packages
  published after it was written.

  Both implementations fetch the artifact with a ranged `GET`
  (`Range: bytes=0-0`) and read a single byte: that proves the bucket object is
  readable without paying egress for a multi-hundred-MB artifact every minute.
  `HEAD` is not an option because the URL is signed for `GET`. `cmd/oxprobe`
  still lists packages through `cdnrelease.Manager`, the client the editor
  installs with; workers cannot use that Go client, so this file reimplements
  the same URL shape, the same list/bundle/artifact legs, and the same failure
  wording. Keep the two in step when either changes.

The ox-api report deliberately omits each layer's `critical` flag from JSON. If
the server reports overall `fail`, the worker adds a critical synthetic `deep`
failure so PagerDuty still fires without guessing the criticality of individual
server-side checks.

Deferred:

- raw TLS certificate-expiry probing. Workers do not expose raw sockets or the
  peer certificate from `fetch()`.
- gRPC, matching the RFC003 deferral.

## Deploy

The repository root Makefile deploys both probe implementations for an
environment:

```bash
make oxprobe-deploy-staging
make oxprobe-deploy-prod
```

To deploy only this Cloudflare Worker from the repository root:

```bash
make oxprobe-worker-deploy-staging
make oxprobe-worker-deploy-prod
```

Install the Python Worker tooling once:

```bash
uv tool install workers-py
```

From this directory:

```bash
uv run pywrangler secret put OXPROBE_SECRET --env staging
uv run pywrangler secret put PAGERDUTY_ROUTING_KEY --env staging
uv run pywrangler deploy --env staging
```

For prod, set separate Cloudflare secrets and deploy the prod environment:

```bash
uv run pywrangler secret put OXPROBE_SECRET --env prod
uv run pywrangler secret put PAGERDUTY_ROUTING_KEY --env prod
uv run pywrangler deploy --env prod
```

## Local/manual test

```bash
uv run pywrangler dev --test-scheduled
curl 'http://localhost:8787/cdn-cgi/handler/scheduled?cron=*/1+*+*+*+*'
curl 'http://localhost:8787/?page=0'
```

Manual HTTP responses return the JSON report. Add `?page=0` to suppress
PagerDuty while debugging.
