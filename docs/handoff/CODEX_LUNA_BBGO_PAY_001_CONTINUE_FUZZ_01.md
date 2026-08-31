# Codex Luna Handoff — BBGO-PAY-001 Continue Fuzz 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`, and
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_GREEN_01.md`.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the eight dirty paths and all hashes/line counts in green recovery 01.
Require the `modern/go.mod` diff to contain only the recorded direct-dependency move,
`modern/go.sum` to remain clean, `git diff --check` to pass, and read-only `gofmt -d` to
be empty over all seven production paths. Confirm no previous fuzz process remains.
Stop on any difference.

## Sole execution scope

Run exactly these five native fuzz commands serially in the foreground from `modern/`:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzDecodeWireEnvelope$' -fuzztime=3s
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzParseFrame$' -fuzztime=3s
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzJCSDeterminism$' -fuzztime=3s
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzSignatureMutation$' -fuzztime=3s
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzReplayKeyCollision$' -fuzztime=3s
```

Each must exit zero and report a nonzero execution count. Do not rerun fuzz target 1 or
any completed tidy, test, falsification, vet, or race gate. Do not run scanners, security
policy suites, Git mutation, commit, push, binary, or SBOM work in this turn.

After the five commands, perform only read-only hash/status checks. Report each exact
command, exit status, execution count, total interesting inputs, elapsed package result,
and final status/hashes. Do not create or edit evidence; Codex XHigh will record this
bounded continuation before authorizing security. Do not use `/tmp`, root, `sudo`,
deletion, cleanup, background work, public peers, or external services.
