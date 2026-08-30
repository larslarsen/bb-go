"""Prove fail-closed Govulncheck SARIF adjudication for BBGO-SEC-001 Exception 1."""

from __future__ import annotations

import copy
import importlib.util
import json
import subprocess
import unittest
from datetime import date
from io import StringIO
from pathlib import Path


NOW = date(2026, 8, 29)
DAY_BEFORE_EXPIRY = date(2026, 11, 28)
EXPIRY = date(2026, 11, 29)
AFTER_EXPIRY = date(2026, 11, 30)

EXCEPTION_ID = "GO-2024-3218"
EXCEPTION_MODULE = "github.com/libp2p/go-libp2p-kad-dht"
EXCEPTION_VERSION = "v0.42.2"
EXCEPTION_MODULE_VERSION = "%s@%s" % (EXCEPTION_MODULE, EXCEPTION_VERSION)
EXCEPTION_OWNER = "Lead Engineer/Reviewer — Codex"
OFFICIAL_DB = "https://vuln.go.dev"
SCANNER_NAME = "govulncheck"
SCANNER_VERSION = "v1.7.0"
SCANNER_URI = "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck"
NOTE_ID = "GO-2024-0001"
OTHER_ERROR_ID = "GO-2024-9999"
MAIN_MODULE = "github.com/larslarsen/bb-go/modern@"
WRONG_DHT_VERSION = "%s@v0.42.1" % EXCEPTION_MODULE
BINARY_PATH = "/runner/temp/bitbookd"
RAW_SARIF_MARKER = "RAW_SARIF_PADDING_" + ("Q" * 2048)


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def load_adjudicator():
    path = repo_root() / "scripts" / "govulncheck_policy.py"
    if not path.is_file():
        raise AssertionError(
            "required adjudicator scripts/govulncheck_policy.py does not exist"
        )
    spec = importlib.util.spec_from_file_location("bb_go_govulncheck_policy", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def valid_driver(mode: str) -> dict:
    return {
        "name": SCANNER_NAME,
        "semanticVersion": SCANNER_VERSION,
        "informationUri": SCANNER_URI,
        "properties": {
            "protocol_version": "v1.0.0",
            "scanner_name": SCANNER_NAME,
            "scanner_version": SCANNER_VERSION,
            "db": OFFICIAL_DB,
            "scan_level": "symbol",
            "scan_mode": mode,
        },
        "rules": [],
    }


def message(text: str) -> dict:
    return {"text": text}


def module_frame(module: str, symbol: str) -> dict:
    return {"module": module, "location": {"message": message(symbol)}}


def error_result(rule_id: str, module_version: str) -> dict:
    return {
        "ruleId": rule_id,
        "level": "error",
        "message": message(
            "Your code calls vulnerable functions in 1 package (%s)." % EXCEPTION_MODULE
        ),
        "codeFlows": [
            {
                "threadFlows": [
                    {
                        "locations": [
                            module_frame(MAIN_MODULE, "github.com/larslarsen/bb-go/modern/network.New"),
                            module_frame(module_version, "github.com/libp2p/go-libp2p-kad-dht.New"),
                        ]
                    }
                ],
                "message": message(
                    "A summarised code flow for vulnerable function github.com/libp2p/go-libp2p-kad-dht.New"
                ),
            }
        ],
        "stacks": [
            {
                "message": message(
                    "A call stack for vulnerable function github.com/libp2p/go-libp2p-kad-dht.New"
                ),
                "frames": [
                    module_frame(MAIN_MODULE, "github.com/larslarsen/bb-go/modern/network.New"),
                    module_frame(module_version, "github.com/libp2p/go-libp2p-kad-dht.New"),
                ],
            }
        ],
    }


def note_result(rule_id: str, module_path: str) -> dict:
    return {
        "ruleId": rule_id,
        "level": "note",
        "message": message(
            "Your code depends on 1 vulnerable module (%s), but doesn't appear to call any of the vulnerable symbols."
            % module_path
        ),
    }


def sarif_document(mode: str, results: list, rules: list | None = None) -> dict:
    driver = valid_driver(mode)
    if rules is None:
        rules = [
            {"id": result["ruleId"], "properties": {"tags": []}} for result in results
        ]
    driver["rules"] = rules
    return {
        "version": "2.1.0",
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "runs": [{"tool": {"driver": driver}, "results": results}],
    }


def clean_sarif(mode: str = "source") -> dict:
    return sarif_document(mode, [])


def exception_sarif(mode: str = "source") -> dict:
    return sarif_document(
        mode,
        [
            error_result(EXCEPTION_ID, EXCEPTION_MODULE_VERSION),
            note_result(NOTE_ID, "golang.org/x/example"),
        ],
    )


def padded_sarif(document: dict) -> dict:
    document = copy.deepcopy(document)
    document["runs"][0]["invocations"] = [{"commandLine": RAW_SARIF_MARKER}]
    return document


class AdjudicatorFixture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.policy = load_adjudicator()
        cls.error = cls.policy.AdjudicationError


class RequiredAdjudicatorExistsTest(unittest.TestCase):
    def test_required_adjudicator_exists(self):
        path = repo_root() / "scripts" / "govulncheck_policy.py"
        self.assertTrue(
            path.is_file(),
            "required adjudicator scripts/govulncheck_policy.py does not exist",
        )


class SarifAcceptanceTest(AdjudicatorFixture):
    def test_clean_source_sarif_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(clean_sarif("source")),
            expected_mode="source",
            now=NOW,
        )
        self.assertEqual(result.disposition, "clean")
        self.assertEqual(result.errors, [])

    def test_clean_binary_sarif_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(clean_sarif("binary")),
            expected_mode="binary",
            now=NOW,
        )
        self.assertEqual(result.disposition, "clean")

    def test_exact_unexpired_exception_with_notes_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=NOW,
        )
        self.assertEqual(result.disposition, "exception")
        self.assertEqual([item.rule_id for item in result.errors], [EXCEPTION_ID])
        self.assertEqual([item.rule_id for item in result.notes], [NOTE_ID])

    def test_exact_exception_on_day_before_expiry_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=DAY_BEFORE_EXPIRY,
        )
        self.assertEqual(result.disposition, "exception")

    def test_binary_exact_exception_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("binary")),
            expected_mode="binary",
            now=NOW,
        )
        self.assertEqual(result.disposition, "exception")

    def test_exit_3_with_exact_exception_is_accepted(self):
        result = self.policy.evaluate_scan(
            scanner_exit=3,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=NOW,
        )
        self.assertEqual(result.disposition, "exception")


