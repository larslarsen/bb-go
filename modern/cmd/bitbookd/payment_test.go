package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/payment"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	paymentDaemonChildEnv    = "BITBOOK_PAY002_DAEMON_CHILD"
	paymentDaemonDataDirEnv  = "BITBOOK_PAY002_DATA_DIR"
	paymentDaemonChildFilter = "^TestPaymentDaemonChild$"
	maxDaemonLogBytes        = 256 << 10
	daemonReadyTimeout       = 20 * time.Second
	daemonOpTimeout          = 15 * time.Second
	daemonStopTimeout        = 15 * time.Second
	daemonKillTimeout        = 5 * time.Second
	daemonReaderTimeout      = 2 * time.Second
)

func TestPaymentDaemonReceivesAndPersistsRequests(t *testing.T) {
	linked := startLinkedPaymentDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel()

	signed := signPayeeRequest(t, linked.payee, daemonPaymentRequest(t, linked.payee.ID(), linked.daemonID, time.Now()))
	digest := independentRequestDigest(t, signed)
	payload := marshalSignedObject(t, signed)

	first := sendPaymentBytes(t, ctx, linked.payee, linked.daemonID, payload)
	requireAcceptedDigest(t, first, digest)
	second := sendPaymentBytes(t, ctx, linked.payee, linked.daemonID, payload)
	requireAcceptedDigest(t, second, digest)

	linked.stop(t)
	linked.startChild(t)
	linked.stop(t)
	requireSingleInboundRequest(t, inspectDaemonPayments(t, linked.dataDir), signed, digest)

	linked.startChild(t)
	ctx2, cancel2 := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel2()
	third := sendPaymentBytes(t, ctx2, linked.payee, linked.daemonID, payload)
	requireAcceptedDigest(t, third, digest)
	linked.stop(t)
	requireSingleInboundRequest(t, inspectDaemonPayments(t, linked.dataDir), signed, digest)
}

func TestPaymentDaemonRejectsWrongPayer(t *testing.T) {
	linked := startLinkedPaymentDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel()

	valid := signPayeeRequest(t, linked.payee, daemonPaymentRequest(t, linked.payee.ID(), linked.daemonID, time.Now()))
	validDigest := independentRequestDigest(t, valid)
	validAck := sendPaymentBytes(t, ctx, linked.payee, linked.daemonID, marshalSignedObject(t, valid))
	requireAcceptedDigest(t, validAck, validDigest)

	wrongPayer := thirdLocalPeerID(t)
	invalidReq := daemonPaymentRequest(t, linked.payee.ID(), wrongPayer, time.Now())
	invalidSigned := signPayeeRequest(t, linked.payee, invalidReq)
	invalidCtx, invalidCancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer invalidCancel()
	invalidAck, err := writePaymentBytes(invalidCtx, linked.payee, linked.daemonID, marshalSignedObject(t, invalidSigned))
	if err != nil {
		t.Fatalf("wrong-payer delivery disconnected or timed out instead of a PAYER rejection: %v", err)
	}
	if invalidAck.Accepted || invalidAck.Code != payment.CodePayer {
		t.Fatalf("wrong-payer acknowledgement accepted=%t code=%q, want rejection %s", invalidAck.Accepted, invalidAck.Code, payment.CodePayer)
	}

	linked.stop(t)
	records := inspectDaemonPayments(t, linked.dataDir)
	requireSingleInboundRequest(t, records, valid, validDigest)
	invalidDigest := independentRequestDigest(t, invalidSigned)
	for _, record := range records {
		if record.Digest == invalidDigest {
			t.Fatal("wrong-payer request was retained")
		}
	}
}

