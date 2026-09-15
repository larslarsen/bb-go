package localclient_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larslarsen/bb-go/modern/localclient"
	"github.com/larslarsen/bb-go/modern/payment"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	recordsPath     = "/v1/payment/records"
	maxSuccessBytes = 4 << 20
	clientDirName   = "local-client"
	descriptorName  = "connection.json"
)

type descriptor struct {
	V          int    `json:"v"`
	Endpoint   string `json:"endpoint"`
	PeerID     string `json:"peer_id"`
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
}

type countingLister struct {
	mu      sync.Mutex
	calls   int
	records []payment.RecordedObject
	err     error
}

func (c *countingLister) List(ctx context.Context) ([]payment.RecordedObject, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	out := make([]payment.RecordedObject, len(c.records))
	copy(out, c.records)
	return out, nil
}

func (c *countingLister) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestLocalClientStartUnavailableOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("unsupported-platform branch")
	}
	_, err := localclient.Start(t.TempDir(), newPeerID(t), &countingLister{})
	if !errors.Is(err, localclient.ErrUnavailable) {
		t.Fatalf("Start error = %v, want ErrUnavailable", err)
	}
}

func TestLocalClientHTTPRejectionsAndSuccessfulRead(t *testing.T) {
	records := []payment.RecordedObject{
		sampleRecord(payment.KindRequest, payment.DirectionInbound, "inbound-canonical"),
		sampleRecord(payment.KindRequest, payment.DirectionOutbound, "outbound-canonical"),
		sampleRecord(payment.KindStatus, payment.DirectionInbound, "cancelled-canonical"),
	}
	lister := &countingLister{records: records}
	_, desc := startServer(t, lister)
	client := localHTTPClient()

	type step struct {
		name      string
		mutate    func(*http.Request)
		method    string
		path      string
		body      []byte
		wantCode  int
		wantError string
		wantRead  bool
	}
	cases := []step{
		{name: "missingAuth", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED"},
		{name: "unknownPathUnauthenticated", method: http.MethodGet, path: "/v1/secret", wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED"},
		{name: "wrongMethodUnauthenticated", method: http.MethodPost, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED"},
		{name: "malformedBearer", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED", mutate: func(r *http.Request) {
			r.Header.Set("Authorization", "bearer "+desc.Token)
			r.Header.Set("X-BitBook-Instance", desc.InstanceID)
		}},
		{name: "wrongToken", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED", mutate: func(r *http.Request) {
			authorize(r, strings.Repeat("ab", 32), desc.InstanceID)
		}},
		{name: "wrongInstance", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED", mutate: func(r *http.Request) {
			authorize(r, desc.Token, strings.Repeat("cd", 16))
		}},
		{name: "duplicateAuthorization", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED", mutate: func(r *http.Request) {
			r.Header["Authorization"] = []string{"Bearer " + desc.Token, "Bearer " + desc.Token}
			r.Header.Set("X-BitBook-Instance", desc.InstanceID)
		}},
		{name: "duplicateInstance", method: http.MethodGet, path: recordsPath, wantCode: http.StatusUnauthorized, wantError: "UNAUTHORIZED", mutate: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+desc.Token)
			r.Header["X-Bitbook-Instance"] = []string{desc.InstanceID, desc.InstanceID}
		}},
		{name: "originPresent", method: http.MethodGet, path: recordsPath, wantCode: http.StatusForbidden, wantError: "FORBIDDEN", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
			r.Header.Set("Origin", "http://127.0.0.1")
		}},
		{name: "originNull", method: http.MethodGet, path: recordsPath, wantCode: http.StatusForbidden, wantError: "FORBIDDEN", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
			r.Header.Set("Origin", "null")
		}},
		{name: "originEmpty", method: http.MethodGet, path: recordsPath, wantCode: http.StatusForbidden, wantError: "FORBIDDEN", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
			r.Header["Origin"] = []string{""}
		}},
		{name: "hostMismatch", method: http.MethodGet, path: recordsPath, wantCode: http.StatusForbidden, wantError: "FORBIDDEN", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
			r.Host = "127.0.0.1:1"
		}},
		{name: "wrongMethodAuthenticated", method: http.MethodPost, path: recordsPath, wantCode: http.StatusMethodNotAllowed, wantError: "METHOD_NOT_ALLOWED", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
		}},
		{name: "unknownPathAuthenticated", method: http.MethodGet, path: "/v1/payment/other", wantCode: http.StatusNotFound, wantError: "NOT_FOUND", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
		}},
		{name: "queryAuthenticated", method: http.MethodGet, path: recordsPath + "?limit=1", wantCode: http.StatusBadRequest, wantError: "BAD_REQUEST", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
		}},
		{name: "bodyAuthenticated", method: http.MethodGet, path: recordsPath, body: []byte("{}"), wantCode: http.StatusBadRequest, wantError: "BAD_REQUEST", mutate: func(r *http.Request) {
			authorize(r, desc.Token, desc.InstanceID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := lister.Calls()
			req := newClientRequest(t, tc.method, desc.Endpoint+tc.path, tc.body)
			if tc.mutate != nil {
				tc.mutate(req)
			}
			res := doRequest(t, client, req)
			defer res.Body.Close()
			assertErrorStatus(t, res, tc.wantCode, tc.wantError)
			if tc.wantRead {
				if lister.Calls() == before {
					t.Fatal("expected the record reader to be called")
				}
			} else if lister.Calls() != before {
				t.Fatalf("denied request called the record reader %d times", lister.Calls()-before)
			}
		})
	}

	before := lister.Calls()
	req := newClientRequest(t, http.MethodGet, desc.Endpoint+recordsPath, nil)
	authorize(req, desc.Token, desc.InstanceID)
	res := doRequest(t, client, req)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("success status %d body %s", res.StatusCode, body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", res.Header.Get("Cache-Control"))
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS header %q", got)
	}
	var payload struct {
		V          int                      `json:"v"`
		PeerID     string                   `json:"peer_id"`
		InstanceID string                   `json:"instance_id"`
		Records    []payment.RecordedObject `json:"records"`
	}
	dec := json.NewDecoder(res.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("decode success body: %v", err)
	}
	if payload.V != 1 || payload.PeerID != desc.PeerID || payload.InstanceID != desc.InstanceID {
		t.Fatalf("success envelope mismatch: %+v", payload)
	}
	if len(payload.Records) != len(records) {
		t.Fatalf("records = %d, want %d", len(payload.Records), len(records))
	}
	for i, record := range payload.Records {
		if record.Signed.Canonical != records[i].Signed.Canonical ||
			record.Signed.Kind != records[i].Signed.Kind ||
			record.Direction != records[i].Direction ||
			record.Digest != records[i].Digest ||
			!bytes.Equal(record.Signed.PublicKey, records[i].Signed.PublicKey) ||
			!bytes.Equal(record.Signed.Signature, records[i].Signed.Signature) {
			t.Fatalf("record %d does not preserve signed object fields", i)
		}
	}
	if lister.Calls() != before+1 {
		t.Fatalf("success path reader calls = %d, want %d", lister.Calls()-before, 1)
	}
}

