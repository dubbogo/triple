package common

import (
	"fmt"
)

import (
	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

// Codec is used to marshal interface to bytes and unmarshal bytes to interface.
// It is not used directly by triple network, instead, it used by TwoWayCodec, and TwoWayCodec is
// directly used by triple processor/h2Controller
type Codec interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

// CodecFactory is Codec Factory
type CodecFactory func() Codec

// codecFactoryMap stores map
var codecFactoryMap = make(map[string]CodecFactory)

// codecInWrapperSerializerTypeMap stores Map of [constant.CodecType -> SerializeType of proto.TripleRequestWrapper]
var codecInWrapperSerializerTypeMap = make(map[string]string)

// SetTripleCodec register CodecFactory @f and CodecType @codecType, with @opt[0].SerializerTypeInWrapper
func SetTripleCodec(codecType constant.CodecType, f CodecFactory, opt ...*config.Option) {
	codecFactoryMap[string(codecType)] = f
	if len(opt) == 0 {
		return
	}
	if opt[0].SerializerTypeInWrapper == "" {
		return
	}
	codecInWrapperSerializerTypeMap[string(codecType)] = opt[0].SerializerTypeInWrapper
}

// GetTripleCodec get Codec impl by @codecName
func GetTripleCodec(codecName constant.CodecType) (Codec, error) {
	if f, ok := codecFactoryMap[string(codecName)]; ok {
		return f(), nil
	}
	return nil, perrors.New(fmt.Sprintf("Codec %s factory undefined!", codecName))
}

// GetCodecInWrapperName get SerializeType of proto.TripleRequestWrapper from CodecType @name registered before
func GetCodecInWrapperName(name constant.CodecType) string {
	if inWrapperName, ok := codecInWrapperSerializerTypeMap[string(name)]; ok {
		return inWrapperName
	}
	return string(name)
}

// TwoWayCodec is directly used by triple network logic
// It can specify the marshal and unmarshal logic of req and rsp
type TwoWayCodec interface {
	MarshalRequest(interface{}) ([]byte, error)
	MarshalResponse(interface{}) ([]byte, error)
	UnmarshalRequest(data []byte, v interface{}) error
	UnmarshalResponse(data []byte, v interface{}) error
}
