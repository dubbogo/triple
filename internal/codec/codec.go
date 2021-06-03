/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package codec

import (
	hessian "github.com/apache/dubbo-go-hessian2"
	"github.com/golang/protobuf/proto"
	perrors "github.com/pkg/errors"
	mp "github.com/ugorji/go/codec"
)

import (
	proto2 "github.com/dubbogo/triple/internal/codec/proto"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
)

func init() {
	common.SetDubbo3Serializer(constant.PBSerializerName, NewProtobufCodeC)
	common.SetDubbo3Serializer(constant.HessianSerializerName, NewHessianCodeC)
	common.SetDubbo3Serializer(constant.TripleHessianWrapperSerializerName, NewTripleHessianWrapperSerializer)
	common.SetDubbo3Serializer(constant.MsgPackSerializerName, NewTripleMsgPackWrapperSerializer)
}

// ProtobufCodeC is the protobuf impl of Dubbo3Serializer interface
type ProtobufCodeC struct{}

// Marshal serialize interface @v to bytes
func (p *ProtobufCodeC) MarshalRequest(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

// Unmarshal deserialize @data to interface
func (p *ProtobufCodeC) UnmarshalRequest(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

// Marshal serialize interface @v to bytes
func (p *ProtobufCodeC) MarshalResponse(v interface{}) ([]byte, error) {
	return p.MarshalRequest(v)
}

// Unmarshal deserialize @data to interface
func (p *ProtobufCodeC) UnmarshalResponse(data []byte, v interface{}) error {
	return p.UnmarshalRequest(data, v)
}

// NewProtobufCodeC returns new ProtobufCodeC
func NewProtobufCodeC() common.Dubbo3Serializer {
	return &ProtobufCodeC{}
}

//// HessianTransferPackage is hessian encode package, wrapping pb data
//type HessianTransferPackage struct {
//	Length int
//	Type   string
//	Data   []byte
//}
//
//func (HessianTransferPackage) JavaClassName() string {
//	return "org.apache.dubbo.HessianPkg"
//}

// nolint
type HessianCodeC struct{}

// nolint
func (h *HessianCodeC) MarshalRequest(v interface{}) ([]byte, error) {
	encoder := hessian.NewEncoder()
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return encoder.Buffer(), nil
}

// nolint
type HessianUnmarshalStruct struct {
	Val interface{}
}

// nolint
func (h *HessianCodeC) UnmarshalRequest(data []byte, v interface{}) error {
	decoder := hessian.NewDecoder(data)
	val, err := decoder.Decode()
	v.(*HessianUnmarshalStruct).Val = val
	return err
}

// nolint
func (h *HessianCodeC) MarshalResponse(v interface{}) ([]byte, error) {
	return h.MarshalRequest(v)
}

// nolint
func (h *HessianCodeC) UnmarshalResponse(data []byte, v interface{}) error {
	return h.UnmarshalRequest(data, v)
}

// NewHessianCodeC returns new HessianCodeC
func NewHessianCodeC() common.Dubbo3Serializer {
	return &HessianCodeC{}
}

// TripleHessianWrapperSerializer
type TripleHessianWrapperSerializer struct {
	pbSerializer      common.Dubbo3Serializer
	hessianSerializer common.Dubbo3Serializer
}

// nolint
func NewTripleHessianWrapperSerializer() common.Dubbo3Serializer {
	return &TripleHessianWrapperSerializer{
		pbSerializer:      NewProtobufCodeC(),
		hessianSerializer: NewHessianCodeC(),
	}
}

// nolint
func (h *TripleHessianWrapperSerializer) MarshalRequest(v interface{}) ([]byte, error) {
	args := v.([]interface{})
	argsBytes := make([][]byte, 0)
	argsTypes := make([]string, 0)
	for _, v := range args {
		data, err := h.hessianSerializer.MarshalRequest(v)
		if err != nil {
			return nil, err
		}
		argsBytes = append(argsBytes, data)
		argsTypes = append(argsTypes, getArgType(v))
	}

	wrapperRequest := &proto2.TripleRequestWrapper{
		SerializeType: string(constant.HessianSerializerName),
		Args:          argsBytes,
		// todo to java name
		ArgTypes: argsTypes,
	}
	return h.pbSerializer.MarshalRequest(wrapperRequest)
}

// nolint
func (h *TripleHessianWrapperSerializer) UnmarshalRequest(data []byte, v interface{}) error {
	wrapperRequest := proto2.TripleRequestWrapper{}
	err := h.pbSerializer.UnmarshalRequest(data, &wrapperRequest)
	if err != nil {
		return err
	}
	args := []interface{}{}
	for _, v := range wrapperRequest.Args {
		arg := HessianUnmarshalStruct{}
		if err := h.hessianSerializer.UnmarshalRequest(v, &arg); err != nil {
			return err
		}
		args = append(args, arg.Val)
	}
	v.(*HessianUnmarshalStruct).Val = args
	return nil
}

// nolint
func (h *TripleHessianWrapperSerializer) MarshalResponse(v interface{}) ([]byte, error) {
	data, err := h.hessianSerializer.MarshalResponse(v)
	if err != nil {
		return nil, err
	}
	wrapperRequest := &proto2.TripleResponseWrapper{
		SerializeType: string(constant.HessianSerializerName),
		Data:          data,
		Type:          getArgType(v),
	}
	return h.pbSerializer.MarshalResponse(wrapperRequest)
}

// nolint
func (h *TripleHessianWrapperSerializer) UnmarshalResponse(data []byte, v interface{}) error {
	wrapperResponse := proto2.TripleResponseWrapper{}
	err := h.pbSerializer.UnmarshalResponse(data, &wrapperResponse)
	if err != nil {
		return err
	}
	return h.hessianSerializer.UnmarshalResponse(wrapperResponse.Data, v)
}

// MsgPackSerializer is the msgpack impl of Dubbo3Serializer interface
type MsgPackSerializer struct{}

// MarshalRequest serialize interface @v to bytes
func (p *MsgPackSerializer) MarshalRequest(v interface{}) ([]byte, error) {
	var out []byte
	encoder := mp.NewEncoderBytes(&out, new(mp.MsgpackHandle))
	err := encoder.Encode(v)
	return out, err
}

// UnmarshalRequest deserialize @data to interface
func (p *MsgPackSerializer) UnmarshalRequest(data []byte, v interface{}) error {
	dec := mp.NewDecoderBytes(data, new(mp.MsgpackHandle))
	return dec.Decode(v)
}

// MarshalResponse serialize interface @v to bytes
func (p *MsgPackSerializer) MarshalResponse(v interface{}) ([]byte, error) {
	return p.MarshalRequest(v)
}

// UnmarshalResponse deserialize @data to interface
func (p *MsgPackSerializer) UnmarshalResponse(data []byte, v interface{}) error {
	return p.UnmarshalRequest(data, v)
}

// NewMsgPackSerializer returns new ProtobufCodeC
func NewMsgPackSerializer() common.Dubbo3Serializer {
	return &MsgPackSerializer{}
}

// TripleHessianWrapperSerializer
type TripleMsgPackWrapperSerializer struct {
	pbSerializer      common.Dubbo3Serializer
	msgPackSerializer common.Dubbo3Serializer
}

// nolint
func NewTripleMsgPackWrapperSerializer() common.Dubbo3Serializer {
	return &TripleMsgPackWrapperSerializer{
		pbSerializer:      NewProtobufCodeC(),
		msgPackSerializer: NewMsgPackSerializer(),
	}
}

// nolint
func (h *TripleMsgPackWrapperSerializer) MarshalRequest(v interface{}) ([]byte, error) {
	argsBytes := make([][]byte, 0)
	argsTypes := make([]string, 0)
	data, err := h.msgPackSerializer.MarshalRequest(v)
	if err != nil {
		return nil, err
	}
	argsBytes = append(argsBytes, data)
	argsTypes = append(argsTypes, getArgType(v))

	wrapperRequest := &proto2.TripleRequestWrapper{
		SerializeType: string(constant.HessianSerializerName),
		Args:          argsBytes,
		// todo to java name
		ArgTypes: argsTypes,
	}
	return h.pbSerializer.MarshalRequest(wrapperRequest)
}

// nolint
func (h *TripleMsgPackWrapperSerializer) UnmarshalRequest(data []byte, v interface{}) error {
	wrapperRequest := proto2.TripleRequestWrapper{}
	err := h.pbSerializer.UnmarshalRequest(data, &wrapperRequest)
	if err != nil {
		return err
	}
	if len(wrapperRequest.Args) != 1 {
		return perrors.New("wrapper request args len is not 1")
	}
	if err := h.msgPackSerializer.UnmarshalRequest(wrapperRequest.Args[0], v); err != nil {
		return err
	}
	return nil
}

// nolint
func (h *TripleMsgPackWrapperSerializer) MarshalResponse(v interface{}) ([]byte, error) {
	data, err := h.msgPackSerializer.MarshalResponse(v)
	if err != nil {
		return nil, err
	}
	wrapperRequest := &proto2.TripleResponseWrapper{
		SerializeType: string(constant.MsgPackSerializerName),
		Data:          data,
		Type:          getArgType(v),
	}
	return h.pbSerializer.MarshalResponse(wrapperRequest)
}

// nolint
func (h *TripleMsgPackWrapperSerializer) UnmarshalResponse(data []byte, v interface{}) error {
	wrapperResponse := proto2.TripleResponseWrapper{}
	err := h.pbSerializer.UnmarshalResponse(data, &wrapperResponse)
	if err != nil {
		return err
	}
	return h.msgPackSerializer.UnmarshalResponse(wrapperResponse.Data, v)
}
