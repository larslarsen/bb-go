# BBGO-SEC-001 Integration Evidence

Status: BLOCKED — policy-test failure; source returned for correction.

Ticket: [BBGO-SEC-001](../../tickets/BBGO-SEC-001.md)
Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)
Source actor: Sr Dev — Grok Build (Grok 4.6 High)
Source baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

## Source-drop verification

The five Grok-authorized source paths were present and matched the durable delivered
report's SHA-256 hashes and line counts. No unauthorized source path was changed.

## Test-first evidence

### Red

Command:

```text
python3 -m unittest scripts/security_policy_test.py
```

Executed with `scripts/security_policy.py`, `.github/workflows/security.yml`, and
`.github/workflows/sbom.yml` temporarily withheld outside the repository. Result:
expected failure, exit code 1. The required-source test failed because those three
required files were absent; the remaining test classes consequently reported missing
checker errors. The files were restored unchanged immediately afterward.

### Green attempt

Command:

```text
python3 -m unittest scripts/security_policy_test.py
```

Result: failure, exit code 1. `Ran 32 tests`; 31 passed and one errored:
`RequiredCommandsTest.test_committed_workflows_satisfy_the_checker`.

Redacted failure metadata:

```text
bb_go_security_policy.PolicyError:
sbom.yml must build the daemon and SBOM in the same environment
```

This is a blocking policy-test finding. No source was repaired, suppressed, baselined,
downgraded, or otherwise altered.

## Stop condition

Per BBGO-SEC-001, integration stopped before Actionlint, scanners, Gitleaks, maintained
race tests, SBOM build/generation/validation, `git diff --check`, evidence completion,
commit, or push. No secret value was recorded. No binary or SBOM was generated.

Reviewer disposition is required before any correction or further acceptance work.
