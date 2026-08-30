"""Check committed GitHub workflow policy for BBGO-SEC-001 using stdlib only."""

from __future__ import annotations

import json
import re
from pathlib import Path


class PolicyError(Exception):
    """Raised when a committed workflow or SBOM document violates policy."""


class YAMLParseError(ValueError):
    """Raised when a workflow YAML document cannot be parsed."""


CHECKOUT_ACTION = "actions/checkout"
CHECKOUT_SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
CHECKOUT_TAG = "v7.0.1"
SETUP_GO_ACTION = "actions/setup-go"
SETUP_GO_SHA = "b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"
SETUP_GO_TAG = "v7.0.0"
UPLOAD_ARTIFACT_ACTION = "actions/upload-artifact"
UPLOAD_ARTIFACT_SHA = "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
UPLOAD_ARTIFACT_TAG = "v7.0.1"
GO_VERSION = "1.27.0"
MODERN_MODULE = "github.com/larslarsen/bb-go/modern"

GOVULNCHECK_MODULE = "golang.org/x/vuln/cmd/govulncheck@v1.7.0"
GOSEC_MODULE = "github.com/securego/gosec/v2/cmd/gosec@v2.29.0"
GITLEAKS_MODULE = "github.com/zricethezav/gitleaks/v8@v8.30.1"
CYCLONEDX_MODULE = "github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0"
ACTIONLINT_MODULE = "github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"

SECURITY_TOOLS = (
    GOVULNCHECK_MODULE,
    GOSEC_MODULE,
    GITLEAKS_MODULE,
    ACTIONLINT_MODULE,
)
SBOM_TOOLS = (GOVULNCHECK_MODULE, CYCLONEDX_MODULE)

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

_USES_RE = re.compile(
    r"^\s*(?:-\s+)?uses:\s+(?P<spec>\S+)(?:\s+#\s*(?P<comment>\S+))?\s*$"
)
_GO_INSTALL_RE = re.compile(r"\bgo\s+install\s+(\S+)")
_GO_BUILD_OUTPUT_RE = re.compile(r"\bgo\s+build\b[^\n]*?-o\s+(\S+)")
_SHA_RE = re.compile(r"^[0-9a-f]{40}$")
_SUPPRESSION_PATTERNS = (
    (r"continue-on-error\s*:", "continue-on-error"),
    (r"\|\|\s*true\b", "|| true"),
    (r"\bset\s+\+e\b", "set +e"),
    (r"--exit-code\s*=?\s*0", "--exit-code 0"),
    (r"--no-fail\b", "--no-fail"),
    (r"\bgosec\b[^\n]*-exclude", "gosec exclude"),
    (r"\bgovulncheck\b[^\n]*-ignore", "govulncheck -ignore"),
    (r"\bgitleaks\b[^\n]*--baseline(?:\s|=|$)", "gitleaks --baseline"),
    (r"\.gitleaksignore\b", ".gitleaksignore"),
    (r"--gitleaks-ignore-path", "--gitleaks-ignore-path"),
    (r"#nosec\b", "#nosec"),
    (r"\ballowlist\b", "allowlist"),
)


class Workflow:
    def __init__(self, path, text, data):
        self.path = Path(path)
        self.text = text
        self.data = data


class ActionUse:
    def __init__(self, line, spec, action, ref, comment):
        self.line = line
        self.spec = spec
        self.action = action
        self.ref = ref
        self.comment = comment


