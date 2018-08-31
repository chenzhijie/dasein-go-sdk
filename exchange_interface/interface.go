package exchange_interface

import (
	"context"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
	"io"
)

// Interface defines the functionality of the IPFS block exchange protocol.
type Exchange interface { // type Exchanger interface
	GetBlocks(ctx context.Context, to string, key *cid.Cid) ([]blocks.Block, error)

	DelBlock(context.Context, *cid.Cid) error
	DelBlocks(context.Context, []*cid.Cid) error

	PreAddBlocks(context.Context, string, []*cid.Cid, int32, []string) error
	AddBlocks(context.Context, string, []blocks.Block, int32, []string) (interface{}, error)
	IsOnline() bool

	io.Closer


}
