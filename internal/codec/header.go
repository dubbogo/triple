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
	"context"
	"net/http"
	"net/textproto"
	"strconv"
)

import (
	h2Triple "github.com/dubbogo/net/http2/triple"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

func init() {
	// if user choose tri as protocol, triple Handler will use it to handle header
	common.SetProtocolHeaderHandler(constant.TRIPLE, NewTripleHeaderHandler)
}

// TripleHeader stores the needed http2 header fields of triple protocol
type TripleHeader struct {
	Path           string
	ContentType    string
	ServiceVersion string
	ServiceGroup   string
	RPCID          string
	TracingID      string
	TracingRPCID   string
	TracingContext string
	ClusterInfo    string
	GrpcStatus     string
	GrpcMessage    string
	Authorization  []string
	Attachment     string
}

func NewTripleHeader(path string, header http.Header) h2Triple.ProtocolHeader {
	tripleHeader := &TripleHeader{}
	tripleHeader.Path = path
	for k, v := range header {
		switch k {
		case textproto.CanonicalMIMEHeaderKey(constant.TripleServiceVersion):
			tripleHeader.ServiceVersion = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleServiceGroup):
			tripleHeader.ServiceGroup = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleRequestID):
			tripleHeader.RPCID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceID):
			tripleHeader.TracingID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceRPCID):
			tripleHeader.TracingRPCID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceProtoBin):
			tripleHeader.TracingContext = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleUnitInfo):
			tripleHeader.ClusterInfo = v[0]
		case textproto.CanonicalMIMEHeaderKey("content-type"):
			tripleHeader.ContentType = v[0]
		case textproto.CanonicalMIMEHeaderKey("authorization"):
			tripleHeader.Authorization = v
		case textproto.CanonicalMIMEHeaderKey(constant.TripleAttachement):
			tripleHeader.Attachment = v[0]
		// todo: usage of these part of fields needs to be discussed later
		//case "grpc-encoding":
		//case "grpc-status":
		//case "grpc-message":
		default:
		}
	}
	return tripleHeader
}

func (t *TripleHeader) GetPath() string {
	return t.Path
}

// FieldToCtx parse triple Header that protocol defined, to ctx of server.
func (t *TripleHeader) FieldToCtx() context.Context {
	ctx := context.WithValue(context.Background(), constant.TripleCtxKey(constant.TripleServiceVersion), t.ServiceVersion)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleServiceGroup), t.ServiceGroup)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleRequestID), t.RPCID)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleTraceID), t.TracingID)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleTraceRPCID), t.TracingRPCID)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleTraceProtoBin), t.TracingContext)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TripleUnitInfo), t.ClusterInfo)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TrailerKeyGrpcStatus), t.GrpcStatus)
	ctx = context.WithValue(ctx, constant.TripleCtxKey(constant.TrailerKeyGrpcMessage), t.GrpcMessage)
	ctx = context.WithValue(ctx, constant.TripleCtxKey("authorization"), t.Authorization)
	ctx = context.WithValue(ctx, constant.CtxAttachmentKey, t.Attachment)
	return ctx
}

// TripleHeaderHandler is the triple imple of net.ProtocolHeaderHandler
// it handles the change of triple header field and h2 field
type TripleHeaderHandler struct {
	Opt *config.Option
	Ctx context.Context
}

// NewTripleHeaderHandler returns new TripleHeaderHandler
func NewTripleHeaderHandler(opt *config.Option, ctx context.Context) h2Triple.ProtocolHeaderHandler {
	return &TripleHeaderHandler{
		Opt: opt,
		Ctx: ctx,
	}
}