func TestParseDaemonIdentity(t *testing.T) {
	id := thirdLocalPeerID(t)
	other := thirdLocalPeerID(t)
	loopback := "/ip4/127.0.0.1/tcp/43210/p2p/" + id.String()
	cases := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name: "timestampedStdFlags",
			output: "2026/09/14 15:42:02 peer ID: " + id.String() + "\n" +
				"2026/09/14 15:42:02 p2p: " + loopback + "\n",
		},
		{
			name: "timestampedMicroseconds",
			output: "2026/09/14 15:42:02.123456 peer ID: " + id.String() + "\n" +
				"2026/09/14 15:42:02.123456 p2p: " + loopback + "\n",
		},
		{
			name: "splashThenTimestamped",
			output: "BitBook Server v0.2.0-dev\n" +
				"2026/09/14 15:42:02 peer ID: " + id.String() + "\n" +
				"2026/09/14 15:42:02 p2p: /ip4/10.0.0.1/tcp/4001/p2p/" + id.String() + "\n" +
				"2026/09/14 15:42:02 p2p: " + loopback + "\n",
		},
		{
			name:    "missingPeerID",
			output:  "2026/09/14 15:42:02 p2p: " + loopback + "\n",
			wantErr: true,
		},
		{
			name:    "missingLoopback",
			output:  "2026/09/14 15:42:02 peer ID: " + id.String() + "\n",
			wantErr: true,
		},
		{
			name: "tcpZero",
			output: "2026/09/14 15:42:02 peer ID: " + id.String() + "\n" +
				"2026/09/14 15:42:02 p2p: /ip4/127.0.0.1/tcp/0/p2p/" + id.String() + "\n",
			wantErr: true,
		},
		{
			name: "mismatchedPeerID",
			output: "2026/09/14 15:42:02 peer ID: " + id.String() + "\n" +
				"2026/09/14 15:42:02 p2p: /ip4/127.0.0.1/tcp/43210/p2p/" + other.String() + "\n",
			wantErr: true,
		},
		{
			name:    "malformedPeerID",
			output:  "2026/09/14 15:42:02 peer ID: not-a-peer\n2026/09/14 15:42:02 p2p: " + loopback + "\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotAddr, err := parseDaemonIdentity(tc.output)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected parse error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotID != id {
				t.Fatalf("peer ID mismatch")
			}
			if !isNumericLoopbackP2P(gotAddr) {
				t.Fatal("parsed address is not a numeric loopback multiaddress")
			}
			info, err := peer.AddrInfoFromP2pAddr(gotAddr)
			if err != nil || info.ID != id {
				t.Fatal("parsed loopback address does not match peer ID")
			}
		})
	}
}

