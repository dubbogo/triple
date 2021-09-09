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
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

import (
	gxsync "github.com/dubbogo/gost/sync"

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
// It processes server RPC method that user defined and get response
type processor interface {
	runRPC(ctx context.Context) error
	close()
}

// baseProcessor is the basic impl of processor, which contains four base fields, such as rpc status handle function
type baseProcessor struct {
	stream       *serverStream
	twoWayCodec  common.TwoWayCodec
	genericCodec common.GenericCodec
	done         chan struct{}
	quitOnce     sync.Once
	pool         gxsync.WorkerPool
	opt          *config.Option
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
func (p *baseProcessor) handleRPCSuccess(data []byte, attachment map[string]string) {
	p.stream.PutSend(data, attachment, message.DataMsgType)
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
func newUnaryProcessor(s *serverStream, desc grpc.MethodDesc, serializer common.TwoWayCodec, genericCodec common.GenericCodec,
	pool gxsync.WorkerPool, option *config.Option) processor {
	return &unaryProcessor{
		baseProcessor: baseProcessor{
			twoWayCodec:  serializer,
			genericCodec: genericCodec,
			stream:       s,
			done:         make(chan struct{}, 1),
			quitOnce:     sync.Once{},
			pool:         pool,
			opt:          option,
		},
		methodDesc: desc,
	}
}

// processUnaryRPC processes unary rpc
func (p *unaryProcessor) processUnaryRPC(buf bytes.Buffer, service interface{}, header h2Triple.ProtocolHeader) ([]byte, common.ErrorWithAttachment) {
	readBuf := buf.Bytes()
	p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: with readBuffer to be unmarshal = %s, header = %+v, server defined serialization type = %s", string(readBuf), header, p.opt.CodecType)

	var rawReplyStruct interface{}
	var reply interface{}
	var err error
	responseAttachment := make(common.TripleAttachment)

	_, methodName, e := tools.GetServiceKeyAndUpperCaseMethodNameFromPath(header.GetPath())
	if e != nil {
		p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: invalid http2 path = %s, error = %s", header.GetPath(), e.Error())
		return nil, *common.NewErrorWithAttachment(e, nil)
	}
	p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: get parsed golang methodName = %s", methodName)
	if p.opt.CodecType == constant.PBCodecName {
		descFunc := func(v interface{}) error {
			if err = p.twoWayCodec.UnmarshalRequest(readBuf, v); err != nil {
				p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: Unary rpc request unmarshal error: %s", err)
				return status.Errorf(codes.Internal, "Unary rpc request unmarshal error: %s", err)
			}
			return nil
		}
		p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: unary invoke pb service method %s with header %+v", methodName, header)
		reply, err = p.methodDesc.Handler(service, header.FieldToCtx(), descFunc, nil)
	} else {
		unaryService, ok := service.(common.TripleUnaryService)
		if !ok {
			p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: msgpack provider service %+v doesn't impl TripleUnaryService", service)
			return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "msgpack provider service %+v doesn't impl TripleUnaryService", service), responseAttachment)
		}

		if methodName == "$invoke" {
			var args []interface{}
			args, err = p.genericCodec.UnmarshalRequest(readBuf)
			if err != nil {
				p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: generic invoke with request %s unmarshal error = %s", string(readBuf), err.Error())
				return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "generic invoke with request %s unmarshal error = %s", string(readBuf), err.Error()), responseAttachment)
			}
			p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: generic invoke service with header %+v and args %v", header, args)
			reply, err = unaryService.InvokeWithArgs(header.FieldToCtx(), methodName, args)
		} else {
			reqParam, ok := unaryService.GetReqParamsInterfaces(methodName)
			if !ok {
				p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: method name %s is not provided by service, please check if correct", methodName)
				return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Unimplemented, "method name %s is not provided by service, please check if correct", methodName), responseAttachment)
			}
			// get args from buf
			if err = p.twoWayCodec.UnmarshalRequest(readBuf, reqParam); err != nil {
				p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: Unary rpc request unmarshal error: %s", err)
				return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "Unary rpc request unmarshal error: %s", err), responseAttachment)
			}
			args := make([]interface{}, 0, len(reqParam))
			for _, v := range reqParam {
				tempParamObj := reflect.ValueOf(v).Elem().Interface()
				args = append(args, tempParamObj)
			}
			// invoke the service
			p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: unary invoke service method %s with header %+v and args %+v", methodName, header, args)
			reply, err = unaryService.InvokeWithArgs(header.FieldToCtx(), methodName, args)
		}
	}

	if err != nil {
		p.opt.Logger.Errorf("Unary rpc process error: %s, the error may be returned by user", err)
		return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "Unary rpc process error: %s,  the error may be returned by user", err), responseAttachment)
	}

	if result, ok := reply.(common.OuterResult); ok {
		// proceess header trailer
		outerAttachment := result.Attachments()
		p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: get outerAttachment = %+v", outerAttachment)
		for k, v := range outerAttachment {
			if str, ok := v.(string); ok {
				responseAttachment[k] = str
			}
		}
		p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: get triple attachment = %+v", responseAttachment)
		rawReplyStruct = result.Result()
		p.opt.Logger.Debugf("unaryProcessor.processUnaryRPC: get reply %+v to be marshal", rawReplyStruct)
	} else {
		p.opt.Logger.Debug("unaryProcessor.processUnaryRPC: DEPRECATED! reply from service not impl common.OuterResult")
		rawReplyStruct = reply
	}
	p.opt.Logger.Debugf("get result rawReplyStruct = %+v", rawReplyStruct)
	replyData, err := p.twoWayCodec.MarshalResponse(rawReplyStruct)
	if err != nil {
		p.opt.Logger.Errorf("unaryProcessor.processUnaryRPC: Unary rpc reply marshal error: %s", err)
		return nil, *common.NewErrorWithAttachment(status.Errorf(codes.Internal, "Unary rpc reply marshal error: %s", err), responseAttachment)
	}

	return replyData, *common.NewErrorWithAttachment(nil, responseAttachment)
}

