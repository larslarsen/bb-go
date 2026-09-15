# BBGO-PAY-003 acceptance evidence

Date: 2026-09-15. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free),
manually relayed by owner. Reviewer: Codex.

## Control-plane timing limitation

Hermes submitted these results while the earlier acceptance handoff was inactive and
CURRENT_TASK's report gate and DEV-001 cancellation heading were replaced. No reviewer
reactivation of that prior authorization is recorded. This documentation preserves useful
results already produced; it does not retroactively claim that the inactive handoff
authorized further stages.

## Model

meituan/longcat-2.0:free via Nous Portal (provider: nous).

## Baseline

HEAD: `2b695a8719a7381f3919bb83c63e54251f43536d`.
Working tree contains uncommitted accepted source.

## Source verification

All eight frozen pins matched preflight.

| Path | SHA-256 | Lines |
|---|---|---|
| modern/localclient/server.go | 5b00cc6e10694a5fbaccda27637751b866cbc6bb1f0bfd98f05dc94479cc7a93 | 440 |
| modern/localclient/server_test.go | 655c7e18769fdd0f066e29636b11184c64b05ab83d29397b32399b018a6bdf6c | 749 |
| modern/cmd/bitbookd/localclient_test.go | 221407662c049dd27e0a31c58952e194d52c8b13ea1e4015de2b129c7f619799 | 195 |
| modern/cmd/bitbookd/main.go | b7b72438c3e41131a5fb9578bc0e22ca12bef11baeac2f3d6a0cdd9429631a20 | 253 |
| modern/payment/service.go | a523b6b886d9eeae4468a2eaf3609c698dd919981ac2dc653bf23bb4e0d59bea | 687 |
| modern/api/handler.go | 70bac95bbde93613e5d1759e5cd826d9d1dbe8b9136acfb6720658ad93a0fc6c | 653 |
| modern/go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 | 133 |
| modern/go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a | 374 |

## Race suite

Command: `go test -race ./localclient ./cmd/bitbookd ./payment ./api -count=1`
Log: `modern/dist/pay003-tmp/race-suite.log`
Exit: 0. No race, panic, or cleanup error. Nonverbose output; individual test/skip counts
not established from this log.

| Package | Time |
|---|---|
| localclient | 2.019s |
| cmd/bitbookd | 7.695s |
| payment | 2.257s |
| api | 1.193s |

## Fuzz campaign

Command: `go test ./localclient -run '^$' -fuzz '^FuzzLocalClientAuth$' -fuzztime=10s`
Log: `modern/dist/pay003-tmp/fuzz.log`
Exit: 0. Configured fuzztime 10s; final progress elapsed 11s, package duration 11.052s.
77,082 native executions with 12 workers. 81 new interesting inputs found, 87 total
(including 6 baseline seeds).

## Falsification

Mutated `modern/localclient/server.go` line 203: `if !s.authorized(r) {` → `if false && !s.authorized(r) {`.
Backup: `modern/dist/pay003-tmp/server.go.backup` (SHA-256 `5b00cc6e10694a5fbaccda27637751b866cbc6bb1f0bfd98f05dc94479cc7a93`).

Falsified command: `go test ./localclient -run '^TestLocalClientHTTPRejectionsAndSuccessfulRead$/^(wrongToken|wrongInstance)$' -count=1 -v`
Log: `modern/dist/pay003-tmp/falsification.log`
Exit: 1. Both wrongToken and wrongInstance returned 200 instead of 401.

Restored server.go to accepted hash. Post-restore focused command: exit 0 in 0.017s.

## Vet

Command: `go vet ./localclient ./cmd/bitbookd`
Log: `modern/dist/pay003-tmp/vet.log` (empty output, consistent with reported clean exit)

## Security scans

### Gosec v2.29.0 (corrected scan, production files only)

**Command:** `gosec -verbose -include-tests ./localclient/... ./cmd/bitbookd/...`
**Log:** `modern/dist/pay003-tmp/gosec-scan02.log`
**Exit:** 0. Files: 2, Lines: 693, Issues: 0.
**Production files loaded and analyzed:**
- localclient/server.go
- cmd/bitbookd/main.go

