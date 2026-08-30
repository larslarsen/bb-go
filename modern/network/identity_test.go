package network

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestLoadOrCreatePrivateKeyPersistsIdentity(t *testing.T) {
	dataDir := t.TempDir()
	first, err := LoadOrCreatePrivateKey(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreatePrivateKey(dataDir)
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

	info, err := os.Stat(filepath.Join(dataDir, identityKeyFilename))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("identity permissions are %o, want 600", info.Mode().Perm())
	}
}

func TestLoadOrCreatePrivateKeyRejectsEscapingSymlink(t *testing.T) {
	outsideDir := t.TempDir()
	dataDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "stolen.key")

	outsideKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := crypto.MarshalPrivateKey(outsideKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outsideFile, filepath.Join(dataDir, identityKeyFilename)); err != nil {
		t.Fatal(err)
	}

	got, err := LoadOrCreatePrivateKey(dataDir)
	if err == nil {
		loadedID, idErr := peer.IDFromPrivateKey(got)
		if idErr != nil {
			t.Fatal(idErr)
		}
		outsideID, idErr := peer.IDFromPrivateKey(outsideKey)
		if idErr != nil {
			t.Fatal(idErr)
		}
		t.Fatalf("escaping identity.key symlink was accepted as %s (outside %s)", loadedID, outsideID)
	}
	if !strings.Contains(err.Error(), "path escapes from parent") {
		t.Fatalf("error = %v, want path escape from parent", err)
	}

	after, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, encoded) {
		t.Fatal("outside identity file was altered")
	}

	info, err := os.Lstat(filepath.Join(dataDir, identityKeyFilename))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("identity.key symlink was replaced")
	}
}