class SarifRejectionTest(AdjudicatorFixture):
    def reject(self, **kwargs):
        with self.assertRaises(self.error):
            self.policy.evaluate_scan(**kwargs)

    def test_additional_error_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["results"].append(
            error_result(OTHER_ERROR_ID, "github.com/example/vuln@v1.0.0")
        )
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_dht_version_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["results"] = [
            error_result(EXCEPTION_ID, WRONG_DHT_VERSION)
        ]
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_missing_dht_module_version_is_rejected(self):
        document = exception_sarif("source")
        finding = error_result(EXCEPTION_ID, EXCEPTION_MODULE_VERSION)
        finding["stacks"][0]["frames"] = [
            module_frame(MAIN_MODULE, "github.com/larslarsen/bb-go/modern/network.New")
        ]
        finding["codeFlows"][0]["threadFlows"][0]["locations"] = [
            module_frame(MAIN_MODULE, "github.com/larslarsen/bb-go/modern/network.New")
        ]
        document["runs"][0]["results"] = [finding]
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_mixed_dht_versions_in_traces_are_rejected(self):
        document = exception_sarif("source")
        finding = document["runs"][0]["results"][0]
        finding["stacks"][0]["frames"].append(
            module_frame(WRONG_DHT_VERSION, "github.com/libp2p/go-libp2p-kad-dht.PutValue")
        )
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_empty_output_is_rejected(self):
        self.reject(scanner_exit=0, stdout="", expected_mode="source", now=NOW)
        self.reject(scanner_exit=0, stdout="   \n", expected_mode="source", now=NOW)

    def test_malformed_json_is_rejected(self):
        self.reject(
            scanner_exit=0,
            stdout="{not-json",
            expected_mode="source",
            now=NOW,
        )

    def test_non_object_sarif_is_rejected(self):
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(["govulncheck"]),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_scanner_name_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["tool"]["driver"]["name"] = "osv-scanner"
        document["runs"][0]["tool"]["driver"]["properties"]["scanner_name"] = "osv-scanner"
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_scanner_version_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["tool"]["driver"]["semanticVersion"] = "v1.6.0"
        document["runs"][0]["tool"]["driver"]["properties"]["scanner_version"] = "v1.6.0"
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_database_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["tool"]["driver"]["properties"]["db"] = "https://example.invalid/vulndb"
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_scan_mode_is_rejected(self):
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("binary")),
            expected_mode="source",
            now=NOW,
        )
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="binary",
            now=NOW,
        )

    def test_wrong_scan_level_is_rejected(self):
        document = exception_sarif("source")
        document["runs"][0]["tool"]["driver"]["properties"]["scan_level"] = "module"
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_wrong_sarif_version_is_rejected(self):
        document = exception_sarif("source")
        document["version"] = "2.0.0"
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )

    def test_expired_exception_is_rejected(self):
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=AFTER_EXPIRY,
        )

    def test_exception_on_expiry_date_is_rejected(self):
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=EXPIRY,
        )

    def test_invocation_failure_exit_is_rejected(self):
        stdout = json.dumps(exception_sarif("source"))
        for code in (1, 2, 4, 127):
            self.reject(
                scanner_exit=code,
                stdout=stdout,
                expected_mode="source",
                now=NOW,
            )

    def test_exit_3_without_sarif_is_rejected(self):
        self.reject(
            scanner_exit=3,
            stdout="Vulnerability #1: GO-2024-3218\n",
            expected_mode="source",
            now=NOW,
        )

    def test_exit_3_with_clean_sarif_is_rejected(self):
        self.reject(
            scanner_exit=3,
            stdout=json.dumps(clean_sarif("source")),
            expected_mode="source",
            now=NOW,
        )

    def test_other_error_without_exception_is_rejected(self):
        document = sarif_document(
            "source",
            [error_result(OTHER_ERROR_ID, "github.com/example/vuln@v1.0.0")],
        )
        self.reject(
            scanner_exit=0,
            stdout=json.dumps(document),
            expected_mode="source",
            now=NOW,
        )


