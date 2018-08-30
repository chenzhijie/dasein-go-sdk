package dasein_go_sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	chunker "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"

	"github.com/daseinio/dasein-go-sdk/core"
	"github.com/daseinio/dasein-go-sdk/importer/balanced"
	"github.com/daseinio/dasein-go-sdk/importer/helpers"
	"github.com/daseinio/dasein-go-sdk/importer/trickle"
	ml "github.com/daseinio/dasein-go-sdk/merkledag"
	ftpb "github.com/daseinio/dasein-go-sdk/unixfs/pb"
)

var log = logging.Logger("daseingosdk")

const (
	MAX_ADD_BLOCKS_SIZE = 10 // max add blocks size by sending msg
)

type Client struct {
	node *core.IpfsNode
}

func Init(server string) {
	core.InitParam(server)
}

func NewClient() (*Client, error) {
	client := &Client{nil}
	var err error
	client.node, err = core.NewNode(context.TODO())
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

// PreSendFile send file information to node for checking the storage requirement
func (c *Client) PreSendFile(root ipld.Node, list []*helpers.UnixfsNode, to string, copyNum int32, nodeList []string) error {
	cids := make([]*cid.Cid, 0)
	cids = append(cids, root.Cid())
	for _, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			cids = append(cids, dagNode.Cid())
		}
	}
	return c.node.Exchange.PreAddBlocks(context.Background(), to, cids, copyNum, nodeList)
}

// SendFile send a file to node with copy number
func (c *Client) SendFile(fileName string, to string, copyNum int32, nodeList []string) error {
	// blocks size in one msg
	blockSizePerMsg := 2
	if blockSizePerMsg > MAX_ADD_BLOCKS_SIZE {
		blockSizePerMsg = MAX_ADD_BLOCKS_SIZE
	}

	id, err := peer.IDB58Decode(to)
	if err != nil {
		return err
	}
	pi := c.node.Peerstore.PeerInfo(id)
	if len(pi.Addrs) == 0 {
		return fmt.Errorf("peer not found")
	}
	root, list, err := nodesFromFile(fileName)
	if err != nil {
		return err
	}

	if copyNum > 0 {
		err = c.PreSendFile(root, list, to, copyNum, nodeList)
		if err != nil {
			log.Errorf("pre send file failed :%s", err)
			return err
		}
		log.Infof("pre add blocks success")
	}

	// send root node
	ret, err := c.node.Exchange.AddBlocks(context.Background(), to, []blocks.Block{root}, copyNum, nodeList)
	if err != nil {
		return err
	}
	if copyNum > 0 {
		log.Infof("send %s success, result:%v", root.Cid(), ret)
	} else {
		log.Infof("send %s success", root.Cid())
	}

	others := make([]blocks.Block, 0)
	for i, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			// send others
			others = append(others, dagNode)
			if len(others) >= blockSizePerMsg || i == len(list)-1 {
				ret, err := c.node.Exchange.AddBlocks(context.Background(), to, others, copyNum, nodeList)
				if err != nil {
					return err
				}
				for _, sent := range others {
					if copyNum > 0 {
						log.Infof("send %s success, size:%d, result:%v", sent.Cid(), len(sent.RawData()), ret)
					} else {
						log.Infof("send %s success, size:%d", sent.Cid(), len(sent.RawData()))
					}

				}
				//clean slice
				others = others[:0]
			}
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

// nodesFromFile open a local file and build dag nodes
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
