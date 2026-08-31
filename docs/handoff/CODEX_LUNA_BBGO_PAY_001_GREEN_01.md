# Codex Luna Handoff — BBGO-PAY-001 Green, Falsification, and Security 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Integrated test baseline: `9dce8b68ebd02cb9a2030170c80a3efdfe647ba5`

Disk-backed Go cache: `/home/lars/OpenBazaar/.security-cache/go-build-20260829`

Disk-backed Go temp: `/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829`

Pinned tool directory: `/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md`,
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md`,
`tickets/BBGO-SEC-001.md`, and `docs/security/BBGO-SEC-001-EVIDENCE.md`.

Run every command serially in the foreground. Never use local `/tmp`; it is tmpfs. Use
the exact disk-backed cache/temp paths above for every Go command. Do not use root,
`sudo`, deletion, cleanup, `rm`, globs, unresolved targets, public peers, external
application services, wallets, rates, transactions, hardware/devices, release binaries,
or SBOM generation.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the seven dirty production paths and every line count/SHA-256 from
production source review 02. Require `git diff --check` and an empty read-only `gofmt -d`
over all seven. Require the four integrated test paths to match gap expected-red evidence
03. Verify the pinned `govulncheck`, `gosec`, `gitleaks`, and `actionlint` binaries with
`go version -m`; require their reviewed versions and Go 1.27.0. Stop on any mismatch.

## Module resolution and focused green

From `modern/`, run once:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go mod tidy
```

Only `modern/go.mod` and `modern/go.sum` may change. `golang.org/x/text v0.40.0` is
expected to move from indirect to direct because accepted production imports
`unicode/norm`; no version or unrelated dependency change is expected. Inspect and record
the exact module diff. Stop on any broader change.

