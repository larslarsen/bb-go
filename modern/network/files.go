package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	unixfspb "github.com/ipfs/boxo/ipld/unixfs/pb"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
)

const (
	maxPublicFileBytes       int64 = 100 << 20
	publicFileChunkBytes           = 1 << 20
	maxEncodedFileBlockBytes       = 2 << 20
	maxFileLinks                   = 1024
	maxFileDepth                   = 32
	maxFileNodeOccurrences         = 4096
	maxFileEncodedBytes      int64 = 256 << 20
)

var (
	ErrInvalidPublicFile  = errors.New("invalid public file")
	ErrPublicFileTooLarge = errors.New("public file exceeds limit")
)

// PublicFile identifies a public UnixFS file and its exact decoded byte length.
// It conveys neither encryption nor a long-term retention guarantee.
type PublicFile struct {
	CID        cid.Cid
	ByteLength int64
}

// ImportPublicFile streams src into this node's public blockstore as a balanced
// UnixFS file using the unixfs-v1-2025 file profile.
func (n *Node) ImportPublicFile(ctx context.Context, src io.Reader, maxBytes int64) (PublicFile, error) {
	if maxBytes < 1 || maxBytes > maxPublicFileBytes {
		return PublicFile{}, fmt.Errorf("%w: maxBytes must be between 1 and %d", ErrInvalidPublicFile, maxPublicFileBytes)
	}
	if isNilInterface(src) {
		return PublicFile{}, fmt.Errorf("%w: nil source reader", ErrInvalidPublicFile)
	}
	opctx, done, err := n.fileOperationContext(ctx)
	if err != nil {
		return PublicFile{}, err
	}
	defer done()

	limited := &publicFileLimitReader{ctx: opctx, source: src, maximum: maxBytes}
	service := &fileImportDAGService{node: n, ctx: opctx}
	params := helpers.DagBuilderParams{
		Dagserv:    service,
		Maxlinks:   maxFileLinks,
		RawLeaves:  true,
		CidBuilder: publicFileCIDBuilder(),
	}
	builder, err := params.New(chunker.NewSizeSplitter(limited, publicFileChunkBytes))
	if err != nil {
		return PublicFile{}, fmt.Errorf("creating public file importer: %w", err)
	}
	root, err := balanced.Layout(builder)
	if err != nil {
		if limited.sourceErr != nil && !errors.Is(err, limited.sourceErr) {
			err = errors.Join(err, limited.sourceErr)
		}
		if errors.Is(err, ErrPublicFileTooLarge) {
			return PublicFile{}, fmt.Errorf("%w: %w", ErrInvalidPublicFile, err)
		}
		return PublicFile{}, fmt.Errorf("importing public file: %w", err)
	}
	if limited.sourceErr != nil {
		return PublicFile{}, fmt.Errorf("importing public file source: %w", limited.sourceErr)
	}
	if err := opctx.Err(); err != nil {
		return PublicFile{}, fmt.Errorf("importing public file: %w", err)
	}
	return PublicFile{CID: root.Cid(), ByteLength: limited.read}, nil
}

// CopyPublicFile verifies and streams a public raw/UnixFS file to dst. If it
// returns an error, dst may contain the reported prefix and callers must discard
// it as an incomplete result.
func (n *Node) CopyPublicFile(ctx context.Context, file PublicFile, dst io.Writer) (int64, error) {
	if err := validatePublicFileDescriptor(file); err != nil {
		return 0, err
	}
	if isNilInterface(dst) {
		return 0, fmt.Errorf("%w: nil destination writer", ErrInvalidPublicFile)
	}
	opctx, done, err := n.fileOperationContext(ctx)
	if err != nil {
		return 0, err
	}
	defer done()

	state := fileCopyState{ctx: opctx, node: n, dst: dst, remaining: file.ByteLength}
	size, err := state.copyNode(file.CID, 0)
	if err != nil {
		return state.written, err
	}
	if size != uint64(file.ByteLength) || state.remaining != 0 {
		return state.written, fmt.Errorf("%w: decoded length %d does not equal declared length %d", ErrInvalidPublicFile, size, file.ByteLength)
	}
	return state.written, nil
}

