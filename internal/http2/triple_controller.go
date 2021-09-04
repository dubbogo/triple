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

package http2

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/golang/protobuf/proto"
	"net/http"
	"runtime"
	"strconv"
	"sync"
)

import (
	gxsync "github.com/dubbogo/gost/sync"

	perrors "github.com/pkg/errors"

	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/codec"
	"github.com/dubbogo/triple/internal/codec/codec_impl"
	codecImpl "github.com/dubbogo/triple/internal/codec/twoway_codec_impl"
	"github.com/dubbogo/triple/internal/codes"
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/internal/stream"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/http2"
	http2Config "github.com/dubbogo/triple/pkg/http2/config"
)

// TripleController is used by dubbo3 client/server, to call http2
type TripleController struct {
	// address stores target ip:port
	address string

	// pkgHandler is to convert between raw data and frame data
	pkgHandler common.PackageHandler

	// rpcServiceMap stores is user impl services
	rpcServiceMap *sync.Map

	closeChan chan struct{}

	// option is 10M by default
	option *config.Option

	twoWayCodec common.TwoWayCodec

	genericCodec common.GenericCodec

	http2Client *http2.Client

	pool gxsync.WorkerPool
}

// GetHandler is called by server when receiving tcp conn, to deal with http2 request
func (hc *TripleController) GetHandler(rpcService interface{}) http2.Handler {
	return func(path string, header http.Header, recvChan chan *bytes.Buffer,
		sendChan chan *bytes.Buffer, ctrlch chan http.Header,
		errCh chan interface{}) {
		/*
			triple trailer fields:
			http 2 trailers are headers fields sent after header response and body response.

			grpcMessage is used to show error message
			grpcCode is uint type and show grpc status code
			traceProtoBin is uint type, triple defined header.
		*/

		hc.option.Logger.Debugf("receive http2 path = %s", path)

		if err := hc.pool.Submit(func() {
			var (
				tripleStatus  *status.Status
				rspAttachment = make(common.TripleAttachment)
			)

			rspHeader := make(map[string][]string)
			rspHeader["content-type"] = []string{constant.TripleContentType}
			ctrlch <- rspHeader

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// new server stream
			st, err := hc.newServerStreamFromTripleHeader(ctx, path, header, rpcService, hc.pool)
			if st == nil || err != nil {
				hc.option.Logger.Errorf("creat server stream error = %v\n", err)
				tripleStatus, _ = status.FromError(err)
				close(sendChan)
				hc.handleStatusAttachmentAndResponse(tripleStatus, nil, ctrlch)
				return
			}

			streamSendChan := st.GetSend()
			closeSendChan := make(chan struct{})

			// start receiving from http2 server, and forward to upper proxy invoker
			sendToStream := func() {
				for {
					select {
					case <-closeSendChan:
						st.Close()
						return
					case msgData := <-recvChan:
						if msgData != nil {
							st.PutRecv(msgData.Bytes(), message.DataMsgType)
							continue
						}
						return
					}
				}
			}
			if err := hc.pool.Submit(sendToStream); err != nil {
				close(sendChan)
				hc.option.Logger.Warnf("go routine pool full with error = %v", err)
				hc.handleStatusAttachmentAndResponse(status.New(codes.ResourceExhausted, fmt.Sprintf("go routine pool full with error = %v", err)), nil, ctrlch)
				return
			}

		Loop:
			for {
				select {
				case <-hc.closeChan:
					tripleStatus = status.New(codes.Canceled, "triple server canceled by force")
					// call finished by force
					break Loop
				case sendMsg := <-streamSendChan:
					if sendMsg.Buffer == nil || sendMsg.MsgType != message.DataMsgType {
						if sendMsg.Status != nil {
							tripleStatus = status.FromProto(sendMsg.Status.Proto())
						}
						break Loop
					}
					rspAttachment = sendMsg.Attachment
					sendChan <- sendMsg.Buffer
				}
			}
			close(sendChan)

			hc.handleStatusAttachmentAndResponse(tripleStatus, rspAttachment, ctrlch)
			// close all related go routines
			close(closeSendChan)

		}); err != nil {
			// failed to occupy worker, return error code
			go func() {
				rspHeader := make(map[string][]string)
				rspHeader["content-type"] = []string{constant.TripleContentType}
				ctrlch <- rspHeader
				close(sendChan)
				hc.option.Logger.Warnf("go routine pool full with error = %v", err)
				hc.handleStatusAttachmentAndResponse(status.New(codes.ResourceExhausted, fmt.Sprintf("go routine pool full with error = %v", err)), nil, ctrlch)
			}()
		}
	}
}

