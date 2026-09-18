package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/ipld/merkledag"
	unixfs "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	unixfspb "github.com/ipfs/boxo/ipld/unixfs/pb"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	query "github.com/ipfs/go-datastore/query"
	dsync "github.com/ipfs/go-datastore/sync"
	format "github.com/ipfs/go-ipld-format"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const mediaTestTimeout = 45 * time.Second

func TestMEDIA001ImportCopyBoundariesAndInterop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)

	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty"},
		{name: "single", data: []byte("public attachment")},
		{name: "exact_chunk", data: mediaPattern(1 << 20)},
		{name: "multiple_chunks", data: mediaPattern(2<<20 + 317)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit := int64(len(tc.data) + 1)
			if limit == 1 && len(tc.data) == 0 {
				limit = 1
			}
			got, err := n.ImportPublicFile(ctx, bytes.NewReader(tc.data), limit)
			if err != nil {
				t.Fatal(err)
			}
			if got.ByteLength != int64(len(tc.data)) {
				t.Fatalf("length = %d, want %d", got.ByteLength, len(tc.data))
			}
			assertMEDIA001CID(t, got.CID)

			oracle := independentMEDIA001Import(t, tc.data)
			if got.CID != oracle.Cid() {
				t.Fatalf("root = %s, upstream Boxo root = %s", got.CID, oracle.Cid())
			}
			again, err := n.ImportPublicFile(ctx, bytes.NewReader(tc.data), limit)
			if err != nil || again != got {
				t.Fatalf("repeat = %+v, %v; want %+v", again, err, got)
			}

			var dst bytes.Buffer
			written, err := n.CopyPublicFile(ctx, got, &dst)
			if err != nil {
				t.Fatal(err)
			}
			if written != int64(len(tc.data)) || !bytes.Equal(dst.Bytes(), tc.data) {
				t.Fatalf("copy = %d bytes / equal %v", written, bytes.Equal(dst.Bytes(), tc.data))
			}
		})
	}

	multi := mediaPattern(2<<20 + 317)
	file, err := n.ImportPublicFile(ctx, bytes.NewReader(multi), int64(len(multi)))
	if err != nil {
		t.Fatal(err)
	}
	if want := "bafybeiam4ft5qmcvb6nje45lvd55rb6ryr4fkd27h6llk22f7ybx3g4egq"; file.CID.String() != want {
		t.Fatalf("golden root = %s, want %s", file.CID, want)
	}
	upstream := newUpstreamFixture(t, ctx)
	connectUpstreamAndNode(t, ctx, upstream, n)
	service := merkledag.NewDAGService(blockservice.New(upstream.blocks, upstream.bitswap))
	root, err := service.Get(ctx, file.CID)
	if err != nil {
		t.Fatalf("independent upstream root read: %v", err)
	}
	reader, err := ufsio.NewDagReader(ctx, root, service)
	if err != nil {
		t.Fatalf("independent upstream UnixFS reader: %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(got, multi) {
		t.Fatalf("independent upstream read equal=%v: %v", bytes.Equal(got, multi), err)
	}
}

