package dasein_go_sdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	chunker "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
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
	MAX_ADD_BLOCKS_SIZE     = 10 // max add blocks size by sending msg
	MAX_RETRY_REQUEST_TIMES = 6  // max request retry times
	MAX_REQUEST_TIMEWAIT    = 10 // request time wait in second
	MIN_CHALLENGE_RATE      = 10 // min challenge blocks count <==> rate
	MIN_CHALLENGE_TIMES     = 2  // min challenge times
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

// GetData get a block from a specific remote node
// If the block is a root dag node, this function will get all blocks recursively
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
	retry := 0
	for {
		log.Debugf("try get pledge")
		if retry < MAX_RETRY_REQUEST_TIMES {
			err := request.GetReadFilePledge(cidString)
			if err == nil {
				log.Debug("loop get pledge success")
				break
			} else {
				log.Debugf("get pledge:%s failed:%s", cidString, err)
			}
			retry++
		}
		log.Debugf("get pledge failed")
		time.Sleep(time.Duration(MAX_REQUEST_TIMEWAIT) * time.Second)
	}
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

// DelData delete a block of all remote nodes which keep the block
// If the block is a dag root node, the nodes will delete all blocks recusively
func (c *Client) DelData(cidString string) error {
	CID, err := cid.Decode(cidString)
	if err != nil {
		return err
	}
	return c.node.Exchange.DelBlock(context.Background(), c.peer.ID(), CID)
}

// PreSendFile send file information to node for checking the storage requirement
// This function does not send any block data.
func (c *Client) PreSendFile(root ipld.Node, list []*helpers.UnixfsNode, copyNum int32, nodeList []string) error {
	cids := make([]*cid.Cid, 0)
	cids = append(cids, root.Cid())
	for _, node := range list {
		dagNode, _ := node.GetDagNode()
		if dagNode.Cid().String() != root.Cid().String() {
			cids = append(cids, dagNode.Cid())
		}
	}
	log.Debugf("all cids length:%d", len(cids))
	return c.node.Exchange.PreAddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), cids, copyNum, nodeList)
}

