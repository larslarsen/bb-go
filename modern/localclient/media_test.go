package localclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/payment"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	testMediaMaxBytes = int64(100 << 20)
	testRangeMaxBytes = int64(8 << 20)
)

type mediaFixture struct {
	ctx     context.Context
	cancel  context.CancelFunc
	node    *network.Node
	store   *attachment.Store
	server  *Server
	desc    descriptor
	client  *http.Client
	dataDir string
}

type mediaRecords struct {
	records []payment.RecordedObject
}

func (r *mediaRecords) List(context.Context) ([]payment.RecordedObject, error) {
	return append([]payment.RecordedObject(nil), r.records...), nil
}

func TestMEDIA001M2CPublicRoundTrip(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 32, MaxLogicalBytes: 256 << 20})
	contents := [][]byte{
		{},
		bytes.Repeat([]byte{0x5a}, 1<<20),
		bytes.Repeat([]byte("multi"), (1<<20)/5+7),
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
	}
	for i, content := range contents {
		id := testReferenceID(byte(i + 1))
		reference := putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
		if reference.ReferenceID != id.String() || reference.ByteLength != int64(len(content)) {
			t.Fatalf("upload %d response = %+v", i, reference)
		}
		status := getReference(t, fixture, id, http.StatusOK)
		if status != reference {
			t.Fatalf("status %d = %+v, want %+v", i, status, reference)
		}
		handle := createHandle(t, fixture, id, http.StatusCreated)
		res := mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)
		body := readResponse(t, res)
		if res.StatusCode != http.StatusOK || !bytes.Equal(body, content) {
			t.Fatalf("download %d status=%d bytes=%d", i, res.StatusCode, len(body))
		}
		assertDownloadHeaders(t, res, int64(len(content)), http.StatusOK)
		if i == len(contents)-1 && res.Header.Get("Content-Type") != "application/octet-stream" {
			t.Fatal("active content was not forced to an inert download type")
		}
		res = mediaRequest(t, fixture, http.MethodHead, "/v1/media/handles/"+handle.Handle, nil)
		body = readResponse(t, res)
		if res.StatusCode != http.StatusOK || len(body) != 0 {
			t.Fatalf("HEAD %d status=%d body=%d", i, res.StatusCode, len(body))
		}
		assertDownloadHeaders(t, res, int64(len(content)), http.StatusOK)
		if len(content) > 64 {
			res = mediaRequestWith(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil, func(req *http.Request) {
				req.Header.Set("Range", "bytes=13-31")
			})
			body = readResponse(t, res)
			if res.StatusCode != http.StatusPartialContent || !bytes.Equal(body, content[13:32]) {
				t.Fatalf("range %d status=%d body=%d", i, res.StatusCode, len(body))
			}
			if got := res.Header.Get("Content-Range"); got != "bytes 13-31/"+decimal(int64(len(content))) {
				t.Fatalf("Content-Range = %q", got)
			}
		}
	}

	chunkedID := testReferenceID(20)
	chunked := []byte("chunked upload")
	putMedia(t, fixture, chunkedID, &unknownLengthReader{reader: bytes.NewReader(chunked)}, -1, true)

	res := mediaRequest(t, fixture, http.MethodGet, recordsPath, nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("payment route status = %d", res.StatusCode)
	}
}

func TestMEDIA001M2CRealHTTPShutdown(t *testing.T) {
	requireM2CLinux(t)
	t.Run("stalled upload", func(t *testing.T) {
		fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20})
		controlID := testReferenceID(21)
		controlContent := []byte("successful upload before shutdown")
		control := putMedia(t, fixture, controlID, bytes.NewReader(controlContent), int64(len(controlContent)), false)

		endpoint, err := url.Parse(fixture.desc.Endpoint)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := net.DialTimeout("tcp", endpoint.Host, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		stalledID := testReferenceID(22)
		request := "PUT /v1/media/public/" + stalledID.String() + " HTTP/1.1\r\n" +
			"Host: " + endpoint.Host + "\r\n" +
			"Authorization: Bearer " + fixture.desc.Token + "\r\n" +
			"X-BitBook-Instance: " + fixture.desc.InstanceID + "\r\n" +
			"Content-Type: application/octet-stream\r\n" +
			"Content-Length: 64\r\n\r\n"
		if _, err := io.WriteString(conn, request); err != nil {
			t.Fatal(err)
		}
		waitForMediaActivity(t, fixture.server.media, 1, 1)

		closeCtx, cancelClose := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancelClose()
		closeResult := make(chan error, 1)
		started := time.Now()
		go func() { closeResult <- fixture.server.Close(closeCtx) }()
		var closeErr error
		prompt := true
		select {
		case closeErr = <-closeResult:
		case <-time.After(750 * time.Millisecond):
			prompt = false
			_ = conn.Close()
			select {
			case closeErr = <-closeResult:
			case <-time.After(2 * time.Second):
				t.Fatal("server shutdown remained blocked after closing the stalled client")
			}
		}
		if !prompt {
			t.Errorf("server shutdown waited beyond its deadline; elapsed=%s err=%v", time.Since(started), closeErr)
		}
		assertMediaActivity(t, fixture.server.media, 0, 0)
		if got, err := fixture.store.GetPublic(fixture.ctx, stalledID); err == nil {
			t.Fatalf("stalled upload became ready: %+v", got)
		}
		if got, err := fixture.store.GetPublic(fixture.ctx, controlID); err != nil || got.ID != controlID ||
			got.File.CID.String() != control.CID || got.File.ByteLength != int64(len(controlContent)) {
			t.Fatalf("shutdown changed successful control: %+v, %v", got, err)
		}
	})

	t.Run("post-stage cancellation", func(t *testing.T) {
		fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
		id := testReferenceID(23)
		content := bytes.Repeat([]byte("delivery"), 1024)
		putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
		handle := createHandle(t, fixture, id, http.StatusCreated)
		if got := readResponse(t, mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)); !bytes.Equal(got, content) {
			t.Fatalf("successful delivery bytes=%d want=%d", len(got), len(content))
		}

		requestCtx, cancelRequest := context.WithCancel(context.Background())
		fixture.server.media.copyPublic = func(_ context.Context, _ network.PublicFile, dst io.Writer) (int64, error) {
			n, err := dst.Write(content)
			cancelRequest()
			return int64(n), err
		}
		req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil, 0).WithContext(requestCtx)
		rec := httptest.NewRecorder()
		fixture.server.ServeHTTP(rec, req)
		assertRecorderError(t, rec, http.StatusServiceUnavailable, "UNAVAILABLE")
		if bytes.Contains(rec.Body.Bytes(), content[:64]) {
			t.Fatal("post-stage cancellation exposed file bytes")
		}
	})

	t.Run("cancel blocked delivery", func(t *testing.T) {
		fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
		id := testReferenceID(24)
		content := bytes.Repeat([]byte("blocked-delivery"), 1024)
		putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
		handle := createHandle(t, fixture, id, http.StatusCreated)
		requestCtx, cancelRequest := context.WithCancel(context.Background())
		writer := newBlockingDeadlineResponseWriter()
		done := make(chan struct{})
		go func() {
			req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil, 0).WithContext(requestCtx)
			fixture.server.ServeHTTP(writer, req)
			close(done)
		}()
		select {
		case <-writer.writeStarted:
		case <-time.After(2 * time.Second):
			writer.unblock()
			t.Fatal("delivery did not reach the response writer")
		}
		cancelRequest()
		prompt := true
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			prompt = false
			writer.unblock()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled delivery remained blocked after test cleanup")
			}
		}
		if !prompt {
			t.Error("request cancellation did not interrupt a blocked response write")
		}
		assertMediaActivity(t, fixture.server.media, 0, 0)
	})
}

