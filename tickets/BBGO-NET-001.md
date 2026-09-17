# BBGO-NET-001 — public IPFS connectivity and BitBook peer discovery

Status: **ACTIVE — Sol High, test source only.** Reviewer: Codex, High.
Read AGENTS.md, TESTING.md and [CURRENT_TASK](../docs/handoff/CURRENT_TASK.md).
This ticket is the complete assignment; chat supplies no additional authority.
ACC-002 is accepted and closed. No NET-001 executor is active yet.

## Outcome and identity boundary

Owner direction, 2026-09-17: use the public IPFS swarm and its bootstrap infrastructure
without a required BitBook seed fleet, manual Connect action or manual bootstrap
configuration. Retain explicit administrative overrides.

The owner also asked whether BitBook users remain identifiable among ordinary IPFS
peers. Each daemon retains its existing key and authenticated libp2p Peer ID.
Discovery finds candidates; a BitBook protocol exchange confirms participation.
An ordinary IPFS connection or advertisement alone must not become a BitBook peer
in the API. Participation proves neither reputation nor a unique human. There is
no secret shared membership key.

The stable account/controller and revocable device-key design remains unchanged.
ACC-001/002 implement isolated verification/storage; daemon integration and signed
account-to-routing bindings are subsequent work. This ticket reports daemon Peer IDs,
not portable AccountIDs. Do not claim account integration is complete. Naming and
community trust policy are unchanged and do not block this task.

## Baseline and exact paths

Source baseline: `42ebaef71a3562211a299d3c1230a58609d8ef23`. Reviewer-only commits may descend
from it. Verify these SHA-256 values before editing. Unexpected identities require
review; do not overwrite another actor's changes.

| Path | SHA-256 |
| --- | --- |
| modern/go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| modern/go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |
| modern/network/node.go | 5add3a890d232af2ed8f53fcb9bd062660b69608937fdb0ea0fa6e0d86e057d9 |
| modern/network/protocols.go | 502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669 |
| modern/network/node_test.go | 2c791449967c412bc35756400dc832b3ec31557f0d9d868882e5103a4ea4ba74 |
| modern/network/open.go | 96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b |
| modern/network/open_test.go | 89a30121d405941a9c8bd839b816febd449f9d18e98a9319d2be42fcb78b9e99 |
| modern/network/identity.go | b43f6ad90149ce41526aa3b0f85329dbbf847feaaf0442529a204c637833b4fa |
| modern/network/protocols_test.go | 08d065c8c53abc39f9cf9d2c0607fab85eee6f37cf3fa6c5e3da306914abbccf |
| modern/api/handler.go | 70bac95bbde93613e5d1759e5cd826d9d1dbe8b9136acfb6720658ad93a0fc6c |
| modern/api/handler_test.go | 31f9a3a68f0ee8cbdd880bb660ea3b76732c6f801c5a39da29d69fbfb75d75ee |
| modern/cmd/bitbookd/main.go | b7b72438c3e41131a5fb9578bc0e22ca12bef11baeac2f3d6a0cdd9429631a20 |
| modern/cmd/bitbookd/payment_test.go | f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2 |

**Authorized now — test source only:**

- Modify `modern/network/node_test.go`, `modern/network/open_test.go` and
  `modern/api/handler_test.go` for the assertions below; retain existing coverage.
- Add `modern/network/bootstrap_test.go`, `modern/network/discovery_test.go`,
  `modern/network/discovery_fuzz_test.go`, and
  `modern/cmd/bitbookd/bootstrap_test.go`. These paths are absent at baseline.
- In `modern/cmd/bitbookd/payment_test.go`, only add the new no-bootstrap flag to
  the child daemon argv so subprocess tests cannot use public defaults.

Sol may read source and format only these test paths. No test/build/scanner execution,
production stubs, dependency changes, records, Git, commits or pushes. Stop after the
test-source drop; reviewer reads the files directly. Sol does not write an execution
report for unexecuted work. A completion notice is only a pointer.

**Production paths reserved for later authorization in this ticket:**

- `modern/network/node.go`, `modern/network/protocols.go`
- `modern/network/discovery.go` (new)
- `modern/cmd/bitbookd/main.go`, `modern/api/handler.go`

No identity-file migration, dependency update, cross-repository change, real user data
access, daemon restart, or direct/payment/account/social source edits. Preserve all
unrelated dirty/untracked work, especially cancelled DEV-001 and its dirty README.
The future executor may add `modern/NETWORK.md` describing this frozen behavior;
do not stage unrelated `modern/README.md` edits.

