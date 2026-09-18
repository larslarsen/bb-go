package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	corediscovery "github.com/libp2p/go-libp2p/core/discovery"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

const (
	discoveryRoundInterval = time.Minute
	discoveryRoundTimeout  = 30 * time.Second
	discoveryProbeTimeout  = 5 * time.Second

	discoveryProviderLimit     = 32
	discoveryConnectedLimit    = 32
	discoveryProbeConcurrency  = 4
	discoveryInboundLimit      = 16
	discoveryConfirmedCapacity = 128

	peerHello = "BBGO001\n"
)

type discoveryState struct {
	node      *Node
	ctx       context.Context
	routing   *routingdiscovery.RoutingDiscovery
	confirmed *confirmedPeers

	inboundSlots chan struct{}
	notifiee     *lp2pnet.NotifyBundle

	mu             sync.Mutex
	changed        chan struct{}
	closing        bool
	started        bool
	running        bool
	loopCancel     context.CancelFunc
	activeHandlers int
	activeStreams  map[lp2pnet.Stream]struct{}
	loopWG         sync.WaitGroup
	handlerWG      sync.WaitGroup

	roundMu               sync.Mutex
	lastAdvertisement     time.Time
	advertisementTTL      time.Duration
	connectedPeerRotation int
}

func newDiscoveryState(node *Node, ctx context.Context) *discoveryState {
	state := &discoveryState{
		node:          node,
		ctx:           ctx,
		routing:       routingdiscovery.NewRoutingDiscovery(node.DHT),
		confirmed:     newConfirmedPeers(node.ID()),
		inboundSlots:  make(chan struct{}, discoveryInboundLimit),
		changed:       make(chan struct{}),
		activeStreams: make(map[lp2pnet.Stream]struct{}),
	}
	state.notifiee = &lp2pnet.NotifyBundle{
		ConnectedF: func(network lp2pnet.Network, connection lp2pnet.Conn) {
			state.reconcilePeerConnections(network, connection.RemotePeer())
		},
		DisconnectedF: func(network lp2pnet.Network, connection lp2pnet.Conn) {
			state.reconcilePeerConnections(network, connection.RemotePeer())
		},
	}
	node.Host.Network().Notify(state.notifiee)
	node.Host.SetStreamHandler(DiscoveryProtocolCurrent, state.handleInboundHello)

	state.loopWG.Add(1)
	go func() {
		defer state.loopWG.Done()
		<-ctx.Done()
		state.resetActiveStreams()
	}()
	return state
}