func (n *Node) fileOperationContext(ctx context.Context) (context.Context, func(), error) {
	if n == nil || n.ctx == nil || n.Blockstore == nil || n.Bitswap == nil {
		return nil, nil, fmt.Errorf("%w: unavailable node", ErrInvalidPublicFile)
	}
	if ctx == nil {
		return nil, nil, fmt.Errorf("%w: nil context", ErrInvalidPublicFile)
	}
	if err := n.ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("node is closed: %w", err)
	}
	opctx, cancel := context.WithCancel(ctx)
	stopNodeCancel := context.AfterFunc(n.ctx, cancel)
	done := func() {
		stopNodeCancel()
		cancel()
	}
	if err := opctx.Err(); err != nil {
		done()
		return nil, nil, err
	}
	return opctx, done, nil
}

func validatePublicFileDescriptor(file PublicFile) error {
	if file.ByteLength < 0 || file.ByteLength > maxPublicFileBytes {
		return fmt.Errorf("%w: byte length %d is outside 0..%d", ErrInvalidPublicFile, file.ByteLength, maxPublicFileBytes)
	}
	if err := validatePublicFileCID(file.CID); err != nil {
		return err
	}
	return nil
}

func validatePublicFileCID(id cid.Cid) error {
	if !id.Defined() {
		return fmt.Errorf("%w: undefined CID", ErrInvalidPublicFile)
	}
	prefix := id.Prefix()
	if id.Version() != 1 || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 || (prefix.Codec != cid.Raw && prefix.Codec != cid.DagProtobuf) {
		return fmt.Errorf("%w: unsupported CID profile %s", ErrInvalidPublicFile, id)
	}
	return nil
}

func publicFileCIDBuilder() cid.Builder {
	return cid.Prefix{Version: 1, Codec: cid.DagProtobuf, MhType: mh.SHA2_256, MhLength: 32}
}

type publicFileLimitReader struct {
	ctx       context.Context
	source    io.Reader
	maximum   int64
	read      int64
	empty     int
	sourceErr error
}

func (r *publicFileLimitReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	remaining := r.maximum - r.read
	if remaining < 0 {
		return 0, ErrPublicFileTooLarge
	}
	if remaining == 0 {
		var probe [1]byte
		n, err := r.source.Read(probe[:])
		r.recordSourceError(err)
		if n < 0 || n > 1 {
			return 0, fmt.Errorf("%w: source returned invalid count %d", ErrInvalidPublicFile, n)
		}
		if n > 0 {
			return 0, ErrPublicFileTooLarge
		}
		if err == nil {
			return 0, io.ErrNoProgress
		}
		return 0, err
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.source.Read(p)
	r.recordSourceError(err)
	if n < 0 || n > len(p) {
		return 0, fmt.Errorf("%w: source returned invalid count %d for buffer %d", ErrInvalidPublicFile, n, len(p))
	}
	if int64(n) > remaining {
		r.read += remaining
		return int(remaining), ErrPublicFileTooLarge
	}
	if n == 0 && err == nil {
		r.empty++
		if r.empty >= 100 {
			return 0, io.ErrNoProgress
		}
	} else {
		r.empty = 0
	}
	r.read += int64(n)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		if n > 0 {
			return n, ctxErr
		}
		return 0, ctxErr
	}
	return n, err
}

func (r *publicFileLimitReader) recordSourceError(err error) {
	if err != nil && !errors.Is(err, io.EOF) && r.sourceErr == nil {
		r.sourceErr = err
	}
}

type fileImportDAGService struct {
	node *Node
	ctx  context.Context
}