## Frozen behavior

### Public IPFS protocols and automatic bootstrap

1. Use public DHT prefix `/ipfs`, current protocol `/ipfs/kad/1.0.0`, empty Bitswap
   prefix, and current Bitswap protocol `/ipfs/bitswap/1.2.0`. Retain Boxo's standard
   supported Bitswap versions. Remove old private DHT/Bitswap registrations. Preserve
   `/bitbook/direct/1.0.0` and `/bitbook/payment/1.0.0` and their authentication.
2. At the daemon boundary, implement
   `selectBootstrapPeers(encoded []string, noBootstrap bool) ([]peer.AddrInfo, error)`.
   No flags selects the pinned DHT's `GetDefaultBootstrapPeerAddrInfos()`.
   Repeated `-bootstrap` selects exactly those parsed peers without defaults.
   New `-no-bootstrap` selects none; combining it with explicit bootstrap peers is an
   error. Reject malformed addresses before opening the data directory/network.
3. `network.New/Open` use exactly `Config.BootstrapPeers`; nil/empty means no seeds.
   Defaults belong to the CLI, so library fixtures remain offline. Copy peer/address
   slices and pass an explicit DHT bootstrap option even when empty.
   `-no-bootstrap` disables automatic seed dialing, not all possible networking.
4. Remove the synchronous fatal bootstrap-connect loop. Use the pinned DHT's existing
   background bootstrap/retry lifecycle, with a 10-second host dial timeout. An
   unreachable seed must neither prevent startup/local API use nor prevent trying
   another seed. Preserve routing-table IP diversity and public-address filters.
   `AllowPrivateAddresses` remains the explicit local-fixture option.
5. Startup logs identify default/override/disabled bootstrap selection without claiming
   connectivity just because construction succeeded. Replace the false raw-CLI
   no-bootstrap warning. Unavailable seeds are recoverable; no new seed manager.
6. Preserve `identity.key`, datastore layout, CIDs, signed IPNS behavior and seven-day
   IPNS lifetime. Existing stored public blocks remain usable.

Checked against local pinned source: go-libp2p-kad-dht v0.42.2 (Amino defaults,
bootstrap and low-peer repair loop), Boxo v0.42.1 Bitswap bsnet, and go-libp2p v0.49.0
routing discovery. No dependency upgrade is needed. Public seed reachability is not
an acceptance prerequisite.

### BitBook discovery and peer identification

Use existing libp2p routing discovery over the DHT, with public rendezvous namespace
`/bitbook/peers/1.0.0` and upstream namespace-to-provider-CID derivation. No new DHT
record or registry. Advertisements are untrusted candidate sources.

Add constants in `protocols.go`:

- `DiscoveryProtocolCurrent = "/bitbook/discovery/1.0.0"`
- `DiscoveryNamespace = "/bitbook/peers/1.0.0"`

The discovery stream uses authenticated libp2p transport. Its entire request and
response are each the eight ASCII bytes `BBGO001\n` (final byte LF). Requester writes
and closes its write half; responder validates, writes and closes its write half.
Require EOF immediately after eight bytes; reject wrong, truncated or trailing input.
Read at most nine bytes to decide validity. This public marker proves protocol
participation, not secret membership. No bespoke signatures or key transmission.
Identity comes from the stream connection's authenticated remote Peer ID, never a
payload claim, advertised address, user agent or display name.

Freeze these public methods and the shared private parser entry point:

```go
func (n *Node) StartDiscovery(ctx context.Context) error
func (n *Node) BitBookPeers() []peer.ID
func readPeerHello(r io.Reader) error // private, used for request and response
```

- New installs the bounded discovery handler. The daemon calls StartDiscovery after
  direct/payment services are ready. Library construction alone does not advertise
  or run periodic discovery. Reject nil/cancelled startup contexts and closed nodes.
  Repeated starts while running are idempotent. Restart after discovery has stopped
  on the same Node is unsupported and returns an error.
- Start a round immediately, then every minute; at most one round in flight, each
  with a 30-second deadline. Advertise initially, retry next round after failure,
  and renew after half the successful upstream TTL. Advertisement and lookup run
  independently within the round so a failed advertisement cannot suppress lookup.
  Use `FindPeers(..., discovery.Limit(32))`.
