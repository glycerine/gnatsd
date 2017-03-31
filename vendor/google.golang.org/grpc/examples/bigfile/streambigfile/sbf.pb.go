// Code generated by protoc-gen-gogo.
// source: sbf.proto
// DO NOT EDIT!

/*
	Package streambigfile is a generated protocol buffer package.

	It is generated from these files:
		sbf.proto

	It has these top-level messages:
		BigFileChunk
		BigFileAck
*/
package streambigfile

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type BigFileChunk struct {
	Filepath    string `protobuf:"bytes,1,opt,name=Filepath,json=filepath,proto3" json:"Filepath,omitempty"`
	SizeInBytes int64  `protobuf:"varint,2,opt,name=SizeInBytes,json=sizeInBytes,proto3" json:"SizeInBytes,omitempty"`
	SendTime    uint64 `protobuf:"fixed64,3,opt,name=SendTime,json=sendTime,proto3" json:"SendTime,omitempty"`
	Blake2B     []byte `protobuf:"bytes,4,opt,name=Blake2b,json=blake2b,proto3" json:"Blake2b,omitempty"`
	Data        []byte `protobuf:"bytes,5,opt,name=Data,json=data,proto3" json:"Data,omitempty"`
	// It is not strictly
	// necessary to chop the
	// Data into chunks, but
	// may be convenient to
	// avoid excessive memory
	// use.
	//
	// Fields Data and Blake2b represent
	// just this chunk.
	//
	// ChunkCountMinus1 is the
	// count of chunks, minus 1.
	//
	// For example, a value of 0
	// in the ChunkCountMinus1
	// field means that there is
	// only one chunk.
	//
	// ChunkNumber will be 0 as
	// well in this case.
	//
	ChunkCountMinus1 int64 `protobuf:"varint,6,opt,name=ChunkCountMinus1,json=chunkCountMinus1,proto3" json:"ChunkCountMinus1,omitempty"`
	ChunkNumber      int64 `protobuf:"varint,7,opt,name=ChunkNumber,json=chunkNumber,proto3" json:"ChunkNumber,omitempty"`
}

func (m *BigFileChunk) Reset()                    { *m = BigFileChunk{} }
func (m *BigFileChunk) String() string            { return proto.CompactTextString(m) }
func (*BigFileChunk) ProtoMessage()               {}
func (*BigFileChunk) Descriptor() ([]byte, []int) { return fileDescriptorSbf, []int{0} }

func (m *BigFileChunk) GetFilepath() string {
	if m != nil {
		return m.Filepath
	}
	return ""
}

func (m *BigFileChunk) GetSizeInBytes() int64 {
	if m != nil {
		return m.SizeInBytes
	}
	return 0
}

func (m *BigFileChunk) GetSendTime() uint64 {
	if m != nil {
		return m.SendTime
	}
	return 0
}

func (m *BigFileChunk) GetBlake2B() []byte {
	if m != nil {
		return m.Blake2B
	}
	return nil
}

func (m *BigFileChunk) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *BigFileChunk) GetChunkCountMinus1() int64 {
	if m != nil {
		return m.ChunkCountMinus1
	}
	return 0
}

func (m *BigFileChunk) GetChunkNumber() int64 {
	if m != nil {
		return m.ChunkNumber
	}
	return 0
}

type BigFileAck struct {
	Filepath         string `protobuf:"bytes,1,opt,name=Filepath,json=filepath,proto3" json:"Filepath,omitempty"`
	SizeInBytes      int64  `protobuf:"varint,2,opt,name=SizeInBytes,json=sizeInBytes,proto3" json:"SizeInBytes,omitempty"`
	RecvTime         uint64 `protobuf:"fixed64,3,opt,name=RecvTime,json=recvTime,proto3" json:"RecvTime,omitempty"`
	WholeFileBlake2B []byte `protobuf:"bytes,4,opt,name=WholeFileBlake2b,json=wholeFileBlake2b,proto3" json:"WholeFileBlake2b,omitempty"`
}

func (m *BigFileAck) Reset()                    { *m = BigFileAck{} }
func (m *BigFileAck) String() string            { return proto.CompactTextString(m) }
func (*BigFileAck) ProtoMessage()               {}
func (*BigFileAck) Descriptor() ([]byte, []int) { return fileDescriptorSbf, []int{1} }

func (m *BigFileAck) GetFilepath() string {
	if m != nil {
		return m.Filepath
	}
	return ""
}

func (m *BigFileAck) GetSizeInBytes() int64 {
	if m != nil {
		return m.SizeInBytes
	}
	return 0
}

