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

package triple

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

import (
	h2 "github.com/dubbogo/net/http2"
	h2Triple "github.com/dubbogo/net/http2/triple"
	perrors "github.com/pkg/errors"
	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/codes"
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/internal/stream"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

// H2Controller is used by dubbo3 client/server, to call http2
type H2Controller struct {
	// client stores http2 client
	client http.Client

	// address stores target ip:port
	address string

	// pkgHandler is to convert between raw data and frame data
	pkgHandler common.PackageHandler

	// rpcServiceMap stores is user impl services
	rpcServiceMap *sync.Map

	closeChan chan struct{}

	// option is 10M by default
	option *config.Option

	serializer common.Dubbo3Serializer
}

// skipHeader is to skip first 5 byte from dataframe with header
func skipHeader(frameData []byte) ([]byte, uint32) {
	if len(frameData) < 5 {
		return []byte{}, 0
	}
	lineHeader := frameData[:5]
	length := binary.BigEndian.Uint32(lineHeader[1:])
	return frameData[5:], length
}

// readSplitData is called when client want to receive data from server
// the param @rBody is from http response. readSplitData can read from it. As data from reader is not a block of data,
// but split data stream, so there needs unpacking and merging logic with split data that receive.
func (hc *H2Controller) readSplitData(rBody io.ReadCloser) chan message.Message {
	cbm := make(chan message.Message)
	go func() {
		buf := make([]byte, hc.option.BufferSize)
		for {
			splitBuffer := message.Message{
				Buffer: bytes.NewBuffer(make([]byte, 0)),
			}

			// fromFrameHeaderDataSize is wanting data size now
			fromFrameHeaderDataSize := uint32(0)
			for {
				var n int
				var err error
				if splitBuffer.Len() < int(fromFrameHeaderDataSize) || splitBuffer.Len() == 0 {
					n, err = rBody.Read(buf)
				}

				if err != nil {
					cbm <- message.Message{
						MsgType: message.ServerStreamCloseMsgType,
					}
					return
				}
				splitedData := buf[:n]
				splitBuffer.Write(splitedData)
				if fromFrameHeaderDataSize == 0 {
					// should parse data frame header first
					data := splitBuffer.Bytes()
					var totalSize uint32
					if data, totalSize = skipHeader(data); totalSize == 0 {
						break
					} else {
						// get wanting data size from header
						fromFrameHeaderDataSize = totalSize
					}
					splitBuffer.Reset()
					splitBuffer.Write(data)
				}
				if splitBuffer.Len() >= int(fromFrameHeaderDataSize) {
					allDataBody := make([]byte, fromFrameHeaderDataSize)
					_, err := splitBuffer.Read(allDataBody)
					if err != nil {
						hc.option.Logger.Errorf("read SplitedDatas error = %v", err)
					}
					cbm <- message.Message{
						Buffer:  bytes.NewBuffer(allDataBody),
						MsgType: message.DataMsgType,
					}
					// temp data is sent, and reset wanting data size
					fromFrameHeaderDataSize = 0
				}
			}
		}
	}()
	return cbm
}

// GetHandler is called by server when receiving tcp conn, to deal with http2 request
func (hc *H2Controller) GetHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		/*
			triple trailer fields:
			http 2 trailers are headers fields sent after header response and body response.

			grpcMessage is used to show error message
			grpcCode is uint type and show grpc status code
			traceProtoBin is uint type, triple defined header.
		*/
		var (
			grpcMessage   = ""
			grpcCode      = 0
			traceProtoBin = 0
		)
		// load handler and header
		headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, context.Background())
		header := headerHandler.ReadFromTripleReqHeader(r)

		// new server stream
		st, err := hc.newServerStreamFromTripleHedaer(header)
		if st == nil || err != nil {
			hc.option.Logger.Errorf("creat server stream error = %v\n", err)
			rspErrMsg := fmt.Sprintf("creat server stream error = %v\n", err)
			w.WriteHeader(400)
			if _, err := w.Write([]byte(rspErrMsg)); err != nil {
				hc.option.Logger.Errorf("write back rsp error message %s, error", rspErrMsg)
			}
			return
			// todo handle interface/method not found error with grpc-status
		}
		sendChan := st.GetSend()
		closeChan := make(chan struct{})

		// start receiving from http2 server, and forward to upper proxy invoker
		ch := hc.readSplitData(r.Body)
		go func() {
			for {
				select {
				case <-closeChan:
					st.Close()
					return
				case msgData := <-ch:
					if msgData.MsgType == message.ServerStreamCloseMsgType {
						return
					}
					data := hc.pkgHandler.Pkg2FrameData(msgData.Bytes())
					// send to upper proxy invoker to exec
					st.PutRecv(data, message.DataMsgType)
				}
			}
		}()

		// todo  in which condition does header response not 200?
		// first response header
		w.Header().Add("Trailer", constant.TrailerKeyGrpcStatus)
		w.Header().Add("Trailer", constant.TrailerKeyGrpcMessage)
		w.Header().Add("Trailer", constant.TrailerKeyTraceProtoBin)
		w.Header().Add("content-type", "application/grpc+proto")

		// start receiving response from upper proxy invoker, and forward to remote http2 client
	LOOP:
		for {
			select {
			case <-hc.closeChan:
				grpcCode = int(codes.Canceled)
				grpcMessage = "triple server canceled by force" // encodeGrpcMessage(sendMsg.st.Message())
				// call finished by force
				break LOOP
			case sendMsg := <-sendChan:
				if sendMsg.Buffer == nil || sendMsg.MsgType != message.DataMsgType {
					if sendMsg.Status != nil {
						grpcCode = int(sendMsg.Status.Code())
						grpcMessage = sendMsg.Status.Message()
						//if sendMsg.Status.Code() != codes.OK {
						//	w.Write([]byte("close msg"))
						//}
					}
					// call finished
					break LOOP
				}
				sendData := sendMsg.Bytes()
				if _, err := w.Write(sendData); err != nil {
					hc.option.Logger.Errorf(" receiving response from upper proxy invoker error = %v", err)
				}
			}
		}

		// second response header with trailer fields
		headerHandler.WriteTripleFinalRspHeaderField(w, grpcCode, grpcMessage, traceProtoBin)

		// close all related go routines
		close(closeChan)
	}
}