Then run these commands from `modern/`, in order. Request a sandbox override only when
needed for ephemeral `127.0.0.1` sockets; explain that this is offline in-process libp2p
testing, not root or public-network access.

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -count=1
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./network -run '^TestPaymentProtocolCurrent$' -count=1
```

All must exit zero. Stop without repair on any failure.

## Required reversible falsifications

Record the current SHA-256 of each file before its falsification. Use only bounded
`apply_patch` edits; never use Git restore/reset, copying, a second worktree, a temporary
file, or deletion. Run the named test once expecting failure for the named reason,
immediately restore the exact hunk with `apply_patch`, reproduce the original hash, and
rerun the same test green. Stop if failure is for another reason or restoration differs.

1. **Domain separation:** in `modern/payment/types.go`, temporarily change only the
   `DomainSeparatorRequest` literal by appending `-falsified`. Run
   `go test ./payment -run '^TestGoldenValidPaymentObjectsMatchCanonicalAndDigest$'
   -count=1`. It must fail on the independent fixture's request domain/digest. Restore and
   require green.
2. **Remote-peer binding:** in `modern/payment/signature.go`, temporarily change only
   `if signer != remote {` to `if false && signer != remote {`. Run
   `go test ./payment -run '^(TestSignatureRejectsWrongRemotePeer|TestStatusSignatureRejectsWrongRemotePeer)$'
   -count=1`. Both must fail because wrong remote identities are no longer classified
   `REMOTE`. Restore and require green.
3. **Durable replay classification:** in `modern/payment/replay.go`, temporarily change
   only the request-key condition to
   `if false && (first.RequestID == second.RequestID || first.Nonce == second.Nonce) {`.
   Run `go test ./payment -run '^TestConflictingRequestIDAndNonceAreRejected$' -count=1`
   with the loopback sandbox override. It must fail because request conflicts no longer
   produce `REPLAY` (the storage validator may still fail closed as `STORAGE`). Restore,
   reproduce the original hash, and require green.

Use the exact Go/toolchain/cache/temp environment from focused green for every
falsification command.

## Broad, race, and fuzz gates

After all restorations and green reruns, reproduce all seven accepted production hashes
except for no permitted source difference (module files are separate), then run from
`modern/` with the exact Go/toolchain/cache/temp environment:

```text
go vet ./...
go test -race ./... -count=1
go test ./payment -run '^$' -fuzz '^FuzzDecodeSignedObject$' -fuzztime=3s
go test ./payment -run '^$' -fuzz '^FuzzDecodeWireEnvelope$' -fuzztime=3s
go test ./payment -run '^$' -fuzz '^FuzzParseFrame$' -fuzztime=3s
go test ./payment -run '^$' -fuzz '^FuzzJCSDeterminism$' -fuzztime=3s
go test ./payment -run '^$' -fuzz '^FuzzSignatureMutation$' -fuzztime=3s
go test ./payment -run '^$' -fuzz '^FuzzReplayKeyCollision$' -fuzztime=3s
```

The displayed commands inherit `GOTOOLCHAIN=go1.27.0`, the exact disk-backed `GOCACHE`,
and exact disk-backed `GOTMPDIR`; record the fully expanded commands in evidence. The
race suite may need the loopback-only sandbox override. Each fuzz command must report a
nonzero execution count and exit zero. Do not run multiple fuzz targets together.

## Pinned source-security ratchet

Run from the repository root unless noted, using the absolute pinned binaries. Never
record a secret or unredacted finding value. Any new/unreviewed finding stops before Git;
write only redacted triage metadata. The existing reviewed Govulncheck exception
`GO-2024-3218` on DHT v0.42.2 and exact 25-entry redacted Gitleaks baseline remain valid
only through 2026-11-29.

1. Run the three complete policy suites:

```text
python3 -m unittest scripts/security_policy_test.py
python3 -m unittest scripts/govulncheck_policy_test.py
python3 -m unittest scripts/gitleaks_baseline_test.py
```

2. Run pinned Actionlint:

```text
/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/actionlint .github/workflows/go.yml .github/workflows/security.yml .github/workflows/sbom.yml
```

3. Run source Govulncheck through policy with `PATH` beginning with the exact pinned tool
   directory and the exact Go cache/temp environment. Request network authority only for
   the official vulnerability database if the sandbox blocks it. The policy must exit
   zero with only the reviewed exception and non-reachable notes:

```text
PATH=/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829:/usr/bin:/bin GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 python3 scripts/govulncheck_policy.py source
```

4. From `modern/`, run pinned Gosec and require zero issues:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 /home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gosec ./...
```

5. From the repository root, validate the baseline immediately before the pinned history
   scan, then require no finding outside the reviewed baseline:

```text
python3 scripts/gitleaks_baseline.py
/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gitleaks git --redact=100 --no-banner --baseline-path security/gitleaks-baseline.json .
```

6. Run `git diff --check` and inspect final status. Do not build a daemon, invoke binary
   Govulncheck, generate an SBOM, dispatch GitHub Actions, or build cross-platform/release
   binaries. Those remain manual release gates.

## Evidence and integration

If and only if every green, falsification, broad, race, fuzz, and source-security gate is
acceptable:

1. Create `docs/testing/BBGO-PAY-001-GREEN-01.md` containing baselines; final source and
   module hashes/line counts; exact module diff; every command and exit code; ordinary
   test counts; all three falsification failure/restoration/green results; six fuzz
   execution counts; scanner versions/results; reviewed exception/baseline expiries; and
   the explicit no-binary/no-SBOM decision.
2. Update `docs/handoff/CURRENT_TASK.md` to `GREEN 01 CAPTURED — XHIGH ACCEPTANCE
   REQUIRED`, link evidence and the commit under review, and authorize no further work.
3. Stage with explicit literal paths only: the seven production paths,
   `modern/go.mod`, `modern/go.sum` only if changed by tidy, the new evidence, and
   `docs/handoff/CURRENT_TASK.md`. Review the staged name list and stop on any other path.
4. Commit once as `feat(payment): add signed payment transport`, push `master`, and
   report the SHA, push result, final status, every committed path, all command outcomes,
   and any retained disk-backed cache state. Do not claim engineering acceptance; Codex
   XHigh owns it.

Do not edit tests, fixtures, tickets, other source, workflows, policies, baselines, or
unrelated paths. Do not repair or suppress an unexpected result under this handoff.