func TestLocalClientRejectsStreamingUnknownLengthBody(t *testing.T) {
	lister := &countingLister{records: []payment.RecordedObject{sampleRecord(payment.KindRequest, payment.DirectionInbound, "chunked")}}
	_, desc := startServer(t, lister)
	reader, writer := io.Pipe()
	req, err := http.NewRequest(http.MethodGet, desc.Endpoint+recordsPath, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = -1
	authorize(req, desc.Token, desc.InstanceID)
	errCh := make(chan error, 1)
	resCh := make(chan *http.Response, 1)
	go func() {
		res, err := localHTTPClient().Do(req)
		if err != nil {
			errCh <- err
			return
		}
		resCh <- res
	}()
	select {
	case err := <-errCh:
		t.Fatalf("streaming request failed: %v", err)
	case res := <-resCh:
		defer res.Body.Close()
		assertErrorStatus(t, res, http.StatusBadRequest, "BAD_REQUEST")
		if lister.Calls() != 0 {
			t.Fatalf("unknown-length body reached the record reader %d times", lister.Calls())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler blocked reading the streaming body")
	}
	_ = writer.Close()
}

func TestLocalClientRejectsNonLoopbackRemote(t *testing.T) {
	lister := &countingLister{records: []payment.RecordedObject{sampleRecord(payment.KindRequest, payment.DirectionInbound, "x")}}
	server, desc := startServer(t, lister)
	req := httptest.NewRequest(http.MethodGet, desc.Endpoint+recordsPath, nil)
	req.Host = strings.TrimPrefix(desc.Endpoint, "http://")
	req.RemoteAddr = "8.8.8.8:9"
	authorize(req, desc.Token, desc.InstanceID)
	rec := httptest.NewRecorder()
	before := lister.Calls()
	server.ServeHTTP(rec, req)
	assertErrorStatus(t, rec.Result(), http.StatusForbidden, "FORBIDDEN")
	if lister.Calls() != before {
		t.Fatal("non-loopback request reached the record reader")
	}
}

func TestLocalClientEmptyStoreReturnsEmptyArray(t *testing.T) {
	lister := &countingLister{}
	_, desc := startServer(t, lister)
	req := newClientRequest(t, http.MethodGet, desc.Endpoint+recordsPath, nil)
	authorize(req, desc.Token, desc.InstanceID)
	res := doRequest(t, localHTTPClient(), req)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"records":[]`)) {
		t.Fatalf("empty store body = %s", raw)
	}
}

func TestLocalClientStorageFailure(t *testing.T) {
	lister := &countingLister{err: errors.New("leveldb exploded")}
	_, desc := startServer(t, lister)
	req := newClientRequest(t, http.MethodGet, desc.Endpoint+recordsPath, nil)
	authorize(req, desc.Token, desc.InstanceID)
	res := doRequest(t, localHTTPClient(), req)
	defer res.Body.Close()
	assertErrorStatus(t, res, http.StatusServiceUnavailable, "UNAVAILABLE")
}

func TestLocalClientResponseSizeBoundary(t *testing.T) {
	base := measureSuccessSize(t, "")
	if base >= maxSuccessBytes {
		t.Fatalf("base success JSON %d already exceeds 4 MiB", base)
	}
	t.Run("atCap", func(t *testing.T) {
		assertRecordSizeStatus(t, maxSuccessBytes-base, http.StatusOK, "")
	})
	t.Run("overCap", func(t *testing.T) {
		assertRecordSizeStatus(t, maxSuccessBytes-base+1, http.StatusServiceUnavailable, "TOO_LARGE")
	})
}

func TestLocalClientPublishesPrivateDescriptorAndCleansUp(t *testing.T) {
	requireLinuxLocalClient(t)
	t.Run("happyPath", func(t *testing.T) {
		lister := &countingLister{}
		dataDir := t.TempDir()
		server, desc := startServerIn(t, dataDir, lister)
		clientDir := filepath.Join(dataDir, clientDirName)
		info, err := os.Lstat(clientDir)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
			t.Fatalf("local-client mode = %s perm %o", info.Mode(), info.Mode().Perm())
		}
		fileInfo, err := os.Lstat(filepath.Join(clientDir, descriptorName))
		if err != nil {
			t.Fatal(err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm() != 0o600 {
			t.Fatalf("connection.json mode = %s perm %o", fileInfo.Mode(), fileInfo.Mode().Perm())
		}
		if desc.V != 1 || !strings.HasPrefix(desc.Endpoint, "http://127.0.0.1:") {
			t.Fatalf("descriptor endpoint %q", desc.Endpoint)
		}
		if !isLowerHex(desc.Token, 64) || !isLowerHex(desc.InstanceID, 32) {
			t.Fatalf("credential encoding token=%q instance=%q", desc.Token, desc.InstanceID)
		}
		matches, err := filepath.Glob(filepath.Join(clientDir, ".connection-*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("leftover temp files: %v", matches)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(clientDir, descriptorName)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("descriptor survived Close: %v", err)
		}
		if _, err := os.Lstat(clientDir); err != nil {
			t.Fatalf("directory was removed: %v", err)
		}
	})

	t.Run("leaveReplacedDescriptor", func(t *testing.T) {
		dataDir := t.TempDir()
		server, _ := startServerIn(t, dataDir, &countingLister{})
		clientDir := filepath.Join(dataDir, clientDirName)
		path := filepath.Join(clientDir, descriptorName)
		replacement := []byte(`{"v":1,"owned":"replacement"}` + "\n")
		tmp := filepath.Join(clientDir, ".replacement-new")
		if err := os.WriteFile(tmp, replacement, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, path); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Close(ctx); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("replacement descriptor was removed: %v", err)
		}
		if !bytes.Equal(got, replacement) {
			t.Fatalf("replacement bytes changed: %q", got)
		}
	})

	t.Run("rejectWorldReadableDir", func(t *testing.T) {
		dataDir := t.TempDir()
		clientDir := filepath.Join(dataDir, clientDirName)
		if err := os.Mkdir(clientDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(clientDir, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := localclient.Start(dataDir, newPeerID(t), &countingLister{})
		if err == nil {
			t.Fatal("accepted a 0755 local-client directory")
		}
		info, statErr := os.Lstat(clientDir)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("existing permissions changed to %o", info.Mode().Perm())
		}
	})

	t.Run("rejectDirectorySymlink", func(t *testing.T) {
		dataDir := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(dataDir, clientDirName)); err != nil {
			t.Fatal(err)
		}
		_, err := localclient.Start(dataDir, newPeerID(t), &countingLister{})
		if err == nil {
			t.Fatal("accepted a symlinked local-client directory")
		}
		info, statErr := os.Lstat(filepath.Join(dataDir, clientDirName))
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("directory symlink was replaced")
		}
	})

	t.Run("rejectDescriptorSymlink", func(t *testing.T) {
		dataDir := t.TempDir()
		clientDir := filepath.Join(dataDir, clientDirName)
		if err := os.Mkdir(clientDir, 0o700); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "stolen.json")
		sentinel := []byte(`{"stolen":true}`)
		if err := os.WriteFile(outside, sentinel, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(clientDir, descriptorName)); err != nil {
			t.Fatal(err)
		}
		_, err := localclient.Start(dataDir, newPeerID(t), &countingLister{})
		if err == nil {
			t.Fatal("published through a connection.json symlink")
		}
		got, readErr := os.ReadFile(outside)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, sentinel) {
			t.Fatal("symlink target was written")
		}
	})
}

func FuzzLocalClientAuth(f *testing.F) {
	if runtime.GOOS != "linux" {
		f.Skip("local payment access is Linux-only")
		return
	}
	const validCred = "VALID"
	lister := &countingLister{records: []payment.RecordedObject{sampleRecord(payment.KindRequest, payment.DirectionInbound, "fuzz-canonical")}}
	dataDir, err := os.MkdirTemp("", "pay003-fuzz-")
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = os.RemoveAll(dataDir) })
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		f.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		f.Fatal(err)
	}
	server, err := localclient.Start(dataDir, id, lister)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})
	desc := readDescriptor(f, dataDir)
	host := strings.TrimPrefix(desc.Endpoint, "http://")
	f.Add("GET", recordsPath, validCred, validCred, "")
	f.Add("GET", recordsPath, "", "", "")
	f.Add("GET", recordsPath, "Bearer x", validCred, "")
	f.Add("GET", recordsPath, validCred, validCred, "http://example")
	f.Add("POST", recordsPath, validCred, validCred, "")
	f.Add("GET", "/v1/other", validCred, validCred, "")
	f.Fuzz(func(t *testing.T, method, path, authorization, instance, origin string) {
		if method == "" {
			method = "GET"
		}
		req, err := http.NewRequest(method, "http://127.0.0.1/x", nil)
		if err != nil {
			return
		}
		if path == "" {
			path = "/"
		}
		req.URL.Path = path
		req.Host = host
		req.RemoteAddr = "127.0.0.1:9"
		mappedAuth := authorization
		mappedInstance := instance
		if authorization == validCred {
			mappedAuth = "Bearer " + desc.Token
		}
		if instance == validCred {
			mappedInstance = desc.InstanceID
		}
		if mappedAuth != "" {
			req.Header.Set("Authorization", mappedAuth)
		}
		if mappedInstance != "" {
			req.Header.Set("X-BitBook-Instance", mappedInstance)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		before := lister.Calls()
		server.ServeHTTP(rec, req)
		wantRead := method == http.MethodGet &&
			path == recordsPath &&
			mappedAuth == "Bearer "+desc.Token &&
			mappedInstance == desc.InstanceID &&
			origin == ""
		if wantRead {
			if rec.Code != http.StatusOK {
				t.Fatalf("positive auth status %d", rec.Code)
			}
			if lister.Calls() == before {
				t.Fatal("expected nonempty read")
			}
			return
		}
		if lister.Calls() != before {
			t.Fatalf("unauthenticated fuzz input reached the record reader method=%q path=%q", method, path)
		}
	})
}

func requireLinuxLocalClient(t testing.TB) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("local payment access is Linux-only")
	}
}

func startServer(t testing.TB, lister *countingLister) (*localclient.Server, descriptor) {
	t.Helper()
	requireLinuxLocalClient(t)
	return startServerIn(t, t.TempDir(), lister)
}

func startServerIn(t testing.TB, dataDir string, lister *countingLister) (*localclient.Server, descriptor) {
	t.Helper()
	requireLinuxLocalClient(t)
	server, err := localclient.Start(dataDir, newPeerID(t), lister)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Close(ctx); err != nil {
			t.Errorf("closing local client: %v", err)
		}
	})
	return server, readDescriptor(t, dataDir)
}

func readDescriptor(t interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}, dataDir string) descriptor {
	t.Helper()
	path := filepath.Join(dataDir, clientDirName, descriptorName)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("refusing to follow connection.json symlink")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var desc descriptor
	if err := dec.Decode(&desc); err != nil {
		t.Fatalf("descriptor JSON: %v", err)
	}
	return desc
}

func newPeerID(t interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}) peer.ID {
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

func sampleRecord(kind payment.Kind, direction payment.Direction, canonical string) payment.RecordedObject {
	return payment.RecordedObject{
		Signed: payment.SignedObject{
			Version:   1,
			Kind:      kind,
			Canonical: canonical,
			PublicKey: []byte{0x01, 0x02, 0x03},
			Signature: []byte{0x04, 0x05},
		},
		Digest:     "digest-fixed",
		Direction:  direction,
		ReceivedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}

func measureSuccessSize(t *testing.T, canonical string) int {
	t.Helper()
	lister := &countingLister{records: []payment.RecordedObject{sampleRecord(payment.KindRequest, payment.DirectionInbound, canonical)}}
	_, desc := startServer(t, lister)
	req := newClientRequest(t, http.MethodGet, desc.Endpoint+recordsPath, nil)
	authorize(req, desc.Token, desc.InstanceID)
	res := doRequest(t, localHTTPClient(), req)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		t.Fatalf("probe status %d body %s", res.StatusCode, body)
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return len(raw)
}

func assertRecordSizeStatus(t *testing.T, extra int, wantCode int, wantErr string) {
	t.Helper()
	record := sampleRecord(payment.KindRequest, payment.DirectionInbound, strings.Repeat("a", extra))
	lister := &countingLister{records: []payment.RecordedObject{record}}
	_, desc := startServer(t, lister)
	req := newClientRequest(t, http.MethodGet, desc.Endpoint+recordsPath, nil)
	authorize(req, desc.Token, desc.InstanceID)
	res := doRequest(t, localHTTPClient(), req)
	defer res.Body.Close()
	if wantCode == http.StatusOK {
		if res.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
			t.Fatalf("status %d body %s", res.StatusCode, body)
		}
		return
	}
	assertErrorStatus(t, res, wantCode, wantErr)
}

func authorize(r *http.Request, token, instance string) {
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-BitBook-Instance", instance)
}

func newClientRequest(t *testing.T, method, url string, body []byte) *http.Request {
	t.Helper()
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func localHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: true,
		},
	}
}

func doRequest(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertErrorStatus(t *testing.T, res *http.Response, status int, code string) {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read error body: %v", err)
	}
	if res.StatusCode != status {
		t.Fatalf("status %d, want %d body %s", res.StatusCode, status, body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", res.Header.Get("Cache-Control"))
	}
	var payload struct {
		Error string `json:"error"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("error body: %v raw %q", err, body)
	}
	if payload.Error != code {
		t.Fatalf("error code %q, want %q raw %q", payload.Error, code, body)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Fatalf("error body was not a closed object, trailing %v raw %q", err, body)
	}
}

func isLowerHex(value string, n int) bool {
	if len(value) != n {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