func (m *BigFileAck) GetRecvTime() uint64 {
	if m != nil {
		return m.RecvTime
	}
	return 0
}

func (m *BigFileAck) GetWholeFileBlake2B() []byte {
	if m != nil {
		return m.WholeFileBlake2B
	}
	return nil
}

func init() {
	proto.RegisterType((*BigFileChunk)(nil), "streambigfile.BigFileChunk")
	proto.RegisterType((*BigFileAck)(nil), "streambigfile.BigFileAck")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Peer service

type PeerClient interface {
	SendFile(ctx context.Context, opts ...grpc.CallOption) (Peer_SendFileClient, error)
}

type peerClient struct {
	cc *grpc.ClientConn
}

func NewPeerClient(cc *grpc.ClientConn) PeerClient {
	return &peerClient{cc}
}

func (c *peerClient) SendFile(ctx context.Context, opts ...grpc.CallOption) (Peer_SendFileClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Peer_serviceDesc.Streams[0], c.cc, "/streambigfile.Peer/SendFile", opts...)
	if err != nil {
		return nil, err
	}
	x := &peerSendFileClient{stream}
	return x, nil
}

type Peer_SendFileClient interface {
	Send(*BigFileChunk) error
	CloseAndRecv() (*BigFileAck, error)
	grpc.ClientStream
}

type peerSendFileClient struct {
	grpc.ClientStream
}

func (x *peerSendFileClient) Send(m *BigFileChunk) error {
	return x.ClientStream.SendMsg(m)
}

func (x *peerSendFileClient) CloseAndRecv() (*BigFileAck, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(BigFileAck)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for Peer service

type PeerServer interface {
	SendFile(Peer_SendFileServer) error
}

func RegisterPeerServer(s *grpc.Server, srv PeerServer) {
	s.RegisterService(&_Peer_serviceDesc, srv)
}

func _Peer_SendFile_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(PeerServer).SendFile(&peerSendFileServer{stream})
}

type Peer_SendFileServer interface {
	SendAndClose(*BigFileAck) error
	Recv() (*BigFileChunk, error)
	grpc.ServerStream
}

type peerSendFileServer struct {
	grpc.ServerStream
}

func (x *peerSendFileServer) SendAndClose(m *BigFileAck) error {
	return x.ServerStream.SendMsg(m)
}

func (x *peerSendFileServer) Recv() (*BigFileChunk, error) {
	m := new(BigFileChunk)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Peer_serviceDesc = grpc.ServiceDesc{
	ServiceName: "streambigfile.Peer",
	HandlerType: (*PeerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SendFile",
			Handler:       _Peer_SendFile_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "sbf.proto",
}

func (m *BigFileChunk) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BigFileChunk) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Filepath) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintSbf(dAtA, i, uint64(len(m.Filepath)))
		i += copy(dAtA[i:], m.Filepath)
	}
	if m.SizeInBytes != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintSbf(dAtA, i, uint64(m.SizeInBytes))
	}
	if m.SendTime != 0 {
		dAtA[i] = 0x19
		i++
		i = encodeFixed64Sbf(dAtA, i, uint64(m.SendTime))
	}
	if len(m.Blake2B) > 0 {
		dAtA[i] = 0x22
		i++
		i = encodeVarintSbf(dAtA, i, uint64(len(m.Blake2B)))
		i += copy(dAtA[i:], m.Blake2B)
	}
	if len(m.Data) > 0 {
		dAtA[i] = 0x2a
		i++
		i = encodeVarintSbf(dAtA, i, uint64(len(m.Data)))
		i += copy(dAtA[i:], m.Data)
	}
	if m.ChunkCountMinus1 != 0 {
		dAtA[i] = 0x30
		i++
		i = encodeVarintSbf(dAtA, i, uint64(m.ChunkCountMinus1))
	}
	if m.ChunkNumber != 0 {
		dAtA[i] = 0x38
		i++
		i = encodeVarintSbf(dAtA, i, uint64(m.ChunkNumber))
	}
	return i, nil
}

func (m *BigFileAck) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BigFileAck) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Filepath) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintSbf(dAtA, i, uint64(len(m.Filepath)))
		i += copy(dAtA[i:], m.Filepath)
	}
	if m.SizeInBytes != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintSbf(dAtA, i, uint64(m.SizeInBytes))
	}
	if m.RecvTime != 0 {
		dAtA[i] = 0x19
		i++
		i = encodeFixed64Sbf(dAtA, i, uint64(m.RecvTime))
	}
	if len(m.WholeFileBlake2B) > 0 {
		dAtA[i] = 0x22
		i++
		i = encodeVarintSbf(dAtA, i, uint64(len(m.WholeFileBlake2B)))
		i += copy(dAtA[i:], m.WholeFileBlake2B)
	}
	return i, nil
}

