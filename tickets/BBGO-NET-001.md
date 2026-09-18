# BBGO-NET-001 — public IPFS connectivity and BitBook peer discovery

Status: **ACTIVE — Sol High, discovery correction under review 05.**
Reviewer: Codex, High. Initial production drop needs the bounded corrections below.
Read AGENTS.md, TESTING.md and [CURRENT_TASK](../docs/handoff/CURRENT_TASK.md).
This ticket is the complete assignment; chat supplies no additional authority.
ACC-002 is accepted and closed. Hermes's red phase is closed; no execution is active.

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

**Initial test-source scope — narrowed by review 01 below:**

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

## Execution plan — red reviewed; further execution awaits production review

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

Falsification (corrected by review 01): in a disposable copy of pinned source,
suppress outbound hello validation so an invalid response reaches confirmation. Run
the completed-probe regression from modern:

```sh
go test ./network -run '^TestNET001OutboundHelloRequiresValidation$' -count=1 -timeout=90s
```

It must fail because the invalid responder is accepted. Record the exact one-site mutation
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

## Review 01 — initial test-source drop; correction required

Reviewer: Codex, 2026-09-17, at HEAD
18eb873bb47666fe59b91a2c883d9b4c391527bf. Source inspection only: no tests, builds,
fuzzing or scanners executed. Production/module pins match the original baseline;
discovery.go remains absent. Eighteen TestNET001 declarations (including the guarded
daemon-child entry) and one fuzz target are present across the eight scoped files.
Existing tests are retained apart from replacing the obsolete isolation assertion.
The payment fixture change is exactly the authorized no-bootstrap argument.

The independent upstream Bitswap/IPNS fixture, literal protocol assertions, parser
fuzzer and persisted-block/public-private control are useful and retained.
The drop is **not accepted for Hermes red execution** because several tests can pass
without the mechanism they claim to prove or fail on a valid asynchronous startup.

### Reviewed drop identities

These replace the original pre-edit test hashes for the next source turn; original
production/module pins remain mandatory. Line numbers below refer to this drop.

| Test path | Lines | SHA-256 |
| --- | --- | --- |
| modern/network/node_test.go | 548 | 9abc12fc479252c390f78802c9df546e6c4a4e78d71b09acf9db20ba815ab000 |
| modern/network/open_test.go | 92 | 29feea3dfb533bd28e2b1c46a37bf33515787b35867fdc732d72d578cad2ef3d |
| modern/network/bootstrap_test.go | 83 | 43d17cb74bcaa8230bfae6d295f65b285bf7a243c0bade97f9a6c9eba6cb664c |
| modern/network/discovery_test.go | 702 | bd8d88a3a0abac4d5a76106de99b10eaac819a73d33dd853e6ae3426d3fddab7 |
| modern/network/discovery_fuzz_test.go | 46 | c4f15be93af81e4d732fedafc80275aadf16b3bdc401bdcc0d3ca0acfcde8b0e |
| modern/api/handler_test.go | 594 | 7e6e65dcfb6d14685c247d130983adf37f41809568c769a0f5a272af393a198a |
| modern/cmd/bitbookd/bootstrap_test.go | 252 | 1fc439e5a58d897a420d70e6c1ea563f01738dfdbc2769397544e55e6fac1e84 |
| modern/cmd/bitbookd/payment_test.go | 734 | f1ac520574b48f22e41b8700d7852214479ccf3cf4b919ffd8be0deaf06eea31 |

### Required corrections

1. **Prove handler admission/rejection, not timeout.** In discovery_test.go:319-340,
   opening 16 streams without I/O does not establish 16 running hello handlers.
   Pinned BasicHost.NewStream uses lazy negotiation for known protocols. The 17th
   read accepts any error, including its own five-second timeout; an unlimited or
   queued implementation can pass. Force negotiation with partial hello writes and
   observe actual admitted-handler occupancy. Verify prompt remote reset for excess
   admission, explicitly rejecting local deadline errors, then release a slot and
   prove a valid exchange succeeds. Cover below/at/above the 16 limit. Likewise
   establish that the stalled stream at lines 277-296 entered production I/O before
   Close, and observe handler/loop completion rather than host disconnection alone.
   Keep a small private admission/lifecycle seam if needed; it must be called by the
   real production handler, not a copied test implementation.

