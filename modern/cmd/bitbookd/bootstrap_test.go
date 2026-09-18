package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	datastore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	bitbooknet "github.com/larslarsen/bb-go/modern/network"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	net001DaemonChildEnv    = "BITBOOK_NET001_DAEMON_CHILD"
	net001DaemonArgsEnv     = "BITBOOK_NET001_DAEMON_ARGS"
	net001DaemonDefaultsEnv = "BITBOOK_NET001_DEFAULT_BOOTSTRAP"
	net001DaemonChildTest   = "^TestNET001DaemonBootstrapChild$"
)

func TestNET001SelectBootstrapPeers(t *testing.T) {
	defaults := dht.GetDefaultBootstrapPeerAddrInfos()
	got, err := selectBootstrapPeers(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !equalAddrInfos(got, defaults) {
		t.Fatalf("default peers differ from pinned DHT defaults: got %v want %v", got, defaults)
	}
	if len(got) == 0 || len(got[0].Addrs) == 0 {
		t.Fatal("pinned default bootstrap list is empty")
	}
	originalFirst := peer.AddrInfo{ID: defaults[0].ID, Addrs: slices.Clone(defaults[0].Addrs)}
	replacementAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/41002")
	if err != nil {
		t.Fatal(err)
	}
	got[0].ID = thirdLocalPeerID(t)
	got[0].Addrs[0] = replacementAddr
	again, err := selectBootstrapPeers(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != originalFirst.ID || !slices.EqualFunc(again[0].Addrs, originalFirst.Addrs, func(left, right ma.Multiaddr) bool {
		return left.Equal(right)
	}) {
		t.Fatal("caller mutation of outer or inner address storage changed a later default selection")
	}

	overrideID := thirdLocalPeerID(t)
	override := "/ip4/127.0.0.1/tcp/41001/p2p/" + overrideID.String()
	overridden, err := selectBootstrapPeers([]string{override}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(overridden) != 1 || overridden[0].ID != overrideID || len(overridden[0].Addrs) != 1 {
		t.Fatalf("override selection = %v", overridden)
	}
	for _, defaultPeer := range defaults {
		if overridden[0].ID == defaultPeer.ID {
			t.Fatal("explicit override retained a default peer")
		}
	}
	disabled, err := selectBootstrapPeers(nil, true)
	if err != nil || len(disabled) != 0 {
		t.Fatalf("disabled bootstrap = %v, %v", disabled, err)
	}
	if _, err := selectBootstrapPeers([]string{override}, true); err == nil {
		t.Fatal("explicit and disabled bootstrap flags were accepted together")
	}
	for _, invalid := range []string{"not-a-multiaddr", "/ip4/127.0.0.1/tcp/41001"} {
		if _, err := selectBootstrapPeers([]string{invalid}, false); err == nil {
			t.Fatalf("invalid bootstrap address %q accepted", invalid)
		}
	}
}

func TestNET001DaemonBootstrapWiring(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	seedStore := dsync.MutexWrap(datastore.NewMapDatastore())
	seedHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer seedHost.Close()
	seedDHT, err := dht.New(seedHost,
		dht.Datastore(seedStore),
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(protocol.ID("/ipfs")),
		dht.BootstrapPeers(),
		dht.AddressFilter(nil),
		dht.QueryFilter(func(_ any, _ peer.AddrInfo) bool { return true }),
		dht.RoutingTableFilter(func(_ any, _ peer.ID) bool { return true }),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer seedDHT.Close()
	if err := seedDHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	seedAddr := daemonSeedAddress(t, seedHost)
	loopbackDefaults := []string{seedAddr}

	explicit := startNET001Daemon(t, []string{
		"-data-dir", t.TempDir(),
		"-listen", "/ip4/127.0.0.1/tcp/0",
		"-api", "127.0.0.1:0",
		"-allow-private",
		"-dht-server",
		"-bootstrap", seedAddr,
	}, loopbackDefaults, false)
	explicitID, _ := waitPaymentDaemonReady(t, explicit, daemonReadyTimeout)
	waitNET001Connection(t, ctx, seedHost, explicitID)
	waitNET001Advertisement(t, ctx, routingdiscovery.NewRoutingDiscovery(seedDHT), explicitID)
	if output := explicit.stdout.String() + explicit.stderr.String(); !strings.Contains(output, "bootstrap: explicit override (1 peer)") {
		t.Fatalf("explicit bootstrap selection was not logged: %s", output)
	}
	if err := explicit.reap(os.Interrupt); err != nil {
		t.Fatalf("stop explicit-bootstrap daemon: %v", err)
	}

	defaulted := startNET001Daemon(t, []string{
		"-data-dir", t.TempDir(),
		"-listen", "/ip4/127.0.0.1/tcp/0",
		"-api", "127.0.0.1:0",
		"-allow-private",
		"-dht-server",
	}, loopbackDefaults, false)
	defaultID, _ := waitPaymentDaemonReady(t, defaulted, daemonReadyTimeout)
	waitNET001Connection(t, ctx, seedHost, defaultID)
	if output := defaulted.stdout.String() + defaulted.stderr.String(); !strings.Contains(output, "bootstrap: default") {
		t.Fatalf("default bootstrap selection was not logged: %s", output)
	}
	if err := defaulted.reap(os.Interrupt); err != nil {
		t.Fatalf("stop default-bootstrap daemon: %v", err)
	}

	disabled := startNET001Daemon(t, []string{
		"-data-dir", t.TempDir(),
		"-listen", "/ip4/127.0.0.1/tcp/0",
		"-api", "127.0.0.1:0",
		"-allow-private",
		"-dht-server",
		"-no-bootstrap",
	}, loopbackDefaults, false)
	disabledID, _ := waitPaymentDaemonReady(t, disabled, daemonReadyTimeout)
	assertNET001NoConnection(t, ctx, seedHost, disabledID, 500*time.Millisecond)
	if output := disabled.stdout.String() + disabled.stderr.String(); !strings.Contains(output, "bootstrap: disabled") {
		t.Fatalf("disabled bootstrap selection was not logged: %s", output)
	}
	if err := disabled.reap(os.Interrupt); err != nil {
		t.Fatalf("stop no-bootstrap daemon: %v", err)
	}
}

func TestNET001DaemonBootstrapErrorsPrecedeDataDirectory(t *testing.T) {
	seedID := thirdLocalPeerID(t)
	seedAddr := "/ip4/127.0.0.1/tcp/41001/p2p/" + seedID.String()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "invalid", args: []string{"-bootstrap", "not-a-multiaddr"}},
		{name: "conflicting", args: []string{"-bootstrap", seedAddr, "-no-bootstrap"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "must-not-exist")
			args := append([]string{"-data-dir", dataDir, "-listen", "/ip4/127.0.0.1/tcp/0", "-api", "127.0.0.1:0"}, tc.args...)
			child := startNET001Daemon(t, args, []string{seedAddr}, true)
			waitNET001ChildFailure(t, child, 5*time.Second)
			if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
				t.Fatalf("invalid CLI created data directory before failing: %v", err)
			}
		})
	}
}