class NoteReportingTest(AdjudicatorFixture):
    def test_notes_remain_visible_in_summary(self):
        result = self.policy.evaluate_scan(
            scanner_exit=0,
            stdout=json.dumps(exception_sarif("source")),
            expected_mode="source",
            now=NOW,
        )
        summary = result.summary()
        self.assertIn(NOTE_ID, summary)
        self.assertIn("note", summary.lower())
        self.assertIn("doesn't appear to call", summary)
        self.assertIn(EXCEPTION_ID, summary)
        self.assertIn(EXCEPTION_MODULE_VERSION, summary)
        self.assertIn(EXCEPTION_OWNER, summary)
        self.assertIn("2026-11-29", summary)


class ExceptionMetadataTest(AdjudicatorFixture):
    def test_exception_metadata_matches_ticket(self):
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_ID, EXCEPTION_ID)
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_MODULE, EXCEPTION_MODULE)
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_VERSION, EXCEPTION_VERSION)
        self.assertEqual(
            self.policy.REVIEWED_EXCEPTION_MODULE_VERSION, EXCEPTION_MODULE_VERSION
        )
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_EXPIRY, EXPIRY)
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_OWNER, EXCEPTION_OWNER)
        self.assertEqual(self.policy.OFFICIAL_VULN_DB, OFFICIAL_DB)
        self.assertEqual(self.policy.REQUIRED_SCANNER_NAME, SCANNER_NAME)
        self.assertEqual(self.policy.REQUIRED_SCANNER_VERSION, SCANNER_VERSION)