// getMethodAndStreamDescMap get unary method desc map and stream method desc map from dubbo3 stub
func getMethodAndStreamDescMap(ds common.Dubbo3GrpcService) (map[string]grpc.MethodDesc, map[string]grpc.StreamDesc, error) {
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
func NewH2Controller(isServer bool, rpcServiceMap *sync.Map, opt *config.Option) (*H2Controller, error) {
	var pkgHandler common.PackageHandler

	pkgHandler, _ = common.GetPackagerHandler(opt)

	serilizer, err := common.GetDubbo3Serializer(opt)
	if err != nil {
		opt.Logger.Errorf("find serilizer named %s error = %v", opt.SerializerType, err)
		return nil, err
	}

	// new http client struct
	var client http.Client
	if !isServer {
		client = http.Client{
			Transport: &h2.Transport{
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			},
		}
	}

	h2c := &H2Controller{
		client:        client,
		rpcServiceMap: rpcServiceMap,
		pkgHandler:    pkgHandler,
		option:        opt,
		closeChan:     make(chan struct{}),
		serializer:    serilizer,
	}
	return h2c, nil
}

/*
newServerStreamFromTripleHedaer can create a serverStream by @data read from frame, after receiving a request from client.

firstly, it checks and gets calling params interfaceKey and methodName and use interfaceKey to find if there is existing service
secondly, it judge if it is streaming rpc or unary rpc
thirdly, new stream and return

any error occurs in the above procedures are fatal, as the invocation target can't be found.
todo how to deal with error in this procedure gracefully is to be discussed next
*/
func (hc *H2Controller) newServerStreamFromTripleHedaer(data h2Triple.ProtocolHeader) (stream.Stream, error) {
	interfaceKey, methodName, err := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(data.GetPath())
	if err != nil {
		return nil, err
	}

	serviceInterface, ok := hc.rpcServiceMap.Load(interfaceKey)
	if !ok {
		return nil, status.Err(codes.Unimplemented, "not found target service key"+interfaceKey)
	}

	var newstm stream.Stream

	// creat server stream
	switch hc.option.SerializerType {
	case constant.TripleHessianWrapperSerializerName:
		service, ok := serviceInterface.(common.Dubbo3HessianService)
		if !ok {
			return nil, status.Err(codes.Internal, "can't assert impl of interface "+interfaceKey+" to Dubbo3HessianService")
		}
		// hessian serializer doesn't need to use grpc.Desc, and now only support unary invocation
		var err error
		newstm, err = stream.NewUnaryServerStreamWithOutDesc(data, hc.option, service, hc.serializer, hc.option)
		if err != nil {
			hc.option.Logger.Errorf("hessian server new server stream error = %v", err)
			return nil, err
		}
	case constant.PBSerializerName:
		service, ok := serviceInterface.(common.Dubbo3GrpcService)
		if !ok {
			return nil, status.Err(codes.Internal, "can't assert impl of interface "+interfaceKey+" to Dubbo3GrpcService")
		}
		// pb serializer needs grpc.Desc to do method discovery, allowing unary and streaming invocation
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
			newstm, err = stream.NewServerStream(data, unaryRPCDiscovery, hc.option, service, hc.serializer)
			if err != nil {
				hc.option.Logger.Error("newServerStream error", err)
				return nil, err
			}
		} else {
			newstm, err = stream.NewServerStream(data, streamRPCDiscovery, hc.option, service, hc.serializer)
			if err != nil {
				hc.option.Logger.Error("newServerStream error", err)
				return nil, err
			}
		}
	default:
		hc.option.Logger.Errorf("http2 controller serializer type = %s is invalid", hc.option.SerializerType)
		return nil, perrors.Errorf("http2 controller serializer type = %s is invalid", hc.option.SerializerType)
	}

	return newstm, nil

}