**Provenance:** executable `${GOBIN}/gosec`, SHA-256 `eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2`.
Module: `github.com/securego/gosec/v2 v2.29.0`.

The earlier zero-file scan (`modern/dist/pay003-tmp/gosec.log`, Files: 0, Lines: 0) is
invalid and superseded by this corrected scan.

**Review 03 Gosec argv interpretation:** The reported command used `-verbose -include-tests`.
Static inspection of pinned Gosec source shows that `-verbose` takes a string, consuming
`-include-tests` as its format argument; an unknown format falls back to text. It does
not enable test analysis (the actual flag is `-tests`). This invocation still analyzes
the same two production files required by the ticket, as the log confirms. Test files
were not analyzed in this corrected scan.

### Gosec v2.29.0 (full scan including tests)

**Command:** `gosec -verbose -include-tests -tests ./localclient/... ./cmd/bitbookd/...`
**Log:** `modern/dist/pay003-tmp/gosec-scan03.log`
**Exit:** 0. Files: 6, Lines: 2623, Issues: 7.

All 7 findings are in test files; production files (server.go, main.go) are clean:

| File | Finding | CWE | Location |
|---|---|---|---|
| cmd/bitbookd/payment_test.go:284 | G204 Subprocess launched with variable | CWE-78 | daemon child re-exec harness |
| localclient/server_test.go:587 | G304 Potential file inclusion via variable | CWE-22 | test file read |
| localclient/server_test.go:442 | G304 Potential file inclusion via variable | CWE-22 | test file read |
| localclient/server_test.go:374 | G304 Potential file inclusion via variable | CWE-22 | test file read |
| cmd/bitbookd/localclient_test.go:110 | G304 Potential file inclusion via variable | CWE-22 | test file read |
| localclient/server_test.go:389 | G302 Expect file permissions to be 0600 or less | CWE-276 | test chmod |
| localclient/server_test.go:386 | G301 Expect directory permissions to be 0750 or less | CWE-276 | test mkdir |

These are test harness patterns (child re-exec, temp file paths, test data setup).
No source repair is authorized by the handoff.

### Govulncheck v1.7.0 (source mode)

Command: `python3 scripts/govulncheck_policy.py source`
Log: `modern/dist/pay003-tmp/govulncheck.log`
Accepted reviewed exception GO-2024-3218 on github.com/libp2p/go-libp2p-kad-dht@v0.42.2,
4 non-reachable notes. No new findings.

## Staged paths (16)

```
modern/localclient/server.go
modern/localclient/server_test.go
modern/cmd/bitbookd/localclient_test.go
modern/cmd/bitbookd/main.go
tickets/BBGO-PAY-003.md
docs/handoff/CURRENT_TASK.md
docs/handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md
docs/handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md
docs/handoff/HERMES_BBGO_PAY_003_SCANS_02.md
docs/handoff/HERMES_BBGO_PAY_003_PUBLICATION_01.md
docs/testing/BBGO-PAY-003-SOURCE-REVIEW-02.md
docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md
docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md
docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-01.md
docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md
docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-03.md
```

## Staged-content Gitleaks

Command: `gitleaks git --pre-commit --staged --redact=100 --no-banner .`
Log: `modern/dist/pay003-tmp/gitleaks-final.log` (historical, 14 paths)
Exit: 0. No leaks found. The index was subsequently cleared; this does not represent
a scan of the final 16-path set.

## Local binary refresh

Command: `go build -mod=readonly -o bitbookd ./cmd/bitbookd`
Log: `modern/dist/pay003-tmp/build.log` (empty output, consistent with reported success)
Output path: `modern/bitbookd`. Binary: 43,899,109 bytes.
SHA-256: `345feab607f2422c491b15e67fbe5382e0804b6888e7a3de9c420fed8dd007c0`
Toolchain: go1.27.0. vcs.modified=true (uncommitted accepted source).
Daemon not restarted.

## Verdict

SCAN FINDINGS — REVIEW REQUIRED.

The production-only Gosec scan is clean (2 files, 693 lines, 0 issues).
The full scan including tests found 7 issues, all in test files (G204 child re-exec
harness, G304 test file reads, G301/G302 test permissions). No source repair is
authorized. No commit or push until Codex reviews the findings.
