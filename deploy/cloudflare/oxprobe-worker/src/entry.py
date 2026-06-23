# Unstable Build LLC ("COMPANY") CONFIDENTIAL
#
# Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
#
# NOTICE: All information contained herein is, and remains the property of COMPANY.
# The intellectual and technical concepts contained herein are proprietary to
# COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
# and are protected by trade secret or copyright law. Dissemination of this information
# or reproduction of this material is strictly forbidden unless prior written permission
# is obtained from COMPANY. Access to the source code contained herein is hereby
# forbidden to anyone except current COMPANY employees, managers or contractors who
# have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
#
# The copyright notice above does not evidence any actual or intended publication or
# disclosure of this source code, which includes information that is confidential and/or
# proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
# DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
# WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
# VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
# THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
# REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
# ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

import asyncio
import json
import time
from typing import Any
from urllib.parse import parse_qs, urlparse

import httpx
from workers import Response, WorkerEntrypoint


CHECK_OK = "ok"
CHECK_FAIL = "fail"
STATUS_OK = "ok"
STATUS_DEGRADED = "degraded"
STATUS_FAIL = "fail"

# Attached to every page so on-call engineers reach the oxprobe
# mitigation guide and metrics dashboard from the incident.
RUNBOOK_URL = "https://x.unstable.build/docs/runbooks/oxprobe"

ENVIRONMENTS = {
    "staging": {
        "api_host": "api.unstable.build",
        "auth_host": "dev-fv7z5qrer6vkxhxf.us.auth0.com",
        "downloads_host": "downloads.unstable.build",
        "expected_api_ip": "",
        "pkg_download_arch": "darwin-arm64",
        "oauth": {
            "token_url": "https://api.unstable.build/o/oauth2/token",
            "auth_url": "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/authorize",
            "jwks_url": "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/.well-known/jwks.json",
            "client_id": "AhY5YlLUiEjOXNFmmyw4Nve32Hp0ag22",
            "scopes": ["offline_access", "openid"],
        },
        "archs": ["darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64"],
    },
    "prod": {
        "api_host": "api.rune.build",
        "auth_host": "rune-prod.us.auth0.com",
        "downloads_host": "downloads.rune.build",
        "expected_api_ip": "",
        "pkg_download_arch": "darwin-arm64",
        "oauth": {
            "token_url": "https://api.rune.build/o/oauth2/token",
            "auth_url": "https://auth.rune.build/authorize",
            "jwks_url": "https://auth.rune.build/.well-known/jwks.json",
            "client_id": "XHBpJIm3q6PYazpxZMAhcwxAuR5Ks9B7",
            "scopes": ["offline_access", "openid"],
        },
        "archs": ["darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64"],
    },
}


class Default(WorkerEntrypoint):
    async def scheduled(self, controller, env=None, ctx=None):
        env = env if env is not None else self.env
        report = await run_worker_probe(env)
        print(json.dumps(public_report(report), separators=(",", ":")))
        if has_critical_failure(report):
            await page_failures(env, report)

    async def fetch(self, request):
        parsed = urlparse(request.url)
        query = parse_qs(parsed.query)
        report = await run_worker_probe(self.env)
        if query.get("page", ["0"])[0] == "1" and has_critical_failure(report):
            await page_failures(self.env, report)
        status = 503 if report["status"] == STATUS_FAIL else 200
        return Response.json(public_report(report), status=status)


async def run_worker_probe(env: Any) -> dict[str, Any]:
    env_name = env_value(env, "ENV", "staging")
    cfg = ENVIRONMENTS.get(env_name)
    if cfg is None:
        return aggregate(
            [
                check_result(
                    "config",
                    CHECK_FAIL,
                    True,
                    f'unknown ENV {env_name!r}; want one of staging, prod',
                )
            ]
        )

    timeout = float(env_value(env, "PROBE_TIMEOUT_SECONDS", "15"))
    probe_secret = env_value(env, "OXPROBE_SECRET", "").strip()
    async with httpx.AsyncClient(timeout=timeout, follow_redirects=True) as client:
        tasks = [
            run_check("dns_api", True, lambda: probe_dns(client, cfg)),
            run_check("oauth_config", True, lambda: probe_oauth_config(client, cfg)),
            run_check("auth0", False, lambda: probe_auth0(client, cfg)),
            run_check("downloads_cdn", False, lambda: probe_downloads(client, cfg)),
            probe_deep(client, cfg, probe_secret),
        ]
        groups = await asyncio.gather(*tasks)

    checks: list[dict[str, Any]] = []
    for group in groups:
        if isinstance(group, list):
            checks.extend(group)
        else:
            checks.append(group)
    checks.sort(key=lambda c: c["layer"])
    return aggregate(checks)