class _Parser:
    def __init__(self, text):
        if text.startswith("\ufeff"):
            text = text[1:]
        self.lines = text.splitlines()
        self.i = 0

    def error(self, message):
        raise YAMLParseError("%s at line %s" % (message, self.i + 1))

    def skip_noise(self):
        while self.i < len(self.lines):
            stripped = self.lines[self.i].strip()
            if stripped == "" or stripped.startswith("#") or stripped == "---":
                self.i += 1
                continue
            return

    def peek(self):
        self.skip_noise()
        if self.i >= len(self.lines):
            return None
        return self.lines[self.i]

    def pop(self):
        line = self.peek()
        if line is None:
            self.error("unexpected end of YAML")
        self.i += 1
        return line

    def parse_document(self):
        self.skip_noise()
        if self.peek() is None:
            return {}
        value = self.parse_value(_indent(self.peek()))
        self.skip_noise()
        if self.i < len(self.lines):
            self.error("trailing YAML content")
        return value

    def parse_value(self, indent):
        line = self.peek()
        if line is None:
            return None
        actual = _indent(line)
        if actual < indent:
            return None
        content = _strip_comment(line[actual:])
        if content.startswith("- ") or content == "-":
            return self.parse_sequence(actual)
        return self.parse_mapping(actual)

    def parse_mapping(self, indent, first_content=None):
        mapping = {}
        if first_content is not None:
            self._add_mapping_entry(mapping, first_content, indent)
        while True:
            line = self.peek()
            if line is None:
                return mapping
            actual = _indent(line)
            if actual < indent:
                return mapping
            if actual > indent:
                self.error("unexpected indent")
            content = _strip_comment(line[actual:])
            if content.startswith("- ") or content == "-":
                return mapping
            self.pop()
            self._add_mapping_entry(mapping, content, indent)
        return mapping

    def _add_mapping_entry(self, mapping, content, indent):
        key, value_text = _split_key_value(content)
        if key in mapping:
            self.error("duplicate key %r" % key)
        mapping[key] = self._parse_entry_value(value_text, indent)

    def _parse_entry_value(self, value_text, indent):
        if value_text in (None, ""):
            nxt = self.peek()
            if nxt is not None and _indent(nxt) > indent:
                return self.parse_value(_indent(nxt))
            return None
        if _is_block_scalar(value_text):
            return self.parse_block_scalar(indent, value_text)
        if value_text.startswith("[") or value_text.startswith("{"):
            return _parse_flow(value_text)
        return parse_scalar(value_text)

    def parse_sequence(self, indent):
        sequence = []
        while True:
            line = self.peek()
            if line is None:
                return sequence
            actual = _indent(line)
            if actual != indent:
                return sequence
            content = _strip_comment(line[actual:])
            if not (content.startswith("- ") or content == "-"):
                return sequence
            self.pop()
            item = "" if content == "-" else content[2:]
            if item == "":
                nxt = self.peek()
                if nxt is not None and _indent(nxt) > indent:
                    sequence.append(self.parse_value(_indent(nxt)))
                else:
                    sequence.append(None)
                continue
            if _looks_like_mapping_entry(item):
                sequence.append(self.parse_mapping(indent + 2, first_content=item))
                continue
            if _is_block_scalar(item):
                sequence.append(self.parse_block_scalar(indent, item))
                continue
            if item.startswith("[") or item.startswith("{"):
                sequence.append(_parse_flow(item))
                continue
            sequence.append(parse_scalar(item))
        return sequence

    def parse_block_scalar(self, parent_indent, indicator):
        collected = []
        content_indent = None
        while self.i < len(self.lines):
            line = self.lines[self.i]
            if line.strip() == "":
                collected.append("")
                self.i += 1
                continue
            actual = _indent(line)
            if actual <= parent_indent:
                break
            if content_indent is None:
                content_indent = actual
            if actual < content_indent:
                break
            collected.append(line[content_indent:])
            self.i += 1
        while collected and collected[-1] == "":
            collected.pop()
        if indicator.startswith(">"):
            return " ".join(item for item in collected if item != "")
        return "\n".join(collected)


def parse_yaml(text):
    return _Parser(text).parse_document()


def parse_scalar(text):
    text = text.strip()
    if text == "":
        return None
    if len(text) >= 2 and text[0] == '"' and text[-1] == '"':
        return _decode_double_quoted(text[1:-1])
    if len(text) >= 2 and text[0] == "'" and text[-1] == "'":
        return text[1:-1].replace("''", "'")
    if text in ("true", "True", "TRUE"):
        return True
    if text in ("false", "False", "FALSE"):
        return False
    if text in ("null", "Null", "NULL", "~"):
        return None
    if re.fullmatch(r"-?\d+", text):
        return int(text)
    if re.fullmatch(r"-?\d+\.\d+", text):
        return float(text)
    return text


def load_workflow(path):
    path = Path(path)
    if not path.is_file():
        raise PolicyError("missing workflow %s" % path)
    text = path.read_text(encoding="utf-8")
    try:
        data = parse_yaml(text)
    except YAMLParseError as exc:
        raise PolicyError("unable to parse %s: %s" % (path, exc)) from exc
    if not isinstance(data, dict):
        raise PolicyError("%s is not a mapping" % path)
    return Workflow(path=path, text=text, data=data)


def event_triggers(data):
    if not isinstance(data, dict) or "on" not in data:
        raise PolicyError("workflow is missing on:")
    on = data["on"]
    if isinstance(on, bool):
        raise PolicyError("workflow on: was parsed as a boolean")
    if isinstance(on, str):
        return {on: None}
    if isinstance(on, list):
        result = {}
        for item in on:
            result[str(item)] = None
        return result
    if isinstance(on, dict):
        return on
    raise PolicyError("workflow on: must be a mapping")


def trigger_paths(data, event):
    triggers = event_triggers(data)
    if event not in triggers:
        raise PolicyError("missing %s trigger" % event)
    body = triggers[event]
    if not isinstance(body, dict):
        raise PolicyError("%s trigger is missing a path filter" % event)
    if "paths-ignore" in body:
        raise PolicyError("%s uses paths-ignore; inclusive paths are required" % event)
    paths = body.get("paths")
    if not isinstance(paths, list) or not paths:
        raise PolicyError("%s trigger is missing paths" % event)
    normalized = []
    for item in paths:
        if not isinstance(item, str) or not item:
            raise PolicyError("%s path filter entries must be strings" % event)
        normalized.append(item)
    return normalized


def iter_steps(data):
    jobs = data.get("jobs")
    if not isinstance(jobs, dict) or not jobs:
        raise PolicyError("workflow is missing jobs")
    for job_name, job in jobs.items():
        if not isinstance(job, dict):
            raise PolicyError("job %s is not a mapping" % job_name)
        steps = job.get("steps")
        if not isinstance(steps, list) or not steps:
            raise PolicyError("job %s is missing steps" % job_name)
        for index, step in enumerate(steps):
            if not isinstance(step, dict):
                raise PolicyError("job %s step %s is not a mapping" % (job_name, index))
            yield job_name, job, step


def step_run_text(step):
    run = step.get("run")
    if run is None:
        return ""
    return str(run)


