package swp

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// DecodeMsg implements msgp.Decodable
func (z *TcpAction) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zxvk int
		zxvk, err = dc.ReadInt()
		(*z) = TcpAction(zxvk)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TcpAction) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TcpAction) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TcpAction) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zbzg int
		zbzg, bts, err = msgp.ReadIntBytes(bts)
		(*z) = TcpAction(zbzg)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TcpAction) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TcpState) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zbai int
		zbai, err = dc.ReadInt()
		(*z) = TcpState(zbai)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TcpState) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TcpState) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TcpState) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zcmr int
		zcmr, bts, err = msgp.ReadIntBytes(bts)
		(*z) = TcpState(zcmr)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TcpState) Msgsize() (s int) {
	s = msgp.IntSize
	return
}
