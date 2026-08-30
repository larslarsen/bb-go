# BBGO-NET-001 — Restore Seven-Day IPNS Record Lifetime

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Sr Dev — Grok Build (Grok 4.6 High)

Source baseline: `5340054d357b50c750e874591db601037f28b1f0`

## Objective

The maintained daemon must publish BitBook social-root IPNS records with an explicit
seven-day EOL. It must retain Boxo's five-minute cache TTL so clients still recheck for
updates promptly. The lifetime must not depend on Boxo's current 48-hour default.

## Invariants

- A record produced through `Node.PublishRoot` expires approximately 168 hours after the
  publication begins.
- The published record remains a valid, decodable IPNS EOL record for the author's peer
  identity and points to the requested root CID.
- Its cache TTL remains `ipns.DefaultRecordTTL` (currently five minutes).
- Existing BitBook DHT/Bitswap isolation and root resolution behavior remain unchanged.

## Authorized Paths

Test source first:

- `modern/network/node_test.go`

Production source only after the red result is recorded:

- `modern/network/node.go`

No other file may be modified. Do not edit dependencies, generated files, governance,
CI, protocol IDs, republish cadence, or Git state.

## Required Red Evidence

Add `TestPublishedIPNSRecordLifetime` through the real `Node.PublishRoot` path. Retrieve
and decode the actual stored IPNS record; do not test a helper or a copied constant.

Run from `modern/`:

```sh
GOTOOLCHAIN=go1.27.0 go test ./network -run '^TestPublishedIPNSRecordLifetime$' -count=1
```

Before production changes, this must fail because the observed EOL is approximately 48
hours rather than seven days. Record the failure and do not weaken the assertion.

## Implementation Boundary

Define a named seven-day record-lifetime constant in `network/node.go`. Pass an explicit
`namesys.PublishWithEOL(...)` option from `Node.PublishRoot`. Do not alter the record TTL,
the ten-minute republish loop, Boxo source, DHT provider behavior, or protocol IDs.

Use before/after timestamps or an equivalently robust observation window in the test so
normal execution time cannot make it flaky. A narrow tolerance around an exact wall-clock
instant is not acceptable.

## Required Green and Acceptance Evidence

The following commands are authorized from `modern/`, in this order:

```sh
gofmt -w network/node.go network/node_test.go
GOTOOLCHAIN=go1.27.0 go test ./network -run '^TestPublishedIPNSRecordLifetime$' -count=1
GOTOOLCHAIN=go1.27.0 go test ./network -count=1
GOTOOLCHAIN=go1.27.0 go test -race ./network -count=1
GOTOOLCHAIN=go1.27.0 go vet ./...
GOTOOLCHAIN=go1.27.0 go test ./... -count=1
sha256sum network/node.go network/node_test.go
wc -l network/node.go network/node_test.go
```

## Test Falsification

After the green result, temporarily remove only the explicit EOL option from
`Node.PublishRoot` and rerun:

```sh
GOTOOLCHAIN=go1.27.0 go test ./network -run '^TestPublishedIPNSRecordLifetime$' -count=1
```

It must fail by observing Boxo's roughly 48-hour default. Restore the production line,
run `gofmt`, and rerun the targeted green command. Do not retain the mutation.

## Security and Supply-Chain Applicability

This ticket changes only a signed record's EOL option. It changes no dependency, parser,
cryptographic primitive, build input, release artifact, or secret-bearing path. The
authorized full `go vet` and test suite plus reviewer inspection are the applicable
source checks. SBOM, dependency, artifact, and secret scans are unchanged and therefore
not regenerated for this patch.

## Developer Report

Report:

- red, green, falsification, race, vet, and full-suite results exactly;
- changed paths, final SHA-256 hashes, line counts, and Go test counts where reported;
- the observed red/default and green/configured lifetime windows; and
- confirmation that no Git operation or out-of-scope edit occurred.

Stop after reporting. Only the reviewer accepts, commits, or pushes the change.

## Reviewer Acceptance

Accepted by Lead Engineer/Reviewer — Codex.

- The test failed before implementation and during falsification by observing Boxo's
  approximately 48-hour default, then passed after restoration with a seven-day EOL.
- The test traverses `Node.PublishRoot`, decodes and validates the actual persisted IPNS
  record, proves its root value, and independently asserts the unchanged default TTL.
- The source drop modified only `modern/network/node.go` and
  `modern/network/node_test.go`; the reviewer found no correctness or scope defects.
- Independent reviewer acceptance passed:

  ```sh
  GOTOOLCHAIN=go1.27.0 go vet ./... && GOTOOLCHAIN=go1.27.0 go test -race ./... -count=1
  ```

The first sandboxed reviewer attempt could not open localhost sockets. Re-running the
same suite with localhost socket access passed every maintained package; this was an
execution-environment restriction, not a code failure.