func TestMEDIA001ImportLimitsErrorsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)

	for _, max := range []int64{-1, 0, maxPublicFileBytes + 1} {
		reader := &mediaCountingReader{r: bytes.NewReader([]byte("unread"))}
		file, err := n.ImportPublicFile(ctx, reader, max)
		if !errors.Is(err, ErrInvalidPublicFile) || file.CID.Defined() || reader.n != 0 {
			t.Fatalf("max %d: file=%+v err=%v reads=%d", max, file, err, reader.n)
		}
	}
	for _, tc := range []struct {
		name string
		src  io.Reader
	}{
		{name: "nil", src: nil},
		{name: "typed_nil", src: (*bytes.Reader)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if file, err := n.ImportPublicFile(ctx, tc.src, 1); !errors.Is(err, ErrInvalidPublicFile) || file.CID.Defined() {
				t.Fatalf("file=%+v err=%v", file, err)
			}
		})
	}

	limit := int64(1 << 20)
	for _, size := range []int64{limit - 1, limit} {
		if _, err := n.ImportPublicFile(ctx, bytes.NewReader(mediaPattern(int(size))), limit); err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
	}
	file, err := n.ImportPublicFile(ctx, bytes.NewReader(mediaPattern(int(limit+1))), limit)
	if !errors.Is(err, ErrPublicFileTooLarge) || file.CID.Defined() {
		t.Fatalf("above limit: file=%+v err=%v", file, err)
	}

	sourceErr := errors.New("source failed")
	file, err = n.ImportPublicFile(ctx, &dataErrorReader{data: []byte("prefix"), err: sourceErr}, 100)
	if !errors.Is(err, sourceErr) || file.CID.Defined() {
		t.Fatalf("reader failure: file=%+v err=%v", file, err)
	}
	file, err = n.ImportPublicFile(ctx, &dataErrorReader{data: []byte("xy"), err: sourceErr}, 1)
	if !errors.Is(err, ErrPublicFileTooLarge) || file.CID.Defined() || file.ByteLength != 0 {
		t.Fatalf("limit plus source failure: file=%+v err=%v", file, err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	file, err = n.ImportPublicFile(cancelled, bytes.NewReader([]byte("x")), 1)
	if !errors.Is(err, context.Canceled) || file.CID.Defined() {
		t.Fatalf("cancelled import: file=%+v err=%v", file, err)
	}

	original := n.Blockstore
	n.Blockstore = &failingBlockstore{Blockstore: original, failAfter: 1, err: errors.New("store failed")}
	file, err = n.ImportPublicFile(ctx, bytes.NewReader(mediaPattern(2<<20)), 2<<20)
	n.Blockstore = original
	if err == nil || file.CID.Defined() {
		t.Fatalf("partial store: file=%+v err=%v", file, err)
	}

	for _, reader := range []io.Reader{
		invalidCountReader{count: 2},
		invalidCountReader{count: -1},
		zeroReader{},
	} {
		if file, err := n.ImportPublicFile(ctx, reader, 1); err == nil || file.CID.Defined() {
			t.Fatalf("invalid reader %T: file=%+v err=%v", reader, file, err)
		}
	}
}

func TestMEDIA001ImportPreservesSourceErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)

	wrappedUnexpected := fmt.Errorf("wrapped source failure: %w", io.ErrUnexpectedEOF)
	fullChunkFailure := errors.New("full chunk source failure")
	for _, tc := range []struct {
		name string
		data []byte
		err  error
	}{
		{name: "partial_unexpected_eof", data: []byte("partial"), err: io.ErrUnexpectedEOF},
		{name: "partial_wrapped_unexpected_eof", data: []byte("wrapped"), err: wrappedUnexpected},
		{name: "full_chunk_custom_error", data: mediaPattern(publicFileChunkBytes), err: fullChunkFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := n.ImportPublicFile(ctx, &oneShotReadError{data: tc.data, err: tc.err}, int64(len(tc.data)+1))
			if !errors.Is(err, tc.err) || file.CID.Defined() || file.ByteLength != 0 {
				t.Fatalf("file=%+v err=%v, want source error %v and zero descriptor", file, err, tc.err)
			}
		})
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "partial_ordinary_eof", data: []byte("valid partial")},
		{name: "full_chunk_ordinary_eof", data: mediaPattern(publicFileChunkBytes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := n.ImportPublicFile(ctx, &oneShotReadError{data: tc.data, err: io.EOF}, int64(len(tc.data)+1))
			if err != nil {
				t.Fatal(err)
			}
			var copied bytes.Buffer
			written, err := n.CopyPublicFile(ctx, file, &copied)
			if err != nil || written != int64(len(tc.data)) || !bytes.Equal(copied.Bytes(), tc.data) {
				t.Fatalf("copy=%d equal=%v err=%v", written, bytes.Equal(copied.Bytes(), tc.data), err)
			}
		})
	}
}

