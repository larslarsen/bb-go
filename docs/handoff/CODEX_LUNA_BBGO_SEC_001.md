# Codex Luna Integration Handoff — BBGO-SEC-001

You are **Jr Dev — Codex Luna**, using `gpt-5.6-luna`. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Read completely before acting:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`
5. `docs/handoff/GROK_BUILD_BBGO_SEC_001.md`, including its delivered source report

Inspect the Grok source drop against the authorized paths. Do not design or author tests
and do not repair source. Stop without Git if the drop is incomplete or out of scope.

For a conforming drop, own the complete red/green/falsification and acceptance sequence
from `BBGO-SEC-001`. Reconstruct red by integrating only the test source before the
checker/workflows, then integrate the production drop for green. Use temporary
directories outside the repository for tool binaries, the Linux daemon, and SBOM. Never
record a secret value.

Do not run `rm -rf`, `rm -r`, `find -delete`, or equivalent recursive deletion. Do not
delete any path expressed through a variable, substitution, glob, symlink, or other
unresolved target. Leave temporary state under `/tmp` for operating-system cleanup; a
GitHub runner is itself ephemeral. Run immutable-pin falsification through the existing
in-memory unittest
`scripts.security_policy_test.CheckerRejectionTest.test_mutable_action_tag_is_rejected`;
do not create and recursively remove a falsification directory.

Local `/tmp` is a RAM-backed tmpfs and must not be used for this task. Reuse the already
installed `actionlint`, `gitleaks`, `gosec`, and `govulncheck` binaries from the exact
disk-backed directory
`/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`. Install only the missing
CycloneDX tool into that same directory. For every remaining local Go install, build,
test, or scanner command, set the exact disk-backed paths
`GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829` and
`GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829`. Write the local daemon
and SBOM only to
`/home/lars/OpenBazaar/.security-artifacts/bb-go-sec-001-20260829`. Do not use `mktemp`,
local `/tmp`, or any unresolved tool/cache/work/artifact path, and do not clean these
directories; leave them in place for owner-reviewed later disposition.

The first Govulncheck attempt was not a vulnerability result: its binary was built with
Go 1.26 and could not load the Go 1.27 maintained module. Before resuming scanners,
rebuild exactly `govulncheck@v1.7.0`, `gosec@v2.29.0`, `gitleaks@v8.30.1`, and
`actionlint@v1.7.12` with `GOTOOLCHAIN=go1.27.0` and the exact disk-backed GOBIN, GOCACHE,
and GOTMPDIR paths above. Do not rebuild the existing Go-1.27 CycloneDX binary. Verify
all five binaries with `go version -m`; each must report Go 1.27.0 before scanner
execution resumes.

All five tool binaries now meet that requirement. The next source Govulncheck attempt
was blocked only by sandbox DNS/network policy before analysis. Rerun the exact source
Govulncheck command with approved external network access to `vuln.go.dev`. If it passes,
continue the normal sequence; the later exact binary-mode Govulncheck command may use the
same network authority. Do not grant network authority to other acceptance commands and
do not provide any scanner with secrets or credentials.

If any scanner reports a finding, stop before Git and write only redacted triage metadata
to `docs/security/BBGO-SEC-001-EVIDENCE.md`; do not suppress, baseline, dismiss, or repair
it. Otherwise record every required command, version, result, SBOM metric/hash, and
falsification result.

Update `docs/handoff/CURRENT_TASK.md` to `AWAITING REVIEW`, link the ticket and evidence,
identify the commit under review, and state that no further implementation is authorized.
Stage only the ticket-authorized source, evidence, and current-task paths. Commit with
message `security: add daemon scanning and SBOM evidence` and push `master`. Report the
exact commit hash and push result, then stop. You do not accept the ticket or begin
remediation outside it.

## Reachable Finding Correction 1 Integration

The earlier stop rules were followed and the reachable DHT finding is recorded. Now
reread the correction section of `tickets/BBGO-SEC-001.md` and the complete delivered
report in `docs/handoff/GROK_BUILD_BBGO_SEC_001_FINDING_01.md`.

Inspect the three-path source drop and verify the recorded hashes before execution. Set
the exact disk-backed GOCACHE and GOTMPDIR already named above for every Go module, test,
build, or scanner command. Run `GOTOOLCHAIN=go1.27.0 go mod tidy` in `modern/` to
regenerate `modern/go.sum`; no other dependency change is authorized.

Reconstruct red without a worktree, temporary repository, deletion, or cleanup: first
record the exact hashes of `modern/network/node.go` and `modern/go.mod`, then use bounded
patch edits to temporarily remove only the new Amino import and diversity-filter option
and restore only the prior private-mode nil-diversity option. Run:

```text
GOTOOLCHAIN=go1.27.0 go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
```

The expected red reason is `constructed BitBook DHT has no routing-table IP-diversity
filter`. Immediately restore the three exact production hunks with bounded patch edits,
verify the original recorded hashes, then run the same targeted command green. Falsify
the high-value test through that same temporary mechanism removal and restoration; the
single red run may serve as both the historical red reconstruction and falsification.
Do not use `/tmp`, `mktemp`, another worktree, copying, recursive deletion, Git restore,
or Git reset.

Then run the maintained Go 1.27 race suite. If targeted or race testing fails, record the
result in evidence and stop before Git; do not repair. If they pass, rerun the exact
network-authorized source Govulncheck command. It is expected to continue reporting
`GO-2024-3218` because the Go advisory lists no fixed version. Record whether the
resolved module is exactly `v0.42.2` and whether it is the only reachable vulnerability,
then stop before every later scanner, SBOM, commit, and push. No exception, suppression,
allowlist, or continued scanning past a finding is authorized yet.

The first bounded red attempt was blocked before the intended assertion only because the
sandbox denied loopback socket binding. Production was restored to the exact delivered
hash. Repeat the same bounded red/restoration/green sequence with approved localhost
socket execution outside that restriction. The maintained race suite may use the same
bounded localhost authority. Do not contact public peers; these tests must remain
offline and in-process. Then continue only to the already authorized source Govulncheck
rerun and expected stop.

## Reviewed Govulncheck Exception 1 Integration

Reread the reviewed-exception section and the complete delivered/correction report in
`docs/handoff/GROK_BUILD_BBGO_SEC_001_EXCEPTION_01.md`. Verify every final recorded hash
before execution. Do not author or repair source.

Run both complete standard-library suites:

```text
python3 -m unittest scripts/security_policy_test.py
python3 -m unittest scripts/govulncheck_policy_test.py
```

Record 42 and 40 passing tests respectively. Falsify the highest-value exception and
safety checks by separately running the additional-error, wrong-DHT-version,
expiry-date, variable-deletion, and mutable-Action rejection tests; each must pass by
proving the mutated input is rejected. Run Actionlint on all three workflows.

Then run the actual source adjudicator with the exact disk-backed tool directory on
`PATH`, GOCACHE, and GOTMPDIR. It may use the previously authorized official advisory
network access. Its concise output must accept only Reviewed Govulncheck Exception 1,
identify exact DHT `v0.42.2`, owner and expiry, and report both non-reachable notes. Any
other result or failure stops integration.

If it passes, continue the previously deferred Gosec, redacted Gitleaks, maintained race
suite, exact disk-backed daemon build, binary adjudicator, CycloneDX generation and
structural validation, and `git diff --check`. Do not use local `/tmp`, `mktemp`, cleanup,
deletion, unresolved targets, or recursive commands. Leave all disk-backed tools,
caches, daemon, and SBOM in place. Stop on any finding or command failure; do not repair,
suppress, or broaden the exception. If every command passes, complete evidence and Git
exactly as originally authorized.
