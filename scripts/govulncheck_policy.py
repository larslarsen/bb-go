"""Fail-closed Govulncheck SARIF adjudicator for BBGO-SEC-001 Exception 1."""

from __future__ import annotations

import json
import subprocess
import sys
from datetime import date
from pathlib import Path


class AdjudicationError(Exception):
    """Raised when a scan is not clean and is not the exact reviewed exception."""


REVIEWED_EXCEPTION_ID = "GO-2024-3218"
REVIEWED_EXCEPTION_MODULE = "github.com/libp2p/go-libp2p-kad-dht"
REVIEWED_EXCEPTION_VERSION = "v0.42.2"
REVIEWED_EXCEPTION_MODULE_VERSION = "%s@%s" % (
    REVIEWED_EXCEPTION_MODULE,
    REVIEWED_EXCEPTION_VERSION,
)
REVIEWED_EXCEPTION_EXPIRY = date(2026, 11, 29)
REVIEWED_EXCEPTION_OWNER = "Lead Engineer/Reviewer — Codex"

OFFICIAL_VULN_DB = "https://vuln.go.dev"
REQUIRED_SCANNER_NAME = "govulncheck"
REQUIRED_SCANNER_VERSION = "v1.7.0"
REQUIRED_SCANNER_URI = "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck"
REQUIRED_SARIF_VERSION = "2.1.0"
REQUIRED_SARIF_SCHEMA = "https://json.schemastore.org/sarif-2.1.0.json"
REQUIRED_PROTOCOL_VERSION = "v1.0.0"
REQUIRED_SCAN_LEVEL = "symbol"
ALLOWED_LEVELS = ("error", "warning", "note")
FINDINGS_EXIT = 3
SUCCESS_EXIT = 0


class Finding:
    def __init__(self, rule_id, level, message):
        self.rule_id = rule_id
        self.level = level
        self.message = message


class ScanResult:
    def __init__(self, disposition, expected_mode, findings, document):
        self.disposition = disposition
        self.expected_mode = expected_mode
        self.findings = list(findings)
        self.document = document
        self.stdout = ""
        self.stderr = ""

    @property
    def errors(self):
        return [item for item in self.findings if item.level == "error"]

    @property
    def warnings(self):
        return [item for item in self.findings if item.level == "warning"]

    @property
    def notes(self):
        return [item for item in self.findings if item.level == "note"]

    def summary(self):
        if self.disposition == "clean":
            label = "clean"
        elif self.disposition == "exception":
            label = "accepted reviewed exception"
        else:
            label = self.disposition
        lines = ["Govulncheck %s scan: %s" % (self.expected_mode, label)]
        if self.disposition == "exception":
            lines.extend(
                [
                    "Accepted reviewed exception %s on %s"
                    % (REVIEWED_EXCEPTION_ID, REVIEWED_EXCEPTION_MODULE_VERSION),
                    "owner: %s" % REVIEWED_EXCEPTION_OWNER,
                    "expires: %s" % REVIEWED_EXCEPTION_EXPIRY.isoformat(),
                ]
            )
        lines.append("error results: %s" % len(self.errors))
        lines.append("warning results: %s" % len(self.warnings))
        lines.append("note results: %s" % len(self.notes))
        for item in self.findings:
            lines.append("- %s %s: %s" % (item.level, item.rule_id, item.message))
        return "\n".join(lines)


def adjudicate_sarif(document, expected_mode, now=None):
    if now is None:
        now = date.today()
    run = _require_valid_run(document, expected_mode)
    findings = _findings(run)
    errors = [item for item in findings if item.level == "error"]
    if not errors:
        return ScanResult("clean", expected_mode, findings, document)
    if len(errors) != 1 or errors[0].rule_id != REVIEWED_EXCEPTION_ID:
        extra = ", ".join("%s (%s)" % (item.rule_id, item.level) for item in errors)
        raise AdjudicationError(
            "reachable Govulncheck errors are not the reviewed exception: %s" % extra
        )
    _require_exact_dht_traces(run, REVIEWED_EXCEPTION_ID)
    if now >= REVIEWED_EXCEPTION_EXPIRY:
        raise AdjudicationError(
            "reviewed exception %s expired on %s"
            % (REVIEWED_EXCEPTION_ID, REVIEWED_EXCEPTION_EXPIRY.isoformat())
        )
    return ScanResult("exception", expected_mode, findings, document)


