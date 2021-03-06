package message

import (
	"fmt"
	"io"

	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"

	pb "github.com/daseinio/dasein-go-sdk/bitswap/message/pb"
	wantlist "github.com/daseinio/dasein-go-sdk/bitswap/wantlist"

	inet "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

const (
	NOOPER = 0
	CANCLE = 1
	DELETE = 2
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	// Wantlist returns a slice of unique keys that represent data wanted by
	// the sender.
	Wantlist() []Entry

	// Blocks returns a slice of unique blocks
	Blocks() []blocks.Block

	// AddEntry adds an entry to the Wantlist.
	AddEntry(key *cid.Cid, priority int)

	Read(key *cid.Cid, priority int, settleSlice []byte)

	Cancel(key *cid.Cid)

	Delete(key *cid.Cid)

	Empty() bool

	// A full wantlist is an authoritative copy, a 'non-full' wantlist is a patch-set
	Full() bool

	AddBlock(blocks.Block)
	Exportable

	Loggable() map[string]interface{}

	SetMessageType(string)

	MessageType() string

	SetBackup(int32, []string)

	Backup() *Backup

	AddLink(string)

	Links() []string

	SetResponse([]byte)

	Response() []byte

	SetFileHash(string)

	FileHash() string

	AddTagPack(*Tagpack)

	TagPacks() []*Tagpack
}

type Exportable interface {
	ToProtoV0() *pb.Message
	ToProtoV1() *pb.Message
	ToNetV0(w io.Writer) error
	ToNetV1(w io.Writer) error
}

type impl struct {
	full     bool
	wantlist map[string]*Entry
	blocks   map[string]blocks.Block
	msgType  string
	backup   *Backup
	links    []string
	resp     []byte
	fileHash string
	tagPacks []*Tagpack
}

func New(full bool) BitSwapMessage {
	return newMsg(full)
}

func newMsg(full bool) *impl {
	return &impl{
		blocks:   make(map[string]blocks.Block),
		wantlist: make(map[string]*Entry),
		full:     full,
		tagPacks: make([]*Tagpack, 0),
	}
}

type Entry struct {
	*wantlist.Entry
	Operation   int
	SettleSlice []byte
}

func newMessageFromProto(pbm pb.Message) (BitSwapMessage, error) {
	m := newMsg(pbm.GetWantlist().GetFull())
	for _, e := range pbm.GetWantlist().GetEntries() {
		c, err := cid.Cast([]byte(e.GetBlock()))
		if err != nil {
			return nil, fmt.Errorf("incorrectly formatted cid in wantlist: %s", err)
		}
		m.addEntry(c, int(e.GetPriority()), int(e.GetOperation()), e.GetSettleSlice())
	}

	// deprecated
	for _, d := range pbm.GetBlocks() {
		// CIDv0, sha256, protobuf only
		b := blocks.NewBlock(d)
		m.AddBlock(b)
	}
	//

	for _, b := range pbm.GetPayload() {
		pref, err := cid.PrefixFromBytes(b.GetPrefix())
		if err != nil {
			return nil, err
		}

		c, err := pref.Sum(b.GetData())
		if err != nil {
			return nil, err
		}

		blk, err := blocks.NewBlockWithCid(b.GetData(), c)
		if err != nil {
			return nil, err
		}

		m.AddBlock(blk)
	}
	m.SetMessageType(pbm.GetMessageType())
	if pbm.GetBackup().GetCopynum() > 0 {
		m.SetBackup(pbm.GetBackup().GetCopynum(), pbm.GetBackup().GetNodelist())
	}
	if len(pbm.GetLinks()) > 0 {
		m.links = pbm.GetLinks()
	}
	if len(pbm.GetResponse()) > 0 {
		m.resp = pbm.GetResponse()
	}
	m.SetFileHash(pbm.GetFileHash())
	for _, tag := range pbm.GetTagPacks() {
		tagPack := &Tagpack{
			Index: tag.GetIndex(),
			Hash:  tag.GetHash(),
			Tag:   tag.GetTag(),
		}
		m.AddTagPack(tagPack)
	}
	return m, nil
}

func (m *impl) Full() bool {
	return m.full
}

func (m *impl) Empty() bool {
	return len(m.blocks) == 0 && len(m.wantlist) == 0
}

func (m *impl) Wantlist() []Entry {
	out := make([]Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		out = append(out, *e)
	}
	return out
}

func (m *impl) Blocks() []blocks.Block {
	bs := make([]blocks.Block, 0, len(m.blocks))
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) Cancel(k *cid.Cid) {
	delete(m.wantlist, k.KeyString())
	m.addEntry(k, 0, CANCLE, nil)
}

func (m *impl) Delete(k *cid.Cid) {
	delete(m.wantlist, k.KeyString())
	m.addEntry(k, 0, DELETE, nil)
}

func (m *impl) Read(k *cid.Cid, priority int, settleSlice []byte) {
	m.addEntry(k, priority, NOOPER, settleSlice)
}

func (m *impl) AddEntry(k *cid.Cid, priority int) {
	m.addEntry(k, priority, NOOPER, nil)
}

func (m *impl) addEntry(c *cid.Cid, priority int, operation int, settleSlice []byte) {
	k := c.KeyString()
	e, exists := m.wantlist[k]
	if exists {
		e.Priority = priority
		e.Operation = operation
		if len(settleSlice) > 0 {
			e.SettleSlice = settleSlice
		}
	} else {
		newEntry := &Entry{
			Entry: &wantlist.Entry{
				Cid:      c,
				Priority: priority,
			},
			Operation: operation,
		}
		if len(settleSlice) > 0 {
			newEntry.SettleSlice = settleSlice
		}
		m.wantlist[k] = newEntry
	}
}

func (m *impl) AddBlock(b blocks.Block) {
	m.blocks[b.Cid().KeyString()] = b
}

func FromNet(r io.Reader) (BitSwapMessage, error) {
	pbr := ggio.NewDelimitedReader(r, inet.MessageSizeMax)
	return FromPBReader(pbr)
}

func FromPBReader(pbr ggio.Reader) (BitSwapMessage, error) {
	pb := new(pb.Message)
	if err := pbr.ReadMsg(pb); err != nil {
		return nil, err
	}

	return newMessageFromProto(*pb)
}

func (m *impl) ToProtoV0() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	pbm.Wantlist.Entries = make([]*pb.Message_Wantlist_Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:       proto.String(e.Cid.KeyString()),
			Priority:    proto.Int32(int32(e.Priority)),
			Operation:   proto.Int32(int32(e.Operation)),
			SettleSlice: e.SettleSlice,
		})
	}
	pbm.Wantlist.Full = proto.Bool(m.full)

	blocks := m.Blocks()
	pbm.Blocks = make([][]byte, 0, len(blocks))
	for _, b := range blocks {
		pbm.Blocks = append(pbm.Blocks, b.RawData())
	}
	pbm.MessageType = proto.String(m.msgType)
	if m.backup != nil {
		pbm.Backup = new(pb.Message_Backup)
		pbm.GetBackup().Copynum = proto.Int32(m.backup.Copynum)
		pbm.GetBackup().Nodelist = m.backup.Nodelist
	}
	if len(m.links) > 0 {
		pbm.Links = make([]string, 0)
		pbm.Links = append(pbm.Links, m.links...)
	}
	if len(m.resp) > 0 {
		pbm.Response = make([]byte, 0)
		pbm.Response = append(pbm.Response, m.resp...)
	}

	pbm.FileHash = proto.String(m.fileHash)
	if len(m.tagPacks) > 0 {
		pbm.TagPacks = make([]*pb.Message_Tagpack, 0)
		for _, tagPack := range m.tagPacks {
			tp := new(pb.Message_Tagpack)
			tp.Index = proto.Int32(tagPack.Index)
			tp.Hash = proto.String(tagPack.Hash)
			tp.Tag = tagPack.Tag
			pbm.TagPacks = append(pbm.TagPacks, tp)
		}
	}
	return pbm
}

