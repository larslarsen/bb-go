package network

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	leveldb "github.com/ipfs/go-ds-leveldb"
)

const datastoreDirectory = "datastore"

// PersistentNode owns the disk-backed datastore in addition to its network
// services. Use Open for daemon processes and New for injected/test stores.
type PersistentNode struct {
	*Node
	store *leveldb.Datastore

	closeOnce sync.Once
	closeErr  error
}

// Open creates or opens a persistent node rooted at dataDir. Existing peer
// identities remain stable across restarts.
func Open(ctx context.Context, dataDir string, cfg Config) (*PersistentNode, error) {
	if dataDir == "" {
		return nil, errors.New("empty data directory")
	}
	if cfg.Datastore != nil {
		return nil, errors.New("Open manages its datastore; use New to inject one")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	key, err := LoadOrCreatePrivateKey(dataDir)
	if err != nil {
		return nil, err
	}
	store, err := leveldb.NewDatastore(filepath.Join(dataDir, datastoreDirectory), nil)
	if err != nil {
		return nil, fmt.Errorf("opening network datastore: %w", err)
	}

	cfg.PrivateKey = key
	cfg.Datastore = store
	node, err := New(ctx, cfg)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return &PersistentNode{Node: node, store: store}, nil
}

// Close stops the network before closing its datastore.
func (n *PersistentNode) Close() error {
	if n == nil {
		return nil
	}
	n.closeOnce.Do(func() {
		n.closeErr = errors.Join(n.Node.Close(), n.store.Close())
	})
	return n.closeErr
}
