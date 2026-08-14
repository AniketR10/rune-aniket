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

import importlib.util
import json
import sys
import types
import unittest
from pathlib import Path


class response:
    @staticmethod
    def json(obj, status=200):
        return {"status": status, "body": obj}


class worker_entrypoint:
    pass


workers = types.ModuleType("workers")
workers.Response = response
workers.WorkerEntrypoint = worker_entrypoint
sys.modules.setdefault("workers", workers)

entry_path = Path(__file__).parents[1] / "src" / "entry.py"
spec = importlib.util.spec_from_file_location("entry", entry_path)
entry = importlib.util.module_from_spec(spec)
spec.loader.exec_module(entry)


class fake_response:
    def __init__(self, status_code=200, body=None, headers=None, content=b""):
        self.status_code = status_code
        self._body = body or {}
        self.headers = headers or {}
        self.text = str(self._body)
        self.content = content

    def json(self):
        return self._body


class fake_client:
    def __init__(self, response):
        self.response = response
        self.get_calls = []

    async def get(self, url, **kwargs):
        self.get_calls.append((url, kwargs))
        return self.response


class fake_paging_client:
    def __init__(self, response):
        self.response = response
        self.post_calls = []

    async def post(self, url, **kwargs):
        self.post_calls.append((url, kwargs))
        return self.response

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False


class fake_release_client:
    """Serves the release API surface probe_pkg_latest walks, plus the
    bucket its signed URLs point at, so a missing artifact is reproduced
    exactly as GCS reports it.

    index maps arch -> pkg -> {"latest", "language", "bundles", "blobs",
    "empty"}, where listing a version under "blobs" implies a bundle row
    for it.
    """

    def __init__(self, index):
        self.index = index
        self.get_calls = []
        self.downloaded = []

    async def get(self, url, **kwargs):
        self.get_calls.append((url, kwargs))
        parts = url.split("://", 1)[1].split("/", 1)[1].strip("/").split("/")

        if parts[0] == "blob":
            _, arch, pkg, version = parts
            state = self.index.get(arch, {}).get(pkg)
            if state is None or version not in state.get("blobs", []):
                return fake_response(status_code=404)
            self.downloaded.append(f"{arch}/{pkg}")
            if state.get("empty", False):
                return fake_response(status_code=206, content=b"")
            return fake_response(status_code=206, content=b"t")

        arch = parts[2]
        pkgs = self.index.get(arch)
        if pkgs is None:
            return fake_response(status_code=404)
        if len(parts) == 4:
            return fake_response(
                body=[
                    {
                        "Name": name,
                        "Latest": st.get("latest", ""),
                        "Metadata": (
                            {"language": "true"} if st.get("language") else {}
                        ),
                    }
                    for name, st in sorted(pkgs.items())
                ]
            )
        pkg = parts[4]
        state = pkgs.get(pkg)
        if state is None:
            return fake_response(status_code=404)
        version = parts[6]
        known = list(state.get("bundles", [])) + list(state.get("blobs", []))
        if version not in known:
            return fake_response(status_code=404)
        return fake_response(
            body={"url": f"https://cdn.example/blob/{arch}/{pkg}/{version}"}
        )


class fake_env:
    def __init__(self, **values):
        for name, value in values.items():
            setattr(self, name, value)


def published(version, language=False):
    return {"latest": version, "blobs": [version], "language": language}


class fake_probe:
    """A zero-argument probe factory whose first fail_until runs fail."""

    def __init__(self, layer, critical, fail_until=0):
        self.layer = layer
        self.critical = critical
        self.fail_until = fail_until
        self.runs = 0

    def __call__(self):
        return self._run()

    async def _run(self):
        self.runs += 1
        if self.runs <= self.fail_until:
            return [
                entry.check_result(self.layer, entry.CHECK_FAIL, self.critical, "flaky")
            ]
        return [entry.check_result(self.layer, entry.CHECK_OK, self.critical)]