2. **Remove startup and response-completion races.** Lifecycle lines 169-178 start
   discovery before background bootstrap is observed, with a 20-second test deadline
   but a one-minute retry interval. A correct first round can find no routing peers
   or advertisements and the test then cannot recover. Wait for seed routing, start
   one advertiser, observe its provider record through the independent seed, then
   start the discovering node. Do not directly dial the two BitBook nodes.
   The retry case at lines 382-395 needs the same readiness plus a completed first
   attempt before changing the handler or invoking another round.
   Both discovery_test.go:366 and handler_test.go:317 signal before the invalid
   response is even written; an empty peer snapshot at that point is not validation
   evidence. Retain the API integration case, but establish rejection at a completed
   production probe boundary in a new TestNET001OutboundHelloRequiresValidation.
   Use the private production seam:
   Node.probeBitBookPeer(ctx context.Context, id peer.ID) error.
   Its return covers response validation and the confirmation decision. The test
   must observe invalid input rejection and no confirmation after return, then a
   valid response positively confirms the same controlled peer. Manual transport
   setup is allowed for this isolated probe test, not the discovery test.
   This becomes the planned falsification target above; no public API is added
   merely to expose a test completion hook.

3. **Observe bootstrap behavior and owned copies.** bootstrap_test.go:28 checks zero
   connected peers immediately after New; it misses background dial attempts entirely.
   CLI lines 140-143 check only a disabled log. Replace these negative-only oracles
   with controlled dial/default-selection evidence, covering nil and empty library
   config and actual disabled daemon wiring. Keep all possible defaults loopback in
   executing fixtures (a scoped, restored test replacement of the upstream default
   address list is permitted, including inside the child), so a regression cannot
   dial public services. Include an explicit-seed positive control and a completed
   bootstrap/configuration observation; a zero-length snapshot or sleep alone is
   insufficient.
   For dead-plus-healthy seeds, demonstrate New/local service remains usable while
   an owned stalled seed is still pending, then healthy connection succeeds. A
   refused port alone does not exercise a blocked dial. Use bounded observation
   shorter than the 10-second dial timeout, without changing production timing.
   Current mutations replace slice fields; they do not exercise address backing-array
   ownership. Test outer and inner-slice ownership after the relevant production
   configuration/copy boundary is observed, without racing unowned shared inputs.

4. **Cover daemon discovery wiring and early errors.** The CLI test currently proves
   explicit seed connectivity and selection logs, but passes if run() never calls
   StartDiscovery. Observe the child advertise the BitBook namespace through the
   controlled DHT after actual startup. This is distinct from manually calling
   StartDiscovery in a library fixture or probing the handler installed by New.
   Exercise invalid/conflicting CLI flags through the child and prove they fail
   before creating a previously nonexistent owned data directory. Keep test-owned
   processes reaped on every outcome and surface cleanup failures.

5. **Make concurrency/cancellation tests finish and prove the real bound.** The loop
   test has unbounded receives at lines 428/438 and blocked callbacks without failure
   cleanup; the probe test has analogous release/receive paths. Add bounded receives,
   context-aware callbacks, unconditional cancellation/release and joins even on
   t.Fatal. Immediate select/default is not proof that another goroutine could not
   start later. Use barriers/controlled scheduling and assert maximum concurrency
   after all work joins. For independent round tasks, block advertisement and require
   lookup to start before releasing it; an immediate advertise error currently permits
   a sequential implementation. Exercise cancellation with active outbound I/O,
   not only a pre-cancelled context. Cover 31/32/33 candidate limits, each source's
   allowance, and 127/128/129 confirmed entries; assert invalid/self/duplicate behavior
   before capacity eviction can hide it. The fake-clock/private helpers must serve
   production code, with the actual startup/stream cases retained as wiring controls.

### Same-machine Keel coexistence check

Owner asked whether joining the public DHT would conflict with Keel on the same
machine. Read-only source check at Keel c42ccd910c8f01691d8c6db3869625784a4db625:
daemon/swarm/swarm.go requests OS-assigned TCP/QUIC ports (port 0), whereas BitBook's
defaults use 4001. Keel's daemon/swarm/rendezvous.go derives its discovery CID from
keel/rendezvous/1 plus its own protocol/key-scheme version; this ticket's BitBook
namespace is distinct. Both retain their own keys and application protocols.
No default port/namespace conflict was found, and no Keel change is required.
This is a configuration/source finding, not a live two-application acceptance run.
The existing ordinary-IPFS-peer exclusion tests cover the applicable BitBook boundary;
do not add a dependency on running Keel or widen the source correction scope.

