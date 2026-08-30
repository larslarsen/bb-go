# BBGO-SEC-002 Evidence

## Source drop

Grok Build's delivered report is preserved in
[`GROK_BUILD_BBGO_SEC_002.md`](../handoff/GROK_BUILD_BBGO_SEC_002.md). The conforming
drop added `scripts/legacy_qa_retirement_test.py` and deleted the complete inherited
`qa/` tree: 90 tracked files, five directories, and 552,042 deleted content bytes.
The new test is 26 lines and 862 bytes with SHA-256
`4ac04254c4b64f542e9ccfb8f736029b5afa2e6339363ab8a7061520a57be183`.

The test is non-vacuous because it resolves the repository root from its own location,
checks that the real `qa/` directory (or symlink) is absent, recursively enumerates any
remaining descendants, and reports their repository-relative paths. It does not shell
out, inspect Git history, use dependencies, or encode success as a constant.

## Red, green, and falsification

- Test-only red: `python3 -m unittest scripts/legacy_qa_retirement_test.py` — exit 1;
  `qa/` existed and the failure enumerated inherited paths including
  `qa/requirements.txt`.
- Green after integrating the `qa/**` deletion:
  `python3 -m unittest scripts/legacy_qa_retirement_test.py` — exit 0; 1 test passed.
- Falsification: temporarily created `qa/requirements.txt`, then ran the same command —
  exit 1; failure named `qa/requirements.txt` (and `qa`). The temporary file and tree
  were removed.
- Green after falsification cleanup: same command — exit 0; 1 test passed.

## Acceptance commands

All commands were run in the prescribed order from the repository root unless noted.

1. `python3 -m unittest scripts/legacy_qa_retirement_test.py` — exit 0; 1 test passed.
2. `./scripts/go.sh test -vet=off ./... -run '^$' -count=1` — exit 0; all packages
   compiled; no tests run by design.
3. `./scripts/go.sh test -vet=off ./api ./net/service ./net/retriever ./core ./cmd ./ipfs ./schema -run 'TestSocialOnly|^$' -count=1` — exit 0; API and service tests passed and remaining targeted packages compiled.
4. From `modern/`, `GOTOOLCHAIN=go1.27.0 go test -race ./... -count=1` — initial
   sandbox attempt was blocked by denied localhost sockets; the exact command was then
   rerun with approved socket access and exited 0. `modern/api`, `direct`, `network`,
   and `social` passed; `cmd/bitbookd` had no test files.
5. `git diff --check` — exit 0.
6. `git ls-files qa` — empty output.

## Security and scope

The four Dependabot alerts were not dismissed. Removing `qa/requirements.txt` adds no
replacement dependency and removes the vulnerable Requests 2.20.0 manifest. No
marketplace, wallet, payment, listing, escrow, dispute, or moderation behavior was
restored or moved. No maintained source or test behavior was changed. No temporary
falsification content remains in the worktree.

Deleted tracked-file count: 90. Deleted content size: 552,042 bytes. Git diff summary:
90 files changed, 12,462 deletions.

Commit under review: `ce3cb4d2` (to be amended only to record this final hash).
