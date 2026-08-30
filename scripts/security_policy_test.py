"""Prove maintained-daemon security gates, immutable pins, and manual-only SBOM policy."""

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path


CHECKOUT_SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
CHECKOUT_TAG = "v7.0.1"
SETUP_GO_SHA = "b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"
SETUP_GO_TAG = "v7.0.0"
UPLOAD_SHA = "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
UPLOAD_TAG = "v7.0.1"
GO_VERSION = "1.27.0"
MODERN_MODULE = "github.com/larslarsen/bb-go/modern"

GOVULNCHECK_MODULE = "golang.org/x/vuln/cmd/govulncheck@v1.7.0"
GOSEC_MODULE = "github.com/securego/gosec/v2/cmd/gosec@v2.29.0"
GITLEAKS_MODULE = "github.com/zricethezav/gitleaks/v8@v8.30.1"
CYCLONEDX_MODULE = "github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0"
ACTIONLINT_MODULE = "github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"

GO_PATHS = [
    "*.go",
    "**/*.go",
    "go.mod",
    "go.sum",
    "**/go.mod",
    "**/go.sum",
    "vendor/**",
    "gx/**",
    "scripts/go.sh",
    ".github/workflows/go.yml",
]
SECURITY_PATHS = [
    "modern/**",
    ".github/workflows/**",
    "scripts/security_policy.py",
    "scripts/security_policy_test.py",
    "scripts/govulncheck_policy.py",
    "scripts/govulncheck_policy_test.py",
    "scripts/gitleaks_baseline.py",
    "scripts/gitleaks_baseline_test.py",
    "security/gitleaks-baseline.json",
]
FORBIDDEN_GO_PATHS = {
    "**",
    "**/*",
    "docs/**",
    "**/*.md",
    "*.md",
    "README.md",
}

COMPILE_CMD = "./scripts/go.sh test -vet=off ./... -run '^$' -count=1"
SOCIAL_CMD = (
    "./scripts/go.sh test -vet=off ./api ./net/service ./net/retriever "
    "./core ./cmd ./ipfs ./schema -run 'TestSocialOnly|^$' -count=1"
)
MODERN_TEST_CMD = "go test ./... -count=1"
GITLEAKS_CMD = (
    "gitleaks git --redact=100 --no-banner "
    "--baseline-path security/gitleaks-baseline.json ."
)
GITLEAKS_BASELINE_CMD = "python3 scripts/gitleaks_baseline.py"
GITLEAKS_BASELINE_TEST_CMD = "python3 -m unittest scripts/gitleaks_baseline_test.py"
POLICY_TEST_CMD = "python3 -m unittest scripts/security_policy_test.py"
GOVULNCHECK_POLICY_TEST_CMD = "python3 -m unittest scripts/govulncheck_policy_test.py"
ACTIONLINT_CMD = (
    "actionlint .github/workflows/go.yml "
    ".github/workflows/security.yml .github/workflows/sbom.yml"
)
DIVERSITY_TEST_CMD = (
    "go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1"
)
GOVULNCHECK_SOURCE_CMD = "python3 scripts/govulncheck_policy.py source"
GOVULNCHECK_BINARY_CMD = 'python3 scripts/govulncheck_policy.py binary "${binary}"'
GOSEC_CMD = "gosec ./..."

