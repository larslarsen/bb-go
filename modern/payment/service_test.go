package payment

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/network"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestTwoNodePayeeRequestThenPayerAcceptsLinkedCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if !ack.Accepted || ack.Digest == "" {
		t.Fatalf("request was not acknowledged: %+v", ack)
	}
	stored, err := payer.svc.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest != ack.Digest || stored.Direction != DirectionInbound || stored.Signed.Canonical == "" {
		t.Fatalf("payer did not persist the verified request: %+v", stored)
	}
	if !stored.ReceivedAt.Equal(payer.clock.Now()) {
		t.Fatalf("inbound request receipt time = %s, want injected %s", stored.ReceivedAt, payer.clock.Now())
	}
	outbound, err := payee.svc.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if outbound.Direction != DirectionOutbound || outbound.Digest != ack.Digest {
		t.Fatalf("payee outbound record = %+v", outbound)
	}
	if !outbound.ReceivedAt.Equal(payee.clock.Now()) {
		t.Fatalf("outbound request receipt time = %s, want injected %s", outbound.ReceivedAt, payee.clock.Now())
	}

	cancelEvent := liveCancel(req.RequestID)
	statusAck, err := payee.svc.SendStatus(ctx, cancelEvent)
	if err != nil {
		t.Fatal(err)
	}
	if !statusAck.Accepted || statusAck.Digest == "" {
		t.Fatalf("cancellation was not acknowledged: %+v", statusAck)
	}
	event, err := payer.svc.GetEvent(ctx, cancelEvent.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Digest != statusAck.Digest || event.Direction != DirectionInbound {
		t.Fatalf("payer did not persist linked cancellation: %+v", event)
	}
	if !event.ReceivedAt.Equal(payer.clock.Now()) {
		t.Fatalf("inbound status receipt time = %s, want injected %s", event.ReceivedAt, payer.clock.Now())
	}
	if event.Signed.Kind != KindStatus {
		t.Fatalf("stored status kind = %q", event.Signed.Kind)
	}
	outboundEvent, err := payee.svc.GetEvent(ctx, cancelEvent.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if outboundEvent.Direction != DirectionOutbound || outboundEvent.Digest != statusAck.Digest {
		t.Fatalf("payee outbound status record = %+v", outboundEvent)
	}
	if !outboundEvent.ReceivedAt.Equal(payee.clock.Now()) {
		t.Fatalf("outbound status receipt time = %s, want injected %s", outboundEvent.ReceivedAt, payee.clock.Now())
	}
}

func TestRestartFromSameDatastoreView(t *testing.T) {
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
	payer.svc.Close()
	restarted, err := NewService(payer.node, WithClock(payer.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	stored, err := restarted.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest != ack.Digest {
		t.Fatalf("restart lost digest %s, got %s", ack.Digest, stored.Digest)
	}
}

func TestIdenticalRetryIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	signed, err := SignRequest(payee.node.PrivateKey, req)
	if err != nil {
		t.Fatal(err)
	}
	first, err := payee.svc.SendSigned(ctx, payer.node.ID(), signed)
	if err != nil || !first.Accepted {
		t.Fatalf("first delivery = %+v err=%v", first, err)
	}
	second, err := payee.svc.SendSigned(ctx, payer.node.ID(), signed)
	if err != nil || !second.Accepted {
		t.Fatalf("identical retry = %+v err=%v", second, err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("retry digest changed %s -> %s", first.Digest, second.Digest)
	}
	listed, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if countRecords(listed, signed.Kind, req.RequestID) != 1 {
		t.Fatalf("retry created a duplicate record: %+v", listed)
	}
}

func TestConflictingRequestIDAndNonceAreRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	originalSigned, err := SignRequest(payee.node.PrivateKey, req)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := DigestHex(DomainSeparatorRequest, []byte(originalSigned.Canonical))
	if _, err := payee.svc.SendRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
	conflictID := req
	conflictID.Memo = "tea"
	conflictIDSigned, err := SignRequest(payee.node.PrivateKey, conflictID)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := payee.svc.SendRequest(ctx, conflictID)
	requireRejectedCode(t, ack, err, CodeReplay)
	conflictNonce := req
	conflictNonce.RequestID = "abcdabcdabcdabcdabcdabcdabcdabcd"
	conflictNonce.Memo = "other"
	conflictNonceSigned, err := SignRequest(payee.node.PrivateKey, conflictNonce)
	if err != nil {
		t.Fatal(err)
	}
	ack, err = payee.svc.SendRequest(ctx, conflictNonce)
	requireRejectedCode(t, ack, err, CodeReplay)
	stored, err := payer.svc.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest != originalDigest || stored.Signed.Canonical != originalSigned.Canonical {
		t.Fatalf("request conflict replaced the original record: %+v", stored)
	}
	if _, err := payer.svc.GetRequest(ctx, conflictNonce.RequestID); err == nil {
		t.Fatal("nonce-conflicting request left a record under its distinct request_id")
	}
	records, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertOnlyRecordedObject(
		t,
		records,
		KindRequest,
		originalDigest,
		originalSigned.Canonical,
		conflictIDSigned.Canonical,
		conflictNonceSigned.Canonical,
	)
}

func TestConflictingEventIDAndNonceAreRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	if _, err := payee.svc.SendRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
	event := liveCancel(req.RequestID)
	originalSigned, err := SignStatus(payee.node.PrivateKey, event)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := DigestHex(DomainSeparatorStatus, []byte(originalSigned.Canonical))
	if _, err := payee.svc.SendStatus(ctx, event); err != nil {
		t.Fatal(err)
	}
	conflictID := event
	conflictID.At = "2026-08-30T12:06:00Z"
	conflictIDSigned, err := SignStatus(payee.node.PrivateKey, conflictID)
	if err != nil {
		t.Fatal(err)
	}
	if codeOf(CheckStatusReplay(event, "one", conflictID, "two")) != CodeReplay {
		t.Fatal("event_id reuse with different bytes was not a replay")
	}
	ack, err := payee.svc.SendStatus(ctx, conflictID)
	requireRejectedCode(t, ack, err, CodeReplay)
	stored, err := payer.svc.GetEvent(ctx, event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Digest != originalDigest || stored.Signed.Canonical != originalSigned.Canonical {
		t.Fatalf("event_id conflict replaced the original event: %+v", stored)
	}
	conflictNonce := event
	conflictNonce.EventID = "ccccddddaaaabbbbffffeeee00001111"
	conflictNonce.At = "2026-08-30T12:07:00Z"
	conflictNonceSigned, err := SignStatus(payee.node.PrivateKey, conflictNonce)
	if err != nil {
		t.Fatal(err)
	}
	if codeOf(CheckStatusReplay(event, "one", conflictNonce, "two")) != CodeReplay {
		t.Fatal("event nonce reuse with different bytes was not a replay")
	}
	ack, err = payee.svc.SendStatus(ctx, conflictNonce)
	requireRejectedCode(t, ack, err, CodeReplay)
	if _, err := payer.svc.GetEvent(ctx, conflictNonce.EventID); err == nil {
		t.Fatal("nonce-conflicting event left a record under its distinct event_id")
	}
	records, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertOnlyRecordedObject(
		t,
		records,
		KindStatus,
		originalDigest,
		originalSigned.Canonical,
		conflictIDSigned.Canonical,
		conflictNonceSigned.Canonical,
	)
}

func TestStatusNonceMustDifferFromLinkedRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	requestAck, err := payee.svc.SendRequest(ctx, req)
	if err != nil || !requestAck.Accepted {
		t.Fatalf("linked request was not persisted: ack=%+v err=%v", requestAck, err)
	}
	if _, err := payer.svc.GetRequest(ctx, req.RequestID); err != nil {
		t.Fatalf("linked request is absent before status replay: %v", err)
	}
	event := liveCancel(req.RequestID)
	event.Nonce = req.Nonce
	if event.Nonce != req.Nonce {
		t.Fatal("status fixture did not reuse the linked request nonce")
	}
	signed, err := SignStatus(payee.node.PrivateKey, event)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := payee.svc.SendSigned(ctx, payer.node.ID(), signed)
	requireRejectedCode(t, ack, err, CodeReplay)
	if _, err := payer.svc.GetEvent(ctx, event.EventID); err == nil {
		t.Fatal("request-nonce-conflicting status left a stored event")
	}
	records, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	statusRecords := 0
	for _, record := range records {
		if record.Signed.Kind == KindStatus {
			statusRecords++
		}
	}
	if statusRecords != 0 {
		t.Fatalf("stored status records = %d, want zero: %+v", statusRecords, records)
	}
}

func TestStatusBeforeRequestIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	event := liveCancel("00112233445566778899aabbccddeeff")
	signed, err := SignStatus(payee.node.PrivateKey, event)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := payee.svc.SendSigned(ctx, payer.node.ID(), signed)
	if err == nil && ack.Accepted {
		t.Fatal("status-before-request was accepted")
	}
	if codeOf(err) != CodeLinkage && ack.Code != CodeLinkage {
		t.Fatalf("status-before-request code err=%q ack=%q (%v)", codeOf(err), ack.Code, err)
	}
	if _, err := payer.svc.GetEvent(ctx, event.EventID); err == nil {
		t.Fatal("status-before-request left a stored event")
	}
}

func TestWrongSignerAndLinkAreRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	attacker := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	defer attacker.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	connectPaymentNodes(t, ctx, attacker.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	if _, err := payee.svc.SendRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
	event := liveCancel(req.RequestID)
	signed, err := SignStatus(attacker.node.PrivateKey, event)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := attacker.svc.SendSigned(ctx, payer.node.ID(), signed)
	requireRejectedCode(t, ack, err, CodePayee)
	if _, err := payer.svc.GetEvent(ctx, event.EventID); err == nil {
		t.Fatal("status signed by a non-payee left a stored event")
	}
	wrongLink := liveCancel("deadbeefdeadbeefdeadbeefdeadbeef")
	wrongLink.EventID = "01230123012301230123012301230123"
	wrongLink.Nonce = "32103210321032103210321032103210"
	ack, err = payee.svc.SendStatus(ctx, wrongLink)
	requireRejectedCode(t, ack, err, CodeLinkage)
	if _, err := payer.svc.GetEvent(ctx, wrongLink.EventID); err == nil {
		t.Fatal("status for an unknown request left a stored event")
	}
}

func TestExpiredRequestIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	payer.clock.Set(time.Date(2026, 8, 30, 12, 15, 0, 0, time.UTC))
	req := liveRequest(payee.node.ID(), payer.node.ID())
	ack, err := payee.svc.SendRequest(ctx, req)
	if err == nil && ack.Accepted {
		t.Fatal("expired request was accepted")
	}
	if codeOf(err) != CodeExpired && ack.Code != CodeExpired {
		t.Fatalf("expired code err=%q ack=%q (%v)", codeOf(err), ack.Code, err)
	}
	if _, err := payer.svc.GetRequest(ctx, req.RequestID); err == nil {
		t.Fatal("expired request left a stored record")
	}
}

func TestConcurrentDuplicateDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	signed, err := SignRequest(payee.node.PrivateKey, req)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	acks := make(chan Acknowledgement, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ack, err := payee.svc.SendSigned(ctx, payer.node.ID(), signed)
			acks <- ack
			errs <- err
		}()
	}
	wg.Wait()
	close(acks)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var accepted int
	for ack := range acks {
		if ack.Accepted && ack.Digest != "" {
			accepted++
		}
	}
	if accepted != 2 {
		t.Fatalf("concurrent identical deliveries accepted = %d, want 2", accepted)
	}
	listed, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if countRecords(listed, KindRequest, req.RequestID) != 1 {
		t.Fatalf("concurrent delivery duplicated state: %+v", listed)
	}
}

func TestStorageFailureBeforeAcknowledgementLeavesNoPartialRecord(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payerNode := newPaymentNode(t, ctx)
	defer payee.close()
	defer payerNode.Close()
	clock := newTestClock(time.Date(2026, 8, 30, 12, 10, 0, 0, time.UTC))
	payerSvc, err := NewService(payerNode, WithClock(clock.Now), WithDatastore(failingDatastore{Batching: payerNode.Datastore}))
	if err != nil {
		t.Fatal(err)
	}
	defer payerSvc.Close()
	connectPaymentNodes(t, ctx, payee.node, payerNode)
	req := liveRequest(payee.node.ID(), payerNode.ID())
	ack, err := payee.svc.SendRequest(ctx, req)
	if err == nil && ack.Accepted {
		t.Fatal("storage failure still produced an acknowledgement")
	}
	if codeOf(err) != CodeStorage && ack.Code != CodeStorage {
		t.Fatalf("storage failure code err=%q ack=%q (%v)", codeOf(err), ack.Code, err)
	}
	if _, err := payerSvc.GetRequest(ctx, req.RequestID); err == nil {
		t.Fatal("partial accepted record survived a storage failure")
	}
	listed, err := payerSvc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("storage failure left records: %+v", listed)
	}
}