class InvocationTest(AdjudicatorFixture):
    def execute_recording(self, returncode, stdout, stderr=""):
        recorded = {}

        def execute(args, cwd=None, text=None, capture_output=None, check=None):
            recorded["args"] = list(args)
            recorded["cwd"] = cwd
            recorded["text"] = text
            recorded["capture_output"] = capture_output
            recorded["check"] = check
            return subprocess.CompletedProcess(
                args, returncode, stdout=stdout, stderr=stderr
            )

        return recorded, execute

    def test_source_invocation_uses_sarif_and_official_db(self):
        recorded, execute = self.execute_recording(
            0, json.dumps(clean_sarif("source"))
        )
        result = self.policy.run_scan(
            "source",
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertEqual(result.disposition, "clean")
        self.assertEqual(
            recorded["args"],
            [
                "govulncheck",
                "-format",
                "sarif",
                "-db",
                OFFICIAL_DB,
                "-test",
                "./...",
            ],
        )
        self.assertEqual(Path(recorded["cwd"]), repo_root() / "modern")
        self.assertIs(recorded["text"], True)
        self.assertIs(recorded["capture_output"], True)
        self.assertIs(recorded["check"], False)
        self.assertNotIn("-ignore", recorded["args"])
        self.assertNotIn("-mode", recorded["args"])

    def test_binary_invocation_uses_sarif_and_official_db(self):
        recorded, execute = self.execute_recording(
            0, json.dumps(clean_sarif("binary"))
        )
        result = self.policy.run_scan(
            "binary",
            binary=BINARY_PATH,
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertEqual(result.disposition, "clean")
        self.assertEqual(
            recorded["args"],
            [
                "govulncheck",
                "-format",
                "sarif",
                "-db",
                OFFICIAL_DB,
                "-mode",
                "binary",
                BINARY_PATH,
            ],
        )
        self.assertNotIn("-test", recorded["args"])
        self.assertNotIn("-ignore", recorded["args"])
        self.assertIs(recorded["check"], False)

    def test_binary_mode_requires_path(self):
        def execute(*_args, **_kwargs):
            self.fail("govulncheck must not run without a binary path")

        with self.assertRaises(self.error):
            self.policy.run_scan(
                "binary",
                binary="",
                execute=execute,
                now=NOW,
                repo_root=repo_root(),
            )

    def test_source_scan_does_not_require_binary_path(self):
        _recorded, execute = self.execute_recording(
            0, json.dumps(clean_sarif("source"))
        )
        result = self.policy.run_scan(
            "source",
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertEqual(result.disposition, "clean")

    def test_missing_executable_is_rejected(self):
        def execute(*_args, **_kwargs):
            raise FileNotFoundError("govulncheck")

        with self.assertRaises(self.error):
            self.policy.run_scan(
                "source",
                execute=execute,
                now=NOW,
                repo_root=repo_root(),
            )

    def test_source_accepts_exit_3_only_for_exact_exception(self):
        recorded, execute = self.execute_recording(
            3, json.dumps(exception_sarif("source"))
        )
        result = self.policy.run_scan(
            "source",
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertEqual(result.disposition, "exception")
        self.assertEqual(recorded["args"][0], "govulncheck")

    def test_main_source_returns_success_for_clean_scan(self):
        _recorded, execute = self.execute_recording(
            0, json.dumps(clean_sarif("source"))
        )
        code = self.policy.main(
            ["source"],
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertEqual(code, 0)

    def test_main_rejects_extra_error(self):
        document = exception_sarif("source")
        document["runs"][0]["results"].append(
            error_result(OTHER_ERROR_ID, "github.com/example/vuln@v1.0.0")
        )
        _recorded, execute = self.execute_recording(0, json.dumps(document))
        code = self.policy.main(
            ["source"],
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
        )
        self.assertNotEqual(code, 0)

    def test_copy_does_not_mutate_exception_constants(self):
        document = copy.deepcopy(exception_sarif("source"))
        document["runs"][0]["results"][0]["ruleId"] = OTHER_ERROR_ID
        with self.assertRaises(self.error):
            self.policy.evaluate_scan(
                scanner_exit=0,
                stdout=json.dumps(document),
                expected_mode="source",
                now=NOW,
            )
        self.assertEqual(self.policy.REVIEWED_EXCEPTION_ID, EXCEPTION_ID)


class SuccessfulCliOutputTest(InvocationTest):
    def run_main(self, argv, returncode, document, stderr="scanner chatter"):
        raw = json.dumps(padded_sarif(document))
        expected = self.policy.evaluate_scan(
            scanner_exit=returncode,
            stdout=raw,
            expected_mode=argv[0],
            now=NOW,
        )
        _recorded, execute = self.execute_recording(returncode, raw, stderr=stderr)
        stdout = StringIO()
        captured_err = StringIO()
        code = self.policy.main(
            argv,
            execute=execute,
            now=NOW,
            repo_root=repo_root(),
            stdout=stdout,
            stderr=captured_err,
        )
        return code, stdout.getvalue(), captured_err.getvalue(), expected, raw

    def assert_concise_summary_only(self, output, stderr_output, expected, raw):
        self.assertEqual(output, expected.summary() + "\n")
        self.assertEqual(stderr_output, "")
        self.assertNotIn(raw, output)
        self.assertNotIn(RAW_SARIF_MARKER, output)
        self.assertNotIn("$schema", output)
        self.assertNotIn('"runs"', output)
        self.assertNotIn("invocations", output)
        self.assertLess(len(output.splitlines()), 20)
        self.assertLess(len(output), len(raw))

    def test_successful_source_main_prints_concise_summary_not_raw_sarif(self):
        code, output, stderr_output, expected, raw = self.run_main(
            ["source"], 0, clean_sarif("source")
        )
        self.assertEqual(code, 0)
        self.assertEqual(expected.disposition, "clean")
        self.assert_concise_summary_only(output, stderr_output, expected, raw)
        self.assertIn("Govulncheck source scan: clean", output)

    def test_successful_exception_main_prints_notes_and_metadata_not_raw_sarif(self):
        code, output, stderr_output, expected, raw = self.run_main(
            ["source"], 3, exception_sarif("source")
        )
        self.assertEqual(code, 0)
        self.assertEqual(expected.disposition, "exception")
        self.assert_concise_summary_only(output, stderr_output, expected, raw)
        self.assertIn(NOTE_ID, output)
        self.assertIn("doesn't appear to call", output)
        self.assertIn(EXCEPTION_ID, output)
        self.assertIn(EXCEPTION_MODULE_VERSION, output)
        self.assertIn(EXCEPTION_OWNER, output)
        self.assertIn("2026-11-29", output)
        self.assertIn("note results: 1", output)

    def test_successful_binary_main_prints_concise_summary_not_raw_sarif(self):
        code, output, stderr_output, expected, raw = self.run_main(
            ["binary", BINARY_PATH], 0, clean_sarif("binary")
        )
        self.assertEqual(code, 0)
        self.assertEqual(expected.disposition, "clean")
        self.assert_concise_summary_only(output, stderr_output, expected, raw)
        self.assertIn("Govulncheck binary scan: clean", output)


if __name__ == "__main__":
    unittest.main()