// TestPaymentDaemonChild re-execs the test binary into the production run()
// lifecycle. Without the child marker it returns immediately and must not
// construct a payment service or install a stream handler.
func TestPaymentDaemonChild(t *testing.T) {
	if os.Getenv(paymentDaemonChildEnv) != "1" {
		return
	}
	dataDir := os.Getenv(paymentDaemonDataDirEnv)
	if dataDir == "" {
		t.Fatal("child daemon is missing its owned data directory")
	}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(os.Stderr)
	os.Args = []string{
		os.Args[0],
		"-data-dir", dataDir,
		"-listen", "/ip4/127.0.0.1/tcp/0",
		"-api", "127.0.0.1:0",
		"-allow-private",
		"-no-bootstrap",
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

type linkedPaymentDaemon struct {
	t        *testing.T
	dataDir  string
	child    *paymentDaemonChild
	payee    *network.Node
	daemonID peer.ID
}

func startLinkedPaymentDaemon(t *testing.T) *linkedPaymentDaemon {
	t.Helper()
	linked := &linkedPaymentDaemon{t: t, dataDir: t.TempDir()}
	t.Cleanup(func() {
		if linked.payee != nil {
			if err := linked.payee.Close(); err != nil {
				t.Errorf("closing payee node: %v", err)
			}
			linked.payee = nil
		}
		if linked.child != nil {
			if err := linked.child.reap(syscall.SIGTERM); err != nil {
				t.Errorf("reaping daemon child: %v", err)
			}
		}
	})
	linked.payee = newLocalPayee(t)
	linked.startChild(t)
	return linked
}

func (l *linkedPaymentDaemon) startChild(t *testing.T) {
	t.Helper()
	if l.child != nil {
		t.Fatal("previous daemon child is still owned; reap before restart")
	}
	child := startPaymentDaemon(t, l.dataDir)
	l.child = child
	daemonID, addr := waitPaymentDaemonReady(t, child, daemonReadyTimeout)
	if l.daemonID != "" && l.daemonID != daemonID {
		t.Fatalf("restarted daemon peer ID changed")
	}
	l.daemonID = daemonID
	ctx, cancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel()
	connectAndAssertReady(t, ctx, l.payee, daemonID, addr)
}

func (l *linkedPaymentDaemon) stop(t *testing.T) {
	t.Helper()
	if l.child == nil {
		t.Fatal("no daemon child to reap")
	}
	if err := l.child.reap(syscall.SIGTERM); err != nil {
		t.Fatalf("daemon child did not exit normally: %v", err)
	}
	l.child = nil
}

type paymentDaemonChild struct {
	cmd      *exec.Cmd
	stdout   *boundBuffer
	stderr   *boundBuffer
	stdoutR  io.ReadCloser
	stderrR  io.ReadCloser
	readers  sync.WaitGroup
	waitDone chan struct{}
	waitErr  error
	reapOnce sync.Once
	reapErr  error
}

func startPaymentDaemon(t *testing.T, dataDir string) *paymentDaemonChild {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run="+paymentDaemonChildFilter, "-test.count=1")
	cmd.Env = childEnv(dataDir)
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
	return child
}

func (c *paymentDaemonChild) reap(signal os.Signal) error {
	c.reapOnce.Do(func() {
		c.reapErr = c.shutdown(signal)
	})
	return c.reapErr
}

func (c *paymentDaemonChild) shutdown(signal os.Signal) error {
	if c.cmd == nil || c.cmd.Process == nil || c.waitDone == nil {
		c.closePipes()
		_ = c.waitReaders()
		return errors.New("daemon child has no process")
	}
	if err := c.cmd.Process.Signal(signal); err != nil {
		select {
		case <-c.waitDone:
		default:
			c.closePipes()
			_ = c.waitReaders()
			return err
		}
	}
	if err := c.waitProcess(daemonStopTimeout); err != nil {
		_ = c.cmd.Process.Kill()
		if waitErr := c.waitProcess(daemonKillTimeout); waitErr != nil {
			c.closePipes()
			_ = c.waitReaders()
			return errors.New("daemon child did not exit after SIGTERM or kill")
		}
		c.closePipes()
		if readerErr := c.waitReaders(); readerErr != nil {
			return errors.Join(errors.New("daemon child required kill"), readerErr)
		}
		return errors.New("daemon child required kill")
	}
	c.closePipes()
	if err := c.waitReaders(); err != nil {
		return err
	}
	return c.waitErr
}

func (c *paymentDaemonChild) waitProcess(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-c.waitDone:
		return nil
	case <-timer.C:
		return errors.New("process wait timeout")
	}
}

func (c *paymentDaemonChild) waitReaders() error {
	done := make(chan struct{})
	go func() {
		c.readers.Wait()
		close(done)
	}()
	timer := time.NewTimer(daemonReaderTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("daemon log readers did not finish")
	}
}

func (c *paymentDaemonChild) closePipes() {
	if c.stdoutR != nil {
		_ = c.stdoutR.Close()
		c.stdoutR = nil
	}
	if c.stderrR != nil {
		_ = c.stderrR.Close()
		c.stderrR = nil
	}
}

func (c *paymentDaemonChild) exited() bool {
	if c.waitDone == nil {
		return false
	}
	select {
	case <-c.waitDone:
		return true
	default:
		return false
	}
}

func childEnv(dataDir string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, paymentDaemonChildEnv+"=") || strings.HasPrefix(item, paymentDaemonDataDirEnv+"=") {
			continue
		}
		env = append(env, item)
	}
	return append(env, paymentDaemonChildEnv+"=1", paymentDaemonDataDirEnv+"="+dataDir)
}

