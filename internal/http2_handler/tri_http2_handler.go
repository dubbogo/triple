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

package http2_handler

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"sync"
)

import (
	perrors "github.com/pkg/errors"
	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/codec"
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

// H2Controller is used by dubbo3 client/server, to call http2
type H2Controller struct {
	// address stores target ip:port
	address string

	// pkgHandler is to convert between raw data and frame data
	pkgHandler common.PackageHandler

	// rpcServiceMap stores is user impl services
	rpcServiceMap *sync.Map

	closeChan chan struct{}

	// option is 10M by default
	option *config.Option

	twowayCodec common.TwoWayCodec

	http2Client *http2.Http2Client
}

// GetHandler is called by server when receiving tcp conn, to deal with http2 request
func (hc *H2Controller) GetHandler(rpcService interface{}) http2.Http2Handler {
	return func(path string, header http.Header, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlch chan http.Header, errCh chan interface{}) {
		/*
			triple trailer fields:
			http 2 trailers are headers fields sent after header response and body response.

			grpcMessage is used to show error message
			grpcCode is uint type and show grpc status code
			traceProtoBin is uint type, triple defined header.
		*/
		var (
			grpcMessage = ""
			grpcCode    = 0
			//traceProtoBin = 0
		)
		// new server stream
		st, err := hc.newServerStreamFromTripleHeader(path, header, rpcService)
		if st == nil || err != nil {
			hc.option.Logger.Errorf("creat server stream error = %v\n", err)
			errCh <- err
			close(sendChan)
			return
		}
		streamSendChan := st.GetSend()
		closeChan := make(chan struct{})

		// start receiving from http2 server, and forward to upper proxy invoker
		go func() {
			for {
				select {
				case <-closeChan:
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
		}()
		rspHeader := make(map[string][]string)
		rspHeader["content-type"] = []string{"application/grpc+proto"}
		rspHeader[constant.TrailerKeyGrpcStatus] = []string{"Trailer"}
		rspHeader[constant.TrailerKeyGrpcMessage] = []string{"Trailer"}
		rspHeader[constant.TrailerKeyTraceProtoBin] = []string{"Trailer"}
		ctrlch <- rspHeader
		// start receiving response from upper proxy invoker, and forward to remote http2 client

	LOOP:
		for {
			select {
			case <-hc.closeChan:
				grpcCode = int(codes.Canceled)
				grpcMessage = "triple server canceled by force" // encodeGrpcMessage(sendMsg.st.Message())
				// call finished by force
				break LOOP
			case sendMsg := <-streamSendChan:
				if sendMsg.Buffer == nil || sendMsg.MsgType != message.DataMsgType {
					if sendMsg.Status != nil {
						grpcCode = int(sendMsg.Status.Code())
						grpcMessage = sendMsg.Status.Message()
					}
					break LOOP
				}
				sendChan <- sendMsg.Buffer
			}
		}

		if grpcCode == 0 {
			close(sendChan)
		} else {
			errCh <- perrors.New(grpcMessage)
		}

		// second response header with trailer fields
		rspTrialer := make(map[string][]string)
		rspTrialer[constant.TrailerKeyGrpcStatus] = []string{strconv.Itoa(grpcCode)}
		rspTrialer[constant.TrailerKeyGrpcMessage] = []string{grpcMessage}

		// todo now if add this field, java-provider may caused unexpected error.
		//rspTrialer[constant.TripleTraceProtoBin] = []string{strconv.Itoa(traceProtoBin)}
		ctrlch <- rspTrialer

		// close all related go routines
		close(closeChan)
	}
}

// getMethodAndStreamDescMap get unary method desc map and stream method desc map from dubbo3 stub
func getMethodAndStreamDescMap(ds common.TripleGrpcService) (map[string]grpc.MethodDesc, map[string]grpc.StreamDesc, error) {
	sdMap := make(map[string]grpc.MethodDesc, 8)
	strMap := make(map[string]grpc.StreamDesc, 8)
	for _, v := range ds.ServiceDesc().Methods {
		sdMap[v.MethodName] = v
	}
	for _, v := range ds.ServiceDesc().Streams {
		strMap[v.StreamName] = v
	}
	return sdMap, strMap, nil
}

// NewH2Controller can create H2Controller with impl @rpcServiceMap and url
// @opt can be nil or configured by user
func NewH2Controller(opt *config.Option) (*H2Controller, error) {
	var pkgHandler common.PackageHandler

	pkgHandler, _ = common.GetPackagerHandler(opt)

	twowayCodec, err := codecImpl.NewTwoWayCodec(opt.CodecType)
	if err != nil {
		opt.Logger.Errorf("find serializer named %s error = %v", opt.CodecType, err)
		return nil, err
	}

	h2c := &H2Controller{
		pkgHandler:  pkgHandler,
		option:      opt,
		address:     opt.Location,
		closeChan:   make(chan struct{}),
		twowayCodec: twowayCodec,
		// todo server end, this is useless
		http2Client: http2.NewHttp2Client(config.Option{Logger: opt.Logger}),
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
func (hc *H2Controller) newServerStreamFromTripleHeader(path string, header http.Header, rpcService interface{}) (stream.Stream, error) {
	interfaceKey, methodName, err := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(path)
	if err != nil {
		return nil, err
	}

	var newstm stream.Stream
	triHeader := codec.NewTripleHeader(path, header)

	// creat server stream
	if hc.option.CodecType == constant.PBCodecName {
		service, ok := rpcService.(common.TripleGrpcService)
		if !ok {
			return nil, status.Err(codes.Internal, "can't assert impl of interface "+interfaceKey+" to TripleGrpcService")
		}
		// pb twowayCodec needs grpc.Desc to do method discovery, allowing unary and streaming invocation
		mdMap, strMap, err := getMethodAndStreamDescMap(service)
		if err != nil {
			hc.option.Logger.Error("new H2 controller error:", err)
			return nil, status.Err(codes.Unimplemented, err.Error())
		}
		unaryRPCDiscovery, okm := mdMap[methodName]
		streamRPCDiscovery, oks := strMap[methodName]
		if !okm && !oks {
			hc.option.Logger.Errorf("method name %s not found in desc\n", methodName)
			return nil, status.Err(codes.Unimplemented, "method name %s not found in desc")
		}

		if okm {
			newstm, err = stream.NewServerStream(triHeader, unaryRPCDiscovery, hc.option, service, hc.twowayCodec)
			if err != nil {
				hc.option.Logger.Error("newServerStream error", err)
				return nil, err
			}
		} else {
			newstm, err = stream.NewServerStream(triHeader, streamRPCDiscovery, hc.option, service, hc.twowayCodec)
			if err != nil {
				hc.option.Logger.Error("newServerStream error", err)
				return nil, err
			}
		}
	} else {
		service, ok := rpcService.(common.TripleUnaryService)
		if !ok {
			return nil, status.Err(codes.Internal, "can't assert impl of interface "+interfaceKey+" to TripleUnaryService")
		}
		// hessian twowayCodec doesn't need to use grpc.Desc, and now only support unary invocation
		var err error
		newstm, err = stream.NewUnaryServerStreamWithOutDesc(triHeader, hc.option, service, hc.twowayCodec, hc.option)
		if err != nil {
			hc.option.Logger.Errorf("hessian server new server stream error = %v", err)
			return nil, err
		}
	}

	return newstm, nil

}

// StreamInvoke can start streaming invocation, called by triple client, with @path
func (hc *H2Controller) StreamInvoke(ctx context.Context, path string) (grpc.ClientStream, error) {
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
		ContentType: "application/grpc+proto",
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: newHeader,
	})
	if err != nil {
		hc.option.Logger.Errorf("http2 request error = %s", err)
		// close send stream and return
		close(closeChan)
		return nil, err
	}
	go func() {
	LOOP:
		for {
			select {
			case <-hc.closeChan:
				close(closeChan)
			case data := <-dataChan:
				if data == nil {
					// stream receive done, close send go routine
					close(closeChan)
					break LOOP
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

	return stream.NewClientUserStream(clientStream, hc.twowayCodec, hc.option), nil
}

// UnaryInvoke can start unary invocation, called by dubbo3 client, with @path and request @data
func (hc *H2Controller) UnaryInvoke(ctx context.Context, path string, arg, reply interface{}) error {
	sendData, err := hc.twowayCodec.MarshalRequest(arg)
	if err != nil {
		hc.option.Logger.Errorf("client request marshal error = %v", err)
		return err
	}

	headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, ctx)
	newHeader := http.Header{}
	newHeader = headerHandler.WriteTripleReqHeaderField(newHeader)

	rspData, rspTrailerHeader, err := hc.http2Client.Post(hc.address, path, sendData, &http2Config.PostConfig{
		ContentType: constant.TripleContentType,
		BufferSize:  4096,
		Timeout:     3,
		HeaderField: newHeader,
	})
	if err != nil {
		hc.option.Logger.Errorf("triple unary invoke path %s with addr = %s error = %v", path, hc.address, err)
		return err
	}

	code, err := strconv.Atoi(rspTrailerHeader.Get(constant.TrailerKeyGrpcStatus))
	if err != nil {
		hc.option.Logger.Errorf("get trailer err = %v", err)
		return perrors.Errorf("get trailer err = %v", err)
	}
	msg := rspTrailerHeader.Get(constant.TrailerKeyGrpcMessage)

	if codes.Code(code) != codes.OK {
		hc.option.Logger.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
		return perrors.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
	}

	// all split data are collected and to unmarshal
	if err := hc.twowayCodec.UnmarshalResponse(rspData, reply); err != nil {
		hc.option.Logger.Errorf("client unmarshal rsp err= %v\n", err)
		return err
	}
	return nil
}

// Destroy destroys H2Controller and force close all related goroutine
func (hc *H2Controller) Destroy() {
	close(hc.closeChan)
}

func (hc *H2Controller) IsAvailable() bool {
	select {
	case <-hc.closeChan:
		return false
	default:
		return true
	}
	// todo check if controller's http client is available
}