def step_run_lines(step):
    lines = []
    for raw in step_run_text(step).splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        lines.append(line)
    return lines


def iter_action_uses(text):
    uses = []
    for line_no, raw in enumerate(text.splitlines(), 1):
        match = _USES_RE.match(raw)
        if match is None:
            continue
        spec = match.group("spec")
        if "@" in spec:
            action, ref = spec.rsplit("@", 1)
        else:
            action, ref = spec, ""
        uses.append(
            ActionUse(
                line=line_no,
                spec=spec,
                action=action,
                ref=ref,
                comment=match.group("comment") or "",
            )
        )
    return uses


def check_action_pins(text, required_actions, allow_upload_artifact=False):
    uses = iter_action_uses(text)
    if not uses:
        raise PolicyError("workflow references no GitHub Actions")
    pinned = {
        CHECKOUT_ACTION: (CHECKOUT_SHA, CHECKOUT_TAG),
        SETUP_GO_ACTION: (SETUP_GO_SHA, SETUP_GO_TAG),
    }
    if allow_upload_artifact:
        pinned[UPLOAD_ARTIFACT_ACTION] = (UPLOAD_ARTIFACT_SHA, UPLOAD_ARTIFACT_TAG)
    seen = set()
    for use in uses:
        if not _SHA_RE.fullmatch(use.ref):
            raise PolicyError(
                "line %s: %s is not pinned to a 40-character commit SHA (%s)"
                % (use.line, use.action, use.ref)
            )
        if use.action not in pinned:
            raise PolicyError("line %s: unapproved action %s" % (use.line, use.action))
        sha, tag = pinned[use.action]
        if use.ref != sha:
            raise PolicyError(
                "line %s: %s must be pinned to %s (%s), not %s"
                % (use.line, use.action, sha, tag, use.ref)
            )
        if use.comment != tag:
            raise PolicyError(
                "line %s: %s must retain adjacent comment %s"
                % (use.line, use.action, tag)
            )
        seen.add(use.action)
    required = set(required_actions)
    missing = required - seen
    if missing:
        raise PolicyError("missing required actions: %s" % ", ".join(sorted(missing)))
    extra = seen - required
    if extra:
        raise PolicyError("unexpected actions: %s" % ", ".join(sorted(extra)))


def check_read_only_permissions(data, workflow_name):
    permissions = data.get("permissions")
    if permissions != {"contents": "read"}:
        raise PolicyError(
            "%s permissions must be exactly contents: read, not %r"
            % (workflow_name, permissions)
        )
    jobs = data.get("jobs") or {}
    for job_name, job in jobs.items():
        if isinstance(job, dict) and job.get("permissions") is not None:
            raise PolicyError(
                "%s job %s must not override permissions" % (workflow_name, job_name)
            )


def check_concurrency(data, workflow_name):
    concurrency = data.get("concurrency")
    if not isinstance(concurrency, dict):
        raise PolicyError("%s is missing a concurrency group" % workflow_name)
    group = concurrency.get("group")
    if not isinstance(group, str) or not group.strip():
        raise PolicyError("%s concurrency group is empty" % workflow_name)
    if concurrency.get("cancel-in-progress") is not True:
        raise PolicyError("%s must set cancel-in-progress: true" % workflow_name)


def check_go_workflow(text, data=None):
    if data is None:
        data = parse_yaml(text)
    name = "go.yml"
    triggers = event_triggers(data)
    if set(triggers) != {"push", "pull_request"}:
        raise PolicyError("%s must trigger only on push and pull_request" % name)
    for event in ("push", "pull_request"):
        paths = trigger_paths(data, event)
        if paths != GO_PATHS:
            raise PolicyError("%s %s paths must match the ticketed Go filter" % (name, event))
        forbidden = FORBIDDEN_GO_PATHS.intersection(paths)
        if forbidden:
            raise PolicyError("%s %s paths include documentation globs %s" % (name, event, sorted(forbidden)))
    check_read_only_permissions(data, name)
    check_action_pins(
        text,
        required_actions={CHECKOUT_ACTION, SETUP_GO_ACTION},
        allow_upload_artifact=False,
    )
    _check_setup_go(data, name)
    _require_command(data, COMPILE_CMD)
    _require_command(data, SOCIAL_CMD)
    _require_command(data, MODERN_TEST_CMD, working_directory="modern")
    _check_no_product_binary(text, data, name)
    _check_no_suppression(text, data, name)


