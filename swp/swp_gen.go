package swp

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *BigFile) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zxvk uint32
	zxvk, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zxvk > 0 {
		zxvk--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Filepath":
			z.Filepath, err = dc.ReadString()
			if err != nil {
				return
			}
		case "SizeInBytes":
			z.SizeInBytes, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "Blake2b":
			z.Blake2b, err = dc.ReadBytes(z.Blake2b)
			if err != nil {
				return
			}
		case "SendTime":
			z.SendTime, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "Data":
			z.Data, err = dc.ReadBytes(z.Data)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *BigFile) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 5
	// write "Filepath"
	err = en.Append(0x85, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x70, 0x61, 0x74, 0x68)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Filepath)
	if err != nil {
		return
	}
	// write "SizeInBytes"
	err = en.Append(0xab, 0x53, 0x69, 0x7a, 0x65, 0x49, 0x6e, 0x42, 0x79, 0x74, 0x65, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.SizeInBytes)
	if err != nil {
		return
	}
	// write "Blake2b"
	err = en.Append(0xa7, 0x42, 0x6c, 0x61, 0x6b, 0x65, 0x32, 0x62)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Blake2b)
	if err != nil {
		return
	}
	// write "SendTime"
	err = en.Append(0xa8, 0x53, 0x65, 0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.SendTime)
	if err != nil {
		return
	}
	// write "Data"
	err = en.Append(0xa4, 0x44, 0x61, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Data)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *BigFile) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 5
	// string "Filepath"
	o = append(o, 0x85, 0xa8, 0x46, 0x69, 0x6c, 0x65, 0x70, 0x61, 0x74, 0x68)
	o = msgp.AppendString(o, z.Filepath)
	// string "SizeInBytes"
	o = append(o, 0xab, 0x53, 0x69, 0x7a, 0x65, 0x49, 0x6e, 0x42, 0x79, 0x74, 0x65, 0x73)
	o = msgp.AppendInt64(o, z.SizeInBytes)
	// string "Blake2b"
	o = append(o, 0xa7, 0x42, 0x6c, 0x61, 0x6b, 0x65, 0x32, 0x62)
	o = msgp.AppendBytes(o, z.Blake2b)
	// string "SendTime"
	o = append(o, 0xa8, 0x53, 0x65, 0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65)
	o = msgp.AppendTime(o, z.SendTime)
	// string "Data"
	o = append(o, 0xa4, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendBytes(o, z.Data)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *BigFile) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zbzg uint32
	zbzg, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zbzg > 0 {
		zbzg--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Filepath":
			z.Filepath, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "SizeInBytes":
			z.SizeInBytes, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "Blake2b":
			z.Blake2b, bts, err = msgp.ReadBytesBytes(bts, z.Blake2b)
			if err != nil {
				return
			}
		case "SendTime":
			z.SendTime, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "Data":
			z.Data, bts, err = msgp.ReadBytesBytes(bts, z.Data)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *BigFile) Msgsize() (s int) {
	s = 1 + 9 + msgp.StringPrefixSize + len(z.Filepath) + 12 + msgp.Int64Size + 8 + msgp.BytesPrefixSize + len(z.Blake2b) + 9 + msgp.TimeSize + 5 + msgp.BytesPrefixSize + len(z.Data)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ByteAccount) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zbai uint32
	zbai, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zbai > 0 {
		zbai--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "NumBytesAcked":
			z.NumBytesAcked, err = dc.ReadInt64()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z ByteAccount) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 1
	// write "NumBytesAcked"
	err = en.Append(0x81, 0xad, 0x4e, 0x75, 0x6d, 0x42, 0x79, 0x74, 0x65, 0x73, 0x41, 0x63, 0x6b, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.NumBytesAcked)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ByteAccount) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "NumBytesAcked"
	o = append(o, 0x81, 0xad, 0x4e, 0x75, 0x6d, 0x42, 0x79, 0x74, 0x65, 0x73, 0x41, 0x63, 0x6b, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.NumBytesAcked)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ByteAccount) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zcmr uint32
	zcmr, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zcmr > 0 {
		zcmr--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "NumBytesAcked":
			z.NumBytesAcked, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z ByteAccount) Msgsize() (s int) {
	s = 1 + 14 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Packet) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zajw uint32
	zajw, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "From":
			z.From, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Dest":
			z.Dest, err = dc.ReadString()
			if err != nil {
				return
			}
		case "SendSessNonce":
			z.SendSessNonce, err = dc.ReadString()
			if err != nil {
				return
			}
		case "RecvSessNonce":
			z.RecvSessNonce, err = dc.ReadString()
			if err != nil {
				return
			}
		case "ArrivedAtDestTm":
			z.ArrivedAtDestTm, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "DataSendTm":
			z.DataSendTm, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "SeqNum":
			z.SeqNum, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "SeqRetry":
			z.SeqRetry, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "AckNum":
			z.AckNum, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "AckRetry":
			z.AckRetry, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "AckReplyTm":
			z.AckReplyTm, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "TcpEvent":
			{
				var zwht int
				zwht, err = dc.ReadInt()
				z.TcpEvent = TcpEvent(zwht)
			}
			if err != nil {
				return
			}
		case "AvailReaderBytesCap":
			z.AvailReaderBytesCap, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "AvailReaderMsgCap":
			z.AvailReaderMsgCap, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "FromRttEstNsec":
			z.FromRttEstNsec, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "FromRttSdNsec":
			z.FromRttSdNsec, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "FromRttN":
			z.FromRttN, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "CumulBytesTransmitted":
			z.CumulBytesTransmitted, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "Data":
			z.Data, err = dc.ReadBytes(z.Data)
			if err != nil {
				return
			}
		case "DataOffset":
			z.DataOffset, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "Blake2bChecksum":
			z.Blake2bChecksum, err = dc.ReadBytes(z.Blake2bChecksum)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Packet) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 21
	// write "From"
	err = en.Append(0xde, 0x0, 0x15, 0xa4, 0x46, 0x72, 0x6f, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteString(z.From)
	if err != nil {
		return
	}
	// write "Dest"
	err = en.Append(0xa4, 0x44, 0x65, 0x73, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Dest)
	if err != nil {
		return
	}
	// write "SendSessNonce"
	err = en.Append(0xad, 0x53, 0x65, 0x6e, 0x64, 0x53, 0x65, 0x73, 0x73, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.SendSessNonce)
	if err != nil {
		return
	}
	// write "RecvSessNonce"
	err = en.Append(0xad, 0x52, 0x65, 0x63, 0x76, 0x53, 0x65, 0x73, 0x73, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.RecvSessNonce)
	if err != nil {
		return
	}
	// write "ArrivedAtDestTm"
	err = en.Append(0xaf, 0x41, 0x72, 0x72, 0x69, 0x76, 0x65, 0x64, 0x41, 0x74, 0x44, 0x65, 0x73, 0x74, 0x54, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.ArrivedAtDestTm)
	if err != nil {
		return
	}
	// write "DataSendTm"
	err = en.Append(0xaa, 0x44, 0x61, 0x74, 0x61, 0x53, 0x65, 0x6e, 0x64, 0x54, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.DataSendTm)
	if err != nil {
		return
	}
	// write "SeqNum"
	err = en.Append(0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.SeqNum)
	if err != nil {
		return
	}
	// write "SeqRetry"
	err = en.Append(0xa8, 0x53, 0x65, 0x71, 0x52, 0x65, 0x74, 0x72, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.SeqRetry)
	if err != nil {
		return
	}
	// write "AckNum"
	err = en.Append(0xa6, 0x41, 0x63, 0x6b, 0x4e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AckNum)
	if err != nil {
		return
	}
	// write "AckRetry"
	err = en.Append(0xa8, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x74, 0x72, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AckRetry)
	if err != nil {
		return
	}
	// write "AckReplyTm"
	err = en.Append(0xaa, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x54, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.AckReplyTm)
	if err != nil {
		return
	}
	// write "TcpEvent"
	err = en.Append(0xa8, 0x54, 0x63, 0x70, 0x45, 0x76, 0x65, 0x6e, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.TcpEvent))
	if err != nil {
		return
	}
	// write "AvailReaderBytesCap"
	err = en.Append(0xb3, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42, 0x79, 0x74, 0x65, 0x73, 0x43, 0x61, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AvailReaderBytesCap)
	if err != nil {
		return
	}
	// write "AvailReaderMsgCap"
	err = en.Append(0xb1, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x73, 0x67, 0x43, 0x61, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AvailReaderMsgCap)
	if err != nil {
		return
	}
	// write "FromRttEstNsec"
	err = en.Append(0xae, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x45, 0x73, 0x74, 0x4e, 0x73, 0x65, 0x63)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.FromRttEstNsec)
	if err != nil {
		return
	}
	// write "FromRttSdNsec"
	err = en.Append(0xad, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x53, 0x64, 0x4e, 0x73, 0x65, 0x63)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.FromRttSdNsec)
	if err != nil {
		return
	}
	// write "FromRttN"
	err = en.Append(0xa8, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x4e)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.FromRttN)
	if err != nil {
		return
	}
	// write "CumulBytesTransmitted"
	err = en.Append(0xb5, 0x43, 0x75, 0x6d, 0x75, 0x6c, 0x42, 0x79, 0x74, 0x65, 0x73, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x74, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.CumulBytesTransmitted)
	if err != nil {
		return
	}
	// write "Data"
	err = en.Append(0xa4, 0x44, 0x61, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Data)
	if err != nil {
		return
	}
	// write "DataOffset"
	err = en.Append(0xaa, 0x44, 0x61, 0x74, 0x61, 0x4f, 0x66, 0x66, 0x73, 0x65, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.DataOffset)
	if err != nil {
		return
	}
	// write "Blake2bChecksum"
	err = en.Append(0xaf, 0x42, 0x6c, 0x61, 0x6b, 0x65, 0x32, 0x62, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x73, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Blake2bChecksum)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Packet) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 21
	// string "From"
	o = append(o, 0xde, 0x0, 0x15, 0xa4, 0x46, 0x72, 0x6f, 0x6d)
	o = msgp.AppendString(o, z.From)
	// string "Dest"
	o = append(o, 0xa4, 0x44, 0x65, 0x73, 0x74)
	o = msgp.AppendString(o, z.Dest)
	// string "SendSessNonce"
	o = append(o, 0xad, 0x53, 0x65, 0x6e, 0x64, 0x53, 0x65, 0x73, 0x73, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	o = msgp.AppendString(o, z.SendSessNonce)
	// string "RecvSessNonce"
	o = append(o, 0xad, 0x52, 0x65, 0x63, 0x76, 0x53, 0x65, 0x73, 0x73, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	o = msgp.AppendString(o, z.RecvSessNonce)
	// string "ArrivedAtDestTm"
	o = append(o, 0xaf, 0x41, 0x72, 0x72, 0x69, 0x76, 0x65, 0x64, 0x41, 0x74, 0x44, 0x65, 0x73, 0x74, 0x54, 0x6d)
	o = msgp.AppendTime(o, z.ArrivedAtDestTm)
	// string "DataSendTm"
	o = append(o, 0xaa, 0x44, 0x61, 0x74, 0x61, 0x53, 0x65, 0x6e, 0x64, 0x54, 0x6d)
	o = msgp.AppendTime(o, z.DataSendTm)
	// string "SeqNum"
	o = append(o, 0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, z.SeqNum)
	// string "SeqRetry"
	o = append(o, 0xa8, 0x53, 0x65, 0x71, 0x52, 0x65, 0x74, 0x72, 0x79)
	o = msgp.AppendInt64(o, z.SeqRetry)
	// string "AckNum"
	o = append(o, 0xa6, 0x41, 0x63, 0x6b, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, z.AckNum)
	// string "AckRetry"
	o = append(o, 0xa8, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x74, 0x72, 0x79)
	o = msgp.AppendInt64(o, z.AckRetry)
	// string "AckReplyTm"
	o = append(o, 0xaa, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x54, 0x6d)
	o = msgp.AppendTime(o, z.AckReplyTm)
	// string "TcpEvent"
	o = append(o, 0xa8, 0x54, 0x63, 0x70, 0x45, 0x76, 0x65, 0x6e, 0x74)
	o = msgp.AppendInt(o, int(z.TcpEvent))
	// string "AvailReaderBytesCap"
	o = append(o, 0xb3, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42, 0x79, 0x74, 0x65, 0x73, 0x43, 0x61, 0x70)
	o = msgp.AppendInt64(o, z.AvailReaderBytesCap)
	// string "AvailReaderMsgCap"
	o = append(o, 0xb1, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x73, 0x67, 0x43, 0x61, 0x70)
	o = msgp.AppendInt64(o, z.AvailReaderMsgCap)
	// string "FromRttEstNsec"
	o = append(o, 0xae, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x45, 0x73, 0x74, 0x4e, 0x73, 0x65, 0x63)
	o = msgp.AppendInt64(o, z.FromRttEstNsec)
	// string "FromRttSdNsec"
	o = append(o, 0xad, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x53, 0x64, 0x4e, 0x73, 0x65, 0x63)
	o = msgp.AppendInt64(o, z.FromRttSdNsec)
	// string "FromRttN"
	o = append(o, 0xa8, 0x46, 0x72, 0x6f, 0x6d, 0x52, 0x74, 0x74, 0x4e)
	o = msgp.AppendInt64(o, z.FromRttN)
	// string "CumulBytesTransmitted"
	o = append(o, 0xb5, 0x43, 0x75, 0x6d, 0x75, 0x6c, 0x42, 0x79, 0x74, 0x65, 0x73, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x74, 0x74, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.CumulBytesTransmitted)
	// string "Data"
	o = append(o, 0xa4, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendBytes(o, z.Data)
	// string "DataOffset"
	o = append(o, 0xaa, 0x44, 0x61, 0x74, 0x61, 0x4f, 0x66, 0x66, 0x73, 0x65, 0x74)
	o = msgp.AppendInt(o, z.DataOffset)
	// string "Blake2bChecksum"
	o = append(o, 0xaf, 0x42, 0x6c, 0x61, 0x6b, 0x65, 0x32, 0x62, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x73, 0x75, 0x6d)
	o = msgp.AppendBytes(o, z.Blake2bChecksum)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Packet) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zhct uint32
	zhct, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zhct > 0 {
		zhct--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "From":
			z.From, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Dest":
			z.Dest, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "SendSessNonce":
			z.SendSessNonce, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "RecvSessNonce":
			z.RecvSessNonce, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "ArrivedAtDestTm":
			z.ArrivedAtDestTm, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "DataSendTm":
			z.DataSendTm, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "SeqNum":
			z.SeqNum, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "SeqRetry":
			z.SeqRetry, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "AckNum":
			z.AckNum, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "AckRetry":
			z.AckRetry, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "AckReplyTm":
			z.AckReplyTm, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "TcpEvent":
			{
				var zcua int
				zcua, bts, err = msgp.ReadIntBytes(bts)
				z.TcpEvent = TcpEvent(zcua)
			}
			if err != nil {
				return
			}
		case "AvailReaderBytesCap":
			z.AvailReaderBytesCap, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "AvailReaderMsgCap":
			z.AvailReaderMsgCap, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "FromRttEstNsec":
			z.FromRttEstNsec, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "FromRttSdNsec":
			z.FromRttSdNsec, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "FromRttN":
			z.FromRttN, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "CumulBytesTransmitted":
			z.CumulBytesTransmitted, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "Data":
			z.Data, bts, err = msgp.ReadBytesBytes(bts, z.Data)
			if err != nil {
				return
			}
		case "DataOffset":
			z.DataOffset, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Blake2bChecksum":
			z.Blake2bChecksum, bts, err = msgp.ReadBytesBytes(bts, z.Blake2bChecksum)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Packet) Msgsize() (s int) {
	s = 3 + 5 + msgp.StringPrefixSize + len(z.From) + 5 + msgp.StringPrefixSize + len(z.Dest) + 14 + msgp.StringPrefixSize + len(z.SendSessNonce) + 14 + msgp.StringPrefixSize + len(z.RecvSessNonce) + 16 + msgp.TimeSize + 11 + msgp.TimeSize + 7 + msgp.Int64Size + 9 + msgp.Int64Size + 7 + msgp.Int64Size + 9 + msgp.Int64Size + 11 + msgp.TimeSize + 9 + msgp.IntSize + 20 + msgp.Int64Size + 18 + msgp.Int64Size + 15 + msgp.Int64Size + 14 + msgp.Int64Size + 9 + msgp.Int64Size + 22 + msgp.Int64Size + 5 + msgp.BytesPrefixSize + len(z.Data) + 11 + msgp.IntSize + 16 + msgp.BytesPrefixSize + len(z.Blake2bChecksum)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *SynAckAck) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zlqf uint32
	zlqf, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zlqf > 0 {
		zlqf--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "TcpEvent":
			{
				var zdaf int
				zdaf, err = dc.ReadInt()
				z.TcpEvent = TcpEvent(zdaf)
			}
			if err != nil {
				return
			}
		case "SessionNonce":
			z.SessionNonce, err = dc.ReadString()
			if err != nil {
				return
			}
		case "NextExpected":
			z.NextExpected, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "NackList":
			var zpks uint32
			zpks, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.NackList) >= int(zpks) {
				z.NackList = (z.NackList)[:zpks]
			} else {
				z.NackList = make([]int64, zpks)
			}
			for zxhx := range z.NackList {
				z.NackList[zxhx], err = dc.ReadInt64()
				if err != nil {
					return
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *SynAckAck) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "TcpEvent"
	err = en.Append(0x84, 0xa8, 0x54, 0x63, 0x70, 0x45, 0x76, 0x65, 0x6e, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.TcpEvent))
	if err != nil {
		return
	}
	// write "SessionNonce"
	err = en.Append(0xac, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.SessionNonce)
	if err != nil {
		return
	}
	// write "NextExpected"
	err = en.Append(0xac, 0x4e, 0x65, 0x78, 0x74, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.NextExpected)
	if err != nil {
		return
	}
	// write "NackList"
	err = en.Append(0xa8, 0x4e, 0x61, 0x63, 0x6b, 0x4c, 0x69, 0x73, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteArrayHeader(uint32(len(z.NackList)))
	if err != nil {
		return
	}
	for zxhx := range z.NackList {
		err = en.WriteInt64(z.NackList[zxhx])
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *SynAckAck) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "TcpEvent"
	o = append(o, 0x84, 0xa8, 0x54, 0x63, 0x70, 0x45, 0x76, 0x65, 0x6e, 0x74)
	o = msgp.AppendInt(o, int(z.TcpEvent))
	// string "SessionNonce"
	o = append(o, 0xac, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x4e, 0x6f, 0x6e, 0x63, 0x65)
	o = msgp.AppendString(o, z.SessionNonce)
	// string "NextExpected"
	o = append(o, 0xac, 0x4e, 0x65, 0x78, 0x74, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.NextExpected)
	// string "NackList"
	o = append(o, 0xa8, 0x4e, 0x61, 0x63, 0x6b, 0x4c, 0x69, 0x73, 0x74)
	o = msgp.AppendArrayHeader(o, uint32(len(z.NackList)))
	for zxhx := range z.NackList {
		o = msgp.AppendInt64(o, z.NackList[zxhx])
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *SynAckAck) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zjfb uint32
	zjfb, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zjfb > 0 {
		zjfb--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "TcpEvent":
			{
				var zcxo int
				zcxo, bts, err = msgp.ReadIntBytes(bts)
				z.TcpEvent = TcpEvent(zcxo)
			}
			if err != nil {
				return
			}
		case "SessionNonce":
			z.SessionNonce, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "NextExpected":
			z.NextExpected, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "NackList":
			var zeff uint32
			zeff, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.NackList) >= int(zeff) {
				z.NackList = (z.NackList)[:zeff]
			} else {
				z.NackList = make([]int64, zeff)
			}
			for zxhx := range z.NackList {
				z.NackList[zxhx], bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					return
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *SynAckAck) Msgsize() (s int) {
	s = 1 + 9 + msgp.IntSize + 13 + msgp.StringPrefixSize + len(z.SessionNonce) + 13 + msgp.Int64Size + 9 + msgp.ArrayHeaderSize + (len(z.NackList) * (msgp.Int64Size))
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TcpEvent) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zrsw int
		zrsw, err = dc.ReadInt()
		(*z) = TcpEvent(zrsw)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TcpEvent) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TcpEvent) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TcpEvent) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zxpk int
		zxpk, bts, err = msgp.ReadIntBytes(bts)
		(*z) = TcpEvent(zxpk)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TcpEvent) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TerminatedError) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zdnj uint32
	zdnj, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zdnj > 0 {
		zdnj--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Msg":
			z.Msg, err = dc.ReadString()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TerminatedError) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 1
	// write "Msg"
	err = en.Append(0x81, 0xa3, 0x4d, 0x73, 0x67)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Msg)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TerminatedError) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "Msg"
	o = append(o, 0x81, 0xa3, 0x4d, 0x73, 0x67)
	o = msgp.AppendString(o, z.Msg)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TerminatedError) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zobc uint32
	zobc, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zobc > 0 {
		zobc--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Msg":
			z.Msg, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TerminatedError) Msgsize() (s int) {
	s = 1 + 4 + msgp.StringPrefixSize + len(z.Msg)
	return
}
