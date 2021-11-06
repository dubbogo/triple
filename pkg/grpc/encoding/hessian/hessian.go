package hessian

import (
	hessian "github.com/apache/dubbo-go-hessian2"
	"github.com/dubbogo/triple/pkg/grpc/encoding"
	"github.com/dubbogo/triple/pkg/grpc/encoding/raw_proto"
	"github.com/dubbogo/triple/pkg/grpc/encoding/tools"
)

func init() {
	encoding.RegisterCodec(encoding.NewPBWrapperTwoWayCodec("hessian2", NewHessianCodec(), raw_proto.NewProtobufCodec()))
}

// HessianCodeC is the hessian impl of Codec interface
type HessianCodeC struct{}

func (h *HessianCodeC) Name() string {
	return "raw_hessian2"
}

// Marshal serialize interface @v to bytes
func (h *HessianCodeC) Marshal(v interface{}) ([]byte, error) {
	encoder := hessian.NewEncoder()
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return encoder.Buffer(), nil
}

// Unmarshal deserialize @data to interface
func (h *HessianCodeC) Unmarshal(data []byte, v interface{}) error {
	decoder := hessian.NewDecoder(data)
	val, err := decoder.Decode()
	if err != nil {
		return err
	}
	return tools.ReflectResponse(val, v)
}

// NewHessianCodec returns new HessianCodeC
func NewHessianCodec() encoding.Codec {
	return &HessianCodeC{}
}