// runRPC is called by lower layer's stream
func (p *unaryProcessor) runRPC(ctx context.Context) error {
	recvChan := p.stream.GetRecv()
	if perr := p.pool.Submit(func() {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			// in this case, server doesn't receive data but got close signal, it returns canceled code
			p.opt.Logger.Warn("unaryProcessor:runRPC: unaryProcessor closed by force")
			p.handleRPCErr(status.Errorf(codes.Canceled, "processor has been canceled!"))
			return
		case recvMsg := <-recvChan:
			// in this case, server unary processor have the chance to do process and return result
			defer func() {
				if e := recover(); e != nil {
					p.opt.Logger.Errorf("unaryProcessor:runRPC: when running unary process, cache error = %v", e)
					p.handleRPCErr(errors.New(fmt.Sprintf("%v", e)))
				}
			}()
			if recvMsg.Err != nil {
				p.opt.Logger.Errorf("unaryProcessor:runRPC: unary processor receive message from http2 error = %s", recvMsg.Err)
				p.handleRPCErr(status.Errorf(codes.Internal, "unary processor receive message from http2 error = %s", recvMsg.Err))
				return
			}
			rspData, errWithAttachment := p.processUnaryRPC(*recvMsg.Buffer, p.stream.getService(), p.stream.getHeader())
			if err := errWithAttachment.GetError(); err != nil {
				p.opt.Logger.Errorf("unaryProcessor:runRPC: process unary rpc with header = %+v, data = %s,  error = %s", p.stream.getHeader(), recvMsg.Buffer.String(), err)
				p.handleRPCErr(err)
				return
			}

			// TODO: status sendResponse should has err, then writeStatus(err) use one function and defer
			// it's enough that unary processor just send data msg to stream layer
			// rpc status logic just let stream layer to handle
			p.handleRPCSuccess(rspData, errWithAttachment.GetAttachments())
			return
		}
	}); perr != nil {
		p.opt.Logger.Warnf("unaryProcessor:runRPC: go routine pool full with error = %v", perr)
		return status.Errorf(codes.ResourceExhausted, "go routine pool full with error = %v", perr)
	}
	return nil
}

// streamingProcessor used to process streaming invocation
type streamingProcessor struct {
	baseProcessor
	streamDesc grpc.StreamDesc
}

// newStreamingProcessor can create new streaming processor
func newStreamingProcessor(s *serverStream, desc grpc.StreamDesc, serializer common.TwoWayCodec,
	pool gxsync.WorkerPool, option *config.Option) processor {
	return &streamingProcessor{
		baseProcessor: baseProcessor{
			twoWayCodec: serializer,
			stream:      s,
			done:        make(chan struct{}, 1),
			quitOnce:    sync.Once{},
			pool:        pool,
			opt:         option,
		},
		streamDesc: desc,
	}
}

// runRPC called by stream
func (sp *streamingProcessor) runRPC(ctx context.Context) error {
	serverUserStream := newServerUserStream(sp.stream, sp.twoWayCodec, sp.opt)

	if perr := sp.pool.Submit(func() {
		if err := sp.streamDesc.Handler(sp.stream.getService(), serverUserStream); err != nil {
			sp.opt.Logger.Errorf("streamingProcessor.runRPC: stream processor handle streaming request with service %+v with error = %s", sp.stream.getService(), err)
			sp.handleRPCErr(status.Errorf(codes.Internal, "stream processor handle streaming request with service %+v with error = %s", sp.stream.getService(), err))
			return
		}
		// for stream rpc, processor should send CloseMsg to lower stream layer to call close
		// but unary rpc not, unary rpc processor only send data to stream layer
		sp.handleRPCSuccess(nil, nil)
	}); perr != nil {
		sp.opt.Logger.Warnf("streamingProcessor.runRPC: go routine pool full with error = %v", perr)
		return status.Errorf(codes.ResourceExhausted, "go routine pool full with error = %v", perr)
	}
	return nil
}