func (hc *TripleController) handleStatusAttachmentAndResponse(tripleStatus *status.Status, attachment map[string]string, ctrlch chan http.Header) {
	// second response header with trailer fields
	rspTrialer := make(map[string][]string)
	rspTrialer[constant.TrailerKeyGrpcStatus] = []string{strconv.Itoa(int(tripleStatus.Code()))} //[]string{strconv.Itoa(int(tripleStatus.Code()))}
	rspTrialer[constant.TrailerKeyGrpcMessage] = []string{tripleStatus.Message()}
	if attachment != nil {
		for k, v := range attachment {
			rspTrialer[k] = []string{v}
		}
	}
	statusProto := tripleStatus.Proto()
	if statusProto != nil {
		if stBytes, err := proto.Marshal(statusProto); err != nil {
			hc.option.Logger.Errorf("transport: failed to marshal rpc status: %v, error: %v", statusProto, err)
		} else {
			rspTrialer[constant.TrailerKeyGrpcDetailsBin] = []string{
				base64.RawStdEncoding.EncodeToString(stBytes)}
		}
	}

	// todo now if add this field, java-provider may caused unexpected error.
	//rspTrialer[constant.TripleTraceProtoBin] = []string{strconv.Itoa(traceProtoBin)}

	ctrlch <- rspTrialer
}

// getMethodAndStreamDescMap get unary method desc map and stream method desc map from dubbo3 stub
func getMethodAndStreamDescMap(ds common.TripleGrpcService) (map[string]grpc.MethodDesc, map[string]grpc.StreamDesc, error) {
	sdMap := make(map[string]grpc.MethodDesc, len(ds.ServiceDesc().Methods))
	strMap := make(map[string]grpc.StreamDesc, len(ds.ServiceDesc().Streams))
	for _, v := range ds.ServiceDesc().Methods {
		sdMap[v.MethodName] = v
	}
	for _, v := range ds.ServiceDesc().Streams {
		strMap[v.StreamName] = v
	}
	return sdMap, strMap, nil
}

// NewTripleController can create TripleController with impl @rpcServiceMap and url
// @opt can be nil or configured by user
func NewTripleController(opt *config.Option) (*TripleController, error) {
	var pkgHandler common.PackageHandler

	pkgHandler, _ = common.GetPackagerHandler(opt)

	twowayCodec, err := codecImpl.NewTwoWayCodec(opt.CodecType)
	if err != nil {
		opt.Logger.Errorf("find serializer named %s error = %v", opt.CodecType, err)
		return nil, err
	}

	genericCodec, _ := codec_impl.NewGenericCodec()

	h2c := &TripleController{
		pkgHandler:   pkgHandler,
		option:       opt,
		address:      opt.Location,
		closeChan:    make(chan struct{}),
		twoWayCodec:  twowayCodec,
		genericCodec: genericCodec,
		// todo server end, this is useless
		http2Client: http2.NewClient(config.Option{Logger: opt.Logger}),
		pool: gxsync.NewConnectionPool(gxsync.WorkerPoolConfig{
			NumWorkers: int(opt.NumWorkers),
			NumQueues:  runtime.NumCPU(),
			QueueSize:  0,
			Logger:     opt.Logger,
		}),
	}
	return h2c, nil
}

/*
newServerStreamFromTripleHeader can create a serverStream by @data read from frame, after receiving a request from client.

firstly, it checks and gets calling params interfaceKey and methodName and use interfaceKey to find if there is existing service
secondly, it judge if it is streaming rpc or unary rpc
thirdly, new stream and return

any error occurs in the above procedures are fatal, as the invocation target can't be found.
todo how to deal with error in this procedure gracefully is to be discussed next
*/
func (hc *TripleController) newServerStreamFromTripleHeader(ctx context.Context, path string, header http.Header,
	rpcService interface{}, pool gxsync.WorkerPool) (stream.Stream, error) {
	interfaceKey, methodName, err := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(path)
	if err != nil {
		return nil, err
	}

	var newStream stream.Stream
	triHeader := codec.NewTripleHeader(path, header)

	// creat server stream
	if hc.option.CodecType == constant.PBCodecName {
		service, ok := rpcService.(common.TripleGrpcService)
		if !ok {
			return nil, status.Err(codes.Internal, "can't assert impl of interface "+interfaceKey+" to TripleGrpcService")
		}
		// pb twoWayCodec needs grpc.Desc to do method discovery, allowing unary and streaming invocation
		methodMap, streamMap, err := getMethodAndStreamDescMap(service)
		if err != nil {
			hc.option.Logger.Error("new H2 controller error:", err)
			return nil, status.Err(codes.Unimplemented, err.Error())
		}
		unaryRPCDiscovery, unaryOk := methodMap[methodName]
		streamRPCDiscovery, streamOk := streamMap[methodName]

		if unaryOk {
			newStream, err = stream.NewServerStreamForPB(ctx, triHeader, unaryRPCDiscovery, hc.option,
				pool, service, hc.twoWayCodec)
			if err != nil {
				hc.option.Logger.Errorf("newServerStream error = %v", err)
				return nil, err
			}
		} else if streamOk {
			newStream, err = stream.NewServerStreamForPB(ctx, triHeader, streamRPCDiscovery, hc.option,
				pool, service, hc.twoWayCodec)
			if err != nil {
				hc.option.Logger.Errorf("newServerStream error = %v", err)
				return nil, err
			}
		} else {
			hc.option.Logger.Errorf("method name %s not found in desc\n", methodName)
			return nil, status.Errorf(codes.Unimplemented, "method name %s not found in desc", methodName)
		}

	} else {
		service, ok := rpcService.(common.TripleUnaryService)
		if !ok {
			hc.option.Logger.Errorf("can't assert impl of interface %s service %+v to TripleUnaryService", interfaceKey, rpcService)
			return nil, status.Errorf(codes.Internal, "can't assert impl of interface %s service %+v to TripleUnaryService", interfaceKey, rpcService)
		}
		// unary service doesn't need to use grpc.Desc, and now only support unary invocation
		var err error
		newStream, err = stream.NewServerStreamForNonPB(ctx, triHeader, hc.option, pool, service, hc.twoWayCodec, hc.genericCodec)
		if err != nil {
			hc.option.Logger.Errorf("unary service new server stream error = %v", err)
			return nil, err
		}
	}

	return newStream, nil

}

