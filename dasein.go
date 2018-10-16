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
	fm        *FileMgr
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
	client.fm = NewFileMgr()
	return client, err
}

// DelStoreFileInfo delete store file
func DelStoreFileInfo(fileHashStr string) error {
	fm := NewFileMgr()
	return fm.DelStoreFileInfo(fileHashStr)
}

// GetReadFileNodeInfo get have read node infomation
func GetReadFileNodeInfo(fileHashStr string) *NodeInfo {
	fm := NewFileMgr()
	err := fm.NewReadFile(fileHashStr)
	if err != nil {
		return nil
	}
	addr, walletAddr := fm.GetReadFileNodeInfo(fileHashStr)
	if len(addr) == 0 || len(walletAddr) == 0 {
		return nil
	}
	walletAddress, err := common.AddressFromBase58(walletAddr)
	if err != nil {
		log.Errorf("parse base58 address failed:%s", walletAddr)
		return nil
	}

	n := &NodeInfo{
		Addr:       addr,
		WalletAddr: walletAddress,
	}
	return n
}

// GetData get a block from a specific remote node
// If the block is a root dag node, this function will get all blocks recursively
func (c *Client) GetData(cidString, nodeAddr string, nodeWalletAddr common.Address) error {
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return errors.New("init contract requester fail in GetData")
	}
	fileInfo, err := request.GetFileInfo(cidString)
	if err != nil {
		return err
	}

	if !request.IsReadFilePledgeExist(cidString) {
		plans := make(map[common.Address]uint64, 0)
		plans[nodeWalletAddr] = fileInfo.FileBlockNum
		txHash, err := request.PledgeForReadFile(cidString, plans)
		if err != nil {
			return err
		}
		log.Debugf("txHash:%x", txHash)
		retry := 0
		for {
			log.Debugf("try get pledge")
			if retry > MAX_RETRY_REQUEST_TIMES {
				return errors.New("check read pledge tx failed")
			}
			if request.IsReadFilePledgeExist(cidString) {
				log.Debug("loop get pledge success")
				break
			}
			retry++
			time.Sleep(time.Duration(MAX_REQUEST_TIMEWAIT) * time.Second)
		}
	}
	log.Debug("has paid read pledge")
	err = c.fm.NewReadFile(cidString)
	if err != nil {
		return err
	}
	CID, err := cid.Decode(cidString)
	if err != nil {
		return err
	}

	err = c.decodeBlock(CID, c.peer.ID(), nodeAddr, nodeWalletAddr, cidString, request, fileInfo.FileBlockSize)
	if err != nil {
		return err
	}
	c.fm.RemoveReadFile(cidString)
	return err
}

// CancelGetData user want to cancel read file
// Should wait after expired block height
func (c *Client) CancelGetData(cidString string) error {
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return errors.New("init contract requester fail in GetData")
	}
	err := request.CancelReadPledge(cidString)
	log.Debugf("cancel err %s", err)
	if err != nil {
		return err
	}
	c.fm.RemoveReadFile(cidString)
	return nil
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