// StartDiscovery advertises this node and starts periodic discovery over the
// public DHT. A stopped discovery lifecycle cannot be restarted on the same node.
func (n *Node) StartDiscovery(ctx context.Context) error {
	if n == nil || n.discovery == nil {
		return errors.New("nil network node")
	}
	if ctx == nil {
		return errors.New("nil discovery context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := n.ctx.Err(); err != nil {
		return fmt.Errorf("network node is closed: %w", err)
	}

	state := n.discovery
	state.mu.Lock()
	if state.closing {
		state.mu.Unlock()
		return errors.New("network node is closed")
	}
	if state.started {
		if state.running {
			state.mu.Unlock()
			return nil
		}
		state.mu.Unlock()
		return errors.New("discovery cannot be restarted")
	}

	loopCtx, cancel := context.WithCancel(ctx)
	stopWithNode := context.AfterFunc(n.ctx, cancel)
	state.started = true
	state.running = true
	state.loopCancel = cancel
	state.loopWG.Add(1)
	state.mu.Unlock()

	go func() {
		defer state.loopWG.Done()
		defer stopWithNode()
		ticker := time.NewTicker(discoveryRoundInterval)
		defer ticker.Stop()
		runDiscoveryLoop(loopCtx, ticker.C, n.runDiscoveryRound)
		state.mu.Lock()
		state.running = false
		state.signalLocked()
		state.mu.Unlock()
	}()
	return nil
}

// BitBookPeers returns a sorted owned snapshot of connected peers that have
// completed the BitBook discovery exchange.
func (n *Node) BitBookPeers() []peer.ID {
	if n == nil || n.discovery == nil {
		return []peer.ID{}
	}
	return n.discovery.confirmed.connectedSnapshot(n.Host.Network())
}

func (n *Node) runDiscoveryRound(ctx context.Context) error {
	if n == nil || n.discovery == nil {
		return errors.New("nil network node")
	}
	if ctx == nil {
		return errors.New("nil discovery context")
	}
	if err := n.ctx.Err(); err != nil {
		return fmt.Errorf("network node is closed: %w", err)
	}
	state := n.discovery
	state.roundMu.Lock()
	defer state.roundMu.Unlock()

	roundCtx, cancel := context.WithTimeout(ctx, discoveryRoundTimeout)
	defer cancel()
	stopWithNode := context.AfterFunc(n.ctx, cancel)
	defer stopWithNode()
	return runDiscoveryTasks(roundCtx,
		func(taskCtx context.Context) error {
			now := time.Now()
			if !discoveryAdvertisementDue(now, state.lastAdvertisement, state.advertisementTTL) {
				return nil
			}
			ttl, err := state.routing.Advertise(taskCtx, DiscoveryNamespace)
			if err != nil {
				return err
			}
			state.lastAdvertisement = time.Now()
			state.advertisementTTL = ttl
			return nil
		},
		func(taskCtx context.Context) error {
			providers, err := n.findDiscoveryProviders(taskCtx)
			if err != nil {
				return err
			}
			connected := make([]peer.ID, 0, len(n.Host.Network().Peers()))
			for _, id := range n.Host.Network().Peers() {
				if !state.isConfirmedPeer(id) {
					connected = append(connected, id)
				}
			}
			slices.Sort(connected)
			candidates, nextRotation := selectDiscoveryCandidates(n.ID(), providers, connected, state.connectedPeerRotation)
			state.connectedPeerRotation = nextRotation
			return runDiscoveryProbes(taskCtx, candidates, n.probeBitBookPeer)
		},
	)
}

func (n *Node) findDiscoveryProviders(ctx context.Context) ([]peer.ID, error) {
	found, err := n.discovery.routing.FindPeers(ctx, DiscoveryNamespace, corediscovery.Limit(discoveryProviderLimit))
	if err != nil {
		return nil, err
	}
	providers := make([]peer.ID, 0, discoveryProviderLimit)
	for {
		select {
		case info, ok := <-found:
			if !ok {
				return providers, nil
			}
			if !validPeerID(info.ID) || info.ID == n.ID() || n.discovery.isConfirmedPeer(info.ID) {
				continue
			}
			if len(info.Addrs) > 0 {
				n.Host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)
			}
			providers = append(providers, info.ID)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (n *Node) probeBitBookPeer(ctx context.Context, id peer.ID) error {
	if n == nil || n.discovery == nil {
		return errors.New("nil network node")
	}
	if ctx == nil {
		return errors.New("nil probe context")
	}
	if err := n.ctx.Err(); err != nil {
		return fmt.Errorf("network node is closed: %w", err)
	}
	if !validPeerID(id) || id == n.ID() {
		return errors.New("invalid discovery peer")
	}
	probeCtx, cancel := context.WithTimeout(ctx, discoveryProbeTimeout)
	defer cancel()
	stopWithNode := context.AfterFunc(n.ctx, cancel)
	defer stopWithNode()
	stream, err := n.Host.NewStream(probeCtx, id, DiscoveryProtocolCurrent)
	if err != nil {
		return fmt.Errorf("opening discovery stream to %s: %w", id, err)
	}
	if !n.discovery.trackStream(stream) {
		_ = stream.Reset()
		return errors.New("discovery is stopping")
	}
	defer n.discovery.untrackStream(stream)
	resetDone := make(chan struct{})
	stopOnCancel := context.AfterFunc(probeCtx, func() {
		_ = stream.Reset()
		close(resetDone)
	})
	defer func() {
		if !stopOnCancel() {
			<-resetDone
		}
	}()
	deadline, _ := probeCtx.Deadline()
	if err := stream.SetDeadline(deadline); err != nil {
		_ = stream.Reset()
		return err
	}
	if _, err := io.WriteString(stream, peerHello); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("writing discovery hello to %s: %w", id, err)
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("closing discovery request to %s: %w", id, err)
	}
	if err := readPeerHello(stream); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("validating discovery response from %s: %w", id, err)
	}
	if err := probeCtx.Err(); err != nil {
		_ = stream.Reset()
		return err
	}
	_ = stream.Close()
	n.discovery.confirmConnection(stream.Conn(), time.Now())
	return nil
}

func (state *discoveryState) handleInboundHello(stream lp2pnet.Stream) {
	state.mu.Lock()
	if state.closing || state.ctx.Err() != nil {
		state.mu.Unlock()
		_ = stream.Reset()
		return
	}
	select {
	case state.inboundSlots <- struct{}{}:
	default:
		state.mu.Unlock()
		_ = stream.Reset()
		return
	}
	state.handlerWG.Add(1)
	state.activeHandlers++
	state.activeStreams[stream] = struct{}{}
	state.signalLocked()
	state.mu.Unlock()

	defer func() {
		state.mu.Lock()
		delete(state.activeStreams, stream)
		<-state.inboundSlots
		state.activeHandlers--
		state.signalLocked()
		state.mu.Unlock()
		state.handlerWG.Done()
	}()

	if err := stream.SetDeadline(time.Now().Add(discoveryProbeTimeout)); err != nil {
		_ = stream.Reset()
		return
	}
	if err := readPeerHello(stream); err != nil {
		_ = stream.Reset()
		return
	}
	if _, err := io.WriteString(stream, peerHello); err != nil {
		_ = stream.Reset()
		return
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return
	}
	state.confirmConnection(stream.Conn(), time.Now())
	_ = stream.Close()
}

func (state *discoveryState) reconcilePeerConnections(network lp2pnet.Network, id peer.ID) {
	state.confirmed.reconcileConnections(id, network)
}

func (state *discoveryState) isConfirmedPeer(id peer.ID) bool {
	return state.confirmed.containsConnected(id, state.node.Host.Network())
}

func (state *discoveryState) confirmConnection(connection lp2pnet.Conn, at time.Time) {
	if connection == nil {
		return
	}
	state.confirmed.confirmConnection(connection, state.node.Host.Network(), at)
}

func (n *Node) waitForDiscoveryHandlers(ctx context.Context, count int) error {
	if n == nil || n.discovery == nil {
		return errors.New("nil network node")
	}
	if ctx == nil {
		return errors.New("nil wait context")
	}
	if count < 0 || count > discoveryInboundLimit {
		return errors.New("invalid discovery handler count")
	}
	for {
		n.discovery.mu.Lock()
		if n.discovery.activeHandlers == count {
			n.discovery.mu.Unlock()
			return nil
		}
		changed := n.discovery.changed
		n.discovery.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (state *discoveryState) trackStream(stream lp2pnet.Stream) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closing || state.ctx.Err() != nil {
		return false
	}
	state.activeStreams[stream] = struct{}{}
	return true
}

func (state *discoveryState) untrackStream(stream lp2pnet.Stream) {
	state.mu.Lock()
	delete(state.activeStreams, stream)
	state.mu.Unlock()
}

func (state *discoveryState) resetActiveStreams() {
	state.mu.Lock()
	streams := make([]lp2pnet.Stream, 0, len(state.activeStreams))
	for stream := range state.activeStreams {
		streams = append(streams, stream)
	}
	state.mu.Unlock()
	for _, stream := range streams {
		_ = stream.Reset()
	}
}

func (state *discoveryState) close() {
	state.node.Host.RemoveStreamHandler(DiscoveryProtocolCurrent)
	state.node.Host.Network().StopNotify(state.notifiee)
	state.mu.Lock()
	state.closing = true
	if state.loopCancel != nil {
		state.loopCancel()
	}
	state.signalLocked()
	state.mu.Unlock()
	state.resetActiveStreams()
	state.loopWG.Wait()
	state.handlerWG.Wait()
}

func (state *discoveryState) signalLocked() {
	close(state.changed)
	state.changed = make(chan struct{})
}

func readPeerHello(reader io.Reader) error {
	raw, err := io.ReadAll(io.LimitReader(reader, int64(len(peerHello)+1)))
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, []byte(peerHello)) {
		return errors.New("invalid BitBook peer hello")
	}
	return nil
}

func runDiscoveryLoop(ctx context.Context, ticks <-chan time.Time, round func(context.Context) error) {
	if ctx == nil || round == nil || ctx.Err() != nil {
		return
	}
	_ = round(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			if ctx.Err() != nil {
				return
			}
			_ = round(ctx)
		}
	}
}