func waitPaymentDaemonReady(t *testing.T, child *paymentDaemonChild, timeout time.Duration) (peer.ID, ma.Multiaddr) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if child.exited() {
			t.Fatalf("daemon exited before readiness: %v", child.waitErr)
		}
		id, addr, err := parseDaemonIdentity(child.stdout.String() + "\n" + child.stderr.String())
		if err == nil {
			return id, addr
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	if child.exited() {
		t.Fatalf("daemon exited before readiness: %v", child.waitErr)
	}
	t.Fatalf("daemon readiness timeout (startup, not payment negotiation): %v", last)
	return "", nil
}

func parseDaemonIdentity(output string) (peer.ID, ma.Multiaddr, error) {
	var id peer.ID
	var loopback ma.Multiaddr
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := logValue(line, "peer ID: "); ok {
			parsed, err := peer.Decode(strings.TrimSpace(rest))
			if err != nil {
				return "", nil, err
			}
			id = parsed
		}
		if rest, ok := logValue(line, "p2p: "); ok {
			addr, err := ma.NewMultiaddr(strings.TrimSpace(rest))
			if err != nil {
				continue
			}
			if isNumericLoopbackP2P(addr) {
				loopback = addr
			}
		}
	}
	if id == "" {
		return "", nil, errors.New("missing peer ID")
	}
	if loopback == nil {
		return "", nil, errors.New("missing numeric loopback multiaddress")
	}
	info, err := peer.AddrInfoFromP2pAddr(loopback)
	if err != nil {
		return "", nil, err
	}
	if info.ID != id {
		return "", nil, errors.New("loopback multiaddress does not match peer ID")
	}
	return id, loopback, nil
}

func logValue(line, key string) (string, bool) {
	index := strings.Index(line, key)
	if index < 0 {
		return "", false
	}
	value := strings.TrimSpace(line[index+len(key):])
	if value == "" {
		return "", false
	}
	return value, true
}

func isNumericLoopbackP2P(addr ma.Multiaddr) bool {
	ip, err := addr.ValueForProtocol(ma.P_IP4)
	if err != nil || ip != "127.0.0.1" {
		return false
	}
	portText, err := addr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return false
	}
	_, err = addr.ValueForProtocol(ma.P_P2P)
	return err == nil
}

