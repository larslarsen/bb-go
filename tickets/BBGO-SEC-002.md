# BBGO-SEC-002 — Retire Inherited Marketplace QA and Resolve Requests Alerts

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Source actor: Sr Dev — Grok Build (Grok 4.6 High)

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Source baseline: `f62d7879fca86db6080ea2ce4e83f6709f1351eb`

## Objective

Remove the inherited OpenBazaar marketplace/payment QA harness, including its vulnerable
Requests 2.20.0 manifest, without weakening the maintained BitBook daemon or social-only
compatibility tests. Do not dismiss Dependabot alerts; removal of the manifest must let
GitHub resolve them from repository state.

The reviewed evidence and product decision are recorded in
`docs/security/DEPENDABOT_TRIAGE_2026-08-29.md`.

## Invariants

- The complete inherited `qa/` tree is absent from the resulting repository.
- No replacement Python dependency or compatibility shim is added.
- The maintained `modern/` daemon test suite continues to pass under the race detector.
- Root compatibility compilation and the existing social-only boundary tests continue
  to pass.
- No marketplace, wallet, payment, listing, escrow, dispute, or moderation behavior is
  restored or moved elsewhere.
- GitHub alerts are allowed to close from manifest removal; none is dismissed.

## Authorized Paths

Grok Build may author test source first:

- `scripts/legacy_qa_retirement_test.py`

Only after the test source is complete, Grok Build may delete:

- `qa/**`

Grok Build may not edit any other path, run tests, install dependencies, use Git, or
author repository records.

Codex Luna may integrate those source changes and author/update only:

- `docs/security/BBGO-SEC-002-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`

## Required Test Source

Using Python's standard `unittest` and `pathlib` only, add a repository-root-aware test
which fails while `qa/` exists and passes only when that complete tree is absent. It must
report any remaining paths so failure is actionable. Do not shell out, inspect Git
history, encode a success constant, or add a Python dependency.

## Red, Green, and Falsification Evidence

Codex Luna owns all execution:

1. Integrate only `scripts/legacy_qa_retirement_test.py` and run from repository root:
   `python3 -m unittest scripts/legacy_qa_retirement_test.py`. It must fail because
   `qa/` exists and list at least one inherited path.
2. Integrate the `qa/**` deletions and rerun the same command. It must pass.
3. Falsify the test after green by temporarily creating `qa/requirements.txt`, rerun the
   targeted command, and record that it fails while naming the temporary path. Remove
   the temporary tree and rerun green. The temporary content must not enter Git.

## Codex Luna Acceptance Commands

Run in this order from repository root unless a working directory is stated:

```sh
python3 -m unittest scripts/legacy_qa_retirement_test.py
./scripts/go.sh test -vet=off ./... -run '^$' -count=1
./scripts/go.sh test -vet=off ./api ./net/service ./net/retriever ./core ./cmd ./ipfs ./schema -run 'TestSocialOnly|^$' -count=1
GOTOOLCHAIN=go1.27.0 go test -race ./... -count=1
git diff --check
git ls-files qa
```

The race command runs from `modern/`. The final `git ls-files qa` output must be empty.

## Evidence Record

Codex Luna records:

- Grok's source-drop report;
- red, green, and falsification commands, exit codes, and relevant output;
- every acceptance command and result;
- deleted tracked-file count and total deleted byte count from the Git diff;
- final path list and SHA-256 for the new regression test;
- confirmation that no alert was dismissed and no dependency was added; and
- the final commit hash and push result.

Codex Luna then changes `docs/handoff/CURRENT_TASK.md` to `AWAITING REVIEW`, commits and
pushes the authorized implementation paths, and stops. Only the reviewer accepts the
result or authorizes another ticket.

## Stop Conditions

Stop before Git on any unexpected nonzero acceptance result, any modified path outside
the authorization, any evidence that current CI invokes `qa/`, or any maintained behavior
regression. Do not fix or suppress an unrelated finding under this ticket.

## Reviewer Acceptance

Accepted by Lead Engineer/Reviewer — Codex on 2026-08-29.

- Codex Luna recorded valid red, green, and falsification evidence.
- All local compatibility, social-boundary, maintained race, and diff checks passed.
- Remote Go workflow run `33283908394` passed.
- Remote dependency-graph run `33283910919` passed and GitHub subsequently reported zero
  open Dependabot alerts.
- The accepted implementation is source commit `ce3cb4d2` with final evidence metadata
  commit `5289c564490a54f1adc5be1d451277d2576f7090`.
- No alert was dismissed and no replacement dependency or out-of-scope deletion landed.