// StreamInvoke can start streaming invocation, called by triple client, with @path
func (hc *TripleController) StreamInvoke(ctx context.Context, path string) (grpc.ClientStream, error) {
	clientStream := stream.NewClientStream()
	tosend := clientStream.GetSend()
	sendStreamChan := make(chan *bytes.Buffer)
	closeChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-closeChan:
				clientStream.Close()
				return
			case sendMsg := <-tosend:
				if sendMsg.MsgType == message.ServerStreamCloseMsgType {
					return
				}
				sendStreamChan <- bytes.NewBuffer(sendMsg.Bytes())
			}
		}
	}()
	headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, ctx)
	newHeader := headerHandler.WriteTripleReqHeaderField(http.Header{})
	dataChan, rspHeaderChan, err := hc.http2Client.StreamPost(hc.address, path, sendStreamChan, &http2Config.PostConfig{
		ContentType: constant.TripleContentType,
		BufferSize:  hc.option.BufferSize,
		Timeout:     hc.option.Timeout,
		HeaderField: newHeader,
	})
	if err != nil {
		hc.option.Logger.Errorf("http2 request error = %s", err)
		// close send stream and return
		close(closeChan)
		return nil, err
	}
	go func() {
	Loop:
		for {
			select {
			case <-hc.closeChan:
				close(closeChan)
			case data := <-dataChan:
				if data == nil {
					// stream receive done, close send go routine
					close(closeChan)
					break Loop
				}
				clientStream.PutRecv(data.Bytes(), message.DataMsgType)
			}
		}
		trailer := <-rspHeaderChan
		code, _ := strconv.Atoi(trailer.Get(constant.TrailerKeyGrpcStatus))
		msg := trailer.Get(constant.TrailerKeyGrpcMessage)
		if codes.Code(code) != codes.OK {
			hc.option.Logger.Errorf("grpc status not success,msg = %s, code = %d", msg, code)
		}
	}()

	return stream.NewClientUserStream(clientStream, hc.twoWayCodec, hc.option), nil
}

// UnaryInvoke can start unary invocation, called by dubbo3 client, with @path and request @data
func (hc *TripleController) UnaryInvoke(ctx context.Context, path string, arg, reply interface{}) (common.TripleAttachment, error) {
	var code int
	var msg string
	var attachment = make(common.TripleAttachment)

	sendData, err := hc.twoWayCodec.MarshalRequest(arg)
	if err != nil {
		hc.option.Logger.Errorf("client request marshal error = %v", err)
		return attachment, err
	}

	headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, ctx)
	newHeader := http.Header{}
	newHeader = headerHandler.WriteTripleReqHeaderField(newHeader)

	rspData, rspTrailerHeader, err := hc.http2Client.Post(hc.address, path, sendData, &http2Config.PostConfig{
		ContentType: constant.TripleContentType,
		BufferSize:  hc.option.BufferSize,
		Timeout:     hc.option.Timeout,
		HeaderField: newHeader,
	})
	if err != nil {
		hc.option.Logger.Error("triple unary invoke path" + path + " with addr = " + hc.address + " error = " + err.Error())
		return attachment, err
	}

	for k, v := range rspTrailerHeader {
		if len(v) == 0 {
			continue
		}
		switch k {
		case constant.TrailerKeyGrpcStatus:
			code, err = strconv.Atoi(v[0])
			if err != nil {
				hc.option.Logger.Errorf("get trailer err = %v", err)
				return attachment, perrors.Errorf("get trailer err = %v", err)
			}
		case constant.TrailerKeyGrpcMessage:
			msg = rspTrailerHeader.Get(v[0])
		default:
			attachment[k] = v[0]
		}
	}

	if codes.Code(code) != codes.OK {
		hc.option.Logger.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
		return attachment, perrors.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
	}

	// all split data are collected and to unmarshal
	if err := hc.twoWayCodec.UnmarshalResponse(rspData, reply); err != nil {
		hc.option.Logger.Errorf("client unmarshal rsp err= %v\n", err)
		return attachment, err
	}
	return attachment, nil
}

// Destroy destroys TripleController and force close all related goroutine
func (hc *TripleController) Destroy() {
	close(hc.closeChan)
}

func (hc *TripleController) IsAvailable() bool {
	select {
	case <-hc.closeChan:
		return false
	default:
		return true
	}
	// todo check if controller's http client is available
}