func encodeFixed64Sbf(dAtA []byte, offset int, v uint64) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	dAtA[offset+4] = uint8(v >> 32)
	dAtA[offset+5] = uint8(v >> 40)
	dAtA[offset+6] = uint8(v >> 48)
	dAtA[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Sbf(dAtA []byte, offset int, v uint32) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintSbf(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *BigFileChunk) Size() (n int) {
	var l int
	_ = l
	l = len(m.Filepath)
	if l > 0 {
		n += 1 + l + sovSbf(uint64(l))
	}
	if m.SizeInBytes != 0 {
		n += 1 + sovSbf(uint64(m.SizeInBytes))
	}
	if m.SendTime != 0 {
		n += 9
	}
	l = len(m.Blake2B)
	if l > 0 {
		n += 1 + l + sovSbf(uint64(l))
	}
	l = len(m.Data)
	if l > 0 {
		n += 1 + l + sovSbf(uint64(l))
	}
	if m.ChunkCountMinus1 != 0 {
		n += 1 + sovSbf(uint64(m.ChunkCountMinus1))
	}
	if m.ChunkNumber != 0 {
		n += 1 + sovSbf(uint64(m.ChunkNumber))
	}
	return n
}

func (m *BigFileAck) Size() (n int) {
	var l int
	_ = l
	l = len(m.Filepath)
	if l > 0 {
		n += 1 + l + sovSbf(uint64(l))
	}
	if m.SizeInBytes != 0 {
		n += 1 + sovSbf(uint64(m.SizeInBytes))
	}
	if m.RecvTime != 0 {
		n += 9
	}
	l = len(m.WholeFileBlake2B)
	if l > 0 {
		n += 1 + l + sovSbf(uint64(l))
	}
	return n
}

func sovSbf(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozSbf(x uint64) (n int) {
	return sovSbf(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *BigFileChunk) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSbf
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BigFileChunk: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BigFileChunk: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Filepath", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSbf
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Filepath = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SizeInBytes", wireType)
			}
			m.SizeInBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SizeInBytes |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field SendTime", wireType)
			}
			m.SendTime = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += 8
			m.SendTime = uint64(dAtA[iNdEx-8])
			m.SendTime |= uint64(dAtA[iNdEx-7]) << 8
			m.SendTime |= uint64(dAtA[iNdEx-6]) << 16
			m.SendTime |= uint64(dAtA[iNdEx-5]) << 24
			m.SendTime |= uint64(dAtA[iNdEx-4]) << 32
			m.SendTime |= uint64(dAtA[iNdEx-3]) << 40
			m.SendTime |= uint64(dAtA[iNdEx-2]) << 48
			m.SendTime |= uint64(dAtA[iNdEx-1]) << 56
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Blake2B", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthSbf
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Blake2B = append(m.Blake2B[:0], dAtA[iNdEx:postIndex]...)
			if m.Blake2B == nil {
				m.Blake2B = []byte{}
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthSbf
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data[:0], dAtA[iNdEx:postIndex]...)
			if m.Data == nil {
				m.Data = []byte{}
			}
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChunkCountMinus1", wireType)
			}
			m.ChunkCountMinus1 = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ChunkCountMinus1 |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 7:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChunkNumber", wireType)
			}
			m.ChunkNumber = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ChunkNumber |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipSbf(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthSbf
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BigFileAck) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSbf
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BigFileAck: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BigFileAck: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Filepath", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSbf
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Filepath = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SizeInBytes", wireType)
			}
			m.SizeInBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SizeInBytes |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field RecvTime", wireType)
			}
			m.RecvTime = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += 8
			m.RecvTime = uint64(dAtA[iNdEx-8])
			m.RecvTime |= uint64(dAtA[iNdEx-7]) << 8
			m.RecvTime |= uint64(dAtA[iNdEx-6]) << 16
			m.RecvTime |= uint64(dAtA[iNdEx-5]) << 24
			m.RecvTime |= uint64(dAtA[iNdEx-4]) << 32
			m.RecvTime |= uint64(dAtA[iNdEx-3]) << 40
			m.RecvTime |= uint64(dAtA[iNdEx-2]) << 48
			m.RecvTime |= uint64(dAtA[iNdEx-1]) << 56
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field WholeFileBlake2B", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthSbf
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.WholeFileBlake2B = append(m.WholeFileBlake2B[:0], dAtA[iNdEx:postIndex]...)
			if m.WholeFileBlake2B == nil {
				m.WholeFileBlake2B = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSbf(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthSbf
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipSbf(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSbf
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSbf
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthSbf
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowSbf
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipSbf(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthSbf = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSbf   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("sbf.proto", fileDescriptorSbf) }

var fileDescriptorSbf = []byte{
	// 308 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x91, 0xcb, 0x4a, 0xc3, 0x40,
	0x18, 0x85, 0x3b, 0x36, 0xf6, 0x32, 0xad, 0x10, 0x66, 0x35, 0x56, 0x08, 0xa1, 0xab, 0xe0, 0xa2,
	0x60, 0x7d, 0x82, 0xa6, 0x52, 0x70, 0x61, 0x91, 0x54, 0x70, 0x3d, 0x93, 0xfe, 0x69, 0x86, 0xdc,
	0x4a, 0x66, 0xa2, 0xe8, 0x63, 0xb8, 0xf2, 0x91, 0x5c, 0xfa, 0x08, 0x12, 0x7d, 0x10, 0x99, 0x24,
	0xd0, 0x0b, 0x2e, 0x5d, 0x9e, 0xef, 0x9f, 0xcb, 0x39, 0xff, 0xc1, 0x7d, 0xc9, 0x83, 0xc9, 0x36,
	0xcf, 0x54, 0x46, 0xce, 0xa4, 0xca, 0x81, 0x25, 0x5c, 0x6c, 0x02, 0x11, 0xc3, 0xf8, 0x07, 0xe1,
	0xa1, 0x2b, 0x36, 0x0b, 0x11, 0xc3, 0x3c, 0x2c, 0xd2, 0x88, 0x8c, 0x70, 0x4f, 0x8b, 0x2d, 0x53,
	0x21, 0x45, 0x36, 0x72, 0xfa, 0x5e, 0x2f, 0x68, 0x34, 0xb1, 0xf1, 0x60, 0x25, 0x5e, 0xe1, 0x36,
	0x75, 0x5f, 0x14, 0x48, 0x7a, 0x62, 0x23, 0xa7, 0xed, 0x0d, 0xe4, 0x0e, 0xe9, 0xdb, 0x2b, 0x48,
	0xd7, 0x0f, 0x22, 0x01, 0xda, 0xb6, 0x91, 0xd3, 0xf1, 0x7a, 0xb2, 0xd1, 0x84, 0xe2, 0xae, 0x1b,
	0xb3, 0x08, 0xa6, 0x9c, 0x1a, 0x36, 0x72, 0x86, 0x5e, 0x97, 0xd7, 0x92, 0x10, 0x6c, 0xdc, 0x30,
	0xc5, 0xe8, 0x69, 0x85, 0x8d, 0x35, 0x53, 0x8c, 0x5c, 0x62, 0xb3, 0x32, 0x34, 0xcf, 0x8a, 0x54,
	0xdd, 0x89, 0xb4, 0x90, 0x57, 0xb4, 0x53, 0x7d, 0x68, 0xfa, 0x47, 0x5c, 0xfb, 0xaa, 0xce, 0x2e,
	0x8b, 0x84, 0x43, 0x4e, 0xbb, 0xb5, 0x2f, 0x7f, 0x87, 0xc6, 0x6f, 0x08, 0xe3, 0x26, 0xe6, 0xcc,
	0xff, 0x87, 0x90, 0x1e, 0xf8, 0x4f, 0xfb, 0x21, 0xf3, 0x46, 0x6b, 0xdb, 0x8f, 0x61, 0x16, 0x83,
	0x7e, 0xfe, 0x30, 0xad, 0xf9, 0x7c, 0xc4, 0xa7, 0x4b, 0x6c, 0xdc, 0x03, 0xe4, 0x64, 0x51, 0x2f,
	0x4d, 0x8f, 0xc8, 0xc5, 0xe4, 0xa0, 0x9f, 0xc9, 0x7e, 0x37, 0xa3, 0xf3, 0xbf, 0x87, 0x33, 0x3f,
	0x1a, 0xb7, 0x1c, 0xe4, 0x9a, 0x1f, 0xa5, 0x85, 0x3e, 0x4b, 0x0b, 0x7d, 0x95, 0x16, 0x7a, 0xff,
	0xb6, 0x5a, 0xbc, 0x53, 0x75, 0x7e, 0xfd, 0x1b, 0x00, 0x00, 0xff, 0xff, 0x70, 0x48, 0xb7, 0x3e,
	0x00, 0x02, 0x00, 0x00,
}