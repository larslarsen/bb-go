"""Prove fail-closed reviewed Gitleaks baseline validation for BBGO-SEC-001."""

from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import unittest
from datetime import date
from io import StringIO
from pathlib import Path


NOW = date(2026, 8, 29)
DAY_BEFORE_EXPIRY = date(2026, 11, 28)
EXPIRY = date(2026, 11, 29)
AFTER_EXPIRY = date(2026, 11, 30)

OWNER = "Lead Engineer/Reviewer — Codex"
REVIEWED_SHA256 = "ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00"
REVIEWED_BYTES = 21758
REVIEWED_COUNT = 25
REDACTED = "REDACTED"
BASELINE_RELPATH = "security/gitleaks-baseline.json"
GITLEAKS_MODULE = "github.com/zricethezav/gitleaks/v8@v8.30.1"
MODERN_FILE = "modern/network/node.go"
ADDED_FILE = "docs/unreviewed-example.md"
ADDED_COMMIT = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

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


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def baseline_path() -> Path:
    return repo_root() / BASELINE_RELPATH


def load_validator():
    path = repo_root() / "scripts" / "gitleaks_baseline.py"
    if not path.is_file():
        raise AssertionError(
            "required validator scripts/gitleaks_baseline.py does not exist"
        )
    spec = importlib.util.spec_from_file_location("bb_go_gitleaks_baseline", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_baseline_document():
    path = baseline_path()
    if not path.is_file():
        raise AssertionError("required baseline %s does not exist" % BASELINE_RELPATH)
    return json.loads(path.read_text(encoding="utf-8"))


def identity_of(entry):
    return (entry["RuleID"], entry["File"], entry["Commit"], entry["StartLine"])


def set_identity(entry, rule, file, commit, line):
    entry["RuleID"] = rule
    entry["File"] = file
    entry["Commit"] = commit
    entry["StartLine"] = line
    entry["Fingerprint"] = "%s:%s:%s:%s" % (commit, file, rule, line)
    return entry


class RequiredSourcesExistTest(unittest.TestCase):
    def test_required_validator_exists(self):
        path = repo_root() / "scripts" / "gitleaks_baseline.py"
        self.assertTrue(
            path.is_file(),
            "required validator scripts/gitleaks_baseline.py does not exist",
        )

    def test_required_baseline_exists(self):
        self.assertTrue(
            baseline_path().is_file(),
            "required baseline %s does not exist" % BASELINE_RELPATH,
        )


class ValidatorFixture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.policy = load_validator()
        cls.error = cls.policy.BaselineError
        cls.document = load_baseline_document()

    def valid_document(self):
        return copy.deepcopy(self.document)

    def reject_document(self, document, now=NOW):
        with self.assertRaises(self.error) as ctx:
            self.policy.validate_document(document, now=now)
        return str(ctx.exception)

    def reject_bytes(self, raw, now=NOW):
        with self.assertRaises(self.error) as ctx:
            self.policy.validate_bytes(raw, now=now)
        return str(ctx.exception)


class CommittedBaselineAcceptanceTest(ValidatorFixture):
    def test_committed_baseline_hash_count_and_redaction(self):
        raw = baseline_path().read_bytes()
        self.assertEqual(hashlib.sha256(raw).hexdigest(), REVIEWED_SHA256)
        self.assertEqual(len(raw), REVIEWED_BYTES)
        self.assertEqual(len(self.document), REVIEWED_COUNT)
        identities = {identity_of(entry) for entry in self.document}
        self.assertEqual(identities, REVIEWED_IDENTITIES)
        for entry in self.document:
            self.assertEqual(entry["Secret"], REDACTED)
            self.assertIn(REDACTED, entry["Match"])
            self.assertFalse(self.policy.path_is_modern(entry["File"]))

    def test_committed_baseline_is_accepted_before_expiry(self):
        result = self.policy.validate_file(baseline_path(), now=NOW)
        self.assertEqual(result.digest, REVIEWED_SHA256)
        self.assertEqual(result.count, REVIEWED_COUNT)

    def test_day_before_expiry_is_accepted(self):
        self.policy.validate_document(self.valid_document(), now=DAY_BEFORE_EXPIRY)
        self.policy.validate_file(baseline_path(), now=DAY_BEFORE_EXPIRY)

    def test_in_memory_copy_of_committed_baseline_is_accepted(self):
        self.policy.validate_document(self.valid_document(), now=NOW)


class IdentityMutationRejectionTest(ValidatorFixture):
    def test_added_identity_is_rejected(self):
        document = self.valid_document()
        extra = copy.deepcopy(document[0])
        set_identity(
            extra,
            extra["RuleID"],
            ADDED_FILE,
            ADDED_COMMIT,
            1,
        )
        document.append(extra)
        message = self.reject_document(document)
        self.assertIn("added", message.lower())
        self.assertIn(ADDED_FILE, message)

    def test_removed_identity_is_rejected(self):
        document = self.valid_document()
        removed = identity_of(document.pop(0))
        message = self.reject_document(document)
        self.assertIn("removed", message.lower())
        self.assertIn(removed[1], message)
        self.assertIn(removed[2], message)

    def test_changed_rule_is_rejected(self):
        document = self.valid_document()
        entry = document[0]
        original = identity_of(entry)
        set_identity(entry, "aws-access-token", entry["File"], entry["Commit"], entry["StartLine"])
        message = self.reject_document(document)
        lowered = message.lower()
        self.assertTrue("changed" in lowered or "added" in lowered)
        self.assertIn("aws-access-token", message)
        self.assertIn(original[0], message)

    def test_changed_file_is_rejected(self):
        document = self.valid_document()
        entry = document[0]
        original = entry["File"]
        set_identity(entry, entry["RuleID"], "docs/changed-identity.md", entry["Commit"], entry["StartLine"])
        message = self.reject_document(document)
        self.assertIn("docs/changed-identity.md", message)
        self.assertIn(original, message)

    def test_changed_commit_is_rejected(self):
        document = self.valid_document()
        entry = document[0]
        original = entry["Commit"]
        set_identity(
            entry,
            entry["RuleID"],
            entry["File"],
            "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            entry["StartLine"],
        )
        message = self.reject_document(document)
        self.assertIn("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", message)
        self.assertIn(original, message)

    def test_changed_line_is_rejected(self):
        document = self.valid_document()
        entry = document[0]
        original = entry["StartLine"]
        set_identity(entry, entry["RuleID"], entry["File"], entry["Commit"], original + 1)
        message = self.reject_document(document)
        self.assertIn(str(original + 1), message)
        self.assertIn(str(original), message)


class ContentMutationRejectionTest(ValidatorFixture):
    def test_non_redacted_secret_is_rejected(self):
        document = self.valid_document()
        document[0]["Secret"] = "UNREDACTED_SECRET"
        message = self.reject_document(document)
        lowered = message.lower()
        self.assertIn("redact", lowered)
        self.assertNotIn("UNREDACTED_SECRET", message)

    def test_non_redacted_match_is_rejected(self):
        document = self.valid_document()
        document[0]["Match"] = 'key = "UNREDACTED_MATCH"'
        message = self.reject_document(document)
        lowered = message.lower()
        self.assertIn("redact", lowered)
        self.assertNotIn("UNREDACTED_MATCH", message)

    def test_prefix_identifier_redacted_match_is_rejected(self):
        document = self.valid_document()
        document[0]["Match"] = "PREFIX_REDACTED"
        message = self.reject_document(document)
        lowered = message.lower()
        self.assertIn("redact", lowered)
        self.assertNotIn("PREFIX_REDACTED", message)

    def test_suffix_identifier_redacted_match_is_rejected(self):
        document = self.valid_document()
        document[0]["Match"] = "REDACTED_SUFFIX"
        message = self.reject_document(document)
        lowered = message.lower()
        self.assertIn("redact", lowered)
        self.assertNotIn("REDACTED_SUFFIX", message)

    def test_duplicate_identity_is_rejected(self):
        document = self.valid_document()
        document[1] = copy.deepcopy(document[0])
        message = self.reject_document(document)
        self.assertIn("duplicate", message.lower())

    def test_modern_path_is_rejected(self):
        document = self.valid_document()
        entry = document[0]
        set_identity(entry, entry["RuleID"], MODERN_FILE, entry["Commit"], entry["StartLine"])
        message = self.reject_document(document)
        self.assertIn("modern/", message.lower())
        self.assertIn(MODERN_FILE, message)

    def test_wrong_hash_is_rejected(self):
        message = self.reject_bytes(b"[]")
        lowered = message.lower()
        self.assertIn("sha-256", lowered)
        self.assertIn(REVIEWED_SHA256, lowered)

    def test_wrong_count_is_rejected(self):
        document = self.valid_document()[:20]
        message = self.reject_document(document)
        self.assertIn("count", message.lower())
        self.assertIn("20", message)
        self.assertIn("expected %s" % REVIEWED_COUNT, message)


class ExpiryRejectionTest(ValidatorFixture):
    def test_exception_on_expiry_date_is_rejected(self):
        message = self.reject_document(self.valid_document(), now=EXPIRY)
        self.assertIn("expir", message.lower())
        self.assertIn("2026-11-29", message)

    def test_after_expiry_is_rejected(self):
        message = self.reject_document(self.valid_document(), now=AFTER_EXPIRY)
        self.assertIn("expir", message.lower())
        self.assertIn("2026-11-29", message)


class MetadataAndSummaryTest(ValidatorFixture):
    def test_metadata_matches_ticket(self):
        self.assertEqual(self.policy.REVIEWED_BASELINE_SHA256, REVIEWED_SHA256)
        self.assertEqual(self.policy.REVIEWED_BASELINE_BYTES, REVIEWED_BYTES)
        self.assertEqual(self.policy.REVIEWED_BASELINE_COUNT, REVIEWED_COUNT)
        self.assertEqual(self.policy.REVIEWED_BASELINE_EXPIRY, EXPIRY)
        self.assertEqual(self.policy.REVIEWED_BASELINE_OWNER, OWNER)
        self.assertEqual(self.policy.REVIEWED_BASELINE_RELPATH, BASELINE_RELPATH)
        self.assertEqual(self.policy.REQUIRED_GITLEAKS_MODULE, GITLEAKS_MODULE)
        self.assertEqual(self.policy.REVIEWED_IDENTITIES, REVIEWED_IDENTITIES)

    def test_copy_does_not_mutate_reviewed_identities(self):
        document = self.valid_document()
        set_identity(
            document[0],
            document[0]["RuleID"],
            ADDED_FILE,
            ADDED_COMMIT,
            99,
        )
        self.policy.validate_document(self.valid_document(), now=NOW)
        self.assertEqual(self.policy.REVIEWED_IDENTITIES, REVIEWED_IDENTITIES)

    def test_main_accepts_unexpired_committed_baseline(self):
        out = StringIO()
        err = StringIO()
        code = self.policy.main(
            argv=[],
            now=NOW,
            repo_root=repo_root(),
            stdout=out,
            stderr=err,
        )
        self.assertEqual(code, 0)
        summary = out.getvalue()
        self.assertIn(str(REVIEWED_COUNT), summary)
        self.assertIn(OWNER, summary)
        self.assertIn("2026-11-29", summary)
        self.assertIn(REVIEWED_SHA256, summary)
        self.assertNotIn('"Secret"', summary)
        self.assertNotIn('"Match"', summary)

    def test_main_rejects_on_expiry_date(self):
        out = StringIO()
        err = StringIO()
        code = self.policy.main(
            argv=[],
            now=EXPIRY,
            repo_root=repo_root(),
            stdout=out,
            stderr=err,
        )
        self.assertEqual(code, 1)
        self.assertIn("expir", err.getvalue().lower())
        self.assertNotIn('"Secret"', err.getvalue())
        self.assertNotIn('"Match"', err.getvalue())

    def test_successful_summary_omits_finding_bodies(self):
        result = self.policy.validate_file(baseline_path(), now=NOW)
        summary = result.summary()
        self.assertIn("accepted", summary.lower())
        self.assertIn(OWNER, summary)
        self.assertNotIn('"Secret"', summary)
        self.assertNotIn('"Match"', summary)
        self.assertNotIn("Fingerprint", summary)


if __name__ == "__main__":
    unittest.main()
