package network

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// LoadOrCreatePrivateKey loads a marshaled libp2p key or atomically creates an
// Ed25519 key at path. The file is private because it is the node identity.
func LoadOrCreatePrivateKey(path string) (crypto.PrivKey, error) {
	encoded, err := os.ReadFile(path)
	if err == nil {
		key, err := crypto.UnmarshalPrivateKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("decoding identity key %s: %w", path, err)
		}
		return key, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading identity key %s: %w", path, err)
	}

	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating identity key: %w", err)
	}
	encoded, err = crypto.MarshalPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("encoding identity key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating identity directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".identity-*")
	if err != nil {
		return nil, fmt.Errorf("creating temporary identity key: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("securing temporary identity key: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("writing temporary identity key: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("syncing temporary identity key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("closing temporary identity key: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return nil, fmt.Errorf("installing identity key: %w", err)
	}
	return key, nil
}