func TestMEDIA001M2CRealHTTPTruncatedUpload(t *testing.T) {
	requireM2CLinux(t)
	t.Run("deterministic unexpected EOF", func(t *testing.T) {
		fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
		id := testReferenceID(25)
		body := &unexpectedEOFBody{content: []byte("ab")}
		req := directMediaRequest(fixture.server, http.MethodPut, "/v1/media/public/"+id.String(), body, 3)
		rec := httptest.NewRecorder()
		fixture.server.ServeHTTP(rec, req)
		assertRecorderError(t, rec, http.StatusBadRequest, "BAD_REQUEST")
		if got, err := fixture.store.GetPublic(fixture.ctx, id); err == nil {
			t.Fatalf("deterministically truncated upload became ready: %+v", got)
		}
	})

	t.Run("raw HTTP half-close", func(t *testing.T) {
		fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
		endpoint, err := url.Parse(fixture.desc.Endpoint)
		if err != nil {
			t.Fatal(err)
		}
		rawConn, err := net.DialTimeout("tcp", endpoint.Host, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		conn, ok := rawConn.(*net.TCPConn)
		if !ok {
			rawConn.Close()
			t.Fatal("loopback connection is not TCP")
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		id := testReferenceID(26)
		request := "PUT /v1/media/public/" + id.String() + " HTTP/1.1\r\n" +
			"Host: " + endpoint.Host + "\r\n" +
			"Authorization: Bearer " + fixture.desc.Token + "\r\n" +
			"X-BitBook-Instance: " + fixture.desc.InstanceID + "\r\n" +
			"Content-Type: application/octet-stream\r\n" +
			"Content-Length: 3\r\n" +
			"Connection: close\r\n\r\nab"
		if _, err := io.WriteString(conn, request); err != nil {
			t.Fatal(err)
		}
		if err := conn.CloseWrite(); err != nil {
			t.Fatal(err)
		}
		res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPut})
		if err != nil {
			t.Fatal(err)
		}
		body := readResponse(t, res)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("truncated HTTP upload status=%d body=%s", res.StatusCode, body)
		}
		assertJSONErrorBytes(t, body, "BAD_REQUEST")
		if got, err := fixture.store.GetPublic(fixture.ctx, id); err == nil {
			t.Fatalf("raw truncated upload became ready: %+v", got)
		}
	})
}

