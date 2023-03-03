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
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"sync"
)

import (
	hessian "github.com/apache/dubbo-go-hessian2"

	"github.com/dubbogo/grpc-go"
	"github.com/dubbogo/grpc-go/credentials/insecure"
	"github.com/dubbogo/grpc-go/encoding"
	hessianGRPCCodec "github.com/dubbogo/grpc-go/encoding/hessian"
	"github.com/dubbogo/grpc-go/encoding/msgpack"
	"github.com/dubbogo/grpc-go/encoding/proto_wrapper_api"
	"github.com/dubbogo/grpc-go/encoding/raw_proto"

	perrors "github.com/pkg/errors"

	"github.com/dubbogo/grpc-go/credentials"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/tracing"
)

// TripleServer is the object that can be started and listening remote request
type TripleServer struct {
	lst           net.Listener
	grpcServer    *grpc.Server
	rpcServiceMap *sync.Map
	registeredKey map[string]bool
	// config
	opt *config.Option
}

// NewTripleServer can create Server with url and some user impl providers stored in @serviceMap
// @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
func NewTripleServer(serviceMap *sync.Map, opt *config.Option) *TripleServer {
	if opt == nil {
		opt = config.NewTripleOption()
	}
	return &TripleServer{
		rpcServiceMap: serviceMap,
		opt:           opt,
		registeredKey: make(map[string]bool),
	}
}

// Stop
func (t *TripleServer) Stop() {
	t.lst.Close()
}

/*
var Greeter_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.Greeter",
	HandlerType: (*GreeterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHello",
			Handler:    _Greeter_SayHello_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SayHelloStream",
			Handler:       _Greeter_SayHelloStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "samples_api.proto",
}

*/

/*

func _Greeter_SayHello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	invo := invocation.NewRPCInvocation("SayHello", args, nil)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Greeter/SayHello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GreeterServer).SayHello(ctx, req.(*HelloRequest))
	}
	return interceptor(ctx, in, info, handler)
}
*/

func newGenericCodec() common.GenericCodec {
	return &GenericCodec{
		codec: raw_proto.NewProtobufCodec(),
	}
}

// GenericCodec is pb impl of TwoWayCodec
type GenericCodec struct {
	codec encoding.Codec
}

// UnmarshalRequest unmarshal bytes @data to interface
func (h *GenericCodec) UnmarshalRequest(data []byte) ([]interface{}, error) {
	wrapperRequest := proto_wrapper_api.TripleRequestWrapper{}
	err := h.codec.Unmarshal(data, &wrapperRequest)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, 0, len(wrapperRequest.Args))

	for _, value := range wrapperRequest.Args {
		decoder := hessian.NewDecoder(value)
		val, err := decoder.Decode()
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func createGrpcDesc(serviceName string, service common.TripleUnaryService) *grpc.ServiceDesc {
	genericCodec := newGenericCodec()
	return &grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*common.TripleUnaryService)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "InvokeWithArgs",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					methodName := ctx.Value("XXX_TRIPLE_GO_METHOD_NAME").(string)
					genericPayload, ok := ctx.Value("XXX_TRIPLE_GO_GENERIC_PAYLOAD").([]byte)
					base := srv.(common.TripleUnaryService)
					if methodName == "$invoke" && ok {
						args, err := genericCodec.UnmarshalRequest(genericPayload)
						if err != nil {
							return nil, perrors.Errorf("unaryProcessor.processUnaryRPC: generic invoke with request %s unmarshal error = %s", string(genericPayload), err.Error())
						}
						return base.InvokeWithArgs(ctx, methodName, args)
					} else {

						reqParam, ok := service.GetReqParamsInterfaces(methodName)
						if !ok {
							return nil, perrors.Errorf("method name %s is not provided by service, please check if correct", methodName)
						}
						if e := dec(reqParam); e != nil {
							return nil, e
						}
						args := make([]interface{}, 0, len(reqParam))
						for _, v := range reqParam {
							tempParamObj := reflect.ValueOf(v).Elem().Interface()
							args = append(args, tempParamObj)
						}
						return base.InvokeWithArgs(ctx, methodName, args)
					}
				},
			},
		},
	}
}

