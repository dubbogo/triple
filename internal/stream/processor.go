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
	"errors"
	"reflect"
	"sync"
)

import (
	h2Triple "github.com/dubbogo/net/http2/triple"
	"google.golang.org/grpc"
)

import (
	"github.com/dubbogo/triple/internal/codes"
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/internal/status"
	"github.com/dubbogo/triple/internal/tools"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
)

// processor is the interface, with func runRPC and close
// it process server RPC method that user defined and get response
type processor interface {
	runRPC()
	close()
}

// baseProcessor is the basic impl of processor, which contains four base fields, such as rpc status handle function
type baseProcessor struct {
	stream      *serverStream
	twoWayCodec common.TwoWayCodec
	done        chan struct{}
	quitOnce    sync.Once
	opt         *config.Option
}

// handleRPCErr writes close message with status of given @err
func (p *baseProcessor) handleRPCErr(err error) {
	appStatus, ok := status.FromError(err)
	if !ok {
		err = status.Errorf(codes.Unknown, err.Error())
		appStatus, _ = status.FromError(err)
	}
	p.stream.WriteCloseMsgTypeWithStatus(appStatus)
}

// handleRPCSuccess sends data and grpc success code with message
func (p *baseProcessor) handleRPCSuccess(data []byte) {
	p.stream.PutSend(data, message.DataMsgType)
	p.stream.WriteCloseMsgTypeWithStatus(status.New(codes.OK, ""))
}

// close closes processor once
func (p *baseProcessor) close() {
	p.quitOnce.Do(func() {
		close(p.done)
	})
}

// unaryProcessor used to process unary invocation
type unaryProcessor struct {
	baseProcessor
	methodDesc grpc.MethodDesc
}

// newUnaryProcessor creates unary processor
func newUnaryProcessor(s *serverStream, desc grpc.MethodDesc, serializer common.TwoWayCodec, option *config.Option) (processor, error) {
	return &unaryProcessor{
		baseProcessor: baseProcessor{
			twoWayCodec: serializer,
			stream:      s,
			done:        make(chan struct{}, 1),
			quitOnce:    sync.Once{},
			opt:         option,
		},
		methodDesc: desc,
	}, nil
}

// processUnaryRPC processes unary rpc
func (p *unaryProcessor) processUnaryRPC(buf bytes.Buffer, service interface{}, header h2Triple.ProtocolHeader) ([]byte, error) {
	readBuf := buf.Bytes()

	var reply interface{}
	var err error

	_, methodName, e := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(header.GetPath())
	if e != nil {
		return nil, e
	}

	if p.opt.CodecType == constant.PBCodecName {
		descFunc := func(v interface{}) error {
			if err = p.twoWayCodec.UnmarshalRequest(readBuf, v); err != nil {
				return status.Errorf(codes.Internal, "Unary rpc request unmarshal error: %s", err)
			}
			return nil
		}
		reply, err = p.methodDesc.Handler(service, header.FieldToCtx(), descFunc, nil)
	} else {
		unaryService, ok := service.(common.TripleUnaryService)
		if !ok {
			return nil, status.Errorf(codes.Internal, "msgpack provider service doesn't impl TripleUnaryService")
		}
		reqParam, ok := unaryService.GetReqParamsInterfaces(methodName)
		if !ok {
			return nil, status.Errorf(codes.Internal, "provider unmarshal error: no req param data")
		}
		if !ok {
			return nil, status.Errorf(codes.Internal, "msgpack provider service doesn't impl TripleUnaryService")
		}
		_, methodName, e := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(header.GetPath())
		if e != nil {
			return nil, e
		}
		if err = p.twoWayCodec.UnmarshalRequest(readBuf, reqParam); err != nil {
			return nil, status.Errorf(codes.Internal, "Unary rpc request unmarshal error: %s", err)
		}
		args := make([]interface{}, 0, len(reqParam))
		for _, v := range reqParam {
			tempParamObj := reflect.ValueOf(v).Elem().Interface()
			args = append(args, tempParamObj)
		}

		reply, err = unaryService.InvokeWithArgs(header.FieldToCtx(), methodName, args)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unary rpc handle error: %s", err)
	}

	replyData, err := p.twoWayCodec.MarshalResponse(reply)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unary rpc reoly marshal error: %s", err)
	}

	return replyData, nil
}

// runRPC is called by lower layer's stream
func (p *unaryProcessor) runRPC() {
	recvChan := p.stream.GetRecv()
	go func() {
		select {
		case <-p.done:
			// in this case, server doesn't receive data but got close signal, it returns canceled code
			p.opt.Logger.Warn("unaryProcessor closed by force")
			p.handleRPCErr(status.Errorf(codes.Canceled, "processor has been canceled!"))
			return
		case recvMsg := <-recvChan:
			// in this case, server unary processor have the chance to do process and return result
			defer func() {
				if e := recover(); e != nil {
					p.handleRPCErr(errors.New(e.(error).Error()))
				}
			}()
			if recvMsg.Err != nil {
				p.opt.Logger.Error("error ,s.processUnaryRPC err = ", recvMsg.Err)
				p.handleRPCErr(status.Errorf(codes.Internal, "error ,s.processUnaryRPC err = %s", recvMsg.Err))
				return
			}
			rspData, err := p.processUnaryRPC(*recvMsg.Buffer, p.stream.getService(), p.stream.getHeader())
			if err != nil {
				p.handleRPCErr(err)
				return
			}

			// TODO: status sendResponse should has err, then writeStatus(err) use one function and defer
			// it's enough that unary processor just send data msg to stream layer
			// rpc status logic just let stream layer to handle
			p.handleRPCSuccess(rspData)
			return
		}
	}()
}

// streamingProcessor used to process streaming invocation
type streamingProcessor struct {
	baseProcessor
	streamDesc grpc.StreamDesc
}

// newStreamingProcessor can create new streaming processor
func newStreamingProcessor(s *serverStream, desc grpc.StreamDesc, serializer common.TwoWayCodec, option *config.Option) (processor, error) {
	return &streamingProcessor{
		baseProcessor: baseProcessor{
			twoWayCodec: serializer,
			stream:      s,
			done:        make(chan struct{}, 1),
			quitOnce:    sync.Once{},
			opt:         option,
		},
		streamDesc: desc,
	}, nil
}

// runRPC called by stream
func (sp *streamingProcessor) runRPC() {
	serverUserstream := newServerUserStream(sp.stream, sp.twoWayCodec, sp.opt)
	go func() {
		if err := sp.streamDesc.Handler(sp.stream.getService(), serverUserstream); err != nil {
			sp.handleRPCErr(err)
			return
		}
		// for stream rpc, processor should send CloseMsg to lower stream layer to call close
		// but unary rpc not, unary rpc processor only send data to stream layer
		sp.handleRPCSuccess(nil)
	}()
}
