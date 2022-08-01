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
	"context"
	"reflect"
	"sync"
)

import (
	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/codes"
	"github.com/dubbogo/grpc-go/encoding"
	"github.com/dubbogo/grpc-go/status"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/encoding"
	"github.com/dubbogo/triple/pkg/common/encoding/hessian"
	"github.com/dubbogo/triple/pkg/common/encoding/javaType"
	"github.com/dubbogo/triple/pkg/common/encoding/msgpack"
	"github.com/dubbogo/triple/pkg/common/encoding/proto_wrapper_api"
	"github.com/dubbogo/triple/pkg/common/encoding/raw_proto"
	"github.com/dubbogo/triple/pkg/common/encoding/tools"
	"github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/tracing"
)

// TripleClient client endpoint that using triple protocol
type TripleClient struct {
	stubInvoker reflect.Value

	triplConn *TripleConn

	//once is used when destroy
	once sync.Once

	// triple config
	opt *config.Option

	// serializer is triple serializer to do codec
	serializer encoding.Codec
}

// NewTripleClient creates triple client
// it returns tripleClient, which contains invoker and triple connection.
// @impl must have method: GetDubboStub(cc *dubbo3.TripleConn) interface{}, to be capable with grpc
// @opt is used to init http2 controller, if it's nil, use the default config
func NewTripleClient(impl interface{}, opt *config.Option) (*TripleClient, error) {
	if opt == nil {
		opt = config.NewTripleOption()
	}
	tripleClient := &TripleClient{
		opt: opt,
	}
	dialOpts := []grpc.DialOption{}
	if opt.JaegerAddress != "" {
		var tracer opentracing.Tracer
		if !opt.JaegerUseAgent {
			tracer = tracing.NewJaegerTracerDirect(opt.JaegerServiceName, opt.JaegerAddress, opt.Logger)
		} else {
			tracer = tracing.NewJaegerTracerAgent(opt.JaegerServiceName, opt.JaegerAddress, opt.Logger)
		}

		dialOpts = append(dialOpts,
			grpc.WithUnaryInterceptor(tracing.OpenTracingClientInterceptor(tracer)),
			grpc.WithStreamInterceptor(tracing.OpenTracingStreamClientInterceptor(tracer)),
		)
	}

	defaultCallOpts := make([]grpc.CallOption, 0)
	// max send/receive size
	if opt.GRPCMaxCallSendMsgSize != 0 {
		defaultCallOpts = append(defaultCallOpts, grpc.MaxCallSendMsgSize(opt.GRPCMaxCallSendMsgSize))
	}
	if opt.GRPCMaxCallRecvMsgSize != 0 {
		defaultCallOpts = append(defaultCallOpts, grpc.MaxCallRecvMsgSize(opt.GRPCMaxCallRecvMsgSize))
	}
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(defaultCallOpts...))

	// codec
	if opt.CodecType == constant.PBCodecName {
		// put dubbo3 network logic to tripleConn, creat pb stub invoker
		tripleClient.stubInvoker = reflect.ValueOf(getInvoker(impl, newTripleConn(int(opt.Timeout), opt.Location, dialOpts...)))
	} else {
		tripleClient.triplConn = newTripleConn(int(opt.Timeout), opt.Location, dialOpts...)
	}
	return tripleClient, nil
}