// StreamInvoke can start streaming invocation, called by triple client, with @path
func (hc *H2Controller) StreamInvoke(ctx context.Context, path string) (grpc.ClientStream, error) {
	clientStream := stream.NewClientStream()

	tosend := clientStream.GetSend()
	sendStreamChan := make(chan h2Triple.BufferMsg)
	closeChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-closeChan:
				clientStream.Close()
				return
			case sendMsg := <-tosend:
				sendStreamChan <- h2Triple.BufferMsg{
					Buffer:  bytes.NewBuffer(sendMsg.Bytes()),
					MsgType: h2Triple.MsgType(sendMsg.MsgType),
				}
			}
		}
	}()
	headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, ctx)
	stremaReq := h2Triple.StreamingRequest{
		SendChan: sendStreamChan,
		Handler:  headerHandler,
	}
	go func() {
		rsp, err := hc.client.Post("https://"+hc.address+path, "application/grpc+proto", &stremaReq)
		if err != nil {
			hc.option.Logger.Errorf("http2 request error = %s", err)
			// close send stream and return
			close(closeChan)
			return
		}
		ch := hc.readSplitData(rsp.Body)
	LOOP:
		for {
			select {
			case <-hc.closeChan:
				close(closeChan)
			case data := <-ch:
				if data.Buffer == nil || data.MsgType == message.ServerStreamCloseMsgType {
					// stream receive done, close send go routine
					close(closeChan)
					break LOOP
				}
				pkg := hc.pkgHandler.Pkg2FrameData(data.Bytes())
				clientStream.PutRecv(pkg, message.DataMsgType)
			}

		}
		trailer := rsp.Body.(*h2Triple.ResponseBody).GetTrailer()
		code, _ := strconv.Atoi(trailer.Get(constant.TrailerKeyGrpcStatus))
		msg := trailer.Get(constant.TrailerKeyGrpcMessage)
		if codes.Code(code) != codes.OK {
			hc.option.Logger.Errorf("grpc status not success,msg = %s, code = %d", msg, code)
		}

	}()

	pkgHandler, err := common.GetPackagerHandler(hc.option)
	if err != nil {
		hc.option.Logger.Errorf("triple get package handler error = %v", err)
		return nil, err
	}
	return stream.NewClientUserStream(clientStream, hc.serializer, pkgHandler, hc.option), nil
}

