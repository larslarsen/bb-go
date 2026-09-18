package network

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/namesys"
	boxopath "github.com/ipfs/boxo/path"
	blocks "github.com/ipfs/go-block-format"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	kb "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p-kbucket/peerdiversity"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	manet "github.com/multiformats/go-multiaddr/net"
)

func TestNET001PublicIPFSAndApplicationProtocols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	n, err := New(ctx, Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:     dht.ModeServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	protocols := n.Protocols()
	if DHTProtocolPrefix != "/ipfs" || DHTProtocolCurrent != "/ipfs/kad/1.0.0" {
		t.Fatalf("public DHT constants = prefix %q current %q", DHTProtocolPrefix, DHTProtocolCurrent)
	}
	if BitswapProtocolPrefix != "" || BitswapProtocolCurrent != "/ipfs/bitswap/1.2.0" {
		t.Fatalf("public Bitswap constants = prefix %q current %q", BitswapProtocolPrefix, BitswapProtocolCurrent)
	}
	if DirectProtocolCurrent != "/bitbook/direct/1.0.0" || PaymentProtocolCurrent != "/bitbook/payment/1.0.0" {
		t.Fatalf("application protocol IDs changed: direct=%q payment=%q", DirectProtocolCurrent, PaymentProtocolCurrent)
	}
	if DiscoveryProtocolCurrent != "/bitbook/discovery/1.0.0" || DiscoveryNamespace != "/bitbook/peers/1.0.0" {
		t.Fatalf("discovery constants = protocol %q namespace %q", DiscoveryProtocolCurrent, DiscoveryNamespace)
	}
	for _, bitswapProtocol := range []protocol.ID{
		bsnet.ProtocolBitswap,
		bsnet.ProtocolBitswapOneOne,
		bsnet.ProtocolBitswapOneZero,
		bsnet.ProtocolBitswapNoVers,
	} {
		if !slices.Contains(protocols, bitswapProtocol) {
			t.Fatalf("standard Bitswap protocol %q missing; protocols: %v", bitswapProtocol, protocols)
		}
	}
	if !slices.Contains(protocols, protocol.ID("/ipfs/kad/1.0.0")) {
		t.Fatalf("public DHT protocol missing; protocols: %v", protocols)
	}
	for _, id := range protocols {
		if strings.HasPrefix(string(id), "/bitbook/ipfs/bitswap/") || id == "/bitbook/kad/1.0.0" {
			t.Fatalf("obsolete private IPFS protocol registered: %s", id)
		}
	}
}

