package payment

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/larslarsen/bb-go/modern/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func TestFrameSplitPrefixAndPayload(t *testing.T) {
	payload := []byte(`{"version":1,"kind":"request"}`)
	var framed bytes.Buffer
	if err := WriteFrame(&framed, payload); err != nil {
		t.Fatal(err)
	}
	raw := framed.Bytes()
	if len(raw) < 5 {
		t.Fatal("framed payload is missing the length prefix")
	}
	size := binary.BigEndian.Uint32(raw[:4])
	if int(size) != len(payload) || !bytes.Equal(raw[4:], payload) {
		t.Fatalf("prefix=%d payload=%q", size, raw[4:])
	}
	reader := &splitReader{raw: raw, cuts: []int{2, 4, 8, len(raw)}}
	got, err := ReadFrame(reader)
	if err != nil {
		t.Fatal(err)
	}
	if reader.step != len(reader.cuts) || reader.off != len(raw) {
		t.Fatalf("split boundaries exercised = %d/%d offset=%d/%d", reader.step, len(reader.cuts), reader.off, len(raw))
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("split read = %q, want %q", got, payload)
	}
}

func TestFrameByteAtATime(t *testing.T) {
	payload := []byte(`{"version":1,"kind":"status"}`)
	var framed bytes.Buffer
	if err := WriteFrame(&framed, payload); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&byteReader{raw: framed.Bytes()})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("byte-at-a-time = %q, want %q", got, payload)
	}
}

func TestFrameExactLimitAndLimitPlusOne(t *testing.T) {
	if MaxFrameBytes != 64<<10 {
		t.Fatalf("MaxFrameBytes = %d, want %d", MaxFrameBytes, 64<<10)
	}
	ok := bytes.Repeat([]byte("a"), MaxFrameBytes)
	if _, err := FrameLength(len(ok)); err != nil {
		t.Fatalf("exact limit rejected: %v", err)
	}
	var framed bytes.Buffer
	if err := WriteFrame(&framed, ok); err != nil {
		t.Fatalf("writing exact limit: %v", err)
	}
	got, err := ReadFrame(bytes.NewReader(framed.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, ok) {
		t.Fatal("exact-limit payload changed")
	}
	over := bytes.Repeat([]byte("b"), MaxFrameBytes+1)
	if _, err := FrameLength(len(over)); err == nil {
		t.Fatal("limit+1 was accepted by FrameLength")
	}
	if err := WriteFrame(io.Discard, over); codeOf(err) != CodeFrame {
		t.Fatalf("limit+1 write code = %q (%v)", codeOf(err), err)
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(MaxFrameBytes+1))
	if _, err := ReadFrame(bytes.NewReader(prefix[:])); codeOf(err) != CodeFrame {
		t.Fatalf("limit+1 read code = %q (%v)", codeOf(err), err)
	}
}

func TestFrameRejectsZeroTruncatedCoalescedAndTrailing(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		var prefix [4]byte
		if _, err := ReadFrame(bytes.NewReader(prefix[:])); codeOf(err) != CodeFrame {
			t.Fatalf("zero frame code = %q (%v)", codeOf(err), err)
		}
	})
	t.Run("truncatedPrefix", func(t *testing.T) {
		if _, err := ReadFrame(bytes.NewReader([]byte{0, 0, 1})); codeOf(err) != CodeFrame {
			t.Fatalf("truncated prefix code = %q, want %q (%v)", codeOf(err), CodeFrame, err)
		}
	})
	t.Run("truncatedPayload", func(t *testing.T) {
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], 8)
		if _, err := ReadFrame(bytes.NewReader(append(prefix[:], 'x', 'y'))); codeOf(err) != CodeFrame {
			t.Fatalf("truncated payload code = %q, want %q (%v)", codeOf(err), CodeFrame, err)
		}
	})
	payee, payer := newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WriteFrame(&first, payload); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrame(&second, payload); err != nil {
		t.Fatal(err)
	}
	coalesced := append(bytes.Clone(first.Bytes()), second.Bytes()...)
	t.Run("coalesced", func(t *testing.T) {
		if _, err := ReceiveSignedObject(bytes.NewReader(coalesced)); codeOf(err) != CodeFrame {
			t.Fatalf("coalesced frames code = %q (%v)", codeOf(err), err)
		}
	})
	t.Run("trailing", func(t *testing.T) {
		trailing := append(bytes.Clone(first.Bytes()), 0x0a)
		if _, err := ReceiveSignedObject(bytes.NewReader(trailing)); codeOf(err) != CodeFrame {
			t.Fatalf("trailing bytes code = %q (%v)", codeOf(err), err)
		}
	})
}