func newLocalPayee(t *testing.T) *network.Node {
	t.Helper()
	node, err := network.New(context.Background(), network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func connectAndAssertReady(t *testing.T, ctx context.Context, payee *network.Node, daemonID peer.ID, addr ma.Multiaddr) {
	t.Helper()
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := payee.Connect(ctx, *info); err != nil {
		t.Fatalf("daemon readiness: connect failed: %v", err)
	}
	stream, err := payee.Host.NewStream(ctx, daemonID, network.DirectProtocolCurrent)
	if err != nil {
		t.Fatalf("daemon readiness: direct protocol was not negotiated: %v", err)
	}
	_ = stream.Close()
}

func daemonPaymentRequest(t *testing.T, payee, payer peer.ID, now time.Time) payment.PaymentRequestV1 {
	t.Helper()
	created := now.UTC().Truncate(time.Second)
	return payment.PaymentRequestV1{
		V:            1,
		RequestID:    freshHex32(t),
		PayerPeerID:  payer.String(),
		PayeePeerID:  payee.String(),
		Asset:        "ZEC",
		Network:      "zec-testnet",
		AmountAtomic: "100000000",
		Receiver:     "u1testreceiver",
		ReceiverKind: "zec-ua-orchard-protocol",
		Memo:         "coffee",
		Nonce:        freshHex32(t),
		CreatedAt:    created.Format("2006-01-02T15:04:05Z"),
		ExpiresAt:    created.Add(time.Hour).Format("2006-01-02T15:04:05Z"),
	}
}

func signPayeeRequest(t *testing.T, payee *network.Node, request payment.PaymentRequestV1) payment.SignedObject {
	t.Helper()
	signed, err := payment.SignRequest(payee.PrivateKey, request)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func independentRequestDigest(t *testing.T, signed payment.SignedObject) string {
	t.Helper()
	_, _, digest, err := payment.DecodePaymentRequest([]byte(signed.Canonical))
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func marshalSignedObject(t *testing.T, signed payment.SignedObject) []byte {
	t.Helper()
	payload, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func sendPaymentBytes(t *testing.T, ctx context.Context, payee *network.Node, dest peer.ID, payload []byte) payment.Acknowledgement {
	t.Helper()
	ack, err := writePaymentBytes(ctx, payee, dest, payload)
	if err != nil {
		t.Fatalf("payment protocol negotiation or delivery failed: %v", err)
	}
	return ack
}

func writePaymentBytes(ctx context.Context, payee *network.Node, dest peer.ID, payload []byte) (payment.Acknowledgement, error) {
	stream, err := payee.Host.NewStream(ctx, dest, network.PaymentProtocolCurrent)
	if err != nil {
		return payment.Acknowledgement{}, err
	}
	defer stream.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetDeadline(deadline)
	}
	if err := payment.WriteFrame(stream, payload); err != nil {
		_ = stream.Reset()
		return payment.Acknowledgement{}, err
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return payment.Acknowledgement{}, err
	}
	raw, err := payment.ReadFrame(stream)
	if err != nil {
		_ = stream.Reset()
		return payment.Acknowledgement{}, err
	}
	var ack payment.Acknowledgement
	if err := json.Unmarshal(raw, &ack); err != nil {
		return payment.Acknowledgement{}, err
	}
	return ack, nil
}

func requireAcceptedDigest(t *testing.T, ack payment.Acknowledgement, digest string) {
	t.Helper()
	if !ack.Accepted || ack.Code != "" || ack.Digest != digest {
		t.Fatalf("acknowledgement accepted=%t code=%q digest_match=%t, want accepted digest", ack.Accepted, ack.Code, ack.Digest == digest)
	}
}

func inspectDaemonPayments(t *testing.T, dataDir string) []payment.RecordedObject {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel()
	node, err := network.Open(ctx, dataDir, network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := node.Close(); err != nil {
			t.Errorf("closing inspection node: %v", err)
		}
	}()
	svc, err := payment.NewService(node.Node)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	records, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func requireSingleInboundRequest(t *testing.T, records []payment.RecordedObject, signed payment.SignedObject, digest string) {
	t.Helper()
	found := 0
	for _, record := range records {
		if record.Signed.Kind != payment.KindRequest {
			t.Fatalf("unexpected stored kind %q", record.Signed.Kind)
		}
		found++
		if record.Direction != payment.DirectionInbound || record.Digest != digest {
			t.Fatalf("stored request direction=%q digest_match=%t, want inbound matching digest", record.Direction, record.Digest == digest)
		}
		if record.Signed.Canonical != signed.Canonical ||
			!bytes.Equal(record.Signed.PublicKey, signed.PublicKey) ||
			!bytes.Equal(record.Signed.Signature, signed.Signature) {
			t.Fatal("stored request canonical bytes, public key, or signature do not match the sent object")
		}
	}
	if found != 1 {
		t.Fatalf("stored request records = %d, want 1", found)
	}
}

func thirdLocalPeerID(t *testing.T) peer.ID {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func freshHex32(t *testing.T) string {
	t.Helper()
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw[:])
}

type boundBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newBoundBuffer() *boundBuffer { return &boundBuffer{} }

func (b *boundBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remain := maxDaemonLogBytes - b.buf.Len()
	if remain > 0 {
		if len(p) > remain {
			_, _ = b.buf.Write(p[:remain])
		} else {
			_, _ = b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *boundBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func drainPipe(wg *sync.WaitGroup, dst *boundBuffer, src io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
}
