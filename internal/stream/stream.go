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

package stream

import (
	"bytes"
)

import (
	h2Triple "github.com/dubbogo/net/http2/triple"
	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/config"
)

/////////////////////////////////stream
// Stream is not only a message stream
// but an abstruct stream in h2 definition
type Stream interface {
	// channel usage
	PutRecv(data []byte, msgType message.MsgType)
	PutSend(data []byte, msgType message.MsgType)
	GetSend() <-chan message.Message
	GetRecv() <-chan message.Message
	PutSplitedDataRecv(splitedData []byte, msgType message.MsgType, handler common.PackageHandler)
	Close()
}

// baseStream ServerStream and clientStream work detail:
// in server end, when unary call, msg from client is send to recvChan, and then it is read and push to processor to get response.
// in client end, when unary call, msg from server is send to recvChan, and then response in invoke method.
/*
client  ---> send chan ---> triple ---> recv Chan ---> processor
			sendBuf						recvBuf			   |
		|	clientStream |          | serverStream |       |
			recvBuf						sendBuf			   V
client <--- recv chan <--- triple <--- send chan <---  response
*/
// baseStream is the basic  impl of stream interface, it impl for basic function of stream
type baseStream struct {
	recvBuf *message.MsgChain
	sendBuf *message.MsgChain
	service interface{}
	// splitBuffer is used to cache splited data from network, if exceed
	splitBuffer message.Message
	// fromFrameHeaderDataSize is got from dataFrame's header, which is 5 bytes and contains the total data size
	// of this package
	// when fromFrameHeaderDataSize is zero, its means we should parse header first 5byte, and then read data
	fromFrameHeaderDataSize uint32
}

// WriteCloseMsgTypeWithStatus put bufferMsg with status:  @st and type: ServerStreamCloseMsgType
func (s *baseStream) WriteCloseMsgTypeWithStatus(st *status.Status) {
	s.sendBuf.Put(message.Message{
		Status:  st,
		MsgType: message.ServerStreamCloseMsgType,
	})
}

// PutRecv put message type and @data to recvBuf
func (s *baseStream) PutRecv(data []byte, msgType message.MsgType) {
	s.recvBuf.Put(message.Message{
		Buffer:  bytes.NewBuffer(data),
		MsgType: msgType,
	})
}

// putSplitedDataRecv is called when receive from tripleNetwork, dealing with big package partial to create the whole pkg
// @msgType Must be data
func (s *baseStream) PutSplitedDataRecv(splitedData []byte, msgType message.MsgType, frameHandler common.PackageHandler) {
	if msgType != message.DataMsgType {
		return
	}
	if s.fromFrameHeaderDataSize == 0 {
		// should parse data frame header first
		var totalSize uint32
		if splitedData, totalSize = frameHandler.Frame2PkgData(splitedData); totalSize == 0 {
			return
		} else {
			s.fromFrameHeaderDataSize = totalSize
		}
		s.splitBuffer.Reset()
	}
	s.splitBuffer.Write(splitedData)
	if s.splitBuffer.Len() > int(s.fromFrameHeaderDataSize) {
		panic("Receive Splited Data is bigger than wanted!!!")
	}

	if s.splitBuffer.Len() == int(s.fromFrameHeaderDataSize) {
		s.PutRecv(frameHandler.Pkg2FrameData(s.splitBuffer.Bytes()), msgType)
		s.splitBuffer.Reset()
		s.fromFrameHeaderDataSize = 0
	}
}

// PutRecv put message type and @data to sendBuf
func (s *baseStream) PutSend(data []byte, msgType message.MsgType) {
	s.sendBuf.Put(message.Message{
		Buffer:  bytes.NewBuffer(data),
		MsgType: msgType,
	})
}

// getRecv get channel of receiving message
func (s *baseStream) GetRecv() <-chan message.Message {
	return s.recvBuf.Get()
}

// nolint
func (s *baseStream) GetSend() <-chan message.Message {
	return s.sendBuf.Get()
}

// nolint
func (s *baseStream) Close() {
	s.recvBuf.Close()
	s.sendBuf.Close()
}

func newBaseStream(service interface{}) *baseStream {
	// stream and pkgHeader are the same level
	return &baseStream{
		recvBuf: message.NewBufferMsgChain(),
		sendBuf: message.NewBufferMsgChain(),
		service: service,
		splitBuffer: message.Message{
			Buffer: bytes.NewBuffer(make([]byte, 0)),
		},
	}
}

// serverStream is running in server end
type serverStream struct {
	baseStream
	processor processor
	header    h2Triple.ProtocolHeader
}

// nolint
func (ss *serverStream) Close() {
	// close processor, as there may be rpc call that is waiting for process, let them returns canceled code
	ss.processor.close()
}

// nolint
func NewUnaryServerStreamWithOutDesc(header h2Triple.ProtocolHeader, opt *config.Option, service common.TripleUnaryService, serializer common.TwoWayCodec, option *config.Option) (*serverStream, error) {
	baseStream := newBaseStream(service)

	serverStream := &serverStream{
		baseStream: *baseStream,
		header:     header,
	}
	pkgHandler, err := common.GetPackagerHandler(opt)
	if err != nil {
		opt.Logger.Error("GetPkgHandler error with err = ", err)
		return nil, err
	}
	serverStream.processor, err = newUnaryProcessor(serverStream, pkgHandler, grpc.MethodDesc{}, serializer, option)
	if err != nil {
		opt.Logger.Errorf("new processor error with err = %s\n", err)
		return nil, err
	}

	serverStream.processor.runRPC()

	return serverStream, nil
}

// NewServerStream creates new server stream
func NewServerStream(header h2Triple.ProtocolHeader, desc interface{}, opt *config.Option, service interface{}, serializer common.TwoWayCodec) (*serverStream, error) {
	baseStream := newBaseStream(service)

	serverStream := &serverStream{
		baseStream: *baseStream,
		header:     header,
	}
	pkgHandler, err := common.GetPackagerHandler(opt)
	if err != nil {
		opt.Logger.Error("GetPkgHandler error with err = ", err)
		return nil, err
	}
	if methodDesc, ok := desc.(grpc.MethodDesc); ok {
		// pkgHandler and processor are the same level
		serverStream.processor, err = newUnaryProcessor(serverStream, pkgHandler, methodDesc, serializer, opt)
	} else if streamDesc, ok := desc.(grpc.StreamDesc); ok {
		serverStream.processor, err = newStreamingProcessor(serverStream, pkgHandler, streamDesc, serializer, opt)
	} else {
		opt.Logger.Error("grpc desc invalid:", desc)
		return nil, nil
	}
	if err != nil {
		opt.Logger.Errorf("new processor error with err = %s\n", err)
		return nil, err
	}

	serverStream.processor.runRPC()

	return serverStream, nil
}

// getService return RPCService that user defined and registered.
func (ss *serverStream) getService() interface{} {
	return ss.service
}

// getHeader returns ProtocolHeader of stream
func (ss *serverStream) getHeader() h2Triple.ProtocolHeader {
	return ss.header
}

// clientStream is running in client end
type clientStream struct {
	baseStream
}

// NewClientStream returns new client stream
func NewClientStream() *clientStream {
	baseStream := newBaseStream(nil)
	newclientStream := &clientStream{
		baseStream: *baseStream,
	}
	return newclientStream
}

// Close closes stream
func (cs *clientStream) Close() {
	cs.baseStream.Close()
}
