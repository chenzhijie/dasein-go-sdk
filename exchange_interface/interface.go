package exchange_interface

import (
	"io"
	"context"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// Interface defines the functionality of the IPFS block exchange protocol.
type Exchange interface { // type Exchanger interface
	GetBlocks(context.Context, peer.ID, *cid.Cid) ([]blocks.Block, error)

	DelBlock(context.Context, peer.ID, *cid.Cid) error
	DelBlocks(context.Context, peer.ID, []*cid.Cid) error

	PreAddBlocks(context.Context, peer.ID, []*cid.Cid, int32, []string) error
	AddBlocks(context.Context, peer.ID, []blocks.Block, int32, []string) (interface{}, error)
	IsOnline() bool

	io.Closer
}