func TestMEDIA001M2CTrustBoundaryAndCanonicalRoutes(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20})
	id := testReferenceID(30)
	path := "/v1/media/public/" + id.String()

	tests := []struct {
		name   string
		path   string
		mutate func(*http.Request)
		status int
		code   string
		both   bool
	}{
		{name: "missing_auth", path: path, mutate: func(r *http.Request) { r.Header.Del("Authorization") }, status: http.StatusUnauthorized, code: "UNAUTHORIZED", both: true},
		{name: "duplicate_auth", path: path, mutate: func(r *http.Request) {
			r.Header["Authorization"] = []string{"Bearer " + fixture.desc.Token, "Bearer " + fixture.desc.Token}
		}, status: http.StatusUnauthorized, code: "UNAUTHORIZED", both: true},
		{name: "wrong_instance", path: path, mutate: func(r *http.Request) { r.Header.Set("X-BitBook-Instance", strings.Repeat("a", 32)) }, status: http.StatusUnauthorized, code: "UNAUTHORIZED", both: true},
		{name: "empty_origin", path: path, mutate: func(r *http.Request) { r.Header["Origin"] = []string{""} }, status: http.StatusForbidden, code: "FORBIDDEN", both: true},
		{name: "duplicate_origin", path: path, mutate: func(r *http.Request) { r.Header["Origin"] = []string{"", ""} }, status: http.StatusForbidden, code: "FORBIDDEN", both: true},
		{name: "wrong_host", path: path, mutate: func(r *http.Request) { r.Host = "127.0.0.1:1" }, status: http.StatusForbidden, code: "FORBIDDEN", both: true},
		{name: "non_loopback", path: path, mutate: func(r *http.Request) { r.RemoteAddr = "192.0.2.1:9" }, status: http.StatusForbidden, code: "FORBIDDEN", both: true},
		{name: "query", path: path + "?x=1", status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "bare_query", path: path + "?", status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "trailing_slash", path: path + "/", status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "extra_slash", path: "/v1/media//public/" + id.String(), status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "dot_segment", path: "/v1/media/public/../" + id.String(), status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "backslash", path: "/v1/media/public/" + id.String() + `\x`, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "encoded_alias", path: "/v1/media/public/%" + "33" + id.String()[1:], status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "uppercase_id", path: "/v1/media/public/" + strings.ToUpper(id.String()), status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "zero_id", path: "/v1/media/public/" + strings.Repeat("0", 32), status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "ob_alias", path: "/ob/v1/media/public/" + id.String(), status: http.StatusMethodNotAllowed, code: "METHOD_NOT_ALLOWED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			methods := []string{http.MethodPut}
			if test.both {
				methods = append(methods, http.MethodGet)
			}
			for _, method := range methods {
				body := &countingBody{reader: bytes.NewReader([]byte("denied"))}
				req := directMediaRequest(fixture.server, method, test.path, body, int64(len("denied")))
				if test.mutate != nil {
					test.mutate(req)
				}
				rec := httptest.NewRecorder()
				fixture.server.ServeHTTP(rec, req)
				assertRecorderError(t, rec, test.status, test.code)
				if body.reads != 0 {
					t.Fatalf("%s denial read request body %d times", method, body.reads)
				}
				if _, err := fixture.store.GetPublic(fixture.ctx, id); !errors.Is(err, attachment.ErrNotFound) {
					t.Fatalf("%s denial changed storage: %v", method, err)
				}
				if len(fixture.server.media.handles) != 0 {
					t.Fatalf("%s denial allocated a handle", method)
				}
			}
		})
	}
	entries, err := os.ReadDir(filepath.Join(fixture.dataDir, directoryName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != descriptorName {
		t.Fatalf("denied requests created local-client artifacts: %v", entries)
	}

	for _, test := range []struct {
		method string
		path   string
		allow  string
	}{
		{method: http.MethodPatch, path: path, allow: "DELETE, GET, PUT"},
		{method: http.MethodGet, path: path + "/handles", allow: http.MethodPost},
		{method: http.MethodPost, path: "/v1/media/handles/" + id.String(), allow: "GET, HEAD"},
	} {
		res := mediaRequest(t, fixture, test.method, test.path, nil)
		if res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != test.allow {
			body := readResponse(t, res)
			t.Fatalf("method status=%d Allow=%q body=%s", res.StatusCode, res.Header.Get("Allow"), body)
		}
		assertHTTPError(t, res, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
	}

	for _, transferEncoding := range []bool{false, true} {
		body := &countingBody{reader: bytes.NewReader([]byte("forbidden body"))}
		req := directMediaRequest(fixture.server, http.MethodGet, path, body, int64(len("forbidden body")))
		if transferEncoding {
			req.ContentLength = -1
			req.TransferEncoding = []string{"chunked"}
		}
		rec := httptest.NewRecorder()
		fixture.server.ServeHTTP(rec, req)
		assertRecorderError(t, rec, http.StatusBadRequest, "BAD_REQUEST")
		if body.reads != 0 {
			t.Fatalf("non-PUT body rejection read %d times", body.reads)
		}
	}

	paymentOnlyDir := t.TempDir()
	paymentOnly, err := Start(paymentOnlyDir, fixture.node.Host.ID(), &mediaRecords{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = paymentOnly.Close(context.Background()) })
	body := &countingBody{reader: bytes.NewReader([]byte("x"))}
	req := directMediaRequest(paymentOnly, http.MethodPut, path, body, 1)
	rec := httptest.NewRecorder()
	paymentOnly.ServeHTTP(rec, req)
	assertRecorderError(t, rec, http.StatusNotFound, "NOT_FOUND")
	if body.reads != 0 {
		t.Fatal("payment-only Start read a media body")
	}

	if _, err := StartWithMedia(t.TempDir(), fixture.node.Host.ID(), &mediaRecords{}, nil, fixture.store); err == nil {
		t.Fatal("nil node accepted")
	}
	if _, err := StartWithMedia(t.TempDir(), fixture.node.Host.ID(), &mediaRecords{}, fixture.node, nil); err == nil {
		t.Fatal("nil store accepted")
	}
	otherID := independentPeerID(t)
	if _, err := StartWithMedia(t.TempDir(), otherID, &mediaRecords{}, fixture.node, fixture.store); err == nil {
		t.Fatal("mismatched node identity accepted")
	}
}

func TestMEDIA001M2CUploadLimitsRetryAndErrors(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 2, MaxLogicalBytes: 32})
	id := testReferenceID(40)
	path := "/v1/media/public/" + id.String()

	for _, test := range []struct {
		name   string
		length int64
		known  bool
		ok     bool
	}{
		{name: "unknown", length: -1, ok: true},
		{name: "zero", length: 0, known: true, ok: true},
		{name: "below", length: testMediaMaxBytes - 1, known: true, ok: true},
		{name: "at", length: testMediaMaxBytes, known: true, ok: true},
		{name: "above", length: testMediaMaxBytes + 1, known: true},
	} {
		t.Run("declared_"+test.name, func(t *testing.T) {
			req := directMediaRequest(fixture.server, http.MethodPut, path, http.NoBody, test.length)
			if test.length == -1 {
				req.TransferEncoding = []string{"chunked"}
			}
			max, known, err := validateMediaUpload(req)
			if test.ok {
				if err != nil || known != test.known || max < 1 || max > testMediaMaxBytes {
					t.Fatalf("validate length %d = max=%d known=%t err=%v", test.length, max, known, err)
				}
			} else if !errors.Is(err, errMediaTooLarge) {
				t.Fatalf("validate above limit = %v", err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*http.Request)
		status int
		code   string
	}{
		{name: "missing_content_type", mutate: func(r *http.Request) { r.Header.Del("Content-Type") }, status: http.StatusUnsupportedMediaType, code: "UNSUPPORTED_MEDIA_TYPE"},
		{name: "parameterized_content_type", mutate: func(r *http.Request) { r.Header.Set("Content-Type", "application/octet-stream; charset=binary") }, status: http.StatusUnsupportedMediaType, code: "UNSUPPORTED_MEDIA_TYPE"},
		{name: "content_encoding", mutate: func(r *http.Request) { r.Header.Set("Content-Encoding", "identity") }, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "duplicate_content_type", mutate: func(r *http.Request) {
			r.Header["Content-Type"] = []string{"application/octet-stream", "application/octet-stream"}
		}, status: http.StatusUnsupportedMediaType, code: "UNSUPPORTED_MEDIA_TYPE"},
		{name: "unsupported_transfer_encoding", mutate: func(r *http.Request) {
			r.ContentLength = -1
			r.TransferEncoding = []string{"gzip"}
		}, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "upload_range", mutate: func(r *http.Request) { r.Header.Set("Range", "bytes=0-0") }, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "declared_oversize", mutate: func(r *http.Request) { r.ContentLength = testMediaMaxBytes + 1 }, status: http.StatusRequestEntityTooLarge, code: "TOO_LARGE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &countingBody{reader: bytes.NewReader([]byte("x"))}
			req := directMediaRequest(fixture.server, http.MethodPut, path, body, 1)
			test.mutate(req)
			rec := httptest.NewRecorder()
			fixture.server.ServeHTTP(rec, req)
			assertRecorderError(t, rec, test.status, test.code)
			if body.reads != 0 {
				t.Fatalf("rejected upload read %d times", body.reads)
			}
		})
	}

	truncated := directMediaRequest(fixture.server, http.MethodPut, path, io.NopCloser(bytes.NewReader([]byte("ab"))), 3)
	rec := httptest.NewRecorder()
	fixture.server.ServeHTTP(rec, truncated)
	assertRecorderError(t, rec, http.StatusBadRequest, "BAD_REQUEST")
	if _, err := fixture.store.GetPublic(fixture.ctx, id); !errors.Is(err, attachment.ErrNotFound) {
		t.Fatalf("truncated upload became ready: %v", err)
	}

	original := putMedia(t, fixture, id, bytes.NewReader([]byte("first")), 5, false)
	retryBody := &countingBody{reader: bytes.NewReader([]byte("changed"))}
	retry := directMediaRequest(fixture.server, http.MethodPut, path, retryBody, 7)
	rec = httptest.NewRecorder()
	fixture.server.ServeHTTP(rec, retry)
	if rec.Code != http.StatusOK || retryBody.reads != 0 {
		t.Fatalf("ready retry status=%d reads=%d body=%s", rec.Code, retryBody.reads, rec.Body.Bytes())
	}
	if got := decodeReferenceResponse(t, rec.Body.Bytes()); got != original {
		t.Fatalf("ready retry = %+v, want %+v", got, original)
	}

	quotaFixture := newMediaFixture(t, attachment.Limits{MaxReferences: 1, MaxLogicalBytes: 32})
	putMedia(t, quotaFixture, testReferenceID(90), bytes.NewReader([]byte("full")), 4, false)
	quotaID := testReferenceID(41)
	quotaBody := &countingBody{reader: bytes.NewReader([]byte("quota"))}
	quotaReq := directMediaRequest(quotaFixture.server, http.MethodPut, "/v1/media/public/"+quotaID.String(), quotaBody, 5)
	rec = httptest.NewRecorder()
	quotaFixture.server.ServeHTTP(rec, quotaReq)
	assertRecorderError(t, rec, http.StatusInsufficientStorage, "QUOTA_EXCEEDED")
	if quotaBody.reads != 0 {
		t.Fatalf("quota rejection read %d times", quotaBody.reads)
	}

	notReadyID := testReferenceID(42)
	blocked := newBlockingReadCloser()
	done := make(chan error, 1)
	go func() {
		_, err := fixture.store.ImportPublic(fixture.ctx, notReadyID, blocked, 8)
		done <- err
	}()
	<-blocked.started
	getReference(t, fixture, notReadyID, http.StatusConflict)
	_ = blocked.Close()
	<-done

	getReference(t, fixture, testReferenceID(99), http.StatusNotFound)

	boundaryFixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 256 << 20})
	for _, test := range []struct {
		name    string
		id      byte
		length  int64
		chunked bool
		status  int
	}{
		{name: "known_at", id: 71, length: testMediaMaxBytes, status: http.StatusOK},
		{name: "chunked_at", id: 72, length: testMediaMaxBytes, chunked: true, status: http.StatusOK},
		{name: "chunked_above", id: 73, length: testMediaMaxBytes + 1, chunked: true, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := io.NopCloser(io.LimitReader(zeroReader{}, test.length))
			contentLength := test.length
			if test.chunked {
				contentLength = -1
			}
			req := directMediaRequest(boundaryFixture.server, http.MethodPut, "/v1/media/public/"+testReferenceID(test.id).String(), body, contentLength)
			if test.chunked {
				req.TransferEncoding = []string{"chunked"}
			}
			rec := httptest.NewRecorder()
			boundaryFixture.server.ServeHTTP(rec, req)
			if rec.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, test.status, rec.Body.Bytes())
			}
			if test.status == http.StatusOK {
				if got := decodeReferenceResponse(t, rec.Body.Bytes()); got.ByteLength != test.length {
					t.Fatalf("byte length=%d want=%d", got.ByteLength, test.length)
				}
				if err := boundaryFixture.store.Release(boundaryFixture.ctx, testReferenceID(test.id)); err != nil {
					t.Fatal(err)
				}
			} else {
				assertJSONErrorBytes(t, rec.Body.Bytes(), "TOO_LARGE")
			}
		})
	}
}

