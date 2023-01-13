package hessian

import (
	hessian "github.com/apache/dubbo-go-hessian2"
	"github.com/dubbogo/grpc-go/encoding"
	"github.com/dubbogo/triple/pkg/common/encoding/raw_proto"

	pbwrapper "github.com/dubbogo/triple/pkg/common/encoding"
	"github.com/dubbogo/triple/pkg/common/encoding/tools"
)

func init() {
	encoding.RegisterCodec(pbwrapper.NewPBWrapperTwoWayCodec("hessian2", NewHessianCodec(), raw_proto.NewProtobufCodec()))
}

// HessianCodeC is the hessian impl of Codec interface
type HessianCodec struct{}

func (h *HessianCodec) Name() string {
	return "raw_hessian2"
}

// Marshal serialize interface @v to bytes
func (h *HessianCodec) Marshal(v interface{}) ([]byte, error) {
	encoder := hessian.NewEncoder()
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return encoder.Buffer(), nil
}

// Unmarshal deserialize @data to interface
func (h *HessianCodec) Unmarshal(data []byte, v interface{}) error {
	decoder := hessian.NewDecoder(data)
	val, err := decoder.Decode()
	if err != nil {
		return err
	}
	return tools.ReflectResponse(val, v)
}

// NewHessianCodec returns new HessianCodeC
func NewHessianCodec() encoding.Codec {
	return &HessianCodec{}
}
