package dasein_go_sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
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
	"github.com/daseinio/dasein-go-sdk/crypto"
	"github.com/daseinio/dasein-go-sdk/importer/balanced"
	"github.com/daseinio/dasein-go-sdk/importer/helpers"
	"github.com/daseinio/dasein-go-sdk/importer/trickle"
	ml "github.com/daseinio/dasein-go-sdk/merkledag"
	"github.com/daseinio/dasein-go-sdk/repo/config"
	ftpb "github.com/daseinio/dasein-go-sdk/unixfs/pb"
	"github.com/howeyc/gopass"
)

var log = logging.Logger("daseingosdk")

const (
	MAX_ADD_BLOCKS_SIZE = 10 // max add blocks size by sending msg
)

type Client struct {
	node   *core.IpfsNode
	peer   config.BootstrapPeer
	wallet string
	rpc    string
}

func NewClient(server, wallet, rpc string) (*Client, error) {
	var err error
	client := &Client{
		wallet: wallet,
		rpc:    rpc,
	}

	core.InitParam(server)
	client.node, err = core.NewNode(context.TODO())
	client.peer, err = config.ParseBootstrapPeer(server)
	return client, err
}

func (c *Client) GetData(cidString string) ([]byte, error) {
	CID, err := cid.Decode(cidString)
	if err != nil {
		return nil, err
	}
	return c.decodeBlock(CID, c.peer.ID())
}

func (c *Client) DelData(cidString string) error {
	CID, err := cid.Decode(cidString)
	if err != nil {
		return err
	}
	return c.node.Exchange.DelBlock(context.Background(), c.peer.ID(), CID)
}

// PreSendFile send file information to node for checking the storage requirement
func (c *Client) PreSendFile(root ipld.Node, list []*helpers.UnixfsNode, copyNum int32, nodeList []string) error {
	cids := make([]*cid.Cid, 0)
	cids = append(cids, root.Cid())
	for _, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			cids = append(cids, dagNode.Cid())
		}
	}
	return c.node.Exchange.PreAddBlocks(context.Background(), c.peer.ID(), cids, copyNum, nodeList)
}

// SendFile send a file to node with copy number
func (c *Client) SendFile(fileName string, keepHours uint64, challengeRate uint64, challengeTimes uint64, copyNum int32, encrypt bool, password string) error {
	password, err := getPassword()
	if err != nil {
		return err
	}

	// get nodeList
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	nodeList, err := GetNodeList(uint64(fileInfo.Size()), copyNum, c.wallet, password, c.rpc)
	if err != nil {
		return err
	}

	exist, err := c.isPeerAlive(c.peer.ID())
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("peer is not alive:%s", c.peer.ID())
	}
	nodeList = append(nodeList[:0], nodeList[1:]...)

	// split file to blocks
	root, list, err := nodesFromFile(fileName, encrypt, password)
	if err != nil {
		return err
	}

	// check if file has paid
	isPaid, err := IsFilePaid(root.Cid().String(), c.wallet, password, c.rpc)
	if err != nil {
		return err
	}

	if !isPaid {
		blockSize, err := root.Size()
		if err != nil || blockSize == 0 {
			return err
		}
		blockSizeInKB := uint64(math.Ceil(float64(blockSize) / 1024.0))
		// prepay for store file
		storeFileInfo := &StoreFileInfo{
			FileHashStr:    root.Cid().String(),
			KeepHours:      keepHours,
			ChallengeRate:  challengeRate,
			ChallengeTimes: challengeTimes,
			CopyNum:        uint64(copyNum),
			BlockNum:       uint64(len(list) + 1),
			BlockSize:      blockSizeInKB,
		}
		log.Infof("store file info :%v", storeFileInfo)
		rawTxId, err := PayStoreFile(storeFileInfo, c.wallet, password, c.rpc)
		if err != nil {
			log.Errorf("pay store file order failed:%s", err)
			return err
		}
		log.Infof("pay store file success:%x", rawTxId)
	} else {
		log.Infof("file:%s already paied", root.Cid().String())
	}

	if copyNum > 0 {
		err = c.PreSendFile(root, list, copyNum, nodeList)
		if err != nil {
			log.Errorf("pre send file failed :%s", err)
			return err
		}
		log.Infof("pre add blocks success")
	}

	// send root node
	ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), []blocks.Block{root}, copyNum, nodeList)
	if err != nil {
		return err
	}
	if copyNum > 0 {
		log.Infof("send %s success, result:%v", root.Cid(), ret)
	} else {
		log.Infof("send %s success", root.Cid())
	}

	// send rest nodes
	// blocks size in one msg
	blockSizePerMsg := 2
	if blockSizePerMsg > MAX_ADD_BLOCKS_SIZE {
		blockSizePerMsg = MAX_ADD_BLOCKS_SIZE
	}
	others := make([]blocks.Block, 0)
	for i, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			// send others
			others = append(others, dagNode)
			if len(others) >= blockSizePerMsg || i == len(list)-1 {
				ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), others, copyNum, nodeList)
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

func (c *Client) decodeBlock(CID *cid.Cid, peerId peer.ID) ([]byte, error) {
	var buf bytes.Buffer
	blocks, err := c.node.Exchange.GetBlocks(context.Background(), peerId, CID)
	if err != nil {
		return nil, err
	}

	dagNode, err := ml.DecodeProtobufBlock(blocks[0])
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
			childBuf, err := c.decodeBlock(dagNode.Links()[i].Cid, peerId)
			if err != nil {
				return nil, err
			}
			buf.Write(childBuf)
		}
	}
	return buf.Bytes(), nil
}

func (c *Client) isPeerAlive(idStr peer.ID) (bool, error) {
	// id, err := peer.IDB58Decode(idStr)
	// if err != nil {
	// 	return false, err
	// }
	pi := c.node.Peerstore.PeerInfo(idStr)
	if len(pi.Addrs) == 0 {
		return false, fmt.Errorf("peer not found")
	}
	return true, nil
}

// nodesFromFile open a local file and build dag nodes
func nodesFromFile(fileName string, encrypt bool, password string) (ipld.Node, []*helpers.UnixfsNode, error) {
	cidVer := 0
	hashFunStr := "sha2-256"
	file, err := os.Open(fileName)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	var reader io.Reader = file
	if encrypt {
		encryptedR, err := crypto.AESEncryptFileReader(file, password)
		if err != nil {
			return nil, nil, err
		}
		reader = encryptedR
	}

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

func getPassword() (string, error) {
	fmt.Printf("Password:")
	pwd, err := gopass.GetPasswd()
	if err != nil {
		return "", err
	}
	return string(pwd), nil
}