- Probe at most 32 distinct valid non-self provider candidates per round. Also probe
  at most 32 connected, unconfirmed peers, rotating that selection between rounds
  so a stable first batch of IPFS peers cannot starve the rest. These transport peers
  do not consume the provider allowance. Deduplicate the combined set; use a bounded
  queue and at most four simultaneous outbound probes.
- Each probe's five-second deadline covers dialing, negotiation and hello I/O.
  Stream creation alone is insufficient. Confirm inbound peers only after a valid
  request and successful response write; confirm outbound peers only after a valid
  response. Reset malformed streams without disconnecting infrastructure peers.
- At most 16 inbound hello handlers perform I/O concurrently; reset excess streams
  without spawning queued workers. Keep at most 128 confirmed entries, evicting the
  least recently confirmed when full. Forget confirmation on the last connection's
  disconnect. BitBookPeers returns an owned, sorted snapshot of currently connected
  confirmed peers without duplicates or self.
- Caller/parent cancellation and Node.Close cancel discovery work and reset active
  discovery streams. Close joins owned loops/handlers before releasing host resources,
  including stalled I/O. No unbounded goroutine per advertisement or retry and no
  discovery goroutines surviving Close.

Private helpers may isolate one round/probe and inject clock/routing interfaces for
deterministic bounds tests. No public test-only options. End-to-end discovery must
exercise the actual DHT, stream protocol and production lifecycle.

### API and public/private content

`GET /ob/peers` lists BitBookPeers only, keeping the sorted JSON array of Peer ID
strings. `GET /ob/status/<peerID>` says connected only for that same confirmed set;
retain invalid-ID errors and existing response shapes. Infrastructure peers still
provide routing/content transport. Do not change publication's transport-connectivity
checks to require a BitBook peer.

Keep private direct-message, payment and account-authority datastore records out of
the public blockstore/provider announcements; never copy the datastore wholesale to
blocks. Public profiles/posts and already-published social follow lists retain their
current visibility. Discovery advertises participation publicly. It does not promise
replication, availability, trust or enumeration of every BitBook user.

## Required tests

Controlled loopback peers, temporary synthetic stores and pinned local modules only.
No public seeds, DNS discovery, real accounts/wallets or running daemon. API assertions
call the real HTTP handler. Helpers assemble fixtures, not substitute discovery.
New top-level tests use the prefix `TestNET001`.

1. Replace obsolete isolation assertions with literal public protocol checks and
   preserved application IDs. Construct an independent upstream host + public DHT +
   Boxo Bitswap without network.New or BitBook constants. Prove block transfer both
   ways and signed IPNS resolution across that boundary, with tampered-record rejection.
   Retain existing transfer, IPNS lifetime, identity and diversity coverage.
2. Bootstrap defaults equal the pinned list; overrides exclude defaults; disabled
   is empty; conflicting/invalid flags fail; returned slices are independent. Prove
   actual run() wiring with explicit loopback bootstrap and disabled bootstrap in
   isolated child processes. Inspect default selection without starting a public-default
   daemon. A dead seed plus healthy local seed leaves construction usable and
   eventually connects the healthy seed.
3. Two BitBook nodes receive only an independent public-DHT seed. Production discovery
   must find and confirm them without test dialing between them, injecting candidate
   addresses into peerstores or pre-populating confirmation. Observe the actual startup
   round. Later-round tests may use the same private round helper. Prove retry after
   a controlled initial failure and no overlapping rounds.
4. `TestNET001PeerAPIRequiresHandshake` connects an ordinary IPFS peer and a false
   advertiser of the exact namespace whose hello response is invalid. Neither may
   appear in /ob/peers or receive connected status. Include a real BitBook peer that
   positively appears. Observe the invalid exchange before asserting rejection, so
   the test proves validation ran. Verify disconnect removal and fresh confirmation
   after reconnect. No negative-only tests that pass with discovery disabled.
5. Hello cases: correct bytes, wrong marker, lengths 0/7/8/9, large input, missing EOF,
   stalled read/write, cancellation and concurrent Close. Test the five-second bound
   with controlled time where practical; real streams still prove cancellation.
   `FuzzPeerHello` asserts the exact accepted language and at most nine bytes consumed.
   Never fuzz real network connections.
6. Exercise deduplication, self/empty IDs, snapshot ownership/order, and boundaries
   around 4 outbound / 16 inbound / 32 per candidate source / 128 confirmed entries.
   Controlled seams may prove high-count limits; do not create hundreds of DHTs.
   Prove resources return after cancellation/Close, repeated starts do not duplicate
   work, and an unavailable peer cannot monopolize discovery.