// Invoke call remote using stub
func (t *TripleClient) Invoke(methodName string, in []reflect.Value, reply interface{}) common.ErrorWithAttachment {
	t.opt.Logger.Debugf("TripleClient.Invoke: methodName = %s, inputValue = %+v, expected reply struct = %+v, client defined codec = %s",
		methodName, in, reply, t.opt.CodecType)
	attachment := make(common.DubboAttachment)
	if t.opt.CodecType == constant.PBCodecName {
		method := t.stubInvoker.MethodByName(methodName)
		if method.IsZero() {
			t.opt.Logger.Errorf("TripleClient.Invoke: methodName %s not impl in triple client api.", methodName)
			return *common.NewErrorWithAttachment(status.Errorf(codes.Unimplemented, "TripleClient.Invoke: methodName %s not impl in triple client api.", methodName), attachment)
		}
		res := method.Call(in)
		errWithAtta, ok := res[1].Interface().(common.ErrorWithAttachment)
		if ok {
			t.opt.Logger.Debugf("TripleClient.Invoke: get result final struct is common.ErrorWithAttachment")
			if errWithAtta.GetError() != nil {
				t.opt.Logger.Debugf("TripleClient.Invoke: get result errorWithAttachment, error = %s", errWithAtta.GetError())
				return *common.NewErrorWithAttachment(errWithAtta.GetError(), attachment)
			}
			attachment = errWithAtta.GetAttachments()
			t.opt.Logger.Debugf("TripleClient.Invoke: get response attachement = %+v", attachment)
		} else if res[1].IsValid() && res[1].Interface() != nil {
			// compatible with not updated triple stub
			t.opt.Logger.Debugf("TripleClient.Invoke: get result final struct is error = %s", res[1].Interface().(error))
			return *common.NewErrorWithAttachment(res[1].Interface().(error), attachment)
		}
		t.opt.Logger.Debugf("TripleClient.Invoke: get reply = %+v", res[0])

		// deal with reflection panic
		errChan := make(chan error)
		go func() {
			defer func() {
				if e := recover(); e != nil {
					t.opt.Logger.Errorf("TripleClient.Invoke: response reflect to reply error = %+v", e)
					errChan <- errors.Errorf("TripleClient.Invoke: response reflect to reply error = %+v", e)
					return
				}
				errChan <- nil
			}()
			_ = tools.ReflectResponse(res[0], reply)
		}()
		if err := <-errChan; err != nil {
			return *common.NewErrorWithAttachment(err, attachment)
		}
	} else {
		ctx := in[0].Interface().(context.Context)
		interfaceKey := ctx.Value(constant.InterfaceKey).(string)
		t.opt.Logger.Debugf("TripleClient.Invoke: call with interfaceKey = %s", interfaceKey)
		reqParams := make([]interface{}, 0, len(in)-1)
		for idx, v := range in {
			if idx > 0 {
				reqParams = append(reqParams, v.Interface())
			}
		}

		var innerCodec encoding.Codec
		var err error
		switch t.opt.CodecType {
		case constant.HessianCodecName:
			innerCodec = hessian.NewHessianCodec()
		case constant.MsgPackCodecName:
			innerCodec = msgpack.NewMsgPackCodec()
		default:
			innerCodec, err = common.GetTripleCodec(t.opt.CodecType)
			if err != nil {
				return *common.NewErrorWithAttachment(status.Errorf(codes.Unimplemented, "TripleClient.Invoke: serialization %s not impl in triple client api.", t.opt.CodecType), attachment)
			}
		}

		argsBytes := make([][]byte, 0, len(reqParams))
		argsTypes := make([]string, 0, len(reqParams))
		for _, value := range reqParams {
			data, err := innerCodec.Marshal(value)
			if err != nil {
				return *common.NewErrorWithAttachment(err, nil)
			}
			argsBytes = append(argsBytes, data)
			argsTypes = append(argsTypes, javaType.GetArgType(value))
		}

		wrapperRequest := &proto_wrapper_api.TripleRequestWrapper{
			SerializeType: innerCodec.Name(),
			Args:          argsBytes,
			ArgTypes:      argsTypes,
		}

		// Wrap reply with wrapperResponse
		wrapperReply := &proto_wrapper_api.TripleResponseWrapper{}
		errWithAtta := t.triplConn.Invoke(ctx, "/"+interfaceKey+"/"+methodName, wrapperRequest, wrapperReply, grpc.ForceCodec(
			pbwrapper.NewPBWrapperTwoWayCodec(string(t.opt.CodecType), innerCodec, raw_proto.NewProtobufCodec())))

		// Empty response or error != nil
		if reply == nil || errWithAtta.GetError() != nil {
			return errWithAtta
		}

		// Unwrap reply from wrapperResponse
		unmarshalErr := innerCodec.Unmarshal(wrapperReply.Data, reply)
		if unmarshalErr != nil {
			return *common.NewErrorWithAttachment(status.Errorf(codes.Canceled, "TripleClient.Invoke: Unmarshal serialization %s error %v.", wrapperReply.SerializeType, unmarshalErr), attachment)
		}

		return errWithAtta
	}
	return *common.NewErrorWithAttachment(nil, attachment)
}

// Close destroy http controller and return
func (t *TripleClient) Close() {
	t.opt.Logger.Debug("Triple Client Is closing")
	if t.triplConn != nil && t.triplConn.grpcConn != nil {
		t.triplConn.grpcConn.Close()
	}
}

// IsAvailable returns if triple client is available
func (t *TripleClient) IsAvailable() bool {
	return true
}