async def run_check(layer: str, critical: bool, fn) -> dict[str, Any]:
    started = time.monotonic()
    try:
        detail = await fn()
        return check_result(layer, CHECK_OK, critical, detail, started)
    except Exception as exc:
        return check_result(layer, CHECK_FAIL, critical, str(exc), started)


def check_result(
    layer: str,
    status: str,
    critical: bool,
    detail: str = "",
    started: float | None = None,
) -> dict[str, Any]:
    latency_ms = 0
    if started is not None:
        latency_ms = int((time.monotonic() - started) * 1000)
    res: dict[str, Any] = {
        "layer": layer,
        "status": status,
        "latency_ms": latency_ms,
        "critical": critical,
    }
    if detail:
        res["detail"] = detail
    return res


def aggregate(checks: list[dict[str, Any]]) -> dict[str, Any]:
    status = STATUS_OK
    for check in checks:
        if check["status"] != CHECK_FAIL:
            continue
        if check.get("critical", False):
            return {"status": STATUS_FAIL, "checks": checks}
        status = STATUS_DEGRADED
    return {"status": status, "checks": checks}


def has_critical_failure(report: dict[str, Any]) -> bool:
    return report["status"] == STATUS_FAIL


async def probe_dns(client: httpx.AsyncClient, cfg: dict[str, Any]) -> str:
    host = cfg["api_host"]
    resp = await client.get(
        "https://cloudflare-dns.com/dns-query",
        params={"name": host, "type": "A"},
        headers={"accept": "application/dns-json"},
    )
    if resp.status_code != 200:
        raise RuntimeError(f"doh status {resp.status_code}")
    doc = resp.json()
    answers = [
        a.get("data")
        for a in doc.get("Answer", [])
        if a.get("type") == 1 and a.get("data")
    ]
    if not answers:
        raise RuntimeError(f"{host} returned no A records")
    expected = cfg.get("expected_api_ip", "")
    if expected and expected not in answers:
        raise RuntimeError(f"{host} resolved to {answers}, want {expected}")
    return f"{host} resolved to {', '.join(answers)}"


async def probe_oauth_config(client: httpx.AsyncClient, cfg: dict[str, Any]) -> str:
    resp = await client.get(f"https://{cfg['api_host']}/o/oauth2/config")
    if resp.status_code != 200:
        raise RuntimeError(f"config status {resp.status_code}")
    doc = resp.json()
    endpoint = get_any(doc, "Endpoint", "endpoint") or {}
    expected = cfg["oauth"]
    mismatches = []
    compare(mismatches, "token_url", expected["token_url"], get_any(endpoint, "TokenURL", "token_url"))
    compare(mismatches, "auth_url", expected["auth_url"], get_any(endpoint, "AuthURL", "auth_url"))
    compare(mismatches, "jwks_url", expected["jwks_url"], get_any(doc, "JWKSURL", "jwks_url"))
    compare(mismatches, "client_id", expected["client_id"], get_any(doc, "ClientID", "client_id"))
    got_scopes = get_any(doc, "Scopes", "scopes") or []
    if sorted(got_scopes) != sorted(expected["scopes"]):
        mismatches.append(f"scopes={got_scopes!r} want {expected['scopes']!r}")
    if mismatches:
        raise RuntimeError("oauth config mismatch: " + "; ".join(mismatches))
    return "config matches matrix"


async def probe_auth0(client: httpx.AsyncClient, cfg: dict[str, Any]) -> str:
    auth_host = cfg["auth_host"]
    resp = await client.get(f"https://{auth_host}/.well-known/openid-configuration")
    if resp.status_code != 200:
        raise RuntimeError(f"discovery status {resp.status_code}")
    doc = resp.json()
    if "openid" not in doc.get("scopes_supported", []):
        raise RuntimeError("discovery missing openid scope")
    if "RS256" not in doc.get("id_token_signing_alg_values_supported", []):
        raise RuntimeError("discovery missing RS256 signing alg")

    token = await client.post(
        f"https://{auth_host}/oauth/token",
        data={
            "grant_type": "authorization_code",
            "code": "oxprobe-bogus-code",
            "redirect_uri": "https://oxprobe.invalid/callback",
            "client_id": "oxprobe",
        },
        headers={"content-type": "application/x-www-form-urlencoded"},
    )
    if token.status_code not in (400, 401):
        raise RuntimeError(f"token endpoint returned {token.status_code} for bogus code, want 400/401")
    return "tenant healthy"


async def probe_downloads(client: httpx.AsyncClient, cfg: dict[str, Any]) -> str:
    for arch in cfg["archs"]:
        resp = await client.get(
            f"https://{cfg['downloads_host']}/{arch}/manifest.json",
            headers={"range": "bytes=0-0"},
        )
        if cfg.get("downloads_private", False) and resp.status_code == 403:
            continue
        if resp.status_code not in (200, 206):
            raise RuntimeError(f"{arch} manifest status {resp.status_code}")
    return f"{len(cfg['archs'])} manifests reachable"


