// Code generated by protoc-gen-go. DO NOT EDIT.
// source: message.proto

package bitswap_message_pb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Message struct {
	Wantlist             *Message_Wantlist `protobuf:"bytes,1,opt,name=wantlist" json:"wantlist,omitempty"`
	Blocks               [][]byte          `protobuf:"bytes,2,rep,name=blocks" json:"blocks,omitempty"`
	Payload              []*Message_Block  `protobuf:"bytes,3,rep,name=payload" json:"payload,omitempty"`
	Backup               *Message_Backup   `protobuf:"bytes,4,opt,name=backup" json:"backup,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_2536372e43056908, []int{0}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (dst *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(dst, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetWantlist() *Message_Wantlist {
	if m != nil {
		return m.Wantlist
	}
	return nil
}

func (m *Message) GetBlocks() [][]byte {
	if m != nil {
		return m.Blocks
	}
	return nil
}

func (m *Message) GetPayload() []*Message_Block {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *Message) GetBackup() *Message_Backup {
	if m != nil {
		return m.Backup
	}
	return nil
}

type Message_Wantlist struct {
	Entries              []*Message_Wantlist_Entry `protobuf:"bytes,1,rep,name=entries" json:"entries,omitempty"`
	Full                 *bool                     `protobuf:"varint,2,opt,name=full" json:"full,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                  `json:"-"`
	XXX_unrecognized     []byte                    `json:"-"`
	XXX_sizecache        int32                     `json:"-"`
}

func (m *Message_Wantlist) Reset()         { *m = Message_Wantlist{} }
func (m *Message_Wantlist) String() string { return proto.CompactTextString(m) }
func (*Message_Wantlist) ProtoMessage()    {}
func (*Message_Wantlist) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_2536372e43056908, []int{0, 0}
}
func (m *Message_Wantlist) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message_Wantlist.Unmarshal(m, b)
}
func (m *Message_Wantlist) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message_Wantlist.Marshal(b, m, deterministic)
}
func (dst *Message_Wantlist) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message_Wantlist.Merge(dst, src)
}
func (m *Message_Wantlist) XXX_Size() int {
	return xxx_messageInfo_Message_Wantlist.Size(m)
}
func (m *Message_Wantlist) XXX_DiscardUnknown() {
	xxx_messageInfo_Message_Wantlist.DiscardUnknown(m)
}

var xxx_messageInfo_Message_Wantlist proto.InternalMessageInfo

func (m *Message_Wantlist) GetEntries() []*Message_Wantlist_Entry {
	if m != nil {
		return m.Entries
	}
	return nil
}

func (m *Message_Wantlist) GetFull() bool {
	if m != nil && m.Full != nil {
		return *m.Full
	}
	return false
}

type Message_Wantlist_Entry struct {
	Block                *string  `protobuf:"bytes,1,opt,name=block" json:"block,omitempty"`
	Priority             *int32   `protobuf:"varint,2,opt,name=priority" json:"priority,omitempty"`
	Operate              *int32   `protobuf:"varint,3,opt,name=operate" json:"operate,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message_Wantlist_Entry) Reset()         { *m = Message_Wantlist_Entry{} }
func (m *Message_Wantlist_Entry) String() string { return proto.CompactTextString(m) }
func (*Message_Wantlist_Entry) ProtoMessage()    {}
func (*Message_Wantlist_Entry) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_2536372e43056908, []int{0, 0, 0}
}
func (m *Message_Wantlist_Entry) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message_Wantlist_Entry.Unmarshal(m, b)
}
func (m *Message_Wantlist_Entry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message_Wantlist_Entry.Marshal(b, m, deterministic)
}
func (dst *Message_Wantlist_Entry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message_Wantlist_Entry.Merge(dst, src)
}
func (m *Message_Wantlist_Entry) XXX_Size() int {
	return xxx_messageInfo_Message_Wantlist_Entry.Size(m)
}
func (m *Message_Wantlist_Entry) XXX_DiscardUnknown() {
	xxx_messageInfo_Message_Wantlist_Entry.DiscardUnknown(m)
}

var xxx_messageInfo_Message_Wantlist_Entry proto.InternalMessageInfo

func (m *Message_Wantlist_Entry) GetBlock() string {
	if m != nil && m.Block != nil {
		return *m.Block
	}
	return ""
}

func (m *Message_Wantlist_Entry) GetPriority() int32 {
	if m != nil && m.Priority != nil {
		return *m.Priority
	}
	return 0
}

func (m *Message_Wantlist_Entry) GetOperate() int32 {
	if m != nil && m.Operate != nil {
		return *m.Operate
	}
	return 0
}

type Message_Block struct {
	Prefix               []byte   `protobuf:"bytes,1,opt,name=prefix" json:"prefix,omitempty"`
	Data                 []byte   `protobuf:"bytes,2,opt,name=data" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message_Block) Reset()         { *m = Message_Block{} }