def evaluate_scan(scanner_exit, stdout, expected_mode, now=None, stderr=""):
    if scanner_exit not in (SUCCESS_EXIT, FINDINGS_EXIT):
        raise AdjudicationError(
            "govulncheck execution failed with exit %s" % scanner_exit
        )
    if stdout is None or not str(stdout).strip():
        raise AdjudicationError("govulncheck produced empty output")
    try:
        document = json.loads(stdout)
    except json.JSONDecodeError as exc:
        raise AdjudicationError("govulncheck SARIF is not JSON: %s" % exc) from exc
    result = adjudicate_sarif(document, expected_mode=expected_mode, now=now)
    result.stdout = str(stdout)
    result.stderr = stderr or ""
    if scanner_exit == FINDINGS_EXIT and result.disposition == "clean":
        raise AdjudicationError("govulncheck exit 3 without a reachable SARIF error")
    return result


def run_scan(mode, binary=None, *, execute=None, now=None, repo_root=None):
    if execute is None:
        execute = subprocess.run
    if repo_root is None:
        repo_root = Path(__file__).resolve().parent.parent
    else:
        repo_root = Path(repo_root)
    args, cwd = _govulncheck_command(mode, binary, repo_root)
    try:
        completed = execute(
            args,
            cwd=str(cwd),
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError as exc:
        raise AdjudicationError("govulncheck invocation failed: %s" % exc) from exc
    return evaluate_scan(
        scanner_exit=completed.returncode,
        stdout=completed.stdout or "",
        expected_mode=mode,
        now=now,
        stderr=completed.stderr or "",
    )


def main(argv=None, execute=None, now=None, repo_root=None, stdout=None, stderr=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    out = sys.stdout if stdout is None else stdout
    err = sys.stderr if stderr is None else stderr
    try:
        mode, binary = _parse_args(argv)
        result = run_scan(
            mode,
            binary=binary,
            execute=execute,
            now=now,
            repo_root=repo_root,
        )
    except AdjudicationError as exc:
        print(str(exc), file=err)
        return 1
    print(result.summary(), file=out)
    return 0


def _parse_args(argv):
    if argv == ["source"]:
        return "source", None
    if len(argv) == 2 and argv[0] == "binary":
        return "binary", argv[1]
    raise AdjudicationError("usage: govulncheck_policy.py source | binary PATH")


def _govulncheck_command(mode, binary, root):
    if mode == "source":
        if binary:
            raise AdjudicationError("source mode does not take a binary path")
        return (
            [
                "govulncheck",
                "-format",
                "sarif",
                "-db",
                OFFICIAL_VULN_DB,
                "-test",
                "./...",
            ],
            root / "modern",
        )
    if mode == "binary":
        if not binary or not str(binary).strip():
            raise AdjudicationError("binary mode requires a daemon path")
        return (
            [
                "govulncheck",
                "-format",
                "sarif",
                "-db",
                OFFICIAL_VULN_DB,
                "-mode",
                "binary",
                str(binary),
            ],
            root,
        )
    raise AdjudicationError("unknown scan mode %r" % mode)


def _require_valid_run(document, expected_mode):
    if not isinstance(document, dict):
        raise AdjudicationError("govulncheck SARIF is not an object")
    if document.get("version") != REQUIRED_SARIF_VERSION:
        raise AdjudicationError("SARIF version is not %s" % REQUIRED_SARIF_VERSION)
    if document.get("$schema") != REQUIRED_SARIF_SCHEMA:
        raise AdjudicationError("SARIF schema is not the official 2.1.0 schema")
    runs = document.get("runs")
    if not isinstance(runs, list) or len(runs) != 1:
        raise AdjudicationError("SARIF must contain exactly one run")
    run = runs[0]
    if not isinstance(run, dict):
        raise AdjudicationError("SARIF run is not an object")
    driver = ((run.get("tool") or {}) if isinstance(run.get("tool"), dict) else {}).get(
        "driver"
    )
    if not isinstance(driver, dict):
        raise AdjudicationError("SARIF tool driver is missing")
    if driver.get("name") != REQUIRED_SCANNER_NAME:
        raise AdjudicationError("scanner name is not %s" % REQUIRED_SCANNER_NAME)
    if driver.get("semanticVersion") != REQUIRED_SCANNER_VERSION:
        raise AdjudicationError(
            "scanner semanticVersion is not %s" % REQUIRED_SCANNER_VERSION
        )
    if driver.get("informationUri") != REQUIRED_SCANNER_URI:
        raise AdjudicationError("scanner informationUri is not the official govulncheck URI")
    properties = driver.get("properties")
    if not isinstance(properties, dict):
        raise AdjudicationError("scanner properties are missing")
    if properties.get("scanner_name") != REQUIRED_SCANNER_NAME:
        raise AdjudicationError("scanner_name is not %s" % REQUIRED_SCANNER_NAME)
    if properties.get("scanner_version") != REQUIRED_SCANNER_VERSION:
        raise AdjudicationError(
            "scanner_version is not %s" % REQUIRED_SCANNER_VERSION
        )
    if properties.get("db") != OFFICIAL_VULN_DB:
        raise AdjudicationError("vulnerability database is not %s" % OFFICIAL_VULN_DB)
    if properties.get("scan_mode") != expected_mode:
        raise AdjudicationError(
            "scan_mode is %r, expected %r" % (properties.get("scan_mode"), expected_mode)
        )
    if properties.get("scan_level") != REQUIRED_SCAN_LEVEL:
        raise AdjudicationError("scan_level is not %s" % REQUIRED_SCAN_LEVEL)
    if properties.get("protocol_version") != REQUIRED_PROTOCOL_VERSION:
        raise AdjudicationError(
            "protocol_version is not %s" % REQUIRED_PROTOCOL_VERSION
        )
    results = run.get("results")
    if not isinstance(results, list):
        raise AdjudicationError("SARIF results array is missing")
    return run


def _findings(run):
    findings = []
    for index, raw in enumerate(run.get("results") or []):
        if not isinstance(raw, dict):
            raise AdjudicationError("SARIF result %s is not an object" % index)
        rule_id = raw.get("ruleId")
        level = raw.get("level")
        if not isinstance(rule_id, str) or not rule_id:
            raise AdjudicationError("SARIF result %s is missing ruleId" % index)
        if level not in ALLOWED_LEVELS:
            raise AdjudicationError(
                "SARIF result %s has invalid level %r" % (index, level)
            )
        text = ""
        message = raw.get("message")
        if isinstance(message, dict):
            text = str(message.get("text") or "")
        findings.append(Finding(rule_id=rule_id, level=level, message=text))
    return findings


def _require_exact_dht_traces(run, rule_id):
    versions = []
    for raw in run.get("results") or []:
        if not isinstance(raw, dict) or raw.get("ruleId") != rule_id:
            continue
        if raw.get("level") != "error":
            continue
        for spec in _iter_modules(raw):
            path, version = _split_module(spec)
            if path == REVIEWED_EXCEPTION_MODULE:
                versions.append(version)
    if not versions:
        raise AdjudicationError(
            "reviewed exception %s is missing %s traces"
            % (rule_id, REVIEWED_EXCEPTION_MODULE_VERSION)
        )
    unexpected = sorted(
        {version for version in versions if version != REVIEWED_EXCEPTION_VERSION}
    )
    if unexpected:
        raise AdjudicationError(
            "reviewed exception %s traces are not exactly %s (found %s)"
            % (
                rule_id,
                REVIEWED_EXCEPTION_MODULE_VERSION,
                ", ".join(
                    "%s@%s" % (REVIEWED_EXCEPTION_MODULE, version)
                    for version in unexpected
                ),
            )
        )


def _iter_modules(result):
    for stack in result.get("stacks") or []:
        if not isinstance(stack, dict):
            continue
        for frame in stack.get("frames") or []:
            if isinstance(frame, dict) and frame.get("module"):
                yield str(frame["module"])
    for flow in result.get("codeFlows") or []:
        if not isinstance(flow, dict):
            continue
        for thread in flow.get("threadFlows") or []:
            if not isinstance(thread, dict):
                continue
            for location in thread.get("locations") or []:
                if isinstance(location, dict) and location.get("module"):
                    yield str(location["module"])


def _split_module(spec):
    if "@" not in spec:
        return spec, ""
    path, version = spec.rsplit("@", 1)
    return path, version


if __name__ == "__main__":
    sys.exit(main())
