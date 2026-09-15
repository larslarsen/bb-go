package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/larslarsen/bb-go/modern/payment"
)

const (
	localClientDirName        = "local-client"
	localClientDescriptorName = "connection.json"
	localClientRecordsPath    = "/v1/payment/records"
	localClientReadyWait      = 3 * time.Second
)

type localClientDescriptor struct {
	V          int    `json:"v"`
	Endpoint   string `json:"endpoint"`
	PeerID     string `json:"peer_id"`
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
}

func TestLocalClientDaemonReadsPersistedPaymentRecord(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("local payment access is Linux-only")
	}
	linked := startLinkedPaymentDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), daemonOpTimeout)
	defer cancel()

	signed := signPayeeRequest(t, linked.payee, daemonPaymentRequest(t, linked.payee.ID(), linked.daemonID, time.Now()))
	digest := independentRequestDigest(t, signed)
	ack := sendPaymentBytes(t, ctx, linked.payee, linked.daemonID, marshalSignedObject(t, signed))
	requireAcceptedDigest(t, ack, digest)

	first := waitLocalClientDescriptor(t, linked.dataDir, localClientReadyWait)
	if first.PeerID != linked.daemonID.String() {
		t.Fatalf("descriptor peer_id %s, want %s", first.PeerID, linked.daemonID)
	}
	assertCredentialsAbsentFromLogs(t, linked.child, first)

	got := getPaymentRecords(t, first)
	requireSingleInboundHTTPRecord(t, got, signed, digest)

	linked.stop(t)
	if _, err := os.Lstat(filepath.Join(linked.dataDir, localClientDirName, localClientDescriptorName)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("descriptor survived shutdown: %v", err)
	}

	linked.startChild(t)
	second := waitLocalClientDescriptor(t, linked.dataDir, localClientReadyWait)
	if second.Token == first.Token || second.InstanceID == first.InstanceID {
		t.Fatal("restart reused the previous credential")
	}
	if second.PeerID != first.PeerID {
		t.Fatalf("peer id changed across restart: %s -> %s", first.PeerID, second.PeerID)
	}
	assertCredentialsAbsentFromLogs(t, linked.child, first)
	assertCredentialsAbsentFromLogs(t, linked.child, second)

	stale := getPaymentRecordsStatus(t, second.Endpoint, first.Token, first.InstanceID)
	defer stale.Body.Close()
	if stale.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(io.LimitReader(stale.Body, 512))
		t.Fatalf("previous credential status %d body %s", stale.StatusCode, body)
	}

	again := getPaymentRecords(t, second)
	requireSingleInboundHTTPRecord(t, again, signed, digest)
}

func waitLocalClientDescriptor(t *testing.T, dataDir string, timeout time.Duration) localClientDescriptor {
	t.Helper()
	path := filepath.Join(dataDir, localClientDirName, localClientDescriptorName)
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		desc, err := readLocalClientDescriptor(path)
		if err == nil {
			return desc
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("local-client descriptor was not published: %v", last)
	return localClientDescriptor{}
}

func readLocalClientDescriptor(path string) (localClientDescriptor, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return localClientDescriptor{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return localClientDescriptor{}, errors.New("connection.json is a symlink")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return localClientDescriptor{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var desc localClientDescriptor
	if err := dec.Decode(&desc); err != nil {
		return localClientDescriptor{}, err
	}
	if desc.V != 1 || desc.Endpoint == "" || desc.Token == "" || desc.InstanceID == "" {
		return localClientDescriptor{}, errors.New("incomplete descriptor")
	}
	return desc, nil
}

func getPaymentRecords(t *testing.T, desc localClientDescriptor) []payment.RecordedObject {
	t.Helper()
	res := getPaymentRecordsStatus(t, desc.Endpoint, desc.Token, desc.InstanceID)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		t.Fatalf("records status %d body %s", res.StatusCode, body)
	}
	var payload struct {
		V          int                      `json:"v"`
		PeerID     string                   `json:"peer_id"`
		InstanceID string                   `json:"instance_id"`
		Records    []payment.RecordedObject `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.V != 1 || payload.PeerID != desc.PeerID || payload.InstanceID != desc.InstanceID {
		t.Fatalf("records envelope mismatch: %+v", payload)
	}
	return payload.Records
}

func getPaymentRecordsStatus(t *testing.T, endpoint, token, instance string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, endpoint+localClientRecordsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-BitBook-Instance", instance)
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true},
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func requireSingleInboundHTTPRecord(t *testing.T, records []payment.RecordedObject, signed payment.SignedObject, digest string) {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("http records = %d, want 1", len(records))
	}
	record := records[0]
	if record.Direction != payment.DirectionInbound || record.Digest != digest {
		t.Fatalf("http record direction=%q digest_match=%t", record.Direction, record.Digest == digest)
	}
	if record.Signed.Canonical != signed.Canonical ||
		!bytes.Equal(record.Signed.PublicKey, signed.PublicKey) ||
		!bytes.Equal(record.Signed.Signature, signed.Signature) {
		t.Fatal("http record did not preserve the signed payment object")
	}
}

func assertCredentialsAbsentFromLogs(t *testing.T, child *paymentDaemonChild, desc localClientDescriptor) {
	t.Helper()
	logs := child.stdout.String() + child.stderr.String()
	for _, secret := range []string{desc.Token, desc.InstanceID, "Bearer " + desc.Token} {
		if secret != "" && strings.Contains(logs, secret) {
			t.Fatalf("captured logs contain credential material")
		}
	}
}