### Review 01 source authorization — superseded by review 02

Return this same ticket to **Sol High**. Modify only:

- modern/network/bootstrap_test.go
- modern/network/discovery_test.go
- modern/api/handler_test.go
- modern/cmd/bitbookd/bootstrap_test.go

The other four test files in the table are frozen. Preserve useful assertions and
fixtures; correct the listed proofs rather than rewriting the suite. Private helper
declarations may be referenced by test source as future production requirements; do
not add production stubs or implementations to make the tests compile. Their eventual
real-path use will be reviewed with production. Missing planned symbols alone remain
limited expected-red evidence, as specified above.

No execution, production/dependency edits, report writing, Git or publication by Sol.
No Hermes phase is active. Reviewer will inspect the corrected files, then authorize
red in this ticket. Publication of this review is reviewer-only and limited to this
ticket and CURRENT_TASK.md; developer tests remain uncommitted and untouched.

## Review 02 — corrected drop; three fixture corrections remain

Reviewer: Codex, 2026-09-17. Read source and pinned dependencies only; no test/build
execution. The four frozen tests and all production/module pins still match review 01.
The drop now has twenty TestNET001 declarations (one guarded child) and FuzzPeerHello.

Improvements retained: real handler occupancy and reset checks, capacity boundaries,
startup/provider readiness, completed-probe regression, daemon discovery/early-error
coverage, blocked-advertisement independence, cleanup and final concurrency assertions.
Do not reopen or rewrite those corrections.

### Updated source pins

| Test path | Lines | SHA-256 |
| --- | --- | --- |
| modern/network/bootstrap_test.go | 349 | be3ce5ceca2b05d49010a7113d07782a6e8b7326f86847c64dfb6dd7c007e6f2 |
| modern/network/discovery_test.go | 1069 | dc47eea1009f0a4c77b437232227fd273ce517268ee80bd1c56fd04244e70c58 |
| modern/api/handler_test.go | 606 | b075335978206b134462ea3e64f8d0f7b3b8db62f166154df1ecae433a75ffe9 |
| modern/cmd/bitbookd/bootstrap_test.go | 403 | 23c5348829a97a149567ebafbec53295712be45902d48a63f4b5f21c4e7f41ee |

The other four test hashes remain as in review 01. Original production/module hashes
remain mandatory; discovery.go is still absent.

### Bounded correction instructions

1. **Bootstrap fixture must fit the chosen upstream scheduler.**
   bootstrap_test.go:77-135 waits for both a stalled seed and a gated healthy seed
   before releasing either, then requires healthy connection while the stalled dial
   remains active. Pinned go-libp2p-kad-dht v0.42.2 dht.go:533-555 iterates randomized
   bootstrap peers with synchronous Connect calls, one at a time. With an eight-second
   test context and ten-second dial timeout, the fixture cannot satisfy that ordering.

   Reviewer clarification: review 01's wording could suggest parallel seed dialing.
   That was too strong. The frozen architecture reuses upstream sequential bootstrap;
   New/local work must remain usable during a pending dial, and another seed must be
   tried after failure. Do not add a parallel seed manager to satisfy this test.

   Split the proof into (a) a stalled-only seed where New returns promptly and local
   Put succeeds before releasing the owned stall, and (b) failed-plus-healthy seeds
   where the controlled failure is released and healthy connectivity eventually
   succeeds, independent of seed order. Retain the outer/inner-slice ownership check.
   Remove the now-unneeded gated proxy; keep fixture/constructor goroutines joined
   and close late-arriving connections/results during failure cleanup.

   The loopback-default substitution and positive controls are retained. Their
   250/500 ms negative observations are bounded supporting evidence, not proof a
   bootstrap cycle completed: upstream DHT.Bootstrap only schedules work. Production
   review must separately verify exact empty bootstrap options and disabled CLI wiring.
   No further observation-only production API is required for those negative cases.

