package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/publiccontent"
	"github.com/larslarsen/bb-go/modern/social"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	richDaemonChildEnv   = "BITBOOK_M1B_CHILD"
	richDaemonDataDirEnv = "BITBOOK_M1B_DATA_DIR"
	richDaemonAPIEnv     = "BITBOOK_M1B_API"
)

func TestMEDIA001M1BDaemonRecoversBeforeReadiness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linked daemon local access is Linux-only")
	}
	dataDir := t.TempDir()
	seedDurableRichPost(t, dataDir)
	apiAddr := reserveRichDaemonAddr(t)
	child := startRichDaemonChild(t, dataDir, apiAddr)
	defer child.reap(syscall.SIGTERM)
	descriptor := waitLocalClientDescriptor(t, dataDir, localClientReadyWait)
	if descriptor.Endpoint == "" {
		t.Fatal("local readiness descriptor missing")
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	var response *http.Response
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, _ = client.Get("http://" + apiAddr + "/ob/posts")
		if response != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if response == nil {
		t.Fatal("daemon API did not become reachable")
	}
	defer response.Body.Close()
	var posts []map[string]any
	if err := json.NewDecoder(response.Body).Decode(&posts); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(posts) != 1 || posts[0]["id"] != strings.Repeat("3", 32) || posts[0]["schema"] != "bitbook.public-post/1" {
		t.Fatalf("recovered API status=%d posts=%+v", response.StatusCode, posts)
	}
	if posts[0]["hash"] == "" {
		t.Fatal("recovered API post lacks hash")
	}
}

func TestMEDIA001M1BCorruptRecoveryPreventsReadiness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linked daemon local access is Linux-only")
	}
	dataDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	node, err := network.Open(ctx, dataDir, network.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, BootstrapPeers: []peer.AddrInfo{}, DHTMode: dht.ModeServer, AllowPrivateAddresses: true})
	if err != nil {
		t.Fatal(err)
	}
	key := datastore.NewKey("/bitbook/social/rich-post/v1/record/" + strings.Repeat("4", 32))
	if err := node.Datastore.Put(ctx, key, []byte(`{"version":1,"state":"live","damaged":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := node.Datastore.Sync(ctx, datastore.NewKey("/bitbook/social/rich-post/v1/record")); err != nil {
		t.Fatal(err)
	}
	if err := node.Close(); err != nil {
		t.Fatal(err)
	}
	child := startRichDaemonChild(t, dataDir, reserveRichDaemonAddr(t))
	defer child.reap(syscall.SIGKILL)
	if err := child.waitProcess(5 * time.Second); err != nil {
		t.Fatal("corrupt daemon did not exit")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, localClientDirName, localClientDescriptorName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt recovery published readiness descriptor: %v", err)
	}
}

func TestMEDIA001M1BPrivateClaimStartupLogs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linked daemon local access is Linux-only")
	}
	dataDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	node, err := network.Open(ctx, dataDir, network.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, BootstrapPeers: []peer.AddrInfo{}, DHTMode: dht.ModeServer, AllowPrivateAddresses: true})
	if err != nil {
		t.Fatal(err)
	}
	privateID := strings.Repeat("cd", 16)
	key := datastore.NewKey("/bitbook/attachment/public/v1/reference/" + privateID)
	if err := node.Datastore.Put(ctx, key, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := node.Datastore.Sync(ctx, datastore.NewKey("/bitbook/attachment/public/v1/reference")); err != nil {
		t.Fatal(err)
	}
	if err := node.Close(); err != nil {
		t.Fatal(err)
	}
	child := startRichDaemonChild(t, dataDir, reserveRichDaemonAddr(t))
	defer child.reap(syscall.SIGKILL)
	if err := child.waitProcess(5 * time.Second); err != nil {
		t.Fatal("corrupt attachment daemon did not exit")
	}
	child.closePipes()
	if err := child.waitReaders(); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := child.stdout.String(), child.stderr.String()
	if !strings.Contains(stdout+stderr, "opening attachment store") {
		t.Fatalf("startup failure category absent; stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, privateID) || strings.Contains(stderr, privateID) {
		t.Fatalf("startup logs disclosed private claim; stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Lstat(filepath.Join(dataDir, localClientDirName, localClientDescriptorName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt attachment published readiness: %v", err)
	}
}

func TestMEDIA001M1BDaemonChild(t *testing.T) {
	if os.Getenv(richDaemonChildEnv) != "1" {
		return
	}
	dataDir, apiAddr := os.Getenv(richDaemonDataDirEnv), os.Getenv(richDaemonAPIEnv)
	if dataDir == "" || apiAddr == "" {
		t.Fatal("M1B child configuration missing")
	}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{os.Args[0], "-data-dir", dataDir, "-listen", "/ip4/127.0.0.1/tcp/0", "-api", apiAddr, "-allow-private", "-no-bootstrap"}
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func seedDurableRichPost(t *testing.T, dataDir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	node, err := network.Open(ctx, dataDir, network.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, BootstrapPeers: []peer.AddrInfo{}, DHTMode: dht.ModeServer, AllowPrivateAddresses: true})
	if err != nil {
		t.Fatal(err)
	}
	attachments, err := attachment.Open(ctx, node.Node, attachment.Limits{MaxReferences: 4096, MaxLogicalBytes: 4 << 30})
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	store, err := social.NewStoreWithAttachments(ctx, node.Node, attachments, social.RichPostLimits{MaxRecords: 4096, MaxBytes: 64 << 20})
	if err != nil {
		attachments.Close()
		node.Close()
		t.Fatal(err)
	}
	content, err := publiccontent.Marshal(publiccontent.Content{
		Schema:      publiccontent.SchemaV1,
		Body:        []publiccontent.Paragraph{{Type: "paragraph", Children: []publiccontent.InlineNode{{Type: "text", Text: "daemon recovered"}}}},
		Attachments: []publiccontent.AttachmentDescriptor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddRichPost(ctx, strings.Repeat("3", 32), content, map[string]attachment.ReferenceID{}); err != nil {
		t.Fatal(err)
	}
	if err := attachments.Close(); err != nil {
		t.Fatal(err)
	}
	if err := node.Close(); err != nil {
		t.Fatal(err)
	}
}

func startRichDaemonChild(t *testing.T, dataDir, apiAddr string) *paymentDaemonChild {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestMEDIA001M1BDaemonChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), richDaemonChildEnv+"=1", richDaemonDataDirEnv+"="+dataDir, richDaemonAPIEnv+"="+apiAddr)
	return startCapturedDaemonCommand(t, cmd)
}

func startCapturedDaemonCommand(t *testing.T, cmd *exec.Cmd) *paymentDaemonChild {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	child := &paymentDaemonChild{cmd: cmd, stdout: newBoundBuffer(), stderr: newBoundBuffer(), stdoutR: stdout, stderrR: stderr, waitDone: make(chan struct{})}
	child.readers.Add(2)
	go drainPipe(&child.readers, child.stdout, stdout)
	go drainPipe(&child.readers, child.stderr, stderr)
	go func() { child.waitErr = cmd.Wait(); close(child.waitDone) }()
	return child
}

func reserveRichDaemonAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}
