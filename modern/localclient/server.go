package localclient

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/payment"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	directoryName     = "local-client"
	descriptorName    = "connection.json"
	recordsPath       = "/v1/payment/records"
	maxSuccessJSON    = 4 << 20
	maxHeaderBytes    = 16 << 10
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 30 * time.Second
)

// ErrUnavailable is returned when this host cannot offer the local payment
// access channel. Linux is the only platform with a defined permission contract.
var ErrUnavailable = errors.New("local payment access unavailable")

// RecordReader loads stored payment objects. payment.Service satisfies it.
type RecordReader interface {
	List(ctx context.Context) ([]payment.RecordedObject, error)
}

// Server is the loopback payment-record listener for a single daemon run.
type Server struct {
	http     *http.Server
	listener net.Listener
	root     *os.Root
	records  RecordReader
	media    *mediaService
	failures chan error

	bound     string
	peerID    string
	instance  string
	token     string
	tmpName   string
	published os.FileInfo

	closeOnce sync.Once
}

type descriptor struct {
	V          int    `json:"v"`
	Endpoint   string `json:"endpoint"`
	PeerID     string `json:"peer_id"`
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
}

type successBody struct {
	V          int                      `json:"v"`
	PeerID     string                   `json:"peer_id"`
	InstanceID string                   `json:"instance_id"`
	Records    []payment.RecordedObject `json:"records"`
}

type errorBody struct {
	Error string `json:"error"`
}

// Start binds 127.0.0.1:0, publishes a private descriptor, and serves records.
func Start(dataDir string, peerID peer.ID, records RecordReader) (*Server, error) {
	return start(dataDir, peerID, records, nil, nil)
}

// StartWithMedia starts the authenticated local listener with public attachment
// upload and bounded download handling in addition to payment records.
func StartWithMedia(dataDir string, peerID peer.ID, records RecordReader, node *network.Node, store *attachment.Store) (*Server, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrUnavailable
	}
	if node == nil || node.Host == nil {
		return nil, errors.New("nil media network node")
	}
	if store == nil {
		return nil, errors.New("nil attachment store")
	}
	if peerID == "" || node.Host.ID() != peerID {
		return nil, errors.New("media node peer identity mismatch")
	}
	return start(dataDir, peerID, records, node, store)
}

func start(dataDir string, peerID peer.ID, records RecordReader, node *network.Node, store *attachment.Store) (*Server, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrUnavailable
	}
	if records == nil {
		return nil, errors.New("nil payment record reader")
	}
	if peerID == "" {
		return nil, errors.New("empty peer id")
	}
	dir, err := prepareClientDir(dataDir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening local-client directory: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("binding local payment access: %w", err)
	}
	token, err := randomHex(32)
	if err != nil {
		_ = ln.Close()
		_ = root.Close()
		return nil, err
	}
	instance, err := randomHex(16)
	if err != nil {
		_ = ln.Close()
		_ = root.Close()
		return nil, err
	}
	server := &Server{
		listener: ln,
		root:     root,
		records:  records,
		failures: make(chan error, 1),
		bound:    ln.Addr().String(),
		peerID:   peerID.String(),
		instance: instance,
		token:    token,
	}
	if node != nil {
		server.media = newMediaService(node, store, root)
	}
	server.http = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	if err := server.publishDescriptor(); err != nil {
		if server.media != nil {
			server.media.cancelAndCloseBodies()
			server.media.wait()
		}
		_ = ln.Close()
		_ = root.Close()
		return nil, err
	}
	go func() {
		server.failures <- server.http.Serve(ln)
	}()
	return server, nil
}

// Failures receives the Serve result. http.ErrServerClosed is a normal stop.
func (s *Server) Failures() <-chan error {
	if s == nil {
		return nil
	}
	return s.failures
}