func TestMEDIA001M2CHandlesAdmissionAndFailures(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20})
	id := testReferenceID(50)
	content := []byte("handle content")
	putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)

	fixed := time.Unix(1_800_000_000, 0)
	fixture.server.media.now = func() time.Time { return fixed }
	first := createHandle(t, fixture, id, http.StatusCreated)
	second := createHandle(t, fixture, id, http.StatusCreated)
	if first.Handle == second.Handle || first.ReferenceID != second.ReferenceID {
		t.Fatalf("handles not independent: %+v %+v", first, second)
	}
	for range maxMediaHandles - 2 {
		createHandle(t, fixture, id, http.StatusCreated)
	}
	createHandle(t, fixture, id, http.StatusTooManyRequests)
	fixture.server.media.now = func() time.Time { return fixed.Add(mediaHandleLifetime) }
	res := mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+first.Handle, nil)
	assertHTTPError(t, res, http.StatusNotFound, "NOT_FOUND")
	if got := createHandle(t, fixture, id, http.StatusCreated); got.Handle == "" {
		t.Fatal("expired handle was not purged before capacity check")
	}

	fixture.server.media.random = errorReader{err: errors.New("random failed")}
	fixture.server.media.handles = make(map[string]mediaHandle)
	createHandle(t, fixture, id, http.StatusServiceUnavailable)
	fixture.server.media.random = bytes.NewReader(make([]byte, 16))
	createHandle(t, fixture, id, http.StatusServiceUnavailable)
	fixture.server.media.random = rand.Reader

	blocking := make(chan struct{})
	started := make(chan struct{}, 2)
	fixture.server.media.copyPublic = func(ctx context.Context, _ network.PublicFile, dst io.Writer) (int64, error) {
		started <- struct{}{}
		select {
		case <-blocking:
			written, err := dst.Write(content)
			return int64(written), err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	fixture.server.media.handles = make(map[string]mediaHandle)
	handles := []handleResponse{createHandle(t, fixture, id, http.StatusCreated), createHandle(t, fixture, id, http.StatusCreated), createHandle(t, fixture, id, http.StatusCreated)}
	type result struct {
		code int
		body []byte
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		handle := handles[i].Handle
		go func() {
			req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handle, nil, 0)
			rec := httptest.NewRecorder()
			fixture.server.ServeHTTP(rec, req)
			results <- result{code: rec.Code, body: rec.Body.Bytes()}
		}()
	}
	<-started
	<-started
	req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handles[2].Handle, nil, 0)
	rec := httptest.NewRecorder()
	fixture.server.ServeHTTP(rec, req)
	assertRecorderError(t, rec, http.StatusTooManyRequests, "BUSY")
	close(blocking)
	for range 2 {
		got := <-results
		if got.code != http.StatusOK || !bytes.Equal(got.body, content) {
			t.Fatalf("admitted read = status %d body %q", got.code, got.body)
		}
	}

	fixture.server.media.copyPublic = fixture.node.CopyPublicFile
	fixture.server.media.handles = make(map[string]mediaHandle)
	handle := createHandle(t, fixture, id, http.StatusCreated)
	deleteReference(t, fixture, id)
	res = mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)
	assertHTTPError(t, res, http.StatusNotFound, "NOT_FOUND")
}

