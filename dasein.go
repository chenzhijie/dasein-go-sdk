package dasein_go_sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/daseinio/dasein-go-sdk/bitswap"
	"github.com/daseinio/dasein-go-sdk/bitswap/message"
	"github.com/daseinio/dasein-go-sdk/core"
	"github.com/daseinio/dasein-go-sdk/importer/balanced"
	"github.com/daseinio/dasein-go-sdk/importer/helpers"
	"github.com/daseinio/dasein-go-sdk/importer/trickle"
	ml "github.com/daseinio/dasein-go-sdk/merkledag"
	"github.com/daseinio/dasein-go-sdk/repo/fsrepo"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"

	chunker "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

var repo = "/Users/ggxxjj123/ipfs_test/ipfs2"

// var repo = "/Users/zhijie/Desktop/onchain/ipfs-test/ipfs/node2"

type Client struct {
	node *core.IpfsNode
}

func NewClient() (*Client, error) {
	client := &Client{nil}
	repo, err := fsrepo.Open(repo)
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

func (c *Client) DelData(cidString string, from string) error {
	id, err := peer.IDB58Decode(from)
	if err != nil {
		return err
	}
	pi := c.node.Peerstore.PeerInfo(id)
	if len(pi.Addrs) == 0 {
		return fmt.Errorf("peer not found")
	}

	CID, err := cid.Decode(cidString)
	if err != nil {
		return err
	}
	return c.node.Exchange.DelBlock(context.Background(), CID)
}

func (c *Client) SendFile(fileName string, to string, copynum int32, nodelist []string) error {
	id, err := peer.IDB58Decode(to)
	if err != nil {
		return err
	}
	pi := c.node.Peerstore.PeerInfo(id)
	if len(pi.Addrs) == 0 {
		return fmt.Errorf("peer not found")
	}
	bitswap := c.node.Exchange.(*bitswap.Bitswap)
	bsnet := bitswap.Network()

	root, list, err := nodesFromFile(fileName)
	if err != nil {
		fmt.Printf(err.Error())
	}

	// send root node
	msg := message.New(true)
	msg.AddBlock(root)
	msg.SetBackup(copynum, nodelist)
	err = bsnet.SendMessage(context.TODO(), id, msg)
	if err != nil {
		return err
	}
	fmt.Printf("seend %s success\n", root.Cid())

	for _, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			// send others
			msg := message.New(true)
			msg.AddBlock(dagNode)
			msg.SetBackup(copynum, nodelist)
			err = bsnet.SendMessage(context.TODO(), id, msg)
			if err != nil {
				return err
			}
			fmt.Printf("send %s success, size:%d\n", dagNode.Cid(), len(dagNode.RawData()))
		}
	}
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

func nodesFromFile(fileName string) (ipld.Node, []*helpers.UnixfsNode, error) {
	cidVer := 0
	hashFunStr := "sha2-256"
	file, err := os.Open(fileName)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	var reader io.Reader = file
	chnk, err := chunker.FromString(reader, "size-262144")
	if err != nil {
		return nil, nil, err
	}

	prefix, err := ml.PrefixForCidVersion(cidVer)
	if err != nil {
		return nil, nil, err
	}

	hashFunCode, _ := mh.Names[strings.ToLower(hashFunStr)]
	if err != nil {
		return nil, nil, err
	}
	prefix.MhType = hashFunCode
	prefix.MhLength = -1

	tri := false

	params := &helpers.DagBuilderParams{
		RawLeaves: false,
		Prefix:    &prefix,
		Maxlinks:  helpers.DefaultLinksPerBlock,
		NoCopy:    false,
	}
	db := params.New(chnk)

	if tri {
		return trickle.Layout(db)
	}
	return balanced.Layout(db)
}
