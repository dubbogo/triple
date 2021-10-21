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

package codec_impl

import (
	"encoding/json"
)

import (
	hessian "github.com/apache/dubbo-go-hessian2"

	"github.com/golang/protobuf/proto"

	perrors "github.com/pkg/errors"

	mp "github.com/ugorji/go/codec"
)

import (
	proto2 "github.com/dubbogo/triple/internal/codec/proto"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
)

func init() {
	common.SetTripleCodec(constant.PBCodecName, NewProtobufCodec)
	common.SetTripleCodec(constant.HessianCodecName, NewHessianCodec)
	common.SetTripleCodec(constant.MsgPackCodecName, NewMsgPackCodec)
	common.SetTripleCodec(constant.JSONMapStructCodec, NewJSONMapStruct)
}

// MsgPackCodec is the msgpack impl of common.Codec interface
type MsgPackCodec struct{}

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
func NewMsgPackCodec() common.Codec {
	return &MsgPackCodec{}
}

// ProtobufCodec is the protobuf impl of Codec interface
type ProtobufCodec struct{}

// Marshal serialize interface @v to bytes
func (p *ProtobufCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

// Unmarshal deserialize @data to interface
func (p *ProtobufCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

// NewProtobufCodec returns new ProtobufCodec
func NewProtobufCodec() common.Codec {
	return &ProtobufCodec{}
}

// HessianCodeC is the hessian impl of Codec interface
type HessianCodeC struct{}

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
func NewHessianCodec() common.Codec {
	return &HessianCodeC{}
}

// JSONMapStructCodec is the JSON impl of Codec interface
type JSONMapStructCodec struct{}

// Marshal serialize interface @v to bytes
func (h *JSONMapStructCodec) Marshal(v interface{}) ([]byte, error) {
	if byt, err := json.Marshal(v); err != nil {
		return nil, perrors.WithStack(err)
	} else {
		return byt, nil
	}
}

// Unmarshal deserialize @data to interface
func (h *JSONMapStructCodec) Unmarshal(data []byte, v interface{}) error {
	var t map[string]interface{}
	if err := json.Unmarshal(data, &t); err != nil {
		return perrors.WithStack(err)
	}
	return hessian.ReflectResponse(v, t)
}

// NewJSONMapStruct returns new JSONMapStructCodec
func NewJSONMapStruct() common.Codec {
	return &JSONMapStructCodec{}
}

func NewGenericCodec() (common.GenericCodec, error) {
	return &GenericCodec{
		codec: NewProtobufCodec(),
	}, nil
}

// GenericCodec is pb impl of TwoWayCodec
type GenericCodec struct {
	codec common.Codec
}

// UnmarshalRequest unmarshal bytes @data to interface
func (h *GenericCodec) UnmarshalRequest(data []byte) ([]interface{}, error) {
	wrapperRequest := proto2.TripleRequestWrapper{}
	err := h.codec.Unmarshal(data, &wrapperRequest)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, 0, len(wrapperRequest.Args))

	for _, value := range wrapperRequest.Args {
		decoder := hessian.NewDecoder(value)
		val, err := decoder.Decode()
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}