func TestMEDIA001M2CConcurrentReleaseAndCancellation(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20})
	id := testReferenceID(55)
	content := []byte("concurrent snapshot")
	putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
	handle := createHandle(t, fixture, id, http.StatusCreated)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	fixture.server.media.copyPublic = func(ctx context.Context, _ network.PublicFile, dst io.Writer) (int64, error) {
		started <- struct{}{}
		select {
		case <-release:
			n, err := dst.Write(content)
			return int64(n), err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	readDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil, 0)
		rec := httptest.NewRecorder()
		fixture.server.ServeHTTP(rec, req)
		readDone <- rec
	}()
	<-started
	deleteReference(t, fixture, id)
	createHandleAfterRelease := mediaRequest(t, fixture, http.MethodPost, "/v1/media/public/"+id.String()+"/handles", nil)
	assertHTTPError(t, createHandleAfterRelease, http.StatusNotFound, "NOT_FOUND")
	stale := mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)
	assertHTTPError(t, stale, http.StatusNotFound, "NOT_FOUND")
	close(release)
	admitted := <-readDone
	if admitted.Code != http.StatusOK || !bytes.Equal(admitted.Body.Bytes(), content) {
		t.Fatalf("admitted snapshot status=%d body=%q", admitted.Code, admitted.Body.Bytes())
	}

	putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
	fixture.server.media.handles = make(map[string]mediaHandle)
	handles := []handleResponse{
		createHandle(t, fixture, id, http.StatusCreated),
		createHandle(t, fixture, id, http.StatusCreated),
		createHandle(t, fixture, id, http.StatusCreated),
	}
	started = make(chan struct{}, 3)
	release = make(chan struct{})
	fixture.server.media.copyPublic = func(ctx context.Context, _ network.PublicFile, dst io.Writer) (int64, error) {
		started <- struct{}{}
		select {
		case <-release:
			n, err := dst.Write(content)
			return int64(n), err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	runRead := func(ctx context.Context, handle string) <-chan *httptest.ResponseRecorder {
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			req := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handle, nil, 0).WithContext(ctx)
			rec := httptest.NewRecorder()
			fixture.server.ServeHTTP(rec, req)
			result <- rec
		}()
		return result
	}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := runRead(firstCtx, handles[0].Handle)
	secondDone := runRead(context.Background(), handles[1].Handle)
	<-started
	<-started
	busyReq := directMediaRequest(fixture.server, http.MethodGet, "/v1/media/handles/"+handles[2].Handle, nil, 0)
	busyRec := httptest.NewRecorder()
	fixture.server.ServeHTTP(busyRec, busyReq)
	assertRecorderError(t, busyRec, http.StatusTooManyRequests, "BUSY")
	cancelFirst()
	if cancelled := <-firstDone; cancelled.Code != http.StatusServiceUnavailable {
		t.Fatalf("cancelled read status=%d body=%s", cancelled.Code, cancelled.Body.Bytes())
	}
	thirdDone := runRead(context.Background(), handles[2].Handle)
	<-started
	close(release)
	for _, done := range []<-chan *httptest.ResponseRecorder{secondDone, thirdDone} {
		result := <-done
		if result.Code != http.StatusOK || !bytes.Equal(result.Body.Bytes(), content) {
			t.Fatalf("post-cancellation read status=%d body=%q", result.Code, result.Body.Bytes())
		}
	}
}