func discoveryAdvertisementDue(now, lastSuccess time.Time, ttl time.Duration) bool {
	return lastSuccess.IsZero() || ttl <= 0 || !now.Before(lastSuccess.Add(ttl/2))
}

func runDiscoveryTasks(ctx context.Context, advertise, lookup func(context.Context) error) error {
	if ctx == nil {
		return errors.New("nil discovery task context")
	}
	var wait sync.WaitGroup
	errorsOut := make(chan error, 2)
	for _, task := range []func(context.Context) error{advertise, lookup} {
		if task == nil {
			errorsOut <- errors.New("nil discovery task")
			continue
		}
		wait.Add(1)
		go func(task func(context.Context) error) {
			defer wait.Done()
			errorsOut <- task(ctx)
		}(task)
	}
	wait.Wait()
	close(errorsOut)
	var result error
	for err := range errorsOut {
		result = errors.Join(result, err)
	}
	return result
}

func selectDiscoveryCandidates(self peer.ID, providers, connected []peer.ID, rotation int) ([]peer.ID, int) {
	selected := make([]peer.ID, 0, discoveryProviderLimit+discoveryConnectedLimit)
	seen := make(map[peer.ID]struct{}, cap(selected))
	appendCandidate := func(id peer.ID) bool {
		if !validPeerID(id) || id == self {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
		selected = append(selected, id)
		return true
	}
	providerCount := 0
	for _, id := range providers {
		if appendCandidate(id) {
			providerCount++
			if providerCount == discoveryProviderLimit {
				break
			}
		}
	}

	uniqueConnected := make([]peer.ID, 0, len(connected))
	connectedSeen := make(map[peer.ID]struct{}, len(connected))
	for _, id := range connected {
		if !validPeerID(id) || id == self {
			continue
		}
		if _, exists := connectedSeen[id]; exists {
			continue
		}
		connectedSeen[id] = struct{}{}
		uniqueConnected = append(uniqueConnected, id)
	}
	if len(uniqueConnected) == 0 {
		return selected, 0
	}
	if rotation < 0 {
		rotation = 0
	}
	rotation %= len(uniqueConnected)
	connectedCount := min(discoveryConnectedLimit, len(uniqueConnected))
	for offset := range connectedCount {
		id := uniqueConnected[(rotation+offset)%len(uniqueConnected)]
		appendCandidate(id)
	}
	nextRotation := (rotation + connectedCount) % len(uniqueConnected)
	return selected, nextRotation
}

func runDiscoveryProbes(ctx context.Context, candidates []peer.ID, probe func(context.Context, peer.ID) error) error {
	if ctx == nil {
		return errors.New("nil discovery probe context")
	}
	if probe == nil {
		return errors.New("nil discovery probe")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	workerCount := min(discoveryProbeConcurrency, len(candidates))
	if workerCount == 0 {
		return nil
	}
	jobs := make(chan peer.ID, discoveryProbeConcurrency)
	var wait sync.WaitGroup
	wait.Add(workerCount)
	for range workerCount {
		go func() {
			defer wait.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case id, ok := <-jobs:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}
					_ = probe(ctx, id)
				}
			}
		}()
	}

