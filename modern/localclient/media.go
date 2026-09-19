package localclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
)

const (
	maxMediaBytes         = int64(100 << 20)
	maxMediaRangeBytes    = int64(8 << 20)
	maxMediaHandles       = 128
	mediaHandleLifetime   = 5 * time.Minute
	mediaOperationTimeout = 10 * time.Second
)

var (
	errBadMediaRequest = errors.New("bad media request")
	errMediaTooLarge   = errors.New("media upload too large")
	errMediaLength     = errors.New("media upload length mismatch")
)

type mediaRouteKind uint8

const (
	mediaRouteNone mediaRouteKind = iota
	mediaRouteReference
	mediaRouteCreateHandle
	mediaRouteDownload
)

type mediaRoute struct {
	kind        mediaRouteKind
	referenceID string
	handle      string
}

type mediaHandle struct {
	referenceID attachment.ReferenceID
	expiresAt   time.Time
}

type mediaService struct {
	node  *network.Node
	store *attachment.Store
	root  *os.Root

	ctx    context.Context
	cancel context.CancelFunc

	workMu  sync.Mutex
	closing bool
	work    sync.WaitGroup
	slots   chan struct{}
	bodies  map[io.ReadCloser]struct{}

	mu      sync.Mutex
	handles map[string]mediaHandle
	now     func() time.Time
	random  io.Reader

	copyPublic      func(context.Context, network.PublicFile, io.Writer) (int64, error)
	createStage     func(*os.Root) (*os.File, string, error)
	unlinkStage     func(*os.Root, string) error
	wrapStageWriter func(io.Writer) io.Writer
}

type mediaReferenceResponse struct {
	V           int    `json:"v"`
	PeerID      string `json:"peer_id"`
	InstanceID  string `json:"instance_id"`
	ReferenceID string `json:"reference_id"`
	CID         string `json:"cid"`
	ByteLength  int64  `json:"byte_length"`
}