// UnaryInvoke can start unary invocation, called by dubbo3 client, with @path and request @data
func (hc *H2Controller) UnaryInvoke(ctx context.Context, path string, arg, reply interface{}) error {
	data, err := hc.serializer.MarshalRequest(arg)
	if err != nil {
		hc.option.Logger.Errorf("client request marshal error = %v", err)
		return err
	}

	sendStreamChan := make(chan h2Triple.BufferMsg, 2)

	headerHandler, _ := common.GetProtocolHeaderHandler(hc.option, ctx)

	sendStreamChan <- h2Triple.BufferMsg{
		Buffer:  bytes.NewBuffer(hc.pkgHandler.Pkg2FrameData(data)),
		MsgType: h2Triple.MsgType(message.DataMsgType),
	}

	// send empty message with ServerStreamCloseMsgType flag to send end stream flag in h2 header
	sendStreamChan <- h2Triple.BufferMsg{
		Buffer:  bytes.NewBuffer([]byte{}),
		MsgType: h2Triple.MsgType(message.ServerStreamCloseMsgType),
	}

	stremaReq := h2Triple.StreamingRequest{
		SendChan: sendStreamChan,
		Handler:  headerHandler,
	}

	rsp, err := hc.client.Post("https://"+hc.address+path, "application/grpc+proto", &stremaReq)
	if err != nil {
		hc.option.Logger.Errorf("triple unary invoke error = %v", err)
		return err
	}

	readBuf := make([]byte, hc.option.BufferSize)

	// splitBuffer is to temporarily store collected split data, and add them together
	splitBuffer := message.Message{
		Buffer: bytes.NewBuffer(make([]byte, 0)),
	}

	timeoutTicker := time.After(time.Second * time.Duration(int(hc.option.Timeout)))
	timeoutFlag := false
	readDone := make(chan struct{})

	fromFrameHeaderDataSize := uint32(0)

	splitedDataChain := make(chan message.Message)

	go func() {
		for {
			select {
			case <-readDone:
				return
			default:
			}
			var n int
			n, err = rsp.Body.Read(readBuf)
			if err != nil {
				if err.Error() != "EOF" {
					hc.option.Logger.Errorf("dubbo3 unary invoke read error = %v\n", err)
					return
				}
				continue
			}
			splitedData := make([]byte, n)
			copy(splitedData, readBuf[:n])
			splitedDataChain <- message.Message{
				Buffer: bytes.NewBuffer(splitedData),
			}
		}
	}()

	// get trailer chan from http2
	trailerChan := rsp.Body.(*h2Triple.ResponseBody).GetTrailerChan()
	var trailer http.Header
	recvTrailer := false
LOOP:
	for {
		select {
		case dataMsg := <-splitedDataChain:
			splitedData := dataMsg.Buffer.Bytes()
			if fromFrameHeaderDataSize == 0 {
				// should parse data frame header first
				var totalSize uint32
				if splitedData, totalSize = hc.pkgHandler.Frame2PkgData(splitedData); totalSize == 0 {
					return nil
				} else {
					fromFrameHeaderDataSize = totalSize
				}
				splitBuffer.Reset()
			}
			splitBuffer.Write(splitedData)
			if splitBuffer.Len() > int(fromFrameHeaderDataSize) {
				hc.option.Logger.Error("dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
				return perrors.New("dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
			}

			if splitBuffer.Len() == int(fromFrameHeaderDataSize) {
				close(readDone)
				break LOOP
			}
		case tra := <-trailerChan:
			trailer = tra
			recvTrailer = true
			statusCode, _ := strconv.Atoi(tra.Get(constant.TrailerKeyGrpcStatus))
			if statusCode != 0 {
				break LOOP
			}

		case <-timeoutTicker:
			// close reading loop ablove
			close(readDone)
			// set timeout flag
			timeoutFlag = true
			break LOOP
		}
	}

	if timeoutFlag {
		hc.option.Logger.Errorf("unary call %s timeout", path)
		return perrors.Errorf("unary call %s timeout", path)
	}

	// todo start ticker to avoid trailer timeout
	if !recvTrailer {
		// if not receive err trailer, wait until recv
		trailer = rsp.Body.(*h2Triple.ResponseBody).GetTrailer()
	}

	code, err := strconv.Atoi(trailer.Get(constant.TrailerKeyGrpcStatus))
	if err != nil {
		hc.option.Logger.Errorf("get trailer err = %v", err)
		return perrors.Errorf("get trailer err = %v", err)
	}
	msg := trailer.Get(constant.TrailerKeyGrpcMessage)

	if codes.Code(code) != codes.OK {
		hc.option.Logger.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
		return perrors.Errorf("grpc status not success, msg = %s, code = %d", msg, code)
	}

	// all split data are collected and to unmarshal
	if err := hc.serializer.UnmarshalResponse(splitBuffer.Bytes(), reply); err != nil {
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