7. Reopen one temporary store: identity and a public block survive. An upstream
   Bitswap peer retrieves the public block but cannot retrieve a synthetic private
   datastore value using its ordinary SHA-256 block CID. Successful public retrieval
   is the non-vacuous control.

## Execution plan — pending source review

Reviewer pins the test drop here, then authorizes Hermes red capture. After accepted
red, reviewer authorizes Sol production in the reserved paths. Reviewer then pins
production and authorizes Hermes acceptance/publication. Keep one evidence document,
`docs/testing/BBGO-NET-001-EXECUTION-01.md`, appending actual phase results. No handoff
document per phase. Missing results remain gaps; do not ask the owner to transcribe them.

Future execution uses installed cached Go 1.27.0 directly. In modern:

```sh
export PATH="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin:$HOME/go/bin:$PATH"
export GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off GOFLAGS=-p=2 GOMAXPROCS=2
```

No tool installation or module update. Inspect filesystem type before artifact
creation. Use disk-backed `modern/dist/net001` for phase-specific immutable raw logs,
temporary test state and fuzz artifacts; set TMPDIR/GOTMPDIR inside it. Fault backups
use non-Go suffixes such as .go.txt. Record commands, cwd, environment, exit codes,
start/end times, tool identities and source hashes. Preserve raw output; strip trailing
whitespace only from Markdown excerpts. Report scanner module/version/binary hashes,
not all dependency checksums from build metadata.

Exact targeted red and green, from modern:

```sh
go test ./network ./api ./cmd/bitbookd -run '^TestNET001' -count=1 -timeout=180s
```

Before production, failure must correspond to absent discovery/selection symbols or
old private protocols/raw transport peer listing. Missing-symbol compilation failure
is limited red evidence, not a behavioral proof. Unrelated fixture/dependency errors
are not accepted red. After production all tests must pass.

Broader acceptance from modern, after targeted green:

```sh
go test ./... -count=1 -timeout=300s
go test -race ./... -count=1 -timeout=600s
go vet ./...
go test ./network -run '^$' -fuzz '^FuzzPeerHello$' -fuzztime=30s -parallel=2
gosec -tests ./network/... ./api/... ./cmd/bitbookd/...
go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
```

Immediately follow the diversity regression with this from repository root:

```sh
python3 scripts/govulncheck_policy.py source
```

Falsification: in a disposable copy of pinned source, suppress the outbound hello
validation error so the false advertiser's invalid response reaches confirmation. Run:

```sh
go test ./api -run '^TestNET001PeerAPIRequiresHandshake$' -count=1 -timeout=90s
```

It must fail because the false advertiser appears. Record the exact one-site mutation
and output, restore pinned source byte-for-byte, then rerun successfully. Reviewer
pins the exact fault site after source review. Hermes cannot invent a substitute
fault or rewrite tests.

Security tools: gosec v2.29.0, govulncheck v1.7.0, Gitleaks v8.30.1.
Installed binary SHA-256 values, respectively:

- `eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2`
- `6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400`
- `444a87409b36e0c330caf3fa61f354dd13e66987ecc9db63d787db761641541a`

Cached Go binary: `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
Unchanged dependency-policy script:
`709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c`.

Unexpected inputs require review. Existing SEC-001's DHT exception applies only within
its version/expiry/mitigation limits; no new suppressions. New/unadjudicated source
findings, reachable vulnerabilities outside policy, secrets, races, panics, leaks or
failed restored tests block acceptance. Retain scanner findings for reviewer
adjudication; never label nonzero exits clean or delete evidence to pass a scan.

After acceptance, the same Hermes phase rebuilds the local executable from modern:

```sh
go build -o bitbookd ./cmd/bitbookd
go version -m bitbookd
```

Record binary hash, path and actual build identity, including any dirty VCS flag.
Do not restart or claim the running process changed. Before feature publication,
scan the exact staged set from repository root:

```sh
../.security-tools/bbgo-sec-tools-20260829/gitleaks git --pre-commit --staged --redact=100 --no-banner .
git diff --cached --check
```

Later reviewer authorization enumerates feature/report publication paths and hashes.
Passing tests alone does not authorize an unreviewed source push.

## Reviewer governance publication

This activation supersedes the queued/bootstrap-only proposal. Reviewer publication
is limited to `tickets/BBGO-NET-001.md` and `docs/handoff/CURRENT_TASK.md`. It includes no
developer source, tests, acceptance execution or unrelated working-tree changes.