async def probe_deep(
    client: httpx.AsyncClient,
    cfg: dict[str, Any],
    probe_secret: str,
) -> list[dict[str, Any]]:
    started = time.monotonic()
    if not probe_secret:
        return [check_result("deep", CHECK_FAIL, True, "missing OXPROBE_SECRET", started)]
    try:
        resp = await client.get(
            f"https://{cfg['api_host']}/health?verbose=1",
            headers={"accept": "application/json", "x-probe-secret": probe_secret},
        )
        content_type = resp.headers.get("content-type", "")
        if not content_type.startswith("application/json"):
            raise RuntimeError(
                f'deep health returned no report (status {resp.status_code}, content-type {content_type!r}); check probe secret'
            )
        report = resp.json()
    except Exception as exc:
        return [check_result("deep", CHECK_FAIL, True, str(exc), started)]

    checks = []
    for check in report.get("checks", []):
        fanned = {
            "layer": "deep." + str(check.get("layer", "unknown")),
            "status": check.get("status", CHECK_FAIL),
            "latency_ms": int(check.get("latency_ms", 0)),
            "critical": False,
        }
        if check.get("detail"):
            fanned["detail"] = check["detail"]
        checks.append(fanned)

    if report.get("status") == STATUS_FAIL:
        checks.append(check_result("deep", CHECK_FAIL, True, "server status fail", started))
    elif report.get("status") == STATUS_DEGRADED:
        checks.append(check_result("deep", CHECK_FAIL, False, "server status degraded", started))

    checks.append(await probe_pkg_download(client, cfg, report))
    return checks


async def probe_pkg_download(
    client: httpx.AsyncClient,
    cfg: dict[str, Any],
    report: dict[str, Any],
) -> dict[str, Any]:
    started = time.monotonic()
    arch = cfg.get("pkg_download_arch", "")
    signed = report.get("signed_downloads", {}).get(arch, "")
    if not signed:
        return check_result(
            "pkg_download", CHECK_FAIL, True,
            f"no signed download url for {arch} in /health report", started,
        )
    try:
        resp = await client.get(signed)
        if resp.status_code != 200:
            raise RuntimeError(f"{arch} signed download status {resp.status_code}")
        if not resp.content:
            raise RuntimeError(f"{arch} signed download returned zero bytes")
    except Exception as exc:
        return check_result("pkg_download", CHECK_FAIL, True, str(exc), started)
    return check_result(
        "pkg_download", CHECK_OK, True,
        f"downloaded {arch} ({len(resp.content)} bytes)", started,
    )


async def page_failures(env: Any, report: dict[str, Any]) -> None:
    routing_key = env_value(env, "PAGERDUTY_ROUTING_KEY", "").strip()
    if not routing_key:
        raise RuntimeError("missing PAGERDUTY_ROUTING_KEY")
    env_name = env_value(env, "ENV", "staging")
    async with httpx.AsyncClient(timeout=10) as client:
        for check in report["checks"]:
            if check["status"] != CHECK_FAIL or not check.get("critical", False):
                continue
            detail = check.get("detail", "failed")
            payload = {
                "routing_key": routing_key,
                "event_action": "trigger",
                "dedup_key": f"oxprobe-cloudflare-{env_name}-{check['layer']}",
                "payload": {
                    "summary": f"oxprobe cloudflare {env_name}: {check['layer']} failing — {detail}",
                    "source": "oxprobe-cloudflare",
                    "severity": "critical",
                    "component": check["layer"],
                    "group": env_name,
                    "class": "synthetic-probe",
                    "custom_details": {
                        "env": env_name,
                        "layer": check["layer"],
                        "detail": detail,
                        "latency_ms": check.get("latency_ms", 0),
                    },
                },
                "links": [{"href": RUNBOOK_URL, "text": "oxprobe runbook"}],
            }
            resp = await client.post("https://events.pagerduty.com/v2/enqueue", json=payload)
            if resp.status_code < 200 or resp.status_code >= 300:
                raise RuntimeError(f"pagerduty status {resp.status_code}: {resp.text[:200]}")


def public_report(report: dict[str, Any]) -> dict[str, Any]:
    return {
        "status": report["status"],
        "checks": [
            {k: v for k, v in check.items() if k != "critical"}
            for check in report["checks"]
        ],
    }


def env_value(env: Any, name: str, default: str) -> str:
    try:
        value = getattr(env, name)
    except Exception:
        return default
    if value is None:
        return default
    return str(value)


def get_any(obj: dict[str, Any], *names: str) -> Any:
    for name in names:
        if name in obj:
            return obj[name]
    return None


def compare(mismatches: list[str], name: str, want: str, got: Any) -> None:
    if want and got != want:
        mismatches.append(f"{name}={got!r} want {want!r}")
