// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	bsmsg "github.com/daseinio/dasein-go-sdk/bitswap/message"
	bsnet "github.com/daseinio/dasein-go-sdk/bitswap/network"
	ex "github.com/daseinio/dasein-go-sdk/exchange_interface"

	"gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	process "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

var log = logging.Logger("bitswap")

const (
	kMaxPriority = math.MaxInt32
)

var (
	outChanBufferSize      = 10 // receive msg channel size
	maxPreAddBlocksTimeout = 10 // recive pre addblocks timeout in second
	maxAddBlocksTimeout    = 10 // recive addblocks timeout in second * 1CopyNum * 1Block
	maxGetBlocksTimeout    = 10 // recive getblocks timeout in second
)

var rebroadcastDelay = delay.Fixed(time.Minute)

// Client <--> IPFS message type
const (
	MSG_TYPE_PREADDBLOCKS     = "preaddblocks"     // the client send a preaddblocks msg
	MSG_TYPE_PREADDBLOCKSRESP = "preaddblocksresp" // the client received a preaddblocks response msg
	MSG_TYPE_ADDBLOCKS        = "addblocks"        // the client send a addblocks msg
	MSG_TYPE_ADDBLOCKSRESP    = "addblocksresp"    // the client received a addblocks response msg
)

//CopyState used for set copy state of one copy task
type CopyState int

const (
	NoCopy      CopyState = iota //the node has not copy anything
	CopyFailed                   //the node has copy failed
	CopySuccess                  //the node has copy success
)

// AddBlocksResp used for set in bitswap message response
type AddBlocksResp struct {
	Result string               `json:"result"` // Result: success / fail
	Error  string               `json:"error"`  // Error message
	Copy   map[string]CopyState `json:"copy"`   // Copy state
}