func TestFirstTerminalCancellationBlocksConflictingLaterStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payee := newTestPayment(t, ctx, nil)
	payer := newTestPayment(t, ctx, nil)
	defer payee.close()
	defer payer.close()
	connectPaymentNodes(t, ctx, payee.node, payer.node)
	req := liveRequest(payee.node.ID(), payer.node.ID())
	if _, err := payee.svc.SendRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
	first := liveCancel(req.RequestID)
	firstSigned, err := SignStatus(payee.node.PrivateKey, first)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest := DigestHex(DomainSeparatorStatus, []byte(firstSigned.Canonical))
	if _, err := payee.svc.SendStatus(ctx, first); err != nil {
		t.Fatal(err)
	}
	later := first
	later.EventID = "99998888777766665555444433332222"
	later.Nonce = "22223333444455556666777788889999"
	later.At = "2026-08-30T12:08:00Z"
	laterSigned, err := SignStatus(payee.node.PrivateKey, later)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := payee.svc.SendStatus(ctx, later)
	requireRejectedCode(t, ack, err, CodeStatus)
	if _, err := payer.svc.GetEvent(ctx, later.EventID); err == nil {
		t.Fatal("later terminal status left a stored event")
	}
	listed, err := payer.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertOnlyRecordedObject(
		t,
		listed,
		KindStatus,
		firstDigest,
		firstSigned.Canonical,
		laterSigned.Canonical,
	)
}

func TestPackageHasNoForbiddenCapability(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := []string{
		"net/http", "github.com/gorilla/websocket", "wallet", "zcash", "monero",
		"trezor", "ledger", "usb", "hidapi", "exchangerate", "exchange-rate",
		"btcsuite", "librustzcash",
	}
	forbiddenExports := []string{
		"Wallet", "ZEC", "Zec", "Zcash", "XMR", "Xmr", "Monero", "HTTP", "Http",
		"ExchangeRate", "Rate", "Quote", "Account", "Transaction", "Broadcast", "USB", "Usb", "Device", "Coin",
	}
	productionFiles := 0
	checkExportedName := func(filename, kind, name string) {
		for _, forbidden := range forbiddenExports {
			if name == forbidden || strings.Contains(name, forbidden) {
				t.Fatalf("%s exports forbidden %s %s", filename, kind, name)
			}
		}
	}
	var checkEmbeddedField func(string, ast.Expr)
	checkEmbeddedField = func(filename string, expression ast.Expr) {
		switch embedded := expression.(type) {
		case *ast.Ident:
			if ast.IsExported(embedded.Name) {
				checkExportedName(filename, "embedded struct field", embedded.Name)
			}
		case *ast.SelectorExpr:
			if ast.IsExported(embedded.Sel.Name) {
				checkExportedName(filename, "embedded struct field", embedded.Sel.Name)
			}
		case *ast.StarExpr:
			checkEmbeddedField(filename, embedded.X)
		case *ast.IndexExpr:
			checkEmbeddedField(filename, embedded.X)
		case *ast.IndexListExpr:
			checkEmbeddedField(filename, embedded.X)
		}
	}
	for _, pkg := range pkgs {
		for filename, parsed := range pkg.Files {
			productionFiles++
			for _, spec := range parsed.Imports {
				path := strings.Trim(spec.Path.Value, `"`)
				lower := strings.ToLower(path)
				for _, forbidden := range forbiddenImports {
					if path == forbidden || strings.Contains(lower, forbidden) {
						t.Fatalf("%s imports forbidden capability %q", filename, path)
					}
				}
			}
			for _, declaration := range parsed.Decls {
				switch decl := declaration.(type) {
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						if ast.IsExported(typeSpec.Name.Name) {
							checkExportedName(filename, "type", typeSpec.Name.Name)
						}
						structType, ok := typeSpec.Type.(*ast.StructType)
						if !ok {
							continue
						}
						for _, field := range structType.Fields.List {
							for _, name := range field.Names {
								if ast.IsExported(name.Name) {
									checkExportedName(filename, "struct field", name.Name)
								}
							}
							if len(field.Names) == 0 {
								checkEmbeddedField(filename, field.Type)
							}
						}
					}
				case *ast.FuncDecl:
					if ast.IsExported(decl.Name.Name) {
						kind := "function"
						if decl.Recv != nil {
							kind = "method"
						}
						checkExportedName(filename, kind, decl.Name.Name)
					}
				}
			}
			src, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(src), "/ob/exchangerates") || strings.Contains(string(src), "/wallet/") {
				t.Fatalf("%s introduces a wallet or rate HTTP route", filename)
			}
		}
	}
	if productionFiles == 0 {
		t.Fatal("negative-capability scan inspected zero non-test production files")
	}
	_ = []any{
		Service{}, SignedObject{}, PaymentRequestV1{}, PaymentStatusEventV1{},
		Acknowledgement{}, RecordedObject{}, VerifiedObject{}, Error{},
		KindRequest, KindStatus, DirectionInbound, DirectionOutbound,
		network.PaymentProtocolCurrent, MaxFrameBytes,
	}
}

