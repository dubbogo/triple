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
	hessian "github.com/apache/dubbo-go-hessian2"

	"github.com/golang/protobuf/proto"

	mp "github.com/ugorji/go/codec"
)

import (
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
)

func init() {
	common.SetTripleCodec(constant.PBCodecName, NewProtobufCodec)
	common.SetTripleCodec(constant.HessianCodecName, NewHessianCodec)
	common.SetTripleCodec(constant.MsgPackCodecName, NewMsgPackCodec)
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