sendLoop:
	for _, id := range candidates {
		select {
		case jobs <- id:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wait.Wait()
	return ctx.Err()
}

type confirmedPeers struct {
	self peer.ID
	mu   sync.Mutex
	seen map[peer.ID]time.Time
	// connections is the last live connection set observed for each peer.
	// A confirmation remains valid only while successive sets overlap.
	connections map[peer.ID]map[string]lp2pnet.Conn
}

func newConfirmedPeers(self peer.ID) *confirmedPeers {
	return &confirmedPeers{
		self:        self,
		seen:        make(map[peer.ID]time.Time),
		connections: make(map[peer.ID]map[string]lp2pnet.Conn),
	}
}

func (peers *confirmedPeers) confirm(id peer.ID, at time.Time) {
	if peers == nil || !validPeerID(id) || id == peers.self {
		return
	}
	peers.mu.Lock()
	defer peers.mu.Unlock()
	peers.confirmLocked(id, at)
}

func (peers *confirmedPeers) confirmLocked(id peer.ID, at time.Time) {
	if _, exists := peers.seen[id]; !exists && len(peers.seen) >= discoveryConfirmedCapacity {
		var oldest peer.ID
		var oldestAt time.Time
		for candidate, confirmedAt := range peers.seen {
			if oldest == "" || confirmedAt.Before(oldestAt) || (confirmedAt.Equal(oldestAt) && candidate < oldest) {
				oldest = candidate
				oldestAt = confirmedAt
			}
		}
		delete(peers.seen, oldest)
		delete(peers.connections, oldest)
	}
	peers.seen[id] = at
}

func (peers *confirmedPeers) confirmConnection(connection lp2pnet.Conn, network lp2pnet.Network, at time.Time) {
	if peers == nil || connection == nil || network == nil {
		return
	}
	id := connection.RemotePeer()
	if !validPeerID(id) || id == peers.self {
		return
	}
	peers.mu.Lock()
	defer peers.mu.Unlock()
	// The read-only network snapshot and the commit share this critical section
	// with lifecycle reconciliation. No stream or connection is closed here.
	peers.reconcileLocked(id, network.ConnsToPeer(id))
	connections := peers.connections[id]
	if _, currentConnection := connections[connection.ID()]; connection.IsClosed() || !currentConnection {
		return
	}
	peers.confirmLocked(id, at)
}

func (peers *confirmedPeers) reconcileConnections(id peer.ID, network lp2pnet.Network) {
	if peers == nil || network == nil || !validPeerID(id) || id == peers.self {
		return
	}
	peers.mu.Lock()
	peers.reconcileLocked(id, network.ConnsToPeer(id))
	peers.mu.Unlock()
}

func (peers *confirmedPeers) reconcileLocked(id peer.ID, current []lp2pnet.Conn) {
	previous := peers.connections[id]
	next := make(map[string]lp2pnet.Conn, len(current))
	for _, connection := range current {
		if connection == nil || connection.RemotePeer() != id || connection.IsClosed() {
			continue
		}
		next[connection.ID()] = connection
	}
	continuous := false
	for connectionID := range previous {
		if _, ok := next[connectionID]; ok {
			continuous = true
			break
		}
	}
	if len(previous) == 0 || !continuous {
		delete(peers.seen, id)
	}
	if len(next) == 0 {
		delete(peers.connections, id)
		return
	}
	peers.connections[id] = next
}

func (peers *confirmedPeers) containsConnected(id peer.ID, network lp2pnet.Network) bool {
	if peers == nil || network == nil || !validPeerID(id) || id == peers.self {
		return false
	}
	peers.mu.Lock()
	defer peers.mu.Unlock()
	peers.reconcileLocked(id, network.ConnsToPeer(id))
	_, exists := peers.seen[id]
	return exists
}

func (peers *confirmedPeers) forget(id peer.ID) {
	if peers == nil {
		return
	}
	peers.mu.Lock()
	delete(peers.seen, id)
	delete(peers.connections, id)
	peers.mu.Unlock()
}

func (peers *confirmedPeers) snapshot(connected func(peer.ID) bool) []peer.ID {
	if peers == nil {
		return []peer.ID{}
	}
	peers.mu.Lock()
	result := make([]peer.ID, 0, len(peers.seen))
	for id := range peers.seen {
		if connected == nil || connected(id) {
			result = append(result, id)
		}
	}
	peers.mu.Unlock()
	slices.Sort(result)
	return result
}

func (peers *confirmedPeers) connectedSnapshot(network lp2pnet.Network) []peer.ID {
	if peers == nil || network == nil {
		return []peer.ID{}
	}
	peers.mu.Lock()
	byPeer := make(map[peer.ID][]lp2pnet.Conn)
	for _, connection := range network.Conns() {
		if connection == nil {
			continue
		}
		id := connection.RemotePeer()
		byPeer[id] = append(byPeer[id], connection)
	}
	ids := make(map[peer.ID]struct{}, len(peers.seen)+len(peers.connections)+len(byPeer))
	for id := range peers.seen {
		ids[id] = struct{}{}
	}
	for id := range peers.connections {
		ids[id] = struct{}{}
	}
	for id := range byPeer {
		ids[id] = struct{}{}
	}
	for id := range ids {
		peers.reconcileLocked(id, byPeer[id])
	}
	result := make([]peer.ID, 0, len(peers.seen))
	for id := range peers.seen {
		result = append(result, id)
	}
	peers.mu.Unlock()
	slices.Sort(result)
	return result
}

func validPeerID(id peer.ID) bool {
	if id == "" {
		return false
	}
	decoded, err := peer.IDFromBytes([]byte(id))
	return err == nil && decoded == id
}
