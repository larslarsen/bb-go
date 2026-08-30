package network

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
)

const identityKeyFilename = "identity.key"

// LoadOrCreatePrivateKey loads a marshaled libp2p key from identity.key under
// dataDir, or atomically creates an Ed25519 key there. The file is private
// because it is the node identity. All reads, temporary creation, permission
// setting, and rename stay inside that directory through os.Root.
func LoadOrCreatePrivateKey(dataDir string) (crypto.PrivKey, error) {
	if dataDir == "" {
		return nil, errors.New("empty data directory")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating identity directory: %w", err)
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return nil, fmt.Errorf("opening identity directory: %w", err)
	}
	defer root.Close()

	encoded, err := root.ReadFile(identityKeyFilename)
	if err == nil {
		key, err := crypto.UnmarshalPrivateKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("decoding identity key %s: %w", identityKeyFilename, err)
		}
		return key, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading identity key %s: %w", identityKeyFilename, err)
	}

	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating identity key: %w", err)
	}
	encoded, err = crypto.MarshalPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("encoding identity key: %w", err)
	}

	tmp, tmpName, err := createIdentityTemp(root)
	if err != nil {
		return nil, err
	}
	defer root.Remove(tmpName)
	if err := root.Chmod(tmpName, 0o600); err != nil {
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
	if err := root.Rename(tmpName, identityKeyFilename); err != nil {
		return nil, fmt.Errorf("installing identity key: %w", err)
	}
	return key, nil
}

func createIdentityTemp(root *os.Root) (*os.File, string, error) {
	for attempt := 0; attempt < 10000; attempt++ {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, "", fmt.Errorf("creating temporary identity key: %w", err)
		}
		name := fmt.Sprintf(".identity-%x", suffix)
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", fmt.Errorf("creating temporary identity key: %w", err)
		}
	}
	return nil, "", errors.New("creating temporary identity key: all candidate names exist")
}