// WriteTripleReqHeaderField called before consumer calling remote,
// it parse field of opt and ctx to HTTP2 Header field, developer must assure "tri-" prefix field be string
// if not, it will cause panic!
func (t *TripleHeaderHandler) WriteTripleReqHeaderField(header http.Header) http.Header {
	// set triple user agent, to be capitabile with grpc
	header["user-agent"] = []string{constant.TripleUserAgent}

	// get from ctx
	header[constant.TripleAttachement] = []string{getCtxVaSave(t.Ctx, string(constant.CtxAttachmentKey))}
	header[constant.TripleRequestID] = []string{getCtxVaSave(t.Ctx, constant.TripleRequestID)}
	header[constant.TripleTraceID] = []string{getCtxVaSave(t.Ctx, constant.TripleTraceID)}
	header[constant.TripleTraceRPCID] = []string{getCtxVaSave(t.Ctx, constant.TripleTraceRPCID)}
	header[constant.TripleTraceProtoBin] = []string{getCtxVaSave(t.Ctx, constant.TripleTraceProtoBin)}
	header[constant.TripleUnitInfo] = []string{getCtxVaSave(t.Ctx, constant.TripleUnitInfo)}
	//header["tri-service-version"] = []string{getCtxVaSave(t.Ctx, "tri-service-version")}
	//header["tri-service-group"] = []string{getCtxVaSave(t.Ctx, "tri-service-group")}

	// get from opt
	header[constant.TripleServiceVersion] = []string{t.Opt.HeaderAppVersion}
	header[constant.TripleServiceGroup] = []string{t.Opt.HeaderGroup}

	// set authorization key
	if v, ok := t.Ctx.Value("authorization").([]string); !ok || len(v) != 2 {
		return header
	} else {
		header["authorization"] = v
	}
	return header
}

// WriteTripleFinalRspHeaderField returns trailers header fields that triple and grpc defined
func (t *TripleHeaderHandler) WriteTripleFinalRspHeaderField(w http.ResponseWriter, grpcStatusCode int, grpcMessage string, traceProtoBin int) {
	w.Header().Set(constant.TrailerKeyGrpcStatus, strconv.Itoa(grpcStatusCode)) // sendMsg.st.Code()
	w.Header().Set(constant.TrailerKeyGrpcMessage, grpcMessage)                 //encodeGrpcMessage(""))
	// todo now if add this field, java-provider may caused unexpected error.
	//w.Header().Set(TrailerKeyTraceProtoBin, strconv.Itoa(traceProtoBin)) // sendMsg.st.Code()
}

// getCtxVaSave get key @fields value and return, if not exist, return empty string
func getCtxVaSave(ctx context.Context, field string) string {
	val, ok := ctx.Value(constant.TripleCtxKey(field)).(string)
	if ok {
		return val
	}
	return ""
}

// ReadFromTripleReqHeader read meta header field from h2 header, and parse it to ProtocolHeader as developer defined
func (t *TripleHeaderHandler) ReadFromTripleReqHeader(r *http.Request) h2Triple.ProtocolHeader {
	tripleHeader := &TripleHeader{}
	header := r.Header
	tripleHeader.Path = r.URL.Path
	for k, v := range header {
		switch k {
		case textproto.CanonicalMIMEHeaderKey(constant.TripleServiceVersion):
			tripleHeader.ServiceVersion = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleServiceGroup):
			tripleHeader.ServiceGroup = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleRequestID):
			tripleHeader.RPCID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceID):
			tripleHeader.TracingID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceRPCID):
			tripleHeader.TracingRPCID = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleTraceProtoBin):
			tripleHeader.TracingContext = v[0]
		case textproto.CanonicalMIMEHeaderKey(constant.TripleUnitInfo):
			tripleHeader.ClusterInfo = v[0]
		case textproto.CanonicalMIMEHeaderKey("content-type"):
			tripleHeader.ContentType = v[0]
		case textproto.CanonicalMIMEHeaderKey("authorization"):
			tripleHeader.Authorization = v
		// todo: usage of these part of fields needs to be discussed later
		//case "grpc-encoding":
		//case "grpc-status":
		//case "grpc-message":
		default:
		}
	}
	return tripleHeader
}
