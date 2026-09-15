# BBGO-PAY-003 Grok correction 01 — completion report

(Author note: machine-specific absolute paths were normalized to a declared
`${BBGO_PAY003_MODERN}` placeholder by Hermes during publication preparation.
This identifies the edit as publication normalization, not a change to Grok's
historical argv/environment meaning. All reported outcomes, hashes and evidence
limits are preserved.)

Actor: Grok Build 4.6 High (xAI), owner-relayed. Reviewer: Codex.
Contract: [GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md](../handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md)
(historical source correction plus the active report-only addendum).
This file is Grok's repository completion record for that correction. It is not
Hermes acceptance evidence. No commands were re-run to produce this document.

## Actor, baseline, and changed paths

- Actor/model: Grok Build 4.6 High.
- Task baseline HEAD: `2b695a8719a7381f3919bb83c63e54251f43536d` (verified in the
  correction session before edits).
- Working directory for Go commands: `${BBGO_PAY003_MODERN}` (the repository's
  `modern` module directory).
- Filesystem at correction time: `stat -f -c '%T'` on `${BBGO_PAY003_MODERN}` and
  `${BBGO_PAY003_MODERN}/dist/pay003-tmp` both reported `ext2/ext3`. TMPDIR was
  `${BBGO_PAY003_MODERN}/dist/pay003-tmp`. No `/tmp` override was used.
- Environment on every `go test` below: `GOTOOLCHAIN=go1.27.0` `GOWORK=off`
  `TMPDIR=<${BBGO_PAY003_MODERN}/dist/pay003-tmp>`.
  Exact toolchain binary path, `go version` output, and toolchain SHA-256 for
  this correction pass: **unavailable** (not captured).
- Authorized correction paths actually edited:
  - `${BBGO_PAY003_MODERN}/localclient/server.go`
  - `${BBGO_PAY003_MODERN}/localclient/server_test.go`
  - `${BBGO_PAY003_MODERN}/cmd/bitbookd/localclient_test.go` (platform skip
    only, plus the three-line `runtime` import needed for that skip)
- Frozen: `${BBGO_PAY003_MODERN}/cmd/bitbookd/main.go` was not edited in the
  correction pass.

No repository log files were written for these Grok commands. Under
`${BBGO_PAY003_MODERN}/dist/pay003-tmp/` the retained `*.log` files are later
Hermes acceptance captures (`race-suite.log`, `fuzz.log`, `falsification.log`,
`restored.log`, `vet.log`, `gosec.log`, `govulncheck.log`, `gitleaks-staged.log`,
`build.log`). They are not Grok correction logs and are not used as this
actor's results.

## Final source identities versus source review 02

After the focused green and fuzz-seed commands succeeded, the correction session
hashed the four paths with a Python SHA-256/line count. Those values match
[source review 02](BBGO-PAY-003-SOURCE-REVIEW-02.md) exactly. Difference from
review 02: **none**. This report-only pass did not re-hash the working tree.

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| modern/localclient/server.go | 440 | `5b00cc6e10694a5fbaccda27637751b866cbc6bb1f0bfd98f05dc94479cc7a93` |
| modern/localclient/server_test.go | 749 | `655c7e18769fdd0f066e29636b11184c64b05ab83d29397b32399b018a6bdf6c` |
| modern/cmd/bitbookd/localclient_test.go | 195 | `221407662c049dd27e0a31c58952e194d52c8b13ea1e4015de2b129c7f619799` |
| modern/cmd/bitbookd/main.go (unchanged) | 253 | `b7b72438c3e41131a5fb9578bc0e22ca12bef11baeac2f3d6a0cdd9429631a20` |

## Pre-fix red (two regressions, production still broken)

Sequence: the two focused regressions were added first
(`TestLocalClientRejectsStreamingUnknownLengthBody` and subtest
`leaveReplacedDescriptor`). Production `hasRequestBody` / Close cleanup were
not yet changed.

Command (from `${BBGO_PAY003_MODERN}/`):

```
gofmt -w localclient/server_test.go
env GOTOOLCHAIN=go1.27.0 GOWORK=off TMPDIR="<${BBGO_PAY003_MODERN}/dist/pay003-tmp>" go test ./localclient ./cmd/bitbookd -run 'TestLocalClient' -count=1 -v
```

The authorized argv is the `go test` line; `gofmt -w` was also executed in the
same shell. The red run used `-v`; the handoff's canonical command does not.

- Combined shell exit: **1**
- `FAIL github.com/larslarsen/bb-go/modern/localclient` 0.803s
- `ok github.com/larslarsen/bb-go/modern/cmd/bitbookd` 0.070s
- Retained log path: **unavailable** (stdout only in the actor session)

Observed intended failures:

1. `TestLocalClientRejectsStreamingUnknownLengthBody` — FAIL.
   Diagnostic: `status 200, want 400` with a successful records JSON body
   (authenticated chunked/unknown-length GET reached `List`). Exact response
   bytes are not restated here; they included peer/instance fields from that
   fixture run.
2. `TestLocalClientPublishesPrivateDescriptorAndCleansUp/leaveReplacedDescriptor`
   — FAIL. Diagnostic: replacement `connection.json` was gone
   (`no such file or directory`).

Other TestLocalClient methods/subtests in that verbose run: PASS, including
`TestLocalClientHTTPRejectionsAndSuccessfulRead` (bodyless nonempty read
control), size-boundary subtests, unchanged-descriptor cleanup (`happyPath`),
and `TestLocalClientDaemonReadsPersistedPaymentRecord`.

Skip counts, fuzz, race, vet, and scanners: **not run** in this step.

## Intermediate compile stop (after production edits, before green)

Production framing/cleanup and remaining test corrections were then applied.
The first combined green command failed to compile `localclient` because
`readDescriptor`'s helper interface lacked `Fatalf`.

Command (from `${BBGO_PAY003_MODERN}/`):

```
gofmt -w localclient/server.go localclient/server_test.go cmd/bitbookd/localclient_test.go
env GOTOOLCHAIN=go1.27.0 GOWORK=off TMPDIR="<${BBGO_PAY003_MODERN}/dist/pay003-tmp>" go test ./localclient ./cmd/bitbookd -run 'TestLocalClient' -count=1
env GOTOOLCHAIN=go1.27.0 GOWORK=off TMPDIR="<${BBGO_PAY003_MODERN}/dist/pay003-tmp>" go test ./localclient -run '^FuzzLocalClientAuth$' -count=1
```

The two `go test` invocations were chained with `&&`. Because the first failed,
the fuzz-seed command **did not run**.

- Combined shell exit: **1**
- `localclient` build failed: `t.Fatalf undefined (type interface{Fatal(args ...any); Helper()} has no field or method Fatalf)` at `localclient/server_test.go:594`
- `ok github.com/larslarsen/bb-go/modern/cmd/bitbookd` 0.113s
- Fuzz-seed outcome for this attempt: **unavailable / not executed**

## Focused green

After adding `Fatalf` to the helper interface:

Command (from `${BBGO_PAY003_MODERN}/`):

```
gofmt -w localclient/server_test.go
env GOTOOLCHAIN=go1.27.0 GOWORK=off TMPDIR="<${BBGO_PAY003_MODERN}/dist/pay003-tmp>" go test ./localclient ./cmd/bitbookd -run 'TestLocalClient' -count=1
```

- Exit: **0**
- `ok github.com/larslarsen/bb-go/modern/localclient` 0.812s
- `ok github.com/larslarsen/bb-go/modern/cmd/bitbookd` 0.070s
- Individual test names, subtest names, skip counts, and failure/error counts
  beyond package `ok`: **unavailable** (this run was not verbose).
  Source inventory in review 02 (not this run): eight localclient
  `TestLocalClient` functions (including the off-Linux `ErrUnavailable` test),
  one daemon test.
- Retained log path: **unavailable**

## Fuzz-seed corpus

Command (from `${BBGO_PAY003_MODERN}/`, same environment, immediately after
the green command in the same successful shell):

```
env GOTOOLCHAIN=go1.27.0 GOWORK=off TMPDIR="<${BBGO_PAY003_MODERN}/dist/pay003-tmp>" go test ./localclient -run '^FuzzLocalClientAuth$' -count=1
```

- Exit: **0**
- `ok github.com/larslarsen/bb-go/modern/localclient` 0.012s
- Per-seed names, seed count observed at runtime, and reader-call outcomes:
  **unavailable** (non-verbose). Source review 02 inspects six `f.Add` seeds;
  that is a source count, not this run's output.
- This was seed execution via `-run`, not `-fuzz` / `-fuzztime`. Native fuzz
  for 10s was **not run** by Grok.
- Retained log path: **unavailable**

## Correction content (for traceability, not a new run)

- Reject parsed unknown/chunked bodies via `ContentLength != 0` or nonempty
  `Request.TransferEncoding`; do not read the body.
- On Close, `Lstat` the descriptor, require a regular non-symlink file, and
  `os.SameFile` against the identity recorded at publish; leave replacements.
- Fuzz: one Linux fixture per process, valid Host/RemoteAddr, `VALID` marker
  instead of generated tokens in seeds.
- Linux guards on listener tests and fuzz; off-Linux `ErrUnavailable` test
  included within the eight localclient test functions.
- Error bodies captured once; closed `{"error":code}` plus EOF.

Stop. Codex reviews this file before any further PAY-003 advancement.