func TestMEDIA001CopyValidationAndWriteFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)
	data := mediaPattern(1<<20 + 29)
	file, err := n.ImportPublicFile(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	for _, bad := range []PublicFile{
		{},
		{CID: file.CID, ByteLength: -1},
		{CID: file.CID, ByteLength: maxPublicFileBytes + 1},
		{CID: cid.NewCidV0(file.CID.Hash()), ByteLength: file.ByteLength},
	} {
		if written, err := n.CopyPublicFile(ctx, bad, io.Discard); written != 0 || !errors.Is(err, ErrInvalidPublicFile) {
			t.Fatalf("bad descriptor %+v: written=%d err=%v", bad, written, err)
		}
	}
	for _, dst := range []io.Writer{nil, (*bytes.Buffer)(nil)} {
		if written, err := n.CopyPublicFile(ctx, file, dst); written != 0 || !errors.Is(err, ErrInvalidPublicFile) {
			t.Fatalf("bad writer: written=%d err=%v", written, err)
		}
	}

	wantErr := errors.New("destination failed")
	dst := &limitErrorWriter{remaining: 19, err: wantErr}
	written, err := n.CopyPublicFile(ctx, file, dst)
	if written != 19 || !errors.Is(err, wantErr) {
		t.Fatalf("write failure = %d, %v", written, err)
	}
	for _, writer := range []io.Writer{
		invalidCountWriter{count: len(data) + 1},
		invalidCountWriter{count: -1},
		invalidCountWriter{count: 1},
	} {
		written, err := n.CopyPublicFile(ctx, file, writer)
		if err == nil || written > file.ByteLength {
			t.Fatalf("invalid writer %T: written=%d err=%v", writer, written, err)
		}
	}

	for _, declared := range []int64{file.ByteLength - 1, file.ByteLength + 1} {
		var out bytes.Buffer
		written, err := n.CopyPublicFile(ctx, PublicFile{CID: file.CID, ByteLength: declared}, &out)
		if !errors.Is(err, ErrInvalidPublicFile) {
			t.Fatalf("declared %d: written=%d err=%v", declared, written, err)
		}
		if written > declared {
			t.Fatalf("declared %d emitted %d", declared, written)
		}
	}

	cancelled, stop := context.WithCancel(ctx)
	stop()
	if written, err := n.CopyPublicFile(cancelled, file, io.Discard); written != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled copy: written=%d err=%v", written, err)
	}

	var nilNode *Node
	if _, err := nilNode.ImportPublicFile(ctx, bytes.NewReader(nil), 1); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("nil node import: %v", err)
	}
	if _, err := nilNode.CopyPublicFile(ctx, file, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("nil node copy: %v", err)
	}
	if _, err := n.ImportPublicFile(nil, bytes.NewReader(nil), 1); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("nil context import: %v", err)
	}
	if _, err := n.CopyPublicFile(nil, file, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("nil context copy: %v", err)
	}
}

func TestMEDIA001MalformedDAGBudgetsAndCorruption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)

	raw, err := merkledag.NewRawNodeWPrefix([]byte("abc"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	mustStoreMEDIA001(t, ctx, n, raw)
	for _, tc := range []struct {
		name string
		root cid.Cid
		len  int64
	}{
		{name: "directory", root: mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TDirectory, nil, nil, nil)), len: 0},
		{name: "symlink", root: mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TSymlink, []byte("x"), nil, nil)), len: 1},
		{name: "metadata", root: mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TMetadata, nil, nil, nil)), len: 0},
		{name: "wrong_block_table_count", root: mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TFile, nil, []uint64{3, 0}, []format.Node{raw})), len: 3},
		{name: "dishonest_child_size", root: mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TFile, nil, []uint64{2}, []format.Node{raw})), len: 2},
		{name: "too_many_links", root: mustStoreMEDIA001(t, ctx, n, mediaRepeatedNode(t, raw, maxFileLinks+1)), len: 0},
		{name: "too_deep", root: mediaDeepDAG(t, ctx, n, raw, maxFileDepth+1), len: 3},
		{name: "too_many_occurrences", root: mediaOccurrenceDAG(t, ctx, n, raw), len: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := n.CopyPublicFile(ctx, PublicFile{CID: tc.root, ByteLength: tc.len}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	one, err := merkledag.NewRawNodeWPrefix([]byte("a"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	two, err := merkledag.NewRawNodeWPrefix([]byte("bc"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	mustStoreMEDIA001(t, ctx, n, one)
	mustStoreMEDIA001(t, ctx, n, two)
	swapped := mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TFile, nil, []uint64{2, 1}, []format.Node{one, two}))
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: swapped, ByteLength: 3}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("same-total swapped child sizes err = %v", err)
	}
	overflow := mustStoreMEDIA001(t, ctx, n, mediaProtoNodeWithSize(t, unixfs.TFile, nil, math.MaxUint64, []uint64{math.MaxUint64}, []format.Node{raw}))
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: overflow, ByteLength: 3}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("overflow size table err = %v", err)
	}

	oversized := mediaPattern(maxEncodedFileBlockBytes + 1)
	oversizedNode, err := merkledag.NewRawNodeWPrefix(oversized, ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	oversizedCID := mustStoreMEDIA001(t, ctx, n, oversizedNode)
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: oversizedCID, ByteLength: int64(len(oversized))}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("oversized block err = %v", err)
	}

	good, err := merkledag.NewRawNodeWPrefix([]byte("good"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Blockstore.Put(ctx, good); err != nil {
		t.Fatal(err)
	}
	key := datastore.NewKey("/blocks/" + good.Cid().Hash().B58String())
	// The concrete datastore key is implementation-specific, so locate it through Query.
	results, err := n.Datastore.Query(ctx, query.Query{Prefix: "/blocks"})
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	for entry := range results.Next() {
		if entry.Error == nil && bytes.Equal(entry.Value, good.RawData()) {
			key = datastore.NewKey(entry.Key)
			break
		}
	}
	if err := n.Datastore.Put(ctx, key, []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: good.Cid(), ByteLength: 4}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("corrupt local block err = %v", err)
	}
}