func TestNET001DaemonBootstrapChild(t *testing.T) {
	if os.Getenv(net001DaemonChildEnv) != "1" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv(net001DaemonArgsEnv)), &args); err != nil {
		t.Fatalf("decode child args: %v", err)
	}
	var defaults []string
	if err := json.Unmarshal([]byte(os.Getenv(net001DaemonDefaultsEnv)), &defaults); err != nil {
		t.Fatalf("decode child defaults: %v", err)
	}
	replacement := make([]ma.Multiaddr, len(defaults))
	for i, encoded := range defaults {
		addr, err := ma.NewMultiaddr(encoded)
		if err != nil {
			t.Fatalf("decode controlled default %q: %v", encoded, err)
		}
		replacement[i] = addr
	}
	dht.DefaultBootstrapPeers = replacement
	flagSetForChild(t, args)
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func flagSetForChild(t testing.TB, args []string) {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(os.Stderr)
	os.Args = append([]string{os.Args[0]}, args...)
}

func startNET001Daemon(t testing.TB, args, defaults []string, expectFailure bool) *paymentDaemonChild {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	encodedDefaults, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run="+net001DaemonChildTest, "-test.count=1")
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, net001DaemonChildEnv+"=") || strings.HasPrefix(item, net001DaemonArgsEnv+"=") || strings.HasPrefix(item, net001DaemonDefaultsEnv+"=") {
			continue
		}
		env = append(env, item)
	}
	cmd.Env = append(env,
		net001DaemonChildEnv+"=1",
		net001DaemonArgsEnv+"="+string(encodedArgs),
		net001DaemonDefaultsEnv+"="+string(encodedDefaults),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		t.Fatal(err)
	}
	child := &paymentDaemonChild{
		cmd:      cmd,
		stdout:   newBoundBuffer(),
		stderr:   newBoundBuffer(),
		stdoutR:  stdout,
		stderrR:  stderr,
		waitDone: make(chan struct{}),
	}
	child.readers.Add(2)
	go drainPipe(&child.readers, child.stdout, stdout)
	go drainPipe(&child.readers, child.stderr, stderr)
	go func() {
		child.waitErr = cmd.Wait()
		close(child.waitDone)
	}()
	t.Cleanup(func() {
		reapErr := child.reap(os.Interrupt)
		if expectFailure {
			if reapErr == nil {
				t.Errorf("daemon child unexpectedly succeeded")
			} else {
				var exitErr *exec.ExitError
				if !errors.As(reapErr, &exitErr) {
					t.Errorf("daemon child cleanup failed: %v", reapErr)
				}
			}
		} else if reapErr != nil {
			t.Errorf("daemon child cleanup: %v", reapErr)
		}
	})
	return child
}