2. **Do not read EOF twice as a completion signal.**
   The invalid-response fixtures in discovery_test.go (retry and completed-probe
   cases) and handler_test.go read the entire request through EOF, write the invalid
   response, then read again expecting ErrReset. The requester already closed its
   write half as required by the protocol. That second read can immediately return
   EOF before the remote validation/reset arrives. Pinned go-yamux/v5 v5.1.0
   stream.go:89-108 explicitly returns EOF for an empty half-closed read side.
   This is a fixture race, not a production rejection failure.

   In TestNET001OutboundHelloRequiresValidation, use the production probe's return
   as the completion boundary: require rejection and no confirmed peer, then a valid
   exchange with the same controlled peer. A bounded notification may confirm the
   fixture wrote its intended response, but do not require another read after EOF.
   Keep this test as the falsification target.

   For TestNET001DiscoveryRetryAfterInvalidHello, use two synchronous calls to the
   real runDiscoveryRound helper after seed/provider readiness: finish the invalid
   round, check no confirmation, replace the responder, then finish the valid round.
   Do not run a background StartDiscovery round concurrently in this isolated retry
   test; the separate lifecycle test already proves actual startup discovery.

   For the API integration fixture, notify after successful invalid-response write
   and CloseWrite, without requiring ErrReset on the ended read half. Keep actual
   provider discovery, real-peer positive control, peer/status assertions and
   disconnect/reconfirmation. This is integration coverage; the completed-probe
   regression supplies the deterministic outbound validation/falsification proof.
   Do not describe response-write completion as proof remote validation has completed.

3. **Allow the real daemon's specified retry interval.**
   cmd/bitbookd/bootstrap_test.go:92 permits 45 seconds while requiring the child
   to advertise. Background bootstrap may finish after its first discovery attempt,
   and the next round is scheduled at one minute. Use a bounded 120-second context
   for TestNET001DaemonBootstrapWiring so that valid startup/retry behavior can pass.
   Keep immediate-readiness observations and existing child cleanup; do not shorten
   production intervals or add sleeps to force bootstrap order. The targeted command's
   180-second package timeout remains unchanged.

### Review 02 source authorization — superseded by review 03

Sol High may edit only the four paths in review 02's table, and only to make the three
corrections above plus their necessary fixture/import cleanup. No new test program,
production/dependency changes, execution, records or Git. Preserve the four frozen
test files. This is still the same test-source task, not a new feature or handoff.

No Hermes phase is active until the corrected source is reviewed. Reviewer publication
is limited to this ticket and CURRENT_TASK.md. No developer source is integrated,
committed or executed by this review.

## Review 03 — test source accepted; Hermes expected-red authorization

Reviewer: Codex, 2026-09-17, at HEAD
867c0d6494d078f001749ec87ccb9f5a4e4cc9fa. Static source review only; no tests,
builds or scanners executed. All original production/module pins still match and
discovery.go is absent. The four frozen tests are unchanged. There are 21 TestNET001
declarations, including the guarded child entry, and one FuzzPeerHello target.

All three review-02 corrections are accepted: separate stalled-local-use and
failed-plus-healthy bootstrap cases respect upstream ordering; invalid-response
fixtures no longer reread an ended request half; the daemon discovery case allows
120 seconds for startup and the specified retry. The invalid-probe test checks the
production probe's return, response-write notification and confirmation state, then
positively confirms the same peer with a valid response. The retry case runs two
completed rounds without a competing background loop. API response-write notification
remains supporting integration evidence, not proof that remote validation completed.

Acceptance here authorizes expected-red execution, not feature acceptance. The
bounded negative bootstrap observations retain review 02's limitations. Constructor
failure cleanup cancels and joins New; if New ignores cancellation, the package's
180-second timeout is the outer failure bound. Later production review must verify
empty bootstrap selection, cancellation and real-path use of the private test seams.

### Frozen test inputs for execution

These supersede earlier test hashes; the original production/module pins remain in
force. No source file may change during this phase.

| Test path | Lines | SHA-256 |
| --- | --- | --- |
| modern/network/node_test.go | 548 | 9abc12fc479252c390f78802c9df546e6c4a4e78d71b09acf9db20ba815ab000 |
| modern/network/open_test.go | 92 | 29feea3dfb533bd28e2b1c46a37bf33515787b35867fdc732d72d578cad2ef3d |
| modern/network/bootstrap_test.go | 273 | cc1376f0f8dec262c460c06b40a8283b74b5e8381cccc4bd1fe4a88a559792ad |
| modern/network/discovery_test.go | 1052 | 8e4833bdb1d945f3797aa071f59da7ce33d2b755a2d873c0602eafc59c4f1ae5 |
| modern/network/discovery_fuzz_test.go | 46 | c4f15be93af81e4d732fedafc80275aadf16b3bdc401bdcc0d3ca0acfcde8b0e |
| modern/api/handler_test.go | 602 | 38a990cbccb38a04e6f8394909d429610217a5d134f5f71810a6186c1cf08950 |
| modern/cmd/bitbookd/bootstrap_test.go | 403 | f338c4fe1b8cb96b21e14fdb0d1f0587d50021234816a7ed34ba4d4e0b8848c8 |
| modern/cmd/bitbookd/payment_test.go | 734 | f1ac520574b48f22e41b8700d7852214479ccf3cf4b919ffd8be0deaf06eea31 |