func (m *impl) ToProtoV1() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	pbm.Wantlist.Entries = make([]*pb.Message_Wantlist_Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:       proto.String(e.Cid.KeyString()),
			Priority:    proto.Int32(int32(e.Priority)),
			Operation:   proto.Int32(int32(e.Operation)),
			SettleSlice: e.SettleSlice,
		})
	}
	pbm.Wantlist.Full = proto.Bool(m.full)

	blocks := m.Blocks()
	pbm.Payload = make([]*pb.Message_Block, 0, len(blocks))
	for _, b := range blocks {
		blk := &pb.Message_Block{
			Data:   b.RawData(),
			Prefix: b.Cid().Prefix().Bytes(),
		}
		pbm.Payload = append(pbm.Payload, blk)
	}

	pbm.MessageType = proto.String(m.msgType)
	if m.backup != nil {
		pbm.Backup = new(pb.Message_Backup)
		pbm.GetBackup().Copynum = proto.Int32(m.backup.Copynum)
		pbm.GetBackup().Nodelist = m.backup.Nodelist
	}
	if len(m.links) > 0 {
		pbm.Links = make([]string, 0)
		pbm.Links = append(pbm.Links, m.links...)
	}
	if len(m.resp) > 0 {
		pbm.Response = make([]byte, 0)
		pbm.Response = append(pbm.Response, m.resp...)
	}
	pbm.FileHash = proto.String(m.fileHash)
	if len(m.tagPacks) > 0 {
		pbm.TagPacks = make([]*pb.Message_Tagpack, 0)
		for _, tagPack := range m.tagPacks {
			tp := new(pb.Message_Tagpack)
			tp.Index = proto.Int32(tagPack.Index)
			tp.Hash = proto.String(tagPack.Hash)
			tp.Tag = tagPack.Tag
			pbm.TagPacks = append(pbm.TagPacks, tp)
		}
	}
	return pbm
}

