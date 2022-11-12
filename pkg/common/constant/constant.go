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

package constant

import "time"

// transfer
const (
	// TRIPLE is triple protocol name
	TRIPLE = "tri"

	// DefaultHttp2ControllerReadBufferSize is default read buffer size of triple client/server
	DefaultHttp2ControllerReadBufferSize = 4096

	// DefaultTimeout is default timeout seconds of triple client
	DefaultTimeout = time.Second * 3

	// DefaultListeningAddress is default listening address
	DefaultListeningAddress = "127.0.0.1:20001"
)

// CodecType is the type of triple serializer
type CodecType string

const (
	// PBCodecName is the default serializer name, triple use pb as serializer.
	PBCodecName = CodecType("protobuf")

	// HessianCodecName is the serializer with pb wrapped with hessian2
	HessianCodecName = CodecType("hessian2")

	// MsgPackCodecName is the serializer with pb wrapped with msgpack
	MsgPackCodecName = CodecType("msgpack")

	// JSONMapStructCodec is the serializer jsonCodec
	JSONMapStructCodec = CodecType("jsonMapStruct")
)

// TripleCtxKey is typ of content key
type TripleCtxKey string

const (
	InterfaceKey     = TripleCtxKey("interface")
	CtxAttachmentKey = TripleCtxKey("attachment")
	TrailerKey       = "Trailer"
)

// triple Header

// TrailerKeys are to make triple compatible with grpc
// After server returning the rsp header and body, it returns Trailer header in the end, to send grpc status of this invocation.
const (
	// TrailerKeyGrpcStatus is a trailer header field to response grpc code (int).
	TrailerKeyGrpcStatus = "grpc-status"

	// TrailerKeyGrpcMessage is a trailer header field to response grpc error message.
	TrailerKeyGrpcMessage = "grpc-message"

	// TrailerKeyGrpcDetailsBin is a trailer header field to response grpc details bin message encoded by base64
	TrailerKeyGrpcDetailsBin = "grpc-status-details-bin"

	// TrailerKeyTraceProtoBin is triple trailer header
	TrailerKeyTraceProtoBin = "trace-proto-bin"

	// TrailerKeyHttp2Status is http2 pkg trailer key of success
	TrailerKeyHttp2Status = "http2-status"

	// TrailerKeyHttp2Message is http2 pkg trailer key of error message
	TrailerKeyHttp2Message = "http2-message"
)

// Header keys are header field key from client
const (
	TripleContentType    = "application/grpc+proto"
	TripleUserAgent      = "grpc-go/1.35.0-dev"
	TripleServiceVersion = "tri-service-version"
	TripleAttachement    = "tri-attachment"
	TripleServiceGroup   = "tri-service-group"
	TripleRequestID      = "tri-req-id"
	TripleTraceID        = "tri-trace-traceid"
	TripleTraceRPCID     = "tri-trace-rpcid"
	TripleTraceProtoBin  = "tri-trace-proto-bin"
	TripleUnitInfo       = "tri-unit-info"
)

// gr pool
const (
	// DefaultNumWorkers #workers for connection pool
	DefaultNumWorkers = 720
)

// proxy interface
const (
	ProxyServiceKey = "github.com.dubbogo.triple.proxy"
)