### Hermes assignment — expected red only; closed by review 04

Use this ticket as the complete assignment. Read AGENTS.md and TESTING.md. Verify
the original production/module pins and review 03's eight test pins before and after
execution; retain the observed hashes and line counts. Stop and record any mismatch.
Confirm discovery.go remains absent. Preserve unrelated working-tree changes.

Use the cached Go 1.27.0 binary and environment specified in the execution plan;
verify its listed SHA-256 and capture `go version`. No downloads or module edits.
Inspect filesystem type and available space with `findmnt -T .` and `df -h .` from
modern before creating artifacts. Use disk-backed `modern/dist/net001/red01` for
raw logs and an owned tmp subdirectory, setting TMPDIR and GOTMPDIR to that absolute
tmp path. If red01 already exists, retain it and choose a new numbered directory,
recording the choice. Do not overwrite previous output or use RAM-backed build state.

Run exactly this test command from modern, with the execution plan's environment:

```sh
go test ./network ./api ./cmd/bitbookd -run '^TestNET001' -count=1 -timeout=180s
```

Capture full stdout and stderr directly to retained files, start/end UTC timestamps,
the actual exit code, cwd, command and relevant environment. Preserve the nonzero exit
without letting shell error handling discard the remainder of the evidence capture.
Read-only Git/source/tool metadata commands and artifact/report creation are authorized.

Write the result in `docs/testing/BBGO-NET-001-EXECUTION-01.md`, the single report for
this ticket. Include source identities, static test counts, raw-log paths/hashes,
exact command/result and actual diagnostics. Expected red is missing planned
discovery/bootstrap-selection symbols, or assertions against the old behavior.
A missing-symbol build failure demonstrates only absent implementation; report that
no test bodies ran. Compiler truncation at "too many errors" does not justify claiming
unprinted diagnostics. Fixture syntax, unrelated dependency or environment failures
are gaps requiring review, not accepted red; record them without source repair.

Stop after the capture and report. No production/test changes, stubs, broader tests,
fuzzing, scanners, local daemon build/restart, real-data access or Git mutation are
authorized for Hermes in this phase. Sol's prior edit authority is closed. Reviewer
will read the report and authorize production in this same ticket after accepting red.
Reviewer publication now includes only this ticket and CURRENT_TASK.md; the test drop
remains uncommitted pending the later developer integration phase.

## Review 04 — limited red accepted; production source authorized

Reviewer: Codex, 2026-09-17, at HEAD
f8c459c1218e84706d933bbd704d7a5356f4fa0e. Read the
[execution report](../docs/testing/BBGO-NET-001-EXECUTION-01.md), both retained raw logs,
test source and pinned multiaddr source. No tests, builds or scanners executed.
All 17 distinct production/module/test inputs match their current pins; discovery.go
remains absent. The installed Go binary matches its pinned hash.

### Evidence and disposition

Hermes reports exit 1 for the exact targeted command. Raw output shows all three
packages failed to build, with missing discovery/parser/bootstrap-selection symbols.
No test bodies ran. This is accepted only as evidence of absent implementation.

The report's assertion that all failures are missing symbols is **rejected**.
The compiler also reports a real test defect at bootstrap_test.go:398: slices.Equal
requires comparable elements, but pinned go-multiaddr v0.16.1 defines Multiaddr as
[]Component. This is not a Go-version exception. Static inspection finds the same
invalid comparison at line 59, although that diagnostic is not in the raw log.
Reviewer missed these two sites during source review and owns that oversight.

This fixture error is not accepted as expected red. Its exact repair is authorized
below, alongside production, because the independently reported missing production
symbols already establish the limited pre-implementation failure. Removing the two
invalid comparisons cannot implement those symbols or make the original baseline
pass. No additional red-only relay is required. All later green, race, fuzz and
falsification gates remain in force; no behavioral test validity is claimed here.

