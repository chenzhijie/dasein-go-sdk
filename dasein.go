package dasein_go_sdk

import (
	"bytes"
	"context"
	"fmt"

	"github.com/daseinio/dasein-go-sdk/core"
	ml "github.com/daseinio/dasein-go-sdk/merkledag"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"

	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type Client struct {
	node *core.IpfsNode
}

func NewClient() (*Client, error) {
	client := &Client{nil}
	repo, err := fsrepo.Open("/Users/ggxxjj123/ipfs_test/ipfs2")
	if err != nil {
		fmt.Printf(err.Error())
	}

	nCfg := &core.BuildCfg{
		Repo:      repo,
		Permanent: true, // It is temporary way to signify that node is permanent
		ExtraOpts: map[string]bool{
			"mplex": false,
		},
	}
	client.node, err = core.NewNode(context.TODO(), nCfg)
	return client, err
}

func (c *Client) GetData(cidString string) ([]byte, error) {
	CID, err := cid.Decode(cidString)
	if err != nil {
		return nil, err
	}

	return c.decodeBlock(CID)
}

func (c *Client) RmData(cidString string) error {
	//c.node.PeerHost.Network().ConnsToPeer()

	return nil
}

func (c *Client) decodeBlock(CID *cid.Cid) ([]byte, error) {
	var buf bytes.Buffer
	block, err := c.node.Exchange.GetBlock(context.Background(), CID)
	if err != nil {
		return nil, err
	}

	dagNode, err := ml.DecodeProtobufBlock(block)
	if err != nil {
		return nil, err
	}

	linksNum := len(dagNode.Links())
	if linksNum == 0 {
		pb := new(ftpb.Data)
		if err := proto.Unmarshal(dagNode.(*ml.ProtoNode).Data(), pb); err != nil {
			return nil, err
		}
		n, err := buf.Write(pb.Data)
		if err != nil || n == 0 {
			return nil, err
		}
	} else {
		for i := 0; i < linksNum; i++ {
			childBuf, err := c.decodeBlock(dagNode.Links()[i].Cid)
			if err != nil {
				return nil, err
			}
			buf.Write(childBuf)
		}
	}
	return buf.Bytes(), nil
}