// GetBlocksResp used for set in bitswap message response
type GetBlocksResp struct {
	Result string `json:"result"` // Result: success / fail
	Error  string `json:"error"`  // Error message
}

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, network bsnet.BitSwapNetwork) ex.Exchange {
	bs := &Bitswap{
		network: network,
		outChan: make(chan interface{}, outChanBufferSize),
	}
	network.SetDelegate(bs)
	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {
	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// newBlocks is a channel for newly added blocks to be provided to the
	// network.  blocks pushed down this channel get buffered and fed to the
	// provideKeys channel later on to avoid too much network activity
	newBlocks chan *cid.Cid
	// provideKeys directly feeds provide workers
	provideKeys chan *cid.Cid

	process process.Process

	outChan chan interface{}
}

// DelBlock deletes a given key from the wantlist
func (bs *Bitswap) DelBlock(ctx context.Context, id peer.ID, cid *cid.Cid) error {
	msg := bsmsg.New(true)
	msg.Delete(cid)
	return bs.network.SendMessage(context.TODO(), id, msg)
}

// DelBlocks deletes given keys from the wantlist
func (bs *Bitswap) DelBlocks(ctx context.Context, id peer.ID, cids []*cid.Cid) error {
	msg := bsmsg.New(true)
	for _, cid := range cids {
		msg.Delete(cid)
	}
	return bs.network.SendMessage(context.TODO(), id, msg)
}

func (bs *Bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	switch incoming.MessageType() {
	case MSG_TYPE_PREADDBLOCKSRESP:
		bs.handlePreAddblocksResp(ctx, p, incoming)
		return
	case MSG_TYPE_ADDBLOCKSRESP:
		bs.handleAddblocksResp(ctx, p, incoming)
		return
	default:
		bs.handleGetblockResp(ctx, p, incoming)
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {}

func (bs *Bitswap) ReceiveError(err error) {
	log.Infof("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) IsOnline() bool {
	return true
}

// PreAddBlocks send preaddblocks msg to check the node state
func (bs *Bitswap) PreAddBlocks(ctx context.Context, id peer.ID, fileHash string, cids []*cid.Cid, copyNum int32, nodeList []string) error {
	msg := bsmsg.New(true)
	for _, cid := range cids {
		msg.AddLink(cid.String())
	}
	msg.SetMessageType(MSG_TYPE_PREADDBLOCKS)
	msg.SetBackup(copyNum, nodeList)
	msg.SetFileHash(fileHash)
	err := bs.network.SendMessage(ctx, id, msg)
	if err != nil {
		log.Errorf("pre add blocks err:%s", err)
		return err
	}
	go func() {
		<-time.After(time.Duration(maxPreAddBlocksTimeout) * time.Second)
		bs.outChan <- &AddBlocksResp{
			Result: "fail",
			Error:  "wait for response timeout",
		}
	}()
	ret := <-bs.outChan
	switch ret.(type) {
	case *AddBlocksResp:
		if ret.(*AddBlocksResp).Result == "success" {
			return nil
		} else {
			return fmt.Errorf(ret.(*AddBlocksResp).Error)
		}
	default:
		return fmt.Errorf("convert outChan fail")
	}
}

func (bs *Bitswap) GetBlocks(ctx context.Context, id peer.ID, key *cid.Cid) ([]blocks.Block, error) {
	msg := bsmsg.New(true)
	msg.AddEntry(key, kMaxPriority)
	err := bs.network.SendMessage(ctx, id, msg)
	if err != nil {
		return nil, err
	}
	go func() {
		timeout := maxGetBlocksTimeout
		<-time.After(time.Duration(timeout) * time.Second)
		bs.outChan <- &GetBlocksResp{
			Result: "fail",
			Error:  "wait for response timeout",
		}
	}()

	ret := <-bs.outChan
	switch ret.(type) {
	case []blocks.Block:
		return ret.([]blocks.Block), nil
	case *GetBlocksResp:
		return nil, fmt.Errorf("wait for response timeout")
	default:
		return nil, fmt.Errorf("convert outChan fail")
	}
}

// AddBlock send a block bitswap msg to node with copyNum and nodeList
func (bs *Bitswap) AddBlocks(ctx context.Context, id peer.ID, fileHash string, blk []blocks.Block, copyNum int32, nodeList []string) (interface{}, error) {
	msg := bsmsg.New(true)
	msg.SetMessageType(MSG_TYPE_ADDBLOCKS)
	msg.SetFileHash(fileHash)
	for _, b := range blk {
		msg.AddBlock(b)
	}
	if copyNum > 0 {
		msg.SetBackup(copyNum, nodeList)
	}
	err := bs.network.SendMessage(ctx, id, msg)
	if err != nil {
		return nil, err
	}
	go func() {
		timeout := maxAddBlocksTimeout * len(blk)
		if copyNum > 0 {
			timeout *= int(copyNum)
		}
		<-time.After(time.Duration(timeout) * time.Second)
		bs.outChan <- &AddBlocksResp{
			Result: "fail",
			Error:  "wait for response timeout",
		}
	}()
	ret := <-bs.outChan
	switch ret.(type) {
	case *AddBlocksResp:
		if ret.(*AddBlocksResp).Result == "success" {
			return ret.(*AddBlocksResp).Copy, nil
		} else {
			return nil, fmt.Errorf(ret.(*AddBlocksResp).Error)
		}
	default:
		return nil, fmt.Errorf("convert outChan fail")
	}
}

// handleGetblocksResp handle the response msg of getblock
func (bs *Bitswap) handleGetblockResp(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	iblocks := incoming.Blocks()
	if len(iblocks) == 0 {
		return
	}
	bs.outChan <- iblocks
}

// handleAddblocksResp handle the response msg of addblocks
func (bs *Bitswap) handleAddblocksResp(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	var resp AddBlocksResp
	err := json.Unmarshal(incoming.Response(), &resp)
	if err != nil {
		log.Errorf("json unmarshal response failed:%s", err)
		return
	}
	bs.outChan <- &resp
}

// handlePreAddblocksResp handle the response msg of preaddblocks
func (bs *Bitswap) handlePreAddblocksResp(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	var resp AddBlocksResp
	err := json.Unmarshal(incoming.Response(), &resp)
	if err != nil {
		log.Errorf("json unmarshal response failed:%s", err)
		return
	}
	bs.outChan <- &resp
}