func TestMEDIA001M2CStagingAndShutdownFailures(t *testing.T) {
	requireM2CLinux(t)
	for _, test := range []struct {
		name   string
		mutate func(*mediaService)
	}{
		{name: "create", mutate: func(media *mediaService) {
			media.createStage = func(*os.Root) (*os.File, string, error) { return nil, "", errors.New("create failed") }
		}},
		{name: "unlink", mutate: func(media *mediaService) {
			media.unlinkStage = func(*os.Root, string) error { return errors.New("unlink failed") }
		}},
		{name: "write", mutate: func(media *mediaService) {
			media.wrapStageWriter = func(io.Writer) io.Writer { return errorWriter{err: errors.New("write failed")} }
		}},
		{name: "interrupted_copy", mutate: func(media *mediaService) {
			media.copyPublic = func(_ context.Context, _ network.PublicFile, dst io.Writer) (int64, error) {
				n, _ := dst.Write([]byte("prefix"))
				return int64(n), errors.New("copy failed")
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
			id := testReferenceID(60)
			putMedia(t, fixture, id, bytes.NewReader([]byte("stage failure")), 13, false)
			handle := createHandle(t, fixture, id, http.StatusCreated)
			test.mutate(fixture.server.media)
			res := mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)
			assertHTTPError(t, res, http.StatusServiceUnavailable, "UNAVAILABLE")
			entries, err := os.ReadDir(filepath.Join(fixture.dataDir, directoryName))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() != descriptorName {
					t.Fatalf("staging artifact survived failure: %s", entry.Name())
				}
			}
		})
	}

	cancelFixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 256 << 20})
	cancelID := testReferenceID(63)
	cancelBody := newBlockingReadCloser()
	cancelCtx, cancelRequest := context.WithCancel(context.Background())
	cancelReq := directMediaRequest(cancelFixture.server, http.MethodPut, "/v1/media/public/"+cancelID.String(), cancelBody, -1).WithContext(cancelCtx)
	cancelReq.TransferEncoding = []string{"chunked"}
	cancelDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		cancelFixture.server.ServeHTTP(rec, cancelReq)
		cancelDone <- rec
	}()
	<-cancelBody.started
	cancelRequest()
	select {
	case rec := <-cancelDone:
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("cancelled upload status=%d body=%s", rec.Code, rec.Body.Bytes())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request cancellation did not close blocked upload body")
	}
	putMedia(t, cancelFixture, testReferenceID(64), bytes.NewReader([]byte("slot freed")), 10, false)

	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 256 << 20})
	unrelatedID := testReferenceID(62)
	unrelated, err := fixture.store.ImportPublic(fixture.ctx, unrelatedID, bytes.NewReader([]byte("unrelated")), 9)
	if err != nil {
		t.Fatal(err)
	}
	id := testReferenceID(61)
	body := newBlockingReadCloser()
	req := directMediaRequest(fixture.server, http.MethodPut, "/v1/media/public/"+id.String(), body, -1)
	req.TransferEncoding = []string{"chunked"}
	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		fixture.server.ServeHTTP(rec, req)
		close(done)
	}()
	<-body.started
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := fixture.server.Close(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("shutdown left blocked upload handler")
	}
	if _, err := fixture.store.GetPublic(fixture.ctx, id); err == nil {
		t.Fatal("shutdown left a ready partial upload")
	}
	if got, err := fixture.store.GetPublic(fixture.ctx, unrelatedID); err != nil || got != unrelated {
		t.Fatalf("shutdown changed unrelated state: %+v, %v", got, err)
	}

	copyFixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
	copyID := testReferenceID(65)
	copyContent := []byte("blocked verified copy")
	putMedia(t, copyFixture, copyID, bytes.NewReader(copyContent), int64(len(copyContent)), false)
	copyHandle := createHandle(t, copyFixture, copyID, http.StatusCreated)
	copyStarted := make(chan struct{})
	copyFixture.server.media.copyPublic = func(ctx context.Context, _ network.PublicFile, _ io.Writer) (int64, error) {
		close(copyStarted)
		<-ctx.Done()
		return 0, ctx.Err()
	}
	copyDone := make(chan struct{})
	go func() {
		req := directMediaRequest(copyFixture.server, http.MethodGet, "/v1/media/handles/"+copyHandle.Handle, nil, 0)
		rec := httptest.NewRecorder()
		copyFixture.server.ServeHTTP(rec, req)
		close(copyDone)
	}()
	<-copyStarted
	copyCloseCtx, copyCloseCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer copyCloseCancel()
	if err := copyFixture.server.Close(copyCloseCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-copyDone:
	case <-copyCloseCtx.Done():
		t.Fatal("shutdown left blocked verified-copy handler")
	}
	if got, err := copyFixture.store.GetPublic(copyFixture.ctx, copyID); err != nil || got.File.ByteLength != int64(len(copyContent)) {
		t.Fatalf("shutdown closed caller-owned attachment store: %+v, %v", got, err)
	}
}