def check_security_workflow(text, data=None):
    if data is None:
        data = parse_yaml(text)
    name = "security.yml"
    triggers = event_triggers(data)
    if set(triggers) != {"pull_request", "workflow_dispatch"}:
        raise PolicyError("%s must trigger only on pull_request and workflow_dispatch" % name)
    if "push" in triggers:
        raise PolicyError("%s must not use a push trigger" % name)
    paths = trigger_paths(data, "pull_request")
    if paths != SECURITY_PATHS:
        raise PolicyError("%s pull_request paths must match the ticketed security filter" % name)
    dispatch = triggers["workflow_dispatch"]
    if dispatch not in (None, {}):
        raise PolicyError("%s workflow_dispatch must not add extra configuration" % name)
    check_read_only_permissions(data, name)
    check_concurrency(data, name)
    check_action_pins(
        text,
        required_actions={CHECKOUT_ACTION, SETUP_GO_ACTION},
        allow_upload_artifact=False,
    )
    _check_setup_go(data, name)
    _check_checkout_fetch_depth(data, name, 0)
    _check_temp_tool_installs(data, SECURITY_TOOLS, name)
    if "cyclonedx-gomod" in text:
        raise PolicyError("%s must not install or run CycloneDX" % name)
    _require_diversity_immediately_before_source_scan(data)
    _require_command(data, GOVULNCHECK_SOURCE_CMD, working_directory=None)
    _require_command(data, GOSEC_CMD, working_directory="modern")
    _require_gitleaks_immediately_after_baseline_validation(data)
    _require_command(data, GITLEAKS_BASELINE_CMD, working_directory=None)
    _require_command(data, GITLEAKS_CMD, working_directory=None)
    _require_command(data, POLICY_TEST_CMD, working_directory=None)
    _require_command(data, GOVULNCHECK_POLICY_TEST_CMD, working_directory=None)
    _require_command(data, GITLEAKS_BASELINE_TEST_CMD, working_directory=None)
    _require_command(data, ACTIONLINT_CMD, working_directory=None)
    _check_no_raw_govulncheck(data, name)
    _check_gitleaks_redaction(data, text, name)
    _check_no_product_binary(text, data, name)
    _check_no_suppression(text, data, name)


def check_sbom_workflow(text, data=None):
    if data is None:
        data = parse_yaml(text)
    name = "sbom.yml"
    triggers = event_triggers(data)
    if set(triggers) != {"workflow_dispatch"}:
        raise PolicyError("%s must trigger only on workflow_dispatch" % name)
    check_read_only_permissions(data, name)
    check_concurrency(data, name)
    check_action_pins(
        text,
        required_actions={CHECKOUT_ACTION, SETUP_GO_ACTION, UPLOAD_ARTIFACT_ACTION},
        allow_upload_artifact=True,
    )
    _check_setup_go(data, name)
    _check_temp_tool_installs(data, SBOM_TOOLS, name)
    build_step = _find_step_containing(data, "go build")
    sbom_step = _find_step_containing(data, "cyclonedx-gomod app")
    if build_step is None:
        raise PolicyError("%s must build ./cmd/bitbookd" % name)
    if sbom_step is None:
        raise PolicyError("%s must generate a CycloneDX document" % name)
    if build_step[2] is not sbom_step[2]:
        raise PolicyError("%s must build the daemon and SBOM in the same environment" % name)
    env = _effective_env(build_step[1], build_step[2])
    if str(env.get("GOOS")) != "linux":
        raise PolicyError("%s must set GOOS=linux" % name)
    if str(env.get("GOARCH")) != "amd64":
        raise PolicyError("%s must set GOARCH=amd64" % name)
    if str(env.get("CGO_ENABLED")) != "0":
        raise PolicyError("%s must set CGO_ENABLED=0" % name)
    script = step_run_text(build_step[2])
    _check_ephemeral_build(script, name)
    if GOVULNCHECK_BINARY_CMD not in script:
        raise PolicyError(
            "%s must scan the ephemeral binary with %s" % (name, GOVULNCHECK_BINARY_CMD)
        )
    if re.search(r"(?m)^\s*govulncheck\s", script):
        raise PolicyError(
            "%s must invoke Govulncheck only through scripts/govulncheck_policy.py" % name
        )
    build_lines = step_run_lines(build_step[2])
    if GOVULNCHECK_BINARY_CMD not in build_lines:
        raise PolicyError(
            "%s must invoke %s before uploading artifacts" % (name, GOVULNCHECK_BINARY_CMD)
        )
    binary_index = build_lines.index(GOVULNCHECK_BINARY_CMD)
    upload_index = None
    for index, (_job_name, _job, step) in enumerate(iter_steps(data)):
        uses = str(step.get("uses") or "")
        if uses.split("@", 1)[0] == UPLOAD_ARTIFACT_ACTION:
            upload_index = index
            break
    if upload_index is None:
        raise PolicyError("%s must upload the CycloneDX document after scanning" % name)
    build_index = None
    for index, (_job_name, _job, step) in enumerate(iter_steps(data)):
        if step is build_step[2]:
            build_index = index
            break
    if build_index is None or upload_index <= build_index:
        raise PolicyError("%s must adjudicate the binary before artifact upload" % name)
    if not any("go build" in line for line in build_lines[:binary_index]):
        raise PolicyError("%s must build the daemon before Govulncheck adjudication" % name)
    cyclonedx_line = _cyclonedx_line(script)
    for required in (
        "cyclonedx-gomod app",
        "-json",
        "-packages",
        "-licenses",
        "-main cmd/bitbookd",
        "-output",
        ".cdx.json",
    ):
        if required not in script and required not in cyclonedx_line:
            raise PolicyError("%s is missing %s" % (name, required))
    if "validate_cyclonedx_file" not in script:
        raise PolicyError("%s must validate the CycloneDX document" % name)
    _check_cdx_only_upload(data, name)
    _check_no_unresolved_deletion(data, name)
    if "action-gh-release" in text:
        raise PolicyError("%s must not publish a release" % name)
    _check_no_suppression(text, data, name)