REQUIRED_PATHS = [
    "scripts/security_policy.py",
    "scripts/govulncheck_policy.py",
    "scripts/gitleaks_baseline.py",
    "scripts/gitleaks_baseline_test.py",
    "security/gitleaks-baseline.json",
    ".github/workflows/go.yml",
    ".github/workflows/security.yml",
    ".github/workflows/sbom.yml",
]


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def load_policy():
    path = repo_root() / "scripts" / "security_policy.py"
    if not path.is_file():
        raise AssertionError("required checker scripts/security_policy.py does not exist")
    spec = importlib.util.spec_from_file_location("bb_go_security_policy", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class RequiredSourcesExistTest(unittest.TestCase):
    def test_required_policy_sources_exist(self):
        root = repo_root()
        missing = [rel for rel in REQUIRED_PATHS if not (root / rel).is_file()]
        self.assertEqual(
            missing,
            [],
            "required checker/workflows do not yet exist: %s" % ", ".join(missing),
        )


class PolicyFixture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = repo_root()
        cls.policy = load_policy()
        cls.go = cls.policy.load_workflow(cls.root / ".github/workflows/go.yml")
        cls.security = cls.policy.load_workflow(
            cls.root / ".github/workflows/security.yml"
        )
        cls.sbom = cls.policy.load_workflow(cls.root / ".github/workflows/sbom.yml")

    def commands(self, workflow) -> list[str]:
        lines = []
        for _job_name, _job, step in self.policy.iter_steps(workflow.data):
            lines.extend(self.policy.step_run_lines(step))
        return lines

    def combined_run_text(self, workflow) -> str:
        return "\n".join(
            self.policy.step_run_text(step)
            for _job_name, _job, step in self.policy.iter_steps(workflow.data)
        )

    def env_for_command(self, workflow, needle: str) -> dict:
        for _job_name, job, step in self.policy.iter_steps(workflow.data):
            run = self.policy.step_run_text(step)
            if needle in run:
                env = {}
                env.update(job.get("env") or {})
                env.update(step.get("env") or {})
                return {str(key): value for key, value in env.items()}
        self.fail("command %r is not present in %s" % (needle, workflow.path.name))


class SecurityTriggerInvariantTest(PolicyFixture):
    """Invariant 1: security checks are pull_request + manual only; no push copy."""

    def test_security_workflow_triggers_are_pull_request_and_manual_only(self):
        triggers = self.policy.event_triggers(self.security.data)
        self.assertCountEqual(triggers.keys(), ["pull_request", "workflow_dispatch"])
        self.assertNotIn("push", triggers)
        paths = self.policy.trigger_paths(self.security.data, "pull_request")
        self.assertEqual(paths, SECURITY_PATHS)
        dispatch = triggers["workflow_dispatch"]
        self.assertTrue(dispatch is None or dispatch == {})
        with self.assertRaises(self.policy.PolicyError):
            self.policy.trigger_paths(self.security.data, "push")


class NoRoutineBinaryInvariantTest(PolicyFixture):
    """Invariant 2: routine security/Go validation never builds or uploads a product binary."""

    def test_routine_security_validation_does_not_build_or_upload_binaries(self):
        uses = list(self.policy.iter_action_uses(self.security.text))
        self.assertTrue(uses)
        for use in uses:
            self.assertNotEqual(use.action, "actions/upload-artifact")
        run_text = self.combined_run_text(self.security)
        self.assertNotRegex(run_text, r"\bgo\s+build\b")
        self.assertNotRegex(run_text, r"\bgo\s+install\s+(?:\./)?cmd/bitbookd\b")
        self.assertNotIn("cyclonedx-gomod", run_text)
        self.assertNotIn("upload-artifact", self.security.text)
        self.assertNotIn("softprops/action-gh-release", self.security.text)

    def test_go_workflow_does_not_build_or_upload_product_binaries(self):
        uses = list(self.policy.iter_action_uses(self.go.text))
        self.assertTrue(uses)
        for use in uses:
            self.assertNotEqual(use.action, "actions/upload-artifact")
        run_text = self.combined_run_text(self.go)
        self.assertNotRegex(run_text, r"\bgo\s+build\b")
        self.assertNotIn("upload-artifact", self.go.text)
        self.assertIn(COMPILE_CMD, self.commands(self.go))
        self.assertIn(SOCIAL_CMD, self.commands(self.go))
        self.assertIn(MODERN_TEST_CMD, self.commands(self.go))


class ManualSbomInvariantTest(PolicyFixture):
    """Invariant 3: SBOM is manual, same target env as the ephemeral binary, CycloneDX JSON only."""

    def test_sbom_workflow_is_manual_and_uploads_only_cyclonedx_json(self):
        triggers = self.policy.event_triggers(self.sbom.data)
        self.assertCountEqual(triggers.keys(), ["workflow_dispatch"])
        self.assertNotIn("push", triggers)
        self.assertNotIn("pull_request", triggers)

        uploads = []
        for _job_name, _job, step in self.policy.iter_steps(self.sbom.data):
            uses = str(step.get("uses") or "")
            if uses.split("@", 1)[0] == "actions/upload-artifact":
                uploads.append(step)
        self.assertEqual(len(uploads), 1, "manual SBOM job must upload exactly one artifact")
        uploaded = uploads[0].get("with") or {}
        path = uploaded.get("path")
        self.assertIsInstance(path, str)
        self.assertTrue(path.endswith(".cdx.json"), path)
        self.assertNotIn("*", path)
        self.assertNotIn("\n", path)
        self.assertNotEqual(Path(str(path).replace("\\", "/")).name, "bitbookd")
        self.assertEqual(int(uploaded.get("retention-days")), 7)

        run_text = self.combined_run_text(self.sbom)
        self.assertIn("go build", run_text)
        self.assertIn("./cmd/bitbookd", run_text)
        self.assertIn("RUNNER_TEMP", run_text)
        self.assertIn(GOVULNCHECK_BINARY_CMD, run_text)
        self.assertNotRegex(run_text, r"(?m)^\s*govulncheck\s")
        self.assertIn("cyclonedx-gomod app", run_text)
        self.assertIn("-json", run_text)
        self.assertIn("-packages", run_text)
        self.assertIn("-licenses", run_text)
        self.assertIn("-main cmd/bitbookd", run_text)
        self.assertIn("validate_cyclonedx_file", run_text)
        self.assertNotRegex(run_text, r"go\s+build[^\n]*-o\s+\./")
        self.assertNotIn("action-gh-release", self.sbom.text)
        self.assertNotRegex(run_text, r"(?m)^\s*(rm|rmdir|unlink|shred)\b")
        self.assertNotRegex(run_text, r"(?m)^\s*find\b[^\n]*\s-delete\b")
        self.assertNotRegex(run_text, r"(?m)^\s*find\b[^\n]*\s-exec\s+rm\b")

    def test_sbom_uses_same_target_environment_for_binary_and_document(self):
        build_env = self.env_for_command(self.sbom, "go build")
        sbom_env = self.env_for_command(self.sbom, "cyclonedx-gomod app")
        self.assertEqual(str(build_env.get("GOOS")), "linux")
        self.assertEqual(str(build_env.get("GOARCH")), "amd64")
        self.assertEqual(str(build_env.get("CGO_ENABLED")), "0")
        self.assertEqual(build_env, sbom_env)
        build_step = None
        sbom_step = None
        for _job_name, _job, step in self.policy.iter_steps(self.sbom.data):
            run = self.policy.step_run_text(step)
            if "go build" in run:
                build_step = step
            if "cyclonedx-gomod app" in run:
                sbom_step = step
        self.assertIsNotNone(build_step)
        self.assertIs(build_step, sbom_step)


class PinAndPermissionInvariantTest(PolicyFixture):
    """Invariant 4: immutable Action SHAs, exact tool versions, read-only permissions."""

    def test_third_party_actions_are_immutable_sha_pins(self):
        expected = {
            "go.yml": {
                "actions/checkout": (CHECKOUT_SHA, CHECKOUT_TAG),
                "actions/setup-go": (SETUP_GO_SHA, SETUP_GO_TAG),
            },
            "security.yml": {
                "actions/checkout": (CHECKOUT_SHA, CHECKOUT_TAG),
                "actions/setup-go": (SETUP_GO_SHA, SETUP_GO_TAG),
            },
            "sbom.yml": {
                "actions/checkout": (CHECKOUT_SHA, CHECKOUT_TAG),
                "actions/setup-go": (SETUP_GO_SHA, SETUP_GO_TAG),
                "actions/upload-artifact": (UPLOAD_SHA, UPLOAD_TAG),
            },
        }
        for workflow in (self.go, self.security, self.sbom):
            uses = list(self.policy.iter_action_uses(workflow.text))
            self.assertTrue(uses, workflow.path.name)
            seen = {}
            for use in uses:
                self.assertRegex(
                    use.ref,
                    r"^[0-9a-f]{40}$",
                    "%s:%s uses mutable ref %s" % (workflow.path.name, use.line, use.ref),
                )
                seen[use.action] = (use.ref, use.comment)
            self.assertEqual(seen, expected[workflow.path.name], workflow.path.name)

    def test_tool_installs_are_exact_versions(self):
        security_text = self.combined_run_text(self.security)
        sbom_text = self.combined_run_text(self.sbom)
        for module in (
            GOVULNCHECK_MODULE,
            GOSEC_MODULE,
            GITLEAKS_MODULE,
            ACTIONLINT_MODULE,
        ):
            self.assertIn("go install %s" % module, security_text)
        self.assertNotIn("cyclonedx-gomod", security_text)
        self.assertIn("go install %s" % GOVULNCHECK_MODULE, sbom_text)
        self.assertIn("go install %s" % CYCLONEDX_MODULE, sbom_text)
        self.assertNotIn("@latest", security_text)
        self.assertNotIn("@latest", sbom_text)
        self.assertIn("runner.temp", self.security.text)
        self.assertIn("runner.temp", self.sbom.text)

        for workflow in (self.go, self.security, self.sbom):
            found_go = False
            for _job_name, _job, step in self.policy.iter_steps(workflow.data):
                uses = str(step.get("uses") or "")
                if uses.split("@", 1)[0] != "actions/setup-go":
                    continue
                found_go = True
                with_ = step.get("with") or {}
                self.assertEqual(str(with_.get("go-version")), GO_VERSION)
                self.assertIs(with_.get("cache"), False)
            self.assertTrue(found_go, workflow.path.name)

    def test_workflow_permissions_are_contents_read_only(self):
        for workflow in (self.go, self.security, self.sbom):
            self.assertEqual(
                workflow.data.get("permissions"),
                {"contents": "read"},
                workflow.path.name,
            )
            self.assertNotIn("write", str(workflow.data.get("permissions")))
            for job_name, job, _step in self.policy.iter_steps(workflow.data):
                job_permissions = job.get("permissions")
                if job_permissions is None:
                    continue
                self.fail(
                    "%s job %s sets extra permissions %r"
                    % (workflow.path.name, job_name, job_permissions)
                )


class NoSuppressionInvariantTest(PolicyFixture):
    """Invariant 5: no finding is suppressed, allowlisted, downgraded, or made non-blocking."""

    def test_no_finding_is_suppressed_or_non_blocking(self):
        for workflow in (self.go, self.security, self.sbom):
            self.assertNotIn("continue-on-error", workflow.text)
            run_text = self.combined_run_text(workflow)
            self.assertNotRegex(run_text, r"\|\|\s*true\b")
            self.assertNotRegex(run_text, r"\bset\s+\+e\b")
            self.assertNotRegex(run_text, r"--exit-code\s*=?\s*0")
            self.assertNotRegex(run_text, r"--no-fail\b")
            self.assertNotRegex(run_text, r"\bgosec\b[^\n]*-exclude")
            self.assertNotRegex(run_text, r"\bgovulncheck\b[^\n]*-ignore")
            self.assertNotRegex(run_text, r"\bgitleaks\b[^\n]*--baseline(?:\s|=|$)")
            self.assertNotRegex(
                run_text,
                r"--baseline-path\s+(?!security/gitleaks-baseline\.json)",
            )
            self.assertNotIn(".gitleaksignore", workflow.text)
            self.assertNotIn("--gitleaks-ignore-path", run_text)
            self.assertNotIn("#nosec", workflow.text)
            self.assertNotIn("allowlist", workflow.text.lower())
            for _job_name, _job, step in self.policy.iter_steps(workflow.data):
                self.assertNotIn("continue-on-error", step)


class SecretRedactionInvariantTest(PolicyFixture):
    """Invariant 6: secret values are redacted and no secret-finding report is uploaded."""

    def test_gitleaks_redacts_secrets_and_uploads_no_finding_report(self):
        commands = self.commands(self.security)
        self.assertIn(GITLEAKS_BASELINE_CMD, commands)
        self.assertIn(GITLEAKS_CMD, commands)
        self.assertEqual(
            commands.index(GITLEAKS_CMD),
            commands.index(GITLEAKS_BASELINE_CMD) + 1,
        )
        gitleaks_lines = [
            line for line in commands if line.startswith("gitleaks ")
        ]
        self.assertEqual(gitleaks_lines, [GITLEAKS_CMD])
        self.assertIn("--redact=100", gitleaks_lines[0])
        self.assertIn(
            "--baseline-path security/gitleaks-baseline.json", gitleaks_lines[0]
        )
        self.assertNotIn("--report-path", self.combined_run_text(self.security))
        self.assertNotRegex(
            self.combined_run_text(self.security),
            r"(?m)^\s*gitleaks\b[^\n]*\s-r\s",
        )
        for use in self.policy.iter_action_uses(self.security.text):
            self.assertNotEqual(use.action, "actions/upload-artifact")
        for _job_name, job, step in self.policy.iter_steps(self.security.data):
            working = step.get("working-directory") or job.get("working-directory")
            lines = self.policy.step_run_lines(step)
            if GITLEAKS_CMD in lines or GITLEAKS_BASELINE_CMD in lines:
                self.assertIn(working, (None, ".", ""))


class GoWorkflowDocsFilterInvariantTest(PolicyFixture):
    """Invariant 7: Go workflow uses immutable pins and skips documentation-only changes."""

    def test_go_workflow_has_immutable_pins_and_documentation_path_filter(self):
        triggers = self.policy.event_triggers(self.go.data)
        self.assertCountEqual(triggers.keys(), ["push", "pull_request"])
        for event in ("push", "pull_request"):
            paths = self.policy.trigger_paths(self.go.data, event)
            self.assertEqual(paths, GO_PATHS, event)
            self.assertTrue(set(FORBIDDEN_GO_PATHS).isdisjoint(paths), paths)
            self.assertNotIn("docs/**", paths)
            self.assertNotIn("**/*.md", paths)
        uses = {use.action: use for use in self.policy.iter_action_uses(self.go.text)}
        self.assertEqual(uses["actions/checkout"].ref, CHECKOUT_SHA)
        self.assertEqual(uses["actions/checkout"].comment, CHECKOUT_TAG)
        self.assertEqual(uses["actions/setup-go"].ref, SETUP_GO_SHA)
        self.assertEqual(uses["actions/setup-go"].comment, SETUP_GO_TAG)
        self.assertNotIn("actions/checkout@v5", self.go.text)
        self.assertNotIn("actions/setup-go@v6", self.go.text)


class RequiredCommandsTest(PolicyFixture):
    def test_security_runs_required_scanners_and_policy_checks(self):
        commands = self.commands(self.security)
        self.assertIn(DIVERSITY_TEST_CMD, commands)
        self.assertIn(GOVULNCHECK_SOURCE_CMD, commands)
        self.assertEqual(
            commands.index(GOVULNCHECK_SOURCE_CMD),
            commands.index(DIVERSITY_TEST_CMD) + 1,
        )
        self.assertIn(GOSEC_CMD, commands)
        self.assertIn(GITLEAKS_BASELINE_CMD, commands)
        self.assertIn(GITLEAKS_CMD, commands)
        self.assertEqual(
            commands.index(GITLEAKS_CMD),
            commands.index(GITLEAKS_BASELINE_CMD) + 1,
        )
        self.assertIn(POLICY_TEST_CMD, commands)
        self.assertIn(GOVULNCHECK_POLICY_TEST_CMD, commands)
        self.assertIn(GITLEAKS_BASELINE_TEST_CMD, commands)
        self.assertIn(ACTIONLINT_CMD, commands)
        self.assertNotIn("govulncheck -test ./...", commands)

        for _job_name, job, step in self.policy.iter_steps(self.security.data):
            run_lines = self.policy.step_run_lines(step)
            working = step.get("working-directory")
            if DIVERSITY_TEST_CMD in run_lines or GOSEC_CMD in run_lines:
                self.assertEqual(working, "modern")
            if GOVULNCHECK_SOURCE_CMD in run_lines:
                self.assertNotEqual(working, "modern")
                self.assertNotEqual(
                    (job.get("defaults") or {}).get("run"),
                    {"working-directory": "modern"},
                )
            if (
                GITLEAKS_CMD in run_lines
                or GITLEAKS_BASELINE_CMD in run_lines
                or POLICY_TEST_CMD in run_lines
            ):
                self.assertNotEqual(working, "modern")
                self.assertNotEqual((job.get("defaults") or {}).get("run"), {"working-directory": "modern"})
            if GOVULNCHECK_POLICY_TEST_CMD in run_lines:
                self.assertNotEqual(working, "modern")
            if GITLEAKS_BASELINE_TEST_CMD in run_lines:
                self.assertNotEqual(working, "modern")

        checkout_seen = False
        for _job_name, _job, step in self.policy.iter_steps(self.security.data):
            uses = str(step.get("uses") or "")
            if uses.split("@", 1)[0] != "actions/checkout":
                continue
            checkout_seen = True
            self.assertEqual((step.get("with") or {}).get("fetch-depth"), 0)
        self.assertTrue(checkout_seen)

        for workflow in (self.security, self.sbom):
            concurrency = workflow.data.get("concurrency")
            self.assertIsInstance(concurrency, dict, workflow.path.name)
            self.assertTrue(str(concurrency.get("group") or "").strip(), workflow.path.name)
            self.assertIs(concurrency.get("cancel-in-progress"), True, workflow.path.name)

    def test_go_workflow_preserves_existing_commands(self):
        commands = self.commands(self.go)
        self.assertEqual(commands.count(COMPILE_CMD), 1)
        self.assertEqual(commands.count(SOCIAL_CMD), 1)
        self.assertEqual(commands.count(MODERN_TEST_CMD), 1)
        modern_step = None
        for _job_name, _job, step in self.policy.iter_steps(self.go.data):
            if MODERN_TEST_CMD in self.policy.step_run_lines(step):
                modern_step = step
        self.assertIsNotNone(modern_step)
        self.assertEqual(modern_step.get("working-directory"), "modern")

    def test_checker_constants_match_ticket_pins(self):
        self.assertEqual(self.policy.CHECKOUT_SHA, CHECKOUT_SHA)
        self.assertEqual(self.policy.CHECKOUT_TAG, CHECKOUT_TAG)
        self.assertEqual(self.policy.SETUP_GO_SHA, SETUP_GO_SHA)
        self.assertEqual(self.policy.SETUP_GO_TAG, SETUP_GO_TAG)
        self.assertEqual(self.policy.UPLOAD_ARTIFACT_SHA, UPLOAD_SHA)
        self.assertEqual(self.policy.UPLOAD_ARTIFACT_TAG, UPLOAD_TAG)
        self.assertEqual(self.policy.GO_VERSION, GO_VERSION)
        self.assertEqual(self.policy.MODERN_MODULE, MODERN_MODULE)
        self.assertEqual(self.policy.SECURITY_PATHS, SECURITY_PATHS)
        self.assertEqual(self.policy.DIVERSITY_TEST_CMD, DIVERSITY_TEST_CMD)
        self.assertEqual(self.policy.GOVULNCHECK_SOURCE_CMD, GOVULNCHECK_SOURCE_CMD)
        self.assertEqual(self.policy.GOVULNCHECK_BINARY_CMD, GOVULNCHECK_BINARY_CMD)
        self.assertEqual(
            self.policy.GOVULNCHECK_POLICY_TEST_CMD, GOVULNCHECK_POLICY_TEST_CMD
        )
        self.assertEqual(self.policy.GITLEAKS_CMD, GITLEAKS_CMD)
        self.assertEqual(self.policy.GITLEAKS_BASELINE_CMD, GITLEAKS_BASELINE_CMD)
        self.assertEqual(
            self.policy.GITLEAKS_BASELINE_TEST_CMD, GITLEAKS_BASELINE_TEST_CMD
        )

    def test_yaml_parser_preserves_on_key_and_block_scalars(self):
        data = self.policy.parse_yaml(
            "\n".join(
                [
                    "on:",
                    "  pull_request:",
                    "    paths:",
                    '      - "modern/**"',
                    "  workflow_dispatch:",
                    "jobs:",
                    "  scan:",
                    "    steps:",
                    "      - run: |",
                    "          echo one",
                    "          echo two",
                ]
            )
        )
        self.assertIn("on", data)
        self.assertNotIsInstance(data["on"], bool)
        self.assertEqual(data["on"]["pull_request"]["paths"], ["modern/**"])
        self.assertIn("workflow_dispatch", data["on"])
        run = data["jobs"]["scan"]["steps"][0]["run"]
        self.assertIn("echo one", run)
        self.assertIn("echo two", run)

    def test_committed_workflows_satisfy_the_checker(self):
        self.policy.check_repository(self.root)


class CheckerRejectionTest(PolicyFixture):
    def test_mutable_action_tag_is_rejected(self):
        mutated = self.go.text.replace(CHECKOUT_SHA, "v7.0.1", 1)
        self.assertNotEqual(mutated, self.go.text)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_go_workflow(mutated)

    def test_security_push_trigger_is_rejected(self):
        mutated = self.security.text.replace("on:\n", "on:\n  push:\n", 1)
        self.assertIn("\n  push:\n", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_sbom_binary_upload_is_rejected(self):
        mutated = self.sbom.text.replace(
            "path: ${{ runner.temp }}/bb-go-sbom/bitbookd.cdx.json",
            "path: ${{ runner.temp }}/bb-go-sbom/bitbookd",
            1,
        )
        self.assertNotEqual(mutated, self.sbom.text)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_sbom_workflow(mutated)

    def test_go_workflow_docs_path_is_rejected(self):
        mutated = self.go.text.replace('- "*.go"\n', '- "*.go"\n      - "docs/**"\n', 1)
        self.assertIn("docs/**", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_go_workflow(mutated)

    def test_continue_on_error_is_rejected(self):
        mutated = self.security.text.replace(
            "runs-on: ubuntu-latest\n",
            "runs-on: ubuntu-latest\n    continue-on-error: true\n",
            1,
        )
        self.assertIn("continue-on-error", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_write_permission_is_rejected(self):
        mutated = self.security.text.replace("contents: read", "contents: write", 1)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_missing_gitleaks_redact_is_rejected(self):
        mutated = self.security.text.replace(" --redact=100", "", 1)
        self.assertNotIn(GITLEAKS_CMD, mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_gitleaks_redact_without_percent_is_rejected(self):
        mutated = self.security.text.replace("--redact=100", "--redact", 1)
        self.assertNotIn("--redact=100", mutated)
        self.assertIn("gitleaks git --redact --no-banner", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_unpinned_go_install_is_rejected(self):
        mutated = self.security.text.replace("@v1.7.0", "@latest", 1)
        self.assertIn("@latest", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_raw_govulncheck_source_is_rejected(self):
        mutated = self.security.text.replace(
            GOVULNCHECK_SOURCE_CMD, "govulncheck -test ./...", 1
        )
        self.assertIn("govulncheck -test ./...", mutated)
        self.assertNotIn(GOVULNCHECK_SOURCE_CMD, mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_missing_diversity_test_is_rejected(self):
        mutated = self.security.text.replace(
            "      - name: Prove DHT routing-table IP diversity\n"
            "        working-directory: modern\n"
            "        run: %s\n\n" % DIVERSITY_TEST_CMD,
            "",
            1,
        )
        self.assertNotIn(DIVERSITY_TEST_CMD, mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_diversity_test_not_immediately_before_scan_is_rejected(self):
        mutated = self.security.text.replace(
            "        run: %s\n\n      - name: Run govulncheck\n" % DIVERSITY_TEST_CMD,
            "        run: %s\n\n"
            "      - name: Interrupt diversity and scan\n"
            "        run: echo interrupt\n\n"
            "      - name: Run govulncheck\n" % DIVERSITY_TEST_CMD,
            1,
        )
        self.assertIn("echo interrupt", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_sbom_raw_govulncheck_is_rejected(self):
        mutated = self.sbom.text.replace(
            GOVULNCHECK_BINARY_CMD, "govulncheck -mode binary \"${binary}\"", 1
        )
        self.assertIn("govulncheck -mode binary", mutated)
        self.assertNotIn(GOVULNCHECK_BINARY_CMD, mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_sbom_workflow(mutated)

    def test_missing_govulncheck_policy_trigger_path_is_rejected(self):
        mutated = self.security.text.replace(
            '      - "scripts/govulncheck_policy.py"\n',
            "",
            1,
        )
        self.assertNotIn('scripts/govulncheck_policy.py"', mutated.split("jobs:", 1)[0])
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_adjudicator_continue_on_error_is_rejected(self):
        mutated = self.security.text.replace(
            "      - name: Run govulncheck\n        run: %s\n" % GOVULNCHECK_SOURCE_CMD,
            "      - name: Run govulncheck\n"
            "        continue-on-error: true\n"
            "        run: %s\n" % GOVULNCHECK_SOURCE_CMD,
            1,
        )
        self.assertIn("continue-on-error: true", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def sbom_with_run_line(self, command):
        marker = 'validate_cyclonedx_file(Path(sys.argv[1]))\' "${sbom}"\n'
        mutated = self.sbom.text.replace(
            marker,
            marker + "          %s\n" % command,
            1,
        )
        self.assertNotEqual(mutated, self.sbom.text)
        self.assertIn(command, mutated)
        return mutated

    def reject_sbom_deletion(self, command):
        mutated = self.sbom_with_run_line(command)
        with self.assertRaises(self.policy.PolicyError) as ctx:
            self.policy.check_sbom_workflow(mutated)
        message = str(ctx.exception).lower()
        self.assertIn("delete", message)
        self.assertTrue(
            any(
                token in message
                for token in ("variable", "substitution", "glob", "symlink")
            ),
            message,
        )

    def test_sbom_variable_deletion_is_rejected(self):
        self.reject_sbom_deletion('rm -f "${binary}"')
        self.reject_sbom_deletion('rm -f "$RUNNER_TEMP/bitbookd"')
        self.reject_sbom_deletion('rm -f ${binary}')

    def test_sbom_command_substitution_deletion_is_rejected(self):
        self.reject_sbom_deletion("rm -f $(printf /tmp/bitbookd)")
        self.reject_sbom_deletion("rm -f `/bin/echo /tmp/bitbookd`")

    def test_sbom_glob_deletion_is_rejected(self):
        self.reject_sbom_deletion("rm -f /tmp/bb-go-sbom/*")
        self.reject_sbom_deletion("rm -f /tmp/bb-go-sbom/bitbookd?")
        self.reject_sbom_deletion("rm -f /tmp/bb-go-sbom/bitbookd[0-9]")

    def test_sbom_symlink_derived_deletion_is_rejected(self):
        self.reject_sbom_deletion("rm -f $(readlink -f /tmp/bitbookd)")
        self.reject_sbom_deletion("rm -f $(realpath /tmp/bitbookd)")
        self.reject_sbom_deletion("rm -f `/bin/readlink -f /tmp/bitbookd`")

    def test_gitleaks_alternate_baseline_is_rejected(self):
        mutated = self.security.text.replace(
            "--baseline-path security/gitleaks-baseline.json",
            "--baseline-path security/other-baseline.json",
            1,
        )
        self.assertIn("other-baseline.json", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_gitleaks_ignore_path_allowlist_is_rejected(self):
        mutated = self.security.text.replace(
            GITLEAKS_CMD,
            GITLEAKS_CMD + " --gitleaks-ignore-path .gitleaksignore",
            1,
        )
        self.assertIn(".gitleaksignore", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_gitleaks_config_allowlist_is_rejected(self):
        mutated = self.security.text.replace(
            GITLEAKS_CMD,
            GITLEAKS_CMD + " --config .gitleaks.toml",
            1,
        )
        self.assertIn("--config .gitleaks.toml", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_gitleaks_commit_allowlist_is_rejected(self):
        mutated = self.security.text.replace(
            "runs-on: ubuntu-latest\n",
            "runs-on: ubuntu-latest\n    env:\n      allowlist: commits\n",
            1,
        )
        self.assertIn("allowlist", mutated.lower())
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_missing_gitleaks_baseline_validator_is_rejected(self):
        mutated = self.security.text.replace(
            "      - name: Validate Gitleaks baseline\n"
            "        run: %s\n\n" % GITLEAKS_BASELINE_CMD,
            "",
            1,
        )
        self.assertNotIn(GITLEAKS_BASELINE_CMD, mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_gitleaks_validator_not_immediately_before_scan_is_rejected(self):
        mutated = self.security.text.replace(
            "        run: %s\n\n      - name: Run gitleaks\n" % GITLEAKS_BASELINE_CMD,
            "        run: %s\n\n"
            "      - name: Interrupt baseline and gitleaks\n"
            "        run: echo interrupt\n\n"
            "      - name: Run gitleaks\n" % GITLEAKS_BASELINE_CMD,
            1,
        )
        self.assertIn("echo interrupt", mutated)
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_missing_gitleaks_baseline_trigger_path_is_rejected(self):
        mutated = self.security.text.replace(
            '      - "scripts/gitleaks_baseline.py"\n',
            "",
            1,
        )
        self.assertNotIn('scripts/gitleaks_baseline.py"', mutated.split("jobs:", 1)[0])
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)

    def test_missing_gitleaks_baseline_json_trigger_path_is_rejected(self):
        mutated = self.security.text.replace(
            '      - "security/gitleaks-baseline.json"\n',
            "",
            1,
        )
        self.assertNotIn(
            'security/gitleaks-baseline.json"', mutated.split("jobs:", 1)[0]
        )
        with self.assertRaises(self.policy.PolicyError):
            self.policy.check_security_workflow(mutated)


class CycloneDXValidatorTest(PolicyFixture):
    def valid_document(self):
        return {
            "bomFormat": "CycloneDX",
            "specVersion": "1.6",
            "metadata": {
                "component": {
                    "name": MODERN_MODULE,
                    "purl": "pkg:golang/%s@v0.0.0" % MODERN_MODULE,
                }
            },
            "components": [{"name": "github.com/libp2p/go-libp2p"}],
            "dependencies": [
                {
                    "ref": MODERN_MODULE,
                    "dependsOn": ["github.com/libp2p/go-libp2p"],
                }
            ],
        }

    def test_validator_accepts_cyclonedx_document_for_modern_module(self):
        self.policy.validate_cyclonedx_document(self.valid_document())

    def test_validator_rejects_non_json_object(self):
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_document(["CycloneDX"])

    def test_validator_rejects_missing_cyclonedx_declaration(self):
        document = self.valid_document()
        document["bomFormat"] = "spdx"
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_document(document)

    def test_validator_rejects_wrong_root_component(self):
        document = self.valid_document()
        document["metadata"]["component"] = {"name": "github.com/openbazaar/openbazaar-go"}
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_document(document)

    def test_validator_rejects_empty_components(self):
        document = self.valid_document()
        document["components"] = []
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_document(document)

    def test_validator_rejects_empty_dependencies(self):
        document = self.valid_document()
        document["dependencies"] = []
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_document(document)

    def test_validator_rejects_invalid_json_file(self):
        path = self.root / "scripts" / "security_policy.py"
        with self.assertRaises(self.policy.PolicyError):
            self.policy.validate_cyclonedx_file(path)


if __name__ == "__main__":
    unittest.main()
