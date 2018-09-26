package dasein_go_sdk

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/daseinio/dasein-go-PoR/PoR"
	"github.com/daseinio/dasein-go-sdk/core"
	"github.com/daseinio/dasein-go-sdk/crypto"
	"github.com/daseinio/dasein-go-sdk/importer/balanced"
	"github.com/daseinio/dasein-go-sdk/importer/helpers"
	"github.com/daseinio/dasein-go-sdk/importer/trickle"
	ml "github.com/daseinio/dasein-go-sdk/merkledag"
	"github.com/daseinio/dasein-go-sdk/repo/config"
	ftpb "github.com/daseinio/dasein-go-sdk/unixfs/pb"
	"github.com/ontio/ontology/common"
)

var log = logging.Logger("daseingosdk")

const (
	MAX_ADD_BLOCKS_SIZE = 10 // max add blocks size by sending msg
)

type Client struct {
	node      *core.IpfsNode
	peer      config.BootstrapPeer
	wallet    string
	walletPwd string
	rpc       string
	rfm       *ReadFileMgr
}

func NewClient(server, wallet, password, rpc string) (*Client, error) {
	var err error
	client := &Client{
		wallet:    wallet,
		walletPwd: password,
		rpc:       rpc,
	}

	if len(server) > 0 {
		core.InitParam(server)
		client.node, err = core.NewNode(context.TODO())
		client.peer, err = config.ParseBootstrapPeer(server)
	}
	client.rfm = NewReadFileMgr()
	return client, err
}

func (c *Client) GetData(cidString string, nodeWalletAddr common.Address) ([]byte, error) {
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return nil, errors.New("init contract requester fail in GetData")
	}
	fileInfo, err := request.GetFileInfo(cidString)
	if err != nil {
		return nil, err
	}
	fee, err := request.CalculateReadFee(fileInfo, cidString)
	if err != nil {
		return nil, err
	}
	log.Infof("fee:%d", fee)
	txHash, err := request.PledgeForReadFile(cidString, nodeWalletAddr, fee)
	if err != nil {
		return nil, err
	}
	log.Debugf("txHash:%x", txHash)
	CID, err := cid.Decode(cidString)
	if err != nil {
		return nil, err
	}

	buf, err := c.decodeBlock(CID, c.peer.ID(), cidString, request, nodeWalletAddr, fileInfo.BlockSize)
	if err != nil {
		return nil, err
	}
	c.rfm.RemoveSliceId(cidString)
	return buf, err
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
	return c.node.Exchange.PreAddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), cids, copyNum, nodeList)
}

// SendFile send a file to node with copy number
func (c *Client) SendFile(fileName string, challengeRate uint64, challengeTimes uint64, copyNum int32, encrypt bool, encryptPassword string) error {
	// get nodeList
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return errors.New("init contract requester failed in SendFile")
	}
	nodeList, err := request.GetNodeList(uint64(fileInfo.Size()), copyNum)
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

	peer := peer.IDB58Encode(c.peer.ID())
	for i, n := range nodeList {
		if n == peer {
			nodeList = append(nodeList[:i], nodeList[i+1:]...)
			break
		}
	}
	// split file to blocks
	root, list, err := nodesFromFile(fileName, encrypt, encryptPassword)
	if err != nil {
		return err
	}

	// check if file has paid
	isPaid, err := request.IsFilePaid(root.Cid().String())
	if err != nil {
		return err
	}

	g, g0, pubKey, privKey, fileID, r, pairing := PoR.Init(fileName)
	if !isPaid {
		blockSize, err := root.Size()
		if err != nil || blockSize == 0 {
			return err
		}
		blockSizeInKB := uint64(math.Ceil(float64(blockSize) / 1024.0))
		log.Debugf("blocksize:%d, inkb:%d", blockSize, blockSizeInKB)
		// prepay for store file
		storeFileInfo := &StoreFileInfo{
			FileHashStr:    root.Cid().String(),
			ChallengeRate:  challengeRate,
			ChallengeTimes: challengeTimes,
			CopyNum:        uint64(copyNum),
			BlockNum:       uint64(len(list)),
			BlockSize:      blockSizeInKB,
		}
		paramsBuf, err := request.ProveParamSer(g, g0, pubKey, []byte(fileID), r)
		if err != nil {
			log.Errorf("serialzation prove params failed:%s", err)
			return err
		}
		rawTxId, err := request.PayStoreFile(storeFileInfo, paramsBuf)
		if err != nil {
			log.Errorf("pay store file order failed:%s", err)
			return err
		}
		log.Infof("txId:%x", rawTxId)
	} else {
	}
	err = c.PreSendFile(root, list, copyNum, nodeList)
	if err != nil {
		log.Errorf("pre send file failed :%s", err)
		return err
	}
	// index of root tag start from 1
	tag, err := PoR.SignGenerate(root.RawData(), fileID, r, 1, pairing, g0, privKey)
	if err != nil {
		log.Errorf("generate root tag failed:%s", err)
		return err
	}
	log.Infof("root tag:%d", len(tag))

	// send root node
	ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), []blocks.Block{root}, []int32{0}, [][]byte{tag}, copyNum, nodeList)
	log.Infof("add root file to:%s ret:%v, err:%s", c.peer.ID(), ret, err)
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
	blockSizePerMsg := 1
	if blockSizePerMsg > MAX_ADD_BLOCKS_SIZE {
		blockSizePerMsg = MAX_ADD_BLOCKS_SIZE
	}
	otherBlks := make([]blocks.Block, 0)
	otherIndxs := make([]int32, 0)
	otherTags := make([][]byte, 0)
	index := int32(0)
	for i, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			index++
			// send others
			otherBlks = append(otherBlks, dagNode)
			otherIndxs = append(otherIndxs, index)
			tag, err := PoR.SignGenerate(dagNode.RawData(), fileID, r, uint32(index+1), pairing, g0, privKey)
			if err != nil {
				log.Errorf("generate %d tag failed:%s", index+1, err)
				return err
			}
			otherTags = append(otherTags, tag)
			log.Debugf("index:%v", otherIndxs)
			if len(otherBlks) >= blockSizePerMsg || i == len(list)-1 {
				ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), otherBlks, otherIndxs, otherTags, copyNum, nodeList)
				if err != nil {
					return err
				}
				for _, sent := range otherBlks {
					if copyNum > 0 {
						log.Debugf("send %s success, size:%d, result:%v", sent.Cid(), len(sent.RawData()), ret)
					} else {
						log.Debugf("send %s success, size:%d", sent.Cid(), len(sent.RawData()))
					}
				}
				//clean slice
				otherBlks = otherBlks[:0]
				otherIndxs = otherIndxs[:0]
				otherTags = otherTags[:0]
			}
		} else {
		}
	}
	log.Infof("File have stored: %s", root.Cid().String())
	return nil
}

func (c *Client) decodeBlock(CID *cid.Cid, peerId peer.ID, fileHashStr string, request *ContractRequest, nodeWalletAddr common.Address, blockSize uint64) ([]byte, error) {
	// sliceId equal to blockIndex
	sliceId := c.rfm.IncreAndGetSliceId(fileHashStr)
	settleSlice, err := request.GenFileReadSettleSlice(fileHashStr, nodeWalletAddr, sliceId, blockSize, sliceId)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	blocks, err := c.node.Exchange.GetBlocks(context.Background(), peerId, fileHashStr, CID, settleSlice)
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
			childBuf, err := c.decodeBlock(dagNode.Links()[i].Cid, peerId, fileHashStr, request, nodeWalletAddr, blockSize)
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
// return root, list, list include root node
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
