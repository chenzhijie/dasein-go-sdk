package testu

import (
	"context"
	"fmt"
	"io"
	mdag "github.com/daseinio/dasein-go-sdk/merkledag"
	ft "github.com/daseinio/dasein-go-sdk/unixfs"

	chunker "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// SizeSplitterGen creates a generator.
func SizeSplitterGen(size int64) chunker.SplitterGen {
	return func(r io.Reader) chunker.Splitter {
		return chunker.NewSizeSplitter(r, size)
	}
}

// NodeOpts is used by GetNode, GetEmptyNode and GetRandomNode
type NodeOpts struct {
	Prefix cid.Prefix
	// ForceRawLeaves if true will force the use of raw leaves
	ForceRawLeaves bool
	// RawLeavesUsed is true if raw leaves or either implicitly or explicitly enabled
	RawLeavesUsed bool
}

// Some shorthands for NodeOpts.
var (
	UseProtoBufLeaves = NodeOpts{Prefix: mdag.V0CidPrefix()}
	UseRawLeaves      = NodeOpts{Prefix: mdag.V0CidPrefix(), ForceRawLeaves: true, RawLeavesUsed: true}
	UseCidV1          = NodeOpts{Prefix: mdag.V1CidPrefix(), RawLeavesUsed: true}
	UseBlake2b256     NodeOpts
)

func init() {
	UseBlake2b256 = UseCidV1
	UseBlake2b256.Prefix.MhType = mh.Names["blake2b-256"]
	UseBlake2b256.Prefix.MhLength = -1
}

// ArrComp checks if two byte slices are the same.
func ArrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("arrays differ at index: %d", i)
		}
	}
	return nil
}

// PrintDag pretty-prints the given dag to stdout.
func PrintDag(nd *mdag.ProtoNode, ds ipld.DAGService, indent int) {
	pbd, err := ft.FromBytes(nd.Data())
	if err != nil {
		panic(err)
	}

	for i := 0; i < indent; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("{size = %d, type = %s, children = %d", pbd.GetFilesize(), pbd.GetType().String(), len(pbd.GetBlocksizes()))
	if len(nd.Links()) > 0 {
		fmt.Println()
	}
	for _, lnk := range nd.Links() {
		child, err := lnk.GetNode(context.Background(), ds)
		if err != nil {
			panic(err)
		}
		PrintDag(child.(*mdag.ProtoNode), ds, indent+1)
	}
	if len(nd.Links()) > 0 {
		for i := 0; i < indent; i++ {
			fmt.Print(" ")
		}
	}
	fmt.Println("}")
}