def check_repository(root):
    root = Path(root)
    checker = root / "scripts" / "security_policy.py"
    tests = root / "scripts" / "security_policy_test.py"
    adjudicator = root / "scripts" / "govulncheck_policy.py"
    adjudicator_tests = root / "scripts" / "govulncheck_policy_test.py"
    gitleaks_validator = root / "scripts" / "gitleaks_baseline.py"
    gitleaks_validator_tests = root / "scripts" / "gitleaks_baseline_test.py"
    gitleaks_baseline = root / "security" / "gitleaks-baseline.json"
    if not checker.is_file():
        raise PolicyError("missing scripts/security_policy.py")
    if not tests.is_file():
        raise PolicyError("missing scripts/security_policy_test.py")
    if not adjudicator.is_file():
        raise PolicyError("missing scripts/govulncheck_policy.py")
    if not adjudicator_tests.is_file():
        raise PolicyError("missing scripts/govulncheck_policy_test.py")
    if not gitleaks_validator.is_file():
        raise PolicyError("missing scripts/gitleaks_baseline.py")
    if not gitleaks_validator_tests.is_file():
        raise PolicyError("missing scripts/gitleaks_baseline_test.py")
    if not gitleaks_baseline.is_file():
        raise PolicyError("missing security/gitleaks-baseline.json")
    go = load_workflow(root / ".github/workflows/go.yml")
    security = load_workflow(root / ".github/workflows/security.yml")
    sbom = load_workflow(root / ".github/workflows/sbom.yml")
    check_go_workflow(go.text, go.data)
    check_security_workflow(security.text, security.data)
    check_sbom_workflow(sbom.text, sbom.data)


def validate_cyclonedx_document(document):
    if not isinstance(document, dict):
        raise PolicyError("SBOM is not a JSON object")
    if document.get("bomFormat") != "CycloneDX":
        raise PolicyError("SBOM does not declare CycloneDX")
    metadata = document.get("metadata")
    if not isinstance(metadata, dict):
        raise PolicyError("SBOM metadata is missing")
    component = metadata.get("component")
    if not isinstance(component, dict):
        raise PolicyError("SBOM root component is missing")
    identities = []
    for key in ("name", "purl", "bom-ref", "bom_ref", "group"):
        value = component.get(key)
        if value:
            identities.append(str(value))
    if not any(MODERN_MODULE in item for item in identities):
        raise PolicyError("SBOM root component is not %s" % MODERN_MODULE)
    components = document.get("components")
    if not isinstance(components, list) or len(components) == 0:
        raise PolicyError("SBOM components array is empty")
    dependencies = document.get("dependencies")
    if not isinstance(dependencies, list) or len(dependencies) == 0:
        raise PolicyError("SBOM dependencies array is empty")


def validate_cyclonedx_file(path):
    path = Path(path)
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise PolicyError("unable to read SBOM: %s" % exc) from exc
    try:
        document = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise PolicyError("SBOM is not JSON: %s" % exc) from exc
    validate_cyclonedx_document(document)


def _check_setup_go(data, workflow_name):
    found = False
    for _job_name, _job, step in iter_steps(data):
        uses = str(step.get("uses") or "")
        if uses.split("@", 1)[0] != SETUP_GO_ACTION:
            continue
        found = True
        with_ = step.get("with") or {}
        if str(with_.get("go-version")) != GO_VERSION:
            raise PolicyError("%s must pin Go %s" % (workflow_name, GO_VERSION))
        if with_.get("cache") is not False:
            raise PolicyError("%s must disable setup-go caching" % workflow_name)
    if not found:
        raise PolicyError("%s is missing actions/setup-go" % workflow_name)


def _check_checkout_fetch_depth(data, workflow_name, expected):
    found = False
    for _job_name, _job, step in iter_steps(data):
        uses = str(step.get("uses") or "")
        if uses.split("@", 1)[0] != CHECKOUT_ACTION:
            continue
        found = True
        fetch_depth = (step.get("with") or {}).get("fetch-depth")
        if fetch_depth != expected:
            raise PolicyError(
                "%s checkout fetch-depth must be %s" % (workflow_name, expected)
            )
    if not found:
        raise PolicyError("%s is missing actions/checkout" % workflow_name)


def _check_temp_tool_installs(data, required_modules, workflow_name):
    found = set()
    used_temp = False
    for _job_name, job, step in iter_steps(data):
        run = step_run_text(step)
        if "go install" not in run:
            continue
        env = _effective_env(job, step)
        gobin = str(env.get("GOBIN") or "")
        if "runner.temp" in gobin or "RUNNER_TEMP" in gobin or "RUNNER_TEMP" in run:
            used_temp = True
        else:
            raise PolicyError(
                "%s tool installs must use a job-local temporary GOBIN" % workflow_name
            )
        for match in _GO_INSTALL_RE.finditer(run):
            spec = match.group(1)
            if spec.endswith("@latest") or "@" not in spec:
                raise PolicyError("%s tool install %s is not pinned" % (workflow_name, spec))
            found.add(spec)
    if not used_temp:
        raise PolicyError("%s must install tools into a temporary directory" % workflow_name)
    required = set(required_modules)
    if found != required:
        raise PolicyError(
            "%s tool installs must be exactly %s, not %s"
            % (workflow_name, ", ".join(sorted(required)), ", ".join(sorted(found)))
        )


