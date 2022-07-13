package raw_proto

import (
	"github.com/golang/protobuf/proto"
)

import (
	"github.com/dubbogo/grpc-go/encoding"
)

// ProtobufCodec is the protobuf impl of Codec interface
type ProtobufCodec struct{}

func (p *ProtobufCodec) Name() string {
	return "raw_proto"
}

// Marshal serialize interface @v to bytes
func (p *ProtobufCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

// Unmarshal deserialize @data to interface
func (p *ProtobufCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

func NewProtobufCodec() encoding.Codec {
	return &ProtobufCodec{}
}
