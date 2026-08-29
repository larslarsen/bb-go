package network

import (
	"context"
	"testing"
)

func TestOpenPersistsPeerIdentity(t *testing.T) {
	dataDir := t.TempDir()
	config := Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}}

	first, err := Open(context.Background(), dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.ID()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(context.Background(), dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.ID() != firstID {
		t.Fatalf("peer identity changed: got %s want %s", second.ID(), firstID)
	}
}