// SendFile send a file to node with copy number
func (c *Client) SendFile(fileName string, challengeRate uint64, challengeTimes uint64, copyNum int32, encrypt bool, encryptPassword string) error {
	if challengeRate < MIN_CHALLENGE_RATE {
		return fmt.Errorf("challenge rate must more than %d", MIN_CHALLENGE_RATE)
	}
	if challengeTimes < MIN_CHALLENGE_TIMES {
		return fmt.Errorf("challenge times must more than %d", MIN_CHALLENGE_TIMES)
	}
	fileStat, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return errors.New("init contract requester failed in SendFile")

	}

	// get nodeList
	nodeList, err := request.GetNodeList(uint64(fileStat.Size()), copyNum)
	if err != nil {
		return err
	}

	// split file to blocks
	root, list, err := nodesFromFile(fileName, encrypt, encryptPassword)
	if err != nil {
		return err
	}
	fileHashStr := root.Cid().String()
	log.Debugf("root:%s, list.len:%d", fileHashStr, len(list))

	fileInfo, _ := request.GetFileInfo(fileHashStr)

	g, g0, pubKey, privKey, fileID, r, pairing := PoR.Init(fileName)
	if fileInfo == nil {
		blockSize, err := root.Size()
		if err != nil || blockSize == 0 {
			return err
		}
		blockSizeInKB := uint64(math.Ceil(float64(blockSize) / 1024.0))
		// prepay for store file
		storeFileInfo := &StoreFileInfo{
			FileHashStr:    root.Cid().String(),
			ChallengeRate:  challengeRate,
			ChallengeTimes: challengeTimes,
			CopyNum:        uint64(copyNum),
			BlockNum:       uint64(len(list) + 1),
			BlockSize:      blockSizeInKB,
		}
		log.Debugf("pay blocksize:%d, inkb:%d, blockNum:%d", blockSize, blockSizeInKB, len(list)+1)
		paramsBuf, err := request.ProveParamSer(g, g0, pubKey, []byte(fileID), r, pairing)
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
		retry := 0
		for {
			if retry < MAX_RETRY_REQUEST_TIMES {
				payOK, _ := request.IsFilePaid(storeFileInfo.FileHashStr)
				if payOK {
					log.Debug("loop check paid success")
					break
				}
				retry++
			}
			time.Sleep(time.Duration(MAX_REQUEST_TIMEWAIT) * time.Second)
		}
	} else {
		return fmt.Errorf("file:%s has stored", fileHashStr)
		// g, g0, pubKey, fileID, r, pairing, err = request.GetFileProveParams(fileInfo.FileProveParam)
		// if err != nil {
		// 	return err
		// }
		// storedNodes, err := request.GetStoreFileNodes(root.Cid().String())
		// if err != nil {
		// 	log.Errorf("get store file nodes err:%s", err)
		// 	return err
		// }

		// if copyNum <= int32(len(storedNodes)) {
		// 	return fmt.Errorf("the file:%s already store in %d nodes", root.Cid().String(), len(storedNodes))
		// }
		// // update nodelist
		// newNodeList := make([]string, 0)
		// for _, node := range nodeList {
		// 	exist := false
		// 	for _, storedNode := range storedNodes {
		// 		if storedNode.Addr == node {
		// 			exist = true
		// 			break
		// 		}
		// 	}
		// 	if !exist {
		// 		newNodeList = append(newNodeList, node)
		// 	}
		// }
		// log.Debugf("oldnodelist:%v, newNodelist:%v, storedNode:%v", nodeList, newNodeList, storedNodes)
		// if copyNum+1 < int32(len(newNodeList)) {
		// 	return fmt.Errorf("the file:%s copynum :%d less than nodelist len: %d ", root.Cid().String(), copyNum, len(newNodeList))
		// }
		// nodeList = newNodeList
	}

	var server string
	for _, s := range nodeList {
		core.InitParam(s)
		c.node, err = core.NewNode(context.TODO())
		c.peer, err = config.ParseBootstrapPeer(s)
		if err != nil {
			log.Debugf("parse bootstrap peer failed:%s", err)
			continue
		}
		peerInfo := c.node.Peerstore.PeerInfo(c.peer.ID())
		log.Debugf("addrs:%v, connectedness:%d\n", peerInfo.Addrs, c.node.PeerHost.Network().Connectedness(c.peer.ID()))
		if len(peerInfo.Addrs) == 0 || c.node.PeerHost.Network().Connectedness(c.peer.ID()) != net.Connected {
			continue
		}
		server = s
		break
	}
	if len(server) == 0 {
		return fmt.Errorf("no online nodes from nodeList")
	}
	log.Debugf("nodelist:%v, store to :%s", nodeList, server)

	for i, n := range nodeList {
		if n == server {
			nodeList = append(nodeList[:i], nodeList[i+1:]...)
			break
		}
	}
	log.Debugf("has paid file:%s, blocknum:%d rate:%d, times:%d, copynum:%d", root.Cid().String(), len(list), challengeRate, challengeTimes, copyNum)
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
	log.Debugf("root tag:%v, r:%s, pari:%s", tag, r, pairing)
	// send root node
	ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), []blocks.Block{root}, []int32{0}, [][]byte{tag}, copyNum, nodeList)
	log.Infof("add root file to:%s ret:%v, err:%s", c.peer.ID(), ret, err)
	// log.Debugf("r:%s, pari:%s, tag:%v, hash:%s", r, pairing, tag, root.Cid().String())
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
			log.Debugf("index:%v tag:%v, hash:%s", otherIndxs, tag, dagNode.Cid().String())

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
	// var buf bytes.Buffer
	blocks, err := c.node.Exchange.GetBlocks(context.Background(), peerId, fileHashStr, CID, settleSlice)
	if err != nil {
		return nil, err
	}

	dagNode, err := ml.DecodeProtobufBlock(blocks[0])
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(fileHashStr, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	linksNum := len(dagNode.Links())
	if linksNum == 0 {
		pb := new(ftpb.Data)
		if err := proto.Unmarshal(dagNode.(*ml.ProtoNode).Data(), pb); err != nil {
			return nil, err
		}
		n, err := file.Write(pb.Data)
		if err != nil || n == 0 {
			return nil, err
		}
	} else {
		for i := 0; i < linksNum; i++ {
			_, err := c.decodeBlock(dagNode.Links()[i].Cid, peerId, fileHashStr, request, nodeWalletAddr, blockSize)
			if err != nil {
				return nil, err
			}
			// buf.Write(childBuf)
		}
	}
	// return buf.Bytes(), nil
	return nil, nil
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
// return root, list, list not include root node
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

	var root ipld.Node
	var list []*helpers.UnixfsNode
	if tri {
		root, list, err = trickle.Layout(db)
	} else {
		root, list, err = balanced.Layout(db)
	}
	if err != nil {
		return root, list, err
	}

	for index, l := range list {
		lNode, err := l.GetDagNode()
		if err != nil {
			continue
		}
		if lNode.Cid().String() == root.Cid().String() {
			list = append(list[:index], list[index+1:]...)
			break
		}
	}
	return root, list, nil
}
