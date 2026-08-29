package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/larslarsen/bb-go/modern/api"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var version = "0.2.0-dev"

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.SetOutput(os.Stdout)
	defaultDataDir, err := dataDirectory()
	if err != nil {
		return err
	}
	var listenAddrs stringList
	var bootstrapAddrs stringList
	dataDir := flag.String("data-dir", defaultDataDir, "identity and social data directory")
	apiAddr := flag.String("api", "127.0.0.1:4002", "HTTP API listen address")
	allowPrivate := flag.Bool("allow-private", false, "allow private and loopback peers in the DHT")
	dhtServer := flag.Bool("dht-server", false, "run the DHT in server mode")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Var(&listenAddrs, "listen", "libp2p multiaddress; may be repeated")
	flag.Var(&bootstrapAddrs, "bootstrap", "bootstrap peer multiaddress ending in /p2p/<peerID>; may be repeated")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}
	bootstrapPeers, err := parseBootstrapPeers(bootstrapAddrs)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mode := dht.ModeAuto
	if *dhtServer {
		mode = dht.ModeServer
	}
	node, err := network.Open(ctx, *dataDir, network.Config{
		ListenAddrs:           listenAddrs,
		BootstrapPeers:        bootstrapPeers,
		DHTMode:               mode,
		AllowPrivateAddresses: *allowPrivate,
	})
	if err != nil {
		return err
	}
	defer node.Close()

	store, err := social.NewStore(node.Node)
	if err != nil {
		return err
	}
	handler, err := api.NewHandler(node.Node, store)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *apiAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	printSplash()
	log.Printf("peer ID: %s", node.ID())
	for _, addr := range node.Addrs() {
		log.Printf("p2p: %s", addr)
	}
	log.Printf("social API: http://%s/ob/config", *apiAddr)
	if len(bootstrapPeers) == 0 {
		log.Print("no bootstrap peers configured; this node will save locally until a peer is supplied")
	}

	go republish(ctx, node.Node, store)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func republish(ctx context.Context, node *network.Node, store *social.Store) {
	publish := func() {
		if len(node.Host.Network().Peers()) == 0 {
			_, _ = store.Commit(ctx)
			return
		}
		publishCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if root, err := store.Publish(publishCtx); err != nil {
			log.Printf("social root publication failed: %v", err)
		} else {
			log.Printf("published social root %s", root)
		}
	}
	publish()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publish()
		}
	}
}

func parseBootstrapPeers(encoded []string) ([]peer.AddrInfo, error) {
	result := make([]peer.AddrInfo, 0, len(encoded))
	for _, value := range encoded {
		addr, err := ma.NewMultiaddr(value)
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap address %q: %w", value, err)
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, fmt.Errorf("bootstrap address %q must end in /p2p/<peerID>: %w", value, err)
		}
		result = append(result, *info)
	}
	return result, nil
}

func dataDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".bitbook", "modern"), nil
}

func printSplash() {
	fmt.Println(`__________.__  __ __________               __
\______   \__|/  |\______   \ ____   ____ |  | __
 |    |  _/  \   __\    |  _//  _ \ /  _ \|  |/ /
 |    |   \  ||  | |    |   (  <_> |  <_> )    <
 |______  /__||__| |______  /\____/ \____/|__|_ \
        \/                \/                   \/`)
	fmt.Printf("\nBitBook Server v%s\n[Press Ctrl+C to exit]\n", version)
}