// SendFile send a file to node with copy number
func (c *Client) SendFile(fileName string, challengeRate uint64, challengeTimes uint64, copyNum int32, encrypt bool, encryptPassword string) (string, error) {
	if challengeRate < MIN_CHALLENGE_RATE {
		return "", fmt.Errorf("challenge rate must more than %d", MIN_CHALLENGE_RATE)
	}
	if challengeTimes < MIN_CHALLENGE_TIMES {
		return "", fmt.Errorf("challenge times must more than %d", MIN_CHALLENGE_TIMES)
	}
	if copyNum <= 0 {
		copyNum = 0
	}
	if encrypt && len(encryptPassword) == 0 {
		return "", fmt.Errorf("encrypt password is missing")
	}
	fileStat, err := os.Stat(fileName)
	if err != nil {
		return "", err
	}
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	if request == nil {
		return "", errors.New("init contract requester failed in SendFile")

	}

	// split file to blocks
	root, list, err := nodesFromFile(fileName, encrypt, encryptPassword)
	if err != nil {
		return "", err
	}
	fileHashStr := root.Cid().String()
	log.Debugf("root:%s, list.len:%d", fileHashStr, len(list))

	paramsBuf, privKey, err := c.payForSendFile(fileName, root, challengeRate, challengeTimes, uint64(copyNum), uint64(len(list)+1))
	if err != nil {
		return "", err
	}
	if len(paramsBuf) == 0 || len(privKey) == 0 {
		return "", fmt.Errorf("params.length is %d, prove private key length is %d", len(paramsBuf), len(privKey))
	}
	_, g0, _, fileID, r, pairing, err := request.GetFileProveParams(paramsBuf)
	if err != nil {
		return "", err
	}

	// get nodeList
	nodeList := c.fm.GetStoredBlockNodeList(fileHashStr, fileHashStr)
	log.Debugf("stored nodelist:%v", nodeList)
	if len(nodeList) == 0 {
		nodeList, err = request.GetNodeList(uint64(fileStat.Size()), copyNum)
		if err != nil {
			return "", err
		}
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
		return "", fmt.Errorf("no online nodes from nodeList")
	}
	log.Debugf("nodelist:%v, store to :%s", nodeList, server)

	for i, n := range nodeList {
		if n == server {
			nodeList = append(nodeList[:i], nodeList[i+1:]...)
			break
		}
	}
	log.Debugf("has paid file:%s, blocknum:%d rate:%d, times:%d, copynum:%d", root.Cid().String(), len(list), challengeRate, challengeTimes, copyNum)
	err = c.preSendFile(root, list, copyNum, nodeList)
	if err != nil {
		log.Errorf("presend file failed :%s", err)
		return "", err
	}
	// index of root tag start from 1
	log.Debugf("r:\n%s\npairing:\n%s", r, pairing)
	tag, err := PoR.SignGenerate(root.RawData(), fileID, r, 1, pairing, g0, privKey)
	log.Debugf("r:%s, pari:%s, privKey:%v, index:%s", r, pairing, privKey, 1)
	if err != nil {
		log.Errorf("generate root tag failed:%s", err)
		return "", err
	}

	// send root node
	if !c.fm.IsBlockStored(fileHashStr, root.Cid().String()) {
		ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), []blocks.Block{root}, []int32{0}, [][]byte{tag}, copyNum, nodeList)
		log.Infof("add root file to:%s ret:%v, err:%s", c.peer.ID(), ret, err)
		if err != nil {
			return "", err
		}
		err = c.fm.AddStoredBlock(fileHashStr, root.Cid().String(), server, nodeList)
		if err != nil {
			return "", err
		}
		if copyNum > 0 {
			log.Infof("send %s success, result:%v", root.Cid(), ret)
		} else {
			log.Infof("send %s success", root.Cid())
		}
	} else {
		log.Debugf("root node has already sent")
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
		if dagNode.Cid().String() == root.Cid().String() {
			continue
		}
		index++
		if c.fm.IsBlockStored(fileHashStr, dagNode.Cid().String()) {
			continue
		}
		// send others
		otherBlks = append(otherBlks, dagNode)
		otherIndxs = append(otherIndxs, index)
		tag, err := PoR.SignGenerate(dagNode.RawData(), fileID, r, uint32(index+1), pairing, g0, privKey)
		log.Debugf("r:%s, pari:%s, privKey:%v, index:%d", r, pairing, privKey, index+1)
		if err != nil {
			log.Errorf("generate %d tag failed:%s", index+1, err)
			return "", err
		}
		otherTags = append(otherTags, tag)
		log.Debugf("index:%v tag:%v, hash:%s", otherIndxs, tag, dagNode.Cid().String())

		if len(otherBlks) >= blockSizePerMsg || i == len(list)-1 {
			ret, err := c.node.Exchange.AddBlocks(context.Background(), c.peer.ID(), root.Cid().String(), otherBlks, otherIndxs, otherTags, copyNum, nodeList)
			if err != nil {
				return "", err
			}
			for _, sent := range otherBlks {
				err = c.fm.AddStoredBlock(fileHashStr, sent.Cid().String(), server, nodeList)
				if err != nil {
					return "", err
				}
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

	}
	log.Infof("File have stored: %s", root.Cid().String())
	return root.Cid().String(), nil
}

// payForSendFile pay before send a file
// return PoR params byte slice, PoR private key of the file or error
func (c *Client) payForSendFile(fileName string, root ipld.Node, challengeRate, challengeTimes, copyNum, blockNum uint64) ([]byte, []byte, error) {
	fileHashStr := root.Cid().String()
	request := NewContractRequest(c.wallet, c.walletPwd, c.rpc)
	fileInfo, _ := request.GetFileInfo(fileHashStr)

	var paramsBuf, privateKey []byte
	if fileInfo == nil {
		blockSize, err := root.Size()
		if err != nil || blockSize == 0 {
			return nil, nil, err
		}

		blockSizeInKB := uint64(math.Ceil(float64(blockSize) / 1024.0))
		log.Debugf("pay blocksize:%d, inkb:%d, blockNum:%d", blockSize, blockSizeInKB, blockNum)
		g, g0, pubKey, privKey, fileID, r, pairing := PoR.Init(fileName)
		paramsBuf, err = request.ProveParamSer(g, g0, pubKey, []byte(fileID), r, pairing)
		privateKey = privKey
		if err != nil {
			log.Errorf("serialzation prove params failed:%s", err)
			return nil, nil, err
		}
		rawTxId, err := request.PayStoreFile(fileHashStr, blockNum, blockSizeInKB, challengeRate, challengeTimes, copyNum, paramsBuf)
		if err != nil {
			log.Errorf("pay store file order failed:%s", err)
			return nil, nil, err
		}
		log.Infof("txId:%x", rawTxId)
		retry := 0
		for {
			if retry > MAX_RETRY_REQUEST_TIMES {
				return nil, nil, fmt.Errorf("retry request pay file tx failed")
			}
			payOK, _ := request.IsFilePaid(fileHashStr)
			if payOK {
				log.Debug("loop check paid success")
				break
			}
			retry++
			time.Sleep(time.Duration(MAX_REQUEST_TIMEWAIT) * time.Second)
		}
		err = c.fm.NewStoreFile(fileHashStr, privateKey)
		if err != nil {
			return nil, nil, err
		}
	} else {
		storedNodes, _ := request.GetStoreFileNodes(fileHashStr)
		if len(storedNodes) > 0 {
			return nil, nil, fmt.Errorf("file:%s has stored", fileHashStr)
		}
		log.Debugf("has paid but not store")
		err := c.fm.NewStoreFile(fileHashStr, []byte{})
		if err != nil {
			return nil, nil, err
		}
		if uint64(c.fm.StoredBlockCount(fileHashStr)) == blockNum {
			return nil, nil, fmt.Errorf("has sent all block, waiting for ipfs node commit proves")
		}
		paramsBuf = fileInfo.FileProveParam
		privateKey = c.fm.GetFileProvePrivKey(fileHashStr)
	}
	return paramsBuf, privateKey, nil
}

// PreSendFile send file information to node for checking the storage requirement
// This function does not send any block data.
func (c *Client) preSendFile(root ipld.Node, list []*helpers.UnixfsNode, copyNum int32, nodeList []string) error {
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

func (c *Client) decodeBlock(CID *cid.Cid, peerId peer.ID, nodeAddr string, nodeWalletAddr common.Address, fileHashStr string, request *ContractRequest, blockSize uint64) error {
	hasRead := c.fm.IsBlockRead(fileHashStr, nodeWalletAddr.ToBase58(), CID.String())
	log.Debugf("has read:%s %t", CID.String(), hasRead)
	childs := make([]string, 0)
	if !hasRead {
		// sliceId equal to blockCount
		nextSliceId := uint64(c.fm.GetReadNodeSliceId(fileHashStr, nodeWalletAddr.ToBase58()) + 1)
		settleSlice, err := request.GenFileReadSettleSlice(fileHashStr, nodeWalletAddr, nextSliceId)
		if err != nil {
			return err
		}
		// var buf bytes.Buffer
		blocks, err := c.node.Exchange.GetBlocks(context.Background(), peerId, fileHashStr, CID, settleSlice)
		if err != nil {
			return err
		}

		dagNode, err := ml.DecodeProtobufBlock(blocks[0])
		if err != nil {
			return err
		}

		file, err := os.OpenFile(fileHashStr, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			return err
		}
		defer file.Close()
		linksNum := len(dagNode.Links())

		for _, l := range dagNode.Links() {
			childs = append(childs, l.Cid.String())
		}
		c.fm.ReceivedBlockFromNode(fileHashStr, CID.String(), nodeAddr, nodeWalletAddr.ToBase58(), childs)
		if linksNum == 0 {
			pb := new(ftpb.Data)
			if err := proto.Unmarshal(dagNode.(*ml.ProtoNode).Data(), pb); err != nil {
				return err
			}
			n, err := file.Write(pb.Data)
			if err != nil || n == 0 {
				return err
			}
		}
	} else {
		childs = c.fm.GetBlockChilds(fileHashStr, nodeWalletAddr.ToBase58(), CID.String())
	}

	for i := 0; i < len(childs); i++ {
		childCid, err := cid.Decode(childs[i])
		if err != nil {
			return err
		}
		err = c.decodeBlock(childCid, peerId, nodeAddr, nodeWalletAddr, fileHashStr, request, blockSize)
		if err != nil {
			return err
		}
	}
	return nil
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