def _require_diversity_immediately_before_source_scan(data):
    steps = list(iter_steps(data))
    diversity_idx = None
    scan_idx = None
    for index, (_job_name, job, step) in enumerate(steps):
        lines = step_run_lines(step)
        if DIVERSITY_TEST_CMD in lines:
            if diversity_idx is not None:
                raise PolicyError("DHT IP-diversity test must appear once")
            if step.get("working-directory") != "modern":
                raise PolicyError(
                    "DHT IP-diversity test must run with working-directory modern"
                )
            diversity_idx = index
        if GOVULNCHECK_SOURCE_CMD in lines:
            if scan_idx is not None:
                raise PolicyError("source Govulncheck adjudicator must appear once")
            if step.get("working-directory") == "modern":
                raise PolicyError(
                    "source Govulncheck adjudicator must run from the repository root"
                )
            defaults = (job.get("defaults") or {}).get("run") or {}
            if defaults.get("working-directory") == "modern":
                raise PolicyError(
                    "source Govulncheck adjudicator must run from the repository root"
                )
            scan_idx = index
    if diversity_idx is None:
        raise PolicyError("missing DHT IP-diversity regression test")
    if scan_idx is None:
        raise PolicyError("missing source Govulncheck adjudicator")
    if scan_idx == diversity_idx:
        lines = step_run_lines(steps[scan_idx][2])
        try:
            diversity_line = lines.index(DIVERSITY_TEST_CMD)
            scan_line = lines.index(GOVULNCHECK_SOURCE_CMD)
        except ValueError as exc:
            raise PolicyError(
                "DHT IP-diversity test must run immediately before source Govulncheck"
            ) from exc
        if scan_line != diversity_line + 1:
            raise PolicyError(
                "DHT IP-diversity test must run immediately before source Govulncheck"
            )
        return
    if scan_idx != diversity_idx + 1:
        raise PolicyError(
            "DHT IP-diversity test must run immediately before source Govulncheck"
        )


def _require_gitleaks_immediately_after_baseline_validation(data):
    steps = list(iter_steps(data))
    validator_idx = None
    scan_idx = None
    for index, (_job_name, job, step) in enumerate(steps):
        lines = step_run_lines(step)
        if GITLEAKS_BASELINE_CMD in lines:
            if validator_idx is not None:
                raise PolicyError("Gitleaks baseline validator must appear once")
            if step.get("working-directory") == "modern":
                raise PolicyError(
                    "Gitleaks baseline validator must run from the repository root"
                )
            defaults = (job.get("defaults") or {}).get("run") or {}
            if defaults.get("working-directory") == "modern":
                raise PolicyError(
                    "Gitleaks baseline validator must run from the repository root"
                )
            validator_idx = index
        if GITLEAKS_CMD in lines:
            if scan_idx is not None:
                raise PolicyError("Gitleaks must appear once")
            if step.get("working-directory") == "modern":
                raise PolicyError("Gitleaks must run from the repository root")
            defaults = (job.get("defaults") or {}).get("run") or {}
            if defaults.get("working-directory") == "modern":
                raise PolicyError("Gitleaks must run from the repository root")
            scan_idx = index
    if validator_idx is None:
        raise PolicyError("missing Gitleaks baseline validator")
    if scan_idx is None:
        raise PolicyError("missing Gitleaks command")
    if scan_idx == validator_idx:
        lines = step_run_lines(steps[scan_idx][2])
        try:
            validator_line = lines.index(GITLEAKS_BASELINE_CMD)
            scan_line = lines.index(GITLEAKS_CMD)
        except ValueError as exc:
            raise PolicyError(
                "Gitleaks baseline validator must run immediately before Gitleaks"
            ) from exc
        if scan_line != validator_line + 1:
            raise PolicyError(
                "Gitleaks baseline validator must run immediately before Gitleaks"
            )
        return
    if scan_idx != validator_idx + 1:
        raise PolicyError(
            "Gitleaks baseline validator must run immediately before Gitleaks"
        )


def _check_no_raw_govulncheck(data, workflow_name):
    for _job_name, _job, step in iter_steps(data):
        for line in step_run_lines(step):
            if line == GOVULNCHECK_SOURCE_CMD or line.startswith(
                "python3 scripts/govulncheck_policy.py "
            ):
                continue
            if line == "govulncheck" or line.startswith("govulncheck "):
                raise PolicyError(
                    "%s must invoke Govulncheck only through scripts/govulncheck_policy.py"
                    % workflow_name
                )


def _require_command(data, command, working_directory=False):
    found = False
    for _job_name, job, step in iter_steps(data):
        if command not in step_run_lines(step):
            continue
        found = True
        wd = step.get("working-directory")
        if working_directory is False:
            continue
        if working_directory is None:
            if wd == "modern":
                raise PolicyError("%s must run from the repository root" % command)
            defaults = (job.get("defaults") or {}).get("run") or {}
            if defaults.get("working-directory") == "modern":
                raise PolicyError("%s must run from the repository root" % command)
            continue
        if wd != working_directory:
            raise PolicyError("%s must run with working-directory %s" % (command, working_directory))
    if not found:
        raise PolicyError("missing command %s" % command)


def _gitleaks_invocations(data):
    lines = []
    for _job_name, _job, step in iter_steps(data):
        for line in step_run_lines(step):
            if line.startswith("gitleaks "):
                lines.append(line)
    return lines


