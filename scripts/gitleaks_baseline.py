"""Fail-closed reviewed Gitleaks baseline validator for BBGO-SEC-001."""

from __future__ import annotations

import hashlib
import json
import sys
from datetime import date
from pathlib import Path


class BaselineError(Exception):
    """Raised when the committed Gitleaks baseline is not the reviewed set."""


REVIEWED_BASELINE_RELPATH = "security/gitleaks-baseline.json"
REVIEWED_BASELINE_SHA256 = (
    "ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00"
)
REVIEWED_BASELINE_BYTES = 21758
REVIEWED_BASELINE_COUNT = 25
REVIEWED_BASELINE_EXPIRY = date(2026, 11, 29)
REVIEWED_BASELINE_OWNER = "Lead Engineer/Reviewer — Codex"
REQUIRED_GITLEAKS_MODULE = "github.com/zricethezav/gitleaks/v8@v8.30.1"
REDACTED = "REDACTED"

REVIEWED_IDENTITIES = frozenset(
    [
        (
            "generic-api-key",
            "repo/migrations/Migration028_test.go",
            "361c5e48a484345aa03304f929ef07f5cb149b11",
            22,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration028_test.go",
            "d79a497dec01e2d8a3c709429c82a4f7c0da8d83",
            22,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration027_test.go",
            "b7d758e91867b05ec26cebf4d9a94041bcfcc653",
            22,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "88a0f36ee91b5754fa0daacc0ecf7bfc160e2af8",
            12,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "f662c7a3fa53adee1ff6e791ac0b31e72363931c",
            12,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "be84e7a8d40484081f6cb1f4ee21df2e05c6e97d",
            13,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "aaf02c72fde4eae6b9ad4bd2b45cf35c81cd36b8",
            13,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration012_test.go",
            "e3de1de357e527e89354a06d33c0d8a7ad1c1f98",
            27,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration012_test.go",
            "8a08e2f075f309590ed4e678c8addead4ae3ea23",
            27,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration012_test.go",
            "4325d6586ac9c18ed13b7c1e398067683126a9b7",
            27,
        ),
        (
            "generic-api-key",
            "repo/migrations/Migration012_test.go",
            "79cf71f926dbb218d82649058e43c97085c0e6d1",
            24,
        ),
        (
            "generic-api-key",
            "bitcoin/zcashd/wallet.go",
            "7f1c6be08974a2c213c0f89cd2d56add0e859127",
            980,
        ),
        (
            "generic-api-key",
            "vendor/gx/ipfs/QmNUKMfTHQQpEwE8bUdv5qmKC3ymdW7zw82LFS8D6MQXmu/go-ipfs/core/commands/publish.go",
            "1bf7750aa2b49fb5eae20baf3baa0de43175930c",
            54,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "904c2411e1e06ca22e2a890066cdae01bc3afe24",
            11,
        ),
        (
            "generic-api-key",
            "docs/security.md",
            "4847c21819b15a901c23a791508f98f6a5e80392",
            51,
        ),
        (
            "generic-api-key",
            "docs/security.md",
            "4847c21819b15a901c23a791508f98f6a5e80392",
            60,
        ),
        (
            "generic-api-key",
            "test/config.go",
            "ac48cd9ba2115f364d520238108dc895388882bf",
            19,
        ),
        (
            "generic-api-key",
            "test/config.go",
            "e3b1277cd2541d5814cf243a44a67aebc11c109f",
            19,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "5420cfc42837885e4a5e8fa7c8e69f1cca39a5b1",
            11,
        ),
        (
            "generic-api-key",
            "net/encryption_test.go",
            "96ab4e7450fa8975cf00b0a0b6a2c56421eec13f",
            118,
        ),
        (
            "generic-api-key",
            "ipfs/identity_test.go",
            "318663df4ba42c262798e6be761294be95100408",
            11,
        ),
        (
            "generic-api-key",
            "docs/api.yaml",
            "1d2c17809dc5a1cc2081013378d39028e2b046af",
            3068,
        ),
        (
            "generic-api-key",
            "docs/api.yaml",
            "de5de2db0c92982f6763ccb6ddf86f47d7daa050",
            25,
        ),
        (
            "private-key",
            ".travis/sign.key.gpg",
            "f2aeed417980a797ad20534592cd309764cb1300",
            54,
        ),
        (
            "generic-api-key",
            "net/encryption_test.go",
            "93fb166c90a0e4528ec005749d3683c6a9769783",
            31,
        ),
    ]
)

_ENTRY_KEYS = ("RuleID", "File", "Commit", "StartLine", "Secret", "Match", "Fingerprint")
_ASCII_IDENTIFIER_CHARS = frozenset(
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
)


class BaselineResult:
    def __init__(self, digest, count):
        self.digest = digest
        self.count = count

    def summary(self):
        return "\n".join(
            [
                "Gitleaks baseline: accepted reviewed baseline",
                "entries: %s" % self.count,
                "owner: %s" % REVIEWED_BASELINE_OWNER,
                "expires: %s" % REVIEWED_BASELINE_EXPIRY.isoformat(),
                "sha256: %s" % self.digest,
            ]
        )


def path_is_modern(path):
    normalized = str(path or "").replace("\\", "/")
    parts = [part for part in normalized.split("/") if part not in ("", ".")]
    return "modern" in parts