func (m *impl) ToNetV0(w io.Writer) error {
	pbw := ggio.NewDelimitedWriter(w)

	return pbw.WriteMsg(m.ToProtoV0())
}

func (m *impl) ToNetV1(w io.Writer) error {
	pbw := ggio.NewDelimitedWriter(w)

	return pbw.WriteMsg(m.ToProtoV1())
}

func (m *impl) Loggable() map[string]interface{} {
	blocks := make([]string, 0, len(m.blocks))
	for _, v := range m.blocks {
		blocks = append(blocks, v.Cid().String())
	}
	return map[string]interface{}{
		"blocks": blocks,
		"wants":  m.Wantlist(),
	}
}

func (m *impl) SetMessageType(msgType string) {
	m.msgType = msgType
}

func (m *impl) MessageType() string {
	return m.msgType
}

type Backup struct {
	Copynum  int32
	Nodelist []string
}

func (m *impl) SetBackup(cn int32, nl []string) {
	m.backup = &Backup{
		Copynum:  cn,
		Nodelist: nl,
	}
}

func (m *impl) Backup() *Backup {
	return m.backup
}

func (m *impl) AddLink(l string) {
	if len(l) == 0 {
		return
	}
	if m.links == nil {
		m.links = make([]string, 0)
	}
	m.links = append(m.links, l)
}

func (m *impl) Links() []string {
	return m.links
}

func (m *impl) SetResponse(r []byte) {
	m.resp = r
}

func (m *impl) Response() []byte {
	return m.resp
}

func (m *impl) SetFileHash(fileHash string) {
	m.fileHash = fileHash
}

func (m *impl) FileHash() string {
	return m.fileHash
}

type Tagpack struct {
	Index int32
	Hash  string
	Tag   []byte
}

func (m *impl) AddTagPack(tagpack *Tagpack) {
	m.tagPacks = append(m.tagPacks, tagpack)
}

func (m *impl) TagPacks() []*Tagpack {
	return m.tagPacks
}