// Close stops handlers, then removes this run's descriptor and owned temp file.
func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		if s.media != nil {
			s.media.cancelAndCloseBodies()
		}
		if s.http != nil {
			shutdownErr := s.http.Shutdown(ctx)
			if shutdownErr != nil {
				_ = s.http.Close()
				if ctx.Err() != nil {
					err = errors.Join(err, shutdownErr)
				} else if !errors.Is(shutdownErr, http.ErrServerClosed) {
					err = errors.Join(err, shutdownErr)
				}
			}
		} else if s.listener != nil {
			err = errors.Join(err, s.listener.Close())
		}
		if s.media != nil {
			s.media.wait()
		}
		if s.root != nil {
			if s.tmpName != "" {
				_ = s.root.Remove(s.tmpName)
				s.tmpName = ""
			}
			if rmErr := s.removeOwnedDescriptor(); rmErr != nil {
				err = errors.Join(err, rmErr)
			}
			err = errors.Join(err, s.root.Close())
			s.root = nil
		}
	})
	return err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status := localRequestAuthorized(s, r); status != 0 {
		if status == http.StatusUnauthorized {
			writeError(w, status, "UNAUTHORIZED")
		} else {
			writeError(w, status, "FORBIDDEN")
		}
		return
	}
	mediaRoute, routeErr := parseMediaRoute(r)
	if routeErr != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	if mediaRoute.kind != mediaRouteNone {
		if s.media == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		s.media.serve(w, r, mediaRoute, s)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	if r.URL.Path != recordsPath {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || hasRequestBody(r) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()
	records, err := s.records.List(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	if records == nil {
		records = []payment.RecordedObject{}
	}
	payload, err := json.Marshal(successBody{
		V:          1,
		PeerID:     s.peerID,
		InstanceID: s.instance,
		Records:    records,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}
	if len(payload) > maxSuccessJSON {
		writeError(w, http.StatusServiceUnavailable, "TOO_LARGE")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func localRequestAuthorized(s *Server, r *http.Request) int {
	if s == nil || r == nil || !loopbackAddr(r.RemoteAddr) || r.Host != s.bound {
		return http.StatusForbidden
	}
	if len(r.Header.Values("Origin")) > 0 {
		return http.StatusForbidden
	}
	if !s.authorized(r) {
		return http.StatusUnauthorized
	}
	return 0
}

func (s *Server) authorized(r *http.Request) bool {
	auth := r.Header.Values("Authorization")
	instance := r.Header.Values("X-BitBook-Instance")
	gotToken := ""
	if len(auth) == 1 && strings.HasPrefix(auth[0], "Bearer ") {
		gotToken = auth[0][len("Bearer "):]
	}
	gotInstance := ""
	if len(instance) == 1 {
		gotInstance = instance[0]
	}
	tokenOK := subtle.ConstantTimeCompare([]byte(gotToken), []byte(s.token)) == 1
	instanceOK := subtle.ConstantTimeCompare([]byte(gotInstance), []byte(s.instance)) == 1
	return len(auth) == 1 && len(instance) == 1 && tokenOK && instanceOK
}

func (s *Server) publishDescriptor() error {
	if err := refuseNonRegularDescriptor(s.root); err != nil {
		return err
	}
	payload, err := json.Marshal(descriptor{
		V:          1,
		Endpoint:   "http://" + s.bound,
		PeerID:     s.peerID,
		InstanceID: s.instance,
		Token:      s.token,
	})
	if err != nil {
		return err
	}
	tmp, name, err := createDescriptorTemp(s.root)
	if err != nil {
		return err
	}
	s.tmpName = name
	if err := s.root.Chmod(name, 0o600); err != nil {
		_ = tmp.Close()
		_ = s.root.Remove(name)
		s.tmpName = ""
		return fmt.Errorf("securing local-client descriptor: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		_ = s.root.Remove(name)
		s.tmpName = ""
		return fmt.Errorf("writing local-client descriptor: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = s.root.Remove(name)
		s.tmpName = ""
		return fmt.Errorf("syncing local-client descriptor: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = s.root.Remove(name)
		s.tmpName = ""
		return fmt.Errorf("closing local-client descriptor: %w", err)
	}
	if err := refuseNonRegularDescriptor(s.root); err != nil {
		_ = s.root.Remove(name)
		s.tmpName = ""
		return err
	}
	if err := s.root.Rename(name, descriptorName); err != nil {
		_ = s.root.Remove(name)
		s.tmpName = ""
		return fmt.Errorf("publishing local-client descriptor: %w", err)
	}
	s.tmpName = ""
	info, err := s.root.Lstat(descriptorName)
	if err != nil {
		return fmt.Errorf("recording local-client descriptor identity: %w", err)
	}
	s.published = info
	return nil
}

func (s *Server) removeOwnedDescriptor() error {
	if s.root == nil || s.published == nil {
		return nil
	}
	info, err := s.root.Lstat(descriptorName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil
	}
	if !os.SameFile(s.published, info) {
		return nil
	}
	if err := s.root.Remove(descriptorName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func prepareClientDir(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("empty data directory")
	}
	path := filepath.Join(dataDir, directoryName)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return "", fmt.Errorf("creating local-client directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return "", fmt.Errorf("inspecting local-client directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("local-client is a symlink")
	}
	if !info.IsDir() {
		return "", errors.New("local-client is not a directory")
	}
	if info.Mode().Perm() != 0o700 {
		return "", errors.New("local-client directory is not private")
	}
	return path, nil
}

func createDescriptorTemp(root *os.Root) (*os.File, string, error) {
	for range 10000 {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, "", fmt.Errorf("creating local-client descriptor: %w", err)
		}
		name := fmt.Sprintf(".connection-%x", suffix)
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", fmt.Errorf("creating local-client descriptor: %w", err)
		}
	}
	return nil, "", errors.New("creating local-client descriptor: all candidate names exist")
}

func refuseNonRegularDescriptor(root *os.Root) error {
	info, err := root.Lstat(descriptorName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspecting local-client descriptor: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("connection.json is not a regular file")
	}
	return nil
}

func randomHex(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating local-client credential: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func loopbackAddr(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hasRequestBody(r *http.Request) bool {
	if r.ContentLength != 0 {
		return true
	}
	return len(r.TransferEncoding) > 0
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if status == http.StatusMethodNotAllowed && w.Header().Get("Allow") == "" {
		w.Header().Set("Allow", http.MethodGet)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: code})
}
