package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestLoadOrCreatePrivateKeyPersistsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.key")
	first, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}

	firstID, err := peer.IDFromPrivateKey(first)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := peer.IDFromPrivateKey(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstID != secondID {
		t.Fatalf("identity changed: %s != %s", firstID, secondID)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("identity permissions are %o, want 600", info.Mode().Perm())
	}
}
