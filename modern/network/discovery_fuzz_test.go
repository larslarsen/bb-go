package network

import (
	"bytes"
	"io"
	"testing"
)

type countingReader struct {
	reader io.Reader
	read   int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

func FuzzPeerHello(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		[]byte("BBGO001\n"),
		[]byte("BBGO001"),
		[]byte("BBGO001\r"),
		[]byte("BBGO001\nX"),
		bytes.Repeat([]byte{'B'}, 1024),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		reader := &countingReader{reader: bytes.NewReader(raw)}
		err := readPeerHello(reader)
		wantValid := bytes.Equal(raw, []byte("BBGO001\n"))
		if wantValid && err != nil {
			t.Fatalf("exact hello rejected: %v", err)
		}
		if !wantValid && err == nil {
			t.Fatalf("invalid hello accepted: %q", raw)
		}
		if reader.read > 9 {
			t.Fatalf("readPeerHello consumed %d bytes, want at most 9", reader.read)
		}
	})
}
