# BBGO-PAY-001 Fuzz Hang 01

Execution actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `90f4d612`

Result: **FUZZ TARGET 2 REPEATEDLY HANGS BEFORE OUTPUT — STATIC AUDIT REQUIRED**

After green recovery 01, Luna re-preflighted the exact eight-path recovered worktree and
invoked only the prescribed second native fuzz target:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^$' -fuzz '^FuzzDecodeWireEnvelope$' -fuzztime=3s
```

The foreground exec invocation produced no stdout/stderr, never returned, never yielded a
pollable session, and was not awaiting approval. It was interrupted after approximately
626.1 seconds. This reproduces the first green turn's identical silent stall at the same
target. In both cases the requested fuzz duration was three seconds, no result was
captured, no process remained after recovery, and no command was counted as a completed
fuzz gate.

No source/module state changed. The eight dirty paths and every hash in green recovery 01
remain exact; all temporary falsification hunks are restored. Targets 1 and all earlier
green gates remain completed and must not be rerun.

Codex XHigh read `FuzzDecodeWireEnvelope`, `DecodeSignedObject`, the strict JSON parser,
and canonical base64 handling. Every visible source loop is input-bounded and JSON
container recursion is capped at 32, so the cause is not safely established by inspection
alone. The same target must not be invoked a third time unchanged. Sol may perform only
the static audit in `CODEX_SOL_BBGO_PAY_001_FUZZ_HANG_AUDIT_01.md`; no execution, edit,
security scan, integration, binary, or SBOM work is authorized.
