package network

import (
	"testing"

	"github.com/ipfs/boxo/ipld/merkledag"
	unixfs "github.com/ipfs/boxo/ipld/unixfs"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func FuzzMEDIA001FileNode(f *testing.F) {
	raw := []byte("raw seed")
	rawNode, err := merkledag.NewRawNodeWPrefix(raw, ufsio.UnixFS_v1_2025.CidBuilder())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(byte(0), rawNode.Cid().Bytes(), raw)

	fs := unixfs.NewFSNode(unixfs.TFile)
	fs.SetData([]byte("inline seed"))
	payload, err := fs.GetBytes()
	if err != nil {
		f.Fatal(err)
	}
	pb := new(merkledag.ProtoNode)
	pb.SetCidBuilder(ufsio.UnixFS_v1_2025.CidBuilder())
	pb.SetData(payload)
	encoded := pb.RawData()
	f.Add(byte(1), pb.Cid().Bytes(), encoded)
	f.Add(byte(2), []byte{1, 2, 3}, []byte{0xff, 0, 1})

	f.Fuzz(func(t *testing.T, mode byte, cidBytes, data []byte) {
		var id cid.Cid
		var err error
		switch mode % 3 {
		case 0:
			id, err = cid.Prefix{Version: 1, Codec: cid.Raw, MhType: mh.SHA2_256, MhLength: 32}.Sum(data)
		case 1:
			id, err = cid.Prefix{Version: 1, Codec: cid.DagProtobuf, MhType: mh.SHA2_256, MhLength: 32}.Sum(data)
		default:
			id, err = cid.Cast(cidBytes)
		}
		if err != nil {
			return
		}
		_, _ = validateFileBlock(id, data)
	})
}