func (s *fileImportDAGService) Add(_ context.Context, node format.Node) error {
	if node == nil {
		return fmt.Errorf("%w: importer produced nil node", ErrInvalidPublicFile)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if len(node.RawData()) > maxEncodedFileBlockBytes {
		return fmt.Errorf("%w: importer produced oversized block", ErrInvalidPublicFile)
	}
	if err := s.node.Blockstore.Put(s.ctx, node); err != nil {
		return fmt.Errorf("storing public file block: %w", err)
	}
	if err := s.node.Bitswap.NotifyNewBlocks(s.ctx, node); err != nil {
		return fmt.Errorf("notifying Bitswap of public file block: %w", err)
	}
	return nil
}

func (s *fileImportDAGService) AddMany(ctx context.Context, nodes []format.Node) error {
	for _, node := range nodes {
		if err := s.Add(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

func (s *fileImportDAGService) Get(context.Context, cid.Cid) (format.Node, error) {
	return nil, errors.New("public file importer does not read DAG nodes")
}

func (s *fileImportDAGService) GetMany(context.Context, []cid.Cid) <-chan *format.NodeOption {
	result := make(chan *format.NodeOption)
	close(result)
	return result
}

func (s *fileImportDAGService) Remove(context.Context, cid.Cid) error {
	return errors.New("public file importer does not remove DAG nodes")
}

func (s *fileImportDAGService) RemoveMany(context.Context, []cid.Cid) error {
	return errors.New("public file importer does not remove DAG nodes")
}

type validatedFileBlock struct {
	data       []byte
	links      []*format.Link
	blockSizes []uint64
	fileSize   uint64
}

func validateFileBlock(id cid.Cid, data []byte) (validatedFileBlock, error) {
	if err := validatePublicFileCID(id); err != nil {
		return validatedFileBlock{}, err
	}
	if len(data) > maxEncodedFileBlockBytes {
		return validatedFileBlock{}, fmt.Errorf("%w: encoded block exceeds %d bytes", ErrInvalidPublicFile, maxEncodedFileBlockBytes)
	}
	computed, err := id.Prefix().Sum(data)
	if err != nil || computed != id {
		return validatedFileBlock{}, fmt.Errorf("%w: block bytes do not match CID %s", ErrInvalidPublicFile, id)
	}
	if id.Prefix().Codec == cid.Raw {
		return validatedFileBlock{data: data, fileSize: uint64(len(data))}, nil
	}

	node, err := merkledag.DecodeProtobuf(data)
	if err != nil {
		return validatedFileBlock{}, fmt.Errorf("%w: decoding DAG-PB block: %v", ErrInvalidPublicFile, err)
	}
	message := new(unixfspb.Data)
	if err := proto.Unmarshal(node.Data(), message); err != nil {
		return validatedFileBlock{}, fmt.Errorf("%w: decoding UnixFS data: %v", ErrInvalidPublicFile, err)
	}
	if message.Type == nil || message.Filesize == nil {
		return validatedFileBlock{}, fmt.Errorf("%w: UnixFS file omits required type or size", ErrInvalidPublicFile)
	}
	if message.GetType() != unixfspb.Data_File && message.GetType() != unixfspb.Data_Raw {
		return validatedFileBlock{}, fmt.Errorf("%w: UnixFS node type %s is not a file", ErrInvalidPublicFile, message.GetType())
	}
	links := node.Links()
	if len(links) > maxFileLinks {
		return validatedFileBlock{}, fmt.Errorf("%w: file node has %d links", ErrInvalidPublicFile, len(links))
	}
	if len(message.Blocksizes) != len(links) {
		return validatedFileBlock{}, fmt.Errorf("%w: %d block sizes for %d links", ErrInvalidPublicFile, len(message.Blocksizes), len(links))
	}
	total := uint64(len(message.Data))
	for i, link := range links {
		if link == nil || link.Name != "" {
			return validatedFileBlock{}, fmt.Errorf("%w: file link %d is nil or named", ErrInvalidPublicFile, i)
		}
		if err := validatePublicFileCID(link.Cid); err != nil {
			return validatedFileBlock{}, fmt.Errorf("%w: file link %d: %v", ErrInvalidPublicFile, i, err)
		}
		size := message.Blocksizes[i]
		if size > uint64(maxPublicFileBytes) || total > uint64(maxPublicFileBytes)-size {
			return validatedFileBlock{}, fmt.Errorf("%w: UnixFS size overflow or limit exceeded", ErrInvalidPublicFile)
		}
		total += size
	}
	if message.GetFilesize() != total {
		return validatedFileBlock{}, fmt.Errorf("%w: UnixFS file size %d does not equal data/block table total %d", ErrInvalidPublicFile, message.GetFilesize(), total)
	}
	return validatedFileBlock{
		data:       message.Data,
		links:      links,
		blockSizes: message.Blocksizes,
		fileSize:   total,
	}, nil
}

type fileCopyState struct {
	ctx              context.Context
	node             *Node
	dst              io.Writer
	remaining        int64
	written          int64
	occurrences      int
	encodedProcessed int64
}

func (s *fileCopyState) copyNode(id cid.Cid, depth int) (uint64, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	if depth > maxFileDepth {
		return 0, fmt.Errorf("%w: traversal depth exceeds %d", ErrInvalidPublicFile, maxFileDepth)
	}
	s.occurrences++
	if s.occurrences > maxFileNodeOccurrences {
		return 0, fmt.Errorf("%w: traversal exceeds %d node occurrences", ErrInvalidPublicFile, maxFileNodeOccurrences)
	}

	data, err := s.loadVerifiedBlock(id)
	if err != nil {
		return 0, err
	}
	if int64(len(data)) > maxFileEncodedBytes-s.encodedProcessed {
		return 0, fmt.Errorf("%w: traversal exceeds %d encoded bytes", ErrInvalidPublicFile, maxFileEncodedBytes)
	}
	s.encodedProcessed += int64(len(data))
	block, err := validateFileBlock(id, data)
	if err != nil {
		return 0, err
	}
	if err := s.write(block.data); err != nil {
		return 0, err
	}
	actual := uint64(len(block.data))
	for i, link := range block.links {
		childSize, err := s.copyNode(link.Cid, depth+1)
		if err != nil {
			return 0, err
		}
		if childSize != block.blockSizes[i] {
			return 0, fmt.Errorf("%w: child %d decoded size %d does not equal table size %d", ErrInvalidPublicFile, i, childSize, block.blockSizes[i])
		}
		if actual > uint64(maxPublicFileBytes)-childSize {
			return 0, fmt.Errorf("%w: decoded subtree size overflow", ErrInvalidPublicFile)
		}
		actual += childSize
	}
	if actual != block.fileSize {
		return 0, fmt.Errorf("%w: decoded subtree size %d does not equal UnixFS size %d", ErrInvalidPublicFile, actual, block.fileSize)
	}
	return actual, nil
}

func (s *fileCopyState) loadVerifiedBlock(id cid.Cid) ([]byte, error) {
	block, err := s.node.Blockstore.Get(s.ctx, id)
	if err != nil {
		if !format.IsNotFound(err) {
			return nil, fmt.Errorf("getting local public file block %s: %w", id, err)
		}
		block, err = s.node.Bitswap.GetBlock(s.ctx, id)
		if err != nil {
			return nil, fmt.Errorf("getting public file block %s: %w", id, err)
		}
	}
	data := block.RawData()
	if len(data) > maxEncodedFileBlockBytes {
		return nil, fmt.Errorf("%w: encoded block exceeds %d bytes", ErrInvalidPublicFile, maxEncodedFileBlockBytes)
	}
	computed, hashErr := id.Prefix().Sum(data)
	if hashErr != nil || computed != id {
		return nil, fmt.Errorf("%w: block bytes do not match CID %s", ErrInvalidPublicFile, id)
	}
	return data, nil
}

func (s *fileCopyState) write(data []byte) error {
	if len(data) == 0 {
		return s.ctx.Err()
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if int64(len(data)) > s.remaining {
		data = data[:s.remaining]
		if len(data) == 0 {
			return fmt.Errorf("%w: decoded output exceeds declared length", ErrInvalidPublicFile)
		}
		n, err := s.writeOnce(data)
		if err != nil {
			return err
		}
		if n != len(data) {
			return io.ErrShortWrite
		}
		return fmt.Errorf("%w: decoded output exceeds declared length", ErrInvalidPublicFile)
	}
	_, err := s.writeOnce(data)
	return err
}

func (s *fileCopyState) writeOnce(data []byte) (int, error) {
	n, err := s.dst.Write(data)
	if n < 0 || n > len(data) {
		return 0, fmt.Errorf("%w: destination returned invalid count %d for buffer %d", ErrInvalidPublicFile, n, len(data))
	}
	s.written += int64(n)
	s.remaining -= int64(n)
	if err != nil {
		return n, err
	}
	if n != len(data) {
		return n, io.ErrShortWrite
	}
	if err := s.ctx.Err(); err != nil {
		return n, err
	}
	return n, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	kind := reflect.ValueOf(value).Kind()
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflect.ValueOf(value).IsNil()
	default:
		return false
	}
}