def _check_gitleaks_redaction(data, text, workflow_name):
    lines = _gitleaks_invocations(data)
    if lines != [GITLEAKS_CMD]:
        raise PolicyError("%s must run %s" % (workflow_name, GITLEAKS_CMD))
    if "--redact=100" not in lines[0]:
        raise PolicyError(
            "%s gitleaks must redact secret values with --redact=100" % workflow_name
        )
    if "--baseline-path security/gitleaks-baseline.json" not in lines[0]:
        raise PolicyError(
            "%s gitleaks must use --baseline-path security/gitleaks-baseline.json"
            % workflow_name
        )
    if re.search(r"(?m)^\s*gitleaks\b[^\n]*--report-path", text):
        raise PolicyError("%s must not write a gitleaks report" % workflow_name)
    if re.search(r"(?m)^\s*gitleaks\b[^\n]*\s-r\s", text):
        raise PolicyError("%s must not write a gitleaks report" % workflow_name)


def _check_no_product_binary(text, data, workflow_name):
    for use in iter_action_uses(text):
        if use.action == UPLOAD_ARTIFACT_ACTION:
            raise PolicyError("%s must not upload artifacts" % workflow_name)
    run = "\n".join(step_run_text(step) for _job, _jobdata, step in iter_steps(data))
    if re.search(r"\bgo\s+build\b", run):
        raise PolicyError("%s must not build a product binary" % workflow_name)
    if re.search(r"\bgo\s+install\s+(?:\./)?cmd/bitbookd\b", run):
        raise PolicyError("%s must not install a product binary" % workflow_name)
    if "cyclonedx-gomod" in run:
        raise PolicyError("%s must not generate an SBOM" % workflow_name)
    if "action-gh-release" in text or "upload-artifact" in text:
        raise PolicyError("%s must not upload or publish binaries" % workflow_name)


def _check_no_suppression(text, data, workflow_name):
    for pattern, label in _SUPPRESSION_PATTERNS:
        if re.search(pattern, text, flags=re.IGNORECASE):
            raise PolicyError("%s suppresses findings via %s" % (workflow_name, label))
    for job_name, job, step in iter_steps(data):
        if "continue-on-error" in job or "continue-on-error" in step:
            raise PolicyError(
                "%s job %s converts a finding to non-blocking" % (workflow_name, job_name)
            )


def _check_ephemeral_build(script, workflow_name):
    if "go build" not in script or "./cmd/bitbookd" not in script:
        raise PolicyError("%s must build ./cmd/bitbookd" % workflow_name)
    if "cd modern" not in script and "working-directory: modern" not in script:
        # working-directory is checked at the YAML step; the script may use cd.
        if "modern" not in script:
            raise PolicyError("%s must build from modern/" % workflow_name)
    if "RUNNER_TEMP" not in script:
        raise PolicyError("%s must write the daemon under RUNNER_TEMP" % workflow_name)
    if re.search(r"go\s+build[^\n]*-o\s+\./", script):
        raise PolicyError("%s must not write a binary into the worktree" % workflow_name)
    outputs = _GO_BUILD_OUTPUT_RE.findall(script)
    if not outputs:
        raise PolicyError("%s go build must set -o to a temporary path" % workflow_name)
    for dest in outputs:
        dest = dest.strip("'\"")
        if dest in {"bitbookd", "./bitbookd"} or dest.startswith("./") or dest.startswith("modern/"):
            raise PolicyError("%s must not write a binary into the worktree" % workflow_name)
        if not (
            dest.startswith("${")
            or dest.startswith("$")
            or "RUNNER_TEMP" in dest
            or "runner.temp" in dest
        ):
            raise PolicyError("%s daemon -o path %r is not runner-temp" % (workflow_name, dest))


def _check_cdx_only_upload(data, workflow_name):
    uploads = []
    for _job_name, _job, step in iter_steps(data):
        uses = str(step.get("uses") or "")
        if uses.split("@", 1)[0] == UPLOAD_ARTIFACT_ACTION:
            uploads.append(step)
    if len(uploads) != 1:
        raise PolicyError("%s must upload exactly one artifact" % workflow_name)
    uploaded = uploads[0].get("with") or {}
    path = uploaded.get("path")
    if not isinstance(path, str) or not path.endswith(".cdx.json"):
        raise PolicyError("%s must upload only a .cdx.json document, not %r" % (workflow_name, path))
    if "\n" in path or "*" in path or "?" in path:
        raise PolicyError("%s upload path must be a single CycloneDX JSON file" % workflow_name)
    name = path.replace("\\", "/").rstrip("/").split("/")[-1]
    if name == "bitbookd" or not name.endswith(".cdx.json"):
        raise PolicyError("%s must not upload the daemon binary" % workflow_name)
    retention = uploaded.get("retention-days")
    try:
        retention_days = int(retention)
    except (TypeError, ValueError):
        raise PolicyError("%s upload retention-days must be 7" % workflow_name)
    if retention_days != 7:
        raise PolicyError("%s upload retention-days must be 7" % workflow_name)


_UNRESOLVED_DELETION_TARGET_RE = re.compile(
    r"(?:\$|`|[\*\?\[]|\breadlink(?:f)?\b|\brealpath\b)"
)
_ENV_ASSIGNMENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")