func newGrpcServerWithCodec(opt *config.Option) *grpc.Server {
	var innerCodec encoding.Codec
	serverOpts := []grpc.ServerOption{}

	if opt.JaegerAddress != "" {
		tracer := tracing.NewJaegerTracerDirect(opt.JaegerServiceName, opt.JaegerAddress, opt.Logger)
		serverOpts = append(serverOpts,
			grpc.UnaryInterceptor(tracing.OpenTracingServerInterceptor(tracer)),
			grpc.StreamInterceptor(tracing.OpenTracingStreamServerInterceptor(tracer)),
		)
	}

	if opt.GRPCMaxServerRecvMsgSize != 0 {
		serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(opt.GRPCMaxServerRecvMsgSize))
	}
	if opt.GRPCMaxCallSendMsgSize != 0 {
		serverOpts = append(serverOpts, grpc.MaxSendMsgSize(opt.GRPCMaxCallSendMsgSize))
	}

	if opt.ProxyModeEnable {
		serverOpts = append(serverOpts, grpc.ProxyModeEnable(true))
	}
	// TLS config
	if creds, err := getServerTlsCertificate(opt); err != nil {
		if err != nil {
			opt.Logger.Errorf("TripleClient.Start: TLS config err: %v", err)
		}
	} else if creds != nil {
		serverOpts = append(serverOpts, grpc.Creds(creds))
	} else {
		serverOpts = append(serverOpts, grpc.Creds(insecure.NewCredentials()))
	}

	var err error
	switch opt.CodecType {
	case constant.PBCodecName:
		return grpc.NewServer(serverOpts...)
	case constant.HessianCodecName:
		innerCodec = hessianGRPCCodec.NewHessianCodec()
	case constant.MsgPackCodecName:
		innerCodec = msgpack.NewMsgPackCodec()
	default:
		innerCodec, err = common.GetTripleCodec(opt.CodecType)
		if err != nil {
			fmt.Printf("TripleServer.Start: serialization %s not supported", opt.CodecType)
		}
	}
	serverOpts = append(serverOpts, grpc.ForceServerCodec(encoding.NewPBWrapperTwoWayCodec(string(opt.CodecType), innerCodec, raw_proto.NewProtobufCodec())))

	return grpc.NewServer(serverOpts...)
}

// Start can start a triple server
func (t *TripleServer) Start() {
	lst, err := net.Listen("tcp", t.opt.Location)
	if err != nil {
		panic(err)
	}
	grpcServer := newGrpcServerWithCodec(t.opt)
	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		t.registeredKey[key.(string)] = true
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
		} else {
			desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
			grpcServer.RegisterService(desc, value)
		}
		if key == "grpc.reflection.v1alpha.ServerReflection" {
			grpcService.(common.TripleGrpcReflectService).SetGRPCServer(grpcServer)
		}
		return true
	})

	go grpcServer.Serve(lst)
	t.lst = lst
	t.grpcServer = grpcServer
}

func (t *TripleServer) RefreshService() {
	t.opt.Logger.Debugf("TripleServer.Refresh: call refresh services")
	grpcServer := newGrpcServerWithCodec(t.opt)
	t.rpcServiceMap.Range(func(key, value interface{}) bool {
		grpcService, ok := value.(common.TripleGrpcService)
		if ok {
			desc := grpcService.XXX_ServiceDesc()
			desc.ServiceName = key.(string)
			grpcServer.RegisterService(desc, value)
		} else {
			desc := createGrpcDesc(key.(string), value.(common.TripleUnaryService))
			grpcServer.RegisterService(desc, value)
		}
		if key == "grpc.reflection.v1alpha.ServerReflection" {
			grpcService.(common.TripleGrpcReflectService).SetGRPCServer(grpcServer)
		}
		return true
	})
	t.grpcServer.Stop()
	t.lst.Close()
	lst, _ := net.Listen("tcp", t.opt.Location)
	go grpcServer.Serve(lst)
	t.grpcServer = grpcServer
	t.lst = lst
}

func getServerTlsCertificate(opt *config.Option) (credentials.TransportCredentials, error) {
	// no TLS
	if opt.TLSCertFile == "" && opt.TLSKeyFile == "" {
		return nil, nil
	}
	var ca *x509.CertPool
	cfg := &tls.Config{}
	// need mTLS
	if opt.CACertFile != "" {
		ca = x509.NewCertPool()
		caBytes, err := ioutil.ReadFile(opt.CACertFile)
		if err != nil {
			return nil, err
		}
		if ok := ca.AppendCertsFromPEM(caBytes); !ok {
			return nil, err
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = ca
	}
	cert, err := tls.LoadX509KeyPair(opt.TLSCertFile, opt.TLSKeyFile)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = []tls.Certificate{cert}
	cfg.ServerName = opt.TLSServerName

	return credentials.NewTLS(cfg), nil
}