def validate_document(document, now=None):
    if now is None:
        now = date.today()
    if not isinstance(document, list):
        raise BaselineError("gitleaks baseline is not a JSON array")
    identities = []
    for index, entry in enumerate(document):
        identities.append(_validate_entry(entry, index))
    actual = set(identities)
    if len(identities) != len(actual):
        raise BaselineError("gitleaks baseline contains duplicate identities")
    added = actual - REVIEWED_IDENTITIES
    removed = REVIEWED_IDENTITIES - actual
    if added or removed or len(document) != REVIEWED_BASELINE_COUNT:
        parts = [
            "count %s, expected %s" % (len(document), REVIEWED_BASELINE_COUNT)
        ]
        if added:
            parts.append("added %s" % _format_identities(added))
        if removed:
            parts.append("removed %s" % _format_identities(removed))
        raise BaselineError(
            "gitleaks baseline identities or count are not the reviewed set (%s)"
            % "; ".join(parts)
        )
    if now >= REVIEWED_BASELINE_EXPIRY:
        raise BaselineError(
            "reviewed Gitleaks baseline expired on %s"
            % REVIEWED_BASELINE_EXPIRY.isoformat()
        )
    return BaselineResult(digest=REVIEWED_BASELINE_SHA256, count=len(document))


def validate_bytes(raw, now=None):
    if not isinstance(raw, (bytes, bytearray)):
        raise BaselineError("gitleaks baseline bytes are missing")
    digest = hashlib.sha256(bytes(raw)).hexdigest()
    if digest != REVIEWED_BASELINE_SHA256:
        raise BaselineError(
            "gitleaks baseline SHA-256 is %s, expected %s"
            % (digest, REVIEWED_BASELINE_SHA256)
        )
    if len(raw) != REVIEWED_BASELINE_BYTES:
        raise BaselineError(
            "gitleaks baseline size is %s, expected %s"
            % (len(raw), REVIEWED_BASELINE_BYTES)
        )
    try:
        document = json.loads(bytes(raw).decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise BaselineError("gitleaks baseline is not JSON: %s" % exc) from exc
    result = validate_document(document, now=now)
    result.digest = digest
    return result


def validate_file(path, now=None):
    path = Path(path)
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise BaselineError("unable to read gitleaks baseline: %s" % exc) from exc
    return validate_bytes(raw, now=now)


def main(argv=None, now=None, repo_root=None, stdout=None, stderr=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    out = sys.stdout if stdout is None else stdout
    err = sys.stderr if stderr is None else stderr
    try:
        if argv:
            raise BaselineError("usage: gitleaks_baseline.py")
        if repo_root is None:
            root = Path(__file__).resolve().parent.parent
        else:
            root = Path(repo_root)
        result = validate_file(root / REVIEWED_BASELINE_RELPATH, now=now)
    except BaselineError as exc:
        print(str(exc), file=err)
        return 1
    print(result.summary(), file=out)
    return 0


def _contains_complete_redaction_marker(value):
    if not isinstance(value, str):
        return False
    start = 0
    marker_len = len(REDACTED)
    while True:
        index = value.find(REDACTED, start)
        if index < 0:
            return False
        prefix_is_identifier = (
            index > 0 and value[index - 1] in _ASCII_IDENTIFIER_CHARS
        )
        suffix_index = index + marker_len
        suffix_is_identifier = (
            suffix_index < len(value)
            and value[suffix_index] in _ASCII_IDENTIFIER_CHARS
        )
        if not prefix_is_identifier and not suffix_is_identifier:
            return True
        start = index + marker_len


def _validate_entry(entry, index):
    if not isinstance(entry, dict):
        raise BaselineError("gitleaks baseline entry %s is not an object" % index)
    for key in _ENTRY_KEYS:
        if key not in entry:
            raise BaselineError(
                "gitleaks baseline entry %s is missing %s" % (index, key)
            )
    secret = entry.get("Secret")
    match = entry.get("Match")
    if secret != REDACTED or not _contains_complete_redaction_marker(match):
        raise BaselineError("gitleaks baseline entry %s is not redacted" % index)
    rule = entry.get("RuleID")
    file = entry.get("File")
    commit = entry.get("Commit")
    line = entry.get("StartLine")
    if not isinstance(rule, str) or not rule:
        raise BaselineError("gitleaks baseline entry %s is missing RuleID" % index)
    if not isinstance(file, str) or not file:
        raise BaselineError("gitleaks baseline entry %s is missing File" % index)
    if not isinstance(commit, str) or not commit:
        raise BaselineError("gitleaks baseline entry %s is missing Commit" % index)
    if not isinstance(line, int) or isinstance(line, bool):
        raise BaselineError(
            "gitleaks baseline entry %s has a non-integer line" % index
        )
    if path_is_modern(file) or path_is_modern(entry.get("SymlinkFile") or ""):
        raise BaselineError("gitleaks baseline contains a modern/ path: %s" % file)
    fingerprint = entry.get("Fingerprint")
    expected = "%s:%s:%s:%s" % (commit, file, rule, line)
    if fingerprint != expected:
        raise BaselineError(
            "gitleaks baseline entry %s fingerprint is not the reviewed identity"
            % index
        )
    return (rule, file, commit, line)


def _format_identities(identities):
    if not identities:
        return "none"
    items = sorted("%s %s %s %s" % item for item in identities)
    return "; ".join(items)


if __name__ == "__main__":
    sys.exit(main())