func TestMEDIA001TraversalBudgetBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	n := newMEDIA001Node(t, ctx)
	empty := mediaProtoNode(t, unixfs.TFile, nil, nil, nil)
	mustStoreMEDIA001(t, ctx, n, empty)

	for _, links := range []int{maxFileLinks - 1, maxFileLinks} {
		root := mustStoreMEDIA001(t, ctx, n, mediaRepeatedNode(t, empty, links))
		if written, err := n.CopyPublicFile(ctx, PublicFile{CID: root, ByteLength: 0}, io.Discard); err != nil || written != 0 {
			t.Fatalf("%d links: written=%d err=%v", links, written, err)
		}
	}
	tooMany := mustStoreMEDIA001(t, ctx, n, mediaRepeatedNode(t, empty, maxFileLinks+1))
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: tooMany, ByteLength: 0}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("above link limit: %v", err)
	}

	raw, err := merkledag.NewRawNodeWPrefix([]byte("abc"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	mustStoreMEDIA001(t, ctx, n, raw)
	for _, depth := range []int{maxFileDepth - 1, maxFileDepth} {
		root := mediaDeepDAG(t, ctx, n, raw, depth)
		if written, err := n.CopyPublicFile(ctx, PublicFile{CID: root, ByteLength: 3}, io.Discard); err != nil || written != 3 {
			t.Fatalf("depth %d: written=%d err=%v", depth, written, err)
		}
	}
	deep := mediaDeepDAG(t, ctx, n, raw, maxFileDepth+1)
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: deep, ByteLength: 3}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("above depth limit: %v", err)
	}

	atOccurrences := mediaOccurrenceBoundaryDAG(t, ctx, n, empty, false)
	if written, err := n.CopyPublicFile(ctx, PublicFile{CID: atOccurrences, ByteLength: 0}, io.Discard); err != nil || written != 0 {
		t.Fatalf("at occurrence limit: written=%d err=%v", written, err)
	}
	aboveOccurrences := mediaOccurrenceBoundaryDAG(t, ctx, n, empty, true)
	if _, err := n.CopyPublicFile(ctx, PublicFile{CID: aboveOccurrences, ByteLength: 0}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
		t.Fatalf("above occurrence limit: %v", err)
	}

	padded := mediaPaddedZeroFileNode(t, maxEncodedFileBlockBytes-1024)
	paddedSize := int64(len(padded.RawData()))
	mustStoreMEDIA001(t, ctx, n, padded)
	count := int((maxFileEncodedBytes - 64*1024) / paddedSize)
	for {
		control := mediaRepeatedNode(t, padded, count)
		total := int64(len(control.RawData())) + int64(count)*paddedSize
		if total <= maxFileEncodedBytes {
			root := mustStoreMEDIA001(t, ctx, n, control)
			if _, err := n.CopyPublicFile(ctx, PublicFile{CID: root, ByteLength: 0}, io.Discard); err != nil {
				t.Fatalf("encoded-byte control %d: %v", total, err)
			}
			break
		}
		count--
	}
	for {
		count++
		over := mediaRepeatedNode(t, padded, count)
		total := int64(len(over.RawData())) + int64(count)*paddedSize
		if total > maxFileEncodedBytes {
			root := mustStoreMEDIA001(t, ctx, n, over)
			if _, err := n.CopyPublicFile(ctx, PublicFile{CID: root, ByteLength: 0}, io.Discard); !errors.Is(err, ErrInvalidPublicFile) {
				t.Fatalf("encoded-byte excess %d: %v", total, err)
			}
			break
		}
	}
}

