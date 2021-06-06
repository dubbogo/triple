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

package common

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

import (
	netTriple "github.com/dubbogo/net/http2/triple"
	"gotest.tools/assert"
)

import (
	"github.com/dubbogo/triple/pkg/config"
)

type ImplProtocolHeader struct {
	Method   string
	StreamID uint32
}

func (t *ImplProtocolHeader) GetPath() string {
	return t.Method
}
func (t *ImplProtocolHeader) GetStreamID() uint32 {
	return t.StreamID
}

// FieldToCtx parse triple Header that user defined, to ctx of server end
func (t *ImplProtocolHeader) FieldToCtx() context.Context {
	return context.Background()
}

type ImplProtocolHeaderHandler struct {
}

func (ihh *ImplProtocolHeaderHandler) ReadFromTripleReqHeader(header *http.Request) netTriple.ProtocolHeader {
	return &ImplProtocolHeader{}
}

func (hh *ImplProtocolHeaderHandler) WriteTripleReqHeaderField(header http.Header) http.Header {
	return nil
}

func (hh *ImplProtocolHeaderHandler) WriteTripleFinalRspHeaderField(w http.ResponseWriter, grpcStatusCode int, grpcMessage string, traceProtoBin int) {

}

func NewTestHeaderHandler(opt *config.Option, ctx context.Context) netTriple.ProtocolHeaderHandler {
	return &ImplProtocolHeaderHandler{}
}

func TestSetAndGetProtocolHeaderHandler(t *testing.T) {
	oriHandler := NewTestHeaderHandler(nil, context.Background())
	SetProtocolHeaderHandler("test-protocol", NewTestHeaderHandler)
	handler, err := GetProtocolHeaderHandler(config.NewTripleOption(config.WithProtocol("test-protocol")), context.Background())
	assert.Equal(t, err, nil)
	assert.Equal(t, reflect.TypeOf(handler), reflect.TypeOf(oriHandler))
}

type TestTriplePackageHandler struct {
}

func (t *TestTriplePackageHandler) Frame2PkgData(frameData []byte) ([]byte, uint32) {
	return frameData, 0
}
func (t *TestTriplePackageHandler) Pkg2FrameData(pkgData []byte) []byte {
	return pkgData
}

func newTestTriplePackageHandler() PackageHandler {
	return &TestTriplePackageHandler{}
}

func TestSetAndGetGetPackagerHandler(t *testing.T) {
	oriHandler := newTestTriplePackageHandler()
	SetPackageHandler("test-protocol", newTestTriplePackageHandler)
	opt := config.NewTripleOption(config.WithProtocol("test-protocol"))
	opt.Validate()
	handler, err := GetPackagerHandler(opt)
	assert.Equal(t, err, nil)
	assert.Equal(t, reflect.TypeOf(handler), reflect.TypeOf(oriHandler))
}

type TestDubbo3Serializer struct {
}

func (p *TestDubbo3Serializer) Marshal(v interface{}) ([]byte, error) {
	return []byte{}, nil
}
func (p *TestDubbo3Serializer) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func newTestDubbo3Serializer() Codec {
	return &TestDubbo3Serializer{}
}

func TestGetAndSetSerilizer(t *testing.T) {
	oriSerializer := newTestDubbo3Serializer()
	SetTripleCodec("test-protocol", newTestDubbo3Serializer)
	opt := config.NewTripleOption(config.WithCodecType("test-protocol"))
	opt.Validate()
	ser, err := GetTripleCodec(opt.CodecType)
	assert.Equal(t, err, nil)
	assert.Equal(t, reflect.TypeOf(ser), reflect.TypeOf(oriSerializer))
}