Retained identities (SHA-256):

| Evidence | Bytes | SHA-256 |
| --- | --- | --- |
| docs/testing/BBGO-NET-001-EXECUTION-01.md | 5248 | c655f46fe7ae92b29fce880f931986ae3dd0d193bc06e50b3ea295672458ac80 |
| modern/dist/net001/red01/test.stdout.log | 195 | 2d9b4760cbeecb482e2749502b73f94aaeca3d7794051b72dbcd58a33eda9086 |
| modern/dist/net001/red01/test.stderr.log | 2744 | b2fed211f6c1f5937886b92f32733ae7b22e98ebe44d7211a33a7963e17fb958 |

Capture limitations: only stdout/stderr logs were retained. Start/end timestamps,
execution-time tool identity, full relevant environment and before/after pin captures
are missing. Current hashes verify the present files, not an independently retained
execution-time snapshot. Exit 1 and the environment are executor-reported. The report
adds -mod=readonly to GOFLAGS; this strengthens the no-module-edit restriction and is
accepted. Do not reconstruct missing metadata from file times or invent captures.

This review supersedes the report's erroneous verdict. At its next authorized phase,
Hermes must correct the same report, record these gaps, include the static declaration
counts from review 03, and replace its local absolute cwd with repository-relative
modern before publication. Preserve the original raw logs. No separate report-only
task or rerun is needed now; the report remains uncommitted.

### Sol High assignment — superseded by review 05

Read the full frozen behavior and accepted tests in this ticket. Verify original
production/module pins and review 03's test pins before editing. First repair only
the two address-slice comparisons at lines 59 and 398 of
modern/cmd/bitbookd/bootstrap_test.go: use slices.EqualFunc with ma.Multiaddr.Equal
as the element comparator. Preserve ordered address equality, peer-ID checks and
all assertions. No other test changes are authorized; the other seven tests stay
byte-identical. This repair is required even if compiler truncation hides one site.

Then implement the frozen contract in exactly these five production paths:

- modern/network/node.go
- modern/network/protocols.go
- modern/network/discovery.go (new)
- modern/cmd/bitbookd/main.go
- modern/api/handler.go

Use upstream public DHT/Bitswap and background bootstrap, with CLI defaults and
explicit empty library bootstrap options. Implement discovery/hello validation,
resource limits, cancellation/Close joins and confirmed-peer API filtering exactly
as specified above. Keep transport-based publication checks intact. The tests'
private helpers must serve the real production handler/round/probe/lifecycle;
disconnected test-only implementations do not satisfy the contract. Preserve the
existing keys, private datastore boundary, direct/payment protocols and services.

Sol may read dependencies/source and format only these six writable paths. No
dependency changes, other source/test edits, execution, records, Git or publication.
Do not change intervals, capacities or assertions to accommodate implementation.
Stop after the source drop; reviewer reads it directly and authorizes Hermes's next
execution phase in this same ticket. No actor has been launched by this review.
Reviewer publication is limited to this ticket and CURRENT_TASK.md; preserve all
unrelated work and the uncommitted developer source/report.

## Review 05 — production review; discovery correction required

Reviewer: Codex, 2026-09-17, at HEAD
0c2a7569f7c17bc5f7fdd65149cd507904e94505. Source inspection only; no execution.
The four existing production edits stay within scope. Public protocol selection,
explicit empty library bootstrap, upstream background bootstrap, CLI selection,
daemon discovery startup and API filtering match the specified wiring. Publication's
transport checks are unchanged. Seven frozen tests and original module/identity/open
inputs match. Reversing exactly the two EqualFunc repairs reconstructs the previous
bootstrap_test.go hash; their comparator closures are equivalent to the requested
Multiaddr.Equal method expression. No further repair to that test is needed.

### Drop identities

| Path | Lines | SHA-256 |
| --- | --- | --- |
| modern/network/node.go | 275 | 044de22794980774d0268c902ed4a54441c8ab4f52e7af2ac25640f029cc94f9 |
| modern/network/protocols.go | 26 | a17f85edf8bb8dd52a12f6d826d1637ad32cd5d27daf534bac672943f1eafd03 |
| modern/network/discovery.go | 652 | 781e937f1a3d348baaf191b8163fa0f6d45a76f8d283ceaf8b6f6a1cc2a6a832 |
| modern/cmd/bitbookd/main.go | 287 | fe2d3b2c07d3ddc0948889dc37158b90f5aecda79bd83d0974d1f61548f1caea |
| modern/api/handler.go | 652 | 678511fee172b1b9f0aef0c85eb5e724f162aa233d65e535bcbe8708630054fd |
| modern/cmd/bitbookd/bootstrap_test.go | 407 | b348eabe4afcef497200767dac8e4a39743441b88cefb0be98d299d1dcd50ef5 |

