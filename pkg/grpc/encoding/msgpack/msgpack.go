package msgpack

import (
	"github.com/dubbogo/triple/pkg/grpc/encoding"
	"github.com/dubbogo/triple/pkg/grpc/encoding/raw_proto"

	mp "github.com/ugorji/go/codec"
)

func init() {
	encoding.RegisterCodec(encoding.NewPBWrapperTwoWayCodec("msgpack", NewMsgPackCodec(), raw_proto.NewProtobufCodec()))
}

// MsgPackCodec is the msgpack impl of common.Codec interface
type MsgPackCodec struct{}

func (p *MsgPackCodec) Name() string {
	return "raw_msgpack"
}

// Marshal serialize interface @v to bytes
func (p *MsgPackCodec) Marshal(v interface{}) ([]byte, error) {
	var out []byte
	encoder := mp.NewEncoderBytes(&out, new(mp.MsgpackHandle))
	return out, encoder.Encode(v)
}

// Unmarshal deserialize @data to interface
func (p *MsgPackCodec) Unmarshal(data []byte, v interface{}) error {
	dec := mp.NewDecoderBytes(data, new(mp.MsgpackHandle))
	return dec.Decode(v)
}

// NewMsgPackCodec returns new ProtobufCodec
func NewMsgPackCodec() encoding.Codec {
	return &MsgPackCodec{}
}