func daemonSeedAddress(t testing.TB, seed host.Host) string {
	t.Helper()
	peerSuffix, err := ma.NewMultiaddr("/p2p/" + seed.ID().String())
	if err != nil {
		t.Fatal(err)
	}
	addrs := seed.Addrs()
	if len(addrs) == 0 {
		t.Fatal("independent seed has no listen address")
	}
	return addrs[0].Encapsulate(peerSuffix).String()
}

func waitNET001Connection(t testing.TB, ctx context.Context, h host.Host, id peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if h.Network().Connectedness(id) == lp2pnet.Connected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for daemon bootstrap connection: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertNET001NoConnection(t testing.TB, ctx context.Context, h host.Host, id peer.ID, observation time.Duration) {
	t.Helper()
	timer := time.NewTimer(observation)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if h.Network().Connectedness(id) == lp2pnet.Connected {
			t.Fatal("disabled daemon dialed the controlled default bootstrap peer")
		}
		select {
		case <-timer.C:
			return
		case <-ctx.Done():
			t.Fatalf("disabled bootstrap observation: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitNET001Advertisement(t testing.TB, ctx context.Context, discovery *routingdiscovery.RoutingDiscovery, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		findCtx, cancel := context.WithTimeout(ctx, time.Second)
		found, err := discovery.FindPeers(findCtx, bitbooknet.DiscoveryNamespace)
		if err == nil {
			for info := range found {
				if info.ID == want {
					cancel()
					return
				}
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for daemon discovery advertisement from %s: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitNET001ChildFailure(t testing.TB, child *paymentDaemonChild, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-child.waitDone:
	case <-timer.C:
		t.Fatal("invalid daemon CLI did not fail promptly")
	}
	if err := child.reap(os.Interrupt); err == nil {
		t.Fatalf("invalid daemon CLI succeeded: %s", child.stdout.String()+child.stderr.String())
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("invalid daemon CLI cleanup failed: %v", err)
		}
	}
}

func equalAddrInfos(left, right []peer.AddrInfo) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].ID != right[i].ID || !slices.EqualFunc(left[i].Addrs, right[i].Addrs, func(left, right ma.Multiaddr) bool {
			return left.Equal(right)
		}) {
			return false
		}
	}
	return true
}
