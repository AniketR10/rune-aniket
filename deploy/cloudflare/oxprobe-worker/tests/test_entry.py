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


class EntryTest(unittest.IsolatedAsyncioTestCase):
    async def test_scheduled_accepts_cloudflare_runtime_arguments(self):
        calls = []
        old_run_worker_probe = entry.run_worker_probe
        old_has_critical_failure = entry.has_critical_failure
        try:
            async def fake_run_worker_probe(env):
                calls.append(env)
                return {"status": entry.STATUS_OK, "checks": []}

            entry.run_worker_probe = fake_run_worker_probe
            entry.has_critical_failure = lambda report: False
            env = object()

            await entry.Default().scheduled(object(), env, object())

            self.assertEqual([env], calls)
        finally:
            entry.run_worker_probe = old_run_worker_probe
            entry.has_critical_failure = old_has_critical_failure

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

    async def test_pkg_download_uses_signed_url(self):
        cfg = {"pkg_download_arch": "darwin-arm64"}
        report = {"signed_downloads": {"darwin-arm64": "https://signed.example/ada"}}
        client = fake_client(fake_response(content=b"ada-bytes"))

        res = await entry.probe_pkg_download(client, cfg, report)

        self.assertEqual("pkg_download", res["layer"])
        self.assertEqual(entry.CHECK_OK, res["status"], res.get("detail"))
        self.assertEqual(
            [("https://signed.example/ada", {})], client.get_calls
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


if __name__ == "__main__":
    unittest.main()