func TestFrameRejectsInvalidUTF8JSONAndBase64(t *testing.T) {
	t.Run("invalidUTF8", func(t *testing.T) {
		payload := []byte("{\"version\":1,\"kind\":\"request\",\"canonical\":\"\xc3\x28\"}")
		if utf8.Valid(payload) {
			t.Fatal("payload is unexpectedly valid UTF-8")
		}
		if _, err := DecodeSignedObject(payload); codeOf(err) != CodeSchema {
			t.Fatalf("invalid UTF-8 code = %q (%v)", codeOf(err), err)
		}
	})
	t.Run("invalidJSON", func(t *testing.T) {
		if _, err := DecodeSignedObject([]byte(`{"version":1,`)); codeOf(err) != CodeSchema {
			t.Fatalf("invalid JSON code = %q (%v)", codeOf(err), err)
		}
	})
	t.Run("invalidBase64", func(t *testing.T) {
		payload := []byte(`{"version":1,"kind":"request","canonical":"{}","public_key":"@@@","signature":"@@@"}`)
		if _, err := DecodeSignedObject(payload); codeOf(err) != CodeSchema {
			t.Fatalf("invalid base64 code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
		}
		if _, err := base64.StdEncoding.DecodeString("@@@"); err == nil {
			t.Fatal("test fixture is valid base64")
		}
	})
}

func TestClosedEnvelopeRejectsUnknownFields(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	obj["diagnostic"] = "log-line"
	extra, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSignedObject(extra); codeOf(err) != CodeSchema {
		t.Fatalf("unknown envelope field code = %q (%v)", codeOf(err), err)
	}
	delete(obj, "diagnostic")
	delete(obj, "canonical")
	missing, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSignedObject(missing); codeOf(err) != CodeSchema {
		t.Fatalf("missing canonical field code = %q (%v)", codeOf(err), err)
	}
}

func TestProtocolMismatchIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	_, err := payee.node.Host.NewStream(ctx, payer.node.ID(), protocol.ID("/bitbook/payment/9.0.0"))
	if err == nil {
		t.Fatal("mismatched payment protocol was negotiated")
	}
	stream, err := payee.node.Host.NewStream(ctx, payer.node.ID(), network.DirectProtocolCurrent)
	if err == nil {
		_ = stream.Close()
		t.Fatal("payment recipient advertised the direct protocol as a payment handler")
	}
	if network.PaymentProtocolCurrent != "/bitbook/payment/1.0.0" {
		t.Fatalf("PaymentProtocolCurrent = %q", network.PaymentProtocolCurrent)
	}
}

func TestNetworkStatusIsCancellationOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	ack, err := payee.svc.SendRequest(ctx, req)
	if err != nil || !ack.Accepted || ack.Digest == "" {
		t.Fatalf("request ack = %+v err=%v", ack, err)
	}
	paid := PaymentStatusEventV1{
		V: 1, RequestID: req.RequestID, EventID: "aaaabbbbccccddddeeeeffff00001111",
		Nonce: "0123456789abcdef0123456789abcdef", Status: StatusPaid, At: "2026-08-30T12:06:00Z", TxRef: "txid",
	}
	if _, err := DecodePaymentStatus(mustJSON(t, paid)); err != nil {
		t.Fatalf("codec rejected paid shape: %v", err)
	}
	ack, err = payee.svc.SendStatus(ctx, paid)
	if err == nil && ack.Accepted {
		t.Fatal("network paid status was accepted")
	}
	if codeOf(err) != CodeStatus && ack.Code != CodeStatus {
		t.Fatalf("paid transport code err=%q ack=%q (%v)", codeOf(err), ack.Code, err)
	}
	expired := paid
	expired.Status = StatusExpired
	expired.TxRef = ""
	expired.EventID = "bbbbaaaaccccddddeeeeffff00001111"
	expired.Nonce = "fedcba9876543210fedcba9876543210"
	if _, err := DecodePaymentStatus(mustJSON(t, expired)); err != nil {
		t.Fatalf("codec rejected expired shape: %v", err)
	}
	ack, err = payee.svc.SendStatus(ctx, expired)
	if err == nil && ack.Accepted {
		t.Fatal("network expired status was accepted")
	}
	if codeOf(err) != CodeStatus && ack.Code != CodeStatus {
		t.Fatalf("expired transport code err=%q ack=%q (%v)", codeOf(err), ack.Code, err)
	}
}

func TestTransportAcknowledgement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	ack, err := payee.svc.SendRequest(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !ack.Accepted || ack.Digest == "" || ack.Code != "" {
		t.Fatalf("ack = %+v, want accepted digest", ack)
	}
	stored, err := payer.svc.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest != ack.Digest || stored.Direction != DirectionInbound {
		t.Fatalf("stored = %+v digest=%s", stored, ack.Digest)
	}
}

func TestTransportDoesNotLeakGoroutinesOrFileDescriptors(t *testing.T) {
	runTransportCycle(t)
	warmDeadline := time.Now().Add(2 * time.Second)
	lastG, lastFD, stable := -1, -1, 0
	for time.Now().Before(warmDeadline) {
		runtime.GC()
		currentG, currentFD := runtime.NumGoroutine(), fdCount(t)
		if currentG == lastG && currentFD == lastFD {
			stable++
			if stable >= 2 {
				break
			}
		} else {
			stable = 0
		}
		lastG, lastFD = currentG, currentFD
		time.Sleep(20 * time.Millisecond)
	}
	if stable < 2 {
		t.Fatal("network warm-up resources did not converge before the deadline")
	}
	beforeG, beforeFD := runtime.NumGoroutine(), fdCount(t)
	const cycles = 3
	for cycle := 0; cycle < cycles; cycle++ {
		runTransportCycle(t)
	}
	const allowance = cycles - 1
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= beforeG+allowance && fdCount(t) <= beforeFD+allowance {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("leak across %d cycles: goroutines %d->%d fds %d->%d allowance=%d", cycles, beforeG, runtime.NumGoroutine(), beforeFD, fdCount(t), allowance)
}

func runTransportCycle(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer func() {
		payee.close()
		payer.close()
		cancel()
	}()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	if _, err := payee.svc.SendRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
}

type splitReader struct {
	raw  []byte
	cuts []int
	off  int
	step int
}

func (r *splitReader) Read(p []byte) (int, error) {
	if r.off >= len(r.raw) {
		return 0, io.EOF
	}
	end := len(r.raw)
	if r.step < len(r.cuts) {
		end = r.cuts[r.step]
		r.step++
		if end > len(r.raw) {
			end = len(r.raw)
		}
		if end < r.off {
			end = r.off
		}
	}
	n := copy(p, r.raw[r.off:end])
	r.off += n
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

type byteReader struct{ raw []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.raw) == 0 {
		return 0, io.EOF
	}
	p[0] = r.raw[0]
	r.raw = r.raw[1:]
	return 1, nil
}

func fdCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}