class EntryTest(unittest.IsolatedAsyncioTestCase):
    async def confirm(self, probes, delay):
        """Runs probes once and confirms, with the delay awaited but not slept."""
        slept = []
        old_sleep = entry.asyncio.sleep
        entry.asyncio.sleep = lambda seconds: slept.append(seconds) or old_sleep(0)
        try:
            groups = list(await entry.asyncio.gather(*(probe() for probe in probes)))
            groups, unconfirmed = await entry.confirm_failures(probes, groups, delay)
        finally:
            entry.asyncio.sleep = old_sleep
        return groups, unconfirmed, slept

    async def test_confirm_reruns_only_the_failing_critical_probe(self):
        dns = fake_probe("dns_api", True, fail_until=1)
        auth0 = fake_probe("auth0", False)

        groups, unconfirmed, slept = await self.confirm([dns, auth0], 30.0)

        self.assertEqual([30.0], slept)
        self.assertEqual(2, dns.runs)
        self.assertEqual(1, auth0.runs)
        self.assertEqual(["dns_api"], unconfirmed)
        self.assertEqual(entry.STATUS_OK, entry.aggregate(groups[0] + groups[1])["status"])

    async def test_confirm_keeps_persistent_critical_failure(self):
        dns = fake_probe("dns_api", True, fail_until=2)

        groups, unconfirmed, _ = await self.confirm([dns], 30.0)

        self.assertEqual(2, dns.runs)
        self.assertEqual([], unconfirmed)
        self.assertTrue(entry.has_critical_failure(entry.aggregate(groups[0])))

    async def test_confirm_skips_noncritical_failure(self):
        auth0 = fake_probe("auth0", False, fail_until=1)

        groups, unconfirmed, slept = await self.confirm([auth0], 30.0)

        self.assertEqual([], slept)
        self.assertEqual(1, auth0.runs)
        self.assertEqual([], unconfirmed)
        self.assertEqual(entry.STATUS_DEGRADED, entry.aggregate(groups[0])["status"])

    async def test_confirm_disabled_by_zero_delay(self):
        dns = fake_probe("dns_api", True, fail_until=1)

        groups, unconfirmed, slept = await self.confirm([dns], 0)

        self.assertEqual([], slept)
        self.assertEqual(1, dns.runs)
        self.assertEqual([], unconfirmed)
        self.assertTrue(entry.has_critical_failure(entry.aggregate(groups[0])))

    async def test_confirm_skipped_when_healthy(self):
        dns = fake_probe("dns_api", True)

        _, unconfirmed, slept = await self.confirm([dns], 30.0)

        self.assertEqual([], slept)
        self.assertEqual(1, dns.runs)
        self.assertEqual([], unconfirmed)

    async def test_scheduled_accepts_cloudflare_runtime_arguments(self):
        calls = []
        old_run_worker_probe = entry.run_worker_probe
        old_reconcile_pages = entry.reconcile_pages
        try:
            async def fake_run_worker_probe(env):
                calls.append(env)
                return {"status": entry.STATUS_OK, "checks": []}

            async def fake_reconcile_pages(env, report):
                return None

            entry.run_worker_probe = fake_run_worker_probe
            entry.reconcile_pages = fake_reconcile_pages
            env = object()

            await entry.Default().scheduled(object(), env, object())

            self.assertEqual([env], calls)
        finally:
            entry.run_worker_probe = old_run_worker_probe
            entry.reconcile_pages = old_reconcile_pages

    def test_aggregate_degrades_on_noncritical_failure(self):
        report = entry.aggregate(
            [
                entry.check_result("dns_api", entry.CHECK_OK, True),
                entry.check_result("downloads_cdn", entry.CHECK_FAIL, False, "403"),
            ]
        )

        self.assertEqual(entry.STATUS_DEGRADED, report["status"])
        self.assertFalse(entry.has_critical_failure(report))

    def test_aggregate_fails_on_critical_failure(self):
        report = entry.aggregate(
            [entry.check_result("oauth_config", entry.CHECK_FAIL, True, "bad config")]
        )

        self.assertEqual(entry.STATUS_FAIL, report["status"])
        self.assertTrue(entry.has_critical_failure(report))

    def test_public_report_drops_critical_flag(self):
        report = entry.aggregate([entry.check_result("dns_api", entry.CHECK_OK, True)])

        public = entry.public_report(report)

        self.assertNotIn("critical", public["checks"][0])

    async def test_oauth_config_accepts_go_json_shape(self):
        cfg = entry.ENVIRONMENTS["staging"]
        client = fake_client(
            fake_response(
                body={
                    "JWKSURL": cfg["oauth"]["jwks_url"],
                    "ClientID": cfg["oauth"]["client_id"],
                    "Scopes": cfg["oauth"]["scopes"],
                    "Endpoint": {
                        "AuthURL": cfg["oauth"]["auth_url"],
                        "TokenURL": cfg["oauth"]["token_url"],
                    },
                }
            )
        )

        detail = await entry.probe_oauth_config(client, cfg)

        self.assertEqual("config matches matrix", detail)

    async def test_downloads_uses_range_get(self):
        cfg = {"downloads_host": "downloads.example.com", "archs": ["darwin-arm64"]}
        client = fake_client(fake_response())

        detail = await entry.probe_downloads(client, cfg)

        self.assertEqual("1 manifests reachable", detail)
        self.assertEqual(
            [
                (
                    "https://downloads.example.com/darwin-arm64/manifest.json",
                    {"headers": {"range": "bytes=0-0"}},
                )
            ],
            client.get_calls,
        )

    async def test_downloads_allows_forbidden_for_private_host(self):
        cfg = {
            "downloads_host": "downloads.example.com",
            "downloads_private": True,
            "archs": ["darwin-arm64"],
        }
        client = fake_client(fake_response(status_code=403))

        detail = await entry.probe_downloads(client, cfg)

        self.assertEqual("1 manifests reachable", detail)

    async def test_downloads_rejects_forbidden_for_public_host(self):
        cfg = {"downloads_host": "downloads.example.com", "archs": ["darwin-arm64"]}
        client = fake_client(fake_response(status_code=403))

        with self.assertRaisesRegex(RuntimeError, "darwin-arm64 manifest status 403"):
            await entry.probe_downloads(client, cfg)

    async def test_pkg_latest_checks_owned_packages_and_skips_grammars(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {
                "darwin-arm64": {
                    "rune-agent": published("v1.1.2"),
                    "rg": published("v1.0.0"),
                    "go": published("v1.26.5", language=True),
                    "ada": published("v0.0.1", language=True),
                    "zig": published("v0.0.1", language=True),
                }
            }
        )

        detail = await entry.probe_pkg_latest(client, cfg)

        self.assertEqual("3 latest versions downloadable (3 packages, 1 archs)", detail)
        self.assertEqual(
            ["darwin-arm64/go", "darwin-arm64/rg", "darwin-arm64/rune-agent"],
            sorted(client.downloaded),
        )

    async def test_pkg_latest_grammar_marker_on_one_arch_filters_every_arch(self):
        # Only darwin-arm64 marks grammars, so an arch-local rule would
        # drag every grammar on the other archs into the owned set.
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64", "linux-amd64"]}
        client = fake_release_client(
            {
                "darwin-arm64": {
                    "ada": published("v0.0.1", language=True),
                    "rune-agent": published("v1.1.2"),
                },
                "linux-amd64": {
                    "ada": published("v0.0.1"),
                    "rune-agent": published("v1.1.2"),
                },
            }
        )

        await entry.probe_pkg_latest(client, cfg)

        self.assertEqual(
            ["darwin-arm64/rune-agent", "linux-amd64/rune-agent"],
            sorted(client.downloaded),
        )

    async def test_pkg_latest_rejects_bundle_whose_artifact_is_missing(self):
        # The rune-agent v1.1.2 regression: the bundle row exists and the
        # API mints a signed URL, but the object is not in the bucket.
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {"darwin-arm64": {"rune-agent": {"latest": "v1.1.2", "bundles": ["v1.1.2"]}}}
        )

        with self.assertRaisesRegex(
            RuntimeError,
            r"1/1 latest versions not downloadable: "
            r"darwin-arm64/rune-agent: v1\.1\.2: artifact: status 404",
        ):
            await entry.probe_pkg_latest(client, cfg)

    async def test_pkg_latest_rejects_latest_without_a_bundle(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {"darwin-arm64": {"rune-agent": {"latest": "v9.9.9", "blobs": ["v1.1.0"]}}}
        )

        with self.assertRaisesRegex(RuntimeError, r"v9\.9\.9: bundle: status 404"):
            await entry.probe_pkg_latest(client, cfg)

    async def test_pkg_latest_skips_package_without_latest(self):
        # package_versions.sh skips these rather than reporting them: not
        # publishing a package for an arch is routine.
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {"darwin-arm64": {"rune-agent": published("v1.1.2"), "runectl": {}}}
        )

        detail = await entry.probe_pkg_latest(client, cfg)

        self.assertEqual("1 latest versions downloadable (1 packages, 1 archs)", detail)
        self.assertEqual(["darwin-arm64/rune-agent"], client.downloaded)

    async def test_pkg_latest_rejects_registry_with_nothing_installable(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {"darwin-arm64": {"ada": published("v0.0.1", language=True)}}
        )

        with self.assertRaisesRegex(
            RuntimeError, "no owned package advertises a Latest version"
        ):
            await entry.probe_pkg_latest(client, cfg)

    async def test_pkg_latest_rejects_unreachable_registry(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}

        with self.assertRaisesRegex(RuntimeError, "list darwin-arm64: status 404"):
            await entry.probe_pkg_latest(fake_release_client({}), cfg)

    async def test_pkg_latest_rejects_empty_artifact(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {
                "darwin-arm64": {
                    "rune-agent": {
                        "latest": "v1.1.2",
                        "blobs": ["v1.1.2"],
                        "empty": True,
                    }
                }
            }
        )

        with self.assertRaisesRegex(RuntimeError, r"v1\.1\.2: artifact is empty"):
            await entry.probe_pkg_latest(client, cfg)

    async def test_pkg_latest_reports_every_broken_target(self):
        broken = {"latest": "v1.1.2", "bundles": ["v1.1.2"]}
        cfg = {"api_host": "api.example.com", "archs": ["darwin-amd64", "darwin-arm64"]}
        client = fake_release_client(
            {
                "darwin-amd64": {
                    "rune-agent": broken,
                    "fuzzy-search": published("v1.1.2"),
                },
                "darwin-arm64": {
                    "rune-agent": broken,
                    "fuzzy-search": published("v1.1.2"),
                },
            }
        )

        with self.assertRaises(RuntimeError) as caught:
            await entry.probe_pkg_latest(client, cfg)

        detail = str(caught.exception)
        self.assertIn("2/4 latest versions not downloadable", detail)
        self.assertIn("darwin-amd64/rune-agent: v1.1.2: artifact: status 404", detail)
        self.assertIn("darwin-arm64/rune-agent: v1.1.2: artifact: status 404", detail)

    async def test_pkg_latest_uses_ranged_get_for_artifacts(self):
        cfg = {"api_host": "api.example.com", "archs": ["darwin-arm64"]}
        client = fake_release_client(
            {"darwin-arm64": {"rune-agent": published("v1.1.2")}}
        )

        await entry.probe_pkg_latest(client, cfg)

        artifact = [c for c in client.get_calls if "/blob/" in c[0]]
        self.assertEqual(
            [
                (
                    "https://cdn.example/blob/darwin-arm64/rune-agent/v1.1.2",
                    {"headers": {"range": "bytes=0-0"}},
                )
            ],
            artifact,
        )

    def test_owned_languages_matches_script(self):
        script = (
            Path(__file__).parents[3] / "package_versions.sh"
        ).read_text()
        literal = script.split("OWNED_LANGUAGES='", 1)[1].split("'", 1)[0]

        self.assertCountEqual(json.loads(literal), entry.OWNED_LANGUAGES)

    async def test_pkg_download_uses_signed_url(self):
        cfg = {"pkg_download_arch": "darwin-arm64"}
        report = {"signed_downloads": {"darwin-arm64": "https://signed.example/ada"}}
        client = fake_client(fake_response(status_code=206, content=b"a"))

        res = await entry.probe_pkg_download(client, cfg, report)

        self.assertEqual("pkg_download", res["layer"])
        self.assertEqual(entry.CHECK_OK, res["status"], res.get("detail"))
        self.assertEqual("signed download readable (darwin-arm64)", res["detail"])
        self.assertEqual(
            [
                (
                    "https://signed.example/ada",
                    {"headers": {"range": "bytes=0-0"}},
                )
            ],
            client.get_calls,
        )

    async def test_pkg_download_missing_url_fails(self):
        cfg = {"pkg_download_arch": "darwin-arm64"}
        client = fake_client(fake_response())

        res = await entry.probe_pkg_download(client, cfg, {})

        self.assertEqual(entry.CHECK_FAIL, res["status"])
        self.assertIn("no signed download url", res["detail"])
        self.assertTrue(res["critical"])

    async def test_pkg_download_non_200_fails(self):
        cfg = {"pkg_download_arch": "darwin-arm64"}
        report = {"signed_downloads": {"darwin-arm64": "https://signed.example/ada"}}
        client = fake_client(fake_response(status_code=403))

        res = await entry.probe_pkg_download(client, cfg, report)

        self.assertEqual(entry.CHECK_FAIL, res["status"])
        self.assertIn("403", res["detail"])

    async def test_deep_emits_ok_critical_check_when_healthy(self):
        cfg = {"api_host": "api.example.com", "pkg_download_arch": "darwin-arm64"}
        health = {
            "status": entry.STATUS_OK,
            "checks": [{"layer": "stripe", "status": entry.CHECK_OK}],
            "signed_downloads": {"darwin-arm64": "https://signed.example/ada"},
        }
        client = fake_client(
            fake_response(
                body=health,
                headers={"content-type": "application/json"},
                content=b"ada-bytes",
            )
        )

        checks = await entry.probe_deep(client, cfg, "secret")

        deep = [c for c in checks if c["layer"] == "deep"]
        self.assertEqual(1, len(deep), "healthy deep probe must emit a deep check")
        self.assertEqual(entry.CHECK_OK, deep[0]["status"])
        self.assertTrue(deep[0]["critical"])

    async def test_reconcile_triggers_failing_critical_with_runbook_link(self):
        client = fake_paging_client(fake_response(status_code=202))
        old_async_client = entry.httpx.AsyncClient
        entry.httpx.AsyncClient = lambda *args, **kwargs: client
        try:
            env = fake_env(PAGERDUTY_ROUTING_KEY="rk", ENV="prod")
            report = {
                "status": entry.STATUS_FAIL,
                "checks": [
                    entry.check_result("dns_api", entry.CHECK_FAIL, True, "nxdomain"),
                ],
            }

            await entry.reconcile_pages(env, report)
        finally:
            entry.httpx.AsyncClient = old_async_client

        self.assertEqual(1, len(client.post_calls))
        _, kwargs = client.post_calls[0]
        payload = kwargs["json"]
        self.assertEqual("trigger", payload["event_action"])
        self.assertEqual("oxprobe-cloudflare-prod-dns_api", payload["dedup_key"])
        self.assertEqual(
            [{"href": entry.RUNBOOK_URL, "text": "oxprobe runbook"}],
            payload["links"],
        )

    async def test_reconcile_resolves_passing_critical(self):
        client = fake_paging_client(fake_response(status_code=202))
        old_async_client = entry.httpx.AsyncClient
        entry.httpx.AsyncClient = lambda *args, **kwargs: client
        try:
            env = fake_env(PAGERDUTY_ROUTING_KEY="rk", ENV="prod")
            report = {
                "status": entry.STATUS_OK,
                "checks": [
                    entry.check_result("dns_api", entry.CHECK_OK, True),
                ],
            }

            await entry.reconcile_pages(env, report)
        finally:
            entry.httpx.AsyncClient = old_async_client

        self.assertEqual(1, len(client.post_calls))
        _, kwargs = client.post_calls[0]
        payload = kwargs["json"]
        self.assertEqual("resolve", payload["event_action"])
        self.assertEqual("oxprobe-cloudflare-prod-dns_api", payload["dedup_key"])
        self.assertNotIn("payload", payload)

    async def test_reconcile_skips_noncritical_layers(self):
        client = fake_paging_client(fake_response(status_code=202))
        old_async_client = entry.httpx.AsyncClient
        entry.httpx.AsyncClient = lambda *args, **kwargs: client
        try:
            env = fake_env(PAGERDUTY_ROUTING_KEY="rk", ENV="prod")
            report = {
                "status": entry.STATUS_DEGRADED,
                "checks": [
                    entry.check_result("auth0", entry.CHECK_FAIL, False, "down"),
                    entry.check_result("downloads_cdn", entry.CHECK_OK, False),
                ],
            }

            await entry.reconcile_pages(env, report)
        finally:
            entry.httpx.AsyncClient = old_async_client

        self.assertEqual(0, len(client.post_calls))


if __name__ == "__main__":
    unittest.main()
