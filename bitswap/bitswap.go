// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/daseinio/dasein-go-sdk/bitswap/decision"
	bsmsg "github.com/daseinio/dasein-go-sdk/bitswap/message"
	bsnet "github.com/daseinio/dasein-go-sdk/bitswap/network"
	"github.com/daseinio/dasein-go-sdk/bitswap/notifications"
	ex "github.com/daseinio/dasein-go-sdk/exchange_interface"

	"gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	"gx/ipfs/QmRMGdC6HKdLsPDABL9aXPDidrpmEHzJqFWSvshkbn9Hj8/go-ipfs-flags"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	"gx/ipfs/QmRg1gKTHzc3CZXSKzem8aR4E3TubFhbgXwfVuWnSK5CC5/go-metrics-interface"
	process "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

var log = logging.Logger("bitswap")

const (
	// maxProvidersPerRequest specifies the maximum number of providers desired
	// from the network. This value is specified because the network streams
	// results.
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	providerRequestTimeout = time.Second * 10
	provideTimeout         = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	HasBlockBufferSize    = 256
	provideKeysBufferSize = 2048
	provideWorkerMax      = 512

	// the 1<<18+15 is to observe old file chunks that are 1<<18 + 14 in size
	metricsBuckets = []float64{1 << 6, 1 << 10, 1 << 14, 1 << 18, 1<<18 + 15, 1 << 22}

	outChanBufferSize      = 10 // receive msg channel size
	maxPreAddBlocksTimeout = 10 // recive pre addblocks timeout in second
	maxAddBlocksTimeout    = 10 // recive addblocks timeout in second * 1CopyNum * 1Block
	maxGetBlocksTimeout    = 10 // recive getblocks timeout in second
)

func init() {
	if flags.LowMemMode {
		HasBlockBufferSize = 64
		provideKeysBufferSize = 512
		provideWorkerMax = 16
	}
}

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
	Result string               `json:"result"` // Result: success / fail
	Error  string               `json:"error"`  // Error message
}


// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, network bsnet.BitSwapNetwork) ex.Exchange {

	// important to use provided parent context (since it may include important
	// loggable data). It's probably not a good idea to allow bitswap to be
	// coupled to the concerns of the ipfs daemon in this way.
	//
	// FIXME(btc) Now that bitswap manages itself using a process, it probably
	// shouldn't accept a context anymore. Clients should probably use Close()
	// exclusively. We should probably find another way to share logging data
	ctx, cancelFunc := context.WithCancel(parent)
	ctx = metrics.CtxSubScope(ctx, "bitswap")
	dupHist := metrics.NewCtx(ctx, "recv_dup_blocks_bytes", "Summary of duplicate"+
		" data blocks recived").Histogram(metricsBuckets)
	allHist := metrics.NewCtx(ctx, "recv_all_blocks_bytes", "Summary of all"+
		" data blocks recived").Histogram(metricsBuckets)

	notif := notifications.New()
	px := process.WithTeardown(func() error {
		notif.Shutdown()
		return nil
	})

	bs := &Bitswap{
		notifications: notif,
		engine:        decision.NewEngine(ctx), // TODO close the engine with Close() method
		network:       network,
		findKeys:      make(chan *blockRequest, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan *cid.Cid, HasBlockBufferSize),
		provideKeys:   make(chan *cid.Cid, provideKeysBufferSize),
		wm:            NewWantManager(ctx, network),
		counters:      new(counters),

		dupMetric: dupHist,
		allMetric: allHist,
		outChan:   make(chan interface{}, outChanBufferSize),
	}
	go bs.wm.Run()
	network.SetDelegate(bs)

	// Start up bitswaps async worker routines
	bs.startWorkers(px, ctx)

	// bind the context and process.
	// do it over here to avoid closing before all setup is done.
	go func() {
		<-px.Closing() // process closes first
		cancelFunc()
	}()
	procctx.CloseAfterContext(px, ctx) // parent cancelled first

	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {
	// the peermanager manages sending messages to peers in a way that
	// wont block bitswap operation
	wm *WantManager

	// the engine is the bit of logic that decides who to send which blocks to
	engine *decision.Engine

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// notifications engine for receiving new blocks and routing them to the
	// appropriate user requests
	notifications notifications.PubSub

	// findKeys sends keys to a worker to find and connect to providers for them
	findKeys chan *blockRequest
	// newBlocks is a channel for newly added blocks to be provided to the
	// network.  blocks pushed down this channel get buffered and fed to the
	// provideKeys channel later on to avoid too much network activity
	newBlocks chan *cid.Cid
	// provideKeys directly feeds provide workers
	provideKeys chan *cid.Cid

	process process.Process

	// Counters for various statistics
	counterLk sync.Mutex
	counters  *counters

	// Metrics interface metrics
	dupMetric metrics.Histogram
	allMetric metrics.Histogram

	// Sessions
	sessions []*Session
	sessLk   sync.Mutex

	sessID   uint64
	sessIDLk sync.Mutex

	outChan chan interface{}
}

type counters struct {
	blocksRecvd    uint64
	dupBlocksRecvd uint64
	dupDataRecvd   uint64
	blocksSent     uint64
	dataSent       uint64
	dataRecvd      uint64
	messagesRecvd  uint64
}

type blockRequest struct {
	Cid *cid.Cid
	Ctx context.Context
}


func (bs *Bitswap) getNextSessionID() uint64 {
	bs.sessIDLk.Lock()
	defer bs.sessIDLk.Unlock()
	bs.sessID++
	return bs.sessID
}

// CancelWant removes a given key from the wantlist
func (bs *Bitswap) CancelWants(cids []*cid.Cid, ses uint64) {
	if len(cids) == 0 {
		return
	}
	bs.wm.CancelWants(context.Background(), cids, nil, ses)
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
func (bs *Bitswap) PeerConnected(p peer.ID) {
	bs.wm.Connected(p)
	bs.engine.PeerConnected(p)
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {
	bs.wm.Disconnected(p)
	bs.engine.PeerDisconnected(p)
}

func (bs *Bitswap) ReceiveError(err error) {
	log.Infof("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) GetWantlist() []*cid.Cid {
	entries := bs.wm.wl.Entries()
	out := make([]*cid.Cid, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) IsOnline() bool {
	return true
}

// PreAddBlocks send preaddblocks msg to check the node state
func (bs *Bitswap) PreAddBlocks(ctx context.Context, id peer.ID, cids []*cid.Cid, copyNum int32, nodeList []string) error {
	msg := bsmsg.New(true)
	for _, cid := range cids {
		msg.AddLink(cid.String())
	}
	msg.SetMessageType(MSG_TYPE_PREADDBLOCKS)
	msg.SetBackup(copyNum, nodeList)
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
func (bs *Bitswap) AddBlocks(ctx context.Context, id peer.ID, blk []blocks.Block, copyNum int32, nodeList []string) (interface{}, error) {
	msg := bsmsg.New(true)
	msg.SetMessageType(MSG_TYPE_ADDBLOCKS)
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