type testPayment struct {
	node  *network.Node
	svc   *Service
	clock *testClock
}

func (p *testPayment) close() {
	if p.svc != nil {
		p.svc.Close()
	}
	if p.node != nil {
		_ = p.node.Close()
	}
}

func newTestPayment(t *testing.T, ctx context.Context, ds datastore.Batching) *testPayment {
	t.Helper()
	node := newPaymentNode(t, ctx)
	if ds != nil {
		node.Datastore = ds
	}
	clock := newTestClock(time.Date(2026, 8, 30, 12, 10, 0, 0, time.UTC))
	svc, err := NewService(node, WithClock(clock.Now))
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	return &testPayment{node: node, svc: svc, clock: clock}
}

func newPaymentNode(t *testing.T, ctx context.Context) *network.Node {
	t.Helper()
	identity := newIdentity(t)
	node, err := network.New(ctx, network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
		PrivateKey:            identity.priv,
	})
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func connectPaymentNodes(t *testing.T, ctx context.Context, left, right *network.Node) {
	t.Helper()
	if err := left.Connect(ctx, peer.AddrInfo{ID: right.ID(), Addrs: right.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if err := right.Connect(ctx, peer.AddrInfo{ID: left.ID(), Addrs: left.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
}

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock(now time.Time) *testClock {
	return &testClock{t: now.UTC()}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = now.UTC()
}

type failingDatastore struct {
	datastore.Batching
}

func (f failingDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	return errors.New("injected storage failure")
}

func (f failingDatastore) Batch(ctx context.Context) (datastore.Batch, error) {
	return nil, errors.New("injected storage failure")
}

func countRecords(records []RecordedObject, kind Kind, id string) int {
	n := 0
	for _, record := range records {
		if record.Signed.Kind != kind {
			continue
		}
		switch kind {
		case KindRequest:
			if strings.Contains(record.Signed.Canonical, `"request_id":"`+id+`"`) {
				n++
			}
		case KindStatus:
			if strings.Contains(record.Signed.Canonical, `"event_id":"`+id+`"`) {
				n++
			}
		}
	}
	return n
}

func acceptedReplay(ack Acknowledgement, err error) bool {
	return err == nil && ack.Accepted
}

func requireRejectedCode(t testing.TB, ack Acknowledgement, err error, want string) {
	t.Helper()
	if err == nil && ack.Accepted {
		t.Fatalf("object was accepted, want rejection code %s", want)
	}
	errCode := codeOf(err)
	if errCode != "" && errCode != want {
		t.Fatalf("error code = %q, want %q (%v)", errCode, want, err)
	}
	if ack.Code != "" && ack.Code != want {
		t.Fatalf("acknowledgement code = %q, want %q", ack.Code, want)
	}
	if errCode == "" && ack.Code == "" {
		t.Fatalf("rejection exposed no stable code, want %q (%v)", want, err)
	}
}

func assertOnlyRecordedObject(
	t testing.TB,
	records []RecordedObject,
	kind Kind,
	wantDigest string,
	wantCanonical string,
	forbiddenCanonical ...string,
) {
	t.Helper()
	count := 0
	for _, record := range records {
		if record.Signed.Kind != kind {
			continue
		}
		count++
		if record.Digest != wantDigest || record.Signed.Canonical != wantCanonical {
			t.Fatalf("stored %s record differs from original digest/canonical: %+v", kind, record)
		}
		for _, forbidden := range forbiddenCanonical {
			if record.Signed.Canonical == forbidden {
				t.Fatalf("stored %s record contains conflicting canonical object", kind)
			}
		}
	}
	if count != 1 {
		t.Fatalf("stored %s records = %d, want exactly one: %+v", kind, count, records)
	}
}