func TestMEDIA001InFlightCancellationAndUnavailableBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()

	for _, closeNode := range []bool{false, true} {
		t.Run(fmt.Sprintf("close_node_%v", closeNode), func(t *testing.T) {
			n := newMEDIA001NodeNoCleanup(t, ctx)
			if !closeNode {
				t.Cleanup(func() { _ = n.Close() })
			}
			started := make(chan struct{})
			n.Blockstore = &blockingBlockstore{Blockstore: n.Blockstore, started: started}
			opctx, stop := context.WithCancel(ctx)
			defer stop()
			result := make(chan error, 1)
			go func() {
				_, err := n.ImportPublicFile(opctx, bytes.NewReader([]byte("blocked")), 7)
				result <- err
			}()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if closeNode {
				if err := n.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				stop()
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("blocked import err = %v", err)
				}
			case <-ctx.Done():
				t.Fatal("blocked import did not stop")
			}
		})
	}

	n := newMEDIA001Node(t, ctx)
	missing, err := merkledag.NewRawNodeWPrefix([]byte("missing"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	root := mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TFile, nil, []uint64{7}, []format.Node{missing}))
	deadline, stop := context.WithTimeout(ctx, 200*time.Millisecond)
	defer stop()
	if _, err := n.CopyPublicFile(deadline, PublicFile{CID: root, ByteLength: 7}, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing remote child err = %v", err)
	}
}

