// Code generated by protoc-gen-go.
// source: github.com/google/trillian/storage/proto/storage.proto
// DO NOT EDIT!

/*
Package storagepb is a generated protocol buffer package.

It is generated from these files:
	github.com/google/trillian/storage/proto/storage.proto

It has these top-level messages:
	NodeIDProto
	SubtreeProto
*/
package storagepb

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

// NodeIDProto is the serialized form of NodeID. It's used only for persistence in storage.
// As this is long-term we prefer not to use a Go specific format.
type NodeIDProto struct {
	Path          []byte `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	PrefixLenBits int32  `protobuf:"varint,2,opt,name=prefix_len_bits,json=prefixLenBits" json:"prefix_len_bits,omitempty"`
}

func (m *NodeIDProto) Reset()                    { *m = NodeIDProto{} }
func (m *NodeIDProto) String() string            { return proto.CompactTextString(m) }
func (*NodeIDProto) ProtoMessage()               {}
func (*NodeIDProto) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

// SubtreeProto contains nodes of a subtree.
type SubtreeProto struct {
	// subtree's prefix (must be a multiple of 8 bits)
	Prefix []byte `protobuf:"bytes,1,opt,name=prefix,proto3" json:"prefix,omitempty"`
	// subtree's depth
	Depth    int32  `protobuf:"varint,2,opt,name=depth" json:"depth,omitempty"`
	RootHash []byte `protobuf:"bytes,3,opt,name=root_hash,json=rootHash,proto3" json:"root_hash,omitempty"`
	// map of suffix (within subtree) to subtree-leaf node hash
	Leaves map[string][]byte `protobuf:"bytes,4,rep,name=leaves" json:"leaves,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Map of suffix (within subtree) to subtree-internal node hash.
	// This structure is only used in RAM as a cache, the internal nodes of
	// the subtree are not generally stored.
	InternalNodes map[string][]byte `protobuf:"bytes,5,rep,name=internal_nodes,json=internalNodes" json:"internal_nodes,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (m *SubtreeProto) Reset()                    { *m = SubtreeProto{} }
func (m *SubtreeProto) String() string            { return proto.CompactTextString(m) }
func (*SubtreeProto) ProtoMessage()               {}
func (*SubtreeProto) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *SubtreeProto) GetLeaves() map[string][]byte {
	if m != nil {
		return m.Leaves
	}
	return nil
}

func (m *SubtreeProto) GetInternalNodes() map[string][]byte {
	if m != nil {
		return m.InternalNodes
	}
	return nil
}

func init() {
	proto.RegisterType((*NodeIDProto)(nil), "trillian.storage.proto.NodeIDProto")
	proto.RegisterType((*SubtreeProto)(nil), "trillian.storage.proto.SubtreeProto")
}

func init() {
	proto.RegisterFile("github.com/google/trillian/storage/proto/storage.proto", fileDescriptor0)
}

var fileDescriptor0 = []byte{
	// 325 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x94, 0x92, 0xcf, 0x4b, 0xfb, 0x40,
	0x10, 0xc5, 0x49, 0xd3, 0x96, 0x6f, 0xa7, 0xed, 0x57, 0x59, 0xa4, 0x84, 0x7a, 0x29, 0x3d, 0x48,
	0x4f, 0x89, 0x54, 0xf1, 0xe7, 0x45, 0x8a, 0x42, 0x0b, 0x45, 0x24, 0xde, 0x3c, 0x18, 0x36, 0x76,
	0x4c, 0x16, 0xd7, 0xdd, 0xb0, 0xbb, 0x2d, 0xf6, 0xe6, 0x9f, 0x2e, 0xd9, 0x6c, 0xa1, 0xa2, 0x87,
	0x7a, 0x9b, 0xf7, 0x98, 0xf7, 0x49, 0xde, 0xb0, 0x70, 0x96, 0x31, 0x93, 0x2f, 0xd3, 0xf0, 0x45,
	0xbe, 0x47, 0x99, 0x94, 0x19, 0xc7, 0xc8, 0x28, 0xc6, 0x39, 0xa3, 0x22, 0xd2, 0x46, 0x2a, 0x9a,
	0x61, 0x54, 0x28, 0x69, 0xe4, 0x46, 0x85, 0x56, 0x91, 0xde, 0x66, 0x2b, 0xfc, 0xe6, 0x0f, 0x67,
	0xd0, 0xbe, 0x97, 0x0b, 0x9c, 0xdd, 0x3e, 0xd8, 0x35, 0x02, 0xf5, 0x82, 0x9a, 0x3c, 0xf0, 0x06,
	0xde, 0xa8, 0x13, 0xdb, 0x99, 0x1c, 0xc1, 0x5e, 0xa1, 0xf0, 0x95, 0x7d, 0x24, 0x1c, 0x45, 0x92,
	0x32, 0xa3, 0x83, 0xda, 0xc0, 0x1b, 0x35, 0xe2, 0x6e, 0x65, 0xcf, 0x51, 0x4c, 0x98, 0xd1, 0xc3,
	0x4f, 0x1f, 0x3a, 0x8f, 0xcb, 0xd4, 0x28, 0xc4, 0x0a, 0xd6, 0x83, 0x66, 0xb5, 0xe1, 0x70, 0x4e,
	0x91, 0x03, 0x68, 0x2c, 0xb0, 0x30, 0xb9, 0xc3, 0x54, 0x82, 0x1c, 0x42, 0x4b, 0x49, 0x69, 0x92,
	0x9c, 0xea, 0x3c, 0xf0, 0x6d, 0xe0, 0x5f, 0x69, 0x4c, 0xa9, 0xce, 0xc9, 0x14, 0x9a, 0x1c, 0xe9,
	0x0a, 0x75, 0x50, 0x1f, 0xf8, 0xa3, 0xf6, 0xf8, 0x38, 0xfc, 0xbd, 0x4f, 0xb8, 0xfd, 0x03, 0xe1,
	0xdc, 0x46, 0xee, 0x84, 0x51, 0xeb, 0xd8, 0xe5, 0xc9, 0x33, 0xfc, 0x67, 0xc2, 0xa0, 0x12, 0x94,
	0x27, 0x42, 0x2e, 0x50, 0x07, 0x0d, 0x4b, 0x3c, 0xdf, 0x89, 0x38, 0x73, 0xd1, 0xf2, 0x66, 0x0e,
	0xdc, 0x65, 0xdb, 0x5e, 0xff, 0x12, 0xda, 0x5b, 0x9f, 0x25, 0xfb, 0xe0, 0xbf, 0xe1, 0xda, 0x1e,
	0xa0, 0x15, 0x97, 0x63, 0xd9, 0x7e, 0x45, 0xf9, 0x12, 0x6d, 0xfb, 0x4e, 0x5c, 0x89, 0xab, 0xda,
	0x85, 0xd7, 0xbf, 0x01, 0xf2, 0x93, 0xff, 0x17, 0xc2, 0xe4, 0xf4, 0x69, 0xbc, 0xeb, 0xfb, 0xb8,
	0x76, 0xaa, 0x48, 0xd3, 0xa6, 0x35, 0x4e, 0xbe, 0x02, 0x00, 0x00, 0xff, 0xff, 0x3a, 0x4a, 0x10,
	0x9e, 0x5c, 0x02, 0x00, 0x00,
}