### Required corrections

1. **A reconnect can inherit stale confirmation.** discovery.go:71-75 decides whether
   to forget using the network's current connections when DisconnectedF runs. Pinned
   libp2p v0.49.0 swarm_conn.go:56-107 explicitly dispatches disconnect notifications
   asynchronously. If the last old connection closes and a new one connects before
   that callback, the callback sees the new connection and skips forgetting. The
   peer-only snapshot at lines 149-151 then reports the new connection as confirmed
   without a fresh hello. Separately, the unguarded confirmation commits at lines
   288 and 339 can run after disconnect cleanup and restore an obsolete confirmation.
   These are source-derived interleavings, not claimed runtime observations.

   Bind confirmation to the connection lifetime that actually performed the hello,
   and serialize its validation/commit with connection-lifecycle invalidation.
   Neither a delayed callback nor a late old-stream completion may carry confirmation
   across a last-connection gap. Apply the same validity decision to BitBookPeers and
   discovery's already-confirmed candidate filtering; hiding stale entries only from
   the API would leave them ineligible for reprobe. Preserve confirmation while
   another connection remains continuously live, and do not let an old delayed
   disconnect erase a freshly validated connection. Keep tracking bounded and avoid
   stream I/O or network shutdown while holding the confirmation lock.

2. **Handler availability is signalled before its slot is released.** The deferred
   cleanup at discovery.go:313-320 decrements activeHandlers, signals and unlocks
   before returning the token. waitForDiscoveryHandlers can return 15 while all 16
   tokens are still occupied, so the existing slot-reuse test can receive a spurious
   excess reset. Release the token and publish the updated occupancy consistently
   under the same synchronization, preserving immediate rejection of excess work.

### Sol High correction scope and regression evidence

Only modern/network/discovery.go and modern/network/discovery_test.go may change.
Freeze the other five paths in the table and every other test/module input. Keep
this as the current implementation correction, with no protocol or product redesign.

Before editing discovery.go, retain its exact pinned bytes as the non-Go source
fixture modern/dist/net001/review05/discovery.go.txt. Inspect the filesystem type
before creation, use disk storage, and never overwrite a differing existing fixture.
This one source copy is explicitly allowed; it is not an execution report or a
claim of test evidence. It remains uncommitted for Hermes's later reproduction.

First add TestNET001ConfirmationDoesNotSurviveReconnect in discovery_test.go, using
the existing production entry points so it also compiles with the retained old
discovery.go. With controlled authenticated loopback peers, prove a valid initial
hello, delay the subject's production disconnect callback, close the last connection,
and reconnect without a new hello. Assert the peer is unconfirmed both before and
after delivery of the delayed callback. Then complete a fresh valid hello and prove
confirmation returns. Delay only the relevant callback, forward other lifecycle
events, and use barriers with bounded cleanup rather than sleeps to arrange ordering.
Do not disable discovery logic or manually insert confirmation state. Manual dialing
is appropriate for this connection-lifetime test; retain the existing real discovery
test without test-side dialing. Preserve all existing assertions and tests.

Then correct the two production findings. Existing inbound slot-reuse and
disconnect/malformed-reconnect tests remain required. Reviewer will inspect the
correction and new regression before authorizing execution. At the next Hermes phase,
first run the regression in a disposable copy with only discovery.go restored from
the pinned fixture; it must fail on stale confirmation, not compilation or timeout.
Then run it against corrected source and proceed to the existing acceptance plan.
The exact regression command, from each copy's modern directory, is:

```sh
go test ./network -run '^TestNET001ConfirmationDoesNotSurviveReconnect$' -count=1 -timeout=90s
```

This specifies the later execution; it is not yet authorization for Hermes. No
execution, dependency changes, other source edits, report writing or Git by Sol.
No extra report/handoff document: use this ticket and the existing execution report.
Reviewer publication is limited to this ticket and CURRENT_TASK.md. The production
drop is not accepted for broader execution, build or publication yet.