func TestNET001IndependentPublicIPFSInterop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	upstream := newUpstreamFixture(t, ctx)
	node, err := New(ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	connectUpstreamAndNode(t, ctx, upstream, node)

	fromNode := []byte("public block from BitBook node")
	nodeCID, err := node.Put(ctx, fromNode)
	if err != nil {
		t.Fatal(err)
	}
	upstreamBlock, err := upstream.bitswap.GetBlock(ctx, nodeCID)
	if err != nil {
		t.Fatalf("upstream Bitswap read: %v", err)
	}
	if !bytes.Equal(upstreamBlock.RawData(), fromNode) {
		t.Fatalf("upstream Bitswap read = %q", upstreamBlock.RawData())
	}

	fromUpstream := blocks.NewBlock([]byte("public block from independent upstream"))
	if err := upstream.blocks.Put(ctx, fromUpstream); err != nil {
		t.Fatal(err)
	}
	if err := upstream.bitswap.NotifyNewBlocks(ctx, fromUpstream); err != nil {
		t.Fatal(err)
	}
	got, err := node.Get(ctx, fromUpstream.Cid())
	if err != nil || !bytes.Equal(got, fromUpstream.RawData()) {
		t.Fatalf("BitBook Bitswap read = %q, %v", got, err)
	}

	if err := node.PublishRoot(ctx, nodeCID); err != nil {
		t.Fatal(err)
	}
	resolvedNode, err := upstream.resolver.Resolve(ctx, ipns.NameFromPeer(node.ID()).AsPath(), namesys.ResolveWithDhtRecordCount(1))
	if err != nil || resolvedNode.Path.String() != "/ipfs/"+nodeCID.String() {
		t.Fatalf("upstream IPNS resolution = %v, %v", resolvedNode.Path, err)
	}

	upstreamKey := upstream.host.Peerstore().PrivKey(upstream.host.ID())
	if upstreamKey == nil {
		t.Fatal("independent host has no retained private key")
	}
	if err := upstream.publisher.Publish(ctx, upstreamKey, boxopath.FromCid(fromUpstream.Cid()), namesys.PublishWithEOL(time.Now().Add(7*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	resolvedUpstream, err := node.ResolveRoot(ctx, upstream.host.ID())
	if err != nil || resolvedUpstream != fromUpstream.Cid() {
		t.Fatalf("BitBook IPNS resolution = %s, %v", resolvedUpstream, err)
	}

	name := ipns.NameFromPeer(node.ID())
	rawRecord, err := node.Datastore.Get(ctx, namesys.IpnsDsKey(name))
	if err != nil {
		t.Fatal(err)
	}
	tampered := slices.Clone(rawRecord)
	tampered[len(tampered)-1] ^= 1
	if record, err := ipns.UnmarshalRecord(tampered); err == nil {
		if err := ipns.ValidateWithName(record, name); err == nil {
			t.Fatal("tampered IPNS record passed independent validation")
		}
	}
	if err := upstream.dht.PutValue(ctx, string(name.RoutingKey()), tampered); err == nil {
		t.Fatal("public DHT accepted tampered IPNS record")
	}
}

func TestNodeCloseIsIdempotent(t *testing.T) {
	n, err := New(context.Background(), Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTwoNodesTransferBlockOverBitBookBitswap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	server, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Connect(ctx, peer.AddrInfo{ID: server.ID()}); err == nil {
		t.Fatal("connect unexpectedly succeeded without server addresses")
	}
	serverInfo := server.Host.Peerstore().PeerInfo(server.ID())
	serverInfo.Addrs = server.Host.Addrs()
	if err := client.Connect(ctx, serverInfo); err != nil {
		t.Fatalf("connect: %v", err)
	}

	want := []byte("BitBook modern network block")
	id, err := server.Put(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wrong block: got %q want %q", got, want)
	}
}

func TestTwoNodesPublishAndResolveBitBookRoot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	author, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer author.Close()
	reader, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	authorInfo := peer.AddrInfo{ID: author.ID(), Addrs: author.Host.Addrs()}
	readerInfo := peer.AddrInfo{ID: reader.ID(), Addrs: reader.Host.Addrs()}
	if err := reader.Connect(ctx, authorInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.Connect(ctx, readerInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reader.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	waitForRoutingPeer(t, ctx, author)
	waitForRoutingPeer(t, ctx, reader)

	want := []byte("signed BitBook social root")
	root, err := author.Put(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if err := author.PublishRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	providers := reader.DHT.FindProvidersAsync(ctx, root, 1)
	provider, ok := <-providers
	if !ok {
		t.Fatal("published root has no DHT provider")
	}
	if provider.ID != author.ID() {
		t.Fatalf("root provider is %s, want author %s", provider.ID, author.ID())
	}
	resolved, err := reader.ResolveRoot(ctx, author.ID())
	if err != nil {
		t.Fatal(err)
	}
	if resolved != root {
		t.Fatalf("resolved %s, want %s", resolved, root)
	}
	got, err := reader.Get(ctx, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wrong resolved content: got %q want %q", got, want)
	}
}

func TestPublishedIPNSRecordLifetime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	author, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer author.Close()
	neighbor, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer neighbor.Close()

	authorInfo := peer.AddrInfo{ID: author.ID(), Addrs: author.Host.Addrs()}
	neighborInfo := peer.AddrInfo{ID: neighbor.ID(), Addrs: neighbor.Host.Addrs()}
	if err := neighbor.Connect(ctx, authorInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.Connect(ctx, neighborInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := neighbor.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	waitForRoutingPeer(t, ctx, author)
	waitForRoutingPeer(t, ctx, neighbor)

	root, err := author.Put(ctx, []byte("BitBook IPNS lifetime root"))
	if err != nil {
		t.Fatal(err)
	}

	const wantLifetime = 7 * 24 * time.Hour
	started := time.Now()
	if err := author.PublishRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	finished := time.Now()

	raw, err := author.Datastore.Get(ctx, namesys.IpnsDsKey(ipns.NameFromPeer(author.ID())))
	if err != nil {
		t.Fatalf("reading stored IPNS record: %v", err)
	}
	rec, err := ipns.UnmarshalRecord(raw)
	if err != nil {
		t.Fatalf("decoding stored IPNS record: %v", err)
	}
	if err := ipns.ValidateWithName(rec, ipns.NameFromPeer(author.ID())); err != nil {
		t.Fatalf("stored IPNS record is not valid for author %s: %v", author.ID(), err)
	}

	validityType, err := rec.ValidityType()
	if err != nil {
		t.Fatal(err)
	}
	if validityType != ipns.ValidityEOL {
		t.Fatalf("validity type is %d, want EOL", validityType)
	}

	eol, err := rec.Validity()
	if err != nil {
		t.Fatal(err)
	}
	minEOL := started.Add(wantLifetime)
	maxEOL := finished.Add(wantLifetime)
	if eol.Before(minEOL) || eol.After(maxEOL) {
		t.Fatalf("published IPNS EOL is %s from start (observed %s); want [%s, %s] (~7d)",
			eol.Sub(started), eol, minEOL, maxEOL)
	}

	ttl, err := rec.TTL()
	if err != nil {
		t.Fatal(err)
	}
	if ttl != ipns.DefaultRecordTTL {
		t.Fatalf("record TTL is %s, want DefaultRecordTTL %s", ttl, ipns.DefaultRecordTTL)
	}

	value, err := rec.Value()
	if err != nil {
		t.Fatal(err)
	}
	wantValue := "/ipfs/" + root.String()
	if value.String() != wantValue {
		t.Fatalf("record value is %s, want %s", value, wantValue)
	}
}

func TestDHTRoutingTableEnforcesIPDiversity(t *testing.T) {
	for _, allowPrivate := range []bool{false, true} {
		name := "default"
		if allowPrivate {
			name = "allowPrivateAddresses"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			config := Config{
				ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
				DHTMode:               dht.ModeServer,
				AllowPrivateAddresses: allowPrivate,
			}
			hub, err := New(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			defer hub.Close()

			if hub.DHT.GetRoutingTableDiversityStats() == nil {
				t.Fatal("constructed BitBook DHT has no routing-table IP-diversity filter")
			}

			extraCount := amino.DefaultMaxPeersPerIPGroup + 1
			extras := make([]*Node, 0, extraCount)
			defer func() {
				for i := len(extras) - 1; i >= 0; i-- {
					_ = extras[i].Close()
				}
			}()
			for i := 0; i < extraCount; i++ {
				extra, err := New(ctx, config)
				if err != nil {
					t.Fatal(err)
				}
				extras = append(extras, extra)
				info := peer.AddrInfo{ID: extra.ID(), Addrs: extra.Host.Addrs()}
				if err := hub.Connect(ctx, info); err != nil {
					t.Fatalf("connect extra %d: %v", i, err)
				}
				waitForConnected(t, ctx, hub, extra.ID())
			}

			rt := hub.DHT.RoutingTable()
			accepted := 0
			rejected := 0
			for _, extra := range extras {
				if rt.Find(extra.ID()) != "" {
					accepted++
					continue
				}
				_, err := rt.TryAddPeer(extra.ID(), true, false)
				if err != nil {
					if !strings.Contains(err.Error(), "diversity filter") {
						t.Fatalf("adding %s: %v", extra.ID(), err)
					}
					rejected++
					continue
				}
				if rt.Find(extra.ID()) == "" {
					t.Fatalf("peer %s was not added to the routing table", extra.ID())
				}
				accepted++
			}
			if accepted == 0 {
				t.Fatal("no same-IP-group peers were admitted; the diversity check did not run")
			}
			if rejected == 0 {
				t.Fatalf("admitted %d same-IP-group peers with no diversity-filter rejection; Amino table limit is %d",
					accepted, amino.DefaultMaxPeersPerIPGroup)
			}

			sameGroup, perCpl := countSameIPGroupPeers(t, hub)
			if sameGroup > amino.DefaultMaxPeersPerIPGroup {
				t.Fatalf("routing table has %d peers from one IP group; Amino DefaultMaxPeersPerIPGroup is %d",
					sameGroup, amino.DefaultMaxPeersPerIPGroup)
			}
			for cpl, n := range perCpl {
				if n > amino.DefaultMaxPeersPerIPGroupPerCpl {
					t.Fatalf("cpl %d has %d peers from one IP group; Amino DefaultMaxPeersPerIPGroupPerCpl is %d",
						cpl, n, amino.DefaultMaxPeersPerIPGroupPerCpl)
				}
			}
		})
	}
}

func waitForConnected(t *testing.T, ctx context.Context, n *Node, id peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if n.Host.Network().Connectedness(id) == network.Connected && len(n.Host.Network().ConnsToPeer(id)) > 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for connection to %s: %v", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

func countSameIPGroupPeers(t *testing.T, hub *Node) (int, map[int]int) {
	t.Helper()
	hubKey := kb.ConvertPeerID(hub.ID())
	groupCounts := map[peerdiversity.PeerIPGroupKey]int{}
	groupCpl := map[peerdiversity.PeerIPGroupKey]map[int]int{}
	for _, pid := range hub.DHT.RoutingTable().ListPeers() {
		groups := peerIPGroups(hub, pid)
		if len(groups) == 0 {
			t.Fatalf("routing-table peer %s has no connection addresses", pid)
		}
		seen := map[peerdiversity.PeerIPGroupKey]struct{}{}
		cpl := kb.CommonPrefixLen(hubKey, kb.ConvertPeerID(pid))
		for _, g := range groups {
			if _, ok := seen[g]; ok {
				continue
			}
			seen[g] = struct{}{}
			groupCounts[g]++
			if groupCpl[g] == nil {
				groupCpl[g] = map[int]int{}
			}
			groupCpl[g][cpl]++
		}
	}

	maxGroup := 0
	var maxKey peerdiversity.PeerIPGroupKey
	for g, n := range groupCounts {
		if n > maxGroup {
			maxGroup = n
			maxKey = g
		}
	}
	perCpl := groupCpl[maxKey]
	if perCpl == nil {
		perCpl = map[int]int{}
	}
	return maxGroup, perCpl
}

func peerIPGroups(n *Node, id peer.ID) []peerdiversity.PeerIPGroupKey {
	conns := n.Host.Network().ConnsToPeer(id)
	groups := make([]peerdiversity.PeerIPGroupKey, 0, len(conns))
	for _, c := range conns {
		ip, err := manet.ToIP(c.RemoteMultiaddr())
		if err != nil {
			continue
		}
		key := peerdiversity.IPGroupKey(ip)
		if len(key) == 0 {
			continue
		}
		groups = append(groups, key)
	}
	return groups
}

func waitForRoutingPeer(t *testing.T, ctx context.Context, n *Node) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for n.DHT.RoutingTable().Size() == 0 {
		select {
		case <-ctx.Done():
			t.Fatalf("DHT routing table remained empty: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
