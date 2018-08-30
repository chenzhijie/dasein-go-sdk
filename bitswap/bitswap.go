// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
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

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k *cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, bs.GetBlocks)
}

func (bs *Bitswap) WantlistForPeer(p peer.ID) []*cid.Cid {
	var out []*cid.Cid
	for _, e := range bs.engine.WantlistForPeer(p) {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) LedgerForPeer(p peer.ID) *decision.Receipt {
	return bs.engine.LedgerForPeer(p)
}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []*cid.Cid) (<-chan blocks.Block, error) {
	if len(keys) == 0 {
		out := make(chan blocks.Block)
		close(out)
		return out, nil
	}

	select {
	case <-bs.process.Closing():
		return nil, errors.New("bitswap is closed")
	default:
	}
	promise := bs.notifications.Subscribe(ctx, keys...)

	for _, k := range keys {
		fmt.Println("Bitswap.GetBlockRequest.Start", k)
		log.Event(ctx, "Bitswap.GetBlockRequest.Start", k)
	}

	mses := bs.getNextSessionID()

	bs.wm.WantBlocks(ctx, keys, nil, mses)

	// NB: Optimization. Assumes that providers of key[0] are likely to
	// be able to provide for all keys. This currently holds true in most
	// every situation. Later, this assumption may not hold as true.
	req := &blockRequest{
		Cid: keys[0],
		Ctx: ctx,
	}

	remaining := cid.NewSet()
	for _, k := range keys {
		remaining.Add(k)
	}

	out := make(chan blocks.Block)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer close(out)
		defer func() {
			// can't just defer this call on its own, arguments are resolved *when* the defer is created
			bs.CancelWants(remaining.Keys(), mses)
		}()
		for {
			select {
			case blk, ok := <-promise:
				if !ok {
					return
				}

				bs.CancelWants([]*cid.Cid{blk.Cid()}, mses)
				remaining.Remove(blk.Cid())
				select {
				case out <- blk:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case bs.findKeys <- req:
		return out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
func (bs *Bitswap) DelBlock(ctx context.Context, cid *cid.Cid) error {
	msg := bsmsg.New(true)
	msg.Delete(cid)

	id, err := peer.IDB58Decode("QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		return err
	}

	return bs.network.SendMessage(context.TODO(), id, msg)
}

// DelBlocks deletes given keys from the wantlist
func (bs *Bitswap) DelBlocks(ctx context.Context, cids []*cid.Cid) error {
	msg := bsmsg.New(true)
	for _, cid := range cids {
		msg.Delete(cid)
	}

	id, err := peer.IDB58Decode("QmR1AqNQBqAjPeLswq86dkJZ5Y7ACVGoXzz2K8tz6MHyUB")
	if err != nil {
		return err
	}

	return bs.network.SendMessage(context.TODO(), id, msg)
}

// HasBlock announces the existence of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *Bitswap) HasBlock(blk blocks.Block) error {
	return bs.receiveBlockFrom(blk, "")
}

// TODO: Some of this stuff really only needs to be done when adding a block
// from the user, not when receiving it from the network.
// In case you run `git blame` on this comment, I'll save you some time: ask
// @whyrusleeping, I don't know the answers you seek.
func (bs *Bitswap) receiveBlockFrom(blk blocks.Block, from peer.ID) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}
	fmt.Println("Block Cid: ", blk.Cid().String())

	// NOTE: There exists the possiblity for a race condition here.  If a user
	// creates a node, then adds it to the dagservice while another goroutine
	// is waiting on a GetBlock for that object, they will receive a reference
	// to the same node. We should address this soon, but i'm not going to do
	// it now as it requires more thought and isnt causing immediate problems.
	bs.notifications.Publish(blk)

	k := blk.Cid()
	ks := []*cid.Cid{k}
	for _, s := range bs.SessionsForBlock(k) {
		s.receiveBlockFrom(from, blk)
		bs.CancelWants(ks, s.id)
	}

	bs.engine.AddBlock(blk)

	select {
	case bs.newBlocks <- blk.Cid():
		// send block off to be reprovided
	case <-bs.process.Closing():
		return bs.process.Close()
	}
	return nil
}

// SessionsForBlock returns a slice of all sessions that may be interested in the given cid
func (bs *Bitswap) SessionsForBlock(c *cid.Cid) []*Session {
	bs.sessLk.Lock()
	defer bs.sessLk.Unlock()

	var out []*Session
	for _, s := range bs.sessions {
		if s.interestedIn(c) {
			out = append(out, s)
		}
	}
	return out
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
	}

	atomic.AddUint64(&bs.counters.messagesRecvd, 1)

	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	//bs.engine.MessageReceived(p, incoming)
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger

	iblocks := incoming.Blocks()

	if len(iblocks) == 0 {
		return
	}

	wg := sync.WaitGroup{}
	for _, block := range iblocks {
		wg.Add(1)
		go func(b blocks.Block) { // TODO: this probably doesnt need to be a goroutine...
			defer wg.Done()

			bs.updateReceiveCounters(b)

			log.Debugf("got block %s from %s", b, p)
			if err := bs.receiveBlockFrom(b, p); err != nil {
				log.Warningf("ReceiveMessage recvBlockFrom error: %s", err)
			}
			log.Event(ctx, "Bitswap.GetBlockRequest.End", b.Cid())
		}(block)
	}
	wg.Wait()
}

var ErrAlreadyHaveBlock = errors.New("already have block")

func (bs *Bitswap) updateReceiveCounters(b blocks.Block) {
	blkLen := len(b.RawData())

	bs.allMetric.Observe(float64(blkLen))

	bs.counterLk.Lock()
	defer bs.counterLk.Unlock()
	c := bs.counters

	c.blocksRecvd++
	c.dataRecvd += uint64(len(b.RawData()))
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
func (bs *Bitswap) PreAddBlocks(ctx context.Context, to string, cids []*cid.Cid, copyNum int32, nodeList []string) error {
	id, err := peer.IDB58Decode(to)
	if err != nil {
		return err
	}
	msg := bsmsg.New(true)
	for _, cid := range cids {
		msg.AddLink(cid.String())
	}
	msg.SetMessageType(MSG_TYPE_PREADDBLOCKS)
	msg.SetBackup(copyNum, nodeList)
	err = bs.network.SendMessage(ctx, id, msg)
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

// AddBlock send a block bitswap msg to node with copyNum and nodeList
func (bs *Bitswap) AddBlocks(ctx context.Context, to string, blk []blocks.Block, copyNum int32, nodeList []string) (interface{}, error) {
	id, err := peer.IDB58Decode(to)
	if err != nil {
		return nil, err
	}
	msg := bsmsg.New(true)
	msg.SetMessageType(MSG_TYPE_ADDBLOCKS)
	for _, b := range blk {
		msg.AddBlock(b)
	}
	if copyNum > 0 {
		msg.SetBackup(copyNum, nodeList)
	}
	err = bs.network.SendMessage(ctx, id, msg)
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
