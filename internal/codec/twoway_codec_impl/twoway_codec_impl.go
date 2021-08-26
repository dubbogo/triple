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

package twoway_codec_impl

import (
	"github.com/dubbogo/triple/internal/codec"
	"github.com/dubbogo/triple/internal/codec/codec_impl"
	proto2 "github.com/dubbogo/triple/internal/codec/proto"
	"github.com/dubbogo/triple/internal/codes"
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
)

func NewTwoWayCodec(codecName constant.CodecType) (common.TwoWayCodec, error) {
	if codecName == constant.PBCodecName {
		return NewPBTwoWayCodec(), nil
	} else {
		return NewPBWrapperTwoWayCodec(codecName)
	}
}

// PBWrapperTwoWayCodec is codec impl of pb
type PBWrapperTwoWayCodec struct {
	codecName constant.CodecType
	pbCodec   common.Codec
	codec     common.Codec
}

// NewPBWrapperTwoWayCodec new common.TwoWayCodec PBWrapperTwoWayCodec with @codecName defined Codec inside
func NewPBWrapperTwoWayCodec(codecName constant.CodecType) (common.TwoWayCodec, error) {
	tripleCodec, err := common.GetTripleCodec(codecName)
	if err != nil {
		return nil, err
	}
	pbCodec, err := common.GetTripleCodec("protobuf")
	if err != nil {
		return nil, err
	}
	return &PBWrapperTwoWayCodec{
		codec:     tripleCodec,
		codecName: codecName,
		pbCodec:   pbCodec,
	}, err
}

// MarshalRequest marshal interface @v to []byte
func (h *PBWrapperTwoWayCodec) MarshalRequest(v interface{}) ([]byte, error) {
	argsBytes := make([][]byte, 0)
	argsTypes := make([]string, 0)
	reqList := v.([]interface{})
	for _, value := range reqList {
		data, err := h.codec.Marshal(value)
		if err != nil {
			return nil, err
		}
		argsBytes = append(argsBytes, data)
		argsTypes = append(argsTypes, codec.GetArgType(value))
	}

	wrapperRequest := &proto2.TripleRequestWrapper{
		SerializeType: common.GetCodecInWrapperName(h.codecName),
		Args:          argsBytes,
		ArgTypes:      argsTypes,
	}
	return h.pbCodec.Marshal(wrapperRequest)
}

// UnmarshalRequest unmarshal bytes @data to interface
func (h *PBWrapperTwoWayCodec) UnmarshalRequest(data []byte, v interface{}) error {
	wrapperRequest := proto2.TripleRequestWrapper{}
	err := h.pbCodec.Unmarshal(data, &wrapperRequest)
	if err != nil {
		return err
	}

	paramsInterfaces := v.([]interface{})
	if len(paramsInterfaces) != len(wrapperRequest.Args) {
		return status.Errorf(codes.Internal, "error ,request params len is %d, but exported method has %d", len(wrapperRequest.Args), len(paramsInterfaces))
	}

	for idx, value := range wrapperRequest.Args {
		if err := h.codec.Unmarshal(value, paramsInterfaces[idx]); err != nil {
			return err
		}
	}

	return nil
}

// MarshalResponse marshal interface @v to []byte
func (h *PBWrapperTwoWayCodec) MarshalResponse(v interface{}) ([]byte, error) {
	data, err := h.codec.Marshal(v)
	if err != nil {
		return nil, err
	}

	wrapperRequest := &proto2.TripleResponseWrapper{
		SerializeType: common.GetCodecInWrapperName(h.codecName),
		Data:          data,
		Type:          codec.GetArgType(v),
	}
	return h.pbCodec.Marshal(wrapperRequest)
}

// UnmarshalResponse unmarshal bytes @data to interface
func (h *PBWrapperTwoWayCodec) UnmarshalResponse(data []byte, v interface{}) error {
	wrapperResponse := proto2.TripleResponseWrapper{}
	err := h.pbCodec.Unmarshal(data, &wrapperResponse)
	if err != nil {
		return err
	}
	if v == nil { // empty respose
		return nil
	}
	return h.codec.Unmarshal(wrapperResponse.Data, v)
}

// PBTwoWayCodec is pb impl of TwoWayCodec
type PBTwoWayCodec struct {
	codec common.Codec
}

// NewPBTwoWayCodec new PBTwoWayCodec instance
func NewPBTwoWayCodec() common.TwoWayCodec {
	return &PBTwoWayCodec{
		codec: codec_impl.NewProtobufCodec(),
	}
}

// MarshalRequest marshal interface @v to []byte
func (h *PBTwoWayCodec) MarshalRequest(v interface{}) ([]byte, error) {
	return h.codec.Marshal(v)
}

// UnmarshalRequest unmarshal bytes @data to interface
func (h *PBTwoWayCodec) UnmarshalRequest(data []byte, v interface{}) error {
	return h.codec.Unmarshal(data, v)
}

// MarshalResponse marshal interface @v to []byte
func (h *PBTwoWayCodec) MarshalResponse(v interface{}) ([]byte, error) {
	return h.codec.Marshal(v)
}

// UnmarshalResponse unmarshal bytes @data to interface
func (h *PBTwoWayCodec) UnmarshalResponse(data []byte, v interface{}) error {
	return h.codec.Unmarshal(data, v)
}
