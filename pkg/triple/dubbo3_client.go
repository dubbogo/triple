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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"reflect"
	"sync"
)

import (
	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/codes"
	"github.com/dubbogo/grpc-go/credentials"
	"github.com/dubbogo/grpc-go/credentials/insecure"
	"github.com/dubbogo/grpc-go/encoding"
	"github.com/dubbogo/grpc-go/encoding/hessian"
	"github.com/dubbogo/grpc-go/encoding/msgpack"
	"github.com/dubbogo/grpc-go/encoding/raw_proto"
	"github.com/dubbogo/grpc-go/encoding/tools"
	"github.com/dubbogo/grpc-go/status"

	"github.com/opentracing/opentracing-go"

	"github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
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

	if creds, err := getClientTlsCertificate(opt); err != nil {
		opt.Logger.Errorf("TripleClient.Start: TLS config err: %v", err)
		return nil, err
	} else if creds != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		tripleClient.stubInvoker = reflect.ValueOf(getInvoker(impl, newTripleConn(opt.Timeout, opt.Location, dialOpts...)))
	} else {
		tripleClient.triplConn = newTripleConn(opt.Timeout, opt.Location, dialOpts...)
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
		return t.triplConn.Invoke(ctx, "/"+interfaceKey+"/"+methodName, reqParams, reply, grpc.ForceCodec(
			encoding.NewPBWrapperTwoWayCodec(string(t.opt.CodecType), innerCodec, raw_proto.NewProtobufCodec())))
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

func getClientTlsCertificate(opt *config.Option) (credentials.TransportCredentials, error) {
	// no TLS
	if opt.TLSCertFile == "" && opt.TLSKeyFile == "" {
		return nil, nil
	}

	if opt.CACertFile == "" {
		return credentials.NewClientTLSFromFile(opt.TLSCertFile, opt.TLSServerName)
	}

	// need mTLS
	ca := x509.NewCertPool()
	caBytes, err := ioutil.ReadFile(opt.CACertFile)
	if err != nil {
		return nil, err
	}
	if ok := ca.AppendCertsFromPEM(caBytes); !ok {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(opt.TLSCertFile, opt.TLSKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		ServerName:   opt.TLSServerName,
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
	}), nil
}