def _check_no_unresolved_deletion(data, workflow_name):
    for _job_name, _job, step in iter_steps(data):
        for line in step_run_lines(step):
            for command in _iter_simple_commands(line):
                if not _is_deletion_command(command):
                    continue
                if _UNRESOLVED_DELETION_TARGET_RE.search(command):
                    raise PolicyError(
                        "%s must not delete a target expressed through an "
                        "environment or shell variable, substitution, glob, "
                        "or symlink-derived value"
                        % workflow_name
                    )


def _iter_simple_commands(line):
    for part in re.split(r"(?:&&|\|\||;)", line):
        command = part.strip()
        if command:
            yield command


def _is_deletion_command(command):
    tokens = command.split()
    index = 0
    while index < len(tokens) and _ENV_ASSIGNMENT_RE.match(tokens[index]):
        index += 1
    if index >= len(tokens):
        return False
    name = tokens[index]
    rest = tokens[index + 1 :]
    if name in {"rm", "rmdir", "unlink", "shred"}:
        return True
    if name == "find":
        if "-delete" in rest:
            return True
        for offset, token in enumerate(rest):
            if token in {"-exec", "-execdir"} and offset + 1 < len(rest):
                if rest[offset + 1] == "rm":
                    return True
    if name == "xargs" and "rm" in rest:
        return True
    return False


def _find_step_containing(data, needle):
    for job_name, job, step in iter_steps(data):
        if needle in step_run_text(step):
            return job_name, job, step
    return None


def _effective_env(job, step):
    env = {}
    for source in (job.get("env") or {}, step.get("env") or {}):
        for key, value in source.items():
            env[str(key)] = value
    return env


def _cyclonedx_line(script):
    for line in script.splitlines():
        if "cyclonedx-gomod" in line:
            return line.strip()
    raise PolicyError("missing cyclonedx-gomod invocation")


def _indent(line):
    count = 0
    for char in line:
        if char == " ":
            count += 1
            continue
        if char == "\t":
            raise YAMLParseError("tabs are not allowed in workflow YAML")
        break
    return count


def _strip_comment(text):
    in_single = False
    in_double = False
    in_expr = False
    index = 0
    length = len(text)
    while index < length:
        char = text[index]
        if in_expr:
            if text.startswith("}}", index):
                in_expr = False
                index += 2
                continue
            index += 1
            continue
        if not in_single and not in_double and text.startswith("${{", index):
            in_expr = True
            index += 3
            continue
        if in_single:
            if char == "'":
                in_single = False
            index += 1
            continue
        if in_double:
            if char == "\\" and index + 1 < length:
                index += 2
                continue
            if char == '"':
                in_double = False
            index += 1
            continue
        if char == "'":
            in_single = True
            index += 1
            continue
        if char == '"':
            in_double = True
            index += 1
            continue
        if char == "#":
            return text[:index].rstrip()
        index += 1
    return text.rstrip()


def _split_key_value(content):
    if ": " in content:
        key, value = content.split(": ", 1)
        return _unquote_key(key), value
    if content.endswith(":"):
        return _unquote_key(content[:-1]), ""
    raise YAMLParseError("expected key: value in %r" % content)


def _unquote_key(key):
    key = key.strip()
    if len(key) >= 2 and key[0] == key[-1] and key[0] in {'"', "'"}:
        return parse_scalar(key)
    return key


def _looks_like_mapping_entry(content):
    content = content.strip()
    if not content or content[0] in "[{|>":
        return False
    return ": " in content or content.endswith(":")


def _is_block_scalar(value):
    return value in ("|", "|-", "|+", ">", ">-", ">+")


def _decode_double_quoted(text):
    out = []
    index = 0
    escapes = {"n": "\n", "t": "\t", "r": "\r", '"': '"', "\\": "\\"}
    while index < len(text):
        char = text[index]
        if char == "\\" and index + 1 < len(text):
            out.append(escapes.get(text[index + 1], text[index + 1]))
            index += 2
            continue
        out.append(char)
        index += 1
    return "".join(out)


def _parse_flow(text):
    text = text.strip()
    if text.startswith("[") and text.endswith("]"):
        inner = text[1:-1].strip()
        if inner == "":
            return []
        return [parse_scalar(part.strip()) for part in _split_flow_items(inner)]
    if text.startswith("{") and text.endswith("}"):
        inner = text[1:-1].strip()
        if inner == "":
            return {}
        mapping = {}
        for part in _split_flow_items(inner):
            key, value = _split_key_value(part.strip())
            mapping[key] = parse_scalar(value) if value != "" else None
        return mapping
    raise YAMLParseError("unsupported flow YAML %r" % text)


def _split_flow_items(text):
    items = []
    current = []
    depth = 0
    in_single = False
    in_double = False
    for char in text:
        if in_single:
            current.append(char)
            if char == "'":
                in_single = False
            continue
        if in_double:
            current.append(char)
            if char == '"':
                in_double = False
            continue
        if char == "'":
            in_single = True
            current.append(char)
            continue
        if char == '"':
            in_double = True
            current.append(char)
            continue
        if char in "[{":
            depth += 1
            current.append(char)
            continue
        if char in "]}":
            depth -= 1
            current.append(char)
            continue
        if char == "," and depth == 0:
            items.append("".join(current).strip())
            current = []
            continue
        current.append(char)
    if current:
        items.append("".join(current).strip())
    return items