func (m *Message_Block) String() string { return proto.CompactTextString(m) }
func (*Message_Block) ProtoMessage()    {}
func (*Message_Block) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_2536372e43056908, []int{0, 1}
}
func (m *Message_Block) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message_Block.Unmarshal(m, b)
}
func (m *Message_Block) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message_Block.Marshal(b, m, deterministic)
}
func (dst *Message_Block) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message_Block.Merge(dst, src)
}
func (m *Message_Block) XXX_Size() int {
	return xxx_messageInfo_Message_Block.Size(m)
}
func (m *Message_Block) XXX_DiscardUnknown() {
	xxx_messageInfo_Message_Block.DiscardUnknown(m)
}

var xxx_messageInfo_Message_Block proto.InternalMessageInfo

func (m *Message_Block) GetPrefix() []byte {
	if m != nil {
		return m.Prefix
	}
	return nil
}

func (m *Message_Block) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type Message_Backup struct {
	Copynum              *int32   `protobuf:"varint,1,opt,name=copynum" json:"copynum,omitempty"`
	Nodelist             []string `protobuf:"bytes,2,rep,name=nodelist" json:"nodelist,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message_Backup) Reset()         { *m = Message_Backup{} }
func (m *Message_Backup) String() string { return proto.CompactTextString(m) }
func (*Message_Backup) ProtoMessage()    {}
func (*Message_Backup) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_2536372e43056908, []int{0, 2}
}
func (m *Message_Backup) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message_Backup.Unmarshal(m, b)
}
func (m *Message_Backup) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message_Backup.Marshal(b, m, deterministic)
}
func (dst *Message_Backup) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message_Backup.Merge(dst, src)
}
func (m *Message_Backup) XXX_Size() int {
	return xxx_messageInfo_Message_Backup.Size(m)
}
func (m *Message_Backup) XXX_DiscardUnknown() {
	xxx_messageInfo_Message_Backup.DiscardUnknown(m)
}

var xxx_messageInfo_Message_Backup proto.InternalMessageInfo

func (m *Message_Backup) GetCopynum() int32 {
	if m != nil && m.Copynum != nil {
		return *m.Copynum
	}
	return 0
}

func (m *Message_Backup) GetNodelist() []string {
	if m != nil {
		return m.Nodelist
	}
	return nil
}

func init() {
	proto.RegisterType((*Message)(nil), "bitswap.message.pb.Message")
	proto.RegisterType((*Message_Wantlist)(nil), "bitswap.message.pb.Message.Wantlist")
	proto.RegisterType((*Message_Wantlist_Entry)(nil), "bitswap.message.pb.Message.Wantlist.Entry")
	proto.RegisterType((*Message_Block)(nil), "bitswap.message.pb.Message.Block")
	proto.RegisterType((*Message_Backup)(nil), "bitswap.message.pb.Message.Backup")
}

func init() { proto.RegisterFile("message.proto", fileDescriptor_message_2536372e43056908) }

var fileDescriptor_message_2536372e43056908 = []byte{
	// 314 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x91, 0xbf, 0x4e, 0xc3, 0x30,
	0x10, 0xc6, 0x95, 0xa6, 0x69, 0xd2, 0x6b, 0x59, 0x4e, 0x08, 0x59, 0x99, 0x42, 0xc5, 0x10, 0x31,
	0x64, 0x28, 0x1b, 0x48, 0x08, 0x21, 0x18, 0x59, 0x8e, 0x81, 0xd9, 0x69, 0x5d, 0x14, 0x35, 0x8d,
	0x2d, 0xc7, 0x55, 0xc9, 0x43, 0xf0, 0x40, 0xbc, 0x1d, 0xca, 0xe5, 0xcf, 0x82, 0x54, 0xb1, 0xf9,
	0x97, 0xdc, 0xef, 0xf4, 0xf9, 0x33, 0x5c, 0x1c, 0x54, 0x5d, 0xcb, 0x4f, 0x95, 0x19, 0xab, 0x9d,
	0x46, 0xcc, 0x0b, 0x57, 0x9f, 0xa4, 0xc9, 0xc6, 0xcf, 0xf9, 0xea, 0x7b, 0x0a, 0xe1, 0x5b, 0x87,
	0xf8, 0x04, 0xd1, 0x49, 0x56, 0xae, 0x2c, 0x6a, 0x27, 0xbc, 0xc4, 0x4b, 0x17, 0xeb, 0x9b, 0xec,
	0xaf, 0x92, 0xf5, 0xe3, 0xd9, 0x47, 0x3f, 0x4b, 0xa3, 0x85, 0x57, 0x30, 0xcb, 0x4b, 0xbd, 0xd9,
	0xd7, 0x62, 0x92, 0xf8, 0xe9, 0x92, 0x7a, 0xc2, 0x07, 0x08, 0x8d, 0x6c, 0x4a, 0x2d, 0xb7, 0xc2,
	0x4f, 0xfc, 0x74, 0xb1, 0xbe, 0x3e, 0xb7, 0xf8, 0xb9, 0x95, 0x68, 0x30, 0xf0, 0x1e, 0x66, 0xb9,
	0xdc, 0xec, 0x8f, 0x46, 0x4c, 0x39, 0xd4, 0xea, 0xac, 0xcb, 0x93, 0xd4, 0x1b, 0xf1, 0x8f, 0x07,
	0xd1, 0x90, 0x13, 0x5f, 0x20, 0x54, 0x95, 0xb3, 0x85, 0xaa, 0x85, 0xc7, 0x29, 0x6e, 0xff, 0x73,
	0xbd, 0xec, 0xb5, 0x72, 0xb6, 0xa1, 0x41, 0x45, 0x84, 0xe9, 0xee, 0x58, 0x96, 0x62, 0x92, 0x78,
	0x69, 0x44, 0x7c, 0x8e, 0xdf, 0x21, 0xe0, 0x29, 0xbc, 0x84, 0x80, 0xaf, 0xcc, 0xfd, 0xcd, 0xa9,
	0x03, 0x8c, 0x21, 0x32, 0xb6, 0xd0, 0xb6, 0x70, 0x0d, 0x6b, 0x01, 0x8d, 0x8c, 0x02, 0x42, 0x6d,
	0x94, 0x95, 0x4e, 0x09, 0x9f, 0x7f, 0x0d, 0x18, 0xdf, 0x41, 0xc0, 0x4d, 0xb4, 0xad, 0x1a, 0xab,
	0x76, 0xc5, 0x17, 0x6f, 0x5d, 0x52, 0x4f, 0x6d, 0x92, 0xad, 0x74, 0x92, 0x57, 0x2e, 0x89, 0xcf,
	0xf1, 0x23, 0xcc, 0xba, 0x0a, 0xda, 0xc5, 0x1b, 0x6d, 0x9a, 0xea, 0x78, 0x60, 0x2d, 0xa0, 0x01,
	0xdb, 0x38, 0x95, 0xde, 0x2a, 0x7e, 0xe7, 0xf6, 0x9d, 0xe6, 0x34, 0xf2, 0x6f, 0x00, 0x00, 0x00,
	0xff, 0xff, 0xc2, 0x1d, 0x0f, 0x36, 0x33, 0x02, 0x00, 0x00,
}