type mediaHandleResponse struct {
	V                int    `json:"v"`
	PeerID           string `json:"peer_id"`
	InstanceID       string `json:"instance_id"`
	ReferenceID      string `json:"reference_id"`
	Handle           string `json:"handle"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

type mediaByteRange struct {
	start int64
	end   int64
}

func newMediaService(node *network.Node, store *attachment.Store, root *os.Root) *mediaService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &mediaService{
		node:       node,
		store:      store,
		root:       root,
		ctx:        ctx,
		cancel:     cancel,
		slots:      make(chan struct{}, 2),
		bodies:     make(map[io.ReadCloser]struct{}),
		handles:    make(map[string]mediaHandle),
		now:        time.Now,
		random:     rand.Reader,
		copyPublic: node.CopyPublicFile,
	}
	service.createStage = service.defaultCreateStage
	service.unlinkStage = func(root *os.Root, name string) error { return root.Remove(name) }
	service.wrapStageWriter = func(writer io.Writer) io.Writer { return writer }
	return service
}

func parseMediaRoute(r *http.Request) (mediaRoute, error) {
	if r == nil || r.URL == nil {
		return mediaRoute{}, nil
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/v1/media") {
		return mediaRoute{}, nil
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.URL.Fragment != "" || r.URL.RawPath != "" ||
		strings.Contains(path, `\`) || strings.Contains(path, "//") || strings.Contains(path, "/./") || strings.Contains(path, "/../") ||
		strings.Contains(r.RequestURI, "%") || strings.Contains(r.RequestURI, "#") {
		return mediaRoute{}, errBadMediaRequest
	}
	parts := strings.Split(path, "/")
	if len(parts) == 5 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "public" && canonicalMediaID(parts[4]) {
		return mediaRoute{kind: mediaRouteReference, referenceID: parts[4]}, nil
	}
	if len(parts) == 6 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "public" && canonicalMediaID(parts[4]) && parts[5] == "handles" {
		return mediaRoute{kind: mediaRouteCreateHandle, referenceID: parts[4]}, nil
	}
	if len(parts) == 5 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "handles" && canonicalMediaID(parts[4]) {
		return mediaRoute{kind: mediaRouteDownload, handle: parts[4]}, nil
	}
	return mediaRoute{}, errBadMediaRequest
}

func canonicalMediaID(value string) bool {
	if len(value) != 32 || value == strings.Repeat("0", 32) {
		return false
	}
	for i := range len(value) {
		if (value[i] < '0' || value[i] > '9') && (value[i] < 'a' || value[i] > 'f') {
			return false
		}
	}
	return true
}

func (m *mediaService) serve(w http.ResponseWriter, r *http.Request, route mediaRoute, server *Server) {
	switch route.kind {
	case mediaRouteReference:
		id, err := attachment.ParseReferenceID(route.referenceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST")
			return
		}
		switch r.Method {
		case http.MethodPut:
			m.serveUpload(w, r, server, id)
		case http.MethodGet:
			m.serveReference(w, r, server, id)
		case http.MethodDelete:
			m.serveRelease(w, r, id)
		default:
			w.Header().Set("Allow", "DELETE, GET, PUT")
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		}
	case mediaRouteCreateHandle:
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
			return
		}
		id, err := attachment.ParseReferenceID(route.referenceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST")
			return
		}
		m.serveCreateHandle(w, r, server, id)
	case mediaRouteDownload:
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
			return
		}
		m.serveDownload(w, r, route.handle)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	}
}

func (m *mediaService) serveUpload(w http.ResponseWriter, r *http.Request, server *Server, id attachment.ReferenceID) {
	maximum, knownLength, err := validateMediaUpload(r)
	if err != nil {
		if errors.Is(err, errMediaTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "TOO_LARGE")
		} else if errors.Is(err, errUnsupportedMediaType) {
			writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
		} else {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST")
		}
		return
	}
	ctx, cancel := m.operationContext(r.Context())
	defer cancel()
	existing, getErr := m.store.GetPublic(ctx, id)
	if getErr == nil {
		writeMediaJSON(w, http.StatusOK, mediaReferencePayload(server, existing))
		return
	}
	if errors.Is(getErr, attachment.ErrNotReady) {
		writeError(w, http.StatusConflict, "NOT_READY")
		return
	}
	if !errors.Is(getErr, attachment.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	done, ok := m.beginWork(true)
	if !ok {
		m.writeAdmissionError(w)
		return
	}
	defer done()
	body := &ownedRequestBody{ReadCloser: r.Body}
	if !m.registerBody(body) {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	defer m.unregisterBody(body)
	defer body.Close()
	stopBodyInterrupt := startMediaInterrupt(ctx, func() {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now())
		_ = body.Close()
	})
	defer stopBodyInterrupt()

	var source io.Reader = body
	if knownLength {
		source = &exactLengthReader{source: body, remaining: r.ContentLength}
	}
	reference, err := m.store.ImportPublic(ctx, id, source, maximum)
	if err != nil {
		writeMediaOperationError(w, err)
		return
	}
	writeMediaJSON(w, http.StatusOK, mediaReferencePayload(server, reference))
}

var errUnsupportedMediaType = errors.New("unsupported media type")

func validateMediaUpload(r *http.Request) (int64, bool, error) {
	if r == nil {
		return 0, false, errBadMediaRequest
	}
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) != 1 || contentTypes[0] != "application/octet-stream" {
		return 0, false, errUnsupportedMediaType
	}
	if len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Range")) != 0 {
		return 0, false, errBadMediaRequest
	}
	if r.ContentLength < -1 {
		return 0, false, errBadMediaRequest
	}
	if r.ContentLength == -1 {
		if len(r.TransferEncoding) != 1 || r.TransferEncoding[0] != "chunked" {
			return 0, false, errBadMediaRequest
		}
	} else if len(r.TransferEncoding) != 0 {
		return 0, false, errBadMediaRequest
	}
	if r.ContentLength > maxMediaBytes {
		return 0, true, errMediaTooLarge
	}
	if r.ContentLength >= 0 {
		if r.ContentLength == 0 {
			return 1, true, nil
		}
		return r.ContentLength, true, nil
	}
	return maxMediaBytes, false, nil
}

func (m *mediaService) serveReference(w http.ResponseWriter, r *http.Request, server *Server, id attachment.ReferenceID) {
	if rejectMediaBody(w, r) {
		return
	}
	done, ok := m.beginWork(false)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	defer done()
	ctx, cancel := m.operationContext(r.Context())
	defer cancel()
	reference, err := m.store.GetPublic(ctx, id)
	if err != nil {
		writeMediaLookupError(w, err)
		return
	}
	writeMediaJSON(w, http.StatusOK, mediaReferencePayload(server, reference))
}

func (m *mediaService) serveRelease(w http.ResponseWriter, r *http.Request, id attachment.ReferenceID) {
	if rejectMediaBody(w, r) {
		return
	}
	done, ok := m.beginWork(false)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	defer done()
	ctx, cancel := m.operationContext(r.Context())
	defer cancel()
	m.mu.Lock()
	err := m.store.Release(ctx, id)
	if err == nil {
		for handle, entry := range m.handles {
			if entry.referenceID == id {
				delete(m.handles, handle)
			}
		}
	}
	m.mu.Unlock()
	if err != nil {
		writeMediaOperationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (m *mediaService) serveCreateHandle(w http.ResponseWriter, r *http.Request, server *Server, id attachment.ReferenceID) {
	if rejectMediaBody(w, r) {
		return
	}
	done, ok := m.beginWork(false)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	defer done()
	ctx, cancel := m.operationContext(r.Context())
	defer cancel()

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.purgeExpiredLocked(now)
	if len(m.handles) >= maxMediaHandles {
		writeError(w, http.StatusTooManyRequests, "BUSY")
		return
	}
	reference, err := m.store.GetPublic(ctx, id)
	if err != nil {
		writeMediaLookupError(w, err)
		return
	}
	handle, err := m.newHandleLocked()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	m.handles[handle] = mediaHandle{referenceID: id, expiresAt: now.Add(mediaHandleLifetime)}
	writeMediaJSON(w, http.StatusCreated, mediaHandleResponse{
		V:                1,
		PeerID:           server.peerID,
		InstanceID:       server.instance,
		ReferenceID:      reference.ID.String(),
		Handle:           handle,
		ExpiresInSeconds: int64(mediaHandleLifetime / time.Second),
	})
}

func (m *mediaService) serveDownload(w http.ResponseWriter, r *http.Request, handle string) {
	if rejectMediaBody(w, r) {
		return
	}
	if hasConditionalMediaHeader(r) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	done, ok := m.beginWork(true)
	if !ok {
		m.writeAdmissionError(w)
		return
	}
	defer done()
	ctx, cancel := m.operationContext(r.Context())
	defer cancel()
	stopWriteInterrupt := startMediaInterrupt(ctx, func() {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now())
	})
	defer stopWriteInterrupt()

	m.mu.Lock()
	entry, exists := m.handles[handle]
	if exists && !m.now().Before(entry.expiresAt) {
		delete(m.handles, handle)
		exists = false
	}
	var reference attachment.PublicReference
	var err error
	if exists {
		reference, err = m.store.GetPublic(ctx, entry.referenceID)
	}
	m.mu.Unlock()
	if !exists || errors.Is(err, attachment.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	if err != nil {
		writeMediaLookupError(w, err)
		return
	}

	requested, hasRange, rangeErr := parseMediaRange(r.Header.Values("Range"), reference.File.ByteLength, r.Method == http.MethodHead)
	if rangeErr != nil {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(reference.File.ByteLength, 10))
		writeError(w, http.StatusRequestedRangeNotSatisfiable, "BAD_REQUEST")
		return
	}
	staged, err := m.stageVerified(ctx, reference.File)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	defer staged.Close()
	if ctx.Err() != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}

	start := int64(0)
	length := reference.File.ByteLength
	status := http.StatusOK
	if hasRange {
		start = requested.start
		length = requested.end - requested.start + 1
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", requested.start, requested.end, reference.File.ByteLength))
	}
	if _, err := staged.Seek(start, io.SeekStart); err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	if ctx.Err() != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	setDownloadHeaders(w.Header(), length)
	w.WriteHeader(status)
	if r.Method == http.MethodHead || length == 0 {
		return
	}
	_ = copyMediaResponse(ctx, w, staged, length)
}

func (m *mediaService) writeAdmissionError(w http.ResponseWriter) {
	if m.ctx.Err() != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	writeError(w, http.StatusTooManyRequests, "BUSY")
}

func (m *mediaService) beginWork(admitted bool) (func(), bool) {
	m.workMu.Lock()
	defer m.workMu.Unlock()
	if m.closing {
		return nil, false
	}
	if admitted {
		select {
		case m.slots <- struct{}{}:
		default:
			return nil, false
		}
	}
	m.work.Add(1)
	return func() {
		if admitted {
			<-m.slots
		}
		m.work.Done()
	}, true
}

func (m *mediaService) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, mediaOperationTimeout)
	stop := context.AfterFunc(m.ctx, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

func (m *mediaService) registerBody(body io.ReadCloser) bool {
	m.workMu.Lock()
	defer m.workMu.Unlock()
	if m.closing {
		_ = body.Close()
		return false
	}
	m.bodies[body] = struct{}{}
	return true
}

func (m *mediaService) unregisterBody(body io.ReadCloser) {
	m.workMu.Lock()
	delete(m.bodies, body)
	m.workMu.Unlock()
}

func (m *mediaService) cancelAndCloseBodies() {
	m.workMu.Lock()
	if !m.closing {
		m.closing = true
		m.cancel()
	}
	m.workMu.Unlock()
}

func (m *mediaService) wait() { m.work.Wait() }

func (m *mediaService) purgeExpiredLocked(now time.Time) {
	for handle, entry := range m.handles {
		if !now.Before(entry.expiresAt) {
			delete(m.handles, handle)
		}
	}
}

func (m *mediaService) newHandleLocked() (string, error) {
	for range maxMediaHandles + 1 {
		var value [16]byte
		if _, err := io.ReadFull(m.random, value[:]); err != nil {
			return "", err
		}
		if value == ([16]byte{}) {
			return "", errors.New("random source returned zero handle")
		}
		encoded := hex.EncodeToString(value[:])
		if _, exists := m.handles[encoded]; !exists {
			return encoded, nil
		}
	}
	return "", errors.New("unable to allocate unique media handle")
}

func (m *mediaService) stageVerified(ctx context.Context, file network.PublicFile) (*os.File, error) {
	staged, name, err := m.createStage(m.root)
	if err != nil {
		return nil, err
	}
	if err := m.unlinkStage(m.root, name); err != nil {
		_ = staged.Close()
		_ = m.root.Remove(name)
		return nil, err
	}
	bounded := &boundedStageWriter{writer: staged, remaining: file.ByteLength}
	written, err := m.copyPublic(ctx, file, m.wrapStageWriter(bounded))
	if err != nil || written != file.ByteLength || bounded.written != file.ByteLength || bounded.remaining != 0 {
		_ = staged.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("verified media copy length mismatch")
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return nil, err
	}
	info, err := staged.Stat()
	if err != nil || info.Size() != file.ByteLength {
		_ = staged.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("staged media length mismatch")
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		_ = staged.Close()
		return nil, err
	}
	return staged, nil
}

func (m *mediaService) defaultCreateStage(root *os.Root) (*os.File, string, error) {
	for range 10000 {
		var suffix [8]byte
		if _, err := io.ReadFull(m.random, suffix[:]); err != nil {
			return nil, "", err
		}
		name := fmt.Sprintf(".media-%x", suffix)
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if chmodErr := root.Chmod(name, 0o600); chmodErr != nil {
				_ = file.Close()
				_ = root.Remove(name)
				return nil, "", chmodErr
			}
			return file, name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("unable to allocate staging file")
}

type boundedStageWriter struct {
	writer    io.Writer
	remaining int64
	written   int64
}

func (w *boundedStageWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, errors.New("staged media exceeds declared length")
	}
	n, err := w.writer.Write(p)
	w.written += int64(n)
	w.remaining -= int64(n)
	if n != len(p) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err
}

type exactLengthReader struct {
	source    io.Reader
	remaining int64
}

func (r *exactLengthReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		var probe [1]byte
		n, err := r.source.Read(probe[:])
		if n > 0 {
			return 0, errMediaLength
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			err = io.EOF
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.source.Read(p)
	r.remaining -= int64(n)
	if (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) && r.remaining != 0 {
		err = errMediaLength
	} else if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

type ownedRequestBody struct {
	io.ReadCloser
	closeOnce sync.Once
	closeErr  error
}

func (b *ownedRequestBody) Close() error {
	b.closeOnce.Do(func() { b.closeErr = b.ReadCloser.Close() })
	return b.closeErr
}

func startMediaInterrupt(ctx context.Context, interrupt func()) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		interrupt()
	})
	return func() {
		if stop() {
			return
		}
		<-done
	}
}

func copyMediaResponse(ctx context.Context, destination io.Writer, source io.Reader, length int64) error {
	buffer := make([]byte, 32<<10)
	remaining := length
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		want := int64(len(buffer))
		if remaining < want {
			want = remaining
		}
		read, readErr := io.ReadFull(source, buffer[:want])
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			remaining -= int64(written)
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func parseMediaRange(values []string, length int64, head bool) (mediaByteRange, bool, error) {
	if len(values) == 0 {
		return mediaByteRange{}, false, nil
	}
	if len(values) != 1 || head || length == 0 {
		return mediaByteRange{}, false, errBadMediaRequest
	}
	value := values[0]
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return mediaByteRange{}, false, errBadMediaRequest
	}
	endpoints := strings.Split(value[len("bytes="):], "-")
	if len(endpoints) != 2 || endpoints[0] == "" || endpoints[1] == "" {
		return mediaByteRange{}, false, errBadMediaRequest
	}
	start, err := parseMediaRangeEndpoint(endpoints[0])
	if err != nil {
		return mediaByteRange{}, false, errBadMediaRequest
	}
	end, err := parseMediaRangeEndpoint(endpoints[1])
	if err != nil || start > end || end >= uint64(length) || end-start+1 > uint64(maxMediaRangeBytes) || start > math.MaxInt64 || end > math.MaxInt64 {
		return mediaByteRange{}, false, errBadMediaRequest
	}
	return mediaByteRange{start: int64(start), end: int64(end)}, true, nil
}

func parseMediaRangeEndpoint(value string) (uint64, error) {
	if value == "" {
		return 0, errBadMediaRequest
	}
	for i := range len(value) {
		if value[i] < '0' || value[i] > '9' {
			return 0, errBadMediaRequest
		}
	}
	return strconv.ParseUint(value, 10, 63)
}

func hasConditionalMediaHeader(r *http.Request) bool {
	for _, name := range []string{"If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since", "If-Range"} {
		if len(r.Header.Values(name)) != 0 {
			return true
		}
	}
	return false
}

func rejectMediaBody(w http.ResponseWriter, r *http.Request) bool {
	if hasRequestBody(r) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST")
		return true
	}
	return false
}

func mediaReferencePayload(server *Server, reference attachment.PublicReference) mediaReferenceResponse {
	return mediaReferenceResponse{
		V:           1,
		PeerID:      server.peerID,
		InstanceID:  server.instance,
		ReferenceID: reference.ID.String(),
		CID:         reference.File.CID.String(),
		ByteLength:  reference.File.ByteLength,
	}
}

func writeMediaJSON(w http.ResponseWriter, status int, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func writeMediaLookupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, attachment.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, attachment.ErrNotReady):
		writeError(w, http.StatusConflict, "NOT_READY")
	default:
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
	}
}

func writeMediaOperationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errMediaLength):
		writeError(w, http.StatusBadRequest, "BAD_REQUEST")
	case errors.Is(err, network.ErrPublicFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "TOO_LARGE")
	case errors.Is(err, attachment.ErrQuotaExceeded):
		writeError(w, http.StatusInsufficientStorage, "QUOTA_EXCEEDED")
	case errors.Is(err, attachment.ErrNotReady), errors.Is(err, attachment.ErrConflict):
		writeError(w, http.StatusConflict, "NOT_READY")
	default:
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
	}
}

func setDownloadHeaders(header http.Header, length int64) {
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", "attachment")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", "sandbox")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Content-Length", strconv.FormatInt(length, 10))
}