func TestMEDIA001InlineDataRemoteTransferPrivacyPersistenceAndConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), mediaTestTimeout)
	defer cancel()
	server := newMEDIA001Node(t, ctx)
	child, err := merkledag.NewRawNodeWPrefix([]byte("child"), ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		t.Fatal(err)
	}
	mustStoreMEDIA001(t, ctx, server, child)
	rootNode := mediaProtoNode(t, unixfs.TFile, []byte("head-"), []uint64{5}, []format.Node{child})
	root := mustStoreMEDIA001(t, ctx, server, rootNode)
	file := PublicFile{CID: root, ByteLength: 10}
	var inline bytes.Buffer
	if n, err := server.CopyPublicFile(ctx, file, &inline); err != nil || n != 10 || inline.String() != "head-child" {
		t.Fatalf("inline copy = %q, %d, %v", inline.String(), n, err)
	}

	multiData := mediaPattern(2<<20 + 317)
	multiFile, err := server.ImportPublicFile(ctx, bytes.NewReader(multiData), int64(len(multiData)))
	if err != nil {
		t.Fatal(err)
	}
	oracleRoot := independentMEDIA001Import(t, multiData)
	allFixtureCIDs := []cid.Cid{oracleRoot.Cid()}
	for _, link := range oracleRoot.Links() {
		allFixtureCIDs = append(allFixtureCIDs, link.Cid)
	}

	client := newMEDIA001Node(t, ctx)
	connectMEDIA001Nodes(t, ctx, server, client)
	for _, id := range allFixtureCIDs {
		if has, err := client.Blockstore.Has(ctx, id); err != nil || has {
			t.Fatalf("client had fixture block %s before transfer: %v, %v", id, has, err)
		}
	}
	var transferred bytes.Buffer
	if n, err := client.CopyPublicFile(ctx, multiFile, &transferred); err != nil || n != int64(len(multiData)) || !bytes.Equal(transferred.Bytes(), multiData) {
		t.Fatalf("remote multichunk copy = equal %v, %d, %v", bytes.Equal(transferred.Bytes(), multiData), n, err)
	}

	private := []byte("private sentinel")
	privateCID := blocks.NewBlock(private).Cid()
	if err := server.Datastore.Put(ctx, datastore.NewKey("/bitbook/private/media001"), private); err != nil {
		t.Fatal(err)
	}
	privateCtx, privateCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer privateCancel()
	if _, err := client.Bitswap.GetBlock(privateCtx, privateCID); err == nil {
		t.Fatal("private datastore sentinel was served by Bitswap")
	}

	dataDir := t.TempDir()
	cfg := Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}}
	first, err := Open(ctx, dataDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := first.ImportPublicFile(ctx, bytes.NewReader(mediaPattern(1<<20+7)), 1<<20+7)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, dataDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if n, err := second.CopyPublicFile(ctx, persisted, io.Discard); err != nil || n != persisted.ByteLength {
		t.Fatalf("reopened copy = %d, %v", n, err)
	}

	parallel, err := server.ImportPublicFile(ctx, bytes.NewReader(mediaPattern(2<<20+9)), 2<<20+9)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n, err := server.CopyPublicFile(ctx, parallel, io.Discard); err != nil || n != parallel.ByteLength {
				errCh <- fmt.Errorf("copy=%d: %w", n, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	closed := newMEDIA001NodeNoCleanup(t, ctx)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := closed.ImportPublicFile(ctx, bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("import after close succeeded")
	}
	if _, err := closed.CopyPublicFile(ctx, file, io.Discard); err == nil {
		t.Fatal("copy after close succeeded")
	}
}

func newMEDIA001Node(t testing.TB, ctx context.Context) *Node {
	t.Helper()
	n := newMEDIA001NodeNoCleanup(t, ctx)
	t.Cleanup(func() { _ = n.Close() })
	return n
}

func newMEDIA001NodeNoCleanup(t testing.TB, ctx context.Context) *Node {
	t.Helper()
	n, err := New(ctx, Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, DHTMode: dht.ModeServer, AllowPrivateAddresses: true})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func connectMEDIA001Nodes(t *testing.T, ctx context.Context, a, b *Node) {
	t.Helper()
	if err := a.Connect(ctx, b.Host.Peerstore().PeerInfo(b.ID())); err != nil {
		t.Fatal(err)
	}
	if err := b.Connect(ctx, a.Host.Peerstore().PeerInfo(a.ID())); err != nil {
		t.Fatal(err)
	}
}

func independentMEDIA001Import(t testing.TB, data []byte) format.Node {
	t.Helper()
	store := blockstore.NewBlockstore(dsync.MutexWrap(datastore.NewMapDatastore()))
	service := merkledag.NewDAGService(blockservice.New(store, offline.Exchange(store)))
	profile := ufsio.UnixFS_v1_2025
	params := helpers.DagBuilderParams{Dagserv: service, Maxlinks: profile.FileDAGWidth, RawLeaves: profile.RawLeaves, CidBuilder: profile.CidBuilder()}
	builder, err := params.New(chunker.NewSizeSplitter(bytes.NewReader(data), profile.ChunkSize))
	if err != nil {
		t.Fatal(err)
	}
	root, err := balanced.Layout(builder)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func mediaProtoNode(t testing.TB, typ unixfspb.Data_DataType, data []byte, sizes []uint64, children []format.Node) *merkledag.ProtoNode {
	t.Helper()
	fs := unixfs.NewFSNode(typ)
	fs.SetData(data)
	for _, size := range sizes {
		fs.AddBlockSize(size)
	}
	encoded, err := fs.GetBytes()
	if err != nil {
		t.Fatal(err)
	}
	node := new(merkledag.ProtoNode)
	node.SetCidBuilder(ufsio.UnixFS_v1_2025.CidBuilder())
	node.SetData(encoded)
	for _, child := range children {
		if err := node.AddNodeLink("", child); err != nil {
			t.Fatal(err)
		}
	}
	return node
}

func mediaProtoNodeWithSize(t testing.TB, typ unixfspb.Data_DataType, data []byte, fileSize uint64, sizes []uint64, children []format.Node) *merkledag.ProtoNode {
	t.Helper()
	message := &unixfspb.Data{Type: typ.Enum(), Data: data, Filesize: proto.Uint64(fileSize), Blocksizes: sizes}
	encoded, err := proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	node := new(merkledag.ProtoNode)
	node.SetCidBuilder(ufsio.UnixFS_v1_2025.CidBuilder())
	node.SetData(encoded)
	for _, child := range children {
		if err := node.AddNodeLink("", child); err != nil {
			t.Fatal(err)
		}
	}
	return node
}

func mediaRepeatedNode(t testing.TB, child format.Node, count int) *merkledag.ProtoNode {
	sizes := make([]uint64, count)
	children := make([]format.Node, count)
	for i := range children {
		children[i] = child
	}
	return mediaProtoNode(t, unixfs.TFile, nil, sizes, children)
}

func mediaDeepDAG(t testing.TB, ctx context.Context, n *Node, leaf format.Node, depth int) cid.Cid {
	current := leaf
	for range depth {
		current = mediaProtoNode(t, unixfs.TFile, nil, []uint64{3}, []format.Node{current})
		mustStoreMEDIA001(t, ctx, n, current)
	}
	return current.Cid()
}

func mediaOccurrenceDAG(t testing.TB, ctx context.Context, n *Node, leaf format.Node) cid.Cid {
	empty := mediaProtoNode(t, unixfs.TFile, nil, nil, nil)
	mustStoreMEDIA001(t, ctx, n, empty)
	branch := mediaRepeatedNode(t, empty, 1024)
	mustStoreMEDIA001(t, ctx, n, branch)
	root := mediaRepeatedNode(t, branch, 5)
	return mustStoreMEDIA001(t, ctx, n, root)
}

func mediaOccurrenceBoundaryDAG(t testing.TB, ctx context.Context, n *Node, empty format.Node, above bool) cid.Cid {
	counts := []int{1023, 1023, 1023, 1022}
	if above {
		counts[3]++
	}
	branches := make([]format.Node, 0, len(counts))
	for _, count := range counts {
		branch := mediaRepeatedNode(t, empty, count)
		mustStoreMEDIA001(t, ctx, n, branch)
		branches = append(branches, branch)
	}
	return mustStoreMEDIA001(t, ctx, n, mediaProtoNode(t, unixfs.TFile, nil, make([]uint64, len(branches)), branches))
}

func mediaPaddedZeroFileNode(t testing.TB, target int) *merkledag.ProtoNode {
	t.Helper()
	typ := unixfspb.Data_File
	base, err := proto.Marshal(&unixfspb.Data{Type: &typ, Filesize: proto.Uint64(0)})
	if err != nil {
		t.Fatal(err)
	}
	for padding := target; padding >= 0; padding-- {
		unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
		unknown = protowire.AppendBytes(unknown, make([]byte, padding))
		node := new(merkledag.ProtoNode)
		node.SetCidBuilder(ufsio.UnixFS_v1_2025.CidBuilder())
		node.SetData(append(append([]byte(nil), base...), unknown...))
		if len(node.RawData()) <= target {
			return node
		}
	}
	t.Fatal("could not construct padded UnixFS node")
	return nil
}

func mustStoreMEDIA001(t testing.TB, ctx context.Context, n *Node, node format.Node) cid.Cid {
	t.Helper()
	if err := n.Blockstore.Put(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err := n.Bitswap.NotifyNewBlocks(ctx, node); err != nil {
		t.Fatal(err)
	}
	return node.Cid()
}

func assertMEDIA001CID(t testing.TB, id cid.Cid) {
	t.Helper()
	prefix := id.Prefix()
	if id.Version() != 1 || prefix.MhType != 0x12 || prefix.MhLength != 32 || (prefix.Codec != cid.Raw && prefix.Codec != cid.DagProtobuf) {
		t.Fatalf("CID profile = %+v (%s)", prefix, id)
	}
}

func mediaPattern(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i*31 + 7)
	}
	return b
}

type mediaCountingReader struct {
	r io.Reader
	n int
}

func (r *mediaCountingReader) Read(p []byte) (int, error) {
	r.n++
	return r.r.Read(p)
}

type dataErrorReader struct {
	data []byte
	err  error
	done bool
}

type oneShotReadError struct {
	data []byte
	err  error
	done bool
}

func (r *oneShotReadError) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), r.err
}

type invalidCountReader struct{ count int }

func (r invalidCountReader) Read([]byte) (int, error) { return r.count, nil }

type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) { return 0, nil }

func (r *dataErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

type limitErrorWriter struct {
	remaining int
	err       error
}

type invalidCountWriter struct{ count int }

func (w invalidCountWriter) Write([]byte) (int, error) { return w.count, nil }

func (w *limitErrorWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, w.err
	}
	n := min(len(p), w.remaining)
	w.remaining -= n
	return n, w.err
}

type failingBlockstore struct {
	blockstore.Blockstore
	puts      int
	failAfter int
	err       error
}

func (s *failingBlockstore) Put(ctx context.Context, block blocks.Block) error {
	s.puts++
	if s.puts > s.failAfter {
		return s.err
	}
	return s.Blockstore.Put(ctx, block)
}

type blockingBlockstore struct {
	blockstore.Blockstore
	started chan struct{}
	once    sync.Once
}

func (s *blockingBlockstore) Put(ctx context.Context, _ blocks.Block) error {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return ctx.Err()
}