func TestMEDIA001M2CRangeAndHeaderContract(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 16 << 20})
	id := testReferenceID(70)
	content := bytes.Repeat([]byte("r"), (8<<20)+1)
	putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
	handle := createHandle(t, fixture, id, http.StatusCreated)
	path := "/v1/media/handles/" + handle.Handle
	baseCreateStage := fixture.server.media.createStage
	stageCalls := 0
	fixture.server.media.createStage = func(root *os.Root) (*os.File, string, error) {
		stageCalls++
		return baseCreateStage(root)
	}

	for _, test := range []struct {
		name   string
		values []string
		method string
		status int
	}{
		{name: "at_limit", values: []string{"bytes=0-8388607"}, method: http.MethodGet, status: http.StatusPartialContent},
		{name: "above_limit", values: []string{"bytes=0-8388608"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "suffix", values: []string{"bytes=-1"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "open", values: []string{"bytes=0-"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "reversed", values: []string{"bytes=2-1"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "overflow", values: []string{"bytes=0-999999999999999999999999"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "plus_sign", values: []string{"bytes=+0-1"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "space", values: []string{"bytes=0 -1"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "out_of_file", values: []string{"bytes=0-8388609"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "multiple", values: []string{"bytes=0-1,3-4"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "duplicate", values: []string{"bytes=0-1", "bytes=0-1"}, method: http.MethodGet, status: http.StatusRequestedRangeNotSatisfiable},
		{name: "head_range", values: []string{"bytes=0-1"}, method: http.MethodHead, status: http.StatusRequestedRangeNotSatisfiable},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := stageCalls
			res := mediaRequestWith(t, fixture, test.method, path, nil, func(req *http.Request) {
				req.Header["Range"] = test.values
			})
			body := readResponse(t, res)
			if res.StatusCode != test.status {
				t.Fatalf("status %d body %s", res.StatusCode, body)
			}
			if test.status == http.StatusRequestedRangeNotSatisfiable {
				if stageCalls != before {
					t.Fatal("invalid range triggered staging")
				}
				if got := res.Header.Get("Content-Range"); got != "bytes */"+decimal(int64(len(content))) {
					t.Fatalf("Content-Range = %q", got)
				}
			} else if int64(len(body)) != testRangeMaxBytes {
				t.Fatalf("range body = %d", len(body))
			}
		})
	}

	for _, header := range []string{"If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since", "If-Range"} {
		before := stageCalls
		res := mediaRequestWith(t, fixture, http.MethodGet, path, nil, func(req *http.Request) { req.Header.Set(header, "x") })
		assertHTTPError(t, res, http.StatusBadRequest, "BAD_REQUEST")
		if stageCalls != before {
			t.Fatalf("conditional header %s triggered staging", header)
		}
	}
}

func TestMEDIA001M2CRejectsCorruptBeforeServing(t *testing.T) {
	requireM2CLinux(t)
	fixture := newMediaFixture(t, attachment.Limits{MaxReferences: 4, MaxLogicalBytes: 8 << 20})
	id := testReferenceID(80)
	content := bytes.Repeat([]byte("late-corruption"), 100000)
	reference := putMedia(t, fixture, id, bytes.NewReader(content), int64(len(content)), false)
	corruptLateBlock(t, fixture.ctx, fixture.node, reference.CID)
	handle := createHandle(t, fixture, id, http.StatusCreated)
	res := mediaRequest(t, fixture, http.MethodGet, "/v1/media/handles/"+handle.Handle, nil)
	body := readResponse(t, res)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("corrupt download status=%d body_prefix=%q", res.StatusCode, body[:min(len(body), 64)])
	}
	if bytes.Contains(body, content[:64]) {
		t.Fatal("corrupt response exposed file bytes before verification completed")
	}
	assertJSONErrorBytes(t, body, "UNAVAILABLE")
}

type referenceResponse struct {
	V           int    `json:"v"`
	PeerID      string `json:"peer_id"`
	InstanceID  string `json:"instance_id"`
	ReferenceID string `json:"reference_id"`
	CID         string `json:"cid"`
	ByteLength  int64  `json:"byte_length"`
}

type handleResponse struct {
	V                int    `json:"v"`
	PeerID           string `json:"peer_id"`
	InstanceID       string `json:"instance_id"`
	ReferenceID      string `json:"reference_id"`
	Handle           string `json:"handle"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

func newMediaFixture(t *testing.T, limits attachment.Limits) *mediaFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	node, err := network.New(ctx, network.Config{
		Datastore:             dsync.MutexWrap(datastore.NewMapDatastore()),
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	store, err := attachment.Open(ctx, node, limits)
	if err != nil {
		_ = node.Close()
		cancel()
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	server, err := StartWithMedia(dataDir, node.Host.ID(), &mediaRecords{}, node, store)
	if err != nil {
		_ = store.Close()
		_ = node.Close()
		cancel()
		t.Fatal(err)
	}
	fixture := &mediaFixture{
		ctx:     ctx,
		cancel:  cancel,
		node:    node,
		store:   store,
		server:  server,
		desc:    readMediaDescriptor(t, dataDir),
		client:  m2cHTTPClient(),
		dataDir: dataDir,
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = server.Close(closeCtx)
		_ = store.Close()
		_ = node.Close()
		cancel()
	})
	return fixture
}

func requireM2CLinux(t testing.TB) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("authenticated local media access is Linux-only")
	}
}

func readMediaDescriptor(t testing.TB, dataDir string) descriptor {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, directoryName, descriptorName))
	if err != nil {
		t.Fatal(err)
	}
	var result descriptor
	decodeExactJSON(t, raw, &result)
	return result
}

func putMedia(t *testing.T, fixture *mediaFixture, id attachment.ReferenceID, body io.Reader, length int64, chunked bool) referenceResponse {
	t.Helper()
	path := "/v1/media/public/" + id.String()
	req, err := http.NewRequest(http.MethodPut, fixture.desc.Endpoint+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	authorizeMedia(req, fixture.desc)
	if length >= 0 {
		req.ContentLength = length
	}
	if chunked {
		req.ContentLength = -1
		req.TransferEncoding = []string{"chunked"}
	}
	res, err := fixture.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", res.StatusCode, raw)
	}
	return decodeReferenceResponse(t, raw)
}

func getReference(t *testing.T, fixture *mediaFixture, id attachment.ReferenceID, wantStatus int) referenceResponse {
	t.Helper()
	res := mediaRequest(t, fixture, http.MethodGet, "/v1/media/public/"+id.String(), nil)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("GET reference status=%d want=%d body=%s", res.StatusCode, wantStatus, raw)
	}
	if wantStatus != http.StatusOK {
		code := "NOT_FOUND"
		if wantStatus == http.StatusConflict {
			code = "NOT_READY"
		}
		assertJSONErrorBytes(t, raw, code)
		return referenceResponse{}
	}
	return decodeReferenceResponse(t, raw)
}

func createHandle(t *testing.T, fixture *mediaFixture, id attachment.ReferenceID, wantStatus int) handleResponse {
	t.Helper()
	res := mediaRequest(t, fixture, http.MethodPost, "/v1/media/public/"+id.String()+"/handles", nil)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("create handle status=%d want=%d body=%s", res.StatusCode, wantStatus, raw)
	}
	if wantStatus != http.StatusCreated {
		code := "BUSY"
		if wantStatus == http.StatusServiceUnavailable {
			code = "UNAVAILABLE"
		}
		assertJSONErrorBytes(t, raw, code)
		return handleResponse{}
	}
	var result handleResponse
	decodeExactJSON(t, raw, &result)
	if result.V != 1 || result.Handle == "" || result.ExpiresInSeconds != 300 {
		t.Fatalf("handle response = %+v", result)
	}
	return result
}

func deleteReference(t *testing.T, fixture *mediaFixture, id attachment.ReferenceID) {
	t.Helper()
	res := mediaRequest(t, fixture, http.MethodDelete, "/v1/media/public/"+id.String(), nil)
	defer res.Body.Close()
	if raw, err := io.ReadAll(res.Body); err != nil || res.StatusCode != http.StatusNoContent || len(raw) != 0 {
		t.Fatalf("DELETE status=%d body=%q err=%v", res.StatusCode, raw, err)
	}
}

func mediaRequest(t *testing.T, fixture *mediaFixture, method, path string, body io.Reader) *http.Response {
	t.Helper()
	return mediaRequestWith(t, fixture, method, path, body, nil)
}

func mediaRequestWith(t *testing.T, fixture *mediaFixture, method, path string, body io.Reader, mutate func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, fixture.desc.Endpoint+path, body)
	if err != nil {
		t.Fatal(err)
	}
	authorizeMedia(req, fixture.desc)
	if mutate != nil {
		mutate(req)
	}
	res, err := fixture.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func directMediaRequest(server *Server, method, target string, body io.ReadCloser, length int64) *http.Request {
	parsed, _ := url.ParseRequestURI(target)
	if parsed == nil {
		parsed = &url.URL{Path: target}
	}
	req := &http.Request{
		Method:        method,
		URL:           parsed,
		RequestURI:    target,
		Header:        make(http.Header),
		Body:          body,
		ContentLength: length,
		Host:          server.bound,
		RemoteAddr:    "127.0.0.1:9",
	}
	if body == nil {
		req.Body = http.NoBody
		req.ContentLength = 0
	}
	req.Header.Set("Authorization", "Bearer "+server.token)
	req.Header.Set("X-BitBook-Instance", server.instance)
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	return req
}

func authorizeMedia(req *http.Request, desc descriptor) {
	req.Header.Set("Authorization", "Bearer "+desc.Token)
	req.Header.Set("X-BitBook-Instance", desc.InstanceID)
}

func m2cHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
	}
}

func decodeReferenceResponse(t testing.TB, raw []byte) referenceResponse {
	t.Helper()
	var result referenceResponse
	decodeExactJSON(t, raw, &result)
	if result.V != 1 || result.PeerID == "" || result.InstanceID == "" || result.ReferenceID == "" || result.CID == "" || result.ByteLength < 0 {
		t.Fatalf("reference response = %+v", result)
	}
	return result
}

func decodeExactJSON(t testing.TB, raw []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode JSON %q: %v", raw, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing JSON %q: %v", raw, err)
	}
}

func readResponse(t testing.TB, res *http.Response) []byte {
	t.Helper()
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertDownloadHeaders(t testing.TB, res *http.Response, length int64, status int) {
	t.Helper()
	for name, want := range map[string]string{
		"Content-Type":                 "application/octet-stream",
		"Content-Disposition":          "attachment",
		"X-Content-Type-Options":       "nosniff",
		"Cache-Control":                "no-store",
		"Content-Security-Policy":      "sandbox",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := res.Header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS response header %q", got)
	}
	if res.StatusCode != status || res.ContentLength != length {
		t.Fatalf("status=%d Content-Length=%d, want %d/%d", res.StatusCode, res.ContentLength, status, length)
	}
}

func assertHTTPError(t testing.TB, res *http.Response, status int, code string) {
	t.Helper()
	raw := readResponse(t, res)
	if res.StatusCode != status {
		t.Fatalf("status=%d want=%d body=%s", res.StatusCode, status, raw)
	}
	if res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("error security headers = %v", res.Header)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS response header %q", got)
	}
	assertJSONErrorBytes(t, raw, code)
}

func assertRecorderError(t testing.TB, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.Bytes())
	}
	assertJSONErrorBytes(t, rec.Body.Bytes(), code)
}

func assertJSONErrorBytes(t testing.TB, raw []byte, code string) {
	t.Helper()
	var body errorBody
	decodeExactJSON(t, raw, &body)
	if body.Error != code {
		t.Fatalf("error=%q want=%q raw=%s", body.Error, code, raw)
	}
}

func testReferenceID(value byte) attachment.ReferenceID {
	var id attachment.ReferenceID
	for i := range id {
		id[i] = value
	}
	return id
}

func independentPeerID(t testing.TB) peer.ID {
	t.Helper()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func corruptLateBlock(t *testing.T, ctx context.Context, node *network.Node, cidText string) {
	t.Helper()
	root, err := node.Blockstore.Get(ctx, mustDecodeCID(t, cidText))
	if err != nil {
		t.Fatal(err)
	}
	rootNode, err := merkledag.DecodeProtobuf(root.RawData())
	if err != nil {
		t.Fatal(err)
	}
	links := rootNode.Links()
	if len(links) < 2 {
		t.Fatalf("corruption fixture has %d root links, want multiple", len(links))
	}
	late, err := node.Blockstore.Get(ctx, links[len(links)-1].Cid)
	if err != nil {
		t.Fatal(err)
	}
	results, err := node.Datastore.Query(ctx, query.Query{Prefix: "/blocks"})
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	for result := range results.Next() {
		if result.Error == nil && bytes.Equal(result.Value, late.RawData()) {
			if err := node.Datastore.Put(ctx, datastore.NewKey(result.Key), []byte("corrupt late block")); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("late block datastore record not found")
}

func mustDecodeCID(t testing.TB, encoded string) cid.Cid {
	t.Helper()
	id, err := cid.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func decimal(value int64) string { return strconv.FormatInt(value, 10) }

type countingBody struct {
	reader io.Reader
	reads  int
}

func (b *countingBody) Read(p []byte) (int, error) {
	b.reads++
	return b.reader.Read(p)
}

func (*countingBody) Close() error { return nil }

type unknownLengthReader struct{ reader io.Reader }

func (r *unknownLengthReader) Read(p []byte) (int, error) { return r.reader.Read(p) }

type blockingReadCloser struct {
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{started: make(chan struct{}), release: make(chan struct{})}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.release
	return 0, io.ErrClosedPipe
}

func (r *blockingReadCloser) Close() error {
	r.closeOnce.Do(func() { close(r.release) })
	return nil
}

type unexpectedEOFBody struct {
	content []byte
	done    bool
}

func (b *unexpectedEOFBody) Read(p []byte) (int, error) {
	if b.done {
		return 0, io.ErrUnexpectedEOF
	}
	b.done = true
	return copy(p, b.content), io.ErrUnexpectedEOF
}

func (*unexpectedEOFBody) Close() error { return nil }

type blockingDeadlineResponseWriter struct {
	header       http.Header
	writeStarted chan struct{}
	release      chan struct{}
	startOnce    sync.Once
	releaseOnce  sync.Once
	status       int
}

func newBlockingDeadlineResponseWriter() *blockingDeadlineResponseWriter {
	return &blockingDeadlineResponseWriter{
		header:       make(http.Header),
		writeStarted: make(chan struct{}),
		release:      make(chan struct{}),
	}
}

func (w *blockingDeadlineResponseWriter) Header() http.Header { return w.header }

func (w *blockingDeadlineResponseWriter) WriteHeader(status int) { w.status = status }

func (w *blockingDeadlineResponseWriter) Write([]byte) (int, error) {
	w.startOnce.Do(func() { close(w.writeStarted) })
	<-w.release
	return 0, os.ErrDeadlineExceeded
}

func (w *blockingDeadlineResponseWriter) SetWriteDeadline(time.Time) error {
	w.unblock()
	return nil
}

func (w *blockingDeadlineResponseWriter) unblock() {
	w.releaseOnce.Do(func() { close(w.release) })
}

func waitForMediaActivity(t testing.TB, media *mediaService, slots, bodies int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		gotSlots, gotBodies := currentMediaActivity(media)
		if gotSlots == slots && gotBodies == bodies {
			time.Sleep(20 * time.Millisecond)
			assertMediaActivity(t, media, slots, bodies)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("media activity slots=%d bodies=%d, want %d/%d", gotSlots, gotBodies, slots, bodies)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertMediaActivity(t testing.TB, media *mediaService, slots, bodies int) {
	t.Helper()
	gotSlots, gotBodies := currentMediaActivity(media)
	if gotSlots != slots || gotBodies != bodies {
		t.Fatalf("media activity slots=%d bodies=%d, want %d/%d", gotSlots, gotBodies, slots, bodies)
	}
}

func currentMediaActivity(media *mediaService) (int, int) {
	media.workMu.Lock()
	defer media.workMu.Unlock()
	return len(media.slots), len(media.bodies)
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